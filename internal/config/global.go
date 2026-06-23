package config

import (
	"encoding/json"
	"os"
	"os/exec"
)

// Global is fleet's machine-wide config.json. It records onboarding completion,
// the default provider, and the user's IDE / terminal launch commands.
type Global struct {
	SetupComplete   bool   `json:"setupComplete"`   // gates the first-run onboarding
	DefaultProvider string `json:"defaultProvider"` // pre-selects /new's provider step
	IDE             string `json:"ide"`             // /open and /view (e.g. "code")
	Terminal        string `json:"terminal"`        // terminal-spawn command template; "" = per-OS default
}

// defaultGlobal supplies sane values before onboarding writes the file. The IDE
// is auto-detected from PATH so a fresh setup picks the editor the user actually
// has (Cursor, Zed, …) instead of assuming VS Code.
func defaultGlobal() Global {
	return Global{
		SetupComplete:   false,
		DefaultProvider: "claude",
		IDE:             DetectIDE(),
		Terminal:        "",
	}
}

// ideCandidates is the PATH-preference order for auto-detecting the user's IDE
// when `ide` is unset. Cursor/Windsurf/Codium come before `code` so VS Code forks
// win on machines that have both their own shim and a leftover `code`.
var ideCandidates = []string{"cursor", "windsurf", "codium", "code", "zed", "subl", "idea", "nvim", "vim"}

// DetectIDE returns the first editor CLI found on PATH from ideCandidates, or
// "code" as a last resort. Callers may always override via config.json `ide`.
func DetectIDE() string {
	for _, c := range ideCandidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return "code"
}

// LoadGlobal reads config.json, returning defaults when it is missing or
// malformed (a broken global config should never wedge the whole app).
func LoadGlobal() Global {
	g := defaultGlobal()
	path, err := GlobalPath()
	if err != nil {
		return g
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return g
	}
	_ = json.Unmarshal(data, &g)
	return g
}

// SaveGlobal persists config.json (creating the directory if needed).
func SaveGlobal(g Global) error {
	path, err := GlobalPath()
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
