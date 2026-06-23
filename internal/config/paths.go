// Package config owns everything fleet persists outside a repo: the per-repo
// .ccrc.json overlay (Config), the global config.json (Global), the consolidated
// "last used" prefs.json cache (Prefs), and the transient per-session state dir.
// Nothing here is provider- or TUI-specific, so the rest of fleet can depend on
// it freely.
package config

import (
	"os"
	"path/filepath"
)

// Root returns fleet's per-user config directory — ~/.config/fleet, honoring
// XDG_CONFIG_HOME (matching the original cache.go behavior). It lives outside any
// repo, so nothing fleet owns ever lands in a checkout.
func Root() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "fleet"), nil
}

// GlobalPath is ~/.config/fleet/config.json.
func GlobalPath() (string, error) { return under("config.json") }

// SessionsPath is ~/.config/fleet/sessions.json (the registry).
func SessionsPath() (string, error) { return under("sessions.json") }

// PrefsPath is ~/.config/fleet/prefs.json.
func PrefsPath() (string, error) { return under("prefs.json") }

// legacySetupsPath is the original ~/.config/fleet/setups.json, folded into
// prefs.json on first run (see prefs.go).
func legacySetupsPath() (string, error) { return under("setups.json") }

// StateDir is ~/.config/fleet/state — where `fleet hook` writes per-session
// activity files and the Claude tracker watches.
func StateDir() (string, error) { return under("state") }

// LockPath is ~/.config/fleet/fleet.pid — the single-instance pidfile.
func LockPath() (string, error) { return under("fleet.pid") }

func under(name string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

// ensureDir makes sure the parent directory of path exists.
func ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
