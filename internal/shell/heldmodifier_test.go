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

// A typed query keeps the held row: it is the list's last line over the query
// too (the user's report 「输入内容之后按住 Super 键，底部应该固定出现「在终端
// 中打开」的选项」). The reference's R64 narrowed the row to the empty field;
// this build restores R60's wider gesture at the user's request, so the row —
// and the window grown for it — is present with the modifier down whatever the
// field holds.
func TestHeldModifierRowStaysOverAQuery(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Type("z")
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	results := a.Launcher.Results()
	if !hasLauncherItem(a, "system-terminal-bare") {
		t.Fatalf("the held row hid over a query: %+v", results)
	}
	if results[len(results)-1].ID != "system-terminal-bare" {
		t.Errorf("the held row is not the last line over a query: %+v", results)
	}
	if a.Launcher.Selected != len(results)-1 {
		t.Errorf("the selection is on %d, want the held row %d", a.Launcher.Selected, len(results)-1)
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

// The release edge resets the selection to the empty page's own default —
// the reference's R64 release edge (App.tsx: `setSelectedResultIndex(
// firstRunnableResultIndex < 0 ? 0 : ...)`), unconditionally: the effect's
// dependency list carries no selection, so an arrow step off the row during
// the gesture is discarded with the row.
func TestHeldModifierReleaseRestoresThePageDefault(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)

	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	results := a.Launcher.Results()
	if a.Launcher.Selected != len(results)-1 {
		t.Fatalf("the press edge did not select the held row")
	}
	// The user steps off with the arrows during the gesture: theirs now.
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	moved := a.Launcher.Selected
	if moved == len(results)-1 {
		t.Fatal("the arrow did not move the selection; the test cannot prove its rule")
	}

	tt.HoldModifiers(0)
	tt.Frame()
	if a.Launcher.Selected != moved {
		t.Errorf("the release selection = %d, want the default %d (the reset is unconditional)", a.Launcher.Selected, moved)
	}

	// Without the step, the release lands on the page's default too: the
	// first row the keyboard can act on (the reference's
	// `firstRunnableResultIndex`).
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	tt.HoldModifiers(0)
	tt.Frame()
	if a.Launcher.Selected != 0 {
		t.Errorf("the release selection = %d, want the page's first selectable row 0", a.Launcher.Selected)
	}
}
