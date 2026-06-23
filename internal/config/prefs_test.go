package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetupCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, ok := CachedSetup("/repo/a"); ok {
		t.Fatal("expected no entry before anything is remembered")
	}

	if err := RememberSetup("/repo/a", "uv sync"); err != nil {
		t.Fatal(err)
	}
	if err := RememberSetup("/repo/b", ""); err != nil { // a remembered "skip"
		t.Fatal(err)
	}

	if v, ok := CachedSetup("/repo/a"); !ok || v != "uv sync" {
		t.Fatalf("a = %q, %v; want \"uv sync\", true", v, ok)
	}
	if v, ok := CachedSetup("/repo/b"); !ok || v != "" {
		t.Fatalf("b = %q, %v; want \"\", true (remembered skip)", v, ok)
	}

	if err := RememberSetup("/repo/a", "npm ci"); err != nil {
		t.Fatal(err)
	}
	if v, _ := CachedSetup("/repo/a"); v != "npm ci" {
		t.Fatalf("a after overwrite = %q, want \"npm ci\"", v)
	}
}

func TestBaseCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, ok := CachedBase("/repo/a"); ok {
		t.Fatal("expected no base before anything is remembered")
	}
	if err := RememberBase("/repo/a", "develop"); err != nil {
		t.Fatal(err)
	}
	if v, ok := CachedBase("/repo/a"); !ok || v != "develop" {
		t.Fatalf("base = %q, %v; want develop, true", v, ok)
	}
	// Remembering a base must not clobber a remembered setup for the same repo.
	if err := RememberSetup("/repo/a", "pnpm install"); err != nil {
		t.Fatal(err)
	}
	if err := RememberBase("/repo/a", "main"); err != nil {
		t.Fatal(err)
	}
	if v, ok := CachedSetup("/repo/a"); !ok || v != "pnpm install" {
		t.Fatalf("setup survived base update: %q, %v", v, ok)
	}
}

func TestRecentDirsMRU(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for _, d := range []string{"/a", "/b", "/a"} { // /a touched twice
		if err := PushRecentDir(d); err != nil {
			t.Fatal(err)
		}
	}
	got := LoadPrefs().RecentDirs
	if len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
		t.Fatalf("RecentDirs = %v, want [/a /b]", got)
	}
}

func TestLegacySetupsMigration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)

	// Seed a legacy setups.json with no prefs.json present.
	legacy := filepath.Join(home, "fleet", "setups.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{"/repo/x":"pnpm install","/repo/y":""}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if v, ok := CachedSetup("/repo/x"); !ok || v != "pnpm install" {
		t.Fatalf("migrated x = %q, %v; want pnpm install, true", v, ok)
	}
	if v, ok := CachedSetup("/repo/y"); !ok || v != "" {
		t.Fatalf("migrated y = %q, %v; want \"\", true", v, ok)
	}
}
