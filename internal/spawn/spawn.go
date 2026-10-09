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

// terminalEmulators lists the emulators to try, per platform, most likely
// first. The macOS terminals are applications, so `open -a` is the launch
// path; Terminal.app always exists.
func terminalEmulators() [][]string {
	switch runtime.GOOS {
	case "darwin":
		return [][]string{
			{"open", "-a", "iTerm"},
			{"open", "-a", "Ghostty"},
			{"open", "-a", "Alacritty"},
			{"open", "-a", "WezTerm"},
			{"open", "-a", "Terminal"},
		}
	case "windows":
		return [][]string{{"wt"}, {"cmd"}, {"powershell"}}
	default:
		return [][]string{
			{"ghostty"},
			{"kitty"},
			{"alacritty"},
			{"wezterm"},
			{"konsole"},
			{"gnome-terminal"},
			{"xfce4-terminal"},
			{"x-terminal-emulator"},
			{"xterm"},
		}
	}
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
