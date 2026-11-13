package ui

import (
	"math"
	"strings"

	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Avisos animados: abrem com uma mola no centro da tela, ficam um instante e
// fecham. Cada tipo tem seu texto, sua cor e seu efeito de partículas.

type bannerKind int

const (
	bannerQuest   bannerKind = iota // quest concluída
	bannerStep                      // etapa concluída, com o próximo objetivo
	bannerAllDone                   // a última quest pendente foi concluída
	bannerNew                       // quest criada
	bannerTracked                   // quest rastreada
	bannerReopen                    // quest reaberta
	bannerAbandon                   // quest apagada
	bannerAlert                     // prazos de hoje e atrasos, ao abrir o app
	bannerFocus                     // fim de uma rodada ou pausa do modo foco
)

type banner struct {
	active   bool
	kind     bannerKind
	title    string
	next     string // linha extra: próximo objetivo ou detalhe
	pos, vel float64
	start    float64 // instante em que abriu, no relógio do modelo (segundos)
	spring   harmonica.Spring
}

func (b *banner) show(kind bannerKind, title, next string, now float64) {
	*b = banner{
		active: true,
		kind:   kind,
		title:  title,
		next:   next,
		start:  now,
		spring: harmonica.NewSpring(harmonica.FPS(30), 7.0, 0.6),
	}
}

// hold é quanto tempo o aviso fica aberto, em segundos.
func (b *banner) hold() float64 {
	switch b.kind {
	case bannerAllDone:
		return 3.6
	case bannerAlert:
		return 3.2
	case bannerFocus:
		return 2.6
	case bannerQuest:
		return 2.2
	case bannerStep:
		return 1.5
	}
	return 1.4
}

func (b *banner) step(now float64) {
	target := 1.0
	if now-b.start > b.hold() {
		target = 0
	}
	b.pos, b.vel = b.spring.Update(b.pos, b.vel, target)
	if target == 0 && b.pos < 0.03 {
		b.active = false
	}
}

// bannerLook é o conteúdo e a aparência de um aviso.
type bannerLook struct {
	lines          []string
	border         lipgloss.Border
	color          string // moldura
	rayColor       string
	rayFrom, rayTo int // linhas da caixa (contando a moldura) com raios; -1 desliga
	maxWidth       int
}

func (m *Model) bannerLook(w int) bannerLook {
	bn := m.banner
	fit := func(s string) string { return ansi.Truncate(s, w, "…") }
	bold := lipgloss.NewStyle().Bold(true)
	sweep := math.Mod(m.t*0.9, 1.6) - 0.3
	// heading pinta o título com o brilho que passa; com letras espaçadas
	// quando couber.
	heading := func(text, color string) string {
		if s := spaceOut(text); lipgloss.Width(s)+4 <= w {
			text = s
		}
		return shimmer(ansi.Truncate(text, w, ""), color, m.hi, sweep, bold)
	}
	look := bannerLook{border: m.st.border, color: m.th.Accent, rayColor: m.hi, rayFrom: -1, rayTo: -1}

	switch bn.kind {
	case bannerQuest:
		look.border = lipgloss.DoubleBorder()
		look.lines = []string{
			m.sparkles(w, 0),
			heading(tr("QUEST CONCLUÍDA", "QUEST COMPLETE"), m.th.Accent),
			m.st.text.Render(fit(m.ic.Trophy + " " + bn.title)),
		}
		look.lines = append(look.lines, m.sparkles(w, 1))
		look.rayFrom, look.rayTo = 2, 3

	case bannerAllDone:
		look.border = lipgloss.DoubleBorder()
		look.maxWidth = 62
		look.lines = []string{
			m.sparkles(w, 0),
			heading(tr("TODAS AS QUESTS CONCLUÍDAS", "ALL QUESTS COMPLETE"), m.th.Accent),
			m.st.text.Render(fit(m.ic.Trophy + " " + bn.title)),
			m.st.accent3.Render(fit(tr("Nada pendente. Descanse, herói.", "Nothing left. Rest, hero."))),
			m.sparkles(w, 1),
		}
		look.rayFrom, look.rayTo = 2, 4

	case bannerStep:
		look.lines = []string{
			m.st.muted.Render(fit(tr("ETAPA CONCLUÍDA", "STEP COMPLETE"))),
			m.st.done.Render(fit(m.ic.StepDone + " " + bn.title)),
			m.st.accent.Render(fit(m.ic.StepNow + " " + bn.next)),
		}
		look.rayFrom, look.rayTo = 2, 2

	case bannerNew:
		look.color = m.th.Accent2
		look.rayColor = blend(m.th.Accent2, m.hi, 0.5)
		look.lines = []string{
			heading(tr("NOVA QUEST", "NEW QUEST"), m.th.Accent2),
			m.st.text.Render(fit(m.ic.Journal + " " + bn.title)),
		}
		look.rayFrom, look.rayTo = 1, 2

	case bannerTracked:
		look.color = m.accent3()
		look.rayColor = blend(m.accent3(), m.hi, 0.5)
		look.lines = []string{
			heading(tr("QUEST RASTREADA", "QUEST TRACKED"), m.accent3()),
			m.st.text.Render(fit(m.ic.Tracked + " " + bn.title)),
			m.st.muted.Render(fit(tr("em destaque na vigília", "featured in vigil mode"))),
		}
		look.rayFrom, look.rayTo = 1, 2

	case bannerReopen:
		look.color = m.th.Accent2
		look.rayColor = blend(m.th.Accent2, m.hi, 0.5)
		look.lines = []string{
			heading(tr("QUEST RETOMADA", "QUEST RESUMED"), m.th.Accent2),
			m.st.text.Render(fit(m.ic.Journal + " " + bn.title)),
		}
		if bn.next != "" {
			look.lines = append(look.lines, m.st.accent3.Render(fit(m.ic.StepNow+" "+bn.next)))
		}
		look.rayFrom, look.rayTo = 1, 2

	case bannerAbandon:
		look.color = m.th.Danger
		look.lines = []string{
			m.st.danger.Bold(true).Render(fit(spaceOutFit(tr("QUEST ABANDONADA", "QUEST ABANDONED"), w))),
			m.st.muted.Strikethrough(true).Render(fit(bn.title)),
		}

	case bannerAlert:
		look.color = m.th.Danger
		look.lines = []string{
			m.st.danger.Bold(true).Render(fit(m.ic.Late + " " + spaceOutFit(tr("ATENÇÃO", "HEADS UP"), w-2))),
			m.st.text.Render(fit(bn.title)),
		}

	case bannerFocus:
		look.color = m.accent3()
		look.rayColor = blend(m.accent3(), m.hi, 0.5)
		look.lines = []string{
			heading(bn.title, m.accent3()),
			m.st.text.Render(fit(m.ic.Focus + " " + bn.next)),
		}
		look.rayFrom, look.rayTo = 1, 2
	}
	return look
}

// spaceOutFit espaça as letras quando o texto espaçado cabe em w.
func spaceOutFit(s string, w int) string {
	if spaced := spaceOut(s); lipgloss.Width(spaced)+4 <= w {
		return spaced
	}
	return s
}

// sparkles é uma linha de centelhas que mudam de lugar com o tempo.
func (m *Model) sparkles(width, row int) string {
	tick := int(m.t * 5)
	var b strings.Builder
	for x := 0; x < width; x++ {
		switch h := hash2(x+tick*31, row+tick); {
		case h%11 == 0:
			b.WriteString(paint(lipgloss.NewStyle(), m.hi).Render(m.ic.Spark))
		case h%5 == 0:
			b.WriteString(m.st.accent2.Render("·"))
		default:
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// overlayBanner desenha o aviso por cima do centro da tela. A largura segue
// uma mola: o aviso abre, fica um instante e fecha.
func (m *Model) overlayBanner(view string) string {
	bn := m.banner
	maxW := 52
	if bn.kind == bannerAllDone {
		maxW = 62
	}
	full := clamp(m.w-12, 16, maxW)
	cur := int(bn.pos * float64(full))
	if cur > m.w-6 {
		cur = m.w - 6
	}
	if cur < 4 {
		return view
	}
	look := m.bannerLook(cur)
	box := lipgloss.NewStyle().
		Border(look.border).
		BorderForeground(lipgloss.Color(look.color)).
		Width(cur).
		Align(lipgloss.Center).
		Render(strings.Join(look.lines, "\n"))

	lines := strings.Split(view, "\n")
	boxLines := strings.Split(box, "\n")
	top := (len(lines) - len(boxLines)) / 2
	if top < 0 {
		return view
	}
	// Raios de luz saem das laterais do aviso, nas linhas do texto.
	for i, bl := range boxLines {
		left := (m.w - lipgloss.Width(bl)) / 2
		right := m.w - left - lipgloss.Width(bl)
		if i < look.rayFrom || i > look.rayTo {
			lines[top+i] = strings.Repeat(" ", max(0, left)) + bl + strings.Repeat(" ", max(0, right))
			continue
		}
		lines[top+i] = m.ray(left, bn.pos, true, look.rayColor) + bl + m.ray(right, bn.pos, false, look.rayColor)
	}
	return strings.Join(lines, "\n")
}

// ray desenha um raio de n células que nasce junto ao aviso e esmaece para
// longe dele; reach (0 a 1) é o quanto ele já se estendeu. toLeft indica o
// raio da esquerda, que cresce da direita para a esquerda.
func (m *Model) ray(n int, reach float64, toLeft bool, color string) string {
	if n <= 0 {
		return ""
	}
	lit := int(math.Min(1, reach) * float64(n))
	cells := make([]string, n)
	colors := make([]string, n)
	for i := range cells {
		d := i // distância até o aviso
		if toLeft {
			d = n - 1 - i
		}
		cells[i], colors[i] = " ", ""
		if d < lit {
			fade := float64(d) / float64(n)
			cells[i] = m.ic.BarFull
			if fade > 0.6 {
				cells[i] = m.ic.BarEmpty
			}
			colors[i] = blend(color, m.th.BorderColor, math.Round(fade*8)/8)
		}
	}
	return colorize(cells, colors, lipgloss.NewStyle())
}
