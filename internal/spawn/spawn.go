// Package spawn starts programs the app does not wait for.
//
// Every launch path in the app goes through here so the platform's rules live
// in one place: a child that outlives floter (a browser, a terminal, a user's
// command) gets its own session on Unix and no console of its own on Windows,
// and the app never blocks on it.
package spawn

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Program starts a program detached, with no inherited standard streams. It
// reports the spawn's own error; the child runs on and is reaped in the
// background.
func Program(program string, args ...string) error {
	if strings.TrimSpace(program) == "" {
		return errors.New("spawn: no program to start")
	}
	cmd := exec.Command(program, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	return start(cmd)
}

// Command runs a shell command line with no window and no terminal: the user's
// shell on Unix, cmd on Windows.
func Command(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("spawn: no command to run")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		shell := strings.TrimSpace(os.Getenv("SHELL"))
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd = exec.Command(shell, "-c", command)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	return start(cmd)
}

// Terminal opens the user's terminal emulator with a shell, trying
// `$TERMINAL` first and then the emulators the platform usually has.
func Terminal() error {
	if emulator := strings.TrimSpace(os.Getenv("TERMINAL")); emulator != "" {
		if err := Program(emulator); err == nil {
			return nil
		}
	}
	var lastErr error
	for _, candidate := range terminalEmulators() {
		if err := Program(candidate[0], candidate[1:]...); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("spawn: no terminal emulator was found")
	}
	return lastErr
}

// terminalEmulators lists the emulators to try on this platform.
func terminalEmulators() [][]string {
	return terminalEmulatorsFor(runtime.GOOS)
}

// terminalEmulatorsFor is terminalEmulators for a named platform, so the three
// tables are one list a test can read instead of a switch only the host's own
// branch ever reaches.
//
// The Linux table is the reference's own `TERMINALS`
// (`main:src-tauri/src/terminal/session.rs:1239-1254`) in its order, with
// `xdg-terminal-exec` first: it is the freedesktop launcher that asks the
// desktop's *configured* terminal, which is exactly the answer a launcher owes
// the user. The port's table was a shorter hand-written one (`ghostty` first,
// `xdg-terminal-exec`/`foot`/`tilix`/`terminator`/`urxvt` missing), so a machine
// whose terminal was one of those answered 「no terminal emulator was found」
// while the reference opened it.
//
// macOS opens the *system* Terminal the way the reference does
// (`open -b com.apple.Terminal`, `session.rs:1152-1180`): it is the one terminal
// that always exists, and `$TERMINAL` above still lets a user point at their
// own. The port tried `open -a iTerm`/`Ghostty`/… first, which reports no error
// at spawn time even when the application is not installed (`open` starts, then
// exits 1) — so the first wrong guess ended the search and no terminal opened.
//
// Windows cannot start `cmd` the way the other entries are started: see
// consoleWindowStart.
func terminalEmulatorsFor(goos string) [][]string {
	switch goos {
	case "darwin":
		return [][]string{{"open", "-b", "com.apple.Terminal"}}
	case "windows":
		return [][]string{
			{"wt"},
			consoleWindowStart("cmd"),
			consoleWindowStart("powershell"),
		}
	default:
		return [][]string{
			{"xdg-terminal-exec"},
			{"x-terminal-emulator"},
			{"gnome-terminal"},
			{"konsole"},
			{"xfce4-terminal"},
			{"kitty"},
			{"alacritty"},
			{"wezterm"},
			{"ghostty"},
			{"foot"},
			{"tilix"},
			{"terminator"},
			{"urxvt"},
			{"xterm"},
		}
	}
}

// consoleWindowStart is the argv that opens `program` in a console window of
// its own on Windows.
//
// `spawn` starts every child with `DETACHED_PROCESS` and null stdio (see
// detach), which is right for a browser or the user's application and wrong for
// a terminal: a console program started that way has no console to draw in, so
// the port's `Terminal()` ran `cmd` and the user saw nothing at all — the one
// hand-off whose whole point is a window. `cmd /c start "" <program>` has the
// shell ask the console host for a new window, and that window's stdin/stdout
// are the console's own rather than the null handles the app passes down.
func consoleWindowStart(program string) []string {
	return []string{"cmd", "/c", "start", "", program}
}

// start detaches the child and releases it.
func start(cmd *exec.Cmd) error {
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	// The child is not ours to reap: waiting in the background keeps a
	// long-lived browser or terminal from becoming a zombie of this process.
	go func() { _ = cmd.Wait() }()
	return nil
}
