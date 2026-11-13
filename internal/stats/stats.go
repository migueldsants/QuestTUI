// Package stats calcula as conquistas a partir do histórico de conclusões.
package stats

import (
	"time"

	"questtui/internal/lang"
	"questtui/internal/store"
)

type Medal struct {
	Name, Desc string
	Got        bool
}

type Stats struct {
	Total, Today, Week, Month int
	Streak, BestStreak        int // dias seguidos com ao menos uma conclusão
	BestDay                   int
	BestDayDate               time.Time
	Days                      []int // últimos 14 dias, do mais antigo a hoje
	Weeks                     []int // últimas 8 semanas (segunda a domingo), a atual por último
	MainDone                  int
	Medals                    []Medal
}

func day(t time.Time) time.Time { return store.DateOnly(t.Local()) }

// weekStart devolve a segunda-feira da semana de t.
func weekStart(t time.Time) time.Time {
	d := day(t)
	return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
}

func Compute(cs []store.Completion, now time.Time) Stats {
	var s Stats
	today := day(now)
	perDay := map[time.Time]int{}
	for _, c := range cs {
		d := day(c.At)
		perDay[d]++
		s.Total++
		if c.Kind == store.Main {
			s.MainDone++
		}
		if d.Equal(today) {
			s.Today++
		}
		if !d.Before(weekStart(now)) {
			s.Week++
		}
		if d.Year() == today.Year() && d.Month() == today.Month() {
			s.Month++
		}
	}

	// Melhor dia e maior sequência.
	var first time.Time
	for d, n := range perDay {
		if n > s.BestDay || (n == s.BestDay && d.After(s.BestDayDate)) {
			s.BestDay, s.BestDayDate = n, d
		}
		if first.IsZero() || d.Before(first) {
			first = d
		}
	}
	run := 0
	for d := first; !first.IsZero() && !d.After(today); d = d.AddDate(0, 0, 1) {
		if perDay[d] > 0 {
			run++
			s.BestStreak = max(s.BestStreak, run)
		} else {
			run = 0
		}
	}
	// A sequência atual conta até hoje; se hoje ainda não teve conclusão,
	// ela continua valendo a partir de ontem.
	d := today
	if perDay[d] == 0 {
		d = d.AddDate(0, 0, -1)
	}
	for perDay[d] > 0 {
		s.Streak++
		d = d.AddDate(0, 0, -1)
	}

	for i := 13; i >= 0; i-- {
		s.Days = append(s.Days, perDay[today.AddDate(0, 0, -i)])
	}
	thisWeek := weekStart(now)
	for i := 7; i >= 0; i-- {
		start := thisWeek.AddDate(0, 0, -7*i)
		n := 0
		for k := 0; k < 7; k++ {
			n += perDay[start.AddDate(0, 0, k)]
		}
		s.Weeks = append(s.Weeks, n)
	}

	t := lang.T
	s.Medals = []Medal{
		{t("Primeiro passo", "First step"), t("conclua uma quest", "complete a quest"), s.Total >= 1},
		{t("Aventureiro", "Adventurer"), t("10 quests concluídas", "10 quests completed"), s.Total >= 10},
		{t("Veterano", "Veteran"), t("50 quests concluídas", "50 quests completed"), s.Total >= 50},
		{t("Lenda", "Legend"), t("100 quests concluídas", "100 quests completed"), s.Total >= 100},
		{t("Grande jornada", "Grand journey"), t("conclua uma quest principal", "complete a main quest"), s.MainDone >= 1},
		{t("Dia heroico", "Heroic day"), t("5 quests num só dia", "5 quests in a single day"), s.BestDay >= 5},
		{t("Constância", "Steady"), t("3 dias seguidos", "3 days in a row"), s.BestStreak >= 3},
		{t("Semana perfeita", "Perfect week"), t("7 dias seguidos", "7 days in a row"), s.BestStreak >= 7},
		{t("Mês invicto", "Unbroken month"), t("30 dias seguidos", "30 days in a row"), s.BestStreak >= 30},
	}
	return s
}
