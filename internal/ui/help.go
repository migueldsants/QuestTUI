package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Tela de atalhos (? ou F1): todos os atalhos do app, em seções, com rolagem.
// As listas do editor também alimentam o painel ^G dentro das notas.

type keyGroup struct {
	title string
	keys  [][2]string
}

func (m *Model) journalKeys() [][2]string {
	return [][2]string{
		{"↑ ↓ / j k", tr("mover", "move")},
		{"shift ↑ ↓ / K J", tr("mudar a quest de lugar", "move the quest up or down")},
		{tr("espaço", "space"), tr("avançar: conclui o objetivo atual, depois a quest", "advance: complete the current objective, then the quest")},
		{"u", tr("voltar uma etapa (ou reabrir a quest)", "go back one step (or reopen the quest)")},
		{"tab / → ←", tr("entrar e sair das etapas", "move into and out of the steps")},
		{"n", tr("nova quest", "new quest")},
		{"e", tr("editar a quest", "edit the quest")},
		{"a", tr("adicionar etapas", "add steps")},
		{"o", tr("notas (editor de markdown)", "notes (markdown editor)")},
		{"d", tr("apagar", "delete")},
		{"r", tr("rastrear (destaque na vigília)", "track (featured in vigil mode)")},
		{"p", tr("prioridade alta", "high priority")},
		{"f / F", tr("foco: iniciar ou pausar / parar", "focus: start or pause / stop")},
		{"s", tr("conquistas", "achievements")},
		{"c / 1 2", tr("abas Ativas e Concluídas", "Active and Completed tabs")},
		{"/", tr("filtrar pelo título", "filter by title")},
		{"z", tr("modo vigília", "vigil mode")},
		{"t", tr("temas", "themes")},
		{"i", tr("ícones: Nerd Font, Unicode, ASCII", "icons: Nerd Font, Unicode, ASCII")},
		{"m", tr("som ligado ou desligado", "sound on or off")},
		{"^L", tr("idioma: português ou inglês", "language: Portuguese or English")},
		{"? / F1", tr("esta tela de atalhos", "this shortcuts screen")},
		{"q", tr("sair", "quit")},
	}
}

func (m *Model) stepKeys() [][2]string {
	return [][2]string{
		{tr("espaço", "space"), tr("marcar ou desmarcar a etapa", "check or uncheck the step")},
		{"e", tr("renomear a etapa", "rename the step")},
		{"shift ↑ ↓ / K J", tr("mudar a etapa de lugar", "move the step up or down")},
		{"a / d", tr("nova etapa / apagar", "new step / delete")},
		{"tab / ←", tr("voltar para a lista", "back to the list")},
	}
}

func (m *Model) formKeys() [][2]string {
	return [][2]string{
		{"enter / tab", tr("próximo campo (no último, salva)", "next field (saves on the last one)")},
		{"↑ ↓ / shift+tab", tr("andar entre os campos", "move between fields")},
		{"← →", tr("escolher tipo e prioridade", "pick the type and priority")},
		{"esc", tr("cancelar", "cancel")},
	}
}

func (m *Model) vigilKeys() [][2]string {
	return [][2]string{
		{tr("espaço", "space"), tr("avançar a quest rastreada", "advance the tracked quest")},
		{"f", tr("foco: iniciar ou pausar", "focus: start or pause")},
		{"? / F1", tr("atalhos", "shortcuts")},
		{tr("outra tecla", "any other key"), tr("voltar ao diário", "back to the journal")},
	}
}

func (m *Model) editorGeneralKeys() [][2]string {
	return [][2]string{
		{"^S", tr("salvar", "save")},
		{"^Q", tr("fechar (salva)", "close (saves)")},
		{"^P", tr("prévia do markdown", "markdown preview")},
		{"^F", tr("expandir / reduzir", "expand / shrink")},
		{"^T", tr("modo vim / modo padrão", "vim / standard mode")},
		{"^E", tr("abrir no vim (editor externo)", "open in vim (external editor)")},
		{"^G", tr("mostrar / esconder atalhos", "show / hide shortcuts")},
	}
}

func (m *Model) markdownKeys() [][2]string {
	return [][2]string{
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
	}
}

func (m *Model) editingKeys() [][2]string {
	return [][2]string{
		{"^Z ^Y", tr("desfazer / refazer", "undo / redo")},
		{"^K", tr("recortar linha (repetir acumula)", "cut line (repeat to add)")},
		{"^U", tr("colar", "paste")},
		{"^W", tr("apagar palavra", "delete word")},
		{"tab", tr("recuar item / inserir espaços", "indent item / insert spaces")},
		{"^← ^→", tr("pular palavras", "jump words")},
		{"esc", tr("fechar (salva)", "close (saves)")},
	}
}

func (m *Model) vimKeys() [][2]string {
	return [][2]string{
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
		{":12", tr("ir para a linha 12", "go to line 12")},
		{"3j 2dd", tr("um número repete o comando", "a count repeats it")},
	}
}

func (m *Model) commandKeys() [][2]string {
	return [][2]string{
		{"quest", tr("abrir o diário (--vigilia abre na vigília)", "open the journal (--vigil opens vigil mode)")},
		{"quest add", tr("criar: --prazo --principal --prioridade --nota", "create: --due --main --priority --note")},
		{"quest list", tr("quests pendentes", "pending quests")},
		{"quest done 3", tr("concluir a quest 3", "complete quest 3")},
		{"quest lang en", tr("trocar o idioma", "switch the language")},
		{tr("quest lembretes", "quest reminders"), tr("--instalar --remover --testar", "--install --remove --test")},
	}
}

func (m *Model) helpGroups() []keyGroup {
	return []keyGroup{
		{tr("DIÁRIO", "JOURNAL"), m.journalKeys()},
		{tr("ETAPAS (depois de tab)", "STEPS (after tab)"), m.stepKeys()},
		{tr("FORMULÁRIO DE QUEST", "QUEST FORM"), m.formKeys()},
		{tr("NOTAS (o)", "NOTES (o)"), m.editorGeneralKeys()},
		{tr("NOTAS: EDIÇÃO", "NOTES: EDITING"), m.editingKeys()},
		{tr("NOTAS: MARKDOWN", "NOTES: MARKDOWN"), m.markdownKeys()},
		{tr("NOTAS: MODO VIM (^T)", "NOTES: VIM MODE (^T)"), m.vimKeys()},
		{tr("VIGÍLIA (z)", "VIGIL (z)"), m.vigilKeys()},
		{tr("LINHA DE COMANDO", "COMMAND LINE"), m.commandKeys()},
	}
}

// keyLines desenha grupos de atalhos com largura w; a coluna das teclas tem
// a largura da maior tecla.
func (m *Model) keyLines(groups []keyGroup, w int) []string {
	kw := 0
	for _, g := range groups {
		for _, k := range g.keys {
			kw = max(kw, lipgloss.Width(k[0]))
		}
	}
	var lines []string
	for i, g := range groups {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.section(m.ic.Steps, g.title, w))
		for _, k := range g.keys {
			key := k[0] + strings.Repeat(" ", kw-lipgloss.Width(k[0]))
			line := m.st.accent.Render(key) + "  " + m.st.text.Render(k[1])
			lines = append(lines, ansi.Truncate(line, w, "…"))
		}
	}
	return lines
}

// helpRows é quantas linhas de atalhos cabem na janela.
func (m *Model) helpRows() int {
	return max(4, m.h-9)
}

func (m *Model) helpWidth() int {
	return clamp(m.w-12, 30, 78)
}

func (m *Model) viewHelp() string {
	w := m.helpWidth()
	lines := m.keyLines(m.helpGroups(), w)
	rows := m.helpRows()
	m.helpOff = clamp(m.helpOff, 0, max(0, len(lines)-rows))
	end := min(len(lines), m.helpOff+rows)
	body := lines[m.helpOff:end]
	for len(body) < rows && len(lines) > rows {
		body = append(body, "")
	}

	hint := tr("qualquer tecla fecha", "any key closes")
	if len(lines) > rows {
		hint = fmt.Sprintf(tr("↑↓ rolar · %d–%d de %d · esc fecha", "↑↓ scroll · %d–%d of %d · esc closes"),
			m.helpOff+1, end, len(lines))
	}
	return strings.Join(body, "\n") + "\n\n" + m.st.muted.Render(ansi.Truncate(hint, w, "…"))
}

// openHelp abre a tela de atalhos do começo.
func (m *Model) openHelp() {
	m.helpOff = 0
	m.mode = modeHelp
}

func (m *Model) updateHelp(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	page := m.helpRows() - 2
	switch key.String() {
	case "up", "k":
		m.helpOff--
	case "down", "j":
		m.helpOff++
	case "pgup", "ctrl+u", "b":
		m.helpOff -= page
	case "pgdown", "ctrl+d", " ":
		m.helpOff += page
	case "home", "g":
		m.helpOff = 0
	case "end", "G":
		m.helpOff = 1 << 20
	default:
		m.mode = modeList
		return m, nil
	}
	m.helpOff = max(0, m.helpOff) // o limite de baixo é acertado ao desenhar
	return m, nil
}
