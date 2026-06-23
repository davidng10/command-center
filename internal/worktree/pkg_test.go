package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"command-center/internal/config"
)

func TestDetectSetupCommand(t *testing.T) {
	cases := []struct {
		name    string
		markers []string
		want    string
	}{
		{"pnpm", []string{"pnpm-lock.yaml", "package.json"}, "pnpm install"},
		{"yarn", []string{"yarn.lock", "package.json"}, "yarn install"},
		{"bun", []string{"bun.lockb", "package.json"}, "bun install"},
		{"npm with lockfile", []string{"package-lock.json", "package.json"}, "npm ci"},
		{"npm no lockfile", []string{"package.json"}, "npm install"},
		{"not a node project", []string{"go.mod", "main.go"}, ""},
		{"empty", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, m := range c.markers {
				if err := os.WriteFile(filepath.Join(dir, m), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := DetectSetupCommand(dir); got != c.want {
				t.Errorf("DetectSetupCommand(%v) = %q, want %q", c.markers, got, c.want)
			}
		})
	}
}

func TestResolveSetupDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dir := t.TempDir()
	for _, f := range []string{"package-lock.json", "package.json"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Nothing configured or cached → fall back to detection.
	if cmd, src := ResolveSetupDefault(config.Config{}, dir); cmd != "npm ci" || src != "detected" {
		t.Fatalf("detect: got %q/%q, want \"npm ci\"/\"detected\"", cmd, src)
	}

	// A remembered choice beats detection.
	if err := config.RememberSetup(dir, "pnpm install"); err != nil {
		t.Fatal(err)
	}
	if cmd, src := ResolveSetupDefault(config.Config{}, dir); cmd != "pnpm install" || src != "saved" {
		t.Fatalf("cache: got %q/%q, want \"pnpm install\"/\"saved\"", cmd, src)
	}

	// An explicit .ccrc.json setup beats both.
	if cmd, src := ResolveSetupDefault(config.Config{Setup: "uv sync"}, dir); cmd != "uv sync" || src != "configured" {
		t.Fatalf("config: got %q/%q, want \"uv sync\"/\"configured\"", cmd, src)
	}
}
