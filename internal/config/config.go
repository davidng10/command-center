package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config mirrors .ccrc.json. Defaults live in Default(); a repo's .ccrc.json
// overlays only the keys it sets.
type Config struct {
	BaseBranches []string `json:"baseBranches"`
	DefaultBase  string   `json:"defaultBase"`
	WorktreeName string   `json:"worktreeName"`
	CopyFiles    []string `json:"copyFiles"`
	Install      bool     `json:"install"` // whether to offer a setup step at all
	Setup        string   `json:"setup"`   // explicit setup command; overrides auto-detection when set
	Launch       string   `json:"launch"`
	Fetch        bool     `json:"fetch"`
}

// Default returns fleet's built-in defaults, used when a repo has no .ccrc.json
// or only overrides some keys.
func Default() Config {
	return Config{
		BaseBranches: []string{"main", "develop"},
		DefaultBase:  "main",
		WorktreeName: "{repo}-{branch}",
		CopyFiles:    []string{".env", ".env.local", ".env.development", ".env.development.local"},
		Install:      true,
		Launch:       "claude",
		Fetch:        true,
	}
}

// Load returns defaults overlaid with any .ccrc.json found at repoRoot.
// Unmarshaling onto the pre-filled struct means keys absent from the file keep
// their default value.
func Load(repoRoot string) Config {
	cfg := Default()
	data, err := os.ReadFile(filepath.Join(repoRoot, ".ccrc.json"))
	if err != nil {
		return cfg // no file (or unreadable) → defaults
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "warning: ignoring malformed .ccrc.json:", err)
	}
	return cfg
}
