package spawn

import (
	"errors"
	"runtime"
	"strings"
)

// Open hands a path or a URL to the platform's own opener: the default
// application for a file, the file manager for a directory, the browser for a
// page. It is the reference's `open::that` (`main:src-tauri/src/commands/
// actions.rs:20-32` for a URL, `:73-98` for a path), and it is one table for
// both because the reference has one: ShellExecute on Windows, `open` on
// macOS, `xdg-open` and its kin on Linux all take a path and a URL alike.
//
// The port had two hand-written openers — `shell.systemOpener()` for paths
// (`explorer`, `open`, `xdg-open`) and `browser.openURLCandidates` for pages —
// so the two could disagree about the same machine, and neither had the
// reference's fallbacks. The report 「Windows 下几乎没有办法正常使用」 is
// partly this shape: a machine whose `xdg-open` is missing (a minimal Linux
// install, a container) answered 「could not open」 while the reference's
// `open` crate went on to `gio open`.
//
// Every candidate is one program plus one argument, so nothing is re-parsed as
// a command line: a URL's `?a=1&b=2` stays one argument (see OpenCandidates).
func Open(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("spawn: nothing to open")
	}
	var lastErr error
	for _, argv := range OpenCandidates(runtime.GOOS, target) {
		if err := Program(argv[0], argv[1:]...); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("spawn: no opener was found")
	}
	return lastErr
}

// OpenCandidates is the argv that opens `target`, per platform, so the table
// can be read and exercised without spawning anything.
//
// Windows: `rundll32 url.dll,FileProtocolHandler <target>` is `ShellExecute`
// reached by name — the same call the reference makes — and `explorer` is the
// fallback the port used alone. `explorer` is not equivalent: it is the
// *file manager*, it splits its own argument list on commas, and it always
// exits 1, so it is a plausible opener rather than a faithful one.
//
// Linux: the reference's `open` crate order — `xdg-open`, then `gio open`,
// `gnome-open`, `kde-open5`, `wslview`. The port stopped at `xdg-open`, which
// a minimal install does not have.
//
// macOS has one opener and it never misses: `open`.
func OpenCandidates(goos, target string) [][]string {
	switch goos {
	case "darwin":
		return [][]string{{"open", target}}
	case "windows":
		return [][]string{
			{"rundll32", "url.dll,FileProtocolHandler", target},
			{"explorer", target},
		}
	case "linux":
		return [][]string{
			{"xdg-open", target},
			{"gio", "open", target},
			{"gnome-open", target},
			{"kde-open5", target},
			{"wslview", target},
		}
	default:
		return nil
	}
}
