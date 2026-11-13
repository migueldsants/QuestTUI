package ui

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"questtui/internal/config"
	"questtui/internal/store"
	"questtui/internal/theme"
)

// newTestModel monta um diário com uma quest de três etapas em um banco
// temporário, numa tela de 100x26.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "quests.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	q := store.Quest{Title: "Entregar relatório", Kind: store.Main}
	if err := s.AddQuest(&q); err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"Juntar os dados", "Escrever", "Enviar"} {
		if err := s.AddStep(q.ID, step); err != nil {
			t.Fatal(err)
		}
	}
	// Uma segunda quest: ao concluir a primeira, o cursor cai nela e as
	// centelhas passam por cima da linha selecionada.
	if err := s.AddQuest(&store.Quest{Title: "lavar a louça", Kind: store.Side}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Theme: "heroi", Icons: "unicode", Lang: "pt", Clock24h: true}
	m := New(s, cfg, theme.Builtin(), false)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 26})
	return m
}

// frames simula n quadros de animação de 1/30 s, desenhando a tela em cada um.
func frames(m *Model, n int) {
	for i := 0; i < n; i++ {
		m.started = m.started.Add(-time.Second / 30)
		m.Update(frameMsg{})
		m.View()
	}
}

// Avançar etapa por etapa conclui a quest na última, e as animações de cada
// passo (aviso, centelhas, barra) terminam sozinhas.
func TestAdvanceThroughSteps(t *testing.T) {
	m := newTestModel(t)
	frames(m, 30)

	for step := 1; step <= 3; step++ {
		m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		frames(m, 150) // 5 segundos: tempo para toda a animação acabar

		q := m.quests[0]
		if got := q.StepsDone(); got != step {
			t.Fatalf("após %d espaços: %d etapas concluídas", step, got)
		}
		if q.Done != (step == 3) {
			t.Fatalf("após %d espaços: quest concluída = %v", step, q.Done)
		}
		if m.banner.active || len(m.sparks) > 0 {
			t.Fatalf("após %d espaços: animação não terminou (aviso=%v, centelhas=%d)", step, m.banner.active, len(m.sparks))
		}
	}
}

// O modo vigília desenha exatamente a altura da tela, com e sem estrela
// cadente no céu.
func TestVigilFillsScreen(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	for i := 0; i < 400; i++ { // mais de 13 s: passa por uma estrela cadente
		frames(m, 1)
		if got := len(splitLines(m.View())); got != 26 {
			t.Fatalf("quadro %d: vigília com %d linhas, esperava 26", i, got)
		}
	}
}

// Os outros eventos também abrem avisos e soltam partículas, e tudo termina
// sozinho: criar, rastrear, apagar e concluir a última quest (com fogos).
func TestEventBanners(t *testing.T) {
	m := newTestModel(t)
	frames(m, 30)
	key := func(s string) {
		if s == " " {
			m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			return
		}
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	}
	settle := func(what string) {
		t.Helper()
		frames(m, 200) // quase 7 s
		if m.banner.active || len(m.sparks) > 0 || len(m.shells) > 0 {
			t.Fatalf("%s: animação não terminou", what)
		}
	}

	key("r")
	if m.banner.kind != bannerTracked || !m.banner.active {
		t.Fatal("rastrear não abriu o aviso")
	}
	settle("rastrear")

	// Apaga a segunda quest ("lavar a louça"), sem etapas.
	key("j")
	key("d")
	key("s")
	if m.banner.kind != bannerAbandon || len(m.quests) != 1 {
		t.Fatalf("apagar: aviso %v, %d quests", m.banner.kind, len(m.quests))
	}
	settle("apagar")

	// A única quest que sobrou tem 3 etapas: na terceira, a última quest
	// pendente é concluída e vêm os fogos.
	for i := 0; i < 3; i++ {
		key(" ")
		if i < 2 {
			settle("etapa")
		}
	}
	if m.banner.kind != bannerAllDone || len(m.shells) == 0 {
		t.Fatalf("última quest: aviso %v, %d fogos", m.banner.kind, len(m.shells))
	}
	settle("todas concluídas")

	// Reordenar e prioridade.
	for _, title := range []string{"b", "c"} {
		if err := m.store.AddQuest(&store.Quest{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	m.reload()
	m.setTab(false)
	if m.visible[0].Title != "b" {
		t.Fatalf("primeira ativa: %q", m.visible[0].Title)
	}
	key("J")
	if m.visible[1].Title != "b" || m.selected().Title != "b" {
		t.Fatal("shift+baixo não moveu a quest")
	}
	key("p")
	if m.visible[0].Title != "b" || !m.visible[0].Priority {
		t.Fatal("prioridade não subiu a quest para o topo")
	}

	// Conquistas abrem e desenham sem quebrar.
	key("s")
	if m.mode != modeStats || m.stats.Total != 1 {
		t.Fatalf("conquistas: modo %v, total %d", m.mode, m.stats.Total)
	}
	if m.View() == "" {
		t.Fatal("tela de conquistas vazia")
	}
}

// Nas etapas, e renomeia a etapa e shift+↓ muda ela de lugar.
func TestEditSteps(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.focusSteps {
		t.Fatal("tab não entrou nas etapas")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if m.mode != modeInput || m.inputKind != inputStepEdit || m.input.Value() != "Juntar os dados" {
		t.Fatalf("e nas etapas: modo %v, entrada %q", m.mode, m.input.Value())
	}
	m.input.SetValue("Juntar tudo")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	q := m.selected()
	if q.Steps[0].Title != "Escrever" || q.Steps[1].Title != "Juntar tudo" || m.stepCursor != 1 {
		t.Fatalf("etapas: %q, %q (cursor %d)", q.Steps[0].Title, q.Steps[1].Title, m.stepCursor)
	}
}

// o abre o editor de notas; o texto é gravado com ^S, e ^T troca para o vim,
// que fecha com :wq. A tela sempre ocupa a altura do terminal.
func TestNotesEditor(t *testing.T) {
	m := newTestModel(t)
	runes := func(s string) {
		for _, r := range s {
			if r == ' ' {
				m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			} else {
				m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if m.mode != modeNotes {
		t.Fatal("o não abriu o editor")
	}
	runes("- ler")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	runes("revisar")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if got := m.find(m.notesEd.questID).Notes; got != "- ler\n- revisar" {
		t.Fatalf("notas gravadas: %q", got)
	}
	for _, full := range []bool{false, true} {
		m.notesEd.full = full
		for _, help := range []bool{false, true} {
			m.notesEd.help = help
			if got := len(splitLines(m.View())); got != 26 {
				t.Fatalf("tela de notas (cheia=%v, atalhos=%v) com %d linhas", full, help, got)
			}
		}
	}

	m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if !m.notesEd.ed.Vim() || m.cfg.EditorMode != "vim" {
		t.Fatal("^T não ligou o vim")
	}
	runes("ggdd:wq")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeList {
		t.Fatal(":wq não fechou o editor")
	}
	if got := m.find(m.notesEd.questID).Notes; got != "- revisar" {
		t.Fatalf("notas depois do vim: %q", got)
	}

	// ^C dentro do editor fecha o editor, não o app.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd != nil || m.mode != modeList {
		t.Fatal("^C no editor deveria só fechar o editor")
	}
}

// ^L troca o idioma; L sozinho não faz mais isso.
func TestLanguageShortcut(t *testing.T) {
	m := newTestModel(t)
	before := m.cfg.Lang
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	if m.cfg.Lang != before {
		t.Fatal("L ainda troca o idioma")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if m.cfg.Lang == before {
		t.Fatal("^L não trocou o idioma")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
}

// O modo foco troca de fase quando o tempo acaba.
func TestFocus(t *testing.T) {
	m := newTestModel(t)
	m.cfg.FocusMinutes, m.cfg.BreakMinutes = 25, 5
	m.focusToggle()
	if !m.focus.active || m.focusLabel() == "" {
		t.Fatal("foco não começou")
	}
	m.Update(tickMsg(time.Now().Add(26 * time.Minute)))
	if !m.focus.onBreak || m.focus.rounds != 1 || m.banner.kind != bannerFocus {
		t.Fatalf("depois de 26 min: pausa=%v rodadas=%d", m.focus.onBreak, m.focus.rounds)
	}
	m.focusStop()
	if m.focus.active {
		t.Fatal("foco não parou")
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return append(lines, s[start:])
}
