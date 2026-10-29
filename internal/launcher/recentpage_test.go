package launcher

import (
	"fmt"

	"floter/internal/apps"
)

// recentPage fills a launcher with n recent applications and nothing else.
//
// P11-bk: the empty query is the recents' own block — the built-in commands are
// answers to a typed word, not suggestions (`search.go`'s `otherResults`, and
// the reference's `useLauncherCatalog.ts:1138-1166`). Pins that used to read
// their rows off the empty page's six commands now read the same row geometry
// off this fixture: the rows are the same widget, so the facts they pin (text
// column, row pill, pitch, badge, pointer entry) are unchanged; only where the
// rows come from moved.
//
// The rows carry a `Comment` so each one is the two-line shape the built-in
// rows were (`appItem`: a localized name suppresses the comment, a plain one
// keeps it as the subtitle).
func recentPage(a *App, n int) []apps.App {
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
	a.SetApps(found)
	a.SetRecent(paths, true)
	return found
}

// resultIndex is the position of the row with this title in the current
// results — the index `App.Selected` speaks in (it counts the section heading
// too; the keyboard steps over it).
func resultIndex(a *App, title string) int {
	for i, item := range a.Results() {
		if item.Title == title {
			return i
		}
	}
	return -1
}
