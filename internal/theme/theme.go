// Package theme define os temas embutidos e carrega temas do usuário em TOML.
package theme

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/lucasb-eyer/go-colorful"
)

type Theme struct {
	Name        string `toml:"name"`
	Fg          string `toml:"fg"`
	Muted       string `toml:"muted"`
	Accent      string `toml:"accent"`
	Accent2     string `toml:"accent2"`
	Accent3     string `toml:"accent3"` // objetivo atual e prazos; vazio usa accent2
	BorderColor string `toml:"border_color"`
	Done        string `toml:"done"`
	Danger      string `toml:"danger"`
	Selection   string `toml:"selection"` // fundo da linha selecionada; vazio desliga
	Border      string `toml:"border"`    // rounded, normal, thick, double, hidden
}

// Light diz se o tema é para terminais de fundo claro: o texto dele é escuro.
func (t Theme) Light() bool {
	c, err := colorful.Hex(t.Fg)
	if err != nil {
		return false
	}
	l, _, _ := c.Lab()
	return l < 0.5
}

// Builtin devolve os temas que vêm com o app. As paletas seguem o jeito dos
// jogos de aventura de 16 bits: poucas cores, bem saturadas, com molduras de
// linha dupla. Os primeiros são para terminais escuros (texto creme); alvorada,
// pergaminho e nuvem são para terminais claros (texto escuro).
func Builtin() []Theme {
	return []Theme{
		{
			// Túnica verde, moldura dourada, azul e vermelho de apoio.
			Name: "heroi", Fg: "#f0e8d0", Muted: "#7c8f6c", Accent: "#f8d038", Accent2: "#58c848", Accent3: "#68a8f8",
			BorderColor: "#2f8038", Done: "#48a848", Danger: "#e84838", Selection: "#143018", Border: "double",
		},
		{
			// Castelo sob o sol: azul real, ouro e rosa.
			Name: "reino", Fg: "#f4ecd8", Muted: "#8c88a8", Accent: "#f8c850", Accent2: "#6890f0", Accent3: "#f07888",
			BorderColor: "#4860b8", Done: "#68b858", Danger: "#e03828", Selection: "#1a2248", Border: "double",
		},
		{
			// O mundo do outro lado: roxo, laranja e amarelo doentio.
			Name: "sombrio", Fg: "#ecdcc0", Muted: "#8a7460", Accent: "#f89838", Accent2: "#b870e0", Accent3: "#d8c848",
			BorderColor: "#6a4078", Done: "#88a048", Danger: "#e84858", Selection: "#2c1a34", Border: "double",
		},
		{
			// Pedra azulada, tochas e chaves douradas.
			Name: "masmorra", Fg: "#dce4f4", Muted: "#6878a0", Accent: "#78b8f8", Accent2: "#f8a040", Accent3: "#f8d858",
			BorderColor: "#40508c", Done: "#58b8a0", Danger: "#f05848", Selection: "#151c38", Border: "thick",
		},
		{
			// Gelo e magia: ciano, lilás e um amarelo pálido.
			Name: "cristal", Fg: "#e8f4f8", Muted: "#7090a0", Accent: "#68e0f0", Accent2: "#c890f8", Accent3: "#f8f0a0",
			BorderColor: "#3880a0", Done: "#60c8a8", Danger: "#f06878", Selection: "#10303c", Border: "double",
		},
		{
			// Claro: o herói ao amanhecer. Verde, âmbar e azul sobre fundo claro.
			Name: "alvorada", Fg: "#24301f", Muted: "#6f7a68", Accent: "#a46a00", Accent2: "#2f7d32", Accent3: "#1f62b5",
			BorderColor: "#8db38a", Done: "#2e7d32", Danger: "#c62828", Selection: "#e2efd9", Border: "double",
		},
		{
			// Claro: mapa antigo. Tinta sépia, ferrugem, verde-azulado e roxo.
			Name: "pergaminho", Fg: "#3a2e1f", Muted: "#8a7658", Accent: "#9c4f12", Accent2: "#2c6e6a", Accent3: "#7b3fa0",
			BorderColor: "#c2a878", Done: "#5a7a2a", Danger: "#b0322a", Selection: "#efe2c4", Border: "rounded",
		},
		{
			// Claro: o castelo sobre as nuvens. Azul real, ouro e rosa.
			Name: "nuvem", Fg: "#1d2440", Muted: "#6a7194", Accent: "#3456c0", Accent2: "#b77700", Accent3: "#c03a6b",
			BorderColor: "#a3aedb", Done: "#2f8a5a", Danger: "#c62828", Selection: "#dfe6fa", Border: "double",
		},
		{
			// Sem cores próprias, para qualquer terminal.
			Name: "mono", Fg: "", Muted: "8", Accent: "15", Accent2: "7",
			BorderColor: "8", Done: "8", Danger: "15", Selection: "", Border: "normal",
		},
	}
}

// Load junta os temas embutidos com os .toml da pasta dir. Um tema do usuário
// com o mesmo nome de um embutido substitui o embutido; campos ausentes herdam
// do primeiro tema embutido.
func Load(dir string) []Theme {
	themes := Builtin()
	files, _ := filepath.Glob(filepath.Join(dir, "*.toml"))
	sort.Strings(files)
	for _, f := range files {
		t := themes[0]
		t.Name = ""
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if err := toml.Unmarshal(data, &t); err != nil {
			continue
		}
		if t.Name == "" {
			t.Name = strings.TrimSuffix(filepath.Base(f), ".toml")
		}
		replaced := false
		for i := range themes {
			if themes[i].Name == t.Name {
				themes[i] = t
				replaced = true
			}
		}
		if !replaced {
			themes = append(themes, t)
		}
	}
	return themes
}

// Find devolve o tema com esse nome, ou o primeiro da lista. Um nome de tema
// que não existe mais (de uma versão anterior) cai no primeiro.
func Find(themes []Theme, name string) Theme {
	for _, t := range themes {
		if t.Name == name {
			return t
		}
	}
	return themes[0]
}
