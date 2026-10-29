package launcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/drops"
	"floter/internal/settings"
)

// P11-bu · **文件页也没有两段字压在同一块像素上，也没有一段字跑出窗口。**
//
// The audit (`auditLauncherFrame`) has been reading the ordinary result pages
// (P11-aj), the three plugin mode pages (P11-aw) and the settings pages
// (P11-aj) for several rounds now. The files page — the one a drop opens — was
// only ever measured by a probe that ran once and was deleted, so the one page
// whose rows are three per file, whose titles are `action + file name` and
// whose tail row collapses the rest of the drop had no standing pin.
//
// Its rows are unlike every other list in the app: a file contributes three
// rows that share a `Detail` (its directory, a path that is long by nature),
// the cap at `MaxVisibleDroppedFiles` adds one "…and N more files" row that is
// an ordinary result, and the header row above them is a group heading. A
// directory path that overruns its row, or a heading that lands on the first
// file's title, would never fail a test — `Tester.Texts` reports runs the
// layout clipped just as happily as runs it painted.
//
// The fixture carries one file name long enough to be ellipsized on purpose:
// that is where a title and its trailing keycap meet. A full drop does not fit
// the ten-row window, so the `scrolled` arms walk the list to its end, where
// the expander row is: that row exists only in the layout the list can reach.
func TestTheFilesPagePaintsNoTextOverText(t *testing.T) {
	const longName = "quarterly-report-2026-with-an-extremely-long-file-name-that-ellipsizes-in-its-row-final.pdf"
	const longDir = "/home/someone/Documents/clients/acme/2026/quarterly/exports/archive/final"
	files := []drops.File{
		{Path: longDir + "/" + longName, Name: longName, Directory: longDir},
		{Path: "/tmp/report.pdf", Name: "report.pdf", Directory: "/tmp"},
		{Path: "/tmp/notes.md", Name: "notes.md", Directory: "/tmp"},
		{Path: "/tmp/screenshot.png", Name: "screenshot.png", Directory: "/tmp"},
		{Path: "/tmp/invoice-2026-10.pdf", Name: "invoice-2026-10.pdf", Directory: "/tmp"},
		{Path: "/tmp/photo.jpg", Name: "photo.jpg", Directory: "/tmp"},
		{Path: "/tmp/budget.xlsx", Name: "budget.xlsx", Directory: "/tmp"},
	}
	// A state is a page, and whether the frame is read at the list's own top
	// or after a scroll to its end.
	type state struct {
		name   string
		scroll bool
		build  func(t *testing.T, lang string) *App
	}
	states := []state{
		// The page with nothing dropped on it: its own empty state.
		{name: "empty", build: func(t *testing.T, lang string) *App {
			return filesPageApp(t, lang, nil, false)
		}},
		// Seven files: five listed, two behind the "…and N more" row.
		{name: "collapsed", build: func(t *testing.T, lang string) *App {
			return filesPageApp(t, lang, files, false)
		}},
		{name: "collapsed-scrolled", scroll: true, build: func(t *testing.T, lang string) *App {
			return filesPageApp(t, lang, files, false)
		}},
		// The same drop after the expander ran: every file's three rows.
		{name: "expanded", build: func(t *testing.T, lang string) *App {
			return filesPageApp(t, lang, files, true)
		}},
		{name: "expanded-scrolled", scroll: true, build: func(t *testing.T, lang string) *App {
			return filesPageApp(t, lang, files, true)
		}},
		// A needle that matches none of the drop: the failed-query state of
		// this page (P11-bh gave it the ordinary sentence).
		{name: "no-match", build: func(t *testing.T, lang string) *App {
			a := filesPageApp(t, lang, files, false)
			a.Query = "zzzz"
			return a
		}},
	}
	sizes := [][2]int{
		{InputWindowWidth, frameHeight(maxResultSlots)},
		{InputWindowWidth, 360}, // settings.MinTerminalHeight, the shortest the window opens
	}
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			for _, size := range sizes {
				for _, s := range states {
					tag := fmt.Sprintf("%s %s %dx%d %s", lang, appearance, size[0], size[1], s.name)
					t.Run(tag, func(t *testing.T) {
						a := s.build(t, lang)
						tt := renderAt(t, a, size[0], size[1], dark)
						auditLauncherFrame(t, tag, tt)
						if !s.scroll {
							return
						}
						// The end of the list: the expander, or the last
						// file's third row.
						scrollListToEnd(t, tt)
						auditLauncherFrame(t, tag+" scrolled", tt)
					})
				}
			}
		}
	}
}

// filesPageApp is the launcher in files mode, in one language, over one drop.
func filesPageApp(t *testing.T, lang string, files []drops.File, expanded bool) *App {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
		t.Fatal(err)
	}
	if files != nil {
		a.SetDropped(files)
	}
	a.EnterFiles()
	a.dropsExpanded = expanded
	return a
}

// The audit is only meaningful if the page actually draws the rows it is
// auditing: a list that silently rendered empty would pass the overlap check
// for the wrong reason. This is also the arm that notices the expander row
// disappearing (`…and N more files`), which is where the dropped cap is read.
func TestTheFilesPageAuditDrawsRealRows(t *testing.T) {
	longName := "a-file-name-long-enough-that-its-row-ellipsizes-before-the-keycap.pdf"
	files := []drops.File{
		{Path: "/tmp/" + longName, Name: longName, Directory: "/tmp"},
		{Path: "/tmp/report.pdf", Name: "report.pdf", Directory: "/tmp"},
		{Path: "/tmp/notes.md", Name: "notes.md", Directory: "/tmp"},
		{Path: "/tmp/screenshot.png", Name: "screenshot.png", Directory: "/tmp"},
		{Path: "/tmp/invoice-2026-10.pdf", Name: "invoice-2026-10.pdf", Directory: "/tmp"},
		{Path: "/tmp/photo.jpg", Name: "photo.jpg", Directory: "/tmp"},
		{Path: "/tmp/budget.xlsx", Name: "budget.xlsx", Directory: "/tmp"},
	}
	a := filesPageApp(t, "en", files, false)
	copy := a.copy()
	tt := renderAt(t, a, InputWindowWidth, frameHeight(maxResultSlots), false)
	// The first page of the list is the heading and the first files.
	for _, want := range []string{copy.FilesSection, "Open  report.pdf"} {
		if !tt.HasText(want) {
			t.Errorf("the files page does not draw %q; it drew %q", want, tt.Texts())
		}
	}
	// The seventh file is behind the expander, not on the page.
	if tt.HasText("Open  budget.xlsx") {
		t.Errorf("the collapsed files page already lists the file past the cap")
	}

	// The end of the list: the expander row, with its own sentence.
	scrollListToEnd(t, tt)
	if !tt.HasText(copy.FilesMore(2)) {
		t.Fatalf("the collapsed files page does not draw %q; it drew %q", copy.FilesMore(2), tt.Texts())
	}
	if !tt.HasText(copy.FilesMoreHint) {
		t.Errorf("the expander row lost its hint; the page drew %q", tt.Texts())
	}

	// Running it reveals the rest and takes the expander away.
	if err := tt.Click(copy.FilesMore(2)); err != nil {
		t.Fatalf("expander: %v", err)
	}
	tt.Frame()
	scrollListToEnd(t, tt)
	if !tt.HasText("Open  budget.xlsx") {
		t.Errorf("the expanded files page still hides the last file; it drew %q", tt.Texts())
	}
	if tt.HasText(copy.FilesMore(2)) {
		t.Errorf("the expanded files page still shows the expander row")
	}
}

// scrollListToEnd walks the list to its last row the way the wheel does: the
// pointer over the list, a few notches down, and the settle the scroll's own
// animation needs before the next frame is read.
func scrollListToEnd(t *testing.T, tt *ui.Tester) {
	t.Helper()
	b := tt.Image().Bounds()
	x, y := float32(b.Max.X/2), float32(b.Max.Y*3/4)
	tt.Move(x, y)
	tt.Frame()
	for i := 0; i < 3; i++ {
		tt.Wheel(x, y, 0, 400)
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
	}
}
