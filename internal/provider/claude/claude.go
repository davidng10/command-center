package claude

import (
	"os"
	"os/exec"
	"path/filepath"

	"command-center/internal/provider"
	"command-center/internal/session"
)

// Provider is the Claude Code agent backend.
type Provider struct{}

// New returns the Claude provider as a provider.Provider.
func New() provider.Provider { return Provider{} }

// Name is the stable identifier persisted on sessions.
func (Provider) Name() string { return "claude" }

// LaunchSpec runs `claude` in the session's worktree, injecting fleet's scoped
// settings file via --settings so status hooks fire for THIS session only — never
// for unrelated Claude Code sessions on the machine. The settings file is written
// by Install (called once at startup), so here we only point at it; this stays
// pure (no I/O beyond a stat) because the TUI calls LaunchSpec on the render path.
// If the file is somehow absent, claude still launches — just without tracking.
func (Provider) LaunchSpec(s session.Session) provider.Launch {
	l := provider.Launch{Program: "claude", Dir: s.WorktreePath}
	if path, err := scopedSettingsPath(); err == nil && scopedSettingsExists() {
		l.Args = []string{"--settings", path}
	}
	return l
}

// Install writes fleet's scoped settings file (fresh, with the current binary
// path) and sweeps any legacy hooks an older fleet left in the global
// ~/.claude/settings.json. It is idempotent and safe to call on every startup.
func (Provider) Install() error {
	if _, err := writeScopedSettings(); err != nil {
		return err
	}
	return removeLegacyGlobalHooks()
}

// Uninstall removes fleet's scoped settings file and sweeps any legacy global
// hooks, leaving the user's own settings untouched.
func (Provider) Uninstall() error {
	if err := removeScopedSettings(); err != nil {
		return err
	}
	return removeLegacyGlobalHooks()
}

// Installed reports whether fleet's scoped settings file is in place. Tracking is
// otherwise automatic — there is nothing for the user to opt into.
func (Provider) Installed() bool { return scopedSettingsExists() }

// Tracker returns the state-dir poller that turns hook writes into canonical
// state updates.
func (Provider) Tracker() provider.StateTracker { return tracker{} }

// Preflight is the result of onboarding's prerequisite check. It is
// Claude-specific by design — it confirms `claude` is reachable and that fleet
// can write its own scoped settings file (~/.config/fleet/claude).
type Preflight struct {
	ClaudePath       string // absolute path to the claude binary; "" if not on PATH
	SettingsPath     string // fleet's scoped settings file
	SettingsWritable bool   // whether fleet can write the scoped settings file
}

// Preflight implements provider.Preflighter so onboarding can show prerequisites
// without importing this package directly.
func (Provider) Preflight() []provider.PreflightItem {
	pf := RunPreflight()
	var items []provider.PreflightItem
	if pf.ClaudePath != "" {
		items = append(items, provider.PreflightItem{OK: true, Label: "claude found — " + pf.ClaudePath})
	} else {
		items = append(items, provider.PreflightItem{OK: false, Label: "claude not found on PATH"})
	}
	if pf.SettingsWritable {
		items = append(items, provider.PreflightItem{OK: true, Label: "status tracking ready (scoped to fleet sessions)"})
	} else {
		items = append(items, provider.PreflightItem{OK: false, Label: pf.SettingsPath + " not writable"})
	}
	return items
}

// RunPreflight checks the two prerequisites onboarding surfaces: is `claude` on
// PATH, and can fleet write its own scoped settings file.
func RunPreflight() Preflight {
	pf := Preflight{}
	if p, err := exec.LookPath("claude"); err == nil {
		pf.ClaudePath = p
	}
	if path, err := scopedSettingsPath(); err == nil {
		pf.SettingsPath = path
		pf.SettingsWritable = dirWritable(filepath.Dir(path))
	}
	return pf
}

// dirWritable reports whether fleet can create files in dir (creating dir first
// if it doesn't exist), without leaving anything behind.
func dirWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".fleet-write-check-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
