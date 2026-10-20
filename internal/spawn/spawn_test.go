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
