// Package session defines fleet's provider-independent session model: the
// canonical activity State, the persisted Session record, and the Registry that
// loads/saves/reconciles them. Nothing here knows about Claude or any specific
// provider.
package session

import (
	"encoding/json"
	"fmt"
)

// State is the canonical, provider-independent activity state fleet renders.
type State int

const (
	StateRunning    State = iota // agent generating / working
	StateFinished                // idle, awaiting user input
	StateNeedsInput              // waiting on permission / approval
	StateInactive                // session ended or process gone
)

// stateNames is the on-disk / hook-arg spelling of each state. The wire form is
// a stable string (not the int) so sessions.json and `fleet hook <state>` stay
// readable and reorder-proof.
var stateNames = map[State]string{
	StateRunning:    "running",
	StateFinished:   "finished",
	StateNeedsInput: "needs-input",
	StateInactive:   "inactive",
}

// String returns the wire spelling ("running", …).
func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}
	return "inactive"
}

// Label returns the human badge text used in the UI.
func (s State) Label() string {
	switch s {
	case StateRunning:
		return "Running"
	case StateFinished:
		return "Finished"
	case StateNeedsInput:
		return "Needs input"
	default:
		return "Inactive"
	}
}

// ParseState maps a wire string (from a state file or `fleet hook <state>`) to a
// State. Unknown strings are treated as Inactive, and ok reports whether the
// string was recognized.
func ParseState(s string) (State, bool) {
	for st, name := range stateNames {
		if name == s {
			return st, true
		}
	}
	return StateInactive, false
}

// MarshalJSON writes the wire string.
func (s State) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON accepts the wire string.
func (s *State) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	st, ok := ParseState(str)
	if !ok {
		return fmt.Errorf("unknown session state %q", str)
	}
	*s = st
	return nil
}
