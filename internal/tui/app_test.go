package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"command-center/internal/config"
	"command-center/internal/provider"
	"command-center/internal/provider/claude"
	"command-center/internal/session"
)

// newTestApp builds an App wired to an isolated config root and a sized window.
func newTestApp(t *testing.T, opts Options) App {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	provider.Register(claude.New())

	reg, err := session.Load()
	if err != nil {
		t.Fatal(err)
	}
	acts := provider.NoopTracker{}.Updates(context.Background())
	a := NewApp(reg, claude.New(), config.Global{SetupComplete: true, DefaultProvider: "claude", IDE: "code"}, acts, opts)
	m, _ := a.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	return m.(App)
}

func key(s string) tea.KeyMsg          { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
func special(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func send(a App, m tea.Msg) App {
	next, _ := a.Update(m)
	return next.(App)
}

func TestHomeEmptyAndPopulatedRender(t *testing.T) {
	a := newTestApp(t, Options{})
	if a.scr != scrHome {
		t.Fatalf("expected home screen, got %v", a.scr)
	}
	out := a.View()
	if !strings.Contains(out, "No active sessions") {
		t.Fatalf("empty home missing hint:\n%s", out)
	}
	// Command bar: new placeholder, top+bottom rules only (no rounded corners).
	if !strings.Contains(out, "Type / to see available commands") {
		t.Fatalf("command bar missing new placeholder:\n%s", out)
	}
	if strings.ContainsAny(out, "╭╮╰╯│") {
		t.Fatalf("command bar should have no corners/side borders:\n%s", out)
	}

	// Add a running session and re-render.
	now := time.Now()
	_ = a.reg.Add(session.Session{
		ID: "ab12", Provider: "claude", Branch: "task/SP-1-demo", Base: "main",
		RepoDir: "/tmp/repo", WorktreePath: "/tmp/repo-demo",
		State: session.StateRunning, CreatedAt: now, LastActivity: now,
	})
	out = a.View()
	if !strings.Contains(out, "task/SP-1-demo") || !strings.Contains(out, "Running") {
		t.Fatalf("populated home missing row/badge:\n%s", out)
	}
}

func TestCommandModeStartsWizard(t *testing.T) {
	a := newTestApp(t, Options{})
	a = send(a, key("/"))
	if !a.cmdMode {
		t.Fatal("'/' should enter command mode")
	}
	a = send(a, key("new"))
	a = send(a, special(tea.KeyEnter))
	if a.scr != scrWizard || a.wiz == nil {
		t.Fatalf("/new should open the wizard, screen=%v", a.scr)
	}
	if !strings.Contains(a.View(), "Name this session") {
		t.Fatalf("wizard name step missing:\n%s", a.View())
	}
}

func TestBackspaceLeavesCommandModeCleanly(t *testing.T) {
	a := newTestApp(t, Options{})
	a = send(a, key("/"))
	if !a.cmdMode {
		t.Fatal("'/' should enter command mode")
	}
	// Backspace the lone "/" → empty input should drop back to navigation, and the
	// idle placeholder should render cleanly (no stray first-char "T" artifact).
	a = send(a, special(tea.KeyBackspace))
	if a.cmdMode {
		t.Fatal("backspacing the leading / should exit command mode")
	}
	out := a.View()
	if !strings.Contains(out, "Type / to see available commands") {
		t.Fatalf("idle placeholder not restored after backspace:\n%s", out)
	}
}

func TestUnknownCommandFlashes(t *testing.T) {
	a := newTestApp(t, Options{})
	a = send(a, key("/"))
	a = send(a, key("frobnicate"))
	a = send(a, special(tea.KeyEnter))
	if !strings.Contains(a.flash, "unknown command") {
		t.Fatalf("expected unknown-command flash, got %q", a.flash)
	}
}

func TestWizardNameToDir(t *testing.T) {
	a := newTestApp(t, Options{StartWizard: true})
	if a.scr != scrWizard {
		t.Fatalf("StartWizard should open wizard, got %v", a.scr)
	}
	a = send(a, key("task/demo"))
	a = send(a, special(tea.KeyEnter))
	if a.wiz.step != wsDir {
		t.Fatalf("expected dir step, got %v", a.wiz.step)
	}
	out := a.View()
	if !strings.Contains(out, "Select the directory") {
		t.Fatalf("dir step missing:\n%s", out)
	}
	// Esc backs to the name step.
	a = send(a, special(tea.KeyEsc))
	if a.wiz.step != wsName {
		t.Fatalf("esc should return to name step, got %v", a.wiz.step)
	}
}

func TestOnboardingFlowRendersAndCompletes(t *testing.T) {
	a := newTestApp(t, Options{ForceSetup: true})
	if a.scr != scrOnboard {
		t.Fatalf("ForceSetup should open onboarding, got %v", a.scr)
	}
	if !strings.Contains(a.View(), "Choose your agent provider") {
		t.Fatalf("provider step missing:\n%s", a.View())
	}
	a = send(a, special(tea.KeyEnter)) // provider -> hooks
	if !strings.Contains(a.View(), "Set up status tracking") {
		t.Fatalf("hooks step missing:\n%s", a.View())
	}
	// Choose "Skip for now" so the test doesn't touch real settings beyond temp HOME.
	a = send(a, special(tea.KeyDown))
	a = send(a, special(tea.KeyEnter)) // hooks -> done
	if !strings.Contains(a.View(), "You're all set") {
		t.Fatalf("done step missing:\n%s", a.View())
	}
	a = send(a, special(tea.KeyEnter)) // done -> home, persists setupComplete
	if a.scr != scrHome {
		t.Fatalf("after onboarding should land on home, got %v", a.scr)
	}
	if !config.LoadGlobal().SetupComplete {
		t.Fatal("onboarding should have persisted setupComplete=true")
	}
}

func TestTickReconcilesAndKeepsRunning(t *testing.T) {
	a := newTestApp(t, Options{})
	m, cmd := a.Update(tickMsg(time.Now()))
	a = m.(App)
	if cmd == nil {
		t.Fatal("tick should re-arm itself")
	}
	_ = a.View()
}
