package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// setupCachePath returns the per-user store of remembered setup commands —
// ~/.config/fleet/setups.json (honoring XDG_CONFIG_HOME). It lives outside any
// repo, so it never adds noise to a checkout and works for read-only repos.
func setupCachePath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "fleet", "setups.json"), nil
}

// loadSetupCache reads the cache (repo root path -> setup command). Any problem
// reading or parsing it yields an empty map: a stale cache should never break
// worktree creation.
func loadSetupCache() map[string]string {
	cache := map[string]string{}
	path, err := setupCachePath()
	if err != nil {
		return cache
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cache
	}
	_ = json.Unmarshal(data, &cache)
	return cache
}

// cachedSetup returns the remembered setup command for repoRoot and whether an
// entry exists. The value may be "" — the user chose to skip setup for this
// repo, which is itself worth remembering.
func cachedSetup(repoRoot string) (string, bool) {
	v, ok := loadSetupCache()[repoRoot]
	return v, ok
}

// rememberSetup persists the chosen setup command for repoRoot, writing only
// when the value actually changed.
func rememberSetup(repoRoot, cmd string) error {
	cache := loadSetupCache()
	if existing, ok := cache[repoRoot]; ok && existing == cmd {
		return nil
	}
	cache[repoRoot] = cmd

	path, err := setupCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
