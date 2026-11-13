package ui

import (
	"math"
	"math/rand"
)

// Variações de partículas para cada tipo de aviso.

// palette são as cores das comemorações: os destaques do tema e o brilho.
func (m *Model) palette() []string {
	return []string{m.th.Accent, m.th.Accent2, m.accent3(), m.hi, m.th.Done}
}

// accent3 devolve a terceira cor de destaque, ou a segunda se o tema não tem.
func (m *Model) accent3() string {
	if m.th.Accent3 == "" {
		return m.th.Accent2
	}
	return m.th.Accent3
}

// fountain lança n centelhas para cima, logo abaixo do aviso: elas sobem,
// perdem força e caem.
func (m *Model) fountain(n int, colors []string) {
	cx, cy := float64(m.w)/2, float64(m.h)/2+3
	for i := 0; i < n; i++ {
		m.sparks = append(m.sparks, spark{
			x:     cx + (rand.Float64()-0.5)*16,
			y:     cy,
			vx:    (rand.Float64() - 0.5) * 22,
			vy:    -(9 + rand.Float64()*12),
			life:  1.0 + rand.Float64()*0.8,
			grav:  17,
			drag:  0.6,
			color: colors[rand.Intn(len(colors))],
		})
	}
}

var ringGlyphs = [3]string{"◆", "◇", "·"}

// ring abre um anel de n partículas a partir do centro, como uma onda.
func (m *Model) ring(n int, speed float64, colors []string) {
	cx, cy := float64(m.w)/2, float64(m.h)/2
	for i := 0; i < n; i++ {
		angle := float64(i) / float64(n) * 2 * math.Pi
		m.sparks = append(m.sparks, spark{
			x:      cx,
			y:      cy,
			vx:     math.Cos(angle) * speed * 2.2,
			vy:     math.Sin(angle) * speed,
			life:   0.9,
			drag:   1.6,
			color:  colors[i%len(colors)],
			glyphs: &ringGlyphs,
		})
	}
}

var ashGlyphs = [3]string{"▪", "·", "."}

// ash faz cair n cinzas do topo da tela, devagar e balançando.
func (m *Model) ash(n int) {
	colors := []string{m.th.Danger, m.th.Muted, m.th.BorderColor}
	for i := 0; i < n; i++ {
		m.sparks = append(m.sparks, spark{
			x:      rand.Float64() * float64(m.w),
			y:      -rand.Float64() * float64(m.h) / 3,
			vx:     (rand.Float64() - 0.5) * 3,
			vy:     4 + rand.Float64()*5,
			life:   2.0 + rand.Float64()*1.2,
			grav:   2,
			color:  colors[rand.Intn(len(colors))],
			glyphs: &ashGlyphs,
		})
	}
}

// shell é um fogo de artifício agendado: explode no instante at.
type shell struct {
	at, x, y float64
	colors   []string
}

// fireworks agenda n fogos espalhados pela parte de cima da tela, ao longo
// de span segundos.
func (m *Model) fireworks(n int, span float64) {
	all := m.palette()
	for i := 0; i < n; i++ {
		// Cada fogo usa duas cores da paleta, para variar de um para outro.
		a, b := rand.Intn(len(all)), rand.Intn(len(all))
		m.shells = append(m.shells, shell{
			at:     m.t + span*float64(i)/float64(n) + rand.Float64()*0.2,
			x:      float64(m.w) * (0.12 + 0.76*rand.Float64()),
			y:      float64(m.h) * (0.12 + 0.3*rand.Float64()),
			colors: []string{all[a], all[b], m.hi},
		})
	}
}

// stepShells explode os fogos cuja hora chegou.
func (m *Model) stepShells(now float64) {
	waiting := m.shells[:0]
	for _, s := range m.shells {
		if s.at > now {
			waiting = append(waiting, s)
			continue
		}
		m.burstAt(s.x, s.y, 45, 15, s.colors)
	}
	m.shells = waiting
}
