package main

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Login Fix!":         "login-fix",
		"  Add CSV   export ": "add-csv-export",
		"already-kebab":      "already-kebab",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
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
		if got := sanitizeBranch(in); got != want {
			t.Errorf("sanitizeBranch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyTemplate(t *testing.T) {
	got := applyTemplate("{repo}-{branch}", map[string]string{
		"repo": "product-catalog", "branch": "task-sp-1234-login-fix",
	})
	if want := "product-catalog-task-sp-1234-login-fix"; got != want {
		t.Errorf("applyTemplate = %q, want %q", got, want)
	}
}

func TestBuildPlan(t *testing.T) {
	repo := RepoContext{Root: "/x/product-catalog", Name: "product-catalog", Parent: "/x"}
	p := buildPlan(repo, defaultConfig(), "task/SP-1234-login fix")

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
