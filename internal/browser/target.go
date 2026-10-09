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

// FindProfile resolves a profile key back to a profile, so an action can use
// the browser the row came from.
func FindProfile(profiles []Profile, key string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.Key() == key {
			return profile, true
		}
	}
	return Profile{}, false
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
	switch runtime.GOOS {
	case "darwin":
		if app, ok := macAppName(browserID); ok {
			if err := spawn.Program("open", "-a", app, url); err == nil {
				return nil
			}
		}
		return spawn.Program("open", url)
	case "windows":
		// `start` is a cmd builtin, not an executable; the empty argument is
		// the window-title slot, which `start` would otherwise take the URL
		// for.
		return spawn.Program("cmd", "/c", "start", "", url)
	case "linux":
		return spawn.Program("xdg-open", url)
	default:
		return fmt.Errorf("browser: opening %s is not supported on this platform", url)
	}
}
