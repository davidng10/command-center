package term

import (
	"slices"
	"testing"
)

func TestExpandTemplateBareCmdExpands(t *testing.T) {
	// `… -- {cmd}` must expand {cmd} into program + each arg as separate fields.
	name, args := expandTemplate("wezterm start --cwd {dir} -- {cmd}", "/work tree", "claude", []string{"--flag", "v"}, "claude --flag v")
	if name != "wezterm" {
		t.Fatalf("name = %q, want wezterm", name)
	}
	want := []string{"start", "--cwd", "/work tree", "--", "claude", "--flag", "v"}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestExpandTemplateCmdAsSubstring(t *testing.T) {
	name, args := expandTemplate("sh -c {cmd}", "/d", "claude", nil, "claude")
	if name != "sh" {
		t.Fatalf("name = %q", name)
	}
	// {cmd} as a bare field expands to just the program here (no args).
	want := []string{"-c", "claude"}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestJoinCommandQuotesSpaces(t *testing.T) {
	got := joinCommand("claude", []string{"--dir", "/a b/c"})
	want := "claude --dir '/a b/c'"
	if got != want {
		t.Fatalf("joinCommand = %q, want %q", got, want)
	}
}

func TestSingleQuoteEscapes(t *testing.T) {
	if got := singleQuote("a'b"); got != `'a'\''b'` {
		t.Fatalf("singleQuote = %q", got)
	}
}
