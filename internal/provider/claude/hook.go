package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"command-center/internal/config"
)

// hookPayload is the subset of Claude Code's hook stdin JSON that fleet needs.
// Claude sends session_id, transcript_path, cwd, and hook_event_name on every
// hook (§9); we only read cwd (→ worktree) and session_id (→ agent-native id).
type hookPayload struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Event     string `json:"hook_event_name"`
}

// stateFile is fleet's transient per-session activity record, written by
// `fleet hook` and read by the tracker. cwd maps it to a fleet session.
type stateFile struct {
	Cwd          string `json:"cwd"`
	AgentSession string `json:"agentSession"`
	State        string `json:"state"`
	At           string `json:"at"` // RFC3339
}

// HandleHook implements the `fleet hook <state>` subcommand. It reads the hook
// payload from r, then atomically writes the session's current state to
// ~/.config/fleet/state/<session_id>.json. It is deliberately tolerant: a hook
// must never fail the agent's turn, so a malformed payload or unwritable state
// dir is reported as an error to the caller (main prints it to stderr) but the
// agent is unaffected either way.
func HandleHook(stateArg string, r io.Reader) error {
	if !stateArgs[stateArg] {
		return fmt.Errorf("unknown hook state %q (want running|finished|needs-input|inactive)", stateArg)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading hook payload: %w", err)
	}
	var p hookPayload
	// Empty/garbled stdin still produces a (cwd-less) record rather than crashing.
	_ = json.Unmarshal(raw, &p)

	dir, err := config.StateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	sf := stateFile{
		Cwd:          p.Cwd,
		AgentSession: p.SessionID,
		State:        stateArg,
		At:           time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, stateFileName(p)), data)
}

// stateFileName keys the file by the agent-native session id, falling back to a
// sanitized cwd when the id is absent, so concurrent sessions never collide.
func stateFileName(p hookPayload) string {
	key := p.SessionID
	if key == "" {
		key = "cwd-" + sanitizeKey(p.Cwd)
	}
	return sanitizeKey(key) + ".json"
}

// sanitizeKey makes an arbitrary string safe as a single path segment.
func sanitizeKey(s string) string {
	repl := func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}
	out := strings.Map(repl, s)
	if out == "" {
		return "unknown"
	}
	return out
}

// atomicWrite writes data to path via a temp file + rename.
func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}
