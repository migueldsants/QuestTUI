package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"questtui/internal/lang"
	"questtui/internal/store"
)

// Modo vigília: relógio grande, a quest rastreada e o que falta concluir.

var bigDigits = map[rune][5]string{
	'0': {"███", "█ █", "█ █", "█ █", "███"},
	'1': {" █ ", " █ ", " █ ", " █ ", " █ "},
	'2': {"███", "  █", "███", "█  ", "███"},
	'3': {"███", "  █", "███", "  █", "███"},
	'4': {"█ █", "█ █", "███", "  █", "  █"},
	'5': {"███", "█  ", "███", "  █", "███"},
	'6': {"███", "█  ", "███", "█ █", "███"},
	'7': {"███", "  █", "  █", "  █", "  █"},
	'8': {"███", "█ █", "███", "█ █", "███"},
	'9': {"███", "█ █", "███", "  █", "███"},
	':': {" ", "█", " ", "█", " "},
}

var (
	weekdays = [...]string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"}
	months   = [...]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}
)

// bigClock desenha HH:MM em blocos, uma string por linha; cada célula tem 2
// colunas de largura.
func bigClock(text string, colon bool) [5]string {
	var rows [5]string
	for i, r := range text {
		glyph, ok := bigDigits[r]
		if !ok {
			continue
		}
		for y := 0; y < 5; y++ {
			cell := glyph[y]
			if r == ':' && !colon {
				cell = " "
			}
			if i > 0 {
				rows[y] += "  "
			}
			for _, c := range cell {
				rows[y] += strings.Repeat(string(c), 2)
			}
		}
	}
	return rows
}

func (m *Model) viewVigil() string {
	layout := "15:04"
	if !m.cfg.Clock24h {
		layout = "03:04"
	}
	maxW := clamp(m.w-8, 20, 56)
	ic := m.ic
	fit := func(s string) string { return ansi.Truncate(s, maxW, "…") }

	// Relógio em degradê de cima para baixo, com uma onda de luz que o
	// atravessa na diagonal a cada poucos segundos.
	var parts []string
	wave := math.Mod(m.t*16, 90) - 12
	for y, row := range bigClock(m.now.Format(layout), m.now.Second()%2 == 0) {
		base := blend(m.th.Accent, m.th.Accent2, float64(y)/4)
		runes := []rune(row)
		cells := make([]string, 0, len(runes)/2)
		colors := make([]string, 0, len(runes)/2)
		for x := 0; x+1 < len(runes); x += 2 {
			diag := float64(x/2) + float64(y)*1.5
			cells = append(cells, string(runes[x:x+2]))
			colors = append(colors, blend(base, m.hi, step5(1-math.Abs(diag-wave)/3.5)))
		}
		parts = append(parts, colorize(cells, colors, lipgloss.NewStyle().Bold(true)))
	}
	date := fmt.Sprintf("%s, %d de %s", weekdays[m.now.Weekday()], m.now.Day(), months[m.now.Month()-1])
	if lang.Current == "en" {
		date = m.now.Format("Monday, January 2")
	}
	parts = append(parts, "", m.st.text.Render(date), "")

	tracked := m.tracked()
	var pending []store.Quest
	for _, q := range m.quests {
		if !q.Done && !q.Tracked {
			pending = append(pending, q)
		}
	}
	sortPending(pending)

	if m.focus.active {
		head := tr("FOCO", "FOCUS")
		if m.focus.onBreak {
			head = tr("PAUSA", "BREAK")
		}
		line := ic.Focus + " " + head + "  " + m.focusClock()
		if m.focus.paused {
			line += tr("  (pausado)", "  (paused)")
		}
		parts = append(parts,
			m.st.accent3.Bold(true).Render(fit(line)),
			m.bar(clamp(maxW-8, 10, 28), m.focusRatio()),
			"")
	}

	if tracked != nil {
		parts = append(parts, m.st.accent.Render(fit(ic.Tracked+" "+tracked.Title)))
		if i := tracked.CurrentStep(); i >= 0 {
			count := fmt.Sprintf("  %d/%d", tracked.StepsDone(), len(tracked.Steps))
			parts = append(parts,
				m.bar(clamp(maxW-len(count), 4, 28), m.barPos)+m.st.muted.Render(count),
				m.st.accent3.Render(fit(ic.StepNow+" "+tracked.Steps[i].Title)))
		}
		parts = append(parts, "")
	}

	switch {
	case len(pending) == 0 && tracked == nil:
		parts = append(parts, m.st.done.Render(ic.Done+tr(" Tudo concluído. Bom descanso.", " All done. Rest well.")))
	case len(pending) > 0:
		parts = append(parts, m.st.accent2.Bold(true).Render(fmt.Sprintf("%s %s  %d", ic.Pending, tr("FALTA CONCLUIR", "LEFT TO DO"), len(pending))))
		// Mostra só o que cabe na altura da tela.
		room := clamp(m.h-len(parts)-4, 1, 8)
		for i, q := range pending {
			if i == room {
				parts = append(parts, m.st.muted.Render(fmt.Sprintf(tr("+ %d outras", "+ %d more"), len(pending)-room)))
				break
			}
			mark := ic.Side
			if q.Kind == store.Main {
				mark = ic.Main
			}
			parts = append(parts, m.st.muted.Render(mark)+" "+m.st.text.Render(ansi.Truncate(q.Title, maxW-3, "…")))
		}
	}

	hint := tr("qualquer tecla volta ao diário", "any key returns to the journal")
	if tracked != nil {
		hint = tr("espaço avança a rastreada · f foco · outra tecla volta", "space advances the tracked quest · f focus · any other key returns")
	}
	parts = append(parts, "", m.st.muted.Render(fit(hint)))

	block := strings.Split(lipgloss.JoinVertical(lipgloss.Center, parts...), "\n")
	blockW := 0
	for _, l := range block {
		if w := lipgloss.Width(l); w > blockW {
			blockW = w
		}
	}
	x0 := clamp((m.w-blockW)/2, 0, m.w-blockW)
	y0 := clamp((m.h-len(block))/2, 0, m.h-len(block))

	sky := m.sky()
	rows := make([]string, m.h)
	for y := 0; y < m.h; y++ {
		i := y - y0
		if i < 0 || i >= len(block) {
			rows[y] = m.field(sky, y, 0, m.w)
			continue
		}
		line := block[i]
		pad := strings.Repeat(" ", blockW-lipgloss.Width(line))
		rows[y] = m.field(sky, y, 0, x0) + line + pad + m.field(sky, y, x0+blockW, m.w)
	}

	// Estrela cadente: risca o céu de tempos em tempos, por fora do bloco.
	if p := m.meteorPhase(); p < 1 {
		seed := int(m.t / meteorEvery)
		startX := float64(hash2(seed, 1) % uint32(m.w))
		startY := float64(hash2(seed, 2) % uint32(m.h/3+1))
		for k := 0; k < 7; k++ {
			x := int(startX + p*60 - float64(k)*2)
			y := int(startY + p*16 - float64(k)*0.55)
			inBlock := x >= x0-1 && x < x0+blockW+1 && y >= y0 && y < y0+len(block)
			if x < 0 || x >= m.w || y < 0 || y >= m.h || inBlock {
				continue
			}
			glyph := "·"
			switch {
			case k == 0:
				glyph = m.ic.Spark
			case k < 3:
				glyph = m.ic.BarFull
			case k < 5:
				glyph = m.ic.BarEmpty
			}
			color := blend(m.hi, m.th.BorderColor, float64(k)/7)
			rows[y] = putCell(rows[y], x, paint(lipgloss.NewStyle().Bold(true), color).Render(glyph))
		}
	}
	return strings.Join(rows, "\n")
}

const meteorEvery = 11.0 // segundos entre estrelas cadentes

// meteorPhase é o avanço (0 a 1) da estrela cadente; acima de 1 não há
// nenhuma no céu.
func (m *Model) meteorPhase() float64 {
	return math.Mod(m.t, meteorEvery) / 1.1
}
