// Package term spawns an agent in a NEW terminal window so fleet's own TUI keeps
// terminal #0 (§11). The mechanism is a configurable command template with
// per-OS defaults (decision D-1). PID capture through a GUI terminal is
// unreliable, so Spawn does not promise a usable agent PID — liveness leans on
// the provider's SessionEnd hook instead (D-2).
package term

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Spawn launches `program args...` in a new terminal whose working directory is
// dir. template is the user's config.terminal command (with {dir} and {cmd}
// placeholders); when empty, a per-OS default is used. It returns the spawned
// launcher's PID purely for diagnostics — callers should treat it as
// non-authoritative for liveness (see package doc).
func Spawn(dir, program string, args []string, template string) (int, error) {
	name, cmdArgs, err := resolve(template, dir, program, args)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(name, cmdArgs...)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("spawn terminal %q: %w", name, err)
	}
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	// Reap the launcher so it doesn't linger as a zombie; the GUI terminal it
	// opened lives on independently.
	go func() { _ = cmd.Wait() }()
	return pid, nil
}

// resolve turns (template, dir, program, args) into the executable + args to run.
func resolve(template, dir, program string, args []string) (string, []string, error) {
	cmdline := joinCommand(program, args)
	if strings.TrimSpace(template) != "" {
		name, a := expandTemplate(template, dir, program, args, cmdline)
		if name == "" {
			return "", nil, fmt.Errorf("terminal template produced no command: %q", template)
		}
		return name, a, nil
	}
	return defaultCommand(dir, cmdline)
}

// expandTemplate splits a user template on spaces and substitutes placeholders.
// A bare {cmd} field expands to program + args (so `… -- {cmd}` runs correctly);
// otherwise {dir}/{cmd} are substituted as substrings within a field.
func expandTemplate(template, dir, program string, args []string, cmdline string) (string, []string) {
	fields := strings.Fields(template)
	var out []string
	for _, f := range fields {
		switch f {
		case "{cmd}":
			out = append(out, program)
			out = append(out, args...)
		default:
			f = strings.ReplaceAll(f, "{dir}", dir)
			f = strings.ReplaceAll(f, "{cmd}", cmdline)
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return "", nil
	}
	return out[0], out[1:]
}

// defaultCommand is fleet's best-effort per-OS terminal launcher.
func defaultCommand(dir, cmdline string) (string, []string, error) {
	switch runtime.GOOS {
	case "darwin":
		// AppleScript opens a new Terminal.app window and runs the command in dir.
		script := fmt.Sprintf(`tell application "Terminal" to do script "cd %s && exec %s"`,
			singleQuote(dir), cmdline)
		return "osascript", []string{"-e", script}, nil
	case "windows":
		// Windows Terminal opens a tab in dir; cmd /k keeps it open after exit.
		if path, err := exec.LookPath("wt"); err == nil {
			return path, []string{"-d", dir, "cmd", "/k", cmdline}, nil
		}
		return "cmd", []string{"/c", "start", "", "/D", dir, "cmd", "/k", cmdline}, nil
	default: // linux & friends
		shell := fmt.Sprintf("cd %s && exec %s", singleQuote(dir), cmdline)
		if t := firstOnPath("x-terminal-emulator", "gnome-terminal", "konsole", "alacritty", "kitty", "xterm"); t != "" {
			return t, []string{"-e", "sh", "-c", shell}, nil
		}
		return "", nil, fmt.Errorf("no terminal emulator found on PATH; set `terminal` in config.json")
	}
}

// joinCommand renders program + args as a single shell command line, quoting any
// argument that contains whitespace.
func joinCommand(program string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, program)
	for _, a := range args {
		if strings.ContainsAny(a, " \t") {
			parts = append(parts, singleQuote(a))
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}

// singleQuote wraps s in single quotes, escaping embedded single quotes — safe
// for POSIX shells and AppleScript do-script strings.
func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func firstOnPath(cands ...string) string {
	for _, c := range cands {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}
