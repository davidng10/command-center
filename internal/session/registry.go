package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"command-center/internal/config"
)

// Registry is the persisted list of sessions (sessions.json). It is owned by the
// fleet TUI process; the `fleet hook` writer never touches it (hooks write to the
// state dir instead), so there is no cross-process write contention.
type Registry struct {
	path     string
	sessions []Session
}

// Load reads the registry from sessions.json. A missing file yields an empty
// registry; a malformed file is treated as empty rather than fatal, so a corrupt
// cache never wedges startup. The path is resolved once and reused for saves.
func Load() (*Registry, error) {
	path, err := config.SessionsPath()
	if err != nil {
		return nil, err
	}
	r := &Registry{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return r, nil // no file yet
	}
	_ = json.Unmarshal(data, &r.sessions)
	return r, nil
}

// All returns the sessions newest-first (by creation time).
func (r *Registry) All() []Session {
	out := make([]Session, len(r.sessions))
	copy(out, r.sessions)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// Get returns the session with id and whether it exists.
func (r *Registry) Get(id string) (Session, bool) {
	for _, s := range r.sessions {
		if s.ID == id {
			return s, true
		}
	}
	return Session{}, false
}

// FindByWorktree returns the session whose worktree path matches dir (cleaned),
// which is how a hook firing's cwd maps back to a fleet session.
func (r *Registry) FindByWorktree(dir string) (Session, bool) {
	want := filepath.Clean(dir)
	for _, s := range r.sessions {
		if filepath.Clean(s.WorktreePath) == want {
			return s, true
		}
	}
	return Session{}, false
}

// Add appends a session and persists.
func (r *Registry) Add(s Session) error {
	r.sessions = append(r.sessions, s)
	return r.save()
}

// Remove deletes the session with id and persists. A missing id is a no-op.
func (r *Registry) Remove(id string) error {
	out := r.sessions[:0]
	for _, s := range r.sessions {
		if s.ID != id {
			out = append(out, s)
		}
	}
	r.sessions = out
	return r.save()
}

// Update applies fn to the session with id and persists. It returns false (no
// save) when id is unknown.
func (r *Registry) Update(id string, fn func(*Session)) (bool, error) {
	for i := range r.sessions {
		if r.sessions[i].ID == id {
			fn(&r.sessions[i])
			return true, r.save()
		}
	}
	return false, nil
}

// save writes sessions.json atomically (temp + rename) so a crash mid-write can
// never leave a half-written registry.
func (r *Registry) save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r.sessions, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(r.path), ".sessions-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, r.path)
}
