package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"command-center/internal/session"
	"command-center/internal/term"
	"command-center/internal/worktree"
)

// updateHome handles keys on the home screen across its three input modes:
// removal-confirm, command-bar, and (default) list navigation.
func (a App) updateHome(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	sessions := a.reg.All()
	a.clampHomeCursor(len(sessions))

	// 1) Confirming a removal — only y/n/esc matter.
	if a.confirmRemoveID != "" {
		switch m.String() {
		case "y", "Y":
			return a.removeSession(a.confirmRemoveID), nil
		default: // n, esc, anything else cancels
			a.confirmRemoveID = ""
			a.flash = ""
			return a, nil
		}
	}

	// 2) Command-bar mode — typing a /command.
	if a.cmdMode {
		switch m.Type {
		case tea.KeyEnter:
			cmd := strings.TrimSpace(a.cmdInput.Value())
			a.exitCmdMode()
			return a.runCommand(cmd, sessions)
		case tea.KeyEsc:
			a.exitCmdMode()
			return a, nil
		}
		var cmd tea.Cmd
		a.cmdInput, cmd = a.cmdInput.Update(m)
		// Backspacing the leading "/" (empty input) drops back to navigation.
		if a.cmdInput.Value() == "" {
			a.exitCmdMode()
		}
		return a, cmd
	}

	// 3) Navigation mode.
	switch m.String() {
	case "up", "k":
		if a.homeCursor > 0 {
			a.homeCursor--
		}
	case "down", "j":
		if a.homeCursor < len(sessions)-1 {
			a.homeCursor++
		}
	case "/":
		a.enterCmdMode()
	case "enter":
		if s, ok := a.selected(sessions); ok {
			if s.State == session.StateInactive {
				return a.relaunchSession(s)
			}
			if e := openIDE(a.global.IDE, s.WorktreePath); e != "" {
				a.flash = stWarn(e)
			} else {
				a.flash = stOK("opened " + s.Branch + " in IDE")
			}
		}
	case "o":
		if s, ok := a.selected(sessions); ok {
			if e := openIDE(a.global.IDE, s.WorktreePath); e != "" {
				a.flash = stWarn(e)
			} else {
				a.flash = stOK("opened " + s.Branch + " in IDE")
			}
		}
	case "x":
		if s, ok := a.selected(sessions); ok {
			a.confirmRemoveID = s.ID
			a.flash = ""
		}
	case "esc":
		return a, tea.Quit
	}
	return a, nil
}

func (a *App) enterCmdMode() {
	a.cmdMode = true
	a.cmdInput.SetValue("/")
	a.cmdInput.CursorEnd()
	a.cmdInput.Focus()
	a.flash = ""
}

func (a *App) exitCmdMode() {
	a.cmdMode = false
	a.cmdInput.SetValue("")
	a.cmdInput.Blur()
}

func (a *App) clampHomeCursor(n int) {
	if a.homeCursor >= n {
		a.homeCursor = n - 1
	}
	if a.homeCursor < 0 {
		a.homeCursor = 0
	}
}

func (a App) selected(sessions []session.Session) (session.Session, bool) {
	if a.homeCursor < 0 || a.homeCursor >= len(sessions) {
		return session.Session{}, false
	}
	return sessions[a.homeCursor], true
}

// runCommand parses and dispatches a /command typed in the command bar.
func (a App) runCommand(cmd string, sessions []session.Session) (tea.Model, tea.Cmd) {
	if cmd == "" {
		return a, nil
	}
	fields := strings.Fields(cmd)
	verb := fields[0]
	arg := strings.TrimSpace(strings.TrimPrefix(cmd, verb))

	pick := func() (session.Session, bool) {
		if arg != "" {
			return a.reg.Get(arg)
		}
		return a.selected(sessions)
	}

	switch verb {
	case "/new":
		a.scr = scrWizard
		a.wiz = newWizard(arg, a.prov, a.global)
		return a, nil
	case "/setup":
		a.scr = scrOnboard
		a.onb = newOnboard(a.prov)
		return a, nil
	case "/quit", "/exit":
		return a, tea.Quit
	case "/open":
		if s, ok := pick(); ok {
			if e := openIDE(a.global.IDE, s.WorktreePath); e != "" {
				a.flash = stWarn(e)
			} else {
				a.flash = stOK("opened " + s.Branch + " in IDE")
			}
		} else {
			a.flash = stWarn("no session to open")
		}
	case "/view":
		if s, ok := pick(); ok {
			a.flash = viewFlash(openIDE(a.global.IDE, s.WorktreePath), s)
		} else {
			a.flash = stWarn("no session to view")
		}
	case "/rm", "/remove":
		if s, ok := pick(); ok {
			a.confirmRemoveID = s.ID
		} else {
			a.flash = stWarn("no session to remove")
		}
	default:
		a.flash = stWarn("unknown command: " + verb)
	}
	return a, nil
}

// removeSession deletes the worktree and registry entry for id.
func (a App) removeSession(id string) App {
	a.confirmRemoveID = ""
	s, ok := a.reg.Get(id)
	if !ok {
		a.flash = stWarn("session gone")
		return a
	}
	if s.RepoDir != "" && s.WorktreePath != "" {
		if err := worktree.Remove(worktree.ContextFromRoot(s.RepoDir), s.WorktreePath); err != nil {
			a.flash = stWarn("worktree not fully removed: " + err.Error())
		}
	}
	_ = a.reg.Remove(id)
	if a.flash == "" {
		a.flash = stOK("removed " + s.Branch)
	}
	a.clampHomeCursor(len(a.reg.All()))
	return a
}

// relaunchSession spawns a fresh agent terminal for an Inactive session,
// reusing the existing worktree and branch.
func (a App) relaunchSession(s session.Session) (tea.Model, tea.Cmd) {
	if a.prov == nil {
		a.flash = stWarn("no provider configured")
		return a, nil
	}
	launch := a.prov.LaunchSpec(s)
	if launch.Program == "" {
		a.flash = stWarn("provider returned no launch command")
		return a, nil
	}
	pid, err := term.Spawn(launch.Dir, launch.Program, launch.Args, a.global.Terminal)
	if err != nil {
		a.flash = stWarn("relaunch failed: " + err.Error())
		return a, nil
	}
	now := time.Now()
	a.reg.Update(s.ID, func(s *session.Session) {
		s.State = session.StateRunning
		s.PID = pid
		s.LastActivity = now
		s.CreatedAt = now
	})
	a.flash = stOK("relaunched " + s.Branch)
	return a, nil
}

func viewFlash(ideErr string, s session.Session) string {
	if ideErr != "" {
		return stWarn(ideErr)
	}
	return stOK("viewing " + s.Branch + " — opened IDE (raise the agent's terminal to interact)")
}

// ---- view -------------------------------------------------------------------

func (a App) viewHome() string {
	inner := a.innerWidth()
	sessions := a.reg.All()
	a.clampHomeCursor(len(sessions))

	header := a.homeHeader(inner, sessions)

	if len(sessions) == 0 {
		body := a.emptyState(inner)
		return a.frame(header, body, "", [][2]string{{"/", "command"}, {"Esc", "exit"}}, true)
	}

	rows := make([]string, len(sessions))
	for i, s := range sessions {
		rows[i] = a.renderRow(i+1, s, i == a.homeCursor, inner)
	}
	// Each row is 2 lines; keep the selected row visible within the body region.
	// Chrome: header (1) + leading blank (1) + padding (1) + cmdbar (3) + flash (1) + status (2) = 9.
	maxRows := (a.height - 9) / 2
	if maxRows < 1 {
		maxRows = 1
	}
	visible, scrollUp, scrollDown := windowAroundInfo(rows, a.homeCursor, maxRows)

	var parts []string
	if scrollUp {
		parts = append(parts, "      "+stDimmer.Render(fmt.Sprintf("↑ %d more", a.homeCursor-maxRows/2)))
	}
	parts = append(parts, visible...)
	if scrollDown {
		below := len(rows) - (a.homeCursor + maxRows/2 + 1)
		if below < 1 {
			below = len(rows) - len(visible) // fallback
		}
		parts = append(parts, "      "+stDimmer.Render(fmt.Sprintf("↓ %d more", below)))
	}

	body := "\n" + strings.Join(parts, "\n") + "\n"

	// The status bar shows hotkeys only — the context label was just noise on
	// home. The one exception is the remove confirmation, where the keys (y/n)
	// don't say what's being deleted, so we keep that prompt on the left.
	ctx := ""
	keys := [][2]string{{"↑↓", "navigate"}, {"o", "open IDE"}, {"x", "remove"}, {"/", "command"}, {"Esc", "exit"}}
	if s, ok := a.selected(sessions); ok && s.State == session.StateInactive {
		keys = [][2]string{{"↑↓", "navigate"}, {"enter", "relaunch"}, {"o", "open IDE"}, {"x", "remove"}, {"/", "command"}, {"Esc", "exit"}}
	}
	if a.cmdMode {
		keys = [][2]string{{"enter", "run"}, {"Esc", "cancel"}}
	}
	if a.confirmRemoveID != "" {
		keys = [][2]string{{"y", "remove"}, {"n", "cancel"}}
		if s, ok := a.reg.Get(a.confirmRemoveID); ok {
			ctx = stInkB.Render("Remove this session?") + stDim.Render(" · "+s.Branch+" (worktree will be deleted)")
		}
	}
	return a.frame(header, body, ctx, keys, true)
}

// homeHeader folds the active-sessions label inline with the fleet wordmark, with
// the running count right-aligned.
func (a App) homeHeader(inner int, sessions []session.Session) string {
	if len(sessions) == 0 {
		return brand() + stDim.Render("  ·  no active sessions")
	}
	running := 0
	for _, s := range sessions {
		if s.State == session.StateRunning {
			running++
		}
	}
	left := brand() + stDim.Render("  ·  ") + stAccent.Render("●") + " " +
		stInk.Render("Active sessions ") + stDimmer.Render(fmt.Sprintf("(%d)", len(sessions)))
	right := stDim.Render(fmt.Sprintf("%d running", running))
	gap := inner - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (a App) emptyState(inner int) string {
	block := lipgloss.JoinVertical(lipgloss.Center,
		stDimmer.Render("⚇"),
		"",
		stDim.Render("No active sessions"),
		stDimmer.Render("Type ")+stAccent.Render("/new")+stDimmer.Render(" to spin up your first agent worktree."),
	)
	return lipgloss.NewStyle().Width(inner).Align(lipgloss.Center).
		Padding(2, 0).Render(block)
}

// renderRow draws one session as a two-line block (main line + sub-line). The
// selected row gets a full-color ❯ caret and a bold-accent branch name; every
// other row is rendered faint so the selection pops by contrast.
func (a App) renderRow(idx int, s session.Session, selected bool, inner int) string {
	dim := !selected
	// faint mutes a style for unselected rows while keeping its hue, so the status
	// colors still read — just quieter.
	faint := func(st lipgloss.Style) lipgloss.Style {
		if dim {
			return st.Faint(true)
		}
		return st
	}

	col := stateColor(s.State)
	bulletColor := col
	if s.State == session.StateRunning && !a.pulseOn {
		bulletColor = cDimmer // pulse
	}
	bull := faint(lipgloss.NewStyle().Foreground(bulletColor)).Render("●")
	bdg := faint(lipgloss.NewStyle().Foreground(col)).Render(s.State.Label())
	idxStr := faint(lipgloss.NewStyle().Foreground(cDimmer)).Render(fmt.Sprintf("%2d", idx))

	nameStyle := lipgloss.NewStyle().Foreground(cInk)
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	}
	nameMax := inner - lipgloss.Width(idxStr) - lipgloss.Width(bdg) - 10
	if nameMax < 8 {
		nameMax = 8
	}
	name := faint(nameStyle).Render(truncate(s.Branch, nameMax))

	caret := "  "
	if selected {
		caret = stAccent.Render("❯") + " "
	}
	left := caret + idxStr + " " + bull + " " + name
	gap := inner - lipgloss.Width(left) - lipgloss.Width(bdg) - 1
	if gap < 1 {
		gap = 1
	}
	line1 := left + strings.Repeat(" ", gap) + bdg

	sub := faint(lipgloss.NewStyle().Foreground(cDim)).
		Render(fmt.Sprintf("%s · %s · %s", s.Base, s.Age(a.now), activityPhrase(s, a.now)))
	line2 := "      " + sub // sub-line never carries the caret; aligns under the name
	return line1 + "\n" + line2
}

// activityPhrase is an honest, hook-derived sub-line (no fabricated filenames):
// what the session is doing and how long ago it last changed.
func activityPhrase(s session.Session, now time.Time) string {
	switch s.State {
	case session.StateRunning:
		return "working…"
	case session.StateFinished:
		return "idle " + s.ActivityAge(now) + " · ready to review"
	default:
		return "ended (terminal closed)"
	}
}

// truncate shortens s to at most n display columns, adding an ellipsis.
func truncate(s string, n int) string {
	if n <= 1 {
		return "…"
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) > n-1 {
		r = r[:n-1]
	}
	return string(r) + "…"
}
