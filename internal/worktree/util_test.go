package worktree

import (
	"slices"
	"testing"

	"command-center/internal/config"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Login Fix!":          "login-fix",
		"  Add CSV   export ": "add-csv-export",
		"already-kebab":       "already-kebab",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeBranch(t *testing.T) {
	cases := map[string]string{
		"task/SP-1234-login fix": "task/SP-1234-login-fix",
		"  feature/Login   Fix ": "feature/Login-Fix",
		"PROJ-99":                "PROJ-99",
		"already/kebab":          "already/kebab",
	}
	for in, want := range cases {
		if got := SanitizeBranch(in); got != want {
			t.Errorf("SanitizeBranch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitLaunch(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantArgs []string
	}{
		{"claude", "claude", nil},
		{"code --wait", "code", []string{"--wait"}},
		{"  code   --wait  . ", "code", []string{"--wait", "."}},
		{"", "", nil},
		{"   ", "", nil},
	}
	for _, c := range cases {
		name, args := SplitLaunch(c.in)
		if name != c.wantName {
			t.Errorf("SplitLaunch(%q) name = %q, want %q", c.in, name, c.wantName)
		}
		if !slices.Equal(args, c.wantArgs) {
			t.Errorf("SplitLaunch(%q) args = %v, want %v", c.in, args, c.wantArgs)
		}
	}
}

func TestApplyTemplate(t *testing.T) {
	got := ApplyTemplate("{repo}-{branch}", map[string]string{
		"repo": "product-catalog", "branch": "task-sp-1234-login-fix",
	})
	if want := "product-catalog-task-sp-1234-login-fix"; got != want {
		t.Errorf("ApplyTemplate = %q, want %q", got, want)
	}
}

func TestBuildPlan(t *testing.T) {
	repo := RepoContext{Root: "/x/product-catalog", Name: "product-catalog", Parent: "/x"}
	p := BuildPlan(repo, config.Default(), "task/SP-1234-login fix")

	if p.Branch != "task/SP-1234-login-fix" {
		t.Errorf("Branch = %q", p.Branch)
	}
	if p.WorktreeName != "product-catalog-task-sp-1234-login-fix" {
		t.Errorf("WorktreeName = %q", p.WorktreeName)
	}
	if p.WorktreePath != "/x/product-catalog-task-sp-1234-login-fix" {
		t.Errorf("WorktreePath = %q", p.WorktreePath)
	}
}
