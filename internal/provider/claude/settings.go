// Package claude is fleet's first agent provider. Everything Claude-specific —
// the `claude` binary, the ~/.claude/settings.json hooks, the hook payload
// format — is confined here, so adding Codex later touches nothing else (§14).
package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"command-center/internal/config"
)

// hookEvents maps each Claude Code hook event to the fleet state subcommand it
// triggers (`fleet hook <state>`). These are written into fleet's own scoped
// settings file and injected per-launch via `claude --settings` (§9) — never into
// the user's global ~/.claude/settings.json. Notification/needs-input was dropped:
// fleet tracks only Running / Finished / Inactive.
var hookEvents = []struct{ Event, State string }{
	{"UserPromptSubmit", "running"},
	{"Stop", "finished"},
	{"SessionEnd", "inactive"},
}

// stateArgs is the closed set of state args fleet recognizes. It deliberately
// still includes the retired "needs-input" so the legacy-cleanup sweep
// (removeLegacyGlobalHooks) recognizes and removes hooks an older fleet wrote
// into ~/.claude/settings.json, even though fleet no longer installs that event.
var stateArgs = map[string]bool{"running": true, "finished": true, "needs-input": true, "inactive": true}

// settingsPath is ~/.claude/settings.json — the global location older fleet
// versions installed into. Fleet no longer writes here; it is read only to sweep
// those legacy entries out (removeLegacyGlobalHooks).
func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// scopedSettingsPath is ~/.config/fleet/claude/settings.json — fleet's own
// settings file, injected into only fleet-launched sessions via
// `claude --settings`. It lives under fleet's config root, never in a repo and
// never in the user's global Claude config, so it cannot fire on (or error in)
// unrelated Claude Code sessions.
func scopedSettingsPath() (string, error) {
	root, err := config.Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "claude", "settings.json"), nil
}

// scopedSettingsExists reports whether the scoped settings file is present, so
// LaunchSpec only passes --settings when there is something to point at.
func scopedSettingsExists() bool {
	path, err := scopedSettingsPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
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

// writeScopedSettings (re)writes fleet's scoped settings file with the current
// fleet binary path, returning its path. Rewriting on every call means the hook
// command can never go stale (the old global install's failure mode: a moved or
// rebuilt binary left a dead command firing in every session). The file contains
// nothing but fleet's hooks, so injecting it via --settings adds exactly fleet's
// tracking to a session and nothing else.
func writeScopedSettings() (string, error) {
	path, err := scopedSettingsPath()
	if err != nil {
		return "", err
	}
	hooks := map[string]any{}
	for _, he := range hookEvents {
		hooks[he.Event] = []any{newFleetGroup(he.State)}
	}
	data, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := os.Rename(name, path); err != nil {
		return "", err
	}
	return path, nil
}

// removeScopedSettings deletes fleet's scoped settings file. A missing file is
// not an error.
func removeScopedSettings() error {
	path, err := scopedSettingsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// removeLegacyGlobalHooks strips any fleet hook an older version installed into
// the user's global ~/.claude/settings.json — the entries that used to fire (and
// error) in every unrelated Claude Code session. It sweeps every event, not just
// the ones fleet writes today, so retired events (e.g. Notification) are caught
// too. Non-fleet hooks and unrelated settings are preserved exactly; an event
// left empty is removed. Missing file → nothing to do.
func removeLegacyGlobalHooks() error {
	settings, err := loadSettings()
	if err != nil {
		return err
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return nil // nothing of ours to remove
	}
	changed := false
	for event := range hooks {
		before := asGroupSlice(hooks[event])
		groups := dropFleetGroups(before)
		if len(groups) == len(before) {
			continue // no fleet hook in this event
		}
		changed = true
		if len(groups) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = groups
		}
	}
	if !changed {
		return nil // don't rewrite the user's file when we touched nothing
	}
	return saveSettings(settings)
}

// legacyGlobalHooksPresent reports whether any fleet hook still lingers in the
// global ~/.claude/settings.json (from a pre-`--settings` fleet). Used only to
// decide whether the migration sweep has anything to do.
func legacyGlobalHooksPresent() bool {
	settings, err := loadSettings()
	if err != nil {
		return false
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}
	for event := range hooks {
		if eventHasFleetHook(asGroupSlice(hooks[event])) {
			return true
		}
	}
	return false
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
