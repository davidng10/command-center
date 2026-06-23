package worktree

import (
	"os"
	"path/filepath"

	"command-center/internal/config"
)

// DetectSetupCommand suggests the dependency-install command for a Node project,
// inferred from its lockfile. It returns "" when root isn't a recognizable Node
// project (Go/Rust/etc. need no per-worktree install, and anything else is the
// user's to configure via `setup`). The result is only a suggestion — the user
// confirms or edits it before it runs, so a wrong guess is harmless.
func DetectSetupCommand(root string) string {
	has := func(f string) bool {
		_, err := os.Stat(filepath.Join(root, f))
		return err == nil
	}
	switch {
	case has("pnpm-lock.yaml"):
		return "pnpm install"
	case has("yarn.lock"):
		return "yarn install"
	case has("bun.lockb"):
		return "bun install"
	case has("package-lock.json"):
		return "npm ci" // a committed lockfile + fresh worktree is exactly what `npm ci` is for
	case has("package.json"):
		return "npm install" // Node project with no lockfile — nothing to install from, so plain install
	}
	return ""
}

// CommonSetups are the setup commands offered in the picker alongside whatever
// is detected/remembered — a small fixed catalog covering the usual ecosystems.
var CommonSetups = []string{
	"pnpm install",
	"npm ci",
	"npm install",
	"yarn install",
	"bun install",
	"uv sync",
	"poetry install",
	"docker compose up -d --build",
}

// ResolveSetupDefault picks the setup command to pre-select in the picker:
// an explicit .ccrc.json `setup` wins, then a previously remembered choice for
// this repo, then auto-detection. source names which one, for the picker label.
func ResolveSetupDefault(cfg config.Config, repoRoot string) (cmd, source string) {
	if cfg.Setup != "" {
		return cfg.Setup, "configured"
	}
	if c, ok := config.CachedSetup(repoRoot); ok {
		return c, "saved"
	}
	return DetectSetupCommand(repoRoot), "detected"
}
