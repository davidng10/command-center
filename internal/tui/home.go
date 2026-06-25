package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"command-center/internal/config"
	"command-center/internal/session"
	"command-center/internal/term"
	"command-center/internal/worktree"
)

// updateHome handles keys on the home screen (repo list).
func (a App) updateHome(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.busyLabel != "" {
		return a, nil
	}

	entries := a.filteredRepoEntries()
	a.clampHomeCursor(len(entries))

	// Command-bar mode.
	if a.cmdMode {
		switch m.Type {
		case tea.KeyEnter:
			cmd := strings.TrimSpace(a.cmdInput.Value())
			a.exitCmdMode()
			return a.runCommand(cmd)
		case tea.KeyEsc:
			a.exitCmdMode()
			return a, nil
		}
		var cmd tea.Cmd
		a.cmdInput, cmd = a.cmdInput.Update(m)
		if a.cmdInput.Value() == "" {
			a.exitCmdMode()
		}
		return a, cmd
	}

	// Search mode — live-filtering repos by name.
	if a.searchMode {
		switch m.Type {
		case tea.KeyEnter, tea.KeyEsc:
			a.exitSearchMode()
			return a, nil
		case tea.KeyUp:
			if a.homeCursor > 0 {
				a.homeCursor--
			}
			return a, nil
		case tea.KeyDown:
			if a.homeCursor < len(entries)-1 {
				a.homeCursor++
			}
			return a, nil
		}
		var cmd tea.Cmd
		a.cmdInput, cmd = a.cmdInput.Update(m)
		a.searchFilter = a.cmdInput.Value()
		if a.searchFilter == "" {
			a.exitSearchMode()
		} else {
			a.homeCursor = 0
		}
		return a, cmd
	}

	// Navigation mode.
	switch m.String() {
	case "up", "k":
		if a.homeCursor > 0 {
			a.homeCursor--
		}
	case "down", "j":
		if a.homeCursor < len(entries)-1 {
			a.homeCursor++
		}
	case "/":
		a.enterCmdMode()
	case "enter":
		if e, ok := a.selectedRepoEntry(entries); ok {
			a.enterRepo(e.Path)
		}
	default:
		if isSearchKey(m) {
			a.enterSearchMode(m.String())
			return a, nil
		}
	}
	return a, nil
}

func (a *App) enterCmdMode() {
	a.cmdMode = true
	a.searchMode = false
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

func (a *App) enterSearchMode(initial string) {
	a.searchMode = true
	a.cmdMode = false
	a.cmdInput.SetValue(initial)
	a.cmdInput.CursorEnd()
	a.cmdInput.Focus()
	a.searchFilter = initial
	a.homeCursor = 0
	a.flash = ""
}

func (a *App) exitSearchMode() {
	a.searchMode = false
	a.searchFilter = ""
	a.cmdInput.SetValue("")
	a.cmdInput.Blur()
}

// isSearchKey returns true for printable characters that aren't bound as hotkeys.
func isSearchKey(m tea.KeyMsg) bool {
	if m.Type != tea.KeyRunes || len(m.Runes) == 0 {
		return false
	}
	r := m.Runes[0]
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
		switch m.String() {
		case "j", "k", "o", "t", "x":
			return false
		}
		return true
	}
	return false
}

func (a App) filteredRepoEntries() []repoEntry {
	if a.searchFilter == "" {
		return a.repoEntries
	}
	q := strings.ToLower(a.searchFilter)
	var out []repoEntry
	for _, r := range a.repoEntries {
		if strings.Contains(strings.ToLower(r.Name), q) {
			out = append(out, r)
		}
	}
	return out
}

func (a App) selectedRepoEntry(entries []repoEntry) (repoEntry, bool) {
	if a.homeCursor < 0 || a.homeCursor >= len(entries) {
		return repoEntry{}, false
	}
	return entries[a.homeCursor], true
}

func (a *App) clampHomeCursor(n int) {
	if a.homeCursor >= n {
		a.homeCursor = n - 1
	}
	if a.homeCursor < 0 {
		a.homeCursor = 0
	}
}

// ---- commands ---------------------------------------------------------------

type commandEntry struct {
	Name string
	Desc string
}

var commands = []commandEntry{
	{"/new", "create a new worktree"},
	{"/add", "add a repo to fleet"},
	{"/open", "open worktree in IDE"},
	{"/rm", "remove repo or worktree"},
	{"/setup", "re-run onboarding"},
	{"/quit", "exit fleet"},
}

func matchingCommands(input string) []commandEntry {
	prefix := strings.ToLower(strings.TrimSpace(input))
	if prefix == "" || prefix == "/" {
		return commands
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	var out []commandEntry
	for _, c := range commands {
		if strings.HasPrefix(c.Name, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func renderCommandPalette(matches []commandEntry) string {
	if len(matches) == 0 {
		return "  " + stDimmer.Render("no matching commands")
	}
	var lines []string
	for _, c := range matches {
		lines = append(lines, "  "+stAccent.Render(c.Name)+"  "+stDimmer.Render(c.Desc))
	}
	return strings.Join(lines, "\n")
}

// runCommand dispatches a /command typed in the command bar. Context-aware:
// behaviour varies depending on whether we're on the repo list or detail view.
func (a App) runCommand(cmd string) (tea.Model, tea.Cmd) {
	if cmd == "" {
		return a, nil
	}
	fields := strings.Fields(cmd)
	verb := fields[0]

	switch verb {
	case "/new":
		if a.scr == scrRepoDetail && a.selectedRepo != "" {
			repo := worktree.ContextFromRoot(a.selectedRepo)
			cfg := config.Load(a.selectedRepo)
			a.scr = scrWizard
			a.wiz = newWizardForRepo(repo, cfg, a.prov, a.global)
		} else if len(a.repoEntries) > 0 {
			entries := a.filteredRepoEntries()
			if e, ok := a.selectedRepoEntry(entries); ok {
				a.enterRepo(e.Path)
				repo := worktree.ContextFromRoot(a.selectedRepo)
				cfg := config.Load(a.selectedRepo)
				a.scr = scrWizard
				a.wiz = newWizardForRepo(repo, cfg, a.prov, a.global)
			}
		} else {
			a.flash = stWarn("add a repo first (/add)")
		}
		return a, nil

	case "/add":
		a.scr = scrAddRepo
		a.add = newAddRepoModel()
		return a, nil

	case "/setup":
		a.scr = scrOnboard
		a.onb = newOnboard(a.prov)
		return a, nil

	case "/quit", "/exit":
		return a, tea.Quit

	case "/open":
		if a.scr == scrRepoDetail {
			items := a.filteredDetailItems()
			if it, ok := a.selectedDetailItem(items); ok {
				path := it.Path
				if it.HasSession {
					path = it.Session.WorktreePath
				}
				if e := openIDE(a.global.IDE, path); e != "" {
					a.flash = stWarn(e)
				} else {
					a.flash = stOK("opened " + it.Branch + " in IDE")
				}
			}
		} else {
			a.flash = stWarn("enter a repo first")
		}

	case "/rm", "/remove":
		if a.scr == scrRepoDetail {
			items := a.filteredDetailItems()
			if it, ok := a.selectedDetailItem(items); ok && it.HasSession {
				a.confirmRemoveID = it.Session.ID
			} else {
				a.flash = stWarn("no removable session selected")
			}
		} else if a.scr == scrHome {
			entries := a.filteredRepoEntries()
			if e, ok := a.selectedRepoEntry(entries); ok {
				_ = config.RemoveRepo(e.Path)
				a.refreshRepos()
				a.flash = stOK("removed " + e.Name)
				a.clampHomeCursor(len(a.repoEntries))
			} else {
				a.flash = stWarn("no repo selected")
			}
		}

	default:
		a.flash = stWarn("unknown command: " + verb)
	}
	return a, nil
}

// ---- session actions (shared by repo detail) --------------------------------

func (a App) removeSession(id string) (tea.Model, tea.Cmd) {
	a.confirmRemoveID = ""
	s, ok := a.reg.Get(id)
	if !ok {
		a.flash = stWarn("session gone")
		return a, nil
	}
	a.busyLabel = "Removing " + s.Branch + "…"
	repoDir, wtPath, branch := s.RepoDir, s.WorktreePath, s.Branch
	_ = a.reg.Remove(id)
	return a, func() tea.Msg {
		var errMsg string
		if repoDir != "" && wtPath != "" {
			if err := worktree.Remove(worktree.ContextFromRoot(repoDir), wtPath); err != nil {
				errMsg = err.Error()
			}
		}
		return removeResultMsg{branch: branch, err: errMsg}
	}
}

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
	a.busyLabel = "Launching " + s.Branch + "…"
	id, branch, terminal := s.ID, s.Branch, a.global.Terminal
	return a, func() tea.Msg {
		pid, err := term.Spawn(launch.Dir, launch.Program, launch.Args, terminal)
		if err != nil {
			return relaunchResultMsg{id: id, branch: branch, err: err.Error()}
		}
		return relaunchResultMsg{id: id, branch: branch, pid: pid}
	}
}

func (a App) openTerminal(s session.Session) (tea.Model, tea.Cmd) {
	a.busyLabel = "Opening terminal…"
	branch, dir, terminal := s.Branch, s.WorktreePath, a.global.Terminal
	return a, func() tea.Msg {
		if err := term.SpawnShell(dir, terminal); err != nil {
			return openTermResultMsg{branch: branch, err: err.Error()}
		}
		return openTermResultMsg{branch: branch}
	}
}

// ---- view -------------------------------------------------------------------

func (a App) viewHome() string {
	inner := a.innerWidth()
	entries := a.filteredRepoEntries()
	a.clampHomeCursor(len(entries))

	header := a.repoListHeader(inner)

	if len(a.repoEntries) == 0 {
		body := a.repoEmptyState(inner)
		keys := [][2]string{{"/add", "add repo"}, {"/", "command"}, {"Ctrl+C", "quit"}}
		return a.frame(header, body, "", keys, true)
	}

	if len(entries) == 0 && a.searchFilter != "" {
		body := "\n" + "      " + stDimmer.Render("no repos matching \""+a.searchFilter+"\"")
		keys := [][2]string{{"Esc", "clear search"}}
		return a.frame(header, body, "", keys, true)
	}

	rows := make([]string, len(entries))
	for i, e := range entries {
		rows[i] = a.renderRepoRow(e, i == a.homeCursor, inner)
	}
	maxRows := (a.height - 11) / 2
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
			below = len(rows) - len(visible)
		}
		parts = append(parts, "      "+stDimmer.Render(fmt.Sprintf("↓ %d more", below)))
	}

	body := "\n" + strings.Join(parts, "\n") + "\n"

	keys := [][2]string{{"↑↓", "navigate"}, {"enter", "open repo"}, {"/add", "add repo"}, {"/rm", "remove"}, {"/", "command"}, {"Ctrl+C", "quit"}}
	if a.searchMode {
		keys = [][2]string{{"↑↓", "navigate"}, {"enter", "open repo"}, {"Esc", "clear search"}}
	}
	if a.cmdMode {
		keys = [][2]string{{"enter", "run"}, {"Esc", "cancel"}}
	}
	return a.frame(header, body, "", keys, true)
}

func (a App) repoListHeader(inner int) string {
	art := logo()

	totalRunning := 0
	for _, s := range a.reg.All() {
		if s.State == session.StateRunning {
			totalRunning++
		}
	}

	topLine := art[0]
	if totalRunning > 0 {
		right := stDim.Render(fmt.Sprintf("%d running", totalRunning))
		gap := inner - lipgloss.Width(topLine) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
		topLine = topLine + strings.Repeat(" ", gap) + right
	}

	return topLine + "\n" + art[1] + "\n" + art[2]
}

func (a App) repoEmptyState(inner int) string {
	block := lipgloss.JoinVertical(lipgloss.Center,
		stDimmer.Render("⚇"),
		"",
		stDim.Render("No repos added"),
		stDimmer.Render("Type ")+stAccent.Render("/add")+stDimmer.Render(" to add a repo."),
	)
	return lipgloss.NewStyle().Width(inner).Align(lipgloss.Center).
		Padding(2, 0).Render(block)
}

func (a App) renderRepoRow(e repoEntry, selected bool, inner int) string {
	dim := !selected
	faint := func(st lipgloss.Style) lipgloss.Style {
		if dim {
			return st.Faint(true)
		}
		return st
	}

	nameStyle := lipgloss.NewStyle().Foreground(cInk)
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	}

	caret := "  "
	if selected {
		caret = stAccent.Render("❯") + " "
	}

	name := faint(nameStyle).Render(e.Name)

	wtLabel := faint(lipgloss.NewStyle().Foreground(cDim)).
		Render(fmt.Sprintf("%d worktrees", e.WorktreeCount))

	left := caret + name
	gap := inner - lipgloss.Width(left) - lipgloss.Width(wtLabel) - 1
	if gap < 1 {
		gap = 1
	}
	line1 := left + strings.Repeat(" ", gap) + wtLabel

	// Sub-line: path + running count
	running := a.repoRunningCount(e.Path)
	sub := faint(lipgloss.NewStyle().Foreground(cDim)).Render(homeTilde(e.Path))
	var runInfo string
	if running > 0 {
		runInfo = faint(lipgloss.NewStyle().Foreground(cRun)).
			Render(fmt.Sprintf(" · %d running", running))
	}
	line2 := "      " + sub + runInfo
	return line1 + "\n" + line2
}

func (a App) repoRunningCount(repoPath string) int {
	count := 0
	for _, s := range a.reg.All() {
		if s.RepoDir == repoPath && s.State == session.StateRunning {
			count++
		}
	}
	return count
}

// ---- shared helpers ---------------------------------------------------------

// activityPhrase describes what a session is doing.
func activityPhrase(s session.Session, now time.Time) string {
	switch s.State {
	case session.StateRunning:
		return "working…"
	case session.StateIdle:
		return "idle " + s.ActivityAge(now) + " · ready to review"
	default:
		return "ended (session closed)"
	}
}

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
