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
	"time"
)

// Program starts a program detached, with no inherited standard streams. It
// reports the spawn's own error; the child runs on and is reaped in the
// background.
func Program(program string, args ...string) error {
	return ProgramIn("", program, args...)
}

// ProgramIn is Program, started in `dir`. An empty dir inherits this process's
// own, which is what every launch that has no opinion wants.
func ProgramIn(dir, program string, args ...string) error {
	if strings.TrimSpace(program) == "" {
		return errors.New("spawn: no program to start")
	}
	return start(programCmd(dir, program, args...))
}

// programCmd is the command line every launch path builds: the program's own
// argv, started in `dir` when it has an opinion, with no inherited streams.
func programCmd(dir, program string, args ...string) *exec.Cmd {
	cmd := exec.Command(program, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	return cmd
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
	return TerminalAt("")
}

// TerminalAt is Terminal opened *at* a directory: the active session's own
// (`terminalui`'s OSC 7 report), which is what a hand-off owes the user — a
// shell that starts where the work is, not in the app's own directory or the
// home the emulator would otherwise pick.
//
// The reference does the same thing twice over
// (`main:src-tauri/src/terminal/session.rs:1295-1380`): the emulator process is
// started with `current_dir(dir)`, and the emulators that do *not* start their
// shell in the process's directory — the ones that hand the window to a running
// server and so have no cwd to inherit, like gnome-terminal or konsole — get
// their own directory flag as well. An empty dir keeps the old behaviour
// exactly: a login shell wherever the emulator opens.
func TerminalAt(dir string) error {
	if emulator := strings.TrimSpace(os.Getenv("TERMINAL")); emulator != "" {
		// `$TERMINAL` is a *command line*, not a program name: the
		// documented way to pin a single-instance emulator
		// (`TERMINAL="kitty --single-instance"`) used to be looked up as
		// an executable called `kitty --single-instance`, which never
		// exists, so the variable was silently ignored.
		if argv := splitCommandLine(emulator); len(argv) > 0 {
			if err := startCandidate(dir, argv, true); err == nil {
				return nil
			}
		}
	}
	var lastErr error
	// The probe budget: each candidate is watched for its own early death (see
	// startWatched) only while the hand-off as a whole is still inside this
	// window, so a machine with a dozen broken emulators reports its failure in
	// about this long rather than a dozen times over.
	deadline := time.Now().Add(probeBudget)
	for _, candidate := range terminalLaunchersFor(runtime.GOOS, dir) {
		if err := startCandidate(dir, candidate, time.Now().Before(deadline)); err != nil {
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

// terminalLaunchersFor is the candidate table with a working directory in it:
// the same emulators, in the same order, each asked for `dir` the way its own
// command line spells it. A dir of "" is the bare table, byte for byte.
func terminalLaunchersFor(goos, dir string) [][]string {
	candidates := terminalEmulatorsFor(goos)
	if dir == "" {
		return candidates
	}
	launchers := make([][]string, 0, len(candidates))
	for _, candidate := range candidates {
		launchers = append(launchers, withDirectory(goos, candidate, dir))
	}
	return launchers
}

// withDirectory adds `dir` to one candidate's argv, for the emulators that do
// not inherit the process's directory; the rest are left alone because
// ProgramIn already starts them in it.
//
// The flags are the reference's own table, terminal by terminal
// (`session.rs:1319-1378`), so a machine whose emulator is any of these opens
// where the session was rather than in the home directory.
func withDirectory(goos string, candidate []string, dir string) []string {
	argv := append([]string(nil), candidate...)
	switch goos {
	case "darwin":
		// `open -b com.apple.Terminal <dir>`: the reference's own no-command
		// branch (`session.rs:1172-1180`) asks `open` for a window at the
		// directory, which the system Terminal opens a shell in.
		return append(argv, dir)
	case "windows":
		if len(argv) == 1 && argv[0] == "wt" {
			return append(argv, "-d", dir)
		}
		// `cmd /c start "" cmd` starts its console in cmd.exe's own
		// directory, which ProgramIn sets.
		return argv
	}
	switch argv[0] {
	case "gnome-terminal", "xfce4-terminal", "tilix", "terminator", "ghostty":
		return append(argv, "--working-directory="+dir)
	case "konsole":
		return append(argv, "--workdir", dir)
	case "kitty":
		return append(argv, "--directory", dir)
	case "alacritty":
		return append(argv, "--working-directory", dir)
	case "wezterm":
		return append(argv, "start", "--cwd", dir)
	}
	// xdg-terminal-exec, x-terminal-emulator, foot, urxvt and xterm start
	// their shell in the directory they were started in.
	return argv
}

// splitCommandLine splits a command line into a program and its arguments, for
// the values a user writes in `$TERMINAL`: whitespace separates words, single
// and double quotes group them, and a backslash escapes the next character
// outside single quotes. It is deliberately not a shell: nothing is expanded,
// nothing is run twice.
func splitCommandLine(line string) []string {
	var words []string
	var word strings.Builder
	started := false
	var quote byte
	flush := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			if c == '\\' && quote == '"' && i+1 < len(line) {
				i++
				word.WriteByte(line[i])
				continue
			}
			word.WriteByte(c)
		case c == '\'' || c == '"':
			quote = c
			started = true
		case c == '\\' && i+1 < len(line):
			i++
			word.WriteByte(line[i])
			started = true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
		default:
			word.WriteByte(c)
			started = true
		}
	}
	flush()
	return words
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

// earlyExitWindow is how long a *launcher* is watched for an early death, and
// probeBudget bounds the whole hand-off's watching (see TerminalAt).
//
// A terminal emulator that is on the machine but cannot open its window — no
// display, a single-instance server it cannot reach, a broken config — exits
// within a few milliseconds with a non-zero status, and `Start` reporting
// nothing is exactly the case the port used to call success: the user pressed
// 「在系统终端中继续」, no window appeared, and no message said why. The watch
// is short because it sits on the launch path: an emulator that is still alive
// after it is the window the user asked for.
const (
	earlyExitWindow = 150 * time.Millisecond
	probeBudget     = 400 * time.Millisecond
)

// startCandidate starts one emulator argv. With `watch` it waits out
// earlyExitWindow so a launcher that dies at once is reported (and the next
// candidate gets its turn) instead of being counted as a launch.
func startCandidate(dir string, argv []string, watch bool) error {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return errors.New("spawn: no program to start")
	}
	cmd := programCmd(dir, argv[0], argv[1:]...)
	if !watch {
		return start(cmd)
	}
	return startWatched(cmd)
}

// startWatched starts cmd and watches it for earlyExitWindow. A child that
// exits **0** inside the window is a hand-off, not a failure: the emulators
// that ask a running server for the window (gnome-terminal, konsole, kitty
// --single-instance) exit successfully the moment the server has it. A child
// that exits non-zero is the launch that never appeared, and its own error is
// returned so the caller can try the next candidate and, in the end, say why.
func startWatched(cmd *exec.Cmd) error {
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(earlyExitWindow):
		// Still alive: the window the user asked for. The goroutine above keeps
		// reaping it for as long as it lives.
		return nil
	}
}
