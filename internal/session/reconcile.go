package session

import "time"

// ApplyActivity records an activity-state update for the session living in the
// given worktree (the cwd a provider's tracker reports). It also learns the
// provider-native session id on first sighting. Returns the updated session and
// whether a matching session was found.
func (r *Registry) ApplyActivity(worktree, agentSession string, st State, at time.Time) (Session, bool, error) {
	var found Session
	ok, err := r.updateByWorktree(worktree, func(s *Session) {
		s.State = st
		s.LastActivity = at
		if agentSession != "" {
			s.AgentSession = agentSession
		}
		found = *s
	})
	return found, ok, err
}

func (r *Registry) updateByWorktree(worktree string, fn func(*Session)) (bool, error) {
	s, ok := r.FindByWorktree(worktree)
	if !ok {
		return false, nil
	}
	return r.Update(s.ID, fn)
}

// launchGrace is how long a brand-new session is protected from liveness
// downgrades. The launcher PID (osascript/wt) exits almost immediately; giving
// the hooks a few seconds to fire avoids a momentary flash to Inactive before the
// first UserPromptSubmit arrives. (GUI-terminal launches now return PID 0 which
// skips liveness entirely, so this is a safety net for custom terminal templates
// that might still return a launcher PID.)
const launchGrace = 10 * time.Second

// ReconcileLiveness downgrades to Inactive any session whose spawned process is
// known to be gone. It is deliberately conservative:
//   - PID <= 0 (common for GUI-terminal spawns): left untouched — those rely on
//     the provider's SessionEnd hook.
//   - Session already confirmed by a hook (AgentSession set): left untouched —
//     trust the hook-derived state over a launcher PID.
//   - Session created within the launch grace period: left untouched — the
//     launcher may have exited but the agent hasn't had a chance to fire its
//     first hook yet.
//
// Returns whether anything changed.
func (r *Registry) ReconcileLiveness() (bool, error) {
	now := time.Now()
	changed := false
	for i := range r.sessions {
		s := &r.sessions[i]
		if s.State == StateInactive || s.PID <= 0 {
			continue
		}
		if s.AgentSession != "" {
			continue
		}
		if now.Sub(s.CreatedAt) < launchGrace {
			continue
		}
		if !ProcessAlive(s.PID) {
			s.State = StateInactive
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	return true, r.save()
}
