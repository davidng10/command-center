// Package claude is fleet's first agent provider. Everything Claude-specific —
// the `claude` binary, the ~/.claude/settings.json hooks, the hook payload
// format — is confined here, so adding Codex later touches nothing else (§14).
package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// hookEvents maps each Claude Code hook event to the fleet state subcommand it
// triggers (`fleet hook <state>`). This is the doc-backed mapping from §9.
var hookEvents = []struct{ Event, State string }{
	{"UserPromptSubmit", "running"},
	{"Stop", "finished"},
	{"Notification", "needs-input"},
	{"SessionEnd", "inactive"},
}

// stateArgs is the closed set of fleet hook state args, used to recognize fleet's
// own hook entries on uninstall regardless of the binary's path or name.
var stateArgs = map[string]bool{"running": true, "finished": true, "needs-input": true, "inactive": true}

// settingsPath is ~/.claude/settings.json — the global, consented hook location
// (§9: keeps hooks out of every repo/worktree).
func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// fleetExe is the absolute path to the running fleet binary, used as the hook
// command so it resolves regardless of the agent's PATH. Falls back to "fleet".
func fleetExe() string {
	if p, err := os.Executable(); err == nil {
		return p
	}
	return "fleet"
}

// hookCommand is the command string written for a given state.
func hookCommand(state string) string { return quoteIfNeeded(fleetExe()) + " hook " + state }

// quoteIfNeeded double-quotes a path containing spaces so the hook command parses
// as program + args (Claude runs the command string via a shell).
func quoteIfNeeded(p string) string {
	if strings.ContainsAny(p, " \t") {
		return `"` + p + `"`
	}
	return p
}

// isFleetHookCommand reports whether a hook command string is one fleet installed
// — i.e. it ends with `hook <state>` for a known state. This matches by shape, so
// a moved/renamed binary is still recognized for clean removal.
func isFleetHookCommand(cmd string) bool {
	f := strings.Fields(cmd)
	if len(f) < 3 {
		return false
	}
	return f[len(f)-2] == "hook" && stateArgs[f[len(f)-1]]
}

// loadSettings reads settings.json into a generic map so unknown top-level keys
// (the user's other settings) survive a round-trip untouched. Missing file → {}.
func loadSettings() (map[string]any, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	m := map[string]any{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// saveSettings writes settings.json atomically (temp + rename), creating
// ~/.claude if needed.
func saveSettings(m map[string]any) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// hooksMap returns the (event -> []group) map under "hooks", creating it if
// absent. Returns the settings map's live sub-map so mutations stick.
func hooksMap(settings map[string]any) map[string]any {
	if h, ok := settings["hooks"].(map[string]any); ok {
		return h
	}
	h := map[string]any{}
	settings["hooks"] = h
	return h
}

// dropFleetGroups removes fleet's command-hooks from one event's group slice,
// dropping any group left with no hooks. Non-fleet groups/hooks are preserved
// exactly, including their matcher.
func dropFleetGroups(groups []any) []any {
	out := make([]any, 0, len(groups))
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			out = append(out, g) // unexpected shape — leave it alone
			continue
		}
		hooks, ok := gm["hooks"].([]any)
		if !ok {
			out = append(out, g)
			continue
		}
		kept := make([]any, 0, len(hooks))
		for _, h := range hooks {
			if hm, ok := h.(map[string]any); ok {
				if cmd, _ := hm["command"].(string); isFleetHookCommand(cmd) {
					continue // drop fleet's hook
				}
			}
			kept = append(kept, h)
		}
		if len(kept) == 0 {
			continue // group existed only for fleet — drop it
		}
		gm["hooks"] = kept
		out = append(out, gm)
	}
	return out
}

// asGroupSlice coerces a hooks[event] value to a []any group slice.
func asGroupSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

// newFleetGroup builds the matcher-less group that runs one fleet hook command.
func newFleetGroup(state string) map[string]any {
	return map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": hookCommand(state)},
		},
	}
}

// install merges fleet's 4 hooks into settings.json idempotently: it first
// strips any prior fleet entries (so re-install with a new binary path refreshes
// the command), then appends a fresh group per event, leaving the user's own
// hooks intact.
func install() error {
	settings, err := loadSettings()
	if err != nil {
		return err
	}
	hooks := hooksMap(settings)
	for _, he := range hookEvents {
		groups := dropFleetGroups(asGroupSlice(hooks[he.Event]))
		groups = append(groups, newFleetGroup(he.State))
		hooks[he.Event] = groups
	}
	return saveSettings(settings)
}

// uninstall removes only fleet's hook entries, leaving everything else — and an
// otherwise-empty "hooks" map — as it was.
func uninstall() error {
	settings, err := loadSettings()
	if err != nil {
		return err
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return nil // nothing of ours to remove
	}
	for _, he := range hookEvents {
		groups := dropFleetGroups(asGroupSlice(hooks[he.Event]))
		if len(groups) == 0 {
			delete(hooks, he.Event)
		} else {
			hooks[he.Event] = groups
		}
	}
	return saveSettings(settings)
}

// installed reports whether every fleet hook event currently carries a fleet
// command — used to show accurate onboarding status and to gate re-runs.
func installed() bool {
	settings, err := loadSettings()
	if err != nil {
		return false
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}
	for _, he := range hookEvents {
		if !eventHasFleetHook(asGroupSlice(hooks[he.Event])) {
			return false
		}
	}
	return true
}

func eventHasFleetHook(groups []any) bool {
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := gm["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hooks {
			if hm, ok := h.(map[string]any); ok {
				if cmd, _ := hm["command"].(string); isFleetHookCommand(cmd) {
					return true
				}
			}
		}
	}
	return false
}
