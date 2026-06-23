package tui

import (
	"github.com/charmbracelet/lipgloss"

	"command-center/internal/session"
)

// Palette — the lipgloss equivalent of mockup.html's CSS variables. Truecolor
// hex; lipgloss degrades gracefully on lesser terminals.
var (
	cAccent     = lipgloss.Color("#a78bfa")
	cAccentSoft = lipgloss.Color("#6d5bd0")
	cInk        = lipgloss.Color("#e7e7ee")
	cDim        = lipgloss.Color("#7b7b8b")
	cDimmer     = lipgloss.Color("#4f4f5c")
	cLine       = lipgloss.Color("#2a2a36")
	cRun        = lipgloss.Color("#fbbf24")
	cDone       = lipgloss.Color("#4ade80")
	cInfo       = lipgloss.Color("#38bdf8")
	cInact      = lipgloss.Color("#9a9aa8")
	cSel        = lipgloss.Color("#1d1b2e")
)

var (
	stAccent  = lipgloss.NewStyle().Foreground(cAccent)
	stAccentB = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	stInk     = lipgloss.NewStyle().Foreground(cInk)
	stInkB    = lipgloss.NewStyle().Foreground(cInk).Bold(true)
	stDim     = lipgloss.NewStyle().Foreground(cDim)
	stDimmer  = lipgloss.NewStyle().Foreground(cDimmer)
	stInfo    = lipgloss.NewStyle().Foreground(cInfo)
	stCaret   = lipgloss.NewStyle().Foreground(cAccent)
)

// stateColor returns the badge/bullet color for a canonical state.
func stateColor(st session.State) lipgloss.Color {
	switch st {
	case session.StateRunning:
		return cRun
	case session.StateFinished:
		return cDone
	case session.StateNeedsInput:
		return cInfo
	default:
		return cInact
	}
}
