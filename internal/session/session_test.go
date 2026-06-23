package session

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestStateWireRoundTrip(t *testing.T) {
	for _, st := range []State{StateRunning, StateFinished, StateInactive} {
		data, err := json.Marshal(st)
		if err != nil {
			t.Fatal(err)
		}
		var got State
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", st, err)
		}
		if got != st {
			t.Fatalf("round-trip %s -> %s", st, got)
		}
		parsed, ok := ParseState(st.String())
		if !ok || parsed != st {
			t.Fatalf("ParseState(%q) = %s, %v", st.String(), parsed, ok)
		}
	}
	if _, ok := ParseState("bogus"); ok {
		t.Fatal("ParseState should reject unknown strings")
	}
}

func TestRegistryRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.All()) != 0 {
		t.Fatal("fresh registry should be empty")
	}

	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	if err := r.Add(Session{ID: "old", Branch: "b1", WorktreePath: "/wt/a", State: StateFinished, CreatedAt: older}); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(Session{ID: "new", Branch: "b2", WorktreePath: "/wt/b", State: StateRunning, CreatedAt: newer}); err != nil {
		t.Fatal(err)
	}

	// Reload from disk and confirm newest-first ordering survived.
	r2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	all := r2.All()
	if len(all) != 2 || all[0].ID != "new" || all[1].ID != "old" {
		t.Fatalf("ordering wrong: %+v", all)
	}

	// Map a hook cwd back to its session.
	s, ok := r2.FindByWorktree("/wt/a/")
	if !ok || s.ID != "old" {
		t.Fatalf("FindByWorktree = %+v, %v", s, ok)
	}

	// Activity update learns the agent session id and flips state.
	updated, ok, err := r2.ApplyActivity("/wt/b", "agent-xyz", StateRunning, newer)
	if err != nil || !ok {
		t.Fatalf("ApplyActivity ok=%v err=%v", ok, err)
	}
	if updated.AgentSession != "agent-xyz" || updated.State != StateRunning {
		t.Fatalf("activity not applied: %+v", updated)
	}

	if err := r2.Remove("old"); err != nil {
		t.Fatal(err)
	}
	if _, ok := r2.Get("old"); ok {
		t.Fatal("old should be gone after Remove")
	}
}

func TestProcessAliveSelf(t *testing.T) {
	// The test process itself is alive; pid 0 is "no signal" → not alive.
	if !ProcessAlive(os.Getpid()) {
		t.Fatal("current process should read as alive")
	}
	if ProcessAlive(0) {
		t.Fatal("pid 0 must not read as alive")
	}
}
