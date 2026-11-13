package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"questtui/internal/store"
)

// Calendário (w): o mês em grade, com os prazos marcados em cada dia, e ao
// lado as quests e a nota do dia escolhido. Dá para pular para uma quest no
// diário, criar uma nova já com o prazo naquele dia ou escrever a nota do dia
// no mesmo editor das notas das quests.

var (
	monthsPT = []string{"janeiro", "fevereiro", "março", "abril", "maio", "junho",
		"julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}
	weekdaysPT = []string{"domingo", "segunda-feira", "terça-feira", "quarta-feira",
		"quinta-feira", "sexta-feira", "sábado"}
	weekdaysShortPT = []string{"Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"}
	weekdaysShortEN = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
)

func monthName(mo time.Month) string {
	return tr(monthsPT[mo-1], mo.String())
}

func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// longDate escreve "sexta-feira, 13 de novembro" ou "Friday, November 13".
func longDate(d time.Time) string {
	if tr("pt", "en") == "pt" {
		return fmt.Sprintf("%s, %d de %s", weekdaysPT[d.Weekday()], d.Day(), monthsPT[d.Month()-1])
	}
	return d.Format("Monday, January 2")
}

// addMonths anda n meses mantendo o dia, ou o último dia do mês quando ele
// não existe (31/01 + 1 mês = 28/02).
func addMonths(d time.Time, n int) time.Time {
	first := time.Date(d.Year(), d.Month()+time.Month(n), 1, 0, 0, 0, 0, time.Local)
	last := first.AddDate(0, 1, -1).Day()
	return time.Date(first.Year(), first.Month(), min(d.Day(), last), 0, 0, 0, 0, time.Local)
}

// openCalendar abre o calendário no dia de hoje.
func (m *Model) openCalendar() {
	m.calDay = store.DateOnly(m.now)
	m.calList, m.calCursor = false, 0
	m.mode = modeCalendar
}

// dayMarks resume os prazos de um dia.
type dayMarks struct{ pending, late, done int }

// monthMarks conta, para cada dia do mês de calDay, as quests que vencem nele.
func (m *Model) monthMarks() map[int]dayMarks {
	marks := map[int]dayMarks{}
	for _, q := range m.quests {
		t, _, ok := q.DueTime()
		if !ok || t.Year() != m.calDay.Year() || t.Month() != m.calDay.Month() {
			continue
		}
		mk := marks[t.Day()]
		switch _, late, _ := m.dueInfo(q); {
		case q.Done:
			mk.done++
		case late:
			mk.late++
		default:
			mk.pending++
		}
		marks[t.Day()] = mk
	}
	return marks
}

// calItems são as quests mostradas ao lado da grade: as que vencem no dia
// escolhido (pendentes primeiro, as sem hora antes, depois por horário) e, se
// o dia é hoje, as atrasadas de dias anteriores. nDay é quantas são do dia.
func (m *Model) calItems() (items []store.Quest, nDay int) {
	today := store.DateOnly(m.now)
	var late []store.Quest
	for _, q := range m.quests {
		t, _, ok := q.DueTime()
		if !ok {
			continue
		}
		day := store.DateOnly(t)
		switch {
		case day.Equal(m.calDay):
			items = append(items, q)
		case m.calDay.Equal(today) && day.Before(today) && !q.Done:
			late = append(late, q)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Done != b.Done {
			return !a.Done
		}
		ta, ha, _ := a.DueTime()
		tb, hb, _ := b.DueTime()
		if ha != hb {
			return !ha
		}
		return ta.Before(tb)
	})
	sort.SliceStable(late, func(i, j int) bool { return late[i].Due < late[j].Due })
	return append(items, late...), len(items)
}

func (m *Model) updateCalendar(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	items, _ := m.calItems()
	if m.calList {
		m.calCursor = clamp(m.calCursor, 0, len(items)-1)
		switch key.String() {
		case "up", "k":
			m.calCursor = clamp(m.calCursor-1, 0, len(items)-1)
		case "down", "j":
			m.calCursor = clamp(m.calCursor+1, 0, len(items)-1)
		case "o":
			m.openDayNote(m.calDay)
			return m, nil
		case "enter":
			if len(items) > 0 {
				m.jumpTo(items[m.calCursor])
			}
		case " ", "x":
			if len(items) > 0 && !items[m.calCursor].Done {
				m.advance(items[m.calCursor])
			}
		case "esc", "tab", "left", "h":
			m.calList = false
		case "n":
			return m, m.openFormFrom(modeCalendar, m.calDay)
		case "q", "w":
			m.mode = modeList
		case "?", "f1":
			m.openHelp()
		}
		if items, _ = m.calItems(); len(items) == 0 {
			m.calList = false
		}
		return m, nil
	}

	switch key.String() {
	case "left", "h":
		m.calDay = m.calDay.AddDate(0, 0, -1)
	case "right", "l":
		m.calDay = m.calDay.AddDate(0, 0, 1)
	case "up", "k":
		m.calDay = m.calDay.AddDate(0, 0, -7)
	case "down", "j":
		m.calDay = m.calDay.AddDate(0, 0, 7)
	case "[", "<", "pgup", "H":
		m.calDay = addMonths(m.calDay, -1)
	case "]", ">", "pgdown", "L":
		m.calDay = addMonths(m.calDay, 1)
	case "{":
		m.calDay = addMonths(m.calDay, -12)
	case "}":
		m.calDay = addMonths(m.calDay, 12)
	case ".", "home", "g":
		m.calDay = store.DateOnly(m.now)
	case "enter":
		switch len(items) {
		case 0:
		case 1:
			m.jumpTo(items[0])
		default:
			m.calList, m.calCursor = true, 0
		}
	case "tab":
		if len(items) > 0 {
			m.calList, m.calCursor = true, 0
		}
	case "n":
		return m, m.openFormFrom(modeCalendar, m.calDay)
	case "o":
		m.openDayNote(m.calDay)
	case "esc", "q", "w":
		m.mode = modeList
	case "?", "f1":
		m.openHelp()
	}
	return m, nil
}

// jumpTo volta ao diário com a quest selecionada (na aba certa e sem filtro).
func (m *Model) jumpTo(q store.Quest) {
	m.mode = modeList
	m.filter = ""
	m.doneTab = q.Done
	m.focusSteps = false
	m.rebuild(q.ID)
}

// --- desenho ---

func (m *Model) viewCalendar() string {
	bodyH := m.h - 2
	if bodyH < 3 {
		return m.header()
	}
	var body string
	if m.w < 70 {
		// Terminal estreito: a grade e, embaixo, as quests do dia.
		cellW := clamp((m.w-4)/7, 4, 8)
		content := m.calGrid(cellW, 0) + "\n\n" + m.calDayList(m.w-4)
		body = m.pane(content, m.w, bodyH, true)
	} else {
		cellW := clamp((m.w*45/100-4)/7, 4, 8)
		leftW := 7*cellW + 4
		rightW := m.w - leftW
		left := m.pane(m.calGrid(cellW, bodyH-2), leftW, bodyH, !m.calList)
		right := m.pane(m.calDayList(rightW-4), rightW, bodyH, m.calList)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}
	return m.header() + "\n" + body + "\n" + m.calFooter()
}

// calGrid desenha o mês: título, dias da semana e seis semanas. Com altura
// sobrando (innerH), as semanas ganham uma linha de respiro entre si.
func (m *Model) calGrid(cellW, innerH int) string {
	width := 7 * cellW
	day := m.calDay
	today := store.DateOnly(m.now)
	marks := m.monthMarks()

	var lines []string
	title := capitalize(monthName(day.Month())) + " " + fmt.Sprint(day.Year())
	arrows := m.st.muted.Render("‹ ") + m.st.accent.Render(title) + m.st.muted.Render(" ›")
	lines = append(lines, lipgloss.PlaceHorizontal(width, lipgloss.Center, arrows), "")

	names := weekdaysShortEN
	if tr("pt", "en") == "pt" {
		names = weekdaysShortPT
	}
	var head strings.Builder
	for i, n := range names {
		if cellW < 5 {
			n = string([]rune(n)[:2])
		}
		style := m.st.muted
		if i == 0 || i == 6 {
			style = m.st.accent2
		}
		head.WriteString(style.Render(lipgloss.PlaceHorizontal(cellW, lipgloss.Center, n)))
	}
	lines = append(lines, head.String())

	gap := innerH >= 22
	first := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.Local)
	start := first.AddDate(0, 0, -int(first.Weekday()))
	dot, noteMark := m.ic.Dot, m.ic.Notes
	// O dia escolhido pulsa devagar enquanto a grade está em foco.
	pulse := blend(m.th.Accent, m.hi, 0.5+0.5*math.Sin(m.t*3))
	for w := 0; w < 6; w++ {
		if gap {
			lines = append(lines, "")
		}
		var row strings.Builder
		for i := 0; i < 7; i++ {
			d := start.AddDate(0, 0, w*7+i)
			if d.Month() != day.Month() {
				row.WriteString(strings.Repeat(" ", cellW))
				continue
			}
			selected := d.Equal(day)
			bg := func(s lipgloss.Style) lipgloss.Style {
				if !selected {
					return s
				}
				if m.th.Selection != "" {
					return s.Background(lipgloss.Color(m.th.Selection))
				}
				return s.Reverse(true)
			}

			numStyle := m.st.text
			switch {
			case selected && !m.calList:
				numStyle = paint(lipgloss.NewStyle().Bold(true), pulse)
			case selected:
				numStyle = m.st.accent
			case d.Equal(today):
				numStyle = m.st.accent.Underline(true)
			case d.Before(today):
				numStyle = m.st.muted
			}
			mark, markStyle := " ", m.st.muted
			mk := marks[d.Day()]
			switch {
			case mk.late > 0:
				mark, markStyle = dot, m.st.danger
			case mk.pending > 0:
				mark, markStyle = dot, m.st.accent3
			case mk.done > 0:
				mark, markStyle = dot, m.st.done
			}
			// Célula: o número alinhado à direita em duas colunas e a marca
			// logo depois, centralizados na largura da célula.
			left := (cellW - 3) / 2
			right := cellW - 3 - left
			if selected && d.Equal(today) {
				numStyle = numStyle.Underline(true)
			}
			// Dia com nota: o símbolo das notas logo depois da marca de prazo.
			note := " "
			if m.dayNotes[store.DayKey(d)] != "" {
				note = noteMark
			}
			row.WriteString(bg(lipgloss.NewStyle()).Render(strings.Repeat(" ", left)) +
				bg(numStyle).Render(fmt.Sprintf("%2d", d.Day())) +
				bg(markStyle).Render(mark) +
				bg(m.st.accent2).Render(note) +
				bg(lipgloss.NewStyle()).Render(strings.Repeat(" ", right-1)))
		}
		lines = append(lines, row.String())
	}

	// Legenda e totais do mês.
	total, notes := 0, 0
	for _, mk := range marks {
		total += mk.pending + mk.late + mk.done
	}
	prefix := fmt.Sprintf("%04d-%02d-", day.Year(), day.Month())
	for k, v := range m.dayNotes {
		if strings.HasPrefix(k, prefix) && v != "" {
			notes++
		}
	}
	lines = append(lines, "")
	legend := m.st.danger.Render(dot) + m.st.muted.Render(tr(" atrasada  ", " late  ")) +
		m.st.accent3.Render(dot) + m.st.muted.Render(tr(" pendente  ", " pending  ")) +
		m.st.done.Render(dot) + m.st.muted.Render(tr(" concluída", " done"))
	lines = append(lines, ansi.Truncate(legend, width, ""))
	summary := m.st.muted.Render(plural(total, tr("prazo", "deadline"), tr("prazos", "deadlines"))+" · ") +
		m.st.accent2.Render(noteMark) +
		m.st.muted.Render(" "+plural(notes, tr("nota", "note"), tr("notas", "notes"))+tr(" neste mês", " this month"))
	lines = append(lines, ansi.Truncate(summary, width, ""))
	return strings.Join(lines, "\n")
}

// relDay descreve o dia em relação a hoje.
func (m *Model) relDay(d time.Time) string {
	n := int(math.Round(d.Sub(store.DateOnly(m.now)).Hours() / 24))
	switch {
	case n == 0:
		return tr("hoje", "today")
	case n == 1:
		return tr("amanhã", "tomorrow")
	case n == -1:
		return tr("ontem", "yesterday")
	case n > 1:
		return fmt.Sprintf(tr("em %d dias", "in %d days"), n)
	}
	return fmt.Sprintf(tr("há %d dias", "%d days ago"), -n)
}

// calDayList desenha as quests do dia escolhido.
func (m *Model) calDayList(width int) string {
	ic := m.ic
	var b strings.Builder
	head := strings.ToUpper(longDate(m.calDay))
	b.WriteString(m.section(ic.Calendar, head, width) + "\n")
	b.WriteString(m.st.accent3.Render(m.relDay(m.calDay)))
	if y := m.calDay.Year(); y != m.now.Year() {
		b.WriteString(m.st.muted.Render(fmt.Sprintf(" · %d", y)))
	}
	b.WriteString("\n\n")

	items, nDay := m.calItems()
	if nDay == 0 {
		b.WriteString(m.st.muted.Width(width).Render(tr(
			"Nenhuma quest com prazo neste dia.\n\nPressione n para criar uma.",
			"No quests due on this day.\n\nPress n to create one.")))
		b.WriteString("\n")
	}
	for i, q := range items {
		if i == nDay {
			if nDay > 0 {
				b.WriteString("\n")
			}
			b.WriteString(m.section(ic.Late, tr("ATRASADAS", "OVERDUE"), width) + "\n")
		}
		b.WriteString(m.calRow(q, i, i >= nDay, width) + "\n")
	}

	b.WriteString("\n" + m.section(ic.Notes, tr("NOTA DO DIA", "DAY NOTE"), width) + "\n")
	if note := m.dayNotes[store.DayKey(m.calDay)]; note != "" {
		b.WriteString(m.renderDayNote(note, width))
	} else {
		b.WriteString(m.st.muted.Render(tr("Sem nota. Pressione o para escrever.", "No note. Press o to write one.")))
	}
	return b.String()
}

// renderDayNote renderiza o markdown da nota do dia, com cache.
func (m *Model) renderDayNote(note string, width int) string {
	key := fmt.Sprintf("%d|%s|%s", width, m.glamourStyle(), note)
	if key == m.dayNoteKey {
		return m.dayNoteOut
	}
	out := note
	if r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(m.glamourStyle()), glamour.WithWordWrap(width-2)); err == nil {
		if rendered, err := r.Render(note); err == nil {
			out = strings.Trim(rendered, "\n")
		}
	}
	m.dayNoteKey, m.dayNoteOut = key, out
	return out
}

// calRow é uma quest na lista do dia: cursor, tipo, horário, título e etapas.
func (m *Model) calRow(q store.Quest, i int, overdue bool, width int) string {
	ic := m.ic
	selected := m.calList && i == m.calCursor
	cur := strings.Repeat(" ", lipgloss.Width(ic.Cursor)+1)
	if selected {
		cur = ic.Cursor + " "
	}
	mark, markStyle := ic.Side, m.st.accent2
	if q.Kind == store.Main {
		mark = ic.Main
	}
	if q.Done {
		mark, markStyle = ic.Done, m.st.done
	}

	t, hasTime, _ := q.DueTime()
	when := "     "
	switch {
	case overdue:
		when = t.Format("02/01")
	case hasTime:
		when = t.Format("15:04")
	}
	_, late, _ := m.dueInfo(q)
	whenStyle := m.st.muted
	if late && !q.Done {
		whenStyle = m.st.danger
	}

	right := ""
	if len(q.Steps) > 0 {
		right = fmt.Sprintf("%d/%d", q.StepsDone(), len(q.Steps))
	}
	prefix := ""
	if q.Priority && !q.Done {
		prefix = ic.Priority + " "
	}
	titleW := width - lipgloss.Width(cur) - lipgloss.Width(mark) - 1 - len(when) - 1 - lipgloss.Width(prefix)
	if right != "" {
		titleW -= len(right) + 1
	}
	titleW = max(titleW, 4)
	title := ansi.Truncate(q.Title, titleW, "…")

	style := m.st.text
	switch {
	case q.Done:
		style = m.st.muted.Strikethrough(true)
	case selected:
		style = m.st.accent
	}
	line := m.st.accent.Render(cur) + markStyle.Render(mark) + " " + whenStyle.Render(when) + " "
	if prefix != "" {
		line += m.st.danger.Render(prefix)
	}
	line += style.Render(title)
	if right != "" {
		line += strings.Repeat(" ", max(1, titleW-lipgloss.Width(title)+1)) + m.st.muted.Render(right)
	}
	return line
}

func (m *Model) calFooter() string {
	if m.err != nil {
		return " " + m.st.danger.Render(ansi.Truncate(m.ic.Late+tr(" erro: ", " error: ")+m.err.Error(), m.w-2, "…"))
	}
	if m.note != "" && m.now.Before(m.noteUntil) {
		return " " + m.st.accent.Render(ansi.Truncate(m.ic.Spark+" "+m.note, m.w-2, "…"))
	}
	help := tr(
		"←→↑↓ dia · [ ] mês · . hoje · enter abre · tab lista · o nota do dia · n nova neste dia · esc volta",
		"←→↑↓ day · [ ] month · . today · enter opens · tab list · o day note · n new on this day · esc back")
	if m.calList {
		help = tr(
			"↑↓ escolher · enter abre no diário · espaço avança · o nota do dia · n nova · esc volta ao mês",
			"↑↓ choose · enter opens in the journal · space advances · o day note · n new · esc back to the month")
	}
	return " " + m.st.muted.Render(ansi.Truncate(help, m.w-2, "…"))
}
