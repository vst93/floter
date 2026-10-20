package browser

import (
	"reflect"
	"testing"
)

// Windows hands the URL to ShellExecute, which is the reference's opener
// (`main:src-tauri/src/commands/actions.rs:20-32`); `rundll32
// url.dll,FileProtocolHandler` is that call by name, and `explorer` — the file
// manager, which splits on commas and always exits 1 — is the fallback. The port
// went through `cmd /c start`, and `cmd.exe` re-parses that string as a command
// line: `&` separates commands even inside quotes, so a page whose query string
// is `?a=1&b=2` opened `…?a=1` and tried to run `b=2`. The URL has to reach the
// opener as one argument and never through a shell.
func TestWindowsOpensAURLThroughTheShellExecuteOpener(t *testing.T) {
	const url = "https://example.com/?a=1&b=2"
	candidates, err := openURLCandidates("windows", "", url)
	if err != nil {
		t.Fatalf("windows refused the URL: %v", err)
	}
	want := [][]string{
		{"rundll32", "url.dll,FileProtocolHandler", url},
		{"explorer", url},
	}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("windows candidates are %v, want %v", candidates, want)
	}
	for _, argv := range candidates {
		for _, arg := range argv {
			if arg == "cmd" || arg == "/c" || arg == "start" {
				t.Errorf("candidate %v re-parses the URL through cmd.exe", argv)
			}
		}
	}
}

// A named macOS browser is tried first and the default browser is the fallback:
// `open -a <missing app>` fails only after `open` started, so a single spawn
// cannot tell the two apart.
func TestMacOSFallsBackToTheDefaultBrowser(t *testing.T) {
	candidates, err := openURLCandidates("darwin", BrowserChrome, "https://example.com")
	if err != nil {
		t.Fatalf("darwin refused the URL: %v", err)
	}
	want := [][]string{
		{"open", "-a", "Google Chrome", "https://example.com"},
		{"open", "https://example.com"},
	}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("darwin candidates are %v, want %v", candidates, want)
	}
	fallback, err := openURLCandidates("darwin", "", "https://example.com")
	if err != nil {
		t.Fatalf("darwin refused the URL without a browser id: %v", err)
	}
	if want := [][]string{{"open", "https://example.com"}}; !reflect.DeepEqual(fallback, want) {
		t.Errorf("darwin without a browser id opens %v, want %v", fallback, want)
	}
}

// Linux starts with `xdg-open` and keeps the reference `open` crate's fallbacks,
// so a machine without it still opens the page.
func TestLinuxOpensAURLWithXdgOpen(t *testing.T) {
	candidates, err := openURLCandidates("linux", BrowserChrome, "https://example.com/?a=1&b=2")
	if err != nil {
		t.Fatalf("linux refused the URL: %v", err)
	}
	want := [][]string{
		{"xdg-open", "https://example.com/?a=1&b=2"},
		{"gio", "open", "https://example.com/?a=1&b=2"},
		{"gnome-open", "https://example.com/?a=1&b=2"},
		{"kde-open5", "https://example.com/?a=1&b=2"},
		{"wslview", "https://example.com/?a=1&b=2"},
	}
	if !reflect.DeepEqual(candidates, want) {
		t.Errorf("linux candidates are %v, want %v", candidates, want)
	}
}

// The unsupported platforms still build; the action reports that there is no way
// to open a page instead of spawning something arbitrary.
func TestAnUnsupportedPlatformRefusesTheURL(t *testing.T) {
	if _, err := openURLCandidates("plan9", "", "https://example.com"); err == nil {
		t.Error("plan9 accepted a URL")
	}
}
