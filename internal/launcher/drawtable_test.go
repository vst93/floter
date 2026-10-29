package launcher

// P11-cn · 本帧的动作读本帧那张表。
//
// `View` 每帧只走一遍目录，那是 P11-bq 起的一条规矩（`App.searchRuns` 有 pin 守着）。
// 这条规矩有一个形状：**「本帧要用哪几行」这个问题，在一帧之内只能问目录一次**。
// P11-cl 修掉了指针点击行那一支在绘制期又走一遍的漏网；P11-cn 收掉同族剩下的三支
// ——方向键、历史星标的 ⌘D、星标自己那一下点击——它们都在一帧之内第二次调用
// `a.Results()`，而那一遍全量匹配（`Catalog()` 重建 + 每条打分 + 拼音）买到的信息和
// 本帧已经建好的那张表逐条相同。
//
// 为什么 P11-cl 的计数 pin 没抓住它们：那个 pin 只加了「点一下行」一臂，而键盘那一支
// 改的是 `nextSelectableIn` 本身——`move` 却仍走 `a.nextSelectable`，它自己再
// `a.Results()` 一遍。于是**键盘那一支从来没有真的改到**，而 pin 看不见。
// 本文件把这三支各钉一臂：动作落点（行为）**且**那一帧每遍 draw 的搜索数（代价）。
// 只读其中一半都守不住——只读落点，第二遍全量匹配照样在；只读次数，跑错行也照样绿。

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// drawSearchOnce is the counting half: run the action, draw, and report every
// draw's search count for that frame. Two things are read, and both matter:
//
//   - every draw searched the catalog exactly once (a draw that searched twice
//     walked it again *inside* itself), and
//   - the whole press cost exactly as many walks as it drew (a press that
//     searched more than it drew walked the catalog on the *event*, for a row
//     that was already on screen).
//
// The second half is why the delete key's arm exists: its cost lands on the key
// event, not inside the draw, so a per-draw count alone would miss it.
func drawSearchOnce(t *testing.T, name string, a *App, tt *ui.Tester, per *[]int, act func()) {
	t.Helper()
	*per = (*per)[:0]
	before := a.searchRuns
	act()
	tt.Frame()
	if len(*per) == 0 {
		t.Fatalf("%s: the frame drew nothing", name)
	}
	draws := 0
	for i, searches := range *per {
		draws += searches
		if searches != 1 {
			t.Errorf("%s: the frame's draw %d searched %d times, want 1 — the action walked "+
				"the catalog again inside the draw it had already built (see App.drawn)",
				name, i+1, searches)
		}
	}
	if total := a.searchRuns - before; total != draws {
		t.Errorf("%s: the press walked the catalog %d times over %d draws — %d of those "+
			"happened on the key event, for a row the draw had already built",
			name, total, len(*per), total-draws)
	}
}

// TestTheArrowsReadTheTableTheDrawBuilt is the keyboard arm. The list the arrows
// step through is the one on screen, so the frame's own table answers it: the
// selection moves one selectable row (never onto a heading), and the frame that
// moved it pays one catalog walk.
func TestTheArrowsReadTheTableTheDrawBuilt(t *testing.T) {
	a := testApp()
	recentPage(a, 8)
	tt, per := countingRender(t, a)
	tt.Frame()

	for _, step := range []struct {
		name  string
		delta int
		key   ui.Key
	}{
		{"down", 1, ui.KeyDown},
		{"up", -1, ui.KeyUp},
	} {
		t.Run(step.name, func(t *testing.T) {
			before := a.Selected
			want := nextSelectableIn(a.drawn, before, step.delta)
			drawSearchOnce(t, "the "+step.name+" arrow", a, tt, per, func() { tt.Key(0, step.key) })
			if a.Selected != want {
				t.Errorf("the %s arrow landed on row %d, want %d: the selection is not "+
					"stepping through the table the draw built", step.name, a.Selected, want)
			}
			if a.drawn[a.Selected].heading {
				t.Errorf("the %s arrow landed on a block heading at %d", step.name, a.Selected)
			}
		})
	}
}

// TestTheStarsChordReadsTheTableTheDrawBuilt is the ⌘D arm: the chord stars the
// row the user is looking at — the row of the table the frame drew — and the
// frame that spends the claim pays one catalog walk.
func TestTheStarsChordReadsTheTableTheDrawBuilt(t *testing.T) {
	a, store := clipboardApp(t)
	tt, per := countingRender(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	row := a.Selected
	entry, ok := a.selectedClip()
	if !ok {
		t.Fatalf("no clipboard row is selected at %d", row)
	}
	drawSearchOnce(t, "the star's chord", a, tt, per, func() { tt.Key(historyStarMods, ui.KeyD) })
	if a.selectedClipMust(t) != entry.ID {
		t.Errorf("the chord moved the selection off the row it starred")
	}
	starred := false
	for _, got := range store.Entries() {
		if got.ID == entry.ID {
			starred = got.Favorite
		}
	}
	if !starred {
		t.Errorf("the chord did not star the row it was looking at: %+v", store.Entries())
	}
}

// selectedClipMust is the row the selection is on, as its entry id, for the
// assertion above.
func (a *App) selectedClipMust(t *testing.T) string {
	t.Helper()
	entry, ok := a.selectedClip()
	if !ok {
		t.Fatalf("the selection is not on a clipboard row (Selected=%d, rows=%d)", a.Selected, len(a.drawn))
	}
	return entry.ID
}

// TestTheStarsClickReadsTheTableTheDrawBuilt is the pointer arm the star
// shares: the click fires *inside* the draw, so it must read the table the
// draw built — the same rule P11-cl gave the row's own click. The star clicked
// is the one on the selected row, so the entry starred is that row's.
func TestTheStarsClickReadsTheTableTheDrawBuilt(t *testing.T) {
	a, store := clipboardApp(t)
	tt, per := countingRender(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	entry, ok := a.selectedClip()
	if !ok {
		t.Fatalf("no clipboard row is selected at %d", a.Selected)
	}
	drawSearchOnce(t, "the star's click", a, tt, per, func() {
		if err := tt.Click("Favorite"); err != nil {
			t.Fatalf("the star: %v", err)
		}
	})
	starred := false
	for _, got := range store.Entries() {
		if got.ID == entry.ID {
			starred = got.Favorite
		}
	}
	if !starred {
		t.Errorf("the click did not star the row its star belongs to: %+v", store.Entries())
	}
}

// TestTheDeleteKeyReadsTheTableTheDrawBuilt is the arm that shares
// `selectedHistoryRow` with the star's chord: the delete key acts on the row
// on screen, and the frame that arms it pays one catalog walk.
func TestTheDeleteKeyReadsTheTableTheDrawBuilt(t *testing.T) {
	a, store := clipboardApp(t)
	tt, per := countingRender(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	entry, ok := a.selectedClip()
	if !ok {
		t.Fatalf("no clipboard row is selected at %d", a.Selected)
	}
	drawSearchOnce(t, "the delete key", a, tt, per, func() { tt.Key(historyDeleteMods, ui.KeyBackspace) })
	if a.ArmedDelete() != "clip:"+entry.ID {
		t.Errorf("the delete key armed %q, want the row on screen (clip:%s)",
			a.ArmedDelete(), entry.ID)
	}
	// The second press removes that row and nothing else.
	drawSearchOnce(t, "the delete key's confirmation", a, tt, per, func() {
		tt.Key(historyDeleteMods, ui.KeyBackspace)
	})
	for _, got := range store.Entries() {
		if got.ID == entry.ID {
			t.Errorf("the confirmed delete left the row behind: %+v", store.Entries())
		}
	}
}
