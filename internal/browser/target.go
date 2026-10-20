package browser

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"floter/internal/spawn"
)

// AutoTarget is the `target` setting's default: the first profile that
// actually has data wins.
const AutoTarget = "auto"

// DefaultProfile is the profile the launcher searches when the user has not
// picked one.
//
// A `target` naming a browser id picks that browser; in auto mode the first
// profile that actually has data wins — a browser that is installed but never
// opened has an empty `Default` and would otherwise shadow the one the user
// really browses with.
func DefaultProfile(profiles []Profile, target string) (Profile, bool) {
	target = strings.TrimSpace(target)
	if target != "" && target != AutoTarget {
		for _, profile := range profiles {
			if profile.BrowserID == target {
				return profile, true
			}
		}
	}
	for _, profile := range profiles {
		if profile.HasHistory() {
			return profile, true
		}
	}
	for _, profile := range profiles {
		if profile.HasBookmarks() {
			return profile, true
		}
	}
	if len(profiles) == 0 {
		return Profile{}, false
	}
	return profiles[0], true
}

// ErrNoBrowser is what opening a page reports when no browser is known.
var ErrNoBrowser = errors.New("browser: no browser to open the page in")

// OpenURL opens a page in the browser a browser id names, rather than in the
// system's default one. An empty id (or one the platform cannot target) opens
// it in the default browser, which is the shipped behaviour.
func OpenURL(browserID, url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return errors.New("browser: no URL to open")
	}
	candidates, err := openURLCandidates(runtime.GOOS, browserID, url)
	if err != nil {
		return err
	}
	var lastErr error
	for _, argv := range candidates {
		if err := spawn.Program(argv[0], argv[1:]...); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = ErrNoBrowser
	}
	return lastErr
}

// openURLCandidates is the argv that opens `url`, per platform, so the rule can
// be read (and exercised) without spawning a browser. The default opener is
// the app's one shared table (`spawn.OpenCandidates`), so a page and a path are
// opened by the same program on the same machine; only macOS adds a candidate
// in front of it.
//
// A URL is handed over as **one argument**, never through a shell: the port
// went through `cmd /c start`, which re-parses the string as a *command line* —
// `&` separates commands to `cmd.exe`, and a URL's query string is full of
// them, so `https://example.com/?a=1&b=2` opened `…?a=1` and tried to run
// `b=2`. The reference refuses `cmd /c start` for exactly this reason
// (`main:src-tauri/src/commands/actions.rs:20-32`).
//
// macOS tries the named application first and the default browser second: an
// `open -a <missing app>` only fails after `open` itself has started, so the
// fallback has to be a second spawn rather than an error from the first.
func openURLCandidates(goos, browserID, url string) ([][]string, error) {
	defaults := spawn.OpenCandidates(goos, url)
	if len(defaults) == 0 {
		return nil, fmt.Errorf("browser: opening %s is not supported on this platform", url)
	}
	if goos != "darwin" {
		return defaults, nil
	}
	if app, ok := macAppName(browserID); ok {
		candidates := [][]string{{"open", "-a", app, url}}
		return append(candidates, defaults...), nil
	}
	return defaults, nil
}
