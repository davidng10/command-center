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

// ReconcileLiveness downgrades to Inactive any session whose spawned process is
// known to be gone. It is deliberately conservative: a session with no captured
// PID (pid<=0, common for GUI-terminal spawns) is left untouched, because the
// absence of a PID is not evidence of death — those rely on the provider's
// SessionEnd hook instead (§9). Returns whether anything changed.
func (r *Registry) ReconcileLiveness() (bool, error) {
	changed := false
	for i := range r.sessions {
		s := &r.sessions[i]
		if s.State == StateInactive || s.PID <= 0 {
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
