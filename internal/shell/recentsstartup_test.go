package shell

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/apps"
	"floter/internal/extensions"
	"floter/internal/settings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// recentsApp is the app a fresh start is: a store, a fixture for every
// directory, and the application scan handed to the shell so the scan's
// arrival can be played without a filesystem walk. The returned slice pointer
// is what the next scan answers with.
func recentsApp(t *testing.T, store *settings.Store, found *[]apps.App) *App {
	t.Helper()
	return New(Options{
		Store: store,
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return nil, errors.New("no terminal library in tests")
		},
		OpenAtLogin:    func() bool { return false },
		SetOpenAtLogin: func(bool) error { return nil },
		ScanApps:       func() []apps.App { return *found },
	})
}

// recentHeading reports whether the empty query's most-used block is on the
// page, and the rows under it.
func recentHeading(a *App) (bool, []string) {
	heading := false
	var rows []string
	for _, item := range a.Launcher.Results() {
		if item.ID == "section:recent" {
			heading = true
			continue
		}
		if heading {
			rows = append(rows, item.Title)
		}
	}
	return heading, rows
}

// A cold start's empty query offers the most-used applications.
//
// The scan is the moment both facts are known — the installed list and, with
// it, which of the launched paths still exist — so that is when the recents are
// ranked. The shell used to take the application list alone: `App.ShowRecent`
// stayed at its zero value (which reads as "the switch is off") until the user
// launched something in that session, so every launch after a restart showed an
// empty page where the user's habits belong.
func TestAFreshStartOffersTheMostUsedApps(t *testing.T) {
	editor := apps.App{Name: "Editor", Path: "/Applications/Editor.app"}
	safari := apps.App{Name: "Safari", Path: "/Applications/Safari.app"}
	found := []apps.App{editor, safari}
	store := settings.NewStore(settings.Default())
	a := recentsApp(t, store, &found)
	// The usage file survived the last run: two launches of Safari, one of the
	// editor.
	for i := 0; i < 2; i++ {
		if err := a.Usage.Record(safari.Path); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Usage.Record(editor.Path); err != nil {
		t.Fatal(err)
	}

	a.scanApps()

	if !a.Launcher.ShowRecent {
		t.Error("the first scan left the recents switch off: show_recent_in_launcher defaults to true")
	}
	if got := a.Launcher.Recent; len(got) != 2 || got[0] != safari.Path {
		t.Fatalf("recents after the first scan = %v, want [%s %s]", got, safari.Path, editor.Path)
	}
	heading, rows := recentHeading(a)
	if !heading {
		t.Fatal("the empty query has no most-used block after a cold start")
	}
	if len(rows) != 2 || rows[0] != "Safari" {
		t.Errorf("the most-used block reads %v, want Safari first", rows)
	}
	// And it is on the screen the user reads, not only in the slice: the empty
	// page paints the section heading and the row.
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()
	if !tt.HasText("Recently launched") {
		t.Error("the empty launcher painted no most-used heading after a cold start")
	}
	if !tt.HasText("Safari") {
		t.Error("the empty launcher painted no most-used row after a cold start")
	}
}

// The round's visual evidence: the empty query's most-used block as it is
// painted, in both appearances. Opt-in (`FLOTER_PNG=1`) so a normal run does
// not litter /tmp — the round that wrote the report ran it once with the
// switch on.
func TestDumpRecentsPagePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		editor := apps.App{Name: "Editor", Path: "/Applications/Editor.app"}
		safari := apps.App{Name: "Safari", Path: "/Applications/Safari.app"}
		notes := apps.App{Name: "Notes", Path: "/Applications/Notes.app"}
		found := []apps.App{editor, safari, notes}
		a := recentsApp(t, store, &found)
		for path, launches := range map[string]int{safari.Path: 4, notes.Path: 2, editor.Path: 1} {
			for i := 0; i < launches; i++ {
				if err := a.Usage.Record(path); err != nil {
					t.Fatal(err)
				}
			}
		}
		a.scanApps()
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P9-p-recents-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		img := tt.Image()
		t.Logf("%s (%dx%d) recents=%v", path, img.Rect.Dx(), img.Rect.Dy(), a.Launcher.Recent)
		// The control the fix is measured against: the same page with the
		// switch off, i.e. what the cold start painted before this round.
		if err := store.Update(func(s *settings.Settings) { s.SetShowRecentInLauncher(false) }); err != nil {
			t.Fatal(err)
		}
		a.refreshRecents()
		// The same tester re-renders the same closure: a second Tester in one
		// test would bind the same handles to two windows.
		tt.Frame()
		controlPath := filepath.Join("/tmp", "floter-P9-p-recents-"+appearance+"-off.png")
		out, err := os.Create(controlPath) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(out, tt.Image()); err != nil {
			out.Close()
			t.Fatal(err)
		}
		out.Close()
		t.Logf("%s (switch off)", controlPath)
	}
}

// The switch is honoured on the first scan, not only when it is toggled.
//
// A user who turned `show_recent_in_launcher` off reads an empty page with no
// most-used block — and the switch has to hold from the first frame, which is
// the same first scan.
func TestTheRecentsSwitchIsHonouredOnTheFirstScan(t *testing.T) {
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.SetShowRecentInLauncher(false) }); err != nil {
		t.Fatal(err)
	}
	editor := apps.App{Name: "Editor", Path: "/Applications/Editor.app"}
	found := []apps.App{editor}
	a := recentsApp(t, store, &found)
	if err := a.Usage.Record(editor.Path); err != nil {
		t.Fatal(err)
	}

	a.scanApps()

	if a.Launcher.ShowRecent {
		t.Error("show_recent_in_launcher false did not reach the launcher on the first scan")
	}
	if heading, rows := recentHeading(a); heading || len(rows) != 0 {
		t.Errorf("the empty query still shows the most-used block: %v", rows)
	}
}

// A rescan re-derives the recents from the list it just found.
//
// The tray's Rescan is the user's own "I installed something" press. An app
// that was already launched in an earlier session has a launch count waiting
// for it, and an app that is gone must not keep its slot.
func TestARescanRebuildsTheRecents(t *testing.T) {
	editor := apps.App{Name: "Editor", Path: "/Applications/Editor.app"}
	safari := apps.App{Name: "Safari", Path: "/Applications/Safari.app"}
	found := []apps.App{safari}
	store := settings.NewStore(settings.Default())
	a := recentsApp(t, store, &found)
	if err := a.Usage.Record(editor.Path); err != nil {
		t.Fatal(err)
	}
	if err := a.Usage.Record(safari.Path); err != nil {
		t.Fatal(err)
	}

	a.scanApps()
	if got := a.Launcher.Recent; len(got) != 1 || got[0] != safari.Path {
		t.Fatalf("recents = %v, want the installed Safari only", got)
	}

	// The editor is installed between the two scans: its count is waiting, so
	// it takes the slot it earned.
	found = []apps.App{editor, safari}
	a.scanApps()
	if got := a.Launcher.Recent; len(got) != 2 || got[0] != editor.Path {
		t.Fatalf("recents after installing the editor = %v, want it first", got)
	}

	// And uninstalling Safari drops it, rather than leaving a dead path.
	found = []apps.App{editor}
	a.scanApps()
	if got := a.Launcher.Recent; len(got) != 1 || got[0] != editor.Path {
		t.Errorf("recents after uninstalling Safari = %v, want the editor only", got)
	}
}
