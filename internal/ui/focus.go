package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"questtui/internal/remind"
	"questtui/internal/sound"
)

// Modo foco: rodadas de concentração (25 min por padrão) separadas por
// pausas curtas, mostradas no cabeçalho e na vigília junto da quest rastreada.

type focusState struct {
	active  bool
	paused  bool
	onBreak bool
	end     time.Time     // quando a fase atual acaba
	left    time.Duration // tempo restante enquanto pausado
	rounds  int           // rodadas de foco completas
}

// focusLen é a duração de uma rodada de foco ou de uma pausa.
func (m *Model) focusLen(onBreak bool) time.Duration {
	if onBreak {
		return time.Duration(max(m.cfg.BreakMinutes, 1)) * time.Minute
	}
	return time.Duration(max(m.cfg.FocusMinutes, 1)) * time.Minute
}

// focusToggle inicia o foco, pausa ou retoma.
func (m *Model) focusToggle() {
	f := &m.focus
	now := time.Now()
	switch {
	case !f.active:
		*f = focusState{active: true, end: now.Add(m.focusLen(false))}
		m.flash(tr("foco iniciado", "focus started"))
	case f.paused:
		f.paused, f.end = false, now.Add(f.left)
		m.flash(tr("foco retomado", "focus resumed"))
	default:
		f.paused, f.left = true, f.end.Sub(now)
		m.flash(tr("foco pausado", "focus paused"))
	}
	m.now = now
}

func (m *Model) focusStop() {
	if !m.focus.active {
		return
	}
	m.focus = focusState{}
	m.flash(tr("foco encerrado", "focus stopped"))
}

// focusRemaining é o tempo que falta na fase atual.
func (m *Model) focusRemaining() time.Duration {
	f := m.focus
	if f.paused {
		return f.left
	}
	return max(f.end.Sub(m.now), 0)
}

// focusRatio é quanto da fase atual já passou (0 a 1).
func (m *Model) focusRatio() float64 {
	total := m.focusLen(m.focus.onBreak)
	return 1 - float64(m.focusRemaining())/float64(total)
}

// focusClock é o tempo restante como MM:SS.
func (m *Model) focusClock() string {
	left := m.focusRemaining()
	return fmt.Sprintf("%02d:%02d", int(left.Minutes()), int(left.Seconds())%60)
}

// focusLabel é o texto do cronômetro, ou "" sem foco ativo.
func (m *Model) focusLabel() string {
	if !m.focus.active {
		return ""
	}
	text := m.focusClock()
	switch {
	case m.focus.paused:
		text += tr(" pausado", " paused")
	case m.focus.onBreak:
		text += tr(" pausa", " break")
	}
	return text
}

// stepFocus troca de fase quando o tempo acaba, com aviso, som e notificação.
func (m *Model) stepFocus() tea.Cmd {
	f := &m.focus
	if !f.active || f.paused || m.now.Before(f.end) {
		return nil
	}
	var title, body string
	if f.onBreak {
		f.onBreak, f.end = false, m.now.Add(m.focusLen(false))
		title, body = tr("FIM DA PAUSA", "BREAK OVER"), tr("De volta ao foco", "Back to focus")
	} else {
		f.rounds++
		f.onBreak, f.end = true, m.now.Add(m.focusLen(true))
		title = tr("FOCO CONCLUÍDO", "FOCUS COMPLETE")
		body = fmt.Sprintf(tr("Rodada %d · hora da pausa", "Round %d · time for a break"), f.rounds)
	}
	if q := m.tracked(); q != nil && !f.onBreak {
		body += " · " + q.Title
	}
	m.banner.show(bannerFocus, title, body, m.elapsed())
	m.ring(30, 11, m.palette())
	m.play(sound.Focus)
	if !m.cfg.Reminders {
		return nil
	}
	return func() tea.Msg {
		_ = remind.Send(title, body)
		return nil
	}
}
