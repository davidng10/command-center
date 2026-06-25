package config

import (
	"encoding/json"
	"os"
	"slices"
)

// ReposPath is ~/.config/fleet/repos.json — the registered repo list.
func ReposPath() (string, error) { return under("repos.json") }

// LoadRepos reads the registered repo paths from repos.json.
func LoadRepos() []string {
	path, err := ReposPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var repos []string
	_ = json.Unmarshal(data, &repos)
	return repos
}

// SaveRepos persists the repo list.
func SaveRepos(repos []string) error {
	path, err := ReposPath()
	if err != nil {
		return err
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(repos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// AddRepo appends a repo path (idempotent) and persists.
func AddRepo(path string) error {
	repos := LoadRepos()
	if slices.Contains(repos, path) {
		return nil
	}
	repos = append(repos, path)
	return SaveRepos(repos)
}

// RemoveRepo removes a repo path and persists.
func RemoveRepo(path string) error {
	repos := LoadRepos()
	var out []string
	for _, r := range repos {
		if r != path {
			out = append(out, r)
		}
	}
	return SaveRepos(out)
}
