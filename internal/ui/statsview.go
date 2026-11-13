package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Tela de conquistas: números do histórico, gráficos e medalhas.

var sparkLevels = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

func (m *Model) viewStats() string {
	s := m.stats
	var b strings.Builder

	num := func(n int, label string) string {
		return m.st.accent.Render(fmt.Sprintf("%d", n)) + " " + m.st.muted.Render(label)
	}
	sep := m.st.muted.Render("  ·  ")
	b.WriteString(num(s.Today, tr("hoje", "today")) + sep + num(s.Week, tr("na semana", "this week")) + sep +
		num(s.Month, tr("no mês", "this month")) + sep + num(s.Total, tr("no total", "in total")))
	b.WriteString("\n")

	streak := fmt.Sprintf(tr("%s sequência: %d dias", "%s streak: %d days"), m.ic.Priority, s.Streak)
	if s.Streak == 1 {
		streak = fmt.Sprintf(tr("%s sequência: 1 dia", "%s streak: 1 day"), m.ic.Priority)
	}
	b.WriteString(m.st.accent2.Render(streak) + sep +
		m.st.muted.Render(fmt.Sprintf(tr("recorde %d", "best %d"), s.BestStreak)))
	if s.BestDay > 0 {
		b.WriteString(sep + m.st.muted.Render(fmt.Sprintf(tr("melhor dia: %d em %s", "best day: %d on %s"),
			s.BestDay, s.BestDayDate.Format("02/01"))))
	}
	b.WriteString("\n\n")

	// Últimos 14 dias, em colunas.
	b.WriteString(m.section(m.ic.Due, tr("ÚLTIMOS 14 DIAS", "LAST 14 DAYS"), 56) + "\n")
	peak := 1
	for _, n := range s.Days {
		peak = max(peak, n)
	}
	initials := []string{"D", "S", "T", "Q", "Q", "S", "S"}
	if tr("pt", "en") == "en" {
		initials = []string{"S", "M", "T", "W", "T", "F", "S"}
	}
	for row := 1; row >= 0; row-- { // duas linhas de altura: 16 níveis
		var line strings.Builder
		for i, n := range s.Days {
			level := n * 16 / peak
			if n > 0 && level == 0 {
				level = 1
			}
			cell := level - row*8
			cell = clamp(cell, 0, 8)
			color := blend(m.th.Accent2, m.th.Accent, float64(i)/13)
			line.WriteString(paint(lipgloss.NewStyle(), color).Render(strings.Repeat(sparkLevels[cell], 3)) + " ")
		}
		b.WriteString(line.String() + "\n")
	}
	today := time.Now()
	var labels strings.Builder
	for i := range s.Days {
		d := today.AddDate(0, 0, i-13)
		style := m.st.muted
		if i == 13 {
			style = m.st.accent
		}
		labels.WriteString(style.Render(fmt.Sprintf(" %s  ", initials[d.Weekday()])))
	}
	b.WriteString(labels.String() + "\n\n")

	// Últimas 6 semanas, em barras.
	b.WriteString(m.section(m.ic.Steps, tr("SEMANAS", "WEEKS"), 56) + "\n")
	weeks := s.Weeks[max(0, len(s.Weeks)-6):]
	peak = 1
	for _, n := range weeks {
		peak = max(peak, n)
	}
	for i, n := range weeks {
		label := fmt.Sprintf(tr("%d sem atrás", "%d wk ago"), len(weeks)-1-i)
		if i == len(weeks)-1 {
			label = tr("esta semana", "this week")
		}
		b.WriteString(m.st.muted.Render(fmt.Sprintf("%-12s", label)) + " " +
			m.bar(32, float64(n)/float64(peak)) + " " + m.st.text.Render(fmt.Sprintf("%d", n)) + "\n")
	}
	b.WriteString("\n")

	// Medalhas, em duas colunas.
	got := 0
	for _, md := range s.Medals {
		if md.Got {
			got++
		}
	}
	b.WriteString(m.section(m.ic.Trophy, fmt.Sprintf(tr("MEDALHAS %d/%d", "MEDALS %d/%d"), got, len(s.Medals)), 56) + "\n")
	medal := func(i int) string {
		md := s.Medals[i]
		icon, name, desc := m.st.muted.Render(m.ic.Lock), m.st.muted.Render(md.Name), m.st.muted.Render(md.Desc)
		if md.Got {
			icon, name, desc = m.st.accent.Render(m.ic.Trophy), m.st.accent.Render(md.Name), m.st.text.Render(md.Desc)
		}
		return icon + " " + name + m.st.muted.Render(" · ") + desc
	}
	half := (len(s.Medals) + 1) / 2
	for i := 0; i < half; i++ {
		left := medal(i)
		line := left
		if j := i + half; j < len(s.Medals) {
			line += strings.Repeat(" ", max(2, 38-lipgloss.Width(left))) + medal(j)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + m.st.muted.Render(tr("qualquer tecla fecha", "any key closes")))
	return b.String()
}
