package main

import (
	"os"
	"path/filepath"
	"testing"
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
			if got := detectSetupCommand(dir); got != c.want {
				t.Errorf("detectSetupCommand(%v) = %q, want %q", c.markers, got, c.want)
			}
		})
	}
}
