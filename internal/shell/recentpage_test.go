package shell

import (
	"fmt"

	"floter/internal/apps"
	"floter/internal/launcher"
)

// recentLauncher fills the launcher with n recent applications and nothing
// else.
//
// P11-bk: the empty query is the recents' own block — the six built-in
// commands are answers to a typed word, not suggestions (`launcher.search.go`'s
// `otherResults`). Pins that read their rows off the shipped empty page now
// read them off this fixture; the frame, the geometry and the painted bands are
// the same, only where the rows come from moved.
func recentLauncher(a *App, n int) {
	found := make([]apps.App, 0, n)
	paths := make([]string, 0, n)
	for i := 0; i < n; i++ {
		app := apps.App{
			Name:    fmt.Sprintf("Recent %02d", i),
			Path:    fmt.Sprintf("/Applications/Recent%02d.app", i),
			Comment: fmt.Sprintf("recent application %02d", i),
		}
		found = append(found, app)
		paths = append(paths, app.Path)
	}
	a.Launcher.SetApps(found)
	a.Launcher.SetRecent(paths, true)
}

// appKeysAppWithRecents is appKeysApp plus the empty page's own rows.
func appKeysAppWithRecents() *App {
	a := appKeysApp()
	recentLauncher(a, 6)
	return a
}

// recentRowIndex is the position of the row with this title in the launcher's
// current results — the index `App.Selected` speaks in (it counts the section
// heading too; the keyboard steps over it).
func recentRowIndex(l *launcher.App, title string) int {
	for i, item := range l.Results() {
		if item.Title == title {
			return i
		}
	}
	return 0
}
