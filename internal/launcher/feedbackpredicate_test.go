package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// P11-cm · **底部那条带的「有没有」只许有一个说法。**
//
// 同一个事实此前有三份手写拷贝：`View` 给窗口量高时的一份、`statusBand` 给列表
// 让位时的一份、`FeedbackVisible()` 交给 shell 量窗口时的一份 —— 而第三份漏了
// `pendingPower`。于是电源确认在场时，shell 量的窗口比画面自己量的矮 30u，
// 最低一行整行压在确认带底下（实测 720 宽：shell 532 / 画面 562，
// `Recent 09` 在 `506..530`、带子在 `506..528`）。三处现在都读 `feedbackVisible`。
//
// 这条 pin 读的不是某一个数，而是**几个来源的一致性**：四种状态逐个走一遍，
// 每次都要「shell 说带子在」==「画面按它长了多少」==「列表让位了」。

// bandHeight is the height the launcher charges with the band drawn or not:
// the same Geometry the shell builds in `launcherHeight`, and the same one
// `View` builds for `ResizeTo`.
func bandHeight(a *App, drawn bool) int {
	return Geometry{
		Font: a.Font, Spacing: a.Spacing,
		TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine,
		Rows: a.Rows, Held: len(a.Rows),
		Filter:   a.FiltersVisible(),
		Feedback: drawn,
		Config:   a.ConfigSchema(),
	}.Height()
}

func TestTheBandHasOnePredicate(t *testing.T) {
	a := testApp()
	recentPage(a, 8)
	var charged int
	a.Actions.ResizeTo = func(h int) { charged = h }
	// The list's own reservation is read inside the frame: a Context is the
	// frame's, not the test's.
	var reserved float32
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
		reserved = a.statusBand(c)
	}, InputWindowWidth, frameHeight(maxResultSlots))
	tt.Frame()

	// The four states, one row each: the band is the same 30u whatever is
	// asking, and every reader of the question has to agree on all four.
	for _, tc := range []struct {
		name string
		set  func()
		undo func()
		want bool
	}{
		{"nothing is docked", func() {}, func() {}, false},
		{"a toast", func() { a.Feedback("copied") }, func() { a.toast = "" }, true},
		{"a power confirmation", func() { a.runPower(PowerRestart) }, func() { a.pendingPower = "" }, true},
		{"the modifier held", func() { tt.HoldModifiers(ui.Cmd) }, func() { tt.HoldModifiers(0) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.set()
			tt.Frame()

			// 1. the shell's own reader (`FeedbackVisible`, which `shell.go`'s
			// `launcherHeight` and `targetSize` call before any frame ran).
			if shell := a.FeedbackVisible(); shell != tc.want {
				t.Fatalf("FeedbackVisible = %v, want %v (this is the number the shell sizes the window with)", shell, tc.want)
			}

			// 2. the list's own bottom padding (`statusBand`), which is what
			// keeps a row from hiding under the band (P11-az).
			if got := reserved > 0; got != tc.want {
				t.Fatalf("the list reserves the band = %v, want %v", got, tc.want)
			}

			// 3. the window the frame actually asked for: it grows by exactly
			// one 30u band when the band is owed, and by nothing when it is not.
			// This is the half the shell reads *and* the half the draw asks for,
			// so one number that disagrees with the other cannot pass here.
			base := bandHeight(a, false)
			if got := charged - base; tc.want && got != 30 {
				t.Fatalf("the frame asked for %d, a band over the base %d is %d (want the 30u band)", charged, base, got)
			} else if !tc.want && got != 0 {
				t.Fatalf("the frame asked for %d, the base is %d (want no band)", charged, base)
			}
			// And the shell's reader charges the same one band.
			if tall := bandHeight(a, true); tc.want && tall-base != 30 {
				t.Errorf("the band is %d tall, want the 30u status band", tall-base)
			}

			tc.undo()
			tt.Frame()
		})
	}
}

// The rule the three copies were there to protect, read on the frame: **no row
// ends under the band.** It is the P11-az rule, and the power confirmation was
// the one sentence that broke it again — the band was drawn and the list
// reserved for it, but the window was charged a band short, so the lowest row
// slid under it.
//
// This is the pin that goes red on the old predicate: revert `feedbackVisible`
// to `toast || heldHint` and the lowest row ends at 530 inside the band's
// 506..528.
func TestTheChargedHeightLeavesNoRowUnderTheBand(t *testing.T) {
	a := testApp()
	recentPage(a, 12)
	var charged int
	a.Actions.ResizeTo = func(h int) { charged = h }
	tt := render(t, a)
	tt.Frame()
	a.runPower(PowerRestart)
	tt.Frame()
	tt.Frame()
	if !a.FeedbackVisible() {
		t.Fatal("a pending power confirmation is not charged a band: the shell's window is a band short")
	}
	if charged <= 0 {
		t.Fatalf("the app asked for no window height")
	}

	// Draw at exactly the height the app asked the window for — the frame the
	// user sees, not one the test chose.
	tt.SetSize(InputWindowWidth, charged)
	tt.Frame()
	tt.Frame()

	band, ok := tt.Find(a.powerMessage())
	if !ok {
		t.Fatalf("the confirmation is not drawn: %q", tt.Texts())
	}
	// Every row of the list stops at or above the band's top edge.
	for _, item := range a.Results() {
		if item.heading || item.status {
			continue
		}
		rect, ok := tt.Find(item.Title)
		if !ok {
			continue // scrolled out of the frame is fine; *under* the band is not
		}
		if bottom := rect.Y + rect.H; bottom > band.Y {
			t.Errorf("%q ends at %.0f, under the band's %.0f (window %d): the row is drawn and cannot be clicked",
				item.Title, bottom, band.Y, charged)
		}
	}
}
