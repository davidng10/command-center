package config

import (
	"encoding/json"
	"os"
	"slices"
)

// maxRecentDirs caps the directory MRU so prefs.json can't grow without bound.
const maxRecentDirs = 12

// Prefs is the consolidated "last used" cache (prefs.json), generalizing the
// original setups.json. Everything here is a convenience default — a stale or
// missing prefs file must never block worktree creation.
type Prefs struct {
	LastProvider string               `json:"lastProvider,omitempty"` // pre-selects /new's provider step once >1 exists
	RecentDirs   []string             `json:"recentDirs,omitempty"`   // MRU for the directory step (most-recent first)
	Repos        map[string]RepoPrefs `json:"repos,omitempty"`        // per-repo, keyed by absolute path
}

// RepoPrefs is the remembered choices for one repo. LastSetup is a pointer so we
// can tell "never remembered" (nil) from "remembered the user's skip" (&"") — the
// latter is a real, worth-keeping choice, exactly as the original setups.json
// cache distinguished a missing key from an empty value.
type RepoPrefs struct {
	LastBase  string  `json:"lastBase,omitempty"`
	LastSetup *string `json:"lastSetup,omitempty"`
}

// LoadPrefs reads prefs.json. When it is absent, it transparently folds a legacy
// setups.json (repoRoot → setup command) into the new shape so existing users
// keep their remembered setups. Any read/parse problem yields empty prefs.
func LoadPrefs() Prefs {
	p := Prefs{Repos: map[string]RepoPrefs{}}
	path, err := PrefsPath()
	if err != nil {
		return p
	}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &p)
		if p.Repos == nil {
			p.Repos = map[string]RepoPrefs{}
		}
		return p
	}
	// No prefs.json yet — migrate any legacy setups.json (read-only; the fold is
	// persisted on the next change, per the design's migration note).
	for repo, setup := range loadLegacySetups() {
		s := setup
		p.Repos[repo] = RepoPrefs{LastSetup: &s}
	}
	return p
}

// SavePrefs persists prefs.json (creating the directory if needed).
func SavePrefs(p Prefs) error {
	if p.Repos == nil {
		p.Repos = map[string]RepoPrefs{}
	}
	path, err := PrefsPath()
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// loadLegacySetups reads the old setups.json (repoRoot → setup). Missing/broken
// file yields nil.
func loadLegacySetups() map[string]string {
	path, err := legacySetupsPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	m := map[string]string{}
	_ = json.Unmarshal(data, &m)
	return m
}

// CachedSetup returns the remembered setup command for repoRoot and whether an
// entry exists. The value may be "" — the user chose to skip setup for this repo,
// which is itself worth remembering.
func CachedSetup(repoRoot string) (string, bool) {
	rp, ok := LoadPrefs().Repos[repoRoot]
	if !ok || rp.LastSetup == nil {
		return "", false
	}
	return *rp.LastSetup, true
}

// CachedBase returns the remembered base branch for repoRoot, if any.
func CachedBase(repoRoot string) (string, bool) {
	rp, ok := LoadPrefs().Repos[repoRoot]
	if !ok || rp.LastBase == "" {
		return "", false
	}
	return rp.LastBase, true
}

// RememberSetup persists the chosen setup command for repoRoot, writing only when
// the value actually changed.
func RememberSetup(repoRoot, cmd string) error {
	p := LoadPrefs()
	rp := p.Repos[repoRoot]
	if rp.LastSetup != nil && *rp.LastSetup == cmd {
		return nil
	}
	c := cmd
	rp.LastSetup = &c
	p.Repos[repoRoot] = rp
	return SavePrefs(p)
}

// RememberBase persists the chosen base branch for repoRoot, writing only when it
// changed.
func RememberBase(repoRoot, base string) error {
	if base == "" {
		return nil
	}
	p := LoadPrefs()
	rp := p.Repos[repoRoot]
	if rp.LastBase == base {
		return nil
	}
	rp.LastBase = base
	p.Repos[repoRoot] = rp
	return SavePrefs(p)
}

// RememberProvider records the last-used provider name.
func RememberProvider(name string) error {
	p := LoadPrefs()
	if p.LastProvider == name {
		return nil
	}
	p.LastProvider = name
	return SavePrefs(p)
}

// PushRecentDir moves dir to the front of the directory MRU, de-duplicating and
// capping the list. Writes only when the order actually changes.
func PushRecentDir(dir string) error {
	if dir == "" {
		return nil
	}
	p := LoadPrefs()
	if len(p.RecentDirs) > 0 && p.RecentDirs[0] == dir {
		return nil // already at the front
	}
	next := []string{dir}
	for _, d := range p.RecentDirs {
		if d != dir {
			next = append(next, d)
		}
	}
	if len(next) > maxRecentDirs {
		next = next[:maxRecentDirs]
	}
	if slices.Equal(next, p.RecentDirs) {
		return nil
	}
	p.RecentDirs = next
	return SavePrefs(p)
}
