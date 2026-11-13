package ui

import (
	"math"

	"github.com/charmbracelet/lipgloss"
)

// Céu da vigília, em duas camadas:
//
//   - estrelas paradas que cintilam: cada uma tem seu ritmo e muda de cor
//     aos poucos, do apagado ao brilhante, sem trocar de símbolo;
//   - poeira de estrelas que desliza devagar. Ela é desenhada com pontos
//     braille, que dividem cada célula em 2×4 posições, para andar uma
//     fração de célula por quadro em vez de pular de célula em célula.
//
// Tudo é calculado a partir do tempo, sem estado: a mesma tela no mesmo
// instante desenha sempre o mesmo céu.

// brailleDot é o bit de cada ponto braille, por coluna (0 a 1) e linha (0 a 3).
var brailleDot = [2][4]rune{
	{0x01, 0x02, 0x04, 0x40},
	{0x08, 0x10, 0x20, 0x80},
}

// skyLevels é quantos tons cada estrela pode ter entre apagada e acesa.
const skyLevels = 10

func frac(x float64) float64 { return x - math.Floor(x) }

// wrap traz x para o intervalo [0, n).
func wrap(x, n float64) float64 {
	x = math.Mod(x, n)
	if x < 0 {
		x += n
	}
	return x
}

// sky desenha o céu da tela inteira; a chave é y*largura + x.
func (m *Model) sky() map[int]string {
	w, h := m.w, m.h
	cells := make(map[int]string, w*h/40)
	if w <= 0 || h <= 0 {
		return cells
	}
	ascii := m.cfg.Icons == "ascii"
	st := lipgloss.NewStyle()
	tone := func(from, to string, b float64) lipgloss.Style {
		return paint(st, blend(from, to, math.Round(b*skyLevels)/skyLevels))
	}

	// Poeira: cada partícula tem posição inicial e velocidade próprias. As
	// mais rápidas parecem mais próximas e brilham um pouco mais.
	type dust struct {
		bits rune
		glow float64
	}
	grains := map[int]*dust{}
	n := max(12, w*h/170)
	for i := 0; i < n; i++ {
		hx, hy, hv := hash2(i, 7), hash2(i, 13), hash2(i, 29)
		depth := float64(hv%100) / 100 // 0 = longe, 1 = perto
		vx := -(0.18 + 0.75*depth)     // células por segundo, para a esquerda
		vy := (float64(hv>>8%100)/100 - 0.5) * 0.15
		x := wrap(float64(hx%10000)/10000*float64(w)+vx*m.t, float64(w))
		y := wrap(float64(hy%10000)/10000*float64(h)+vy*m.t, float64(h))
		cx, cy := int(x), int(y)
		k := cy*w + cx
		d := grains[k]
		if d == nil {
			d = &dust{}
			grains[k] = d
		}
		if ascii {
			d.bits = 1
		} else {
			d.bits |= brailleDot[int(frac(x)*2)][int(frac(y)*4)]
		}
		pulse := 0.5 + 0.5*math.Sin(m.t*(0.8+depth)+float64(hv>>16%628)/100)
		d.glow = math.Max(d.glow, 0.25+0.45*depth+0.3*pulse*depth)
	}
	for k, d := range grains {
		g := string(0x2800 | d.bits)
		if ascii {
			g = "."
		}
		cells[k] = tone(m.th.BorderColor, m.th.Muted, d.glow).Render(g)
	}

	// Estrelas que cintilam, por cima da poeira.
	glyphs := [3]string{m.ic.Spark, "+", "·"}
	if ascii {
		glyphs = [3]string{"*", "+", "."}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			hs := hash2(x+101, y+37)
			if hs%67 != 0 {
				continue
			}
			size := 2 // pequena
			switch r := hs >> 8 % 12; {
			case r == 0:
				size = 0 // grande
			case r < 4:
				size = 1
			}
			speed := 0.5 + float64(hs>>12%100)/100*1.6
			phase := float64(hs>>16%628) / 100
			b := 0.5 + 0.5*math.Sin(m.t*speed+phase)
			b *= b // passa mais tempo apagada e acende de vez em quando
			peak := m.hi
			if hs>>20%4 == 0 {
				peak = m.th.Accent2 // algumas têm a cor do tema
			}
			if size == 2 {
				b *= 0.7
			}
			cells[y*w+x] = tone(m.th.BorderColor, peak, 0.15+0.85*b).Render(glyphs[size])
		}
	}
	return cells
}

// field desenha a linha y do céu, das colunas from até to (excluída).
func (m *Model) field(sky map[int]string, y, from, to int) string {
	if to <= from {
		return ""
	}
	var b []byte
	for x := from; x < to; x++ {
		if s, ok := sky[y*m.w+x]; ok {
			b = append(b, s...)
		} else {
			b = append(b, ' ')
		}
	}
	return string(b)
}
