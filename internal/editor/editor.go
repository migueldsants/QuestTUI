// Package editor é o editor de texto das notas: markdown com realce,
// continuação de listas, atalhos de formatação, desfazer e um modo vim.
package editor

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// Pos é uma posição no texto: linha e coluna, em runas.
type Pos struct{ Row, Col int }

func (a Pos) before(b Pos) bool { return a.Row < b.Row || (a.Row == b.Row && a.Col < b.Col) }

// Action é o que o editor pede para quem o usa.
type Action int

const (
	None         Action = iota
	Save                // gravar e continuar editando
	SaveClose           // gravar e fechar
	DiscardClose        // fechar sem gravar
)

type snapshot struct {
	lines [][]rune
	row   int
	col   int
}

type vimState int

const (
	vNormal vimState = iota
	vInsert
	vVisual
	vVisualLine
	vCommand // linha de comando ":"
	vSearch  // busca "/"
)

const maxUndo = 300

type Editor struct {
	lines    [][]rune
	row, col int
	want     int // coluna desejada ao subir e descer
	top      int // primeira linha visual na tela

	vim     bool
	vs      vimState
	pending string // operador esperando o movimento: d, c, y, g, r, >, <
	opCount int
	opG     bool // "dg" esperando o segundo g de "dgg"
	count   string
	anchor  Pos    // início da seleção visual
	cmdline string // texto da linha de comando ou da busca
	search  string

	reg      string // área de transferência interna
	regLine  bool   // o conteúdo é de linhas inteiras
	lastKill bool   // o último comando foi recortar linha (^K): o próximo acumula

	undo, redo []snapshot
	lastKind   string
	lastRow    int

	saved string
	msg   string
	theme Theme
}

// New cria o editor com o texto dado.
func New(text string, vim bool) *Editor {
	e := &Editor{vim: vim}
	e.SetText(text)
	e.saved = e.Text()
	return e
}

// SetText troca todo o texto (tabs viram quatro espaços).
func (e *Editor) SetText(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	parts := strings.Split(s, "\n")
	e.lines = make([][]rune, len(parts))
	for i, p := range parts {
		e.lines[i] = []rune(p)
	}
	e.row = min(e.row, len(e.lines)-1)
	e.clampCol()
}

// Text devolve o texto atual.
func (e *Editor) Text() string {
	parts := make([]string, len(e.lines))
	for i, l := range e.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

func (e *Editor) Dirty() bool { return e.Text() != e.saved }
func (e *Editor) MarkSaved()  { e.saved = e.Text() }
func (e *Editor) Vim() bool   { return e.vim }

// SetVim liga ou desliga o modo vim; ao ligar, começa no modo normal.
func (e *Editor) SetVim(on bool) {
	e.vim, e.vs, e.pending, e.count = on, vNormal, "", ""
	e.clampCol()
}

// ReplaceText troca o texto guardando um ponto de desfazer (usado quando o
// texto volta de um editor externo).
func (e *Editor) ReplaceText(s string) {
	e.push()
	e.SetText(s)
}

func (e *Editor) normal() bool {
	return e.vim && (e.vs == vNormal || e.vs == vVisual || e.vs == vVisualLine)
}

// clampCol mantém o cursor dentro da linha. No modo normal do vim o cursor
// fica sobre um caractere; nos outros pode ficar depois do último.
func (e *Editor) clampCol() {
	e.row = max(0, min(e.row, len(e.lines)-1))
	limit := len(e.lines[e.row])
	if e.normal() && limit > 0 {
		limit--
	}
	e.col = max(0, min(e.col, limit))
}

func (e *Editor) line() []rune { return e.lines[e.row] }

// --- desfazer ---

func (e *Editor) snap() snapshot {
	lines := make([][]rune, len(e.lines))
	for i, l := range e.lines {
		lines[i] = slices.Clone(l)
	}
	return snapshot{lines, e.row, e.col}
}

func (e *Editor) restore(s snapshot) {
	e.lines, e.row, e.col = s.lines, s.row, s.col
	e.clampCol()
}

// push guarda o estado atual para poder desfazer.
func (e *Editor) push() {
	e.undo = append(e.undo, e.snap())
	if len(e.undo) > maxUndo {
		e.undo = e.undo[1:]
	}
	e.redo = nil
}

// checkpoint guarda um ponto de desfazer antes de uma alteração. Digitação e
// apagamento seguidos na mesma linha viram um passo só; no modo de inserção
// do vim, a inserção inteira é um passo (guardado ao entrar nela).
func (e *Editor) checkpoint(kind string) {
	if e.vim && e.vs == vInsert {
		return
	}
	if (kind == "type" || kind == "del") && kind == e.lastKind && e.row == e.lastRow {
		return
	}
	e.push()
	e.lastKind, e.lastRow = kind, e.row
}

func (e *Editor) doUndo() {
	if len(e.undo) == 0 {
		e.msg = t("nada para desfazer", "nothing to undo")
		return
	}
	e.redo = append(e.redo, e.snap())
	e.restore(e.undo[len(e.undo)-1])
	e.undo = e.undo[:len(e.undo)-1]
	e.lastKind = ""
}

func (e *Editor) doRedo() {
	if len(e.redo) == 0 {
		e.msg = t("nada para refazer", "nothing to redo")
		return
	}
	e.undo = append(e.undo, e.snap())
	e.restore(e.redo[len(e.redo)-1])
	e.redo = e.redo[:len(e.redo)-1]
	e.lastKind = ""
}

// --- edição básica ---

// insertText insere o texto no cursor; quebras de linha viram linhas novas.
func (e *Editor) insertText(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	for i, part := range strings.Split(s, "\n") {
		if i > 0 {
			e.splitLine("")
		}
		r := []rune(part)
		l := e.line()
		e.lines[e.row] = slices.Concat(l[:e.col], r, l[e.col:])
		e.col += len(r)
	}
}

// splitLine quebra a linha no cursor; a linha nova começa com prefix.
func (e *Editor) splitLine(prefix string) {
	l := e.line()
	rest := slices.Concat([]rune(prefix), l[e.col:])
	e.lines[e.row] = slices.Clone(l[:e.col])
	e.lines = slices.Insert(e.lines, e.row+1, rest)
	e.row++
	e.col = len([]rune(prefix))
}

var (
	listRe  = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])(\s+)(\[[ xX]\]\s+)?`)
	quoteRe = regexp.MustCompile(`^(\s*)>\s?`)
	indent  = regexp.MustCompile(`^\s*`)
)

// enter quebra a linha continuando listas, tarefas e citações. Num item
// vazio, termina a lista.
func (e *Editor) enter() {
	l := string(e.line())
	if m := listRe.FindStringSubmatch(l); m != nil && e.col >= len([]rune(m[0])) {
		if strings.TrimSpace(l[len(m[0]):]) == "" {
			e.lines[e.row], e.col = []rune{}, 0
			return
		}
		marker := m[2]
		if n, err := strconv.Atoi(strings.TrimRight(marker, ".)")); err == nil {
			marker = strconv.Itoa(n+1) + marker[len(marker)-1:]
		}
		prefix := m[1] + marker + " "
		if m[4] != "" {
			prefix += "[ ] "
		}
		e.splitLine(prefix)
		return
	}
	if m := quoteRe.FindString(l); m != "" && e.col >= len([]rune(m)) {
		if strings.TrimSpace(l[len(m):]) == "" {
			e.lines[e.row], e.col = []rune{}, 0
			return
		}
		e.splitLine(strings.TrimRight(m, " ") + " ")
		return
	}
	ind := indent.FindString(l)
	if e.col < len([]rune(ind)) {
		ind = ""
	}
	e.splitLine(ind)
}

func (e *Editor) backspace() {
	if e.col > 0 {
		l := e.line()
		e.lines[e.row] = slices.Delete(slices.Clone(l), e.col-1, e.col)
		e.col--
		return
	}
	if e.row == 0 {
		return
	}
	prev := e.lines[e.row-1]
	e.col = len(prev)
	e.lines[e.row-1] = slices.Concat(prev, e.line())
	e.lines = slices.Delete(e.lines, e.row, e.row+1)
	e.row--
}

func (e *Editor) deleteForward() {
	l := e.line()
	if e.col < len(l) {
		e.lines[e.row] = slices.Delete(slices.Clone(l), e.col, e.col+1)
		return
	}
	if e.row+1 < len(e.lines) {
		e.lines[e.row] = slices.Concat(l, e.lines[e.row+1])
		e.lines = slices.Delete(e.lines, e.row+1, e.row+2)
	}
}

// deleteWordBack apaga a palavra antes do cursor (^W).
func (e *Editor) deleteWordBack() {
	if e.col == 0 {
		e.backspace()
		return
	}
	l := e.line()
	i := e.col
	for i > 0 && unicode.IsSpace(l[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(l[i-1]) {
		i--
	}
	e.lines[e.row] = slices.Concat(l[:i], l[e.col:])
	e.col = i
}

// tab recua um item de lista para dentro; fora de listas insere espaços.
func (e *Editor) tab() {
	if listRe.MatchString(string(e.line())) {
		e.lines[e.row] = slices.Concat([]rune("  "), e.line())
		e.col += 2
		return
	}
	e.insertText("  ")
}

func (e *Editor) untab() { e.outdentRows(e.row, e.row) }

func (e *Editor) indentRows(a, b int) {
	for r := a; r <= b; r++ {
		if len(e.lines[r]) > 0 {
			e.lines[r] = slices.Concat([]rune("  "), e.lines[r])
		}
	}
	if e.row >= a && e.row <= b && len(e.line()) > 0 {
		e.col += 2
	}
}

func (e *Editor) outdentRows(a, b int) {
	for r := a; r <= b; r++ {
		n := 0
		for n < 2 && n < len(e.lines[r]) && e.lines[r][n] == ' ' {
			n++
		}
		e.lines[r] = e.lines[r][n:]
		if r == e.row {
			e.col = max(0, e.col-n)
		}
	}
}

// killLine recorta a linha atual (^K); recortes seguidos se acumulam.
func (e *Editor) killLine() {
	text := string(e.line())
	if e.lastKill && e.regLine {
		e.reg += "\n" + text
	} else {
		e.reg, e.regLine = text, true
	}
	if len(e.lines) == 1 {
		e.lines[0] = []rune{}
	} else {
		e.lines = slices.Delete(e.lines, e.row, e.row+1)
	}
	e.row = min(e.row, len(e.lines)-1)
	e.col = 0
}

// paste cola o que foi recortado (^U): linhas inteiras entram acima da linha
// atual; um trecho entra no cursor.
func (e *Editor) paste() {
	if e.reg == "" {
		return
	}
	if !e.regLine {
		e.insertText(e.reg)
		return
	}
	for i, l := range strings.Split(e.reg, "\n") {
		e.lines = slices.Insert(e.lines, e.row+i, []rune(l))
	}
	e.row += strings.Count(e.reg, "\n") + 1
	e.row = min(e.row, len(e.lines)-1)
	e.col = 0
}

// --- movimento ---

func (e *Editor) setCol(c int) {
	e.col = c
	e.clampCol()
	e.want = e.col
}

func (e *Editor) vertical(d int) {
	e.row = max(0, min(e.row+d, len(e.lines)-1))
	e.col = e.want
	e.clampCol()
}

func firstNonBlank(l []rune) int {
	for i, r := range l {
		if !unicode.IsSpace(r) {
			return i
		}
	}
	return 0
}

// charAt devolve o caractere na posição; o fim de cada linha conta como uma
// quebra de linha. ok é false depois do fim do texto.
func (e *Editor) charAt(p Pos) (rune, bool) {
	l := e.lines[p.Row]
	if p.Col < len(l) {
		return l[p.Col], true
	}
	return '\n', p.Row < len(e.lines)-1
}

func (e *Editor) next(p Pos) (Pos, bool) {
	if p.Col < len(e.lines[p.Row]) {
		return Pos{p.Row, p.Col + 1}, true
	}
	if p.Row+1 < len(e.lines) {
		return Pos{p.Row + 1, 0}, true
	}
	return p, false
}

func (e *Editor) prev(p Pos) (Pos, bool) {
	if p.Col > 0 {
		return Pos{p.Row, p.Col - 1}, true
	}
	if p.Row > 0 {
		return Pos{p.Row - 1, len(e.lines[p.Row-1])}, true
	}
	return p, false
}

func class(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
		return 1
	}
	return 2
}

func (e *Editor) classAt(p Pos) int {
	r, _ := e.charAt(p)
	return class(r)
}

func (e *Editor) emptyLineAt(p Pos) bool { return p.Col == 0 && len(e.lines[p.Row]) == 0 }

// wordForward vai para o início da próxima palavra (w).
func (e *Editor) wordForward(p Pos) Pos {
	q := p
	if c := e.classAt(q); c != 0 {
		for e.classAt(q) == c {
			n, ok := e.next(q)
			if !ok {
				return q
			}
			q = n
		}
	}
	for e.classAt(q) == 0 {
		if q != p && e.emptyLineAt(q) {
			return q
		}
		n, ok := e.next(q)
		if !ok {
			return q
		}
		q = n
	}
	return q
}

// wordEnd vai para o fim da palavra (e).
func (e *Editor) wordEnd(p Pos) Pos {
	q, ok := e.next(p)
	if !ok {
		return p
	}
	for e.classAt(q) == 0 {
		if q, ok = e.next(q); !ok {
			return q
		}
	}
	c := e.classAt(q)
	for {
		n, ok := e.next(q)
		if !ok || e.classAt(n) != c {
			return q
		}
		q = n
	}
}

// wordBack volta para o início da palavra (b).
func (e *Editor) wordBack(p Pos) Pos {
	q, ok := e.prev(p)
	if !ok {
		return p
	}
	for e.classAt(q) == 0 {
		if e.emptyLineAt(q) {
			return q
		}
		if q, ok = e.prev(q); !ok {
			return q
		}
	}
	c := e.classAt(q)
	for {
		n, ok := e.prev(q)
		if !ok || e.classAt(n) != c {
			return q
		}
		q = n
	}
}

// paragraph vai para a próxima (d=1) ou anterior (d=-1) linha vazia.
func (e *Editor) paragraph(row, d int) int {
	r := row + d
	for r > 0 && r < len(e.lines)-1 && len(strings.TrimSpace(string(e.lines[r]))) > 0 {
		r += d
	}
	return max(0, min(r, len(e.lines)-1))
}

// --- trechos ---

// textRange devolve o texto de a (incluído) até b (excluído).
func (e *Editor) textRange(a, b Pos) string {
	if a.Row == b.Row {
		return string(e.lines[a.Row][a.Col:b.Col])
	}
	parts := []string{string(e.lines[a.Row][a.Col:])}
	for r := a.Row + 1; r < b.Row; r++ {
		parts = append(parts, string(e.lines[r]))
	}
	parts = append(parts, string(e.lines[b.Row][:b.Col]))
	return strings.Join(parts, "\n")
}

// deleteRange apaga de a (incluído) até b (excluído) e devolve o texto.
func (e *Editor) deleteRange(a, b Pos) string {
	text := e.textRange(a, b)
	merged := slices.Concat(e.lines[a.Row][:a.Col], e.lines[b.Row][b.Col:])
	e.lines = slices.Delete(e.lines, a.Row+1, b.Row+1)
	e.lines[a.Row] = merged
	e.row, e.col = a.Row, a.Col
	return text
}

func (e *Editor) yankLines(a, b int) {
	parts := make([]string, 0, b-a+1)
	for r := a; r <= b; r++ {
		parts = append(parts, string(e.lines[r]))
	}
	e.reg, e.regLine = strings.Join(parts, "\n"), true
}

func (e *Editor) deleteLines(a, b int) {
	e.yankLines(a, b)
	e.lines = slices.Delete(e.lines, a, b+1)
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	e.row = min(a, len(e.lines)-1)
	e.col = firstNonBlank(e.line())
}

// --- atalhos de markdown ---

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

// wordBounds devolve a palavra no cursor (ou logo antes dele).
func (e *Editor) wordBounds() (s, t int, ok bool) {
	l := e.line()
	c := e.col
	if c >= len(l) || !isWordRune(l[c]) {
		if c > 0 && c <= len(l) && isWordRune(l[c-1]) {
			c--
		} else {
			return 0, 0, false
		}
	}
	s, t = c, c
	for s > 0 && isWordRune(l[s-1]) {
		s--
	}
	for t < len(l) && isWordRune(l[t]) {
		t++
	}
	return s, t, true
}

func around(l []rune, s, t int, m string) bool {
	mr := []rune(m)
	return s >= len(mr) && t+len(mr) <= len(l) &&
		string(l[s-len(mr):s]) == m && string(l[t:t+len(mr)]) == m
}

// wrap envolve a seleção ou a palavra no cursor com o marcador (negrito,
// itálico, código); se ela já estiver envolvida, tira o marcador. Sem
// palavra, insere o par e põe o cursor no meio.
func (e *Editor) wrap(m string) {
	e.checkpoint("md")
	mr := []rune(m)
	if e.vs == vVisual {
		a, b := e.selection()
		if a.Row == b.Row {
			l := e.lines[a.Row]
			e.lines[a.Row] = slices.Concat(l[:a.Col], mr, l[a.Col:b.Col], mr, l[b.Col:])
			e.row, e.col, e.vs = a.Row, a.Col+len(mr), vNormal
			e.clampCol()
			return
		}
		e.vs = vNormal
	}
	l := e.line()
	s, t, ok := e.wordBounds()
	if !ok {
		e.lines[e.row] = slices.Concat(l[:e.col], mr, mr, l[e.col:])
		e.col += len(mr)
		if e.vim && e.vs == vNormal {
			e.vs = vInsert
		}
		return
	}
	bold := m == "*" && around(l, s, t, "**") && !around(l, s, t, "***")
	if around(l, s, t, m) && !bold {
		e.lines[e.row] = slices.Concat(l[:s-len(mr)], l[s:t], l[t+len(mr):])
		e.col = max(0, e.col-len(mr))
	} else {
		e.lines[e.row] = slices.Concat(l[:s], mr, l[s:t], mr, l[t:])
		e.col += len(mr)
	}
	e.clampCol()
}

// link transforma a palavra em [palavra]() com o cursor dentro dos
// parênteses; sem palavra, insere [](), com o cursor nos colchetes.
func (e *Editor) link() {
	e.checkpoint("md")
	l := e.line()
	if s, t, ok := e.wordBounds(); ok {
		e.lines[e.row] = slices.Concat(l[:s], []rune("["), l[s:t], []rune("]()"), l[t:])
		e.col = t + 3
	} else {
		e.lines[e.row] = slices.Concat(l[:e.col], []rune("[]()"), l[e.col:])
		e.col++
	}
	if e.vim && e.vs == vNormal {
		e.vs = vInsert
	}
}

// setPrefix troca o começo da linha (de 0 até old runas) por repl e acerta o
// cursor.
func (e *Editor) setPrefix(old int, repl string) {
	l := e.line()
	r := []rune(repl)
	e.lines[e.row] = slices.Concat(r, l[old:])
	if e.col >= old {
		e.col += len(r) - old
	} else {
		e.col = min(e.col, len(r))
	}
	e.clampCol()
}

var headingRe = regexp.MustCompile(`^#{1,6}\s+`)

// heading liga o título do nível n na linha, ou tira se já for desse nível.
func (e *Editor) heading(n int) {
	e.checkpoint("md")
	l := string(e.line())
	old := headingRe.FindString(l)
	level := strings.Count(strings.TrimSpace(old), "#")
	if level == n {
		e.setPrefix(len([]rune(old)), "")
		return
	}
	e.setPrefix(len([]rune(old)), strings.Repeat("#", n)+" ")
}

// toggleList: linha comum vira item; tarefa vira item; item volta a ser linha.
func (e *Editor) toggleList() {
	e.checkpoint("md")
	l := string(e.line())
	if m := listRe.FindStringSubmatch(l); m != nil {
		if m[4] != "" {
			e.setPrefix(len([]rune(m[0])), m[1]+m[2]+m[3])
		} else {
			e.setPrefix(len([]rune(m[0])), m[1])
		}
		return
	}
	ind := indent.FindString(l)
	e.setPrefix(len([]rune(ind)), ind+"- ")
}

// toggleTask: tarefa aberta ↔ feita; item vira tarefa; linha vira tarefa.
func (e *Editor) toggleTask() {
	e.checkpoint("md")
	l := string(e.line())
	if m := listRe.FindStringSubmatch(l); m != nil {
		box := "[ ] "
		switch strings.TrimSpace(m[4]) {
		case "[ ]":
			box = "[x] "
		case "[x]", "[X]":
			box = "[ ] "
		}
		e.setPrefix(len([]rune(m[0])), m[1]+m[2]+m[3]+box)
		return
	}
	ind := indent.FindString(l)
	e.setPrefix(len([]rune(ind)), ind+"- [ ] ")
}

func (e *Editor) toggleQuote() {
	e.checkpoint("md")
	l := string(e.line())
	if m := quoteRe.FindStringSubmatch(l); m != nil {
		e.setPrefix(len([]rune(m[0])), m[1])
		return
	}
	ind := indent.FindString(l)
	e.setPrefix(len([]rune(ind)), ind+"> ")
}

// markdownKey trata os atalhos de formatação (alt+letra), em qualquer modo.
func (e *Editor) markdownKey(k tea.KeyMsg) bool {
	switch k.String() {
	case "alt+b":
		e.wrap("**")
	case "alt+i":
		e.wrap("*")
	case "alt+c", "alt+`":
		e.wrap("`")
	case "alt+s":
		e.wrap("~~")
	case "alt+k":
		e.link()
	case "alt+1", "alt+2", "alt+3":
		e.heading(int(k.Runes[0] - '0'))
	case "alt+l":
		e.toggleList()
	case "alt+t":
		e.toggleTask()
	case "alt+q":
		e.toggleQuote()
	default:
		return false
	}
	e.want = e.col
	return true
}

// --- teclas ---

// Update trata uma tecla e devolve o que o editor pede para quem o usa.
func (e *Editor) Update(k tea.KeyMsg) Action {
	// Teclas digitadas rápido podem chegar juntas numa mensagem só ("dd",
	// ":wq"); cada uma é tratada como uma tecla.
	if k.Type == tea.KeyRunes && len(k.Runes) > 1 && !k.Paste {
		act := None
		for _, r := range k.Runes {
			if a := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: k.Alt}); a != None {
				act = a
			}
		}
		return act
	}
	e.msg = ""
	if k.Paste {
		e.checkpoint("paste")
		if e.normal() && len(e.line()) > 0 {
			e.col++ // no modo normal, cola depois do cursor
		}
		e.insertText(string(k.Runes))
		e.clampCol()
		return None
	}
	if k.Alt && e.markdownKey(k) {
		return None
	}
	if !e.vim {
		e.updateStandard(k)
		return None
	}
	switch e.vs {
	case vInsert:
		e.updateInsert(k)
		return None
	case vCommand, vSearch:
		return e.updateCmdline(k)
	case vVisual, vVisualLine:
		e.updateVisual(k)
		return None
	}
	return e.updateNormal(k)
}

// editKey trata as teclas de escrita, comuns ao modo padrão e à inserção.
func (e *Editor) editKey(k tea.KeyMsg) bool {
	kill := false
	switch k.String() {
	case "enter":
		e.checkpoint("enter")
		e.enter()
	case "backspace":
		e.checkpoint("del")
		e.backspace()
	case "delete":
		e.checkpoint("del")
		e.deleteForward()
	case "ctrl+w":
		e.checkpoint("del")
		e.deleteWordBack()
	case "tab":
		e.checkpoint("indent")
		e.tab()
	case "shift+tab":
		e.checkpoint("indent")
		e.untab()
	case "left":
		e.lastKind = ""
		if e.col > 0 {
			e.col--
		} else if e.row > 0 {
			e.row--
			e.col = len(e.line())
		}
	case "right":
		e.lastKind = ""
		if e.col < len(e.line()) {
			e.col++
		} else if e.row+1 < len(e.lines) {
			e.row++
			e.col = 0
		}
	case "up":
		e.lastKind = ""
		e.vertical(-1)
		return true
	case "down":
		e.lastKind = ""
		e.vertical(1)
		return true
	case "pgup":
		e.vertical(-15)
		return true
	case "pgdown":
		e.vertical(15)
		return true
	case "home":
		e.col = 0
	case "end":
		e.col = len(e.line())
	case "ctrl+home":
		e.row, e.col = 0, 0
	case "ctrl+end":
		e.row = len(e.lines) - 1
		e.col = len(e.line())
	case "ctrl+left", "alt+left":
		p := e.wordBack(Pos{e.row, e.col})
		e.row, e.col = p.Row, p.Col
	case "ctrl+right", "alt+right":
		p := e.wordForward(Pos{e.row, e.col})
		e.row, e.col = p.Row, p.Col
	case " ":
		e.checkpoint("type")
		e.insertText(" ")
	default:
		if k.Type != tea.KeyRunes || k.Alt {
			return false
		}
		e.checkpoint("type")
		e.insertText(string(k.Runes))
	}
	e.lastKill = kill
	e.want = e.col
	return true
}

func (e *Editor) updateStandard(k tea.KeyMsg) {
	switch k.String() {
	case "ctrl+z":
		e.doUndo()
	case "ctrl+y":
		e.doRedo()
	case "ctrl+k":
		e.checkpoint("kill")
		e.killLine()
		e.lastKill = true
		e.lastKind = "kill"
		return
	case "ctrl+u":
		e.checkpoint("paste")
		e.paste()
	default:
		e.editKey(k)
	}
	e.lastKill = false
}
