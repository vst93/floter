package spawn

import (
	"reflect"
	"testing"
)

// One target, one argument: the reference hands the string to ShellExecute /
// `open` / `xdg-open` verbatim, so a URL's query string and a path's spaces
// stay inside one argv entry and never reach a shell.
func TestAnOpenedTargetIsOneArgument(t *testing.T) {
	const target = "https://example.com/?a=1&b=2"
	for _, goos := range []string{"darwin", "windows", "linux"} {
		for _, argv := range OpenCandidates(goos, target) {
			if len(argv) < 2 || argv[len(argv)-1] != target {
				t.Errorf("%s candidate %v does not end in the target as one argument", goos, argv)
			}
			for _, arg := range argv {
				switch arg {
				case "cmd", "/c", "start", "sh", "-c", "/bin/sh":
					t.Errorf("%s candidate %v re-parses the target through a shell", goos, argv)
				}
			}
		}
	}
}

// Windows opens through ShellExecute, which is the reference's call
// (`main:src-tauri/src/commands/actions.rs:20-32`): `rundll32
// url.dll,FileProtocolHandler` is that call reached by name, and `explorer` —
// the file manager, which splits on commas and always exits 1 — is only the
// fallback.
func TestWindowsOpensThroughShellExecute(t *testing.T) {
	const target = "https://example.com/?a=1&b=2"
	want := [][]string{
		{"rundll32", "url.dll,FileProtocolHandler", target},
		{"explorer", target},
	}
	if got := OpenCandidates("windows", target); !reflect.DeepEqual(got, want) {
		t.Errorf("windows candidates are %v, want %v", got, want)
	}
}

// Linux has the reference's `open` crate fallbacks, so a machine without
// `xdg-open` (a minimal install, a container, a bare WM session) still opens
// the page or the folder instead of answering 「could not open」.
func TestLinuxOpensPastXdgOpen(t *testing.T) {
	const target = "/home/user/Documents"
	want := [][]string{
		{"xdg-open", target},
		{"gio", "open", target},
		{"gnome-open", target},
		{"kde-open5", target},
		{"wslview", target},
	}
	if got := OpenCandidates("linux", target); !reflect.DeepEqual(got, want) {
		t.Errorf("linux candidates are %v, want the reference's order %v", got, want)
	}
}

// macOS has one opener, and it takes the target as it is.
func TestMacOSOpensWithOpen(t *testing.T) {
	want := [][]string{{"open", "/Users/user/Notes"}}
	if got := OpenCandidates("darwin", "/Users/user/Notes"); !reflect.DeepEqual(got, want) {
		t.Errorf("darwin candidates are %v, want %v", got, want)
	}
}

// A platform with no opener table is an error, not a silent success: the caller
// (a dropped file's row, a page's open action) has a sentence to show.
func TestAnUnsupportedPlatformHasNoOpener(t *testing.T) {
	if got := OpenCandidates("plan9", "/tmp"); len(got) != 0 {
		t.Errorf("plan9 offered %v", got)
	}
	if err := Open("  "); err == nil {
		t.Error("an empty target was accepted")
	}
}
