package launcher

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
)

// P11-bb · **编号只落在「读得全」的那几行上。**
//
// 用户原话：「Super+N 按键在滚动时的渲染是有 bug 的，应该从列表的可见项开始赋予
// 1，一直到 9 或者 0」「滚动时快捷键跟着跳，且超出屏幕的项也显示快捷键」
// 「需要设一个阈值，比如某一项滚出可见范围达到 30% 就算作不可见项」。
//
// 框架的 `ListState.Visible()` 只答「还有一个像素在框里」（`mygo/ui/list.go`
// 的 `y+ht(i) > top && y < bottom`），所以一行只露出 2px 也算可见、也领到 `1`；
// 它的徽章在行中线，早被列表裁掉——用户看到的第一枚徽章是 `2`，按 `Super+1`
// 跑的却是屏幕外那一行。参考实现 `visibleRowRange`
// （`main:src/launcher/result-budget.ts:768-810`）明写跳过被上边缘裁掉的行，
// 本仓移植时丢了这一条。
//
// 这条 pin 判两件事：① 带 `1` 的行至少有七成在视口里；② 在它上面没有
// 「读得全却没领号」的可运行行。把 `numberVisibleShare` 改成 0（旧行为）会让
// ① 红。
func TestTheNumberedFamilyStartsOnARowYouCanRead(t *testing.T) {
	// The user's own threshold, spelled out here rather than read from the
	// product's constant: 「某一项滚出可见范围达到 30% 就算作不可见项」. The
	// fixture search below must not move with the rule it is testing.
	const readable = 0.7
	a := testApp()
	found := make([]apps.App, 0, 30)
	for i := 0; i < 30; i++ {
		found = append(found, apps.App{
			Name: fmt.Sprintf("App %02d", i),
			Path: fmt.Sprintf("/Applications/App %02d.app", i),
		})
	}
	a.SetApps(found)
	a.Query = "app"
	tt := render(t, a)
	tt.Frame()

	// Wheel the list a little at a time until its top row is mostly out of the
	// viewport — the state the old rule numbered anyway.
	clipped := -1
	for _, step := range []float32{4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48} {
		tt.Scroll(340, 200, 0, step)
		tt.Frame()
		tt.Frame()
		first, last := a.List.Visible()
		if first < 0 || last < first {
			continue
		}
		if share := visibleShare(a.rowRects[first], a.listRect); share > 0.02 && share < readable {
			clipped = first
			break
		}
	}
	if clipped < 0 {
		t.Fatalf("the fixture never left a row mostly clipped: %v", a.rowRects)
	}
	// The numbers of the next frame are computed from the boxes this frame laid
	// out, so those are the boxes to judge them by.
	boxes := make(map[int]ui.Rect, len(a.rowRects))
	for i, box := range a.rowRects {
		boxes[i] = box
	}
	viewport := a.listRect
	tt.Frame()

	firstNumbered := -1
	for i, digit := range a.numbers {
		if digit == 1 {
			firstNumbered = i
		}
	}
	if firstNumbered < 0 {
		t.Fatalf("no row carries 1: %v", a.numbers)
	}
	// The field row floats over the list (its top padding is the row plus the
	// breath), so the band the family measures against starts below the list's
	// own top edge: a row that has scrolled under the field is behind an opaque
	// band, and its badge is painted where nobody reads it. The inset is the
	// field's own band, not a number of this test's.
	if want := ui.LightTheme().Space(fieldRowUnits + breathUnits); viewport.Y != want {
		t.Errorf("the family's viewport starts at y=%.0f, want the field's band %.0f", viewport.Y, want)
	}
	if got := visibleShare(boxes[firstNumbered], viewport); got < readable {
		t.Errorf("row %d carries 1 while only %.2f of it is on screen", firstNumbered, got)
	}
	// Nothing readable above it was skipped.
	for i := 0; i < firstNumbered; i++ {
		if !numberableRow(a, i) {
			continue
		}
		if got := visibleShare(boxes[i], viewport); got >= readable {
			t.Errorf("row %d is %.2f on screen but the family starts at %d", i, got, firstNumbered)
		}
	}
	// No badge is drawn on a row the user cannot read — except the family's last
	// row, which the reference keeps while the bottom edge still shows it (that
	// is the row `0` lands on; `visibleRowRange` reads the two ends differently,
	// `main:src/launcher/result-budget.ts:789-810`).
	lastNumbered := -1
	for i := range a.numbers {
		if i > lastNumbered {
			lastNumbered = i
		}
	}
	for i := range a.numbers {
		got := visibleShare(boxes[i], viewport)
		if got <= 0 {
			t.Errorf("row %d is numbered %d while none of it is on screen", i, a.numbers[i])
			continue
		}
		if i != lastNumbered && got < readable {
			t.Errorf("row %d is numbered %d while only %.2f of it is on screen", i, a.numbers[i], got)
		}
	}
	// The clipped row is the one the old rule numbered: it is above the first
	// numbered row, and it has no number.
	if _, ok := a.numbers[clipped]; ok {
		t.Errorf("the mostly clipped row %d kept a number (%v)", clipped, a.numbers)
	}
	if clipped >= firstNumbered {
		t.Errorf("the fixture's clipped row %d is not above the first numbered row %d", clipped, firstNumbered)
	}
}

// numberableRow reports whether the numbered family may land on row i: a row
// with something to run that is neither the action bar nor the held modifier's
// bare terminal (see assignNumbers).
func numberableRow(a *App, i int) bool {
	results := a.Results()
	if i < 0 || i >= len(results) {
		return false
	}
	item := results[i]
	return item.Run != nil && !strings.HasPrefix(item.ID, "bar:") && item.ID != bareTerminalRowID
}

// P11-bb · **按住修饰键时，底部那条带常驻一行 `Super+Enter 快捷进入终端`。**
//
// 用户原话：「当按着 M 键时，底部固定显示一行「Super+Enter 快捷进入终端」」。
// 有查询时修饰键不画 `system-terminal-bare` 行（R64），所以除了这条带子，
// 页面上没有任何地方说这个和弦——这就是为什么它是**常驻**的一句，而不是
// toast 那种三秒的话。
func TestHoldingTheModifierKeepsTheTerminalHint(t *testing.T) {
	a := testApp()
	charged := 0
	a.Actions.ResizeTo = func(h int) { charged = h }
	a.Query = "app"
	tt := render(t, a)
	tt.Frame()

	hint := a.copy().HeldTerminalHint
	if strings.TrimSpace(hint) == "" {
		t.Fatal("the band's sentence is empty")
	}
	if tt.HasText(hint) {
		t.Errorf("the hint shows with the modifier up: %q", tt.Texts())
	}
	if a.FeedbackVisible() {
		t.Error("the band is charged with the modifier up")
	}
	up := charged
	if up <= 0 {
		t.Fatalf("the app asked for no window height")
	}

	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if !a.FeedbackVisible() {
		t.Error("the band is not charged while the modifier is held")
	}
	if !tt.HasText(hint) {
		t.Errorf("the band does not say %q: %q", hint, tt.Texts())
	}
	// The band is the list's own tail: the window grows by exactly one status
	// band, and the list's bottom padding takes it back out of the content.
	if got := charged - up; got != 30 {
		t.Errorf("holding the modifier grew the window by %d, want the 30u status band", got)
	}

	tt.HoldModifiers(0)
	tt.Frame()
	if tt.HasText(hint) {
		t.Errorf("the hint outlived the modifier: %q", tt.Texts())
	}
	if a.FeedbackVisible() {
		t.Error("the band stayed charged after the modifier came up")
	}
	if charged != up {
		t.Errorf("the window stayed at %d, want the released height %d", charged, up)
	}
}

// P11-bi · **恰好十项的列表也会滚，编号必须跟着视口走。**
//
// P11-bb 的判据只在 `resultCount > maxResultSlots` 时读视口。本仓的窗口把搜索
// 字段的浮带（60u）算在列表上方，所以十项列表也能滚；`resultCount` 又只数
// 非 action-bar 行。于是十项（或「十项 + 一条 action bar」）滚动后 `1` 仍钉在
// 被字段带盖住的第一行上，`Super+1` 跑的是屏幕外那一行——用户原话「超出屏幕的
// 项也显示快捷键」。这条 pin 用十项（正好等于 maxResultSlots）复现它：把闸门
// 加回去，第一行的 `share` 只有 0.53，却仍领 `1`，立刻红。
func TestATenRowListRenumbersWhenItScrolls(t *testing.T) {
	const readable = 0.7
	a := testApp()
	found := make([]apps.App, 0, maxResultSlots)
	for i := 0; i < maxResultSlots; i++ {
		found = append(found, apps.App{
			Name: fmt.Sprintf("App %02d", i),
			Path: fmt.Sprintf("/Applications/App %02d.app", i),
		})
	}
	a.SetApps(found)
	a.Query = "app"
	tt := render(t, a)
	tt.Frame()

	// The ten-row list must be able to scroll without crossing the budget: the
	// window charges the field's band above the rows, so the top row can slip
	// under it. Wheel until that row is no longer readable.
	scrolled := false
	for _, step := range []float32{2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 24, 28} {
		tt.Scroll(340, 200, 0, step)
		tt.Frame()
		tt.Frame()
		if got := visibleShare(a.rowRects[0], a.listRect); got > 0 && got < readable {
			scrolled = true
			break
		}
	}
	if !scrolled {
		t.Fatalf("a ten-row list never scrolled a row under the field: row0=%+v viewport=%+v",
			a.rowRects[0], a.listRect)
	}
	// The numbers of the next frame are computed from this frame's boxes.
	boxes := make(map[int]ui.Rect, len(a.rowRects))
	for i, box := range a.rowRects {
		boxes[i] = box
	}
	viewport := a.listRect
	tt.Frame()

	lastNumbered := -1
	for i := range a.numbers {
		if i > lastNumbered {
			lastNumbered = i
		}
	}
	for i, digit := range a.numbers {
		share := visibleShare(boxes[i], viewport)
		if share >= readable {
			continue
		}
		// The family's last row is the one the reference keeps while the bottom
		// edge still shows it (that is where `0` lands). A row scrolled under
		// the *top* edge is not that row and must carry no number.
		if i != lastNumbered {
			t.Errorf("row %d is numbered %d while only %.2f of it is on screen", i, digit, share)
		}
	}
	firstNumbered := -1
	for i, digit := range a.numbers {
		if digit == 1 {
			firstNumbered = i
		}
	}
	if firstNumbered < 0 {
		t.Fatalf("no row carries 1 after scrolling: %v", a.numbers)
	}
	if got := visibleShare(boxes[firstNumbered], viewport); got < readable {
		t.Errorf("row %d carries 1 while only %.2f of it is on screen", firstNumbered, got)
	}
	if _, ok := a.numbers[0]; ok && firstNumbered != 0 {
		t.Errorf("the top row scrolled under the field kept a number: %v", a.numbers)
	}
}

// TestATenRowListDump writes the frame P11-bi is about: a ten-row list
// (exactly the budget) scrolled under the field's band. Opt-in (FLOTER_PNG=1),
// like the other screenshot dumps.
func TestATenRowListDump(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P11-bi-tenrow-<appearance>.png")
	}
	forEachAppearance(t, func(t *testing.T, dark bool) {
		name := "light"
		if dark {
			name = "dark"
		}
		a := testApp()
		found := make([]apps.App, 0, maxResultSlots)
		for i := 0; i < maxResultSlots; i++ {
			found = append(found, apps.App{
				Name: fmt.Sprintf("App %02d", i),
				Path: fmt.Sprintf("/Applications/App %02d.app", i),
			})
		}
		a.SetApps(found)
		a.Query = "app"
		tt := renderIn(t, a, dark)
		tt.Frame()
		for _, step := range []float32{2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 24} {
			tt.Scroll(340, 200, 0, step)
			tt.Frame()
			tt.Frame()
			if got := visibleShare(a.rowRects[0], a.listRect); got > 0 && got < 0.7 {
				break
			}
		}
		path := fmt.Sprintf("/tmp/floter-P11-bi-tenrow-%s.png", name)
		writePNG(t, path, tt)
		t.Logf("wrote %s (numbers %v, row0 share %.2f, viewport %+v)",
			path, a.numbers, visibleShare(a.rowRects[0], a.listRect), a.listRect)
	})
}

// TestNumberBandDump writes the two frames this round is about: the numbered
// family on a scrolled list, and the band's terminal hint while the modifier is
// held. Opt-in (FLOTER_PNG=1), like the other screenshot dumps.
func TestNumberBandDump(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P11-bb-<surface>-<appearance>.png")
	}
	forEachAppearance(t, func(t *testing.T, dark bool) {
		name := "light"
		if dark {
			name = "dark"
		}
		// The numbered family, half a row into a scroll: the top row is on its
		// way out and carries no badge; the first row the user can read carries
		// 1.
		a := testApp()
		found := make([]apps.App, 0, 30)
		for i := 0; i < 30; i++ {
			found = append(found, apps.App{
				Name: fmt.Sprintf("App %02d", i),
				Path: fmt.Sprintf("/Applications/App %02d.app", i),
			})
		}
		a.SetApps(found)
		a.Query = "app"
		tt := renderIn(t, a, dark)
		tt.Frame()
		// Wheel until the top row is mostly gone — the frame the old rule
		// numbered from (the dump has to show the state the pin is about).
		for _, step := range []float32{4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48} {
			tt.Scroll(340, 200, 0, step)
			tt.Frame()
			tt.Frame()
			first, last := a.List.Visible()
			if first < 0 || last < first {
				continue
			}
			if share := visibleShare(a.rowRects[first], a.listRect); share > 0.02 && share < 0.7 {
				break
			}
		}
		path := fmt.Sprintf("/tmp/floter-P11-bb-numbers-%s.png", name)
		writePNG(t, path, tt)
		first, _ := a.List.Visible()
		t.Logf("wrote %s (numbers %v, first visible row %d at %+v, viewport %+v, share %.2f)",
			path, a.numbers, first, a.rowRects[first], a.listRect, visibleShare(a.rowRects[first], a.listRect))

		// The band's hint, with a typed query so the held row is not drawn.
		b := testApp()
		b.Query = "app"
		band := renderIn(t, b, dark)
		band.Frame()
		band.HoldModifiers(ui.Cmd)
		band.Frame()
		path = fmt.Sprintf("/tmp/floter-P11-bb-heldhint-%s.png", name)
		writePNG(t, path, band)
		t.Logf("wrote %s (hint %q)", path, b.copy().HeldTerminalHint)
	})
}

// writePNG encodes the frame at path.
func writePNG(t *testing.T, path string, tt *ui.Tester) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
