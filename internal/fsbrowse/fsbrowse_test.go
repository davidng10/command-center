package fsbrowse

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestListMarksReposAndHidesDotfolders(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"alpha", "beta", ".hidden", "zeta"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// make "beta" look like a repo
	if err := os.MkdirAll(filepath.Join(root, "beta", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// a plain file should not appear
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	repo := map[string]bool{}
	for _, e := range entries {
		names = append(names, e.Name)
		repo[e.Name] = e.IsRepo
	}
	want := []string{"alpha", "beta", "zeta"} // sorted, no .hidden, no file
	if len(names) != len(want) {
		t.Fatalf("entries = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("order/content = %v, want %v", names, want)
		}
	}
	if !repo["beta"] || repo["alpha"] {
		t.Fatalf("repo detection wrong: %v", repo)
	}
}

func TestBranchesLocalAndRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@t.co")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "f")
	git("commit", "-qm", "init")
	git("branch", "develop")

	bs, err := Branches(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, b := range bs {
		got[b.Name] = b.Remote
	}
	if _, ok := got["main"]; !ok {
		t.Fatalf("expected main among %v", bs)
	}
	if _, ok := got["develop"]; !ok {
		t.Fatalf("expected develop among %v", bs)
	}
}
