package editor

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"questtui/internal/lang"
)

// Modo vim: normal, inserção, visual (de caracteres e de linhas), linha de
// comando (:w, :q, :wq, :q!, :número) e busca (/, n, N).

func t(pt, en string) string { return lang.T(pt, en) }

// takeCount devolve o número digitado antes do comando (1 se não houver).
func (e *Editor) takeCount() (n int, given bool) {
	if e.count == "" {
		return 1, false
	}
	n, _ = strconv.Atoi(e.count)
	e.count = ""
	return max(n, 1), true
}

// enterInsert entra no modo de inserção; a inserção inteira vira um passo de
// desfazer.
func (e *Editor) enterInsert() {
	e.push()
	e.vs = vInsert
	e.lastKind = ""
}

func (e *Editor) leaveInsert() {
	e.vs = vNormal
	// Uma inserção que não mudou nada não deixa passo de desfazer.
	if n := len(e.undo); n > 0 {
		s := e.undo[n-1]
		if len(s.lines) == len(e.lines) && slices.EqualFunc(s.lines, e.lines, slices.Equal[[]rune]) {
			e.undo = e.undo[:n-1]
		}
	}
	if e.col > 0 {
		e.col--
	}
	e.clampCol()
	e.want = e.col
}

func (e *Editor) updateInsert(k tea.KeyMsg) {
	switch k.String() {
	case "esc", "ctrl+[":
		e.leaveInsert()
	default:
		e.editKey(k)
	}
}

// motion calcula para onde um movimento leva o cursor. linewise diz se o
// movimento é de linhas inteiras (j, k, G...); inclusive, se o caractere de
// destino entra no trecho quando há um operador (e, $).
func (e *Editor) motion(s string, n int, given bool) (p Pos, linewise, inclusive, ok bool) {
	p = Pos{e.row, e.col}
	l := e.line()
	switch s {
	case "h", "left", "backspace":
		p.Col = max(0, p.Col-n)
	case "l", "right", " ":
		p.Col = min(len(l), p.Col+n)
	case "j", "down", "enter", "+":
		p.Row = min(len(e.lines)-1, p.Row+n)
		p.Col = e.want
		linewise = true
	case "k", "up", "-":
		p.Row = max(0, p.Row-n)
		p.Col = e.want
		linewise = true
	case "0", "home":
		p.Col = 0
	case "^":
		p.Col = firstNonBlank(l)
	case "$", "end":
		p.Col = max(0, len(l)-1)
		inclusive = true
	case "w", "W":
		for range n {
			p = e.wordForward(p)
		}
	case "b", "B":
		for range n {
			p = e.wordBack(p)
		}
	case "e", "E":
		for range n {
			p = e.wordEnd(p)
		}
		inclusive = true
	case "G":
		p.Row = len(e.lines) - 1
		if given {
			p.Row = min(n, len(e.lines)) - 1
		}
		p.Col = firstNonBlank(e.lines[p.Row])
		linewise = true
	case "}":
		for range n {
			p.Row = e.paragraph(p.Row, 1)
		}
		p.Col = 0
	case "{":
		for range n {
			p.Row = e.paragraph(p.Row, -1)
		}
		p.Col = 0
	case "pgdown", "ctrl+d":
		p.Row = min(len(e.lines)-1, p.Row+15)
		linewise = true
	case "pgup", "ctrl+u":
		p.Row = max(0, p.Row-15)
		linewise = true
	default:
		return p, false, false, false
	}
	return p, linewise, inclusive, true
}

// gotoLine leva ao começo da linha n (contando de 1).
func (e *Editor) gotoLine(n int) {
	e.row = max(0, min(n, len(e.lines))-1)
	e.col = firstNonBlank(e.line())
	e.want = e.col
}

func (e *Editor) moveTo(p Pos, vertical bool) {
	e.row, e.col = p.Row, p.Col
	e.clampCol()
	if !vertical {
		e.want = e.col
	}
}

func (e *Editor) updateNormal(k tea.KeyMsg) Action {
	s := k.String()

	if e.pending == "r" {
		e.pending = ""
		if k.Type == tea.KeyRunes && len(k.Runes) == 1 && len(e.line()) > 0 {
			e.checkpoint("edit")
			e.lines[e.row][e.col] = k.Runes[0]
		}
		return None
	}
	if k.Type == tea.KeyRunes && len(k.Runes) == 1 && !k.Alt {
		if r := k.Runes[0]; (r >= '1' && r <= '9') || (r == '0' && e.count != "") {
			e.count += string(r)
			return None
		}
	}
	n, given := e.takeCount()
	if e.pending != "" {
		e.operator(s, n, given)
		return None
	}

	if p, linewise, _, ok := e.motion(s, n, given); ok {
		e.lastKind = ""
		e.moveTo(p, linewise && s != "G")
		return None
	}

	l := e.line()
	switch s {
	case "esc":
		e.count = ""
	case "g", "d", "c", "y", ">", "<":
		e.pending, e.opCount = s, n
	case "r":
		e.pending = "r"
	case "x", "delete":
		if len(l) > 0 {
			e.checkpoint("edit")
			end := min(len(l), e.col+n)
			e.reg, e.regLine = string(l[e.col:end]), false
			e.lines[e.row] = slices.Delete(slices.Clone(l), e.col, end)
			e.clampCol()
		}
	case "X":
		if e.col > 0 {
			e.checkpoint("edit")
			start := max(0, e.col-n)
			e.reg, e.regLine = string(l[start:e.col]), false
			e.lines[e.row] = slices.Delete(slices.Clone(l), start, e.col)
			e.col = start
		}
	case "D", "C":
		e.checkpoint("edit")
		e.reg, e.regLine = string(l[e.col:]), false
		e.lines[e.row] = slices.Clone(l[:e.col])
		if s == "C" {
			e.vs, e.lastKind = vInsert, ""
		}
		e.clampCol()
	case "s":
		e.enterInsert()
		if len(l) > 0 {
			end := min(len(l), e.col+n)
			e.reg, e.regLine = string(l[e.col:end]), false
			e.lines[e.row] = slices.Delete(slices.Clone(l), e.col, end)
		}
	case "S":
		e.changeLines(e.row, min(len(e.lines)-1, e.row+n-1))
	case "Y":
		e.yankLines(e.row, min(len(e.lines)-1, e.row+n-1))
	case "p", "P":
		e.put(s == "p", n)
	case "J":
		e.checkpoint("edit")
		for range max(1, n) {
			e.join()
		}
	case "~":
		if len(l) > 0 {
			e.checkpoint("edit")
			for i := e.col; i < min(len(l), e.col+n); i++ {
				r := l[i]
				if unicode.IsUpper(r) {
					l[i] = unicode.ToLower(r)
				} else {
					l[i] = unicode.ToUpper(r)
				}
			}
			e.col = min(len(l)-1, e.col+n)
		}
	case "u", "ctrl+z":
		for range n {
			e.doUndo()
		}
	case "ctrl+r", "ctrl+y":
		for range n {
			e.doRedo()
		}
	case "i":
		e.enterInsert()
	case "a":
		e.enterInsert()
		e.col = min(len(l), e.col+1)
	case "I":
		e.enterInsert()
		e.col = firstNonBlank(l)
	case "A":
		e.enterInsert()
		e.col = len(l)
	case "o", "O":
		e.enterInsert()
		ind := indent.FindString(string(l))
		at := e.row + 1
		if s == "O" {
			at = e.row
		}
		e.lines = slices.Insert(e.lines, at, []rune(ind))
		e.row, e.col = at, len([]rune(ind))
	case "v":
		e.vs, e.anchor = vVisual, Pos{e.row, e.col}
	case "V":
		e.vs, e.anchor = vVisualLine, Pos{e.row, e.col}
	case ":":
		e.vs, e.cmdline = vCommand, ""
	case "/":
		e.vs, e.cmdline = vSearch, ""
	case "n":
		e.find(e.search, true)
	case "N":
		e.find(e.search, false)
	}
	return None
}

// operator completa um operador pendente (d, c, y, >, <, g) com o movimento.
func (e *Editor) operator(s string, n int, given bool) {
	op := e.pending
	n *= e.opCount
	given = given || e.opCount > 1
	e.pending, e.opCount = "", 0

	if op == "g" {
		if s == "g" {
			e.gotoLine(map[bool]int{true: n, false: 1}[given])
		}
		return
	}
	if s == "esc" {
		e.opG = false
		return
	}
	cur := Pos{e.row, e.col}
	last := len(e.lines) - 1

	// dd, cc, yy, >>, <<: linhas a partir da atual.
	if s == op {
		e.applyLines(op, e.row, min(last, e.row+n-1))
		return
	}
	// dgg / ygg: o segundo g chega depois.
	if s == "g" && !e.opG {
		e.pending, e.opCount, e.opG = op, n, true
		return
	}
	var p Pos
	var linewise, inclusive, ok bool
	if e.opG {
		e.opG = false
		p, linewise, ok = Pos{0, 0}, true, true
		if given {
			p.Row = min(n, len(e.lines)) - 1
		}
	} else {
		ms := s
		// cw troca até o fim da palavra, como o vim faz.
		if op == "c" && (s == "w" || s == "W") && e.classAt(cur) != 0 {
			ms = "e"
		}
		p, linewise, inclusive, ok = e.motion(ms, n, given)
		if ok && (ms == "w" || ms == "W") && p.Row > cur.Row {
			p = Pos{cur.Row, len(e.lines[cur.Row])} // dw no fim da linha não junta linhas
		}
	}
	if !ok {
		return
	}
	if linewise || op == ">" || op == "<" {
		e.applyLines(op, min(cur.Row, p.Row), max(cur.Row, p.Row))
		return
	}
	a, b := cur, p
	if b.before(a) {
		a, b = b, a
	}
	if inclusive {
		b.Col = min(len(e.lines[b.Row]), b.Col+1)
	}
	switch op {
	case "y":
		e.reg, e.regLine = e.textRange(a, b), false
		e.moveTo(a, false)
	case "d":
		e.checkpoint("edit")
		e.reg, e.regLine = e.deleteRange(a, b), false
		e.clampCol()
		e.want = e.col
	case "c":
		e.enterInsert()
		e.reg, e.regLine = e.deleteRange(a, b), false
	}
}

// applyLines aplica um operador a linhas inteiras.
func (e *Editor) applyLines(op string, a, b int) {
	switch op {
	case "d":
		e.checkpoint("edit")
		e.deleteLines(a, b)
	case "y":
		e.yankLines(a, b)
	case "c":
		e.changeLines(a, b)
	case ">":
		e.checkpoint("edit")
		e.indentRows(a, b)
		e.clampCol()
	case "<":
		e.checkpoint("edit")
		e.outdentRows(a, b)
		e.clampCol()
	}
}

// changeLines troca as linhas por uma linha vazia (mantendo o recuo) e entra
// na inserção.
func (e *Editor) changeLines(a, b int) {
	e.enterInsert()
	ind := indent.FindString(string(e.lines[a]))
	e.yankLines(a, b)
	e.lines = slices.Delete(e.lines, a+1, b+1)
	e.lines[a] = []rune(ind)
	e.row, e.col = a, len([]rune(ind))
}

// put cola depois (p) ou antes (P) do cursor.
func (e *Editor) put(after bool, n int) {
	if e.reg == "" {
		return
	}
	e.checkpoint("edit")
	text := strings.Repeat(e.reg+"\n", n)
	text = text[:len(text)-1]
	if e.regLine {
		at := e.row
		if after {
			at++
		}
		for i, l := range strings.Split(text, "\n") {
			e.lines = slices.Insert(e.lines, at+i, []rune(l))
		}
		e.row = at
		e.col = firstNonBlank(e.line())
		return
	}
	if after && len(e.line()) > 0 {
		e.col++
	}
	e.insertText(text)
	e.col--
	e.clampCol()
}

// join junta a próxima linha à atual, com um espaço entre elas.
func (e *Editor) join() {
	if e.row+1 >= len(e.lines) {
		return
	}
	a := []rune(strings.TrimRight(string(e.line()), " "))
	b := []rune(strings.TrimLeft(string(e.lines[e.row+1]), " "))
	sep := []rune{}
	if len(a) > 0 && len(b) > 0 {
		sep = []rune{' '}
	}
	e.col = len(a)
	e.lines[e.row] = slices.Concat(a, sep, b)
	e.lines = slices.Delete(e.lines, e.row+1, e.row+2)
	e.clampCol()
}

// selection devolve o trecho selecionado no modo visual, de a até b
// (excluído).
func (e *Editor) selection() (a, b Pos) {
	a, b = e.anchor, Pos{e.row, e.col}
	if b.before(a) {
		a, b = b, a
	}
	b.Col = min(len(e.lines[b.Row]), b.Col+1)
	return a, b
}

func (e *Editor) selectedRows() (int, int) {
	return min(e.anchor.Row, e.row), max(e.anchor.Row, e.row)
}

func (e *Editor) updateVisual(k tea.KeyMsg) {
	s := k.String()
	if k.Type == tea.KeyRunes && len(k.Runes) == 1 && !k.Alt {
		if r := k.Runes[0]; (r >= '1' && r <= '9') || (r == '0' && e.count != "") {
			e.count += string(r)
			return
		}
	}
	n, given := e.takeCount()
	if e.pending == "g" {
		e.pending = ""
		if s == "g" {
			e.gotoLine(map[bool]int{true: n, false: 1}[given])
		}
		return
	}
	if p, linewise, _, ok := e.motion(s, n, given); ok {
		e.moveTo(p, linewise)
		return
	}
	lines := e.vs == vVisualLine
	switch s {
	case "esc", "ctrl+[":
		e.vs = vNormal
	case "v":
		e.vs = map[bool]vimState{true: vVisual, false: vNormal}[lines]
	case "V":
		e.vs = map[bool]vimState{true: vNormal, false: vVisualLine}[lines]
	case "o":
		e.anchor, e.row, e.col = Pos{e.row, e.col}, e.anchor.Row, e.anchor.Col
	case "g":
		e.pending = "g"
	case "d", "x", "y", "c", "s":
		if lines {
			a, b := e.selectedRows()
			e.vs = vNormal
			op := map[string]string{"d": "d", "x": "d", "y": "y", "c": "c", "s": "c"}[s]
			e.applyLines(op, a, b)
			return
		}
		a, b := e.selection()
		e.vs = vNormal
		switch s {
		case "y":
			e.reg, e.regLine = e.textRange(a, b), false
			e.moveTo(a, false)
		case "d", "x":
			e.checkpoint("edit")
			e.reg, e.regLine = e.deleteRange(a, b), false
			e.clampCol()
		case "c", "s":
			e.enterInsert()
			e.reg, e.regLine = e.deleteRange(a, b), false
		}
	case ">", "<":
		a, b := e.selectedRows()
		e.vs = vNormal
		e.applyLines(s, a, b)
	case "J":
		a, b := e.selectedRows()
		e.vs = vNormal
		e.checkpoint("edit")
		e.row = a
		for range max(1, b-a) {
			e.join()
		}
	}
}

func (e *Editor) updateCmdline(k tea.KeyMsg) Action {
	switch k.String() {
	case "esc", "ctrl+[", "ctrl+c":
		e.vs = vNormal
	case "backspace":
		r := []rune(e.cmdline)
		if len(r) == 0 {
			e.vs = vNormal
		} else {
			e.cmdline = string(r[:len(r)-1])
		}
	case "enter":
		search := e.vs == vSearch
		e.vs = vNormal
		if search {
			if e.cmdline != "" {
				e.search = e.cmdline
			}
			e.find(e.search, true)
			return None
		}
		return e.exec(strings.TrimSpace(e.cmdline))
	case " ":
		e.cmdline += " "
	default:
		if k.Type == tea.KeyRunes && !k.Alt {
			e.cmdline += string(k.Runes)
		}
	}
	return None
}

// exec roda um comando da linha ":".
func (e *Editor) exec(cmd string) Action {
	switch cmd {
	case "":
		return None
	case "w":
		return Save
	case "wq", "x", "wq!", "x!":
		return SaveClose
	case "q":
		if e.Dirty() {
			e.msg = t("há alterações não salvas (:w grava, :q! descarta)", "unsaved changes (:w writes, :q! discards)")
			return None
		}
		return DiscardClose
	case "q!":
		return DiscardClose
	}
	if n, err := strconv.Atoi(cmd); err == nil {
		e.gotoLine(n)
		return None
	}
	e.msg = fmt.Sprintf(t("comando desconhecido: %s", "unknown command: %s"), cmd)
	return None
}

// find busca o texto para frente ou para trás a partir do cursor, dando a
// volta no fim. Só diferencia maiúsculas se o termo tiver alguma.
func (e *Editor) find(term string, forward bool) {
	if term == "" {
		return
	}
	pat := []rune(term)
	fold := strings.ToLower(term) == term
	norm := func(r []rune) []rune {
		if !fold {
			return r
		}
		return []rune(strings.ToLower(string(r)))
	}
	pat = norm(pat)
	match := func(l []rune, i int) bool {
		return i+len(pat) <= len(l) && slices.Equal(norm(l[i:i+len(pat)]), pat)
	}
	total := len(e.lines)
	for k := 0; k <= total; k++ {
		var r int
		if forward {
			r = (e.row + k) % total
		} else {
			r = ((e.row-k)%total + total) % total
		}
		l := e.lines[r]
		if forward {
			start := 0
			if k == 0 {
				start = e.col + 1
			}
			for i := start; i < len(l); i++ {
				if match(l, i) {
					e.moveTo(Pos{r, i}, false)
					return
				}
			}
		} else {
			end := len(l) - 1
			if k == 0 {
				end = e.col - 1
			}
			for i := end; i >= 0; i-- {
				if match(l, i) {
					e.moveTo(Pos{r, i}, false)
					return
				}
			}
		}
	}
	e.msg = fmt.Sprintf(t("não encontrado: %s", "not found: %s"), term)
}
