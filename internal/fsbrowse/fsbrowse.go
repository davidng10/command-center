// Package fsbrowse supplies the data behind the /new wizard's directory step and
// base-branch step: listing a folder's sub-directories (marking git repos),
// detecting repos, and enumerating a repo's branches. It is pure data — the
// wizard owns the cursor/filter UI in the tui package.
package fsbrowse

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"command-center/internal/worktree"
)

// Entry is one sub-directory shown in the directory browser.
type Entry struct {
	Name   string // base name
	Path   string // absolute path
	IsRepo bool   // contains a .git entry (so it can be selected as a worktree source)
}

// List returns dir's immediate sub-directories, sorted case-insensitively, each
// flagged as a git repo or not. Hidden directories (dot-prefixed) and unreadable
// entries are skipped. Symlinks to directories are included.
func List(dir string) ([]Entry, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // hide dotfolders (incl. the .git of a repo itself)
		}
		full := filepath.Join(dir, name)
		if !isDir(full) {
			continue
		}
		out = append(out, Entry{Name: name, Path: full, IsRepo: IsRepo(full)})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// isDir reports whether path is a directory, following symlinks.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsRepo reports whether dir is the root of a git repo (has a .git entry — a
// directory for a normal clone, a file for a submodule/worktree).
func IsRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// Branch is one selectable base branch.
type Branch struct {
	Name   string // "main", "origin/main"
	Remote bool   // true for origin/* entries
}

// Branches lists a repo's local branches followed by its origin/* remotes,
// de-duplicated and ordered for the picker: a remembered/last-used base can be
// pinned by the caller. Local branches come first (most relevant), then remotes.
// On any git error it returns what it could gather (possibly empty) plus the err.
func Branches(repoDir string) ([]Branch, error) {
	var branches []Branch
	seen := map[string]bool{}
	add := func(name string, remote bool) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		branches = append(branches, Branch{Name: name, Remote: remote})
	}

	locals, lerr := worktree.Git(repoDir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	for _, line := range nonEmptyLines(locals) {
		add(line, false)
	}

	remotes, rerr := worktree.Git(repoDir, "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	for _, line := range nonEmptyLines(remotes) {
		if line == "origin/HEAD" || strings.HasSuffix(line, "/HEAD") {
			continue // the symbolic origin/HEAD isn't a real base
		}
		add(line, true)
	}

	if lerr != nil {
		return branches, lerr
	}
	return branches, rerr
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}
