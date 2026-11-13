// Package ui é a interface do diário de quests.
package ui

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/huh"

	"questtui/internal/config"
	"questtui/internal/lang"
	"questtui/internal/remind"
	"questtui/internal/sound"
	"questtui/internal/stats"
	"questtui/internal/store"
	"questtui/internal/theme"
)

type mode int

const (
	modeList mode = iota
	modeForm
	modeInput
	modeConfirm
	modeThemes
	modeHelp
	modeVigil
	modeStats
	modeNotes
)

type inputKind int

const (
	inputStep     inputKind = iota // nova etapa
	inputStepEdit                  // renomear a etapa sob o cursor
	inputFilter
)

type (
	tickMsg  time.Time
	frameMsg struct{}
)

// tr devolve o texto no idioma atual da interface.
func tr(pt, en string) string { return lang.T(pt, en) }

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

const (
	fastFrame = time.Second / 30 // enquanto algo se move
	slowFrame = time.Second / 12 // só brilhos e partículas

	remindEvery = 30 * time.Second // de quanto em quanto tempo checar os lembretes
)

// frame agenda o próximo quadro: rápido durante animações, lento no resto do
// tempo para não gastar CPU.
func (m *Model) frame() tea.Cmd {
	d := slowFrame
	if m.banner.active || m.barPos != m.barTarget() || len(m.sparks) > 0 || len(m.shells) > 0 || m.wiping() ||
		(m.mode == modeVigil && m.meteorPhase() < 1) {
		d = fastFrame
	}
	return tea.Tick(d, func(time.Time) tea.Msg { return frameMsg{} })
}

// elapsed devolve os segundos desde que o app abriu.
func (m *Model) elapsed() float64 {
	return time.Since(m.started).Seconds()
}

// enterVigil abre o modo vigília com a transição de abertura.
func (m *Model) enterVigil() {
	m.mode = modeVigil
	m.startWipe()
}

type formData struct {
	title, kind, due, notes string
	priority                bool
}

type Model struct {
	store  *store.Store
	cfg    *config.Config
	themes []theme.Theme
	th     theme.Theme
	st     styles
	ic     icons
	hi     string // cor de brilho, derivada do destaque do tema
	sound  *sound.Player

	w, h    int
	now     time.Time
	started time.Time
	t       float64 // segundos desde o início, base dos efeitos

	// barra de progresso animada da quest em foco
	barPos, barVel float64
	barSpring      harmonica.Spring

	sparks    []spark // centelhas das comemorações
	shells    []shell // fogos de artifício agendados
	wipeStart float64 // instante (em t) da última transição de abertura

	quests  []store.Quest // tudo que está no banco
	visible []store.Quest // o que a lista mostra, já ordenado
	cursor  int

	focusSteps bool
	stepCursor int
	doneTab    bool // false: aba Ativas; true: aba Concluídas
	filter     string

	mode      mode
	form      *huh.Form
	fd        *formData
	editingID int64
	input     textinput.Model
	inputKind inputKind

	themeCursor int
	prevTheme   theme.Theme

	focus      focusState
	stats      stats.Stats
	lastRemind time.Time

	banner    banner
	lastInput time.Time
	err       error
	note      string // aviso curto no rodapé
	noteUntil time.Time

	notesKey, notesOut string

	notesEd notesState // editor de notas
}

func New(s *store.Store, cfg *config.Config, themes []theme.Theme, startVigil bool) *Model {
	m := &Model{
		store:     s,
		cfg:       cfg,
		themes:    themes,
		now:       time.Now(),
		started:   time.Now(),
		lastInput: time.Now(),
		ic:        iconSet(cfg.Icons),
		sound:     &sound.Player{},
		barSpring: harmonica.NewSpring(harmonica.FPS(30), 8.0, 1.0),
	}
	lang.Set(cfg.Lang)
	m.applyTheme(theme.Find(themes, cfg.Theme))
	if startVigil {
		m.mode = modeVigil
	}
	m.reload()
	m.barPos = m.barTarget()
	m.startupAlert()
	return m
}

// Close libera o que o modelo deixou aberto (o processo de som).
func (m *Model) Close() { m.sound.Close() }

func (m *Model) applyTheme(t theme.Theme) {
	m.th = t
	m.st = newStyles(t)
	// O brilho clareia o destaque; em tema de fundo claro (texto escuro), escurece.
	m.hi = blend(t.Accent, "#ffffff", 0.7)
	if t.Light() {
		m.hi = blend(t.Accent, "#000000", 0.6)
	}
	m.notesKey = "" // as notas são renderizadas com o estilo claro ou escuro
	if m.notesEd.ed != nil {
		m.notesEd.ed.SetTheme(m.editorTheme())
		m.notesEd.prevKey = ""
	}
}

// glamourStyle é o estilo do markdown que combina com o tema.
func (m *Model) glamourStyle() string {
	if m.th.Light() {
		return "light"
	}
	return "dark"
}

func (m *Model) Init() tea.Cmd { return tea.Batch(tick(), m.frame()) }

// play toca o efeito, se o som estiver ligado.
func (m *Model) play(e sound.Effect) {
	if m.cfg.Sound {
		m.sound.Play(e)
	}
}

// flash mostra um aviso curto no rodapé.
func (m *Model) flash(text string) {
	m.note, m.noteUntil = text, time.Now().Add(2500*time.Millisecond)
}

// tracked devolve a quest rastreada pendente, ou nil.
func (m *Model) tracked() *store.Quest {
	for i, q := range m.quests {
		if q.Tracked && !q.Done {
			return &m.quests[i]
		}
	}
	return nil
}

// find devolve a quest com esse id, ou nil.
func (m *Model) find(id int64) *store.Quest {
	for i := range m.quests {
		if m.quests[i].ID == id {
			return &m.quests[i]
		}
	}
	return nil
}

// pendingNow conta as quests pendentes.
func (m *Model) pendingNow() int {
	n := 0
	for _, q := range m.quests {
		if q.Done {
			continue
		}
		n++
	}
	return n
}

// barTarget é o progresso (0 a 1) da quest em foco: a rastreada na vigília,
// a selecionada no diário.
func (m *Model) barTarget() float64 {
	q := m.selected()
	if m.mode == modeVigil {
		q = m.tracked()
	}
	if q == nil || len(q.Steps) == 0 {
		return 0
	}
	return float64(q.StepsDone()) / float64(len(q.Steps))
}

// selected devolve a quest sob o cursor, ou nil se a lista está vazia.
func (m *Model) selected() *store.Quest {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return nil
	}
	return &m.visible[m.cursor]
}

func (m *Model) reload() {
	var selID int64
	if q := m.selected(); q != nil {
		selID = q.ID
	}
	quests, err := m.store.Quests()
	if err != nil {
		m.err = err
		return
	}
	m.quests = quests
	m.rebuild(selID)
}

// rebuild refaz a lista visível (filtro, grupos, ordenação) e tenta manter o
// cursor na quest selID.
func (m *Model) rebuild(selID int64) {
	needle := strings.ToLower(m.filter)
	var main, side, done []store.Quest
	for _, q := range m.quests {
		if needle != "" && !strings.Contains(strings.ToLower(q.Title), needle) {
			continue
		}
		switch {
		case q.Done:
			done = append(done, q)
		case q.Kind == store.Main:
			main = append(main, q)
		default:
			side = append(side, q)
		}
	}
	sortPending(main)
	sortPending(side)
	sort.SliceStable(done, func(i, j int) bool { return done[i].DoneAt.After(done[j].DoneAt) })

	if m.doneTab {
		m.visible = done
	} else {
		m.visible = append(main, side...)
	}

	found := false
	for i, q := range m.visible {
		if q.ID == selID {
			m.cursor, found = i, true
			break
		}
	}
	if !found {
		m.stepCursor = 0
	}
	m.cursor = clamp(m.cursor, 0, len(m.visible)-1)

	q := m.selected()
	if q == nil || len(q.Steps) == 0 {
		m.focusSteps = false
		m.stepCursor = 0
	} else {
		m.stepCursor = clamp(m.stepCursor, 0, len(q.Steps)-1)
	}
}

// sortPending: prioridade alta primeiro; dentro disso, a ordem manual (as
// quests já chegam do banco nessa ordem).
func sortPending(qs []store.Quest) {
	sort.SliceStable(qs, func(i, j int) bool {
		return qs[i].Priority && !qs[j].Priority
	})
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// startupAlert avisa, ao abrir o app, quantas quests vencem hoje ou estão
// atrasadas.
func (m *Model) startupAlert() {
	today, late := 0, 0
	for _, q := range m.quests {
		if q.Done {
			continue
		}
		days, isLate, ok := m.dueInfo(q)
		switch {
		case !ok:
		case isLate:
			late++
		case days == 0:
			today++
		}
	}
	var parts []string
	if today > 0 {
		parts = append(parts, fmt.Sprintf(tr("%d com prazo hoje", "%d due today"), today))
	}
	if late > 0 {
		parts = append(parts, plural(late, tr("atrasada", "overdue"), tr("atrasadas", "overdue")))
	}
	if len(parts) > 0 {
		m.banner.show(bannerAlert, strings.Join(parts, " · "), "", 0)
	}
}

// remindCmd envia, em segundo plano, os lembretes de prazo que estão pendentes.
func (m *Model) remindCmd() tea.Cmd {
	s, now := m.store, m.now
	return func() tea.Msg {
		_ = remind.Run(s, now)
		return nil
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.notesKey = ""

	case tickMsg:
		m.now = time.Time(msg)
		if m.mode == modeList || m.mode == modeVigil {
			// Pega quests adicionadas por fora (quest add) sem precisar reabrir.
			if m.now.Second()%3 == 0 {
				m.reload()
			}
		}
		idle := time.Duration(m.cfg.IdleMinutes) * time.Minute
		if m.mode == modeList && idle > 0 && time.Since(m.lastInput) > idle {
			m.enterVigil()
		}
		cmds := []tea.Cmd{tick(), m.stepFocus()}
		if m.cfg.Reminders && m.now.Sub(m.lastRemind) >= remindEvery {
			m.lastRemind = m.now
			cmds = append(cmds, m.remindCmd())
		}
		return m, tea.Batch(cmds...)

	case frameMsg:
		now := m.elapsed()
		m.stepShells(now)
		m.stepSparks(math.Min(now-m.t, 0.1))
		m.t = now
		if m.banner.active {
			m.banner.step(now)
		}
		target := m.barTarget()
		m.barPos, m.barVel = m.barSpring.Update(m.barPos, m.barVel, target)
		if math.Abs(m.barPos-target) < 0.004 && math.Abs(m.barVel) < 0.004 {
			m.barPos, m.barVel = target, 0
		}
		return m, m.frame()

	case tea.KeyMsg:
		m.lastInput = time.Now()
		if msg.String() == "ctrl+c" && m.mode != modeNotes {
			return m, tea.Quit
		}
		// Teclas digitadas rápido podem chegar juntas ("jj"); nos atalhos,
		// cada uma conta separada. Campos de texto recebem tudo de uma vez.
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !msg.Paste &&
			m.mode != modeForm && m.mode != modeInput && m.mode != modeNotes {
			var cmds []tea.Cmd
			for _, r := range msg.Runes {
				_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: msg.Alt})
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	switch m.mode {
	case modeNotes:
		return m.updateNotes(msg)
	case modeForm:
		return m.updateForm(msg)
	case modeInput:
		return m.updateInput(msg)
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.mode {
	case modeConfirm:
		return m.updateConfirm(key)
	case modeThemes:
		return m.updateThemes(key)
	case modeHelp, modeStats:
		m.mode = modeList
		return m, nil
	case modeVigil:
		return m.updateVigil(key)
	}
	return m.updateList(key)
}

func (m *Model) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.err = nil
	q := m.selected()

	switch key.String() {
	case "q":
		return m, tea.Quit

	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "home", "g":
		m.move(-len(m.visible) - 1000)
	case "end", "G":
		m.move(len(m.visible) + 1000)
	case "shift+up", "K":
		m.reorder(-1)
	case "shift+down", "J":
		m.reorder(1)

	case "tab":
		if q != nil && len(q.Steps) > 0 {
			m.focusSteps = !m.focusSteps
		}
	case "right", "l":
		if q != nil && len(q.Steps) > 0 {
			m.focusSteps = true
		}
	case "left", "h":
		m.focusSteps = false
	case "esc":
		if m.focusSteps {
			m.focusSteps = false
		} else if m.filter != "" {
			m.filter = ""
			m.rebuildKeep()
		}

	case " ", "enter", "x":
		m.toggle()
	case "u", "backspace":
		if q != nil {
			m.undoStep(*q)
		}
	case "ctrl+l":
		m.cfg.Lang = lang.Toggle()
		m.err = m.cfg.Save()
		m.notesKey = ""
		m.flash(tr("idioma: português", "language: English"))
	case "i":
		m.cfg.Icons = nextIconSet(m.cfg.Icons)
		m.ic = iconSet(m.cfg.Icons)
		m.err = m.cfg.Save()
	case "m":
		m.cfg.Sound = !m.cfg.Sound
		m.err = m.cfg.Save()
		if m.cfg.Sound {
			m.flash(tr("som ligado", "sound on"))
			m.play(sound.Tick)
		} else {
			m.flash(tr("som desligado", "sound off"))
		}

	case "n":
		return m, m.openForm(nil)
	case "e":
		switch {
		case q == nil:
		case m.focusSteps:
			return m, m.openInput(inputStepEdit, q.Steps[m.stepCursor].Title)
		default:
			return m, m.openForm(q)
		}
	case "a":
		if q != nil {
			return m, m.openInput(inputStep, "")
		}
	case "o":
		if q != nil {
			m.openNotes(*q)
		}
	case "/":
		return m, m.openInput(inputFilter, m.filter)

	case "d", "delete":
		if q == nil {
			break
		}
		if m.focusSteps {
			m.err = m.store.DeleteStep(q.Steps[m.stepCursor].ID)
			m.reload()
		} else {
			m.mode = modeConfirm
		}

	case "r":
		if q != nil && !q.Done {
			m.track(*q)
		}
	case "p":
		if q != nil && !q.Done {
			m.err = m.store.SetPriority(q.ID, !q.Priority)
			m.reload()
			if !q.Priority {
				m.flash(tr("prioridade alta", "high priority"))
			} else {
				m.flash(tr("prioridade normal", "normal priority"))
			}
		}
	case "f":
		m.focusToggle()
	case "F":
		m.focusStop()
	case "s":
		m.openStats()

	case "c":
		m.setTab(!m.doneTab)
	case "1":
		m.setTab(false)
	case "2":
		m.setTab(true)

	case "t", "T":
		m.prevTheme = m.th
		m.themeCursor = 0
		for i, t := range m.themes {
			if t.Name == m.th.Name {
				m.themeCursor = i
			}
		}
		m.mode = modeThemes
	case "z":
		m.enterVigil()
	case "?":
		m.mode = modeHelp
	}
	return m, nil
}

// setTab troca entre a aba Ativas e a aba Concluídas.
func (m *Model) setTab(done bool) {
	if m.doneTab == done {
		return
	}
	m.doneTab = done
	m.focusSteps = false
	m.cursor = 0
	m.rebuild(0)
}

func (m *Model) rebuildKeep() {
	var id int64
	if q := m.selected(); q != nil {
		id = q.ID
	}
	m.rebuild(id)
}

func (m *Model) move(delta int) {
	q := m.selected()
	if q == nil {
		return
	}
	if m.focusSteps {
		m.stepCursor = clamp(m.stepCursor+delta, 0, len(q.Steps)-1)
		return
	}
	m.cursor = clamp(m.cursor+delta, 0, len(m.visible)-1)
	m.stepCursor = 0
}

// reorder troca a quest selecionada de lugar com a vizinha, dentro do mesmo
// grupo e da mesma prioridade. Nas etapas, troca a etapa sob o cursor.
func (m *Model) reorder(dir int) {
	q := m.selected()
	if q == nil {
		return
	}
	if m.focusSteps {
		i, j := m.stepCursor, m.stepCursor+dir
		if j < 0 || j >= len(q.Steps) {
			return
		}
		if m.err = m.store.SwapSteps(q.Steps[i].ID, q.Steps[j].ID); m.err == nil {
			m.stepCursor = j
		}
		m.reload()
		return
	}
	if q.Done {
		return
	}
	j := m.cursor + dir
	if j < 0 || j >= len(m.visible) {
		return
	}
	o := m.visible[j]
	if o.Done || o.Kind != q.Kind || o.Priority != q.Priority {
		return
	}
	m.err = m.store.Swap(q.ID, o.ID)
	m.reload()
}

// toggle é a ação do espaço. Na lista, avança a quest: conclui o objetivo
// atual e, quando não sobra nenhum, a quest. Nas etapas, marca ou desmarca a
// etapa sob o cursor.
func (m *Model) toggle() {
	q := m.selected()
	if q == nil {
		return
	}
	if !m.focusSteps {
		if q.Done {
			m.reopen(*q)
		} else {
			m.advance(*q)
		}
		return
	}

	st := q.Steps[m.stepCursor]
	if err := m.store.SetStepDone(st.ID, !st.Done); err != nil {
		m.err = err
		return
	}
	switch {
	case !st.Done && !q.Done && q.StepsDone()+1 == len(q.Steps):
		m.complete(*q) // era a última etapa pendente
	case st.Done && q.Done:
		m.err = m.store.Reopen(q.ID)
		m.reload()
	default:
		m.reload()
	}
}

// advance conclui o objetivo atual da quest; sem etapas pendentes, conclui a
// própria quest.
func (m *Model) advance(q store.Quest) {
	i := q.CurrentStep()
	if i < 0 {
		m.complete(q)
		return
	}
	if err := m.store.SetStepDone(q.Steps[i].ID, true); err != nil {
		m.err = err
		return
	}
	for _, st := range q.Steps[i+1:] {
		if !st.Done {
			m.reload()
			m.banner.show(bannerStep, q.Steps[i].Title, st.Title, m.elapsed())
			m.burst(18, 11)
			m.play(sound.Step)
			return
		}
	}
	m.complete(q)
}

// complete conclui a quest; concluir a última quest pendente vira uma
// comemoração maior, com fogos.
func (m *Model) complete(q store.Quest) {
	if err := m.store.Complete(q.ID); err != nil {
		m.err = err
		return
	}
	m.reload()
	switch {
	case m.pendingNow() == 0:
		m.banner.show(bannerAllDone, q.Title, "", m.elapsed())
		m.burst(80, 24)
		m.fireworks(7, 2.6)
		m.play(sound.AllDone)
	default:
		m.banner.show(bannerQuest, q.Title, "", m.elapsed())
		m.burst(110, 26)
		m.play(sound.Quest)
	}
}

// reopen reabre a quest. Se todas as etapas estavam concluídas, a última
// volta a ser o objetivo atual.
func (m *Model) reopen(q store.Quest) {
	if err := m.store.Reopen(q.ID); err != nil {
		m.err = err
		return
	}
	if n := len(q.Steps); n > 0 && q.CurrentStep() < 0 {
		m.err = m.store.SetStepDone(q.Steps[n-1].ID, false)
	}
	m.reload()
	next := ""
	if r := m.find(q.ID); r != nil {
		if i := r.CurrentStep(); i >= 0 {
			next = r.Steps[i].Title
		}
	}
	m.banner.show(bannerReopen, q.Title, next, m.elapsed())
	m.ring(28, 10, []string{m.th.Accent2, m.hi})
}

// undoStep volta um objetivo: desfaz a última etapa concluída.
func (m *Model) undoStep(q store.Quest) {
	if q.Done {
		m.reopen(q)
		return
	}
	for i := len(q.Steps) - 1; i >= 0; i-- {
		if q.Steps[i].Done {
			m.err = m.store.SetStepDone(q.Steps[i].ID, false)
			break
		}
	}
	m.reload()
}

// track rastreia a quest (ou tira o rastreio, se ela já era a rastreada).
func (m *Model) track(q store.Quest) {
	if err := m.store.ToggleTracked(q.ID); err != nil {
		m.err = err
		return
	}
	m.reload()
	if r := m.find(q.ID); r != nil && r.Tracked {
		m.banner.show(bannerTracked, q.Title, "", m.elapsed())
		m.ring(36, 12, []string{m.accent3(), m.hi, m.th.Accent})
		return
	}
	m.flash(tr("rastreio removido", "no longer tracked"))
}

func (m *Model) openStats() {
	cs, err := m.store.Completions()
	if err != nil {
		m.err = err
		return
	}
	m.stats = stats.Compute(cs, time.Now())
	m.mode = modeStats
}

// --- formulário de quest ---

// openForm abre o formulário de quest: em branco, ou preenchido para editar q.
func (m *Model) openForm(q *store.Quest) tea.Cmd {
	fd := &formData{kind: string(store.Side)}
	m.editingID = 0
	if q != nil {
		m.editingID = q.ID
		fd = &formData{
			title:    q.Title,
			kind:     string(q.Kind),
			due:      store.FormatDue(q.Due),
			notes:    q.Notes,
			priority: q.Priority,
		}
	}
	m.fd = fd

	main := huh.NewGroup(
		huh.NewInput().
			Title(tr("Título", "Title")).
			Value(&fd.title).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New(tr("dê um nome para a quest", "give the quest a name"))
				}
				return nil
			}),
		huh.NewSelect[string]().
			Title(tr("Tipo", "Type")).
			Options(
				huh.NewOption(tr("Secundária", "Side quest"), string(store.Side)),
				huh.NewOption(tr("Principal", "Main quest"), string(store.Main)),
			).
			Inline(true).
			Value(&fd.kind),
		huh.NewConfirm().
			Title(tr("Prioridade alta?", "High priority?")).
			Affirmative(tr("Sim", "Yes")).
			Negative(tr("Não", "No")).
			Inline(true).
			Value(&fd.priority),
		huh.NewInput().
			Title(tr("Prazo", "Due date")).
			Description(tr("hoje, amanhã 14h, sexta, +3d, 20/11 14:00 (opcional)", "today, tomorrow 14h, friday, +3d, 20/11 14:00 (optional)")).
			Value(&fd.due).
			Validate(func(s string) error {
				_, err := store.ParseDue(s, time.Now())
				return err
			}),
	)

	// Além de tab e enter, ↑ e ↓ também andam entre os campos de texto e a
	// pergunta de prioridade (nas listas de opções, eles escolhem a opção).
	km := huh.NewDefaultKeyMap()
	km.Input.Next = key.NewBinding(key.WithKeys("enter", "tab", "down"), key.WithHelp("↓", "next"))
	km.Input.Prev = key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("↑", "back"))
	km.Confirm.Next = key.NewBinding(key.WithKeys("enter", "tab", "down"), key.WithHelp("↓", "next"))
	km.Confirm.Prev = key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("↑", "back"))

	m.form = huh.NewForm(main).
		WithTheme(huhTheme(m.th)).WithWidth(m.formWidth()).WithShowHelp(false).WithKeyMap(km)
	m.mode = modeForm
	return m.form.Init()
}

func (m *Model) formWidth() int {
	return clamp(m.w-12, 30, 64)
}

func (m *Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.mode = modeList
		return m, nil
	}
	f, cmd := m.form.Update(msg)
	if form, ok := f.(*huh.Form); ok {
		m.form = form
	}
	switch m.form.State {
	case huh.StateAborted:
		m.mode = modeList
		return m, nil
	case huh.StateCompleted:
		m.mode = modeList
		m.saveForm()
		return m, nil
	}
	return m, cmd
}

func (m *Model) saveForm() {
	now := time.Now()
	due, err := store.ParseDue(m.fd.due, now)
	if err != nil {
		m.err = err
		return
	}
	q := store.Quest{
		ID:       m.editingID,
		Title:    strings.TrimSpace(m.fd.title),
		Notes:    strings.TrimSpace(m.fd.notes),
		Kind:     store.Kind(m.fd.kind),
		Due:      due,
		Priority: m.fd.priority,
	}
	if m.editingID != 0 {
		if m.err = m.store.UpdateQuest(q); m.err == nil {
			m.flash(tr("quest atualizada", "quest updated"))
		}
		m.reload()
		return
	}
	if m.err = m.store.AddQuest(&q); m.err != nil {
		return
	}
	// Leva o cursor para a quest recém-criada.
	quests, err := m.store.Quests()
	if err != nil {
		m.err = err
		return
	}
	m.quests = quests
	m.setTab(false)
	m.rebuild(q.ID)
	m.banner.show(bannerNew, q.Title, "", m.elapsed())
	m.fountain(40, []string{m.th.Accent2, m.th.Accent, m.hi})
	m.play(sound.Tick)
}

// --- entrada de uma linha (etapa nova, filtro) ---

func (m *Model) openInput(kind inputKind, value string) tea.Cmd {
	m.input = textinput.New()
	m.input.Prompt = ""
	m.input.CharLimit = 120
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.inputKind = kind
	m.mode = modeInput
	return m.input.Focus()
}

func (m *Model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			if m.inputKind == inputFilter {
				m.filter = ""
				m.rebuildKeep()
			}
			m.mode = modeList
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			if m.inputKind == inputStepEdit {
				if q := m.selected(); q != nil && value != "" && m.stepCursor < len(q.Steps) {
					m.err = m.store.RenameStep(q.Steps[m.stepCursor].ID, value)
					m.reload()
					m.flash(tr("etapa renomeada", "step renamed"))
				}
				m.mode = modeList
				return m, nil
			}
			if m.inputKind == inputStep && value != "" {
				if q := m.selected(); q != nil {
					m.err = m.store.AddStep(q.ID, value)
					m.reload()
				}
				// Continua aberto para adicionar várias etapas em sequência.
				m.input.SetValue("")
				return m, nil
			}
			m.mode = modeList
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.inputKind == inputFilter && m.filter != m.input.Value() {
		m.filter = m.input.Value()
		m.rebuildKeep()
	}
	return m, cmd
}

// --- confirmação de exclusão ---

func (m *Model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeList
	switch key.String() {
	case "s", "y", "enter":
		if q := m.selected(); q != nil {
			title := q.Title
			if m.err = m.store.DeleteQuest(q.ID); m.err != nil {
				return m, nil
			}
			m.reload()
			m.banner.show(bannerAbandon, title, "", m.elapsed())
			m.ash(70)
		}
	}
	return m, nil
}

// --- seletor de temas ---

func (m *Model) updateThemes(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		m.themeCursor = clamp(m.themeCursor-1, 0, len(m.themes)-1)
		m.applyTheme(m.themes[m.themeCursor])
	case "down", "j":
		m.themeCursor = clamp(m.themeCursor+1, 0, len(m.themes)-1)
		m.applyTheme(m.themes[m.themeCursor])
	case "enter", " ":
		m.cfg.Theme = m.th.Name
		m.err = m.cfg.Save()
		m.notesKey = ""
		m.mode = modeList
	case "esc", "q":
		m.applyTheme(m.prevTheme)
		m.mode = modeList
	}
	return m, nil
}

// --- vigília ---

func (m *Model) updateVigil(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case " ":
		if q := m.tracked(); q != nil {
			m.advance(*q)
			return m, nil
		}
	case "f":
		m.focusToggle()
		return m, nil
	}
	m.mode = modeList
	m.startWipe()
	return m, nil
}
