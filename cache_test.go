package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetupCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, ok := cachedSetup("/repo/a"); ok {
		t.Fatal("expected no entry before anything is remembered")
	}

	if err := rememberSetup("/repo/a", "uv sync"); err != nil {
		t.Fatal(err)
	}
	if err := rememberSetup("/repo/b", ""); err != nil { // a remembered "skip"
		t.Fatal(err)
	}

	if v, ok := cachedSetup("/repo/a"); !ok || v != "uv sync" {
		t.Fatalf("a = %q, %v; want \"uv sync\", true", v, ok)
	}
	if v, ok := cachedSetup("/repo/b"); !ok || v != "" {
		t.Fatalf("b = %q, %v; want \"\", true (remembered skip)", v, ok)
	}

	if err := rememberSetup("/repo/a", "npm ci"); err != nil {
		t.Fatal(err)
	}
	if v, _ := cachedSetup("/repo/a"); v != "npm ci" {
		t.Fatalf("a after overwrite = %q, want \"npm ci\"", v)
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
	if cmd, src := resolveSetupDefault(Config{}, dir); cmd != "npm ci" || src != "detected" {
		t.Fatalf("detect: got %q/%q, want \"npm ci\"/\"detected\"", cmd, src)
	}

	// A remembered choice beats detection.
	if err := rememberSetup(dir, "pnpm install"); err != nil {
		t.Fatal(err)
	}
	if cmd, src := resolveSetupDefault(Config{}, dir); cmd != "pnpm install" || src != "saved" {
		t.Fatalf("cache: got %q/%q, want \"pnpm install\"/\"saved\"", cmd, src)
	}

	// An explicit .ccrc.json setup beats both.
	if cmd, src := resolveSetupDefault(Config{Setup: "uv sync"}, dir); cmd != "uv sync" || src != "configured" {
		t.Fatalf("config: got %q/%q, want \"uv sync\"/\"configured\"", cmd, src)
	}
}
