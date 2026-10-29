package shell

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
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

	// Press: the panel's pinned band says it, the window grows for the band,
	// and no row joins the list (P11-bo).
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if hasLauncherItem(a, "system-terminal-bare") {
		t.Fatalf("the modifier added a row to the list: %+v", a.Launcher.Results())
	}
	if !tt.HasText("Open a terminal session") {
		t.Fatalf("the held band is missing: %q", tt.Texts())
	}
	if a.Launcher.Selected != 0 {
		t.Errorf("the press edge moved the selection to %d, want the page's own 0", a.Launcher.Selected)
	}
	grown := a.launcherTarget
	if grown <= base {
		t.Errorf("the window did not grow for the band: %d -> %d", base, grown)
	}

	// Release: the band is gone and the window is back at its own height.
	tt.HoldModifiers(0)
	tt.Frame()
	if tt.HasText("Open a terminal session") {
		t.Errorf("the band survived the release: %q", tt.Texts())
	}
	if a.launcherTarget != base {
		t.Errorf("the window did not recede: %d -> %d -> %d", base, grown, a.launcherTarget)
	}
}

// A typed query draws no held row at all: the reference's one placement is the
// empty field (R64, `main:src/launcher/terminal-row.ts:77-85` — 「前面加的按住
// 在列表额外增加终端选项的逻辑要去掉」). The port had kept R60's wider gesture,
// so with a query typed the action word appeared beside the matches — the report
// 「输入框中有输入内容时出现的「在终端中打开」项：现在固定在第一项（错误）」.
//
// The modifier still has a job on a typed query: it hands the selection to the
// action bar, the reference's `setSelectedActionBar(true)` on the keydown.
func TestATypedQueryDrawsNoHeldRow(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Type("git")
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	results := a.Launcher.Results()
	if hasLauncherItem(a, "system-terminal-bare") {
		t.Fatalf("a typed query drew the held row: %+v", results)
	}
	if tt.HasText("Open Terminal") {
		t.Errorf("the held row's action word is on the page over a query: %q", tt.Texts())
	}
	// The modifier's own hand-off: the bar takes the choice when the query
	// offers one (here `git` matches the tool row, so the bar is drawn).
	if bar := lastBarIndex(results); bar >= 0 && a.Launcher.Selected != bar {
		t.Errorf("the modifier left the selection on %d, want the action bar %d",
			a.Launcher.Selected, bar)
	}
}

// The row's one placement is the empty page's last line, and it keeps the
// selection there while the list is rebuilt under it (the empty page's list can
// change shape — a dropped file, a clipboard row — without the modifier moving).
func TestTheHeldBandIsTheEmptyPagesLastBand(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	rows := len(a.Launcher.Results())
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if got := len(a.Launcher.Results()); got != rows {
		t.Errorf("the modifier added %d row(s) to the empty page: %+v", got-rows, a.Launcher.Results())
	}
	if !tt.HasText("Open a terminal session") {
		t.Fatalf("the empty page drew no held band: %q", tt.Texts())
	}
	if a.Launcher.Selected != 0 {
		t.Errorf("the selection is on %d, want the page's own first row 0", a.Launcher.Selected)
	}
}

// lastBarIndex is the index of the action bar row in a list, or -1.
func lastBarIndex(results []launcher.Item) int {
	for i, item := range results {
		if strings.HasPrefix(item.ID, "bar:") {
			return i
		}
	}
	return -1
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
	a := appKeysAppWithRecents()
	tt := appKeysRender(t, a)
	start := a.Launcher.Selected

	// The press edge moves nothing: the held line is the band, not a row, so
	// there is no row to preselect (P11-bo, the user's F4-3 「不占排序」).
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if a.Launcher.Selected != start {
		t.Fatalf("the press edge moved the selection to %d, want %d", a.Launcher.Selected, start)
	}
	// The user steps with the arrows during the gesture: theirs stays.
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	moved := a.Launcher.Selected
	if moved == start {
		t.Fatal("the arrow did not move the selection; the test cannot prove its rule")
	}
	tt.HoldModifiers(0)
	tt.Frame()
	if a.Launcher.Selected != moved {
		t.Errorf("the release moved the selection to %d, want the user's %d", a.Launcher.Selected, moved)
	}

	// A press and release with no step leaves the page's own row alone.
	a.Launcher.Selected = recentRowIndex(a.Launcher, "Recent 00")
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	tt.HoldModifiers(0)
	tt.Frame()
	if want := recentRowIndex(a.Launcher, "Recent 00"); a.Launcher.Selected != want {
		t.Errorf("the release selection = %d, want the page's own %d", a.Launcher.Selected, want)
	}
}
