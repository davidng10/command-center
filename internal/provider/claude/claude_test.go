package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"command-center/internal/session"
)

// TestScopedSettingsWriteAndLaunch covers the Option-H mechanism: fleet writes a
// scoped settings file (3 hooks, no Notification) and LaunchSpec injects it via
// --settings only when it exists. Nothing touches ~/.claude/settings.json.
func TestScopedSettingsWriteAndLaunch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Before writing: not installed, and LaunchSpec passes no --settings.
	if scopedSettingsExists() {
		t.Fatal("scoped settings should not exist before write")
	}
	if args := (Provider{}).LaunchSpec(session.Session{WorktreePath: "/wt/x"}).Args; len(args) != 0 {
		t.Fatalf("expected no args before write, got %v", args)
	}

	path, err := writeScopedSettings()
	if err != nil {
		t.Fatal(err)
	}
	if err := func() error { _, err := writeScopedSettings(); return err }(); err != nil {
		t.Fatal(err) // idempotent rewrite
	}
	if !scopedSettingsExists() {
		t.Fatal("scoped settings should exist after write")
	}

	// File contains exactly the three tracked events, each with one fleet hook.
	m := readSettings(t, path)
	hooks, ok := m["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("no hooks map in scoped settings: %v", m)
	}
	for _, ev := range []string{"UserPromptSubmit", "Stop", "SessionEnd"} {
		if got := countFleetHooks(hooks[ev]); got != 1 {
			t.Fatalf("event %q fleet hook count = %d, want 1", ev, got)
		}
	}
	if _, present := hooks["Notification"]; present {
		t.Fatal("Notification (needs-input) should no longer be written")
	}

	// LaunchSpec now injects --settings pointing at the scoped file.
	l := (Provider{}).LaunchSpec(session.Session{WorktreePath: "/wt/x"})
	if l.Program != "claude" || l.Dir != "/wt/x" {
		t.Fatalf("unexpected launch: %+v", l)
	}
	if len(l.Args) != 2 || l.Args[0] != "--settings" || l.Args[1] != path {
		t.Fatalf("expected [--settings %s], got %v", path, l.Args)
	}

	// Uninstall removes the scoped file; LaunchSpec stops injecting.
	if err := removeScopedSettings(); err != nil {
		t.Fatal(err)
	}
	if scopedSettingsExists() {
		t.Fatal("scoped settings should be gone after removeScopedSettings")
	}
	if args := (Provider{}).LaunchSpec(session.Session{WorktreePath: "/wt/x"}).Args; len(args) != 0 {
		t.Fatalf("expected no args after removal, got %v", args)
	}
}

// TestRemoveLegacyGlobalHooks covers the migration: an older fleet's hooks in the
// user's global ~/.claude/settings.json are swept out (including the retired
// Notification event), while the user's own hooks and settings survive.
func TestRemoveLegacyGlobalHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A pre-upgrade user: their own Stop hook + fleet's 4 legacy hooks (one of them
	// the retired Notification/needs-input) + an unrelated top-level key.
	legacy := `{
  "model": "opus",
  "hooks": {
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "/old/fleet hook running"}]}],
    "Stop": [
      {"hooks": [{"type": "command", "command": "my-own-notify"}]},
      {"hooks": [{"type": "command", "command": "/old/fleet hook finished"}]}
    ],
    "Notification": [{"hooks": [{"type": "command", "command": "/old/fleet hook needs-input"}]}],
    "SessionEnd": [{"hooks": [{"type": "command", "command": "/old/fleet hook inactive"}]}]
  }
}`
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if !legacyGlobalHooksPresent() {
		t.Fatal("should detect legacy fleet hooks before sweep")
	}
	if err := removeLegacyGlobalHooks(); err != nil {
		t.Fatal(err)
	}
	if err := removeLegacyGlobalHooks(); err != nil {
		t.Fatal(err) // idempotent; no-op on the second pass
	}
	if legacyGlobalHooksPresent() {
		t.Fatal("legacy fleet hooks should be gone after sweep")
	}

	m := readSettings(t, settingsFile)
	if m["model"] != "opus" {
		t.Fatalf("unrelated key lost: %v", m["model"])
	}
	hooks := m["hooks"].(map[string]any)
	// The user's own Stop hook survives; fleet's Stop hook is gone.
	if got := countCommand(hooks["Stop"], "my-own-notify"); got != 1 {
		t.Fatalf("user Stop hook count = %d, want 1", got)
	}
	if got := countFleetHooks(hooks["Stop"]); got != 0 {
		t.Fatalf("fleet Stop hook count = %d, want 0 after sweep", got)
	}
	// Events that existed only for fleet are removed entirely.
	for _, ev := range []string{"UserPromptSubmit", "Notification", "SessionEnd"} {
		if _, present := hooks[ev]; present {
			t.Fatalf("fleet-only event %q should be removed on sweep", ev)
		}
	}
}

func TestHookWriteThenTrackerEmits(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	payload := `{"session_id":"agent-abc","cwd":"/wt/feature","hook_event_name":"UserPromptSubmit"}`
	if err := HandleHook("running", strings.NewReader(payload)); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ch := tracker{}.Updates(ctx)

	select {
	case upd := <-ch:
		if upd.Cwd != "/wt/feature" || upd.AgentSession != "agent-abc" || upd.State != session.StateRunning {
			t.Fatalf("unexpected update: %+v", upd)
		}
	case <-ctx.Done():
		t.Fatal("tracker did not emit the hook state in time")
	}
}

func TestHandleHookRejectsUnknownState(t *testing.T) {
	if err := HandleHook("bogus", strings.NewReader("{}")); err == nil {
		t.Fatal("expected error for unknown state")
	}
}

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func countCommand(groups any, cmd string) int {
	n := 0
	for _, g := range asGroupSlice(groups) {
		gm, _ := g.(map[string]any)
		hooks, _ := gm["hooks"].([]any)
		for _, h := range hooks {
			hm, _ := h.(map[string]any)
			if c, _ := hm["command"].(string); c == cmd {
				n++
			}
		}
	}
	return n
}

func countFleetHooks(groups any) int {
	n := 0
	for _, g := range asGroupSlice(groups) {
		gm, _ := g.(map[string]any)
		hooks, _ := gm["hooks"].([]any)
		for _, h := range hooks {
			hm, _ := h.(map[string]any)
			if c, _ := hm["command"].(string); isFleetHookCommand(c) {
				n++
			}
		}
	}
	return n
}
