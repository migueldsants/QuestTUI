package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// keys manda uma sequência de teclas: nomes entre < > são teclas especiais
// (<enter>, <esc>, <alt+b>...), o resto é digitado letra por letra.
func keys(e *Editor, seq string) Action {
	var act Action
	for seq != "" {
		var k tea.KeyMsg
		if strings.HasPrefix(seq, "<") && strings.Contains(seq, ">") {
			name := seq[1:strings.Index(seq, ">")]
			seq = seq[len(name)+2:]
			switch name {
			case "enter":
				k = tea.KeyMsg{Type: tea.KeyEnter}
			case "esc":
				k = tea.KeyMsg{Type: tea.KeyEsc}
			case "bs":
				k = tea.KeyMsg{Type: tea.KeyBackspace}
			case "tab":
				k = tea.KeyMsg{Type: tea.KeyTab}
			case "ctrl+z":
				k = tea.KeyMsg{Type: tea.KeyCtrlZ}
			case "ctrl+y":
				k = tea.KeyMsg{Type: tea.KeyCtrlY}
			case "ctrl+k":
				k = tea.KeyMsg{Type: tea.KeyCtrlK}
			case "ctrl+u":
				k = tea.KeyMsg{Type: tea.KeyCtrlU}
			case "ctrl+r":
				k = tea.KeyMsg{Type: tea.KeyCtrlR}
			case "space":
				k = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
			default:
				if strings.HasPrefix(name, "alt+") {
					k = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name[4:]), Alt: true}
				}
			}
		} else {
			r := []rune(seq)[0]
			seq = seq[len(string(r)):]
			if r == ' ' {
				k = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
			} else {
				k = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
			}
		}
		act = e.Update(k)
	}
	return act
}

func check(t *testing.T, e *Editor, want string) {
	t.Helper()
	if got := e.Text(); got != want {
		t.Fatalf("texto:\n%q\nesperava:\n%q", got, want)
	}
}

func TestStandardTyping(t *testing.T) {
	e := New("", false)
	keys(e, "- comprar pão<enter>leite<enter><enter>fim")
	check(t, e, "- comprar pão\n- leite\nfim")

	e = New("", false)
	keys(e, "1. um<enter>dois<enter><enter>- [x] feito<enter>novo")
	check(t, e, "1. um\n2. dois\n- [x] feito\n- [ ] novo")

	e = New("> citação", false)
	keys(e, "<ctrl+u>") // nada recortado: não faz nada
	e.col = len(e.line())
	keys(e, "<enter>mais")
	check(t, e, "> citação\n> mais")
}

func TestUndoRedoAndKill(t *testing.T) {
	e := New("", false)
	keys(e, "abc<enter>def")
	keys(e, "<ctrl+z>")
	check(t, e, "abc\n")
	keys(e, "<ctrl+z><ctrl+z>")
	check(t, e, "")
	keys(e, "<ctrl+y>")
	check(t, e, "abc")

	e = New("um\ndois\ntrês", false)
	keys(e, "<ctrl+k><ctrl+k>")
	check(t, e, "três")
	keys(e, "<ctrl+u>")
	check(t, e, "um\ndois\ntrês")
}

func TestMarkdownShortcuts(t *testing.T) {
	e := New("texto importante", false)
	e.col = 8
	keys(e, "<alt+b>")
	check(t, e, "texto **importante**")
	keys(e, "<alt+b>")
	check(t, e, "texto importante")
	keys(e, "<alt+i>")
	check(t, e, "texto *importante*")
	keys(e, "<alt+2>")
	check(t, e, "## texto *importante*")
	keys(e, "<alt+2>")
	check(t, e, "texto *importante*")
	keys(e, "<alt+t>")
	check(t, e, "- [ ] texto *importante*")
	keys(e, "<alt+t>")
	check(t, e, "- [x] texto *importante*")
	keys(e, "<alt+l>")
	check(t, e, "- texto *importante*")
	keys(e, "<alt+l>")
	check(t, e, "texto *importante*")
	keys(e, "<alt+q>")
	check(t, e, "> texto *importante*")

	e = New("ver site", false)
	e.col = 5
	keys(e, "<alt+k>docs.com")
	check(t, e, "ver [site](docs.com)")
}

func TestVimNormal(t *testing.T) {
	e := New("uma frase com palavras\nsegunda linha\nterceira", true)
	keys(e, "w")
	if e.col != 4 {
		t.Fatalf("w: coluna %d", e.col)
	}
	keys(e, "dw")
	check(t, e, "uma com palavras\nsegunda linha\nterceira")
	keys(e, "cwnova<esc>")
	check(t, e, "uma nova palavras\nsegunda linha\nterceira")
	keys(e, "u")
	check(t, e, "uma com palavras\nsegunda linha\nterceira")
	keys(e, "<ctrl+r>")
	check(t, e, "uma nova palavras\nsegunda linha\nterceira")
	keys(e, "jddp")
	check(t, e, "uma nova palavras\nterceira\nsegunda linha")
	keys(e, "ggyyP")
	check(t, e, "uma nova palavras\numa nova palavras\nterceira\nsegunda linha")
	keys(e, "2dd")
	check(t, e, "terceira\nsegunda linha")
	keys(e, "$x0~")
	check(t, e, "Terceir\nsegunda linha")
	keys(e, "J")
	check(t, e, "Terceir segunda linha")
	keys(e, "Afim<esc>")
	check(t, e, "Terceir segunda linhafim")
	keys(e, "0ofilha<esc>")
	check(t, e, "Terceir segunda linhafim\nfilha")
	keys(e, "ggd$")
	check(t, e, "\nfilha")
	keys(e, "dG")
	check(t, e, "")
}

func TestVimVisualSearchAndCommands(t *testing.T) {
	e := New("abc def ghi\nlinha dois\nfim", true)
	keys(e, "wvey")
	if e.reg != "def" {
		t.Fatalf("yank visual: %q", e.reg)
	}
	keys(e, "0vlld")
	check(t, e, " def ghi\nlinha dois\nfim")
	keys(e, "Vjd")
	check(t, e, "fim")

	e = New("um\ndois\num de novo", true)
	keys(e, "/um<enter>")
	if e.row != 2 || e.col != 0 {
		t.Fatalf("busca: %d,%d", e.row, e.col)
	}
	keys(e, "n")
	if e.row != 0 {
		t.Fatalf("n deu a volta? linha %d", e.row)
	}
	keys(e, "ix<esc>")
	if act := keys(e, ":q<enter>"); act != None || e.msg == "" {
		t.Fatal(":q com alteração deveria recusar")
	}
	if act := keys(e, ":w<enter>"); act != Save {
		t.Fatal(":w deveria gravar")
	}
	if act := keys(e, ":wq<enter>"); act != SaveClose {
		t.Fatal(":wq deveria gravar e fechar")
	}
	if act := keys(e, ":q!<enter>"); act != DiscardClose {
		t.Fatal(":q! deveria fechar")
	}
	keys(e, ":3<enter>")
	if e.row != 2 {
		t.Fatalf(":3 foi para a linha %d", e.row+1)
	}
	keys(e, "<alt+b>")
	check(t, e, "xum\ndois\n**um** de novo")
	keys(e, "A <alt+i>fim<esc>")
	check(t, e, "xum\ndois\n**um** de novo *fim*")
}

// Várias teclas numa mensagem só valem como teclas separadas.
func TestKeyBurst(t *testing.T) {
	e := New("um\ndois", true)
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("dd")})
	check(t, e, "dois")
	if act := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":wq")}); act != None {
		t.Fatal("o comando só roda com enter")
	}
	if act := e.Update(tea.KeyMsg{Type: tea.KeyEnter}); act != SaveClose {
		t.Fatal(":wq em rajada não gravou e fechou")
	}
}

func TestViewSize(t *testing.T) {
	long := strings.Repeat("palavra comprida ", 20)
	e := New("# Título\n\n- [x] feito\n- [ ] **negrito** e `código`\n"+long+"\n```\ncode\n```", false)
	e.SetTheme(Theme{Fg: "#ffffff", Muted: "#888888", Accent: "#ffcc00", Accent2: "#00ff00", Accent3: "#00aaff",
		Done: "#00ff00", Danger: "#ff0000", Border: "#444444", CursorLine: "#111111", Selection: "#333333"})
	for _, size := range [][2]int{{40, 10}, {80, 30}, {20, 3}} {
		out := e.View(size[0], size[1])
		lines := strings.Split(out, "\n")
		if len(lines) != size[1] {
			t.Fatalf("%v: %d linhas", size, len(lines))
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w != size[0] {
				t.Fatalf("%v: linha %d com largura %d: %q", size, i, w, l)
			}
		}
	}
	// O cursor no fim de um texto longo continua visível.
	e.row, e.col = len(e.lines)-1, 0
	if out := e.View(40, 5); !strings.Contains(ansi.Strip(out), "```") {
		t.Fatalf("cursor fora da tela:\n%s", ansi.Strip(out))
	}
}
