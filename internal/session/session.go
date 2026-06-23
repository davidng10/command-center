package session

import "time"

// Session is one agent working in one worktree — the unit shown on the home
// page and the row persisted in sessions.json.
type Session struct {
	ID           string    `json:"id"`           // fleet's short id
	Provider     string    `json:"provider"`     // "claude" | "codex" | …
	Branch       string    `json:"branch"`       // the branch fleet created
	RepoDir      string    `json:"repoDir"`      // source repo root
	Base         string    `json:"base"`         // branch the worktree forked from
	WorktreePath string    `json:"worktreePath"` // the worktree's absolute path (maps hook cwd → session)
	Setup        string    `json:"setup"`        // setup command run before launch ("" = skipped)
	AgentSession string    `json:"agentSession"` // provider-native id, learned from the first hook event; may be ""
	PID          int       `json:"pid"`          // spawned process handle for liveness; 0 when unobtainable
	State        State     `json:"state"`
	LastActivity time.Time `json:"lastActivity"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Age returns how long ago the session was created, as a compact label
// ("now", "4m", "2h", "3d").
func (s Session) Age(now time.Time) string {
	return compactDuration(now.Sub(s.CreatedAt))
}

// ActivityAge returns how long since the last activity, as a compact label.
func (s Session) ActivityAge(now time.Time) string {
	if s.LastActivity.IsZero() {
		return s.Age(now)
	}
	return compactDuration(now.Sub(s.LastActivity))
}

func compactDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return formatUnit(int(d.Minutes()), "m")
	case d < 24*time.Hour:
		return formatUnit(int(d.Hours()), "h")
	default:
		return formatUnit(int(d.Hours()/24), "d")
	}
}

func formatUnit(n int, unit string) string {
	if n < 1 {
		n = 1
	}
	return itoa(n) + unit
}

// itoa avoids pulling in strconv for a single small positive int.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
