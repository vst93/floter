package shell

import (
	"fmt"
	"testing"

	"floter/internal/apps"
	"floter/internal/launcher"
)

// The window's own height prices the action bar outside the ten-row budget,
// exactly as the frame does.
//
// The launcher's frame charges the footer as its own band (`Geometry.Height`,
// see the launcher's `TestTheActionBarIsChargedOutsideTheBudget`), but the
// shell sizes the window from `App.launcherHeight` — the summon target and the
// surface's resize. That one was handed the launcher's *capped* row count, so
// it priced the footer as one of the ten: a full list was a band short of the
// frame that drew it, and the 「在终端中打开」 row sat below the window's bottom
// edge again — the very row the report was about.
func TestTheFooterIsPricedOutsideTheBudgetOnTheWindowToo(t *testing.T) {
	a := appKeysApp()
	budget := launcher.HoldRows(0, 99) // the ten-row budget, as the launcher reports it
	found := make([]apps.App, budget)
	for i := range found {
		found[i] = apps.App{Name: fmt.Sprintf("app%d", i), Path: "/usr/bin/app"}
	}
	a.Launcher.SetApps(found)
	tt := appKeysRender(t, a)
	a.Launcher.SetQuery("app")
	tt.Frame()

	rows := a.Launcher.Rows
	if len(rows) != budget+1 {
		t.Fatalf("the list drew %d bands, want the budget plus the footer (%d): the query is not the full-list case", len(rows), budget+1)
	}
	geometry := func(n int) int {
		return launcher.Geometry{
			Font: a.Launcher.Font, Spacing: a.Launcher.Spacing,
			TitleLine: a.Launcher.TitleLine, SubtitleLine: a.Launcher.SubtitleLine,
			Rows: rows[:n], Held: n,
		}.Height()
	}
	resultsOnly := geometry(budget) // ten results, no footer
	want := resultsOnly + int(float64(a.Launcher.Spacing)*11.25)
	// One pixel of slack: each height rounds once, and the two roundings can
	// land either side of the same half.
	if got := a.launcherTarget; got < want-1 || got > want+1 {
		t.Errorf("the window is %d high, want %d (the ten results' bands plus the footer's own band)", got, want)
	}
	// The two paths agree: the frame's geometry and the shell's summon target
	// are the same number, so a summon cannot land a band short.
	if got := a.launcherHeight(a.Store.Snapshot()); got != a.launcherTarget {
		t.Errorf("launcherHeight = %d, the frame asked for %d", got, a.launcherTarget)
	}
}
