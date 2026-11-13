package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"questtui/internal/theme"
)

type styles struct {
	text    lipgloss.Style
	muted   lipgloss.Style
	accent  lipgloss.Style
	accent2 lipgloss.Style
	accent3 lipgloss.Style
	done    lipgloss.Style
	danger  lipgloss.Style

	border      lipgloss.Border
	borderColor lipgloss.Color
	accentColor lipgloss.Color
}

func fg(color string) lipgloss.Style {
	s := lipgloss.NewStyle()
	if color != "" {
		s = s.Foreground(lipgloss.Color(color))
	}
	return s
}

func newStyles(t theme.Theme) styles {
	accent3 := t.Accent3
	if accent3 == "" {
		accent3 = t.Accent2
	}
	s := styles{
		accent3:     fg(accent3),
		text:        fg(t.Fg),
		muted:       fg(t.Muted),
		accent:      fg(t.Accent).Bold(true),
		accent2:     fg(t.Accent2),
		done:        fg(t.Done),
		danger:      fg(t.Danger),
		borderColor: lipgloss.Color(t.BorderColor),
		accentColor: lipgloss.Color(t.Accent),
	}
	switch t.Border {
	case "normal":
		s.border = lipgloss.NormalBorder()
	case "thick":
		s.border = lipgloss.ThickBorder()
	case "double":
		s.border = lipgloss.DoubleBorder()
	case "hidden":
		s.border = lipgloss.HiddenBorder()
	default:
		s.border = lipgloss.RoundedBorder()
	}
	return s
}

// huhTheme pinta os formulários com as cores do tema atual.
func huhTheme(t theme.Theme) *huh.Theme {
	h := huh.ThemeBase()
	accent := lipgloss.Color(t.Accent)
	muted := lipgloss.Color(t.Muted)

	h.Focused.Base = h.Focused.Base.BorderForeground(accent)
	h.Focused.Title = h.Focused.Title.Foreground(accent).Bold(true)
	h.Focused.Description = h.Focused.Description.Foreground(muted)
	h.Focused.SelectSelector = h.Focused.SelectSelector.Foreground(accent)
	h.Focused.TextInput.Cursor = h.Focused.TextInput.Cursor.Foreground(accent)
	h.Focused.TextInput.Prompt = h.Focused.TextInput.Prompt.Foreground(lipgloss.Color(t.Accent2))
	h.Focused.TextInput.Placeholder = h.Focused.TextInput.Placeholder.Foreground(muted)
	h.Focused.ErrorIndicator = h.Focused.ErrorIndicator.Foreground(lipgloss.Color(t.Danger))
	h.Focused.ErrorMessage = h.Focused.ErrorMessage.Foreground(lipgloss.Color(t.Danger))
	h.Blurred.Title = h.Blurred.Title.Foreground(muted)
	h.Blurred.Description = h.Blurred.Description.Foreground(muted)
	h.Blurred.TextInput.Placeholder = h.Blurred.TextInput.Placeholder.Foreground(muted)
	return h
}
