//go:build !windows

package session

import (
	"errors"
	"syscall"
)

// ProcessAlive reports whether the process with pid is still running. A pid of 0
// (or negative) means "unknown" and is reported as not-alive only by the caller's
// choosing — callers must treat pid<=0 as "no liveness signal", not "dead".
//
// On Unix, signal 0 probes existence without delivering anything: nil means the
// process exists; EPERM means it exists but is owned by another user (still
// alive); ESRCH means it's gone.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}
