// Package provider is the Codex-ready seam: the core never names a concrete
// agent. A Provider supplies how to launch an agent in a worktree and how to
// learn its activity state. Claude Code is the first provider (sub-package
// claude); the registry below is what makes adding a second one additive.
package provider

import (
	"context"
	"time"

	"command-center/internal/session"
)

// Launch is the command to run an agent in a worktree.
type Launch struct {
	Program string   // e.g. "claude"
	Args    []string // extra args, if any
	Env     []string // extra environment (KEY=VALUE), merged onto the parent's
	Dir     string   // working directory (the worktree path)
}

// Provider encapsulates everything agent-specific: launch + state integration.
type Provider interface {
	// Name is the stable identifier persisted on a Session ("claude" | "codex").
	Name() string

	// LaunchSpec returns the command to run the agent for a session.
	LaunchSpec(s session.Session) Launch

	// Install ensures the provider's state integration exists (e.g. Claude's
	// hooks in ~/.claude/settings.json). MUST be idempotent.
	Install() error

	// Uninstall removes only what Install added.
	Uninstall() error

	// Installed reports whether the integration is currently present, so
	// onboarding/`/setup` can show accurate status and decide whether to re-run.
	Installed() bool

	// Tracker supplies activity-state updates for this provider's sessions. May
	// be a no-op tracker, in which case sessions rely on liveness only (§7).
	Tracker() StateTracker
}

// StateUpdate is a single observed activity transition. Cwd maps it back to a
// worktree (and thus a Session); AgentSession is the provider-native id, learned
// on the first event.
type StateUpdate struct {
	Cwd          string
	AgentSession string
	State        session.State
	At           time.Time
}

// StateTracker streams canonical state transitions as the agent works.
type StateTracker interface {
	// Updates emits StateUpdates until ctx is cancelled, then closes the channel.
	Updates(ctx context.Context) <-chan StateUpdate
}

// PreflightItem is one prerequisite check shown during onboarding's install step
// (e.g. "claude found", "settings.json writable").
type PreflightItem struct {
	OK    bool
	Label string
}

// Preflighter is an optional capability: a provider that can report install
// prerequisites for onboarding to display before offering Install(). Keeping it
// behind a type assertion lets the TUI stay provider-agnostic — it never names
// Claude-specific concepts (§14).
type Preflighter interface {
	Preflight() []PreflightItem
}

// NoopTracker is a StateTracker that never emits — for providers with no
// activity integration. Such providers degrade to Active/Inactive via liveness.
type NoopTracker struct{}

// Updates returns an immediately-closed channel.
func (NoopTracker) Updates(ctx context.Context) <-chan StateUpdate {
	ch := make(chan StateUpdate)
	close(ch)
	return ch
}
