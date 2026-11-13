// Package sound toca efeitos curtos com bipes do Windows.
//
// Abrir o PowerShell leva quase um segundo, então um único processo fica
// aberto em segundo plano, lendo comandos de bipe da entrada, e cada efeito
// sai na hora. Fora do Windows e do WSL os efeitos são ignorados.
package sound

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"questtui/internal/remind"
)

type Effect int

const (
	Step    Effect = iota // etapa concluída
	Quest                 // quest concluída
	AllDone               // última quest pendente concluída
	Focus                 // fim de um ciclo do modo foco
	Tick                  // confirmação curta (ligar o som, por exemplo)
)

// tunes: frequência em Hz e duração em ms de cada nota.
var tunes = map[Effect][][2]int{
	Step:    {{988, 60}, {1319, 110}},
	Quest:   {{784, 80}, {988, 80}, {1175, 80}, {1568, 240}},
	AllDone: {{523, 90}, {659, 90}, {784, 90}, {1047, 150}, {784, 90}, {1047, 300}},
	Focus:   {{880, 140}, {660, 140}, {880, 240}},
	Tick:    {{1319, 50}},
}

// loop lê uma linha por vez e a executa; acaba quando a entrada fecha.
const loop = `while (($line = [Console]::In.ReadLine()) -ne $null) { try { Invoke-Expression $line } catch {} }`

type Player struct {
	mu     sync.Mutex
	in     io.WriteCloser
	cmd    *exec.Cmd
	broken bool
}

// Play toca o efeito sem esperar ele terminar.
func (p *Player) Play(e Effect) {
	notes := tunes[e]
	if len(notes) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.start() {
		return
	}
	parts := make([]string, len(notes))
	for i, n := range notes {
		parts[i] = fmt.Sprintf("[console]::Beep(%d,%d)", n[0], n[1])
	}
	if _, err := io.WriteString(p.in, strings.Join(parts, ";")+"\n"); err != nil {
		p.stop()
		p.broken = true
	}
}

// start abre o processo do PowerShell, se ainda não estiver aberto.
func (p *Player) start() bool {
	if p.in != nil {
		return true
	}
	if p.broken {
		return false
	}
	ps := remind.PowerShell()
	if ps == "" {
		p.broken = true
		return false
	}
	cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", remind.Encode(loop))
	in, err := cmd.StdinPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		p.broken = true
		return false
	}
	p.cmd, p.in = cmd, in
	return true
}

func (p *Player) stop() {
	if p.in != nil {
		p.in.Close()
	}
	if p.cmd != nil {
		go p.cmd.Wait() // termina sozinho quando a entrada fecha
	}
	p.in, p.cmd = nil, nil
}

// Close fecha o processo de som.
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stop()
}
