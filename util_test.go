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

func TestNormalizeTicket(t *testing.T) {
	cases := map[string]string{
		"SP-1234": "1234",
		"sp 1234": "1234",
		"1234":    "1234",
		"SP1234":  "1234",
	}
	for in, want := range cases {
		if got := normalizeTicket(in); got != want {
			t.Errorf("normalizeTicket(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyTemplate(t *testing.T) {
	got := applyTemplate("task/SP-{ticket}-{name}", map[string]string{
		"ticket": "1234", "name": "login-fix",
	})
	if want := "task/SP-1234-login-fix"; got != want {
		t.Errorf("applyTemplate = %q, want %q", got, want)
	}
}

func TestBuildPlan(t *testing.T) {
	repo := RepoContext{Root: "/x/product-catalog", Name: "product-catalog", Parent: "/x"}
	p := buildPlan(repo, defaultConfig(), "SP-1234", "login fix")

	if p.Branch != "task/SP-1234-login-fix" {
		t.Errorf("Branch = %q", p.Branch)
	}
	if p.WorktreeName != "product-catalog-1234" {
		t.Errorf("WorktreeName = %q", p.WorktreeName)
	}
	if p.WorktreePath != "/x/product-catalog-1234" {
		t.Errorf("WorktreePath = %q", p.WorktreePath)
	}
}
