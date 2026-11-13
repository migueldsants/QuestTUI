package store

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"questtui/internal/lang"
)

// Prazos: guardados como "AAAA-MM-DD" ou "AAAA-MM-DD HH:MM", e digitados em
// texto livre: "hoje", "amanhã 14h", "sexta", "+3d", "20/11 14:00".

const (
	dateLayout     = "2006-01-02"
	dateTimeLayout = "2006-01-02 15:04"
)

// DateOnly devolve a meia-noite (local) do dia de t.
func DateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// DueTime devolve o prazo. hasTime diz se ele tem hora; ok é false quando a
// quest não tem prazo.
func (q Quest) DueTime() (t time.Time, hasTime, ok bool) {
	return parseStored(q.Due)
}

func parseStored(due string) (time.Time, bool, bool) {
	if due == "" {
		return time.Time{}, false, false
	}
	if t, err := time.ParseInLocation(dateTimeLayout, due, time.Local); err == nil {
		return t, true, true
	}
	if t, err := time.ParseInLocation(dateLayout, due, time.Local); err == nil {
		return t, false, true
	}
	return time.Time{}, false, false
}

// FormatDue mostra o prazo como DD/MM/AAAA, com " HH:MM" quando tem hora.
// O resultado pode ser digitado de volta em ParseDue.
func FormatDue(due string) string {
	t, hasTime, ok := parseStored(due)
	switch {
	case !ok:
		return ""
	case hasTime:
		return t.Format("02/01/2006 15:04")
	}
	return t.Format("02/01/2006")
}

func dueError() error {
	return errors.New(lang.T(
		"use hoje, amanhã, sexta, +3d ou DD/MM, com hora opcional (14:00)",
		"use today, tomorrow, friday, +3d or DD/MM, with an optional time (14:00)"))
}

// ParseDue lê o prazo digitado e devolve a forma guardada. Texto vazio é
// "sem prazo".
func ParseDue(s string, now time.Time) (string, error) {
	fields := strings.Fields(strings.ToLower(s))
	if len(fields) == 0 {
		return "", nil
	}

	hour, minute, hasTime := 0, 0, false
	if h, m, ok := parseClock(fields[len(fields)-1]); ok {
		hour, minute, hasTime = h, m, true
		fields = fields[:len(fields)-1]
		// "sexta às 14h", "friday at 2:00"
		if n := len(fields); n > 0 && (fields[n-1] == "às" || fields[n-1] == "as" || fields[n-1] == "at") {
			fields = fields[:n-1]
		}
	}

	day, err := parseDay(strings.Join(fields, " "), DateOnly(now))
	if err != nil {
		return "", err
	}
	if hasTime {
		return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, time.Local).Format(dateTimeLayout), nil
	}
	return day.Format(dateLayout), nil
}

// parseClock lê "14:00", "9:30", "14h" ou "9h30".
func parseClock(s string) (hour, minute int, ok bool) {
	var hs, ms string
	switch {
	case strings.Contains(s, ":"):
		hs, ms, _ = strings.Cut(s, ":")
	case strings.HasSuffix(s, "h") || strings.Contains(s, "h"):
		hs, ms, _ = strings.Cut(s, "h")
	default:
		return 0, 0, false
	}
	h, err := strconv.Atoi(hs)
	if err != nil || h < 0 || h > 23 {
		return 0, 0, false
	}
	if ms == "" {
		return h, 0, true
	}
	m, err := strconv.Atoi(ms)
	if err != nil || len(ms) != 2 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

var weekdayNames = map[string]time.Weekday{
	"domingo": time.Sunday, "dom": time.Sunday, "sunday": time.Sunday, "sun": time.Sunday,
	"segunda": time.Monday, "seg": time.Monday, "monday": time.Monday, "mon": time.Monday,
	"terça": time.Tuesday, "terca": time.Tuesday, "ter": time.Tuesday, "tuesday": time.Tuesday, "tue": time.Tuesday,
	"quarta": time.Wednesday, "qua": time.Wednesday, "wednesday": time.Wednesday, "wed": time.Wednesday,
	"quinta": time.Thursday, "qui": time.Thursday, "thursday": time.Thursday, "thu": time.Thursday,
	"sexta": time.Friday, "sex": time.Friday, "friday": time.Friday, "fri": time.Friday,
	"sábado": time.Saturday, "sabado": time.Saturday, "sáb": time.Saturday, "sab": time.Saturday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

// weekdayOf reconhece o nome de um dia da semana ("segunda-feira" também).
func weekdayOf(word string) (time.Weekday, bool) {
	wd, ok := weekdayNames[strings.TrimSuffix(word, "-feira")]
	return wd, ok
}

// parseDay lê a parte da data; texto vazio é hoje (quando só a hora foi dada).
func parseDay(text string, today time.Time) (time.Time, error) {
	switch text {
	case "", "hoje", "today":
		return today, nil
	case "amanhã", "amanha", "tomorrow":
		return today.AddDate(0, 0, 1), nil
	case "depois de amanhã", "depois de amanha":
		return today.AddDate(0, 0, 2), nil
	}
	// Dia da semana: a próxima ocorrência, contando hoje.
	if wd, ok := weekdayOf(text); ok {
		return today.AddDate(0, 0, (int(wd)-int(today.Weekday())+7)%7), nil
	}
	if d, ok := parseOffset(text, today); ok {
		return d, nil
	}
	return parseDate(text, today)
}

// parseOffset lê "+3d", "+2s", "+2w", "+1m", "em 3 dias", "in 2 weeks".
func parseOffset(text string, today time.Time) (time.Time, bool) {
	t := strings.TrimPrefix(text, "+")
	t = strings.TrimPrefix(t, "em ")
	t = strings.TrimPrefix(t, "in ")
	i := 0
	for i < len(t) && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	if i == 0 {
		return time.Time{}, false
	}
	n, _ := strconv.Atoi(t[:i])
	switch strings.TrimSpace(t[i:]) {
	case "d", "dia", "dias", "day", "days":
		return today.AddDate(0, 0, n), true
	case "s", "sem", "semana", "semanas", "w", "week", "weeks":
		return today.AddDate(0, 0, 7*n), true
	case "m", "mês", "mes", "meses", "month", "months":
		return today.AddDate(0, n, 0), true
	}
	return time.Time{}, false
}

// parseDate lê DD/MM, DD/MM/AA ou DD/MM/AAAA. Sem ano, uma data que já
// passou neste ano vai para o ano que vem.
func parseDate(text string, today time.Time) (time.Time, error) {
	parts := strings.Split(text, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return time.Time{}, dueError()
	}
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return time.Time{}, dueError()
		}
		nums[i] = n
	}
	year := today.Year()
	if len(nums) == 3 {
		year = nums[2]
		if year < 100 {
			year += 2000
		}
	}
	t := time.Date(year, time.Month(nums[1]), nums[0], 0, 0, 0, 0, time.Local)
	if t.Day() != nums[0] || int(t.Month()) != nums[1] {
		return time.Time{}, dueError() // 31/02 e afins
	}
	if len(nums) == 2 && t.Before(today) {
		t = t.AddDate(1, 0, 0)
	}
	return t, nil
}
