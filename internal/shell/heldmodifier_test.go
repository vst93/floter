package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The held app modifier's row, end to end: pressing it puts a bare
// "open terminal" row at the list's end, preselects it and grows the window;
// releasing it drops the row and the window recedes.
//
// The recede is the report: the row is one result, so the window's row count
// went up by one on the press and back down by one on the release. The old
// `HoldRows` kept a one-row margin on the way out, so the window stayed a row
// too tall - exactly the report 「松手后窗口高度没有回退」. The reference
// tracks the content exactly (R66: the app's `launcherListUnits(displayedResults)`
// with no `resolveLauncherRows`), and this build now does too.
func TestHeldModifierRowGrowsTheWindowAndRecedes(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)

	base := a.launcherTarget
	if base == 0 {
		t.Fatal("the launcher did not report a height")
	}

	// Press: the row appears, the selection goes to it, the window grows.
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if !hasLauncherItem(a, "system-terminal-bare") {
		t.Fatalf("the held row is missing: %+v", a.Launcher.Results())
	}
	results := a.Launcher.Results()
	if a.Launcher.Selected != len(results)-1 {
		t.Errorf("the selection is on %d, want the held row %d", a.Launcher.Selected, len(results)-1)
	}
	grown := a.launcherTarget
	if grown <= base {
		t.Errorf("the window did not grow: %d -> %d", base, grown)
	}

	// Release: the row is gone and the window is back at its own height.
	tt.HoldModifiers(0)
	tt.Frame()
	if hasLauncherItem(a, "system-terminal-bare") {
		t.Errorf("the row survived the release: %+v", a.Launcher.Results())
	}
	if a.launcherTarget != base {
		t.Errorf("the window did not recede: %d -> %d -> %d", base, grown, a.launcherTarget)
	}
}

// A typed query draws no row, whatever the modifier does - the reference's
// R64 rule (`bareTerminalPlacement`: `query.trim().length > 0 ? null`). The
// report read this as a deviation; it is the reference's own behaviour, pinned
// here so a later round cannot quietly change it.
func TestHeldModifierRowFollowsTheReferenceQueryGate(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.HoldModifiers(ui.Cmd)
	tt.Type("z")
	tt.Frame()
	if hasLauncherItem(a, "system-terminal-bare") {
		t.Errorf("the held row showed over a query: %+v", a.Launcher.Results())
	}
}

func hasLauncherItem(a *App, id string) bool {
	for _, item := range a.Launcher.Results() {
		if item.ID == id {
			return true
		}
	}
	return false
}
