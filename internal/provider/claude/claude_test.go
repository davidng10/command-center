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

func TestHookInstallIdempotentAndRemovable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Seed settings.json with a user hook fleet must not clobber.
	settingsDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userSettings := `{
  "model": "opus",
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "my-own-notify"}]}]
  }
}`
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(userSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	if installed() {
		t.Fatal("should not report installed before install")
	}
	if err := install(); err != nil {
		t.Fatal(err)
	}
	if err := install(); err != nil { // idempotent
		t.Fatal(err)
	}
	if !installed() {
		t.Fatal("should report installed after install")
	}

	// The user's settings + their own Stop hook survive; fleet's 4 hooks exist.
	m := readSettings(t, settingsFile)
	if m["model"] != "opus" {
		t.Fatalf("unrelated key lost: %v", m["model"])
	}
	hooks := m["hooks"].(map[string]any)
	for _, ev := range []string{"UserPromptSubmit", "Stop", "Notification", "SessionEnd"} {
		if _, ok := hooks[ev]; !ok {
			t.Fatalf("missing hook event %q", ev)
		}
	}
	// Stop must contain BOTH the user's hook and exactly one fleet hook (no dup
	// after two installs).
	if got := countCommand(hooks["Stop"], "my-own-notify"); got != 1 {
		t.Fatalf("user Stop hook count = %d, want 1", got)
	}
	if got := countFleetHooks(hooks["Stop"]); got != 1 {
		t.Fatalf("fleet Stop hook count = %d, want 1 (idempotent)", got)
	}

	if err := uninstall(); err != nil {
		t.Fatal(err)
	}
	if installed() {
		t.Fatal("should not report installed after uninstall")
	}
	m = readSettings(t, settingsFile)
	hooks = m["hooks"].(map[string]any)
	if got := countCommand(hooks["Stop"], "my-own-notify"); got != 1 {
		t.Fatalf("user Stop hook lost on uninstall: count = %d", got)
	}
	// Events that existed only for fleet are gone.
	if _, ok := hooks["UserPromptSubmit"]; ok {
		t.Fatal("fleet-only event should be removed on uninstall")
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
