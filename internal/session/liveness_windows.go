//go:build windows

package session

import "syscall"

// stillActive is the exit code Windows reports for a process that has not yet
// exited (STILL_ACTIVE). A process that genuinely exits with code 259 will read
// as alive — acceptable for this best-effort liveness probe.
const stillActive = 259

// processQueryLimitedInformation is the minimal access right needed to query a
// process's exit code; it works across privilege boundaries better than the
// broader PROCESS_QUERY_INFORMATION.
const processQueryLimitedInformation = 0x1000

// ProcessAlive reports whether the process with pid is still running. pid<=0 is
// "no liveness signal" and reported as not-alive (callers decide what that means).
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
