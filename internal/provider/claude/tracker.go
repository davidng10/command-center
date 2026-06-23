package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"command-center/internal/config"
	"command-center/internal/provider"
	"command-center/internal/session"
)

// pollInterval is how often the tracker re-scans the state dir. Sub-second so the
// home view feels live, but coarse enough to be cheap (the dir holds a handful of
// tiny files). This is the design's "poll fallback" for D-4 — no fsnotify dep.
const pollInterval = 700 * time.Millisecond

// tracker turns the per-session state files (written by `fleet hook`) into a
// stream of canonical StateUpdates. It detects changes by file mtime, so each
// hook firing surfaces once.
type tracker struct{}

func (tracker) Updates(ctx context.Context) <-chan provider.StateUpdate {
	ch := make(chan provider.StateUpdate)
	go func() {
		defer close(ch)
		dir, err := config.StateDir()
		if err != nil {
			return
		}
		seen := map[string]time.Time{}
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			scan(ctx, dir, seen, ch)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return ch
}

// scan emits an update for every state file whose mtime changed since last seen.
func scan(ctx context.Context, dir string, seen map[string]time.Time, ch chan<- provider.StateUpdate) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // dir not created yet — nothing to report this tick
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if prev, ok := seen[e.Name()]; ok && prev.Equal(info.ModTime()) {
			continue // unchanged
		}
		seen[e.Name()] = info.ModTime()

		upd, ok := readStateFile(filepath.Join(dir, e.Name()), info.ModTime())
		if !ok {
			continue
		}
		select {
		case ch <- upd:
		case <-ctx.Done():
			return
		}
	}
}

// readStateFile parses one state file into a StateUpdate.
func readStateFile(path string, mtime time.Time) (provider.StateUpdate, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return provider.StateUpdate{}, false
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return provider.StateUpdate{}, false
	}
	st, ok := session.ParseState(sf.State)
	if !ok {
		return provider.StateUpdate{}, false
	}
	at := mtime
	if t, err := time.Parse(time.RFC3339, sf.At); err == nil {
		at = t
	}
	return provider.StateUpdate{
		Cwd:          sf.Cwd,
		AgentSession: sf.AgentSession,
		State:        st,
		At:           at,
	}, true
}
