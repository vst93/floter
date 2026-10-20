package spawn

import (
	"reflect"
	"strings"
	"testing"
)

// The Linux table is the reference's own `TERMINALS`
// (`main:src-tauri/src/terminal/session.rs:1239-1254`): the desktop's configured
// terminal first, then the emulators the reference falls back to in its order.
// The port's table was a shorter hand-written one, so a machine whose emulator
// was `foot`, `tilix`, `terminator`, `urxvt` or the freedesktop
// `xdg-terminal-exec` got 「no terminal emulator was found」 while the reference
// opened it.
func TestTheLinuxTerminalCandidatesAreTheReferencesList(t *testing.T) {
	want := [][]string{
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
	if got := terminalEmulatorsFor("linux"); !reflect.DeepEqual(got, want) {
		t.Errorf("linux candidates are\n%v\nwant the reference's order\n%v", got, want)
	}
}

// macOS opens the system Terminal, which is the one that always exists: an
// `open -a <missing app>` reports no spawn error (open starts, then exits 1),
// so the first wrong guess ended the search and no terminal appeared.
func TestTheMacTerminalIsTheSystemTerminal(t *testing.T) {
	candidates := terminalEmulatorsFor("darwin")
	if len(candidates) == 0 {
		t.Fatal("no macOS terminal candidate")
	}
	if want := []string{"open", "-b", "com.apple.Terminal"}; !reflect.DeepEqual(candidates[0], want) {
		t.Errorf("macOS opens %v, want the system Terminal %v", candidates[0], want)
	}
}

// A console program started with `DETACHED_PROCESS` gets no console, so `cmd`
// on its own left the Windows hand-off with no window at all. Every Windows
// fallback has to ask the console host for one.
func TestTheWindowsTerminalCandidatesOpenAConsoleWindow(t *testing.T) {
	candidates := terminalEmulatorsFor("windows")
	if len(candidates) == 0 {
		t.Fatal("no Windows terminal candidate")
	}
	if want := []string{"wt"}; !reflect.DeepEqual(candidates[0], want) {
		t.Errorf("Windows tries %v first, want Windows Terminal %v", candidates[0], want)
	}
	for _, candidate := range candidates[1:] {
		if len(candidate) < 5 || candidate[0] != "cmd" || candidate[1] != "/c" || candidate[2] != "start" {
			t.Errorf("candidate %v does not open a console window (want `cmd /c start \"\" <program>`)", candidate)
			continue
		}
		if candidate[3] != "" {
			t.Errorf("candidate %v passes %q as start's window title, want the empty title", candidate, candidate[3])
		}
		if !strings.HasPrefix(candidate[4], "cmd") && !strings.HasPrefix(candidate[4], "powershell") {
			t.Errorf("candidate %v starts %q, want a shell", candidate, candidate[4])
		}
	}
}

// consoleWindowStart is the shape both Windows fallbacks share; pinned on its
// own so the argv above is read against one definition.
func TestConsoleWindowStartAsksForANewWindow(t *testing.T) {
	want := []string{"cmd", "/c", "start", "", "cmd"}
	if got := consoleWindowStart("cmd"); !reflect.DeepEqual(got, want) {
		t.Errorf("consoleWindowStart(cmd) = %v, want %v", got, want)
	}
}

// A hand-off has to start where the work is. The reference asks each emulator
// for a directory the way that emulator spells it
// (`main:src-tauri/src/terminal/session.rs:1319-1378`) and starts every one of
// them in the directory too, because the emulators whose window belongs to a
// running server (gnome-terminal, konsole) inherit no cwd.
func TestTheTerminalOpensInTheGivenDirectory(t *testing.T) {
	const dir = "/home/user/work"
	launchers := terminalLaunchersFor("linux", dir)
	want := map[string][]string{
		"gnome-terminal": {"gnome-terminal", "--working-directory=" + dir},
		"konsole":        {"konsole", "--workdir", dir},
		"kitty":          {"kitty", "--directory", dir},
		"alacritty":      {"alacritty", "--working-directory", dir},
		"wezterm":        {"wezterm", "start", "--cwd", dir},
		"xfce4-terminal": {"xfce4-terminal", "--working-directory=" + dir},
		"tilix":          {"tilix", "--working-directory=" + dir},
		"terminator":     {"terminator", "--working-directory=" + dir},
		"ghostty":        {"ghostty", "--working-directory=" + dir},
		// The rest start their shell in the directory they are started in.
		"xdg-terminal-exec":   {"xdg-terminal-exec"},
		"x-terminal-emulator": {"x-terminal-emulator"},
		"foot":                {"foot"},
		"urxvt":               {"urxvt"},
		"xterm":               {"xterm"},
	}
	seen := 0
	for _, launcher := range launchers {
		expected, known := want[launcher[0]]
		if !known {
			t.Errorf("launcher %v is in no table here", launcher)
			continue
		}
		seen++
		if !reflect.DeepEqual(launcher, expected) {
			t.Errorf("%s is started as %v, want %v", launcher[0], launcher, expected)
		}
	}
	if seen != len(want) {
		t.Errorf("only %d of %d emulators were checked", seen, len(want))
	}
}

// The macOS hand-off asks `open` for a window at the directory — the reference's
// own no-command branch — so the system Terminal opens a shell there.
func TestTheMacTerminalOpensAtTheDirectory(t *testing.T) {
	launchers := terminalLaunchersFor("darwin", "/tmp/work")
	if len(launchers) != 1 {
		t.Fatalf("macOS candidates are %v", launchers)
	}
	want := []string{"open", "-b", "com.apple.Terminal", "/tmp/work"}
	if !reflect.DeepEqual(launchers[0], want) {
		t.Errorf("macOS opens %v, want %v", launchers[0], want)
	}
}

// No directory keeps the table exactly as it was: a hand-off with nothing to
// carry over is the login shell the port has always opened.
func TestTerminalWithoutADirectoryIsTheBareTable(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		if got, want := terminalLaunchersFor(goos, ""), terminalEmulatorsFor(goos); !reflect.DeepEqual(got, want) {
			t.Errorf("%s with no directory is\n%v\nwant\n%v", goos, got, want)
		}
	}
}

// `$TERMINAL` is a command line. Looked up as a program name, the documented
// single-instance spelling (`TERMINAL="kitty --single-instance"`) is an
// executable that never exists, so the variable was silently skipped.
func TestTheTerminalEnvironmentIsACommandLine(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{"kitty", []string{"kitty"}},
		{"kitty --single-instance", []string{"kitty", "--single-instance"}},
		{`alacritty --class "my term"`, []string{"alacritty", "--class", "my term"}},
		{`wezterm start --cwd '/tmp/a b'`, []string{"wezterm", "start", "--cwd", "/tmp/a b"}},
		{`foot -e sh -c "echo hi"`, []string{"foot", "-e", "sh", "-c", "echo hi"}},
		{`  xterm  `, []string{"xterm"}},
		{`""`, []string{""}},
		{"", nil},
	} {
		if got := splitCommandLine(tc.line); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitCommandLine(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}
