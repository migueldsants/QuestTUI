package ui

import (
	"math"
	"math/rand"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/mattn/go-runewidth"
)

// Efeitos visuais: mistura de cores, brilhos, centelhas e transições.

// blend mistura duas cores hex (t=0 é a, t=1 é b). Cores que não são hex
// (como os números ANSI do tema mono) não têm mistura: vale a mais próxima.
func blend(a, b string, t float64) string {
	t = math.Max(0, math.Min(1, t))
	ca, errA := colorful.Hex(a)
	cb, errB := colorful.Hex(b)
	if errA != nil || errB != nil {
		if t > 0.5 && b != "" {
			return b
		}
		return a
	}
	return ca.BlendLab(cb, t).Clamped().Hex()
}

// paint aplica a cor ao estilo, ignorando cor vazia.
func paint(st lipgloss.Style, color string) lipgloss.Style {
	if color == "" {
		return st
	}
	return st.Foreground(lipgloss.Color(color))
}

// step5 arredonda um brilho (0 a 1) para cinco níveis, para que células
// vizinhas compartilhem a mesma cor e sejam pintadas de uma vez.
func step5(glow float64) float64 {
	return math.Round(math.Max(0, math.Min(1, glow))*5) / 5
}

// colorize pinta cada célula com sua cor, agrupando vizinhas de cor igual.
func colorize(cells, colors []string, st lipgloss.Style) string {
	var b strings.Builder
	for i := 0; i < len(cells); {
		j := i
		var run strings.Builder
		for j < len(cells) && colors[j] == colors[i] {
			run.WriteString(cells[j])
			j++
		}
		b.WriteString(paint(st, colors[i]).Render(run.String()))
		i = j
	}
	return b.String()
}

// shimmer pinta s na cor base com um brilho centrado em pos (0 a 1 ao longo
// do texto). pos fora dessa faixa deixa o texto todo na cor base.
func shimmer(s, base, hi string, pos float64, st lipgloss.Style) string {
	runes := []rune(s)
	cells := make([]string, len(runes))
	colors := make([]string, len(runes))
	for i, r := range runes {
		d := math.Abs(float64(i)/float64(len(runes)) - pos)
		cells[i] = string(r)
		colors[i] = blend(base, hi, step5(1-d/0.18))
	}
	return colorize(cells, colors, st)
}

// spaceOut espaça as letras: "QUEST" vira "Q U E S T".
func spaceOut(s string) string {
	return strings.Join(strings.Split(s, ""), " ")
}

func hash2(x, y int) uint32 {
	h := uint32(x)*73856093 ^ uint32(y)*19349663
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	return h
}

// putCell troca a célula da coluna x da linha (que pode ter cores ANSI) por
// cell, preservando o resto.
//
// A linha é percorrida uma vez, guardando as sequências de cor em vigor, para
// religá-las depois da célula nova. (Cortar a linha com ansi.Truncate e
// ansi.TruncateLeft não serve: cada corte copia todas as sequências da linha,
// e várias trocas na mesma linha a fazem dobrar de tamanho a cada vez.)
func putCell(line string, x int, cell string) string {
	const reset = "\x1b[0m"
	var active []string // sequências de cor em vigor na posição atual
	sawEscape := false
	col := 0
	for i := 0; i < len(line); {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && (line[j] < 0x40 || line[j] > 0x7e) {
				j++
			}
			if j < len(line) {
				j++ // byte final da sequência
			}
			if seq := line[i:j]; strings.HasSuffix(seq, "m") {
				sawEscape = true
				if seq == reset || seq == "\x1b[m" {
					active = active[:0]
				} else {
					active = append(active, seq)
				}
			}
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		w := runewidth.RuneWidth(r)
		if w > 0 && col+w > x {
			// Esta é a célula a trocar.
			if !sawEscape {
				return line[:i] + cell + line[i+size:]
			}
			return line[:i] + reset + cell + reset + strings.Join(active, "") + line[i+size:]
		}
		col += w
		i += size
	}
	// A linha acaba antes da coluna x: completa com espaços.
	if sawEscape {
		line += reset
	}
	return line + strings.Repeat(" ", x-col) + cell
}

// --- centelhas ---

// spark é uma centelha da explosão de comemoração.
type spark struct {
	x, y, vx, vy float64
	age, life    float64
	grav, drag   float64 // gravidade e arrasto próprios de cada centelha
	color        string
	glyphs       *[3]string // do mais novo ao mais velho; nil é centelha
}

// burst solta n centelhas do centro da tela; power é a velocidade inicial em
// células por segundo.
func (m *Model) burst(n int, power float64) {
	m.burstAt(float64(m.w)/2, float64(m.h)/2, n, power, m.palette())
}

// burstAt solta n centelhas a partir de (cx, cy), com cores da paleta dada.
func (m *Model) burstAt(cx, cy float64, n int, power float64, palette []string) {
	for i := 0; i < n; i++ {
		angle := rand.Float64() * 2 * math.Pi
		speed := power * (0.3 + 0.7*rand.Float64())
		m.sparks = append(m.sparks, spark{
			x: cx + (rand.Float64()-0.5)*10,
			y: cy + (rand.Float64()-0.5)*2,
			// Uma célula é duas vezes mais alta que larga.
			vx:    math.Cos(angle) * speed * 2.2,
			vy:    math.Sin(angle)*speed - power*0.3,
			life:  0.8 + rand.Float64()*1.4,
			grav:  16,
			drag:  0.8,
			color: palette[rand.Intn(len(palette))],
		})
	}
}

// stepSparks avança a física das centelhas: gravidade, arrasto e fim de vida.
func (m *Model) stepSparks(dt float64) {
	alive := m.sparks[:0]
	for _, s := range m.sparks {
		s.age += dt
		s.vy += s.grav * dt
		s.vx *= 1 - s.drag*dt
		if s.grav == 0 {
			s.vy *= 1 - s.drag*dt // sem gravidade, o arrasto freia nas duas direções
		}
		s.x += s.vx * dt
		s.y += s.vy * dt
		if s.age >= s.life || s.y >= float64(m.h) || s.x < 0 || s.x >= float64(m.w) {
			continue
		}
		alive = append(alive, s)
	}
	m.sparks = alive
}

// overlaySparks desenha as centelhas por cima da tela.
func (m *Model) overlaySparks(view string) string {
	if len(m.sparks) == 0 {
		return view
	}
	lines := strings.Split(view, "\n")
	bold := lipgloss.NewStyle().Bold(true)
	for _, s := range m.sparks {
		x, y := int(s.x), int(s.y)
		if y < 0 || y >= len(lines) || x < 0 || x >= m.w {
			continue
		}
		// A centelha encolhe e esmaece conforme envelhece.
		f := s.age / s.life
		glyphs := [3]string{m.ic.Spark, "+", "·"}
		if s.glyphs != nil {
			glyphs = *s.glyphs
		}
		glyph := glyphs[min(2, int(f*3))]
		color := blend(s.color, m.th.BorderColor, (f-0.5)*2)
		lines[y] = putCell(lines[y], x, paint(bold, color).Render(glyph))
	}
	return strings.Join(lines, "\n")
}

// --- transição de abertura ---

const wipeDuration = 0.45 // segundos

// startWipe inicia a transição em que a tela se abre do centro para as bordas.
func (m *Model) startWipe() {
	m.wipeStart = m.elapsed()
}

func (m *Model) wiping() bool {
	return m.t-m.wipeStart < wipeDuration+0.1
}

// applyWipe esconde as linhas ainda não reveladas e traça um fio de luz na
// frente da abertura.
func (m *Model) applyWipe(view string) string {
	x := (m.t - m.wipeStart) / wipeDuration
	if x < 0 || x >= 1 {
		return view
	}
	lines := strings.Split(view, "\n")
	center := float64(len(lines)-1) / 2
	reach := (1 - math.Pow(1-x, 3)) * (center + 1)
	edge := paint(lipgloss.NewStyle(), blend(m.th.Accent, m.hi, 0.5)).Render(strings.Repeat(m.ic.Rule, m.w))
	for i := range lines {
		switch d := math.Abs(float64(i) - center); {
		case d > reach+1:
			lines[i] = ""
		case d > reach:
			lines[i] = edge
		}
	}
	return strings.Join(lines, "\n")
}
