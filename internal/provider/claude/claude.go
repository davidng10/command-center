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

// LaunchSpec runs `claude` in the session's worktree. The user types their task
// in the spawned terminal — fleet does not host the agent.
func (Provider) LaunchSpec(s session.Session) provider.Launch {
	return provider.Launch{Program: "claude", Dir: s.WorktreePath}
}

// Install merges fleet's hooks into ~/.claude/settings.json (idempotent).
func (Provider) Install() error { return install() }

// Uninstall removes only fleet's hooks.
func (Provider) Uninstall() error { return uninstall() }

// Installed reports whether all four fleet hooks are present.
func (Provider) Installed() bool { return installed() }

// Tracker returns the state-dir poller that turns hook writes into canonical
// state updates.
func (Provider) Tracker() provider.StateTracker { return tracker{} }

// Preflight is the result of the onboarding hook-install check (§4.7 step 2).
// It is Claude-specific by design — the hooks step is about ~/.claude/settings.json.
type Preflight struct {
	ClaudePath       string // absolute path to the claude binary; "" if not on PATH
	SettingsPath     string // ~/.claude/settings.json
	SettingsWritable bool   // whether fleet can write the settings file
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
		items = append(items, provider.PreflightItem{OK: true, Label: pf.SettingsPath + " writable"})
	} else {
		items = append(items, provider.PreflightItem{OK: false, Label: pf.SettingsPath + " not writable"})
	}
	return items
}

// RunPreflight checks the two prerequisites onboarding surfaces before offering
// to install hooks: is `claude` on PATH, and can fleet write settings.json.
func RunPreflight() Preflight {
	pf := Preflight{}
	if p, err := exec.LookPath("claude"); err == nil {
		pf.ClaudePath = p
	}
	if path, err := settingsPath(); err == nil {
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
