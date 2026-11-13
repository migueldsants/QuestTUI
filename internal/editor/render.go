package editor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// Desenho do editor: numeração, quebra de linha suave, realce de markdown,
// linha do cursor, seleção e cursor.

// Theme são as cores do editor (hex ou número ANSI).
type Theme struct {
	Fg, Muted, Accent, Accent2, Accent3, Done, Danger, Border string
	CursorLine                                                string // fundo da linha do cursor
	Selection                                                 string // fundo da seleção visual
}

func (e *Editor) SetTheme(th Theme) { e.theme = th }

// Classes de realce.
const (
	cPlain = iota
	cHeading
	cBold
	cItalic
	cStrike
	cCode
	cCodeBlock
	cMarker
	cTask
	cDone
	cLink
	cURL
	cQuote
	cRule
)

var (
	fenceRe    = regexp.MustCompile("^\\s*(```|~~~)")
	ruleRe     = regexp.MustCompile(`^\s*([-*_])(\s*[-*_]){2,}\s*$`)
	codeRe     = regexp.MustCompile("`[^`]+`")
	boldRe     = regexp.MustCompile(`\*\*[^*]+\*\*|__[^_]+__`)
	italicRe   = regexp.MustCompile(`\*[^*\s][^*]*\*|\b_[^_\s][^_]*_\b`)
	strikeRe   = regexp.MustCompile(`~~[^~]+~~`)
	linkRe     = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	headingAny = regexp.MustCompile(`^#{1,6}\s`)
)

// classify dá a classe de realce de cada runa da linha.
func classify(l []rune, fence, inFence bool) []int {
	cls := make([]int, len(l))
	fill := func(a, b, c int) {
		for i := max(0, a); i < min(b, len(cls)); i++ {
			cls[i] = c
		}
	}
	s := string(l)
	switch {
	case fence || inFence:
		fill(0, len(l), cCodeBlock)
		return cls
	case headingAny.MatchString(s):
		fill(0, len(l), cHeading)
		return cls
	case ruleRe.MatchString(s):
		fill(0, len(l), cRule)
		return cls
	}

	// Índice de runa de cada byte, para converter as posições das regex.
	at := make([]int, len(s)+1)
	ri := 0
	for bi := 0; bi < len(s); bi++ {
		at[bi] = ri
		if bi+1 < len(s) && isRuneStart(s, bi+1) {
			ri++
		}
	}
	at[len(s)] = len(l)
	mark := func(re *regexp.Regexp, c int) {
		for _, m := range re.FindAllStringIndex(s, -1) {
			fill(at[m[0]], at[m[1]], c)
		}
	}

	rest := 0
	if m := quoteRe.FindStringIndex(s); m != nil {
		fill(0, at[m[1]], cMarker)
		fill(at[m[1]], len(l), cQuote)
		rest = at[m[1]]
	}
	done := false
	if m := listRe.FindStringSubmatchIndex(s); m != nil {
		fill(at[m[4]], at[m[5]], cMarker)
		if m[8] >= 0 {
			fill(at[m[8]], at[m[9]], cTask)
			done = strings.ContainsAny(s[m[8]:m[9]], "xX")
		}
		rest = at[m[1]]
	}
	mark(italicRe, cItalic)
	mark(boldRe, cBold)
	mark(strikeRe, cStrike)
	for _, m := range linkRe.FindAllStringSubmatchIndex(s, -1) {
		fill(at[m[0]], at[m[1]], cMarker)
		fill(at[m[2]], at[m[3]], cLink)
		fill(at[m[4]], at[m[5]], cURL)
	}
	mark(codeRe, cCode)
	if done {
		fill(rest, len(l), cDone)
	}
	return cls
}

func isRuneStart(s string, i int) bool { return s[i]&0xC0 != 0x80 }

func (e *Editor) style(c int) lipgloss.Style {
	th := e.theme
	st := lipgloss.NewStyle()
	fg := func(col string) lipgloss.Style {
		if col == "" {
			return st
		}
		return st.Foreground(lipgloss.Color(col))
	}
	switch c {
	case cHeading:
		return fg(th.Accent).Bold(true)
	case cBold:
		return fg(th.Fg).Bold(true)
	case cItalic:
		return fg(th.Fg).Italic(true)
	case cStrike:
		return fg(th.Muted).Strikethrough(true)
	case cCode, cCodeBlock:
		return fg(th.Accent3)
	case cMarker:
		return fg(th.Accent2).Bold(true)
	case cTask:
		return fg(th.Accent3).Bold(true)
	case cDone:
		return fg(th.Muted).Strikethrough(true)
	case cLink:
		return fg(th.Accent3).Underline(true)
	case cURL:
		return fg(th.Muted)
	case cQuote:
		return fg(th.Muted).Italic(true)
	case cRule:
		return fg(th.Border)
	}
	return fg(th.Fg)
}

// segments quebra a linha em trechos que cabem em width colunas, de
// preferência depois de um espaço.
func segments(l []rune, width int) [][2]int {
	if len(l) == 0 || width < 2 {
		return [][2]int{{0, len(l)}}
	}
	var segs [][2]int
	start := 0
	for start < len(l) {
		w, i, space := 0, start, -1
		for i < len(l) {
			rw := runewidth.RuneWidth(l[i])
			if w+rw > width {
				break
			}
			w += rw
			if l[i] == ' ' {
				space = i
			}
			i++
		}
		if i >= len(l) {
			segs = append(segs, [2]int{start, len(l)})
			break
		}
		end := i
		if space >= start && space+1 < i {
			end = space + 1
		}
		if end == start {
			end = start + 1
		}
		segs = append(segs, [2]int{start, end})
		start = end
	}
	return segs
}

type vrow struct {
	line int
	seg  [2]int
	cont bool // continuação de uma linha quebrada
}

// View desenha a área de texto com exatamente h linhas de largura w.
func (e *Editor) View(w, h int) string {
	if w < 8 || h < 1 {
		return ""
	}
	gw := max(3, len(fmt.Sprint(len(e.lines)))) + 1
	tw := w - gw
	th := e.theme
	cursorRow, cursorCol := e.row, e.col

	// Linhas visuais e a posição do cursor entre elas. A última coluna fica
	// livre para o cursor depois do fim da linha.
	var rows []vrow
	cur := 0
	for i, l := range e.lines {
		segs := segments(l, tw-1)
		for k, sg := range segs {
			if i == cursorRow && cursorCol >= sg[0] && (cursorCol < sg[1] || k == len(segs)-1) {
				cur = len(rows)
			}
			rows = append(rows, vrow{i, sg, k > 0})
		}
	}
	if cur < e.top {
		e.top = cur
	}
	if cur >= e.top+h {
		e.top = cur - h + 1
	}
	e.top = max(0, min(e.top, max(0, len(rows)-1)))

	// Estado de bloco de código no começo de cada linha.
	inFence := make([]bool, len(e.lines))
	isFence := make([]bool, len(e.lines))
	open := false
	for i, l := range e.lines {
		if fenceRe.MatchString(string(l)) {
			isFence[i] = true
			open = !open
			continue
		}
		inFence[i] = open
	}

	var selA, selB Pos
	selLines := false
	switch e.vs {
	case vVisual:
		if e.vim {
			selA, selB = e.selection()
		}
	case vVisualLine:
		if e.vim {
			a, b := e.selectedRows()
			selA, selB, selLines = Pos{a, 0}, Pos{b, len(e.lines[b])}, true
		}
	}
	inSel := func(r, c int) bool {
		if selLines {
			return r >= selA.Row && r <= selB.Row
		}
		p := Pos{r, c}
		return !p.before(selA) && p.before(selB)
	}

	cursorBg := th.Accent3
	if e.vim && e.vs != vInsert {
		cursorBg = th.Accent
	}
	gutter := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Muted))
	gutterCur := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Accent)).Bold(true)
	classes := map[int][]int{}

	out := make([]string, 0, h)
	for i := e.top; i < len(rows) && len(out) < h; i++ {
		vr := rows[i]
		l := e.lines[vr.line]
		cls, ok := classes[vr.line]
		if !ok {
			cls = classify(l, isFence[vr.line], inFence[vr.line])
			classes[vr.line] = cls
		}
		onCursorLine := vr.line == cursorRow

		num := strings.Repeat(" ", gw-1) + " "
		if !vr.cont {
			num = fmt.Sprintf("%*d ", gw-1, vr.line+1)
		}
		var b strings.Builder
		if onCursorLine {
			b.WriteString(gutterCur.Render(num))
		} else {
			b.WriteString(gutter.Render(num))
		}

		// Agrupa runas vizinhas com o mesmo estilo.
		type key struct {
			c        int
			sel, cur bool
		}
		width := 0
		var run strings.Builder
		var runKey key
		flush := func() {
			if run.Len() == 0 {
				return
			}
			st := e.style(runKey.c)
			switch {
			case runKey.cur:
				st = st.Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color(cursorBg))
			case runKey.sel && th.Selection != "":
				st = st.Background(lipgloss.Color(th.Selection))
			case onCursorLine && th.CursorLine != "":
				st = st.Background(lipgloss.Color(th.CursorLine))
			}
			b.WriteString(st.Render(run.String()))
			run.Reset()
		}
		for c := vr.seg[0]; c < vr.seg[1]; c++ {
			k := key{cls[c], inSel(vr.line, c), onCursorLine && c == cursorCol && i == cur}
			if k != runKey {
				flush()
				runKey = k
			}
			run.WriteRune(l[c])
			width += runewidth.RuneWidth(l[c])
		}
		flush()
		// Cursor depois do fim da linha.
		if i == cur && cursorCol >= vr.seg[1] {
			b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(cursorBg)).Render(" "))
			width++
		}
		pad := strings.Repeat(" ", max(0, tw-width))
		if onCursorLine && th.CursorLine != "" {
			b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(th.CursorLine)).Render(pad))
		} else {
			b.WriteString(pad)
		}
		out = append(out, b.String())
	}
	for len(out) < h {
		out = append(out, gutter.Render(fmt.Sprintf("%*s ", gw-1, "~"))+strings.Repeat(" ", tw))
	}
	return strings.Join(out, "\n")
}

// Status é o que a barra de estado do editor mostra.
type Status struct {
	Mode    string // EDITAR, NORMAL, INSERIR, VISUAL...
	Insert  bool   // escrevendo (modo padrão ou inserção)
	Cmdline string // ":wq" ou "/busca" enquanto é digitado
	Pending string // operador ou número esperando o resto do comando
	Msg     string
	Row     int
	Col     int
	Lines   int
	Words   int
	Dirty   bool
}

func (e *Editor) Status() Status {
	s := Status{
		Row: e.row + 1, Col: e.col + 1, Lines: len(e.lines),
		Words: len(strings.Fields(e.Text())), Dirty: e.Dirty(), Msg: e.msg,
		Pending: e.count + e.pending,
	}
	switch {
	case !e.vim:
		s.Mode, s.Insert = t("EDITAR", "EDIT"), true
	case e.vs == vInsert:
		s.Mode, s.Insert = t("INSERIR", "INSERT"), true
	case e.vs == vVisual:
		s.Mode = "VISUAL"
	case e.vs == vVisualLine:
		s.Mode = t("V-LINHA", "V-LINE")
	case e.vs == vCommand:
		s.Mode, s.Cmdline = t("COMANDO", "COMMAND"), ":"+e.cmdline
	case e.vs == vSearch:
		s.Mode, s.Cmdline = t("BUSCA", "SEARCH"), "/"+e.cmdline
	default:
		s.Mode = "NORMAL"
	}
	return s
}

// Progress é a posição do cursor no texto, de 0 a 1 (para rolar a prévia).
func (e *Editor) Progress() float64 {
	if len(e.lines) <= 1 {
		return 0
	}
	return float64(e.row) / float64(len(e.lines)-1)
}
