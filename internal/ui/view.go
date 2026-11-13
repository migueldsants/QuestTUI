package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"questtui/internal/store"
)

func (m *Model) View() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	var out string
	switch m.mode {
	case modeVigil:
		out = m.viewVigil()
	case modeForm:
		title := tr("NOVA QUEST", "NEW QUEST")
		if m.editingID != 0 {
			title = tr("EDITAR QUEST", "EDIT QUEST")
		}
		hint := m.st.muted.Render(tr("enter ou tab avança · shift+tab volta · esc cancela", "enter or tab next · shift+tab back · esc cancel"))
		out = m.modal(m.ic.Journal, title, m.form.View()+"\n"+hint)
	case modeConfirm:
		out = m.modal(m.ic.Late, tr("APAGAR QUEST", "DELETE QUEST"), m.viewConfirm())
	case modeThemes:
		out = m.modal(m.ic.Theme, tr("TEMAS", "THEMES"), m.viewThemes())
	case modeHelp:
		out = m.modal(m.ic.Journal, tr("ATALHOS", "SHORTCUTS"), m.viewHelp())
	case modeStats:
		out = m.modal(m.ic.Trophy, tr("CONQUISTAS", "ACHIEVEMENTS"), m.viewStats())
	case modeNotes:
		out = m.viewNotes()
	default:
		out = m.viewMain()
	}
	// As partículas ficam por baixo do aviso, para não cobrirem o texto dele.
	out = m.overlaySparks(out)
	if m.banner.active {
		out = m.overlayBanner(out)
	}
	return m.applyWipe(out)
}

// --- tela principal ---

func (m *Model) viewMain() string {
	bodyH := m.h - 2 // cabeçalho e rodapé
	if bodyH < 3 {
		return m.header()
	}

	var body string
	if m.w < 70 {
		// Terminal estreito: um painel por vez.
		if m.focusSteps {
			body = m.pane(m.detail(m.w-4), m.w, bodyH, true)
		} else {
			body = m.pane(m.list(m.w-4, bodyH-2), m.w, bodyH, true)
		}
	} else {
		leftW := clamp(m.w*38/100, 30, 48)
		rightW := m.w - leftW
		left := m.pane(m.list(leftW-4, bodyH-2), leftW, bodyH, !m.focusSteps)
		right := m.pane(m.detail(rightW-4), rightW, bodyH, m.focusSteps)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}
	return m.header() + "\n" + body + "\n" + m.footer()
}

func (m *Model) header() string {
	pending, done := 0, 0
	for _, q := range m.quests {
		if q.Done {
			done++
		} else {
			pending++
		}
	}
	// Um brilho atravessa o título a cada poucos segundos.
	sweep := -1.0
	if phase := math.Mod(m.t, 8); phase < 1.6 {
		sweep = phase/1.6*1.4 - 0.2
	}
	left := " " + shimmer(m.ic.Journal+" "+tr("DIÁRIO DE QUESTS", "QUEST JOURNAL"), m.th.Accent, m.hi, sweep, lipgloss.NewStyle().Bold(true))

	sep := m.st.muted.Render(" · ")
	right := m.st.accent2.Render(m.ic.Pending) + m.st.muted.Render(" "+plural(pending, tr("pendente", "pending"), tr("pendentes", "pending"))) + sep +
		m.st.done.Render(m.ic.Done) + m.st.muted.Render(" "+plural(done, tr("concluída", "completed"), tr("concluídas", "completed"))) + sep
	if label := m.focusLabel(); label != "" {
		right += m.st.accent3.Render(m.ic.Focus+" "+label) + sep
	}
	right += m.st.accent2.Render(m.ic.Clock) + m.st.muted.Render(" "+m.clockText()+" ")

	gap := m.w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return ansi.Truncate(left, m.w, "")
	}
	return left + strings.Repeat(" ", gap) + right
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (m *Model) clockText() string {
	if m.cfg.Clock24h {
		return m.now.Format("15:04")
	}
	return m.now.Format("03:04 PM")
}

func (m *Model) footer() string {
	if m.mode == modeInput {
		label := m.ic.Steps + tr(" Nova etapa: ", " New step: ")
		hint := tr("  enter adiciona · esc termina", "  enter adds · esc finishes")
		switch m.inputKind {
		case inputFilter:
			label = m.ic.Filter + tr(" Filtrar: ", " Filter: ")
			hint = tr("  enter mantém · esc limpa", "  enter keeps · esc clears")
		case inputStepEdit:
			label = m.ic.Notes + tr(" Editar etapa: ", " Edit step: ")
			hint = tr("  enter salva · esc cancela", "  enter saves · esc cancels")
		}
		return " " + m.st.accent.Render(label) + m.input.View() + m.st.muted.Render(hint)
	}
	if m.err != nil {
		return " " + m.st.danger.Render(ansi.Truncate(m.ic.Late+tr(" erro: ", " error: ")+m.err.Error(), m.w-2, "…"))
	}
	if m.note != "" && m.now.Before(m.noteUntil) {
		return " " + m.st.accent.Render(ansi.Truncate(m.ic.Spark+" "+m.note, m.w-2, "…"))
	}
	help := tr(
		"espaço avança · n nova · e editar · o notas · a etapas · r rastrear · p prioridade · f foco · s conquistas · t temas · ^L english · ? ajuda",
		"space advance · n new · e edit · o notes · a steps · r track · p priority · f focus · s achievements · t themes · ^L português · ? help")
	if m.doneTab {
		help = tr(
			"espaço reabre · e editar · d apaga · c volta para as ativas · / filtrar · ? ajuda",
			"space reopens · e edit · d delete · c back to active · / filter · ? help")
	}
	if m.focusSteps {
		help = tr(
			"espaço marca · e renomeia · shift ↑↓ move · a nova etapa · d apaga · tab volta para a lista",
			"space toggles · e renames · shift ↑↓ moves · a new step · d delete · tab back to the list")
	}
	if m.filter != "" {
		help = m.ic.Filter + " " + m.filter + tr(" (esc limpa) · ", " (esc clears) · ") + help
	}
	return " " + m.st.muted.Render(ansi.Truncate(help, m.w-2, "…"))
}

// pane desenha um painel com borda; outerW e outerH incluem a borda.
func (m *Model) pane(content string, outerW, outerH int, focused bool) string {
	innerH := outerH - 2
	lines := strings.Split(content, "\n")
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	body := strings.Split(lipgloss.NewStyle().
		Padding(0, 1).
		Width(outerW-2).
		Height(innerH).
		Render(strings.Join(lines, "\n")), "\n")
	if len(body) > innerH {
		body = body[:innerH]
	}

	// A borda é desenhada célula a célula: no painel em foco, um cometa de
	// luz dá a volta nela. As células são numeradas no sentido horário.
	base := m.th.BorderColor
	if focused {
		base = m.th.Accent
	}
	perimeter := 2*outerW + 2*innerH
	head := math.Mod(m.t*float64(perimeter)/6, float64(perimeter)) // uma volta a cada 6s
	color := func(p int) string {
		if !focused {
			return base
		}
		behind := math.Mod(head-float64(p)+float64(perimeter), float64(perimeter))
		return blend(base, m.hi, step5(1-behind/14))
	}
	bd := m.st.border
	st := lipgloss.NewStyle()

	top := make([]string, outerW)
	bottom := make([]string, outerW)
	topColors := make([]string, outerW)
	bottomColors := make([]string, outerW)
	for i := 0; i < outerW; i++ {
		top[i], bottom[i] = bd.Top, bd.Bottom
		topColors[i] = color(i)
		bottomColors[i] = color(outerW + innerH + (outerW - 1 - i))
	}
	top[0], top[outerW-1] = bd.TopLeft, bd.TopRight
	bottom[0], bottom[outerW-1] = bd.BottomLeft, bd.BottomRight

	var b strings.Builder
	b.WriteString(colorize(top, topColors, st))
	for r, line := range body {
		left := color(2*outerW + innerH + (innerH - 1 - r))
		right := color(outerW + r)
		b.WriteString("\n" + paint(st, left).Render(bd.Left) + line + paint(st, right).Render(bd.Right))
	}
	b.WriteString("\n" + colorize(bottom, bottomColors, st))
	return b.String()
}

// section é o título de um grupo, seguido de um filete até a borda.
func (m *Model) section(icon, name string, width int) string {
	head := icon + " " + name
	out := m.st.accent2.Bold(true).Render(head)
	if rule := width - lipgloss.Width(head) - 1; rule > 0 {
		out += " " + lipgloss.NewStyle().Foreground(m.st.borderColor).Render(strings.Repeat(m.ic.Rule, rule))
	}
	return out
}

// list monta a coluna de quests já recortada em volta do cursor.
func (m *Model) list(width, height int) string {
	tabs := m.tabs(width) + "\n"
	height -= 2 // a barra de abas e a linha em branco abaixo dela
	if len(m.visible) == 0 {
		empty := tr("Nenhuma quest por aqui.\n\nPressione n para começar uma.", "No quests here.\n\nPress n to start one.")
		switch {
		case m.filter != "":
			empty = tr("Nenhuma quest com esse filtro.", "No quests match this filter.")
		case m.doneTab:
			empty = tr("Nenhuma quest concluída ainda.", "No completed quests yet.")
		}
		return tabs + "\n" + m.st.muted.Width(width).Render(empty)
	}

	var lines []string
	cursorLine := 0
	lastGroup := ""
	for i, q := range m.visible {
		group, icon := tr("SECUNDÁRIAS", "SIDE QUESTS"), m.ic.Side
		switch {
		case q.Done:
			// Na aba Concluídas os grupos são os dias de conclusão.
			group, icon = m.dayLabel(q.DoneAt), m.ic.Due
		case q.Kind == store.Main:
			group, icon = tr("PRINCIPAIS", "MAIN QUESTS"), m.ic.Main
		}
		if group != lastGroup {
			if lastGroup != "" {
				lines = append(lines, "")
			}
			lines = append(lines, m.section(icon, group, width))
			lastGroup = group
		}
		if i == m.cursor {
			cursorLine = len(lines)
		}
		lines = append(lines, m.row(q, i == m.cursor, width)...)
	}

	if len(lines) > height && height > 0 {
		start := clamp(cursorLine-height/2, 0, len(lines)-height)
		lines = lines[start : start+height]
	}
	return tabs + "\n" + strings.Join(lines, "\n")
}

// tabs desenha as abas Ativas e Concluídas com a contagem de cada uma.
func (m *Model) tabs(width int) string {
	active, done := 0, 0
	for _, q := range m.quests {
		if q.Done {
			done++
		} else {
			active++
		}
	}
	tab := func(icon, name string, n int, on bool) string {
		label := fmt.Sprintf("%s %s %d", icon, name, n)
		if on {
			return m.st.accent.Underline(true).Render(label)
		}
		return m.st.muted.Render(label)
	}
	out := tab(m.ic.Pending, tr("Ativas", "Active"), active, !m.doneTab) + "   " +
		tab(m.ic.Done, tr("Concluídas", "Completed"), done, m.doneTab)
	return ansi.Truncate(out, width, "")
}

// dayLabel nomeia o dia de conclusão: HOJE, ONTEM ou a data.
func (m *Model) dayLabel(t time.Time) string {
	t = t.Local()
	switch int(math.Round(store.DateOnly(m.now).Sub(store.DateOnly(t)).Hours() / 24)) {
	case 0:
		return tr("HOJE", "TODAY")
	case 1:
		return tr("ONTEM", "YESTERDAY")
	}
	return t.Format("02/01/2006")
}

// row desenha uma quest na lista. A selecionada e a rastreada ganham uma
// segunda linha com o objetivo atual.
func (m *Model) row(q store.Quest, selected bool, width int) []string {
	ic := m.ic
	// Na linha selecionada todos os trechos levam a cor de fundo.
	sel := func(s lipgloss.Style) lipgloss.Style {
		if selected && m.th.Selection != "" {
			return s.Background(lipgloss.Color(m.th.Selection))
		}
		return s
	}
	plain := sel(lipgloss.NewStyle())

	cur := strings.Repeat(" ", lipgloss.Width(ic.Cursor)+1)
	if selected {
		cur = ic.Cursor + " "
	}
	mark, markStyle := ic.Todo, m.st.muted
	switch {
	case q.Done:
		mark, markStyle = ic.Done, m.st.done
	case q.Tracked:
		mark, markStyle = ic.Tracked, m.st.accent
	}
	prefix := ""
	if q.Priority && !q.Done {
		prefix = ic.Priority + " "
	}
	var rightParts []string
	if len(q.Steps) > 0 {
		rightParts = append(rightParts, fmt.Sprintf("%d/%d", q.StepsDone(), len(q.Steps)))
	}
	right := strings.Join(rightParts, " ")

	titleW := width - lipgloss.Width(cur) - lipgloss.Width(mark) - 1 - lipgloss.Width(prefix)
	if right != "" {
		titleW -= lipgloss.Width(right) + 1
	}
	if titleW < 4 {
		titleW = 4
	}
	title := ansi.Truncate(q.Title, titleW, "…")
	pad := strings.Repeat(" ", titleW-lipgloss.Width(title))

	style := m.st.text
	switch {
	case q.Done:
		style = m.st.muted.Strikethrough(true)
	case selected && !m.focusSteps:
		style = m.st.accent
	case selected:
		style = m.st.text.Bold(true)
	}
	// O cursor pulsa entre a cor de destaque e o brilho.
	curStyle := paint(lipgloss.NewStyle().Bold(true), blend(m.th.Accent, m.hi, 0.5+0.5*math.Sin(m.t*4)))
	if m.focusSteps {
		curStyle = m.st.muted
	}

	line := sel(curStyle).Render(cur) + sel(markStyle).Render(mark) + plain.Render(" ")
	if prefix != "" {
		line += sel(m.st.danger).Render(prefix)
	}
	line += sel(style).Render(title) + plain.Render(pad)
	if right != "" {
		line += plain.Render(" ") + sel(m.st.muted).Render(right)
	}
	lines := []string{line}

	if i := q.CurrentStep(); i >= 0 && !q.Done && (selected || q.Tracked) {
		indent := strings.Repeat(" ", lipgloss.Width(cur)+lipgloss.Width(mark)+1)
		text := ansi.Truncate(ic.StepNow+" "+q.Steps[i].Title, width-lipgloss.Width(indent), "…")
		fill := strings.Repeat(" ", max(0, width-lipgloss.Width(indent)-lipgloss.Width(text)))
		lines = append(lines, plain.Render(indent)+sel(m.st.accent3).Render(text)+plain.Render(fill))
	}
	return lines
}

// bar desenha uma barra de progresso em degradê; ratio vai de 0 a 1.
func (m *Model) bar(width int, ratio float64) string {
	ratio = math.Max(0, math.Min(1, ratio))
	full := int(math.Round(ratio * float64(width)))
	// Um reflexo corre pela parte cheia da barra a cada poucos segundos.
	glint := math.Mod(m.t*0.45, 1.8) - 0.3
	cells := make([]string, full)
	colors := make([]string, full)
	for i := range cells {
		t := 0.0
		if width > 1 {
			t = float64(i) / float64(width-1)
		}
		cells[i] = m.ic.BarFull
		colors[i] = blend(blend(m.th.Accent2, m.th.Accent, t), m.hi, step5(1-math.Abs(t-glint)/0.12))
	}
	return colorize(cells, colors, lipgloss.NewStyle().Bold(true)) +
		lipgloss.NewStyle().Foreground(m.st.borderColor).Render(strings.Repeat(m.ic.BarEmpty, width-full))
}

// detail monta o painel da quest selecionada.
func (m *Model) detail(width int) string {
	q := m.selected()
	if q == nil {
		return ""
	}
	ic := m.ic
	var b strings.Builder

	b.WriteString(m.st.accent.Width(width).Render(q.Title))
	b.WriteString("\n")

	kind := ic.Side + tr(" Secundária", " Side quest")
	if q.Kind == store.Main {
		kind = ic.Main + tr(" Principal", " Main quest")
	}
	meta := []string{m.st.accent2.Render(kind)}
	if q.Priority && !q.Done {
		meta = append(meta, m.st.danger.Render(ic.Priority+tr(" prioridade alta", " high priority")))
	}
	if q.Done {
		meta = append(meta, m.st.done.Render(ic.Done+tr(" concluída em ", " completed on ")+q.DoneAt.Local().Format("02/01")))
	} else if text, late := m.dueText(*q); text != "" {
		if late {
			meta = append(meta, m.st.danger.Render(ic.Late+" "+text))
		} else {
			meta = append(meta, m.st.accent3.Render(ic.Due+" "+text))
		}
	}
	if q.Tracked {
		meta = append(meta, m.st.accent.Render(ic.Tracked+tr(" rastreada", " tracked")))
	}
	b.WriteString(lipgloss.NewStyle().Width(width).Render(strings.Join(meta, m.st.muted.Render(" · "))))
	b.WriteString("\n\n")

	if len(q.Steps) == 0 {
		b.WriteString(m.st.muted.Render(tr(
			"Sem etapas. Pressione a para dividir a quest em etapas.",
			"No steps. Press a to split the quest into steps.")))
		b.WriteString("\n")
	} else {
		count := fmt.Sprintf("  %d/%d", q.StepsDone(), len(q.Steps))
		b.WriteString(m.bar(clamp(width-len(count), 4, 40), m.barPos) + m.st.muted.Render(count))
		b.WriteString("\n\n")
		b.WriteString(m.section(ic.Steps, tr("JORNADA", "JOURNEY"), width))
		b.WriteString("\n")
		b.WriteString(m.steps(*q, width))
	}

	b.WriteString("\n")
	b.WriteString(m.section(ic.Notes, tr("NOTAS", "NOTES"), width))
	b.WriteString("\n")
	if q.Notes == "" {
		b.WriteString(m.st.muted.Render(tr("Sem notas. Pressione o para escrever.", "No notes. Press o to write some.")))
	} else {
		b.WriteString(m.notes(*q, width))
	}
	return b.String()
}

// steps lista as etapas como uma jornada: concluídas, o objetivo atual em
// destaque e as próximas esmaecidas.
func (m *Model) steps(q store.Quest, width int) string {
	ic := m.ic
	current := q.CurrentStep()
	// O marcador do objetivo atual pulsa devagar.
	pulse := blend(m.th.Accent, m.hi, 0.5+0.5*math.Sin(m.t*3))

	var b strings.Builder
	for i, st := range q.Steps {
		cur := strings.Repeat(" ", lipgloss.Width(ic.Cursor)+1)
		if m.focusSteps && i == m.stepCursor {
			cur = ic.Cursor + " "
		}
		mark, markStyle, style, tag := ic.StepNext, m.st.muted, m.st.muted, ""
		switch {
		case st.Done:
			mark, markStyle, style = ic.StepDone, m.st.done, m.st.muted.Strikethrough(true)
		case i == current:
			mark, markStyle, style = ic.StepNow, paint(lipgloss.NewStyle().Bold(true), pulse), m.st.accent
			tag = tr("objetivo atual", "current objective")
		}

		titleW := width - lipgloss.Width(cur) - lipgloss.Width(mark) - 1
		if tag != "" && titleW-len(tag)-2 < lipgloss.Width(st.Title) {
			tag = "" // não cabe ao lado do título
		}
		title := ansi.Truncate(st.Title, titleW, "…")
		line := m.st.accent.Render(cur) + markStyle.Render(mark) + " " + style.Render(title)
		if tag != "" {
			gap := titleW - lipgloss.Width(title) - len(tag)
			line += strings.Repeat(" ", gap) + m.st.accent3.Render(tag)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// dueInfo diz quantos dias faltam para o prazo (negativo se já passou) e se a
// quest está atrasada; ok é false quando não há prazo.
func (m *Model) dueInfo(q store.Quest) (days int, late, ok bool) {
	t, hasTime, ok := q.DueTime()
	if !ok {
		return 0, false, false
	}
	days = int(math.Round(store.DateOnly(t).Sub(store.DateOnly(m.now)).Hours() / 24))
	late = days < 0 || (hasTime && days == 0 && m.now.After(t))
	return days, late, true
}

// dueText descreve o prazo em relação a agora; late indica atraso.
func (m *Model) dueText(q store.Quest) (text string, late bool) {
	t, hasTime, ok := q.DueTime()
	if !ok {
		return "", false
	}
	days, late, _ := m.dueInfo(q)
	clock := ""
	if hasTime {
		clock = " " + t.Format("15:04")
	}
	date := t.Format("02/01")
	switch {
	case days < -1:
		return fmt.Sprintf(tr("%s · atrasada %d dias", "%s · %d days late"), date, -days), true
	case days == -1:
		return fmt.Sprintf(tr("%s · atrasada 1 dia", "%s · 1 day late"), date), true
	case days == 0 && late:
		return tr("hoje", "today") + clock + tr(" · atrasada", " · late"), true
	case days == 0:
		return tr("hoje", "today") + clock, false
	case days == 1:
		return tr("amanhã", "tomorrow") + clock, false
	default:
		return fmt.Sprintf(tr("%s · em %d dias", "%s · in %d days"), date+clock, days), false
	}
}

// notes renderiza o markdown das notas, com cache por quest e largura.
func (m *Model) notes(q store.Quest, width int) string {
	key := fmt.Sprintf("%d|%d|%s", q.ID, width, q.Notes)
	if key == m.notesKey {
		return m.notesOut
	}
	out := q.Notes
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(m.glamourStyle()), glamour.WithWordWrap(width-2))
	if err == nil {
		if rendered, err := r.Render(q.Notes); err == nil {
			out = strings.Trim(rendered, "\n")
		}
	}
	m.notesKey, m.notesOut = key, out
	return out
}

// --- janelas sobrepostas ---

func (m *Model) modal(icon, title, body string) string {
	box := lipgloss.NewStyle().
		Border(m.st.border).
		BorderForeground(m.st.accentColor).
		Padding(1, 3).
		Render(m.st.accent.Render(icon+" "+title) + "\n\n" + body)
	// Numa tela baixa demais, mostra o começo da janela em vez de cortar o topo.
	if lines := strings.Split(box, "\n"); len(lines) > m.h {
		box = strings.Join(lines[:m.h], "\n")
	}
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) viewConfirm() string {
	q := m.selected()
	if q == nil {
		return ""
	}
	name := ansi.Truncate(q.Title, clamp(m.w-16, 10, 50), "…")
	return m.st.text.Render(fmt.Sprintf(tr("Apagar \"%s\" e suas etapas?", "Delete \"%s\" and its steps?"), name)) + "\n\n" +
		m.st.muted.Render(tr("s confirma · qualquer outra tecla cancela", "y confirms · any other key cancels"))
}

func (m *Model) viewThemes() string {
	var b strings.Builder
	for i, t := range m.themes {
		swatch := fg(t.Accent).Render("██") + fg(t.Accent2).Render("██")
		if t.Accent3 != "" {
			swatch += fg(t.Accent3).Render("██")
		}
		swatch += fg(t.Done).Render("██") + fg(t.Danger).Render("██") + fg(t.Muted).Render("██")
		name := fmt.Sprintf("%-14s", t.Name)
		if t.Light() {
			swatch += m.st.muted.Render(tr("  claro", "  light"))
		}
		if i == m.themeCursor {
			b.WriteString(m.st.accent.Render(m.ic.Cursor+" "+name) + swatch)
		} else {
			b.WriteString(m.st.text.Render(strings.Repeat(" ", lipgloss.Width(m.ic.Cursor)+1)+name) + swatch)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.st.muted.Render(tr("↑↓ pré-visualiza · enter aplica · esc cancela", "↑↓ preview · enter apply · esc cancel")))
	return b.String()
}

func (m *Model) viewHelp() string {
	keys := [][2]string{
		{"↑ ↓ / j k", tr("mover", "move")},
		{"shift ↑ ↓ / K J", tr("mudar a quest (ou a etapa) de lugar", "move the quest (or the step) up or down")},
		{tr("espaço", "space"), tr("avançar: conclui o objetivo atual, depois a quest", "advance: completes the current objective, then the quest")},
		{"u", tr("voltar uma etapa (ou reabrir a quest)", "go back one step (or reopen the quest)")},
		{"tab / → ←", tr("alternar entre lista e etapas", "switch between list and steps")},
		{"n", tr("nova quest", "new quest")},
		{"e", tr("editar a quest; nas etapas, renomear a etapa", "edit the quest; in the steps, rename the step")},
		{"a", tr("adicionar etapas", "add steps")},
		{"o", tr("notas: editor de markdown (prévia, modo vim)", "notes: markdown editor (preview, vim mode)")},
		{"d", tr("apagar quest ou etapa", "delete quest or step")},
		{"r", tr("rastrear quest (destaque na vigília)", "track quest (featured in vigil mode)")},
		{"p", tr("prioridade alta", "high priority")},
		{"f / F", tr("modo foco: iniciar ou pausar / parar", "focus mode: start or pause / stop")},
		{"s", tr("conquistas", "achievements")},
		{"c / 1 2", tr("alternar entre as abas Ativas e Concluídas", "switch between the Active and Completed tabs")},
		{"/", tr("filtrar pelo título", "filter by title")},
		{"t", tr("trocar tema", "change theme")},
		{"i", tr("trocar ícones (nerd, unicode, ascii)", "change icons (nerd, unicode, ascii)")},
		{"m", tr("ligar ou desligar o som", "turn sound on or off")},
		{"^L", tr("trocar idioma (português, inglês)", "change language (Portuguese, English)")},
		{"z", tr("modo vigília (relógio)", "vigil mode (clock)")},
		{"q", tr("sair", "quit")},
	}
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(m.st.accent.Render(fmt.Sprintf("%-16s", k[0])) + m.st.text.Render(k[1]) + "\n")
	}
	b.WriteString("\n" + m.st.muted.Render(tr("qualquer tecla fecha", "any key closes")))
	return b.String()
}
