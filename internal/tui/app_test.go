package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func TestHomeEmptyRepoList(t *testing.T) {
	a := newTestApp(t, Options{})
	if a.scr != scrHome {
		t.Fatalf("expected home screen, got %v", a.scr)
	}
	out := a.View()
	if !strings.Contains(out, "No repos added") {
		t.Fatalf("empty home missing hint:\n%s", out)
	}
	if !strings.Contains(out, "/add") {
		t.Fatalf("empty home missing /add hint:\n%s", out)
	}
}

func TestHomeWithRepoShowsRepoList(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := newTestApp(t, Options{})

	// Create a real git repo to add.
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "test-repo")
	os.MkdirAll(repoDir, 0o755)
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-q", "-b", "main")
	run("git", "config", "user.email", "t@t.co")
	run("git", "config", "user.name", "t")
	os.WriteFile(filepath.Join(repoDir, "f"), []byte("x"), 0o644)
	run("git", "add", "f")
	run("git", "commit", "-qm", "init")

	_ = config.AddRepo(repoDir)
	a.refreshRepos()

	out := a.View()
	if !strings.Contains(out, "test-repo") {
		t.Fatalf("home should show repo name:\n%s", out)
	}
	if !strings.Contains(out, "█▀▀") {
		t.Fatalf("home should show logo art:\n%s", out)
	}
}

func TestHomeAutoPopulatesFromSessions(t *testing.T) {
	a := newTestApp(t, Options{})
	now := time.Now()
	_ = a.reg.Add(session.Session{
		ID: "ab12", Provider: "claude", Branch: "task/SP-1-demo", Base: "main",
		RepoDir: "/tmp/fake-repo", WorktreePath: "/tmp/fake-repo-demo",
		State: session.StateRunning, CreatedAt: now, LastActivity: now,
	})
	a.refreshRepos()
	if len(a.repoEntries) == 0 {
		t.Fatal("repos should be auto-populated from existing sessions")
	}
	if a.repoEntries[0].Path != "/tmp/fake-repo" {
		t.Fatalf("expected /tmp/fake-repo, got %s", a.repoEntries[0].Path)
	}
}

func TestEnterRepoSwitchesToDetail(t *testing.T) {
	a := newTestApp(t, Options{})
	_ = config.AddRepo("/tmp/fake-repo")
	a.refreshRepos()

	// Press Enter to drill into the repo
	a = send(a, special(tea.KeyEnter))
	if a.scr != scrRepoDetail {
		t.Fatalf("expected repo detail screen, got %v", a.scr)
	}
	if a.selectedRepo != "/tmp/fake-repo" {
		t.Fatalf("expected selectedRepo=/tmp/fake-repo, got %s", a.selectedRepo)
	}
}

func TestRepoDetailEscGoesBack(t *testing.T) {
	a := newTestApp(t, Options{})
	_ = config.AddRepo("/tmp/fake-repo")
	a.refreshRepos()

	a = send(a, special(tea.KeyEnter)) // enter repo
	if a.scr != scrRepoDetail {
		t.Fatalf("expected repo detail, got %v", a.scr)
	}
	a = send(a, special(tea.KeyEsc)) // back to list
	if a.scr != scrHome {
		t.Fatalf("expected home after Esc, got %v", a.scr)
	}
}

func TestCommandModeAdd(t *testing.T) {
	a := newTestApp(t, Options{})
	a = send(a, key("/"))
	if !a.cmdMode {
		t.Fatal("'/' should enter command mode")
	}
	a = send(a, key("add"))
	a = send(a, special(tea.KeyEnter))
	if a.scr != scrAddRepo || a.add == nil {
		t.Fatalf("/add should open the add-repo screen, screen=%v", a.scr)
	}
}

func TestBackspaceLeavesCommandModeCleanly(t *testing.T) {
	a := newTestApp(t, Options{})
	a = send(a, key("/"))
	if !a.cmdMode {
		t.Fatal("'/' should enter command mode")
	}
	a = send(a, special(tea.KeyBackspace))
	if a.cmdMode {
		t.Fatal("backspacing the leading / should exit command mode")
	}
	out := a.View()
	if !strings.Contains(out, "Type to search") {
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
	a = send(a, special(tea.KeyEnter)) // provider -> done
	if !strings.Contains(a.View(), "You're all set") {
		t.Fatalf("done step missing:\n%s", a.View())
	}
	a = send(a, special(tea.KeyEnter)) // done -> home
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
