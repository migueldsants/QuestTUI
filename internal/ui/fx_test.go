package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Fora de um terminal o lipgloss desliga as cores; os testes as ligam para
// exercitar o mesmo texto com sequências ANSI que o app produz de verdade.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// putCell deve trocar exatamente uma célula, em qualquer coluna, sem mudar a
// largura da linha — inclusive em linhas com cores e na última coluna.
func TestPutCell(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("║ abc") +
		lipgloss.NewStyle().Bold(true).Render(" ✓ déf ") + "xyz ║"
	bg := lipgloss.NewStyle().Background(lipgloss.Color("#1c2e22"))
	withBg := bg.Foreground(lipgloss.Color("#f2c94c")).Bold(true).Render("▸ ") + bg.Render("○ ") +
		bg.Strikethrough(true).Render("lavar a louça") + bg.Render("   ") +
		lipgloss.NewStyle().Underline(true).Render("Ativas 2")
	for _, line := range []string{"", "plain text", styled, withBg, strings.Repeat(" ", 12)} {
		width := lipgloss.Width(line)
		for x := 0; x < width+3; x++ {
			got := putCell(line, x, "*")
			want := width
			if x >= width {
				want = x + 1
			}
			if w := lipgloss.Width(got); w != want {
				t.Fatalf("putCell(%q, %d): largura %d, esperava %d", line, x, w, want)
			}
		}
	}
}

// Muitas trocas na mesma linha não podem fazê-la crescer sem controle: era
// esse o travamento ao concluir uma quest.
func TestPutCellDoesNotBlowUp(t *testing.T) {
	st := lipgloss.NewStyle().Foreground(lipgloss.Color("#f2c94c")).Background(lipgloss.Color("#1c2e22"))
	line := strings.Repeat(st.Render("ab")+st.Bold(true).Render("cd"), 20)
	before := len(line)
	for i := 0; i < 200; i++ {
		line = putCell(line, (i*7)%80, st.Render("*"))
	}
	if len(line) > before*20 {
		t.Fatalf("linha cresceu de %d para %d bytes", before, len(line))
	}
	if w := lipgloss.Width(line); w != 80 {
		t.Fatalf("largura %d, esperava 80", w)
	}
}
