package theme

import (
	"math"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
)

// Os temas claros têm texto escuro; os escuros, texto claro; o mono não é
// nenhum dos dois (usa as cores do terminal).
func TestLight(t *testing.T) {
	light := map[string]bool{
		"heroi": false, "reino": false, "sombrio": false, "masmorra": false, "cristal": false,
		"alvorada": true, "pergaminho": true, "nuvem": true, "mono": false,
	}
	themes := Builtin()
	if len(themes) != len(light) {
		t.Fatalf("%d temas, esperava %d", len(themes), len(light))
	}
	for _, th := range themes {
		want, ok := light[th.Name]
		if !ok {
			t.Errorf("tema inesperado %q", th.Name)
			continue
		}
		if th.Light() != want {
			t.Errorf("%s: Light() = %v, esperava %v", th.Name, th.Light(), want)
		}
	}
}

// luminance é a luminância relativa do WCAG.
func luminance(c colorful.Color) float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func contrast(a, b colorful.Color) float64 {
	la, lb := luminance(a)+0.05, luminance(b)+0.05
	return math.Max(la, lb) / math.Min(la, lb)
}

// Nos temas claros, o texto e os destaques precisam ser legíveis sobre um
// fundo branco: contraste de pelo menos 3:1.
func TestLightThemesContrast(t *testing.T) {
	white, _ := colorful.Hex("#ffffff")
	for _, th := range Builtin() {
		if !th.Light() {
			continue
		}
		colors := map[string]string{"fg": th.Fg, "accent": th.Accent, "accent2": th.Accent2,
			"accent3": th.Accent3, "done": th.Done, "danger": th.Danger}
		for name, col := range colors {
			c, err := colorful.Hex(col)
			if err != nil {
				t.Fatalf("%s.%s: %v", th.Name, name, err)
			}
			if r := contrast(c, white); r < 3 {
				t.Errorf("%s.%s (%s): contraste %.2f com o branco", th.Name, name, col, r)
			}
		}
	}
}
