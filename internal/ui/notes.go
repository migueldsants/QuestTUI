package ui

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"questtui/internal/editor"
	"questtui/internal/store"
)

// Tela de notas: o editor próprio, com prévia do markdown ao lado, tela
// cheia (^F), painel de atalhos (^G), modo vim (^T) e a opção de abrir no
// vim ou em outro editor externo (^E).

type notesState struct {
	ed      *editor.Editor
	questID int64
	title   string
	full    bool // expandido para a tela inteira
	preview bool
	help    bool

	prevKey, prevOut string // cache da prévia
}

// externalDoneMsg chega quando o editor externo fecha.
type externalDoneMsg struct {
	path string
	err  error
}

func (m *Model) editorTheme() editor.Theme {
	fg := m.th.Fg
	if fg == "" {
		fg = "7"
	}
	return editor.Theme{
		Fg: fg, Muted: m.th.Muted, Accent: m.th.Accent, Accent2: m.th.Accent2, Accent3: m.accent3(),
		Done: m.th.Done, Danger: m.th.Danger, Border: m.th.BorderColor,
		CursorLine: m.th.Selection,
		Selection:  m.selectionColor(),
	}
}

// selectionColor é o fundo da seleção visual: o destaque escurecido nos
// temas escuros e clareado nos claros.
func (m *Model) selectionColor() string {
	if m.th.Light() {
		return blend(m.th.Accent2, "#ffffff", 0.7)
	}
	return blend(m.th.Accent2, "#000000", 0.55)
}

func (m *Model) openNotes(q store.Quest) {
	ed := editor.New(q.Notes, m.cfg.EditorMode == "vim")
	ed.SetTheme(m.editorTheme())
	m.notesEd = notesState{ed: ed, questID: q.ID, title: q.Title, preview: m.cfg.NotesPreview}
	m.mode = modeNotes
	m.startWipe()
}

// saveNotes grava as notas, se mudaram.
func (m *Model) saveNotes() bool {
	n := &m.notesEd
	if !n.ed.Dirty() {
		return true
	}
	text := strings.TrimRight(n.ed.Text(), " \n")
	if err := m.store.SetNotes(n.questID, text); err != nil {
		m.err = err
		return false
	}
	n.ed.MarkSaved()
	m.notesKey = ""
	m.reload()
	return true
}

func (m *Model) closeNotes(save bool) {
	if save && !m.saveNotes() {
		return
	}
	if save {
		m.flash(tr("notas salvas", "notes saved"))
	}
	m.err = nil
	m.mode = modeList
	m.startWipe()
}

// externalEditor escolhe o editor externo: o do config, $VISUAL, $EDITOR, ou
// o primeiro instalado entre nvim, vim, nano e vi.
func (m *Model) externalEditor() string {
	for _, e := range []string{m.cfg.ExternalEditor, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if strings.TrimSpace(e) != "" {
			return e
		}
	}
	for _, e := range []string{"nvim", "vim", "nano", "vi"} {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	return ""
}

// externalEdit abre as notas num editor externo, num arquivo temporário; ao
// fechar, o texto volta para o editor e é gravado.
func (m *Model) externalEdit() tea.Cmd {
	name := m.externalEditor()
	if name == "" {
		m.err = errors.New(tr("nenhum editor externo encontrado (defina external_editor no config.toml)",
			"no external editor found (set external_editor in config.toml)"))
		return nil
	}
	f, err := os.CreateTemp("", "questtui-*.md")
	if err != nil {
		m.err = err
		return nil
	}
	_, err = f.WriteString(m.notesEd.ed.Text() + "\n")
	f.Close()
	if err != nil {
		m.err = err
		return nil
	}
	args := strings.Fields(name)
	cmd := exec.Command(args[0], append(args[1:], f.Name())...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return externalDoneMsg{f.Name(), err} })
}

func (m *Model) updateNotes(msg tea.Msg) (tea.Model, tea.Cmd) {
	n := &m.notesEd
	switch msg := msg.(type) {
	case externalDoneMsg:
		data, err := os.ReadFile(msg.path)
		os.Remove(msg.path)
		if msg.err != nil {
			m.err = fmt.Errorf("%s: %w", m.externalEditor(), msg.err)
			return m, nil
		}
		if err != nil {
			m.err = err
			return m, nil
		}
		text := strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if text != n.ed.Text() {
			n.ed.ReplaceText(text)
			if m.saveNotes() {
				m.flash(tr("notas atualizadas pelo editor externo", "notes updated from the external editor"))
			}
		}
		return m, nil

	case tea.KeyMsg:
		m.err = nil
		switch msg.String() {
		case "ctrl+s":
			if m.saveNotes() {
				m.flash(tr("notas salvas", "notes saved"))
			}
			return m, nil
		case "ctrl+q", "ctrl+c":
			m.closeNotes(true)
			return m, nil
		case "ctrl+p":
			n.preview = !n.preview
			m.cfg.NotesPreview = n.preview
			m.err = m.cfg.Save()
			return m, nil
		case "ctrl+f":
			n.full = !n.full
			return m, nil
		case "ctrl+g":
			n.help = !n.help
			return m, nil
		case "ctrl+t":
			vim := !n.ed.Vim()
			n.ed.SetVim(vim)
			m.cfg.EditorMode = map[bool]string{true: "vim", false: "padrao"}[vim]
			m.err = m.cfg.Save()
			m.flash(map[bool]string{true: tr("modo vim", "vim mode"), false: tr("modo padrão", "standard mode")}[vim])
			return m, nil
		case "ctrl+e":
			return m, m.externalEdit()
		case "esc":
			if !n.ed.Vim() {
				m.closeNotes(true)
				return m, nil
			}
		}
		switch n.ed.Update(msg) {
		case editor.Save:
			if m.saveNotes() {
				m.flash(tr("notas salvas", "notes saved"))
			}
		case editor.SaveClose:
			m.closeNotes(true)
		case editor.DiscardClose:
			m.closeNotes(false)
		}
	}
	return m, nil
}

// --- desenho ---

func (m *Model) viewNotes() string {
	n := &m.notesEd
	bw, bh := m.w, m.h
	if !n.full {
		bw = min(m.w, clamp(m.w*88/100, 40, 128))
		bh = min(m.h, clamp(m.h*88/100, 12, 40))
	}
	iw, ih := bw-4, bh-2 // dentro da moldura e do recuo
	if iw < 20 || ih < 5 {
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center,
			m.st.muted.Render(tr("tela pequena demais para o editor", "screen too small for the editor")))
	}
	bodyH := ih - 3 // título, estado e atalhos

	// Corpo: editor e, ao lado, a prévia ou os atalhos.
	side := ""
	if n.help {
		side = "help"
	} else if n.preview {
		side = "preview"
	}
	edW, sideW := iw, 0
	if side != "" {
		if iw >= 64 {
			edW = iw * 55 / 100
			sideW = iw - edW - 3
		} else if side == "help" {
			edW = 0
			sideW = iw
		} else {
			side = ""
		}
	}
	var edLines, sideLines []string
	if edW > 0 {
		edLines = strings.Split(n.ed.View(edW, bodyH), "\n")
	}
	switch side {
	case "preview":
		sideLines = m.previewLines(sideW, bodyH)
	case "help":
		sideLines = m.notesHelp(sideW, bodyH)
	}
	sep := lipgloss.NewStyle().Foreground(m.st.borderColor).Render(" │ ")
	body := make([]string, bodyH)
	for i := range body {
		var b strings.Builder
		if edW > 0 {
			b.WriteString(edLines[i])
		}
		if sideW > 0 {
			if edW > 0 {
				b.WriteString(sep)
			}
			line := ""
			if i < len(sideLines) {
				line = ansi.Truncate(sideLines[i], sideW, "")
			}
			b.WriteString(line + strings.Repeat(" ", max(0, sideW-lipgloss.Width(line))))
		}
		body[i] = b.String()
	}

	content := strings.Join(append(append([]string{m.notesTitle(iw)}, body...), m.notesStatus(iw), m.notesHints(iw)), "\n")
	box := lipgloss.NewStyle().
		Border(m.st.border).
		BorderForeground(m.st.accentColor).
		Padding(0, 1).
		Width(bw - 2).
		Render(content)
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) notesTitle(w int) string {
	n := &m.notesEd
	left := m.st.accent.Render(m.ic.Notes+" "+tr("NOTAS", "NOTES")) + m.st.muted.Render(" · ") + m.st.text.Render(n.title)
	right := m.st.done.Render(m.ic.StepDone + tr(" salvo", " saved"))
	if n.ed.Dirty() {
		right = m.st.danger.Render("● " + tr("não salvo", "unsaved"))
	}
	expand := tr("^F expandir", "^F expand")
	if n.full {
		expand = tr("^F reduzir", "^F shrink")
	}
	right = m.st.muted.Render(expand+"  ") + right
	left = ansi.Truncate(left, max(0, w-lipgloss.Width(right)-1), "…")
	return left + strings.Repeat(" ", max(1, w-lipgloss.Width(left)-lipgloss.Width(right))) + right
}

func (m *Model) notesStatus(w int) string {
	s := m.notesEd.ed.Status()
	bg := m.th.Accent
	if s.Insert {
		bg = m.accent3()
	}
	badge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ink(bg))).
		Background(lipgloss.Color(bg)).Render(" " + s.Mode + " ")

	info := ""
	switch {
	case s.Cmdline != "":
		info = m.st.text.Render(s.Cmdline) + lipgloss.NewStyle().Background(lipgloss.Color(m.th.Accent)).Render(" ")
	case m.err != nil:
		info = m.st.danger.Render(m.err.Error())
	case s.Msg != "":
		info = m.st.danger.Render(s.Msg)
	case m.note != "" && m.now.Before(m.noteUntil):
		info = m.st.accent.Render(m.ic.Spark + " " + m.note)
	case s.Pending != "":
		info = m.st.muted.Render(s.Pending)
	}
	right := m.st.muted.Render(fmt.Sprintf(tr("ln %d/%d · col %d · %s", "ln %d/%d · col %d · %s"),
		s.Row, s.Lines, s.Col, plural(s.Words, tr("palavra", "word"), tr("palavras", "words"))))
	left := badge + " " + info
	left = ansi.Truncate(left, max(0, w-lipgloss.Width(right)-1), "…")
	return left + strings.Repeat(" ", max(1, w-lipgloss.Width(left)-lipgloss.Width(right))) + right
}

func (m *Model) notesHints(w int) string {
	hints := tr(
		"^S salvar · ^Q fechar · ^Z desfazer · ^K recortar · ^U colar · ^P prévia · ^T vim · ^E editor externo · ^G atalhos",
		"^S save · ^Q close · ^Z undo · ^K cut · ^U paste · ^P preview · ^T vim · ^E external editor · ^G shortcuts")
	if m.notesEd.ed.Vim() {
		hints = tr(
			":w salvar · :q fechar · :wq salvar e fechar · ^P prévia · ^T modo padrão · ^E vim externo · ^G atalhos",
			":w save · :q close · :wq save and close · ^P preview · ^T standard mode · ^E external vim · ^G shortcuts")
	}
	return m.st.muted.Render(ansi.Truncate(hints, w, "…"))
}

// previewLines desenha o markdown, rolando junto com o cursor do editor.
func (m *Model) previewLines(w, h int) []string {
	n := &m.notesEd
	text := n.ed.Text()
	key := fmt.Sprintf("%d|%s|%s", w, m.glamourStyle(), text)
	if key != n.prevKey {
		out := text
		if r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(m.glamourStyle()), glamour.WithWordWrap(max(10, w-2))); err == nil {
			if rendered, err := r.Render(text); err == nil {
				out = strings.Trim(rendered, "\n")
			}
		}
		n.prevKey, n.prevOut = key, out
	}
	head := m.section(m.ic.Journal, tr("PRÉVIA", "PREVIEW"), w)
	lines := strings.Split(n.prevOut, "\n")
	if strings.TrimSpace(text) == "" {
		lines = []string{"", m.st.muted.Render(tr("  (vazio)", "  (empty)"))}
	}
	room := h - 1
	off := 0
	if len(lines) > room {
		off = int(math.Round(n.ed.Progress() * float64(len(lines)-room)))
	}
	end := min(len(lines), off+room)
	return append([]string{head}, lines[off:end]...)
}

// notesHelp lista os atalhos do editor.
func (m *Model) notesHelp(w, h int) []string {
	group := func(title string, rows [][2]string) []string {
		out := []string{m.section(m.ic.Steps, title, w)}
		for _, r := range rows {
			out = append(out, m.st.accent.Render(fmt.Sprintf("%-10s", r[0]))+" "+m.st.text.Render(r[1]))
		}
		return append(out, "")
	}
	var lines []string
	lines = append(lines, group(tr("GERAL", "GENERAL"), [][2]string{
		{"^S", tr("salvar", "save")},
		{"^Q", tr("fechar (salva)", "close (saves)")},
		{"^P", tr("prévia do markdown", "markdown preview")},
		{"^F", tr("expandir / reduzir", "expand / shrink")},
		{"^T", tr("modo vim / modo padrão", "vim / standard mode")},
		{"^E", tr("abrir no vim (editor externo)", "open in vim (external editor)")},
		{"^G", tr("mostrar / esconder atalhos", "show / hide shortcuts")},
	})...)
	lines = append(lines, group("MARKDOWN", [][2]string{
		{"alt+b", tr("negrito", "bold")},
		{"alt+i", tr("itálico", "italic")},
		{"alt+s", tr("riscado", "strikethrough")},
		{"alt+c", tr("código", "code")},
		{"alt+k", "link"},
		{"alt+1 2 3", tr("título", "heading")},
		{"alt+l", tr("lista", "list")},
		{"alt+t", tr("tarefa: criar / marcar", "task: add / check")},
		{"alt+q", tr("citação", "quote")},
		{"enter", tr("continua listas e tarefas", "continues lists and tasks")},
	})...)
	if m.notesEd.ed.Vim() {
		lines = append(lines, group("VIM", [][2]string{
			{"i a I A o O", tr("inserir", "insert")},
			{"esc", tr("modo normal", "normal mode")},
			{"h j k l", tr("mover", "move")},
			{"w b e", tr("palavras", "words")},
			{"0 ^ $", tr("início / fim da linha", "line start / end")},
			{"gg G { }", tr("topo / fim / parágrafo", "top / end / paragraph")},
			{"d c y", tr("apagar / trocar / copiar + movimento", "delete / change / yank + motion")},
			{"dd cc yy", tr("a linha inteira", "the whole line")},
			{"x X D C", tr("apagar caractere / até o fim", "delete char / to the end")},
			{"p P", tr("colar depois / antes", "put after / before")},
			{"u ^R", tr("desfazer / refazer", "undo / redo")},
			{"v V", tr("seleção / seleção de linhas", "visual / visual line")},
			{"> <", tr("recuar", "indent")},
			{"J ~ r", tr("juntar / maiúscula / trocar", "join / case / replace")},
			{"/ n N", tr("buscar", "search")},
			{":w :q :wq", tr("salvar / fechar", "write / quit")},
			{":q!", tr("fechar sem salvar", "quit without saving")},
			{tr("3j 2dd", "3j 2dd"), tr("número repete o comando", "a count repeats it")},
		})...)
	} else {
		lines = append(lines, group(tr("EDIÇÃO", "EDITING"), [][2]string{
			{"^Z ^Y", tr("desfazer / refazer", "undo / redo")},
			{"^K", tr("recortar linha (repetir acumula)", "cut line (repeat to add)")},
			{"^U", tr("colar", "paste")},
			{"^W", tr("apagar palavra", "delete word")},
			{"tab", tr("recuar item / inserir espaços", "indent item / insert spaces")},
			{"^← ^→", tr("pular palavras", "jump words")},
			{"esc", tr("fechar (salva)", "close (saves)")},
		})...)
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}
