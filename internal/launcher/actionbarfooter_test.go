package launcher

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
	"floter/internal/theme"
)

// R-next · The action bar is the panel's **pinned footer**, not a first row.
//
// The user's report, verbatim: 「输入框中有输入内容时出现的"在终端中打开"项：现在
// 固定在第一项（错误）」. The old build drew `.launcher-action-bar` as a sibling of
// the scroller, *below* it — the bar is the field's own hand-off and the list
// sits above it — where this build had folded it into the list's head as a
// `bar:` row. This pins the position: every match comes first, the bar closes
// the list.
func TestTheActionBarClosesTheListNotHeadsIt(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Type("term")
	tt.Frame()

	results := a.Results()
	if len(results) < 2 {
		t.Fatalf("the list is too short to pin an order: %v", ids(results))
	}
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the action bar is not the list's last row: %v", ids(results))
	}
	if strings.HasPrefix(results[0].ID, "bar:") {
		t.Errorf("the action bar heads the list: %v", ids(results))
	}
	if results[bar-1].ID != "terminal" {
		t.Errorf("the match did not come before the bar: %v", ids(results))
	}
	// The bar is the *footer*: it must be the last thing drawn on the panel, so
	// its row box sits below every matched row's.
	barRect, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatal("the bar row was not drawn")
	}
	topRect, ok := tt.Find("Terminal")
	if !ok {
		t.Fatal("the matched row was not drawn")
	}
	if barRect.Y <= topRect.Y {
		t.Errorf("the bar is drawn above the match: bar y=%.0f, match y=%.0f", barRect.Y, topRect.Y)
	}
}

// R-next · The footer is not a numbered result and cannot take a number: the
// old build ran `resultShortcutSlots` over `displayedResults`, which excludes
// the bar. With the bar at the list's *head* it did not matter; as the footer
// it is the row a numbering over the matched rows would otherwise leave a
// number for.
func TestTheActionBarTakesNoNumber(t *testing.T) {
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	tt := render(t, a)
	tt.Type("git")
	tt.Frame()

	results := a.Results()
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	if _, ok := a.numbers[bar]; ok {
		t.Errorf("the action bar was numbered: %v", a.numbers)
	}
	if got := a.numbers[0]; got != 1 {
		t.Errorf("the first result's number = %d, want 1 (%v)", got, a.numbers)
	}
	// The held modifier's own row can never reach a number either — it is the
	// one row the numbered family cannot reach (the reference's R67).
	b := testApp()
	tb := render(t, b)
	tb.HoldModifiers(ui.Cmd)
	tb.Frame()
	bresults := b.Results()
	if bresults[len(bresults)-1].ID != bareTerminalRowID {
		t.Fatalf("the held row is not the list's last line: %v", ids(bresults))
	}
	if _, ok := b.numbers[len(bresults)-1]; ok {
		t.Errorf("the held row was numbered: %v", b.numbers)
	}
}

// R-next · The footer takes the selection on the command line the catalogue
// also matches, and it is what makes the row's selected face reachable at all.
//
// The user's second report is that the item had no selected style. It had none
// because it was never the selected row: the bar was drawn at the list's head
// *beside* a match that owned the choice. The reference hands Enter to the bar
// for exactly this query (`shouldDefaultToActionBar`: a shell command line with
// no runnable command plan), and this build inherited that rule
// (`defaultsToShell`) — the placement was the only thing hiding it.
func TestTheActionBarTakesTheSelectionOnACommandLine(t *testing.T) {
	forEachAppearance(t, actionBarTakesTheSelectionOnACommandLine)
}

func actionBarTakesTheSelectionOnACommandLine(t *testing.T, dark bool) {
	a := testApp()
	// The `git` row is a PATH tool: runnable, but not a *command plan*, which
	// is the distinction `shouldDefaultToActionBar` turns on.
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	ran := 0
	a.Actions.RunInTerminal = func([]string) { ran++ }
	tt := renderIn(t, a, dark)
	tt.Type("git")
	tt.Frame()
	// The rows enter over 120ms (`rowTransition`), and the tester's clock is
	// its own: without the walk the row is read at opacity 0, i.e. the panel's
	// fill, and the selected face looks missing.
	tt.Advance(200 * time.Millisecond)

	results := a.Results()
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	if a.Selected != bar {
		t.Errorf("the selection is on row %d, want the footer %d (%v)", a.Selected, bar, ids(results))
	}
	// The drawn row carries the selection, so its block is tinted against the
	// panel's own fill at the same height — the visible "selected style" the
	// report said was missing. P8-a 重定基线：旧 base.css 值 → 合约 §4.6——页脚
	// 不再是「抬升面 + 旁边投影」的第二张卡，而只是卡片的一个 block，所以选中
	// 的可见性落在**行内药丸**上（合约 §4.3 的中性圆角块），而非页脚自己的面。
	rect, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatal("the selected footer was not drawn")
	}
	img := tt.Image()
	y := int(rect.Y + rect.H/2)
	gutter := int(ui.LightTheme().Space(2)) // the row block's 8u inset
	panel := img.RGBAAt(1, y)
	// The pill starts on the block's own gutter, so two pixels in is inside it
	// with no glyph over it.
	pill := img.RGBAAt(gutter+2, y)
	if pill == panel {
		t.Errorf("the selected footer's pill is not tinted: %v at x=%d over the panel %v",
			pill, gutter+2, panel)
	}
	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	if want := over(tokens.RowSelected, panel); !closeRGB(pill, want) {
		t.Errorf("the selected footer's pill is %v, want the neutral chosen-row rung %v over the panel (→ %v)",
			pill, tokens.RowSelected, want)
	}
	// And the footer casts no shadow beside itself: the band left of the pill is
	// the panel, the same colour it is further up. The old build gave the footer
	// the raised pane's contact cast, which fell into the panel's own 4u inset.
	// `docs/raycast-design.md` §4.6: the footer is a block of the card, so it has
	// no elevation of its own — and a cast reaching past the block's gutter was
	// the tell that it was drawn as a floating pane instead.
	side := img.RGBAAt(3, y)
	clean := img.RGBAAt(3, y-60)
	if side != clean {
		t.Errorf("the selected footer casts a shadow beside itself (%v at its own height, %v further up); "+
			"the contract's footer is a block of the card, not a raised pane", side, clean)
	}
	// The footer's block sits on the *same* 8u gutter the list's rows do
	// (`docs/raycast-design.md` §4.3/§4.6), so the seam between the list and the
	// footer is one column rather than two: the pill's painted left edge is the
	// block's gutter, exactly what TestTheListIsTheCentredReferenceColumn pins
	// for a row above it.
	footerLeft := -1
	for x := 0; x < InputWindowWidth; x++ {
		if img.RGBAAt(x, y) != panel {
			footerLeft = x
			break
		}
	}
	if footerLeft != gutter {
		t.Errorf("the footer's block starts at x=%d, want the list's own 8u gutter (x=%d)",
			footerLeft, gutter)
	}
	// Pinned: the footer's band ends at the window's own bottom edge, the
	// reference's `.launcher-action-bar` sitting below the scroller rather than
	// scrolling with it. The row's box is a result row (40u) inside the band the
	// bar is charged (45u), so the only space below it is that band's own
	// leftover — less than a row. Anything larger means the footer is riding the
	// scroller again.
	bottom := img.Bounds().Max.Y
	if gap := bottom - int(rect.Y+rect.H); gap >= int(rect.H) {
		t.Errorf("the footer is not pinned to the window's bottom edge: row ends at %.0f, window %d (%d px below the row, more than the row's own %.0f px box)",
			rect.Y+rect.H, bottom, gap, rect.H)
	}
	// Enter runs the footer, not the tool row above it: the bar is the field's
	// own hand-off and it is the row the choice is on.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if ran != 1 {
		t.Errorf("Enter ran the shell row %d times, want once (it is the selected row)", ran)
	}
}

// R-next · A page where a match keeps the choice leaves the footer untinted —
// "selected" is a state the row has only when Enter would run it.
func TestAnUnselectedActionBarStaysQuiet(t *testing.T) {
	forEachAppearance(t, unselectedActionBarStaysQuiet)
}

func unselectedActionBarStaysQuiet(t *testing.T, dark bool) {
	a := testApp()
	tt := renderIn(t, a, dark)
	tt.Type("term")
	tt.Frame()

	results := a.Results()
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	if a.Selected == bar {
		t.Fatalf("the match did not keep the selection: selected=%d bar=%d", a.Selected, bar)
	}
	rect, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatal("the bar row was not drawn")
	}
	img := tt.Image()
	y := int(rect.Y + rect.H/2)
	if got, want := img.RGBAAt(10, y), img.RGBAAt(3, y); got != want {
		t.Errorf("the unselected footer is tinted: %v at x=10, %v at x=3", got, want)
	}
}

// R-next · The field row and the result rows share the reference's text column.
//
// The user's first report is 「搜索页面左右两边各有一大块空白」. The horizontal
// insets had been written in the wrong unit — a `Space(n)` is **four** reference
// units (`--u * 4n`; `internal/theme/theme.go`), so the reference's
// `.collapsed-card__input-row { padding: 0 16u }` and `.launcher-result
// { padding: 0 12u }` (`src/styles/launcher.css:212-215,1099-1114`) are
// `Space(4)` and `Space(3)`, not `Space(16)` and `Space(12)`. At the shipped
// step this build was drawing 57.6px of side padding where the design asks for
// 14.4 — a query at 64u and row titles at 120u instead of 16 and 54.
func TestTheFieldAndTheRowsShareTheReferenceTextColumn(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	// The empty field's caret sits on the row's own 16u inset.
	if caret, ok := tt.TextCaret(); ok {
		if got := int(caret.X); got != 16 {
			t.Errorf("the field's caret starts at x=%d, want the reference's 16", got)
		}
	} else {
		t.Fatal("the field's caret was not drawn")
	}
	// A result row's title starts on 16u + the 28u icon + the 10u gap = 54u.
	if r, ok := tt.Find("Restart the computer"); ok {
		if got := int(r.X); got != 54 {
			t.Errorf("a result title starts at x=%d, want the reference's 54", got)
		}
	} else {
		t.Fatal("the first result's title was not drawn")
	}
}

// R-next · The bar row's two text registers follow the reference's split: the
// *title* is the string the action hands over (the command line, the URL, the
// path) and the *subtitle* is the action's own word.
//
// `main:src/launcher/LauncherResults.tsx:1067-1068` prints
// `{actionBar.value}` in `.launcher-action-bar__title` and `{actionBar.label}`
// in `.launcher-action-bar__subtitle`; `.launcher-action-bar__title`
// (`main:src/styles/launcher.css:1632-1638`) is the monospace register,
// because the string it holds is code. The port had the two swapped: the
// action word was the title (and so wore the monospace) while the command line
// was the subtitle — the visible result of which is that the row reads
// "Run in shell" in the code face and the line you are about to run in the
// interface one.
func TestTheActionBarTitleIsTheValueNotTheActionWord(t *testing.T) {
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	tt := render(t, a)
	tt.Type("git")
	tt.Frame()

	results := a.Results()
	var bar *Item
	for i := range results {
		if strings.HasPrefix(results[i].ID, "bar:") {
			bar = &results[i]
		}
	}
	if bar == nil {
		t.Fatalf("the bar row is missing: %v", ids(results))
	}
	// The title is the typed line; the subtitle is the action's word. The
	// word must not be the row's title (it belongs to the subtitle slot).
	if bar.Title != "git" {
		t.Errorf("the bar's title = %q, want the typed line %q", bar.Title, "git")
	}
	if bar.Detail != "Run in shell" {
		t.Errorf("the bar's subtitle = %q, want the action word %q", bar.Detail, "Run in shell")
	}
	// Both texts are drawn, and the value's box is the wide one — the action
	// word sits under it in the subtitle slot.
	valueRect, ok := tt.Find("git")
	if !ok {
		t.Fatal("the bar's value was not drawn")
	}
	wordRect, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatal("the bar's action word was not drawn")
	}
	if wordRect.Y <= valueRect.Y {
		t.Errorf("the action word is not the subtitle: value y=%.0f, word y=%.0f",
			valueRect.Y, wordRect.Y)
	}

	// A URL query prints the address as the title and "Open in browser" as the
	// subtitle, the same split.
	b := testApp()
	tb := render(t, b)
	tb.Type("https://example.com/docs")
	tb.Frame()
	var urlBar *Item
	for i := range b.Results() {
		if b.Results()[i].ID == "bar:url" {
			bar := b.Results()[i]
			urlBar = &bar
		}
	}
	if urlBar == nil {
		t.Fatalf("the url bar is missing: %v", ids(b.Results()))
	}
	if urlBar.Title != "https://example.com/docs" || urlBar.Detail != "Open in browser" {
		t.Errorf("the url bar = (%q, %q), want (value, action word)",
			urlBar.Title, urlBar.Detail)
	}
}

// R-next · The footer's top edge is one weak hairline, and the row block inside
// it squares its top corners when a list sits above it.
//
// P8-a 重定基线：旧 base.css 值 → 合约 §4.6。先验产品（任务书第 4 条）：
// 合约下页脚顶部只是一条 1px 弱发丝线，**没有**抬升面、也没有自己的块。逐像素
// 复核过两张图（有列表 / 无列表）：
//
//   - 两边的 y=443 都是整行一条 `HairlineSoft` 合成值（239,239,240），x=0..719
//     全宽——页脚没有在卡片的角上画任何成块的方角，产品正确；
//   - 两者唯一的差别在发丝线**之下**的那一行：有列表时行块的左沿在沟槽处是
//     238 的直角（rowRadius = 0,0,8,8，「焊在列表上」），无列表时同一位置是
//     250→244→240 的弧（完整的 8px 圆角，即「没有列表可焊」）。
//
// 旧断言（`x=4` 的 `panelDistance > 8`）量的是旧结构：旧页脚是一个填满的抬升面，
// 它的左上角就在面板的 4u 内缩处。合约下 x=4 已落在 8u 沟槽的面板上，而它读到的
// 「方角」其实是那条通栏弱发丝线本身（正确产品），不是块。所以这条改成量合约
// 真正说的两件事：顶边只有一条发丝线，且行块的**上**两角由「上面有没有列表」
// 决定。
func TestTheActionBarSquaresItsTopCornersOverAList(t *testing.T) {
	forEachAppearance(t, actionBarSquaresItsTopCornersOverAList)
}

func actionBarSquaresItsTopCornersOverAList(t *testing.T, dark bool) {
	// The bar after a match: the block is welded to the list above it.
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	ta := renderIn(t, a, dark)
	ta.Type("git")
	ta.Frame()
	ta.Advance(200 * time.Millisecond) // the rows' 120ms enter

	// The bar at the list's head (a URL with no match): it keeps its own full
	// rounding, because there is no list to weld to.
	b := testApp()
	tb := renderIn(t, b, dark)
	tb.Type("https://example.com/nope")
	tb.Frame()
	tb.Advance(200 * time.Millisecond)

	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	gutter := int(ui.LightTheme().Space(2)) // the row block's 8u inset

	// bandTop finds the footer's top edge: the panel's *last* full-width rule.
	// The field block's fill and its own hairline are full-width too, so the scan
	// starts below the field's band and keeps the last row painted edge to edge
	// (nothing full-width is drawn under the footer).
	bandTop := func(tt *ui.Tester) int {
		img := tt.Image()
		panel := img.RGBAAt(1, 300)
		last := -1
		for y := 100; y < img.Bounds().Max.Y; y++ {
			if img.RGBAAt(1, y) != panel && img.RGBAAt(gutter+2, y) != panel &&
				img.RGBAAt(InputWindowWidth-1, y) != panel {
				last = y
			}
		}
		if last < 0 {
			t.Fatalf("the footer's band was not found: %q", tt.Texts())
		}
		return last
	}

	// Both cases: the top edge is exactly one pixel of the weak hairline, full
	// width, and it is nothing heavier than that (the contract's §4.6 rule). A
	// raised pane's own edge — a card's strong 1px border, or a filled band's
	// top rim — would be a different, darker colour here.
	for _, tc := range []struct {
		name  string
		tt    *ui.Tester
		above bool
	}{{"list above", ta, true}, {"no list above", tb, false}} {
		t.Run(tc.name, func(t *testing.T) {
			img := tc.tt.Image()
			top := bandTop(tc.tt)
			panel := img.RGBAAt(1, top)
			if !closeRGB(panel, over(tokens.HairlineSoft, img.RGBAAt(1, top-2))) {
				t.Errorf("the footer's top edge at y=%d is %v, want the weak hairline %v over the panel",
					top, panel, tokens.HairlineSoft)
			}
			// Every column on that row carries the same hairline, so the rule is
			// full-width rather than a block's own border.
			for _, x := range []int{gutter, InputWindowWidth / 2, InputWindowWidth - gutter} {
				if img.RGBAAt(x, top) != panel {
					t.Errorf("the footer's rule is not full-width: x=%d at y=%d is %v, the rule is %v",
						x, top, img.RGBAAt(x, top), panel)
				}
			}
			// One pixel below it is the band: nothing is painted there, so the
			// footer has no face of its own.
			if below := img.RGBAAt(gutter+2, top+1); closeRGB(below, over(tokens.Control, panel)) {
				t.Errorf("the footer paints a face of its own under its rule (%v at y=%d); "+
					"the contract's footer is a block of the card, not a raised pane", below, top+1)
			}

			// The block's own top-left corner, read on the band's first content
			// row: a squared corner paints the pill's fill right at the gutter, a
			// rounded one leaves the panel showing under the arc.
			img = tc.tt.Image()
			top = bandTop(tc.tt)
			// The unpainted panel, read two rows above the rule (the rule's own
			// row is the hairline, not the panel).
			blank := img.RGBAAt(1, top-2)
			corner := img.RGBAAt(gutter, top+3)
			// P9-f 重定基线：`panelDistance` 量的是**浅色**面板常量（254,254,254），
			// 深色下「什么都没画」的角落在它眼里差着 230 个色阶，圆角会被读成方角。
			// 改成与该外观实际读到的面板比（同阈值 8）。
			if tc.above {
				if channelDistance(corner, blank) <= 8 {
					t.Errorf("the footer over a list is rounded at its top-left corner (y=%d): %v",
						top+3, corner)
				}
				return
			}
			if channelDistance(corner, blank) > 8 {
				t.Errorf("the footer with no list above it is squared at its top-left corner: %v over panel %v",
					corner, blank)
			}
		})
	}
}

// P8-a 重定基线：旧 base.css 值「行块左右各 4u 沟槽」→ 合约 §4.3「行块内缩 8u，
// 选中填充成为一块不顶到卡片边的圆角 8 药丸」。行内 padding 从 12u 降到 8u 与行块
// 内缩的 +4u 相互抵掉，所以文本列（54u）一个字都没动——这条只钉绘制边的位移。
//
// R-next · The list is the reference's centred column: every row is
// `calc(100% - 8u)` wide and `justify-self: center`, so the panel keeps 8u on
// each side of a row block.
//
// The user's first report is 「搜索页面左右两边各有一大块空白」. The insets had
// once been written in the wrong unit (Space(16) is 64u, not 16u), which put a
// 64u gutter around every row; the text column is pinned above, and this pins
// the other half of the same fact: the row's painted box spans the panel from
// its 8u gutter to the mirror-image 8u gutter, so neither edge carries a dead
// margin and the selected pill does not run into the card's own edge.
func TestTheListIsTheCentredReferenceColumn(t *testing.T) {
	forEachAppearance(t, listIsTheCentredReferenceColumn)
}

func listIsTheCentredReferenceColumn(t *testing.T, dark bool) {
	a := testApp()
	tt := renderIn(t, a, dark)
	tt.Frame()
	// The gutter in pixels: `Space(2)` is 8 reference units (one Space is 4u).
	gutter := int(ui.LightTheme().Space(2))
	row, ok := tt.Find("Restart the computer") // the empty page's selected first row
	if !ok {
		t.Fatalf("the first result was not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	y := int(row.Y + row.H/2)
	// The panel's own fill, read outside the row's box (the gutter is the panel
	// and nothing else).
	panel := img.RGBAAt(1, y)
	left, right := -1, -1
	for x := 0; x < InputWindowWidth; x++ {
		if img.RGBAAt(x, y) != panel {
			if left < 0 {
				left = x
			}
			right = x
		}
	}
	if left != gutter {
		t.Errorf("the row's box starts at x=%d, want the contract's 8u gutter (x=%d)", left, gutter)
	}
	if want := InputWindowWidth - gutter - 1; right != want {
		t.Errorf("the row's box ends at x=%d, want %d (the mirrored 8u gutter)", right, want)
	}
}

// R-next · The held modifier's row takes the selected face. It is the row Enter
// would run — `syncHeldModifier` puts the choice on the list's last line — so
// the user's 「应该有选中样式（当前缺失）」 is a painted fact, not an index: the
// band carries the row's tint rather than the panel's own fill.
func TestTheHeldRowWearsTheSelectedFace(t *testing.T) {
	forEachAppearance(t, heldRowWearsTheSelectedFace)
}

func heldRowWearsTheSelectedFace(t *testing.T, dark bool) {
	a := testApp()
	tt := renderIn(t, a, dark)
	// The empty page: the row's one placement (R64). It closes the list, below
	// the built-in commands and the recents.
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	row, ok := tt.Find("Open Terminal")
	if !ok {
		t.Fatalf("the held row was not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	const x = 600 // past the row's texts, where only the row's fill shows
	sel := img.RGBAAt(x, int(row.Y+row.H/2))
	panel := img.RGBAAt(1, int(row.Y+row.H/2))
	if sel == panel {
		t.Errorf("the held row is untinted: %v over the panel's %v", sel, panel)
	}
	// And it is specifically the *chosen-row* rung, not just any tint: the two
	// neutral rungs above it (hover 0.045 / selected 0.085 in the dark, 0.04 /
	// 0.06 in the light) are different fills, and "tinted" alone would pass on
	// a row that wore the hover rung. Mutation check: `launcher.go`'s selected
	// branch → `RowHover` reddens this line on both appearances.
	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	if want := over(tokens.RowSelected, panel); !closeRGB(sel, want) {
		t.Errorf("the held row's fill is %v, want the neutral chosen-row rung %v over the panel (→ %v)",
			sel, tokens.RowSelected, want)
	}
}

// R-next · The held row's one page is the empty field.
//
// The user's second report, verbatim: 「输入框中有输入内容时出现的「在终端中打
// 开」项：现在固定在第一项（错误）…原版行为：按住时默认加到底部，松手就消失
// （不是固定项）…应该有选中样式（当前缺失）」.
//
// The reference's placement is a function of the *query*: `bareTerminalPlacement`
// returns "appended" on an empty field and null the moment one character is
// typed (R64, `main:src/launcher/terminal-row.ts:77-85` — the user's own earlier
// correction, 「前面加的按住在列表额外增加终端选项的逻辑要去掉」). The port had
// kept R60's wider gesture, so a typed query drew the action word beside the
// matches; the row is now the empty page's alone.
func TestTheHeldRowIsTheEmptyPagesAlone(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if got, want := a.Selected, len(a.Results())-1; got != want {
		t.Fatalf("the press edge did not select the held row: selected=%d want=%d", got, want)
	}

	// Typing gives the row up: no action word on the page, and the choice
	// returns to the query's own default (the action bar for a command line).
	tt.Type("term")
	tt.Frame()
	if hasItem(a.Results(), bareTerminalRowID) {
		t.Fatalf("a typed query kept the held row: %v", ids(a.Results()))
	}
	if tt.HasText(a.copy().BareTerminal) {
		t.Errorf("the held row's action word is still on screen: %q", tt.Texts())
	}
	if a.Selected >= len(a.Results()) {
		t.Errorf("the selection = %d, past the list's %d rows", a.Selected, len(a.Results()))
	}

	// Clearing the field brings it back — last, and selected.
	a.Query = ""
	tt.Frame()
	results := a.Results()
	if last := results[len(results)-1].ID; last != bareTerminalRowID {
		t.Fatalf("the cleared field did not bring the row back as the last line: %v", ids(results))
	}
	if a.Selected != len(results)-1 {
		t.Errorf("the selection is on %d, want the held row %d", a.Selected, len(results)-1)
	}

	// The row's trailing badge is the action bar's own chord — not the empty
	// slot the port had, and not a number either.
	//
	// R67 on the reference itself (`main:src/launcher/LauncherResults.tsx:1004-1018`):
	// the row carries no `shortcut` of its own in `bareTerminalRow`, and the
	// numbered family never reaches it — `resultShortcutSlots[index]` is null
	// there — so `LauncherResults` substitutes `actionBarShortcut` for exactly
	// this row (`isBareTerminalRow(item) ? actionBarShortcut : …`). The port
	// read only the first half of that and printed nothing where every other
	// row on the page prints its key; this pins the substituted chord.
	held := results[len(results)-1]
	if want := a.enterBadge(); held.Shortcut != want {
		t.Errorf("the held row prints %q, want the action bar's chord %q (R67)",
			held.Shortcut, want)
	}
	if held.Shortcut == "" {
		t.Error("the held row is the page's only row with no key hint at all")
	}

	// Releasing gives the row up for good — it is not a fixed item.
	tt.HoldModifiers(0)
	tt.Frame()
	if hasItem(a.Results(), bareTerminalRowID) {
		t.Errorf("the held row survived the release: %v", ids(a.Results()))
	}
}

// A full list keeps its tenth result **and** the footer.
//
// The reference draws the bar as a sibling below the scroller and budgets ten
// *results* (`main:src/launcher/LauncherResults.tsx:1043-1073`,
// `main:src/launcher/result-budget.ts:47-49` — 「九项加一个底部的…项」, then
// 「上方一共有 10 项了，应该最大只能有 9 项」: the ten are the rows above the
// bar). The port charged the bar as one of the ten, so with a full list the bar
// was the eleventh band the window never paid for: the tenth result kept its
// slot and the 「在终端中打开」 row was drawn below the window's bottom edge.
func TestAFullListKeepsItsTenthResultAndTheFooter(t *testing.T) {
	a := testApp()
	found := make([]apps.App, rowBudget)
	for i := range found {
		found[i] = apps.App{Name: "app" + string(rune('a'+i)), Path: "/usr/bin/app"}
	}
	a.SetApps(found)
	tt := render(t, a)
	a.SetQuery("app")
	tt.Frame()

	results := a.Results()
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	// Ten results above the bar: the budget is the results', not the bands'.
	if bar != rowBudget {
		t.Fatalf("%d results above the bar, want the whole budget of %d: %v", bar, rowBudget, ids(results))
	}
	// The window pays for those ten bands and the bar's own band on top.
	geometry := Geometry{
		Font: a.Font, Spacing: a.Spacing, Rows: a.Rows, Held: len(a.Rows),
		TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine,
	}
	height := geometry.Height()
	withoutBar := Geometry{
		Font: a.Font, Spacing: a.Spacing, Rows: a.Rows[:bar], Held: bar,
		TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine,
	}.Height()
	if band := float64(a.Spacing) * 11.25; height-withoutBar < int(band)-1 || height-withoutBar > int(band)+1 {
		t.Errorf("the bar adds %d to the window, want its own %.0f band", height-withoutBar, band)
	}
	// Sized to that height, the last result and the footer are both on screen:
	// the footer is not drawn past the window's bottom edge.
	tt.SetSize(InputWindowWidth, height)
	tt.Frame()
	for _, title := range []string{results[bar-1].Title, results[bar].Title} {
		rect, ok := tt.Find(title)
		if !ok {
			t.Fatalf("%q was not drawn: %q", title, tt.Texts())
		}
		if bottom := rect.Y + rect.H; bottom > float32(height)+1 {
			t.Errorf("%q ends at y=%.1f, past the %d-high window", title, bottom, height)
		}
	}
	// And the tenth result keeps the tenth key of the numbered family — `0`,
	// the slot right beside `9` (see maxResultSlots' family).
	if got, ok := a.numbers[bar-1]; !ok || got != 0 {
		t.Errorf("the tenth result's number = %d (present %v), want the 0 slot (%v)", got, ok, a.numbers)
	}
}

// R-next · Over a typed query the modifier hands the footer the selection, and
// a list that grows under it does not take the choice away.
//
// The user's report had three parts, and this is the third: 「应该有选中样式
// （当前缺失）」. A typed query draws no held row (R64, see heldTerminalItem), so
// the modifier's press edge hands the choice to the action bar — the old build's
// `setSelectedActionBar(true)` — and the bar is the row Enter then runs.
//
// The bar is the list's *last* row, so its index moves whenever a row arrives
// under it: the catalogue's own scan finishing, a plugin's list refreshing, a
// clipboard entry landing. The press edge set an index; without re-asserting it
// the choice stays on that index and the modifier's "run this line" row is the
// one row the user is *not* on — `holdBarSelection` is what keeps the two
// together, and nothing pinned it.
func TestTheHeldModifierKeepsTheActionBarSelectedOverAGrowingList(t *testing.T) {
	forEachAppearance(t, heldModifierKeepsTheActionBarSelectedOverAGrowingList)
}

func heldModifierKeepsTheActionBarSelectedOverAGrowingList(t *testing.T, dark bool) {
	a := testApp()
	tt := renderIn(t, a, dark)
	tt.Type("ter")
	tt.Frame()

	results := a.Results()
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	if a.Selected == bar {
		t.Fatalf("the match kept the selection with the modifier up: selected=%d bar=%d", a.Selected, bar)
	}

	// The press edge: the modifier hands the footer the choice.
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if a.Selected != bar {
		t.Fatalf("the modifier left the selection on %d, want the footer %d (%v)", a.Selected, bar, ids(results))
	}

	// A row arrives under the held bar (a tool the scan just found, a plugin's
	// own refresh). The bar is no longer at the index the press edge set, and
	// the choice has to move with it.
	a.SetTools([]apps.App{{Name: "termls", Path: "/usr/bin/termls"}}, true)
	tt.Frame()
	results = a.Results()
	bar = len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the grown list has no footer: %v", ids(results))
	}
	if bar == 3 {
		t.Fatalf("the list did not grow, so the index never moved: %v", ids(results))
	}
	if a.Selected != bar {
		t.Errorf("the grown list left the selection on %d, want the held footer %d (%v)", a.Selected, bar, ids(results))
	}
	// The rows enter over 120ms (`rowTransition`) and the tester's clock is its
	// own: the row the fixture just grew was still at opacity 0 when it was read,
	// so the walk below is what makes the pill's fill real.
	tt.Advance(200 * time.Millisecond)

	// And the held selection is the *drawn* one: the footer's own pill (the
	// chosen-row rung inside the row block) against the panel at the same height.
	//
	// P8-a 重定基线：旧 base.css 值 → 合约 §4.3/§4.6。旧读点是 x=10（落在旧 4u
	// 沟槽的内侧面板上），量的是「页脚的抬升面 vs 面板」；合约下页脚不带自己的面，
	// 选中只画在**行块内**的药丸上，所以读点要按 8u 缩进重算到药丸里（block 的
	// gutter + 2px），否则量到的是药丸外的面板，看上去「没染色」。
	rect, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatalf("the held footer was not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	y := int(rect.Y + rect.H/2)
	gutter := int(ui.LightTheme().Space(2)) // the row block's 8u inset
	panel := img.RGBAAt(1, y)
	pill := img.RGBAAt(gutter+2, y)
	if pill == panel {
		t.Errorf("the held footer's pill is not tinted: %v at x=%d over the panel %v", pill, gutter+2, panel)
	}
	tokens := theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	if want := over(tokens.RowSelected, panel); !closeRGB(pill, want) {
		t.Errorf("the held footer's pill is %v, want the neutral chosen-row rung %v over the panel (→ %v)",
			pill, tokens.RowSelected, want)
	}

	// Released, the choice goes back to the page's own default — a match, not
	// the footer.
	tt.HoldModifiers(0)
	tt.Frame()
	if bar := actionBarIndex(a.Results()); a.Selected == bar {
		t.Errorf("the released modifier left the footer selected: %v", ids(a.Results()))
	}
}

// R-next · The footer is pinned, so a full list cannot push it off the window.
//
// The port drew the bar as the scroller's *last row*: with nine matches above
// it the row sat exactly at the window's bottom edge, and the tenth pushed it
// past it — the footer was drawn at y=563 in a 471px window, i.e. only a scroll
// (or the held modifier's own scroll-into-view) could reach it. The reference
// draws `.launcher-action-bar` as a sibling *below* `.launcher-results`
// (`main:src/launcher/LauncherResults.tsx:1043-1073`), so it is on screen
// whatever the list is doing. This pins the fix: at the window height the app
// itself asks for, the bar's band is inside the window and below every match.
func TestTheFooterStaysPinnedOverAFullList(t *testing.T) {
	wide := func(app *App) {
		var list []apps.App
		for _, name := range []string{
			"app-a", "app-b", "app-c", "app-d", "app-e", "app-f", "app-g",
			"app-h", "app-i", "app-j", "app-k", "app-l", "app-m", "app-n",
		} {
			list = append(list, apps.App{Name: name, Path: "/usr/bin/" + name})
		}
		app.SetApps(list)
	}
	// The window the app asks for, measured off a frame that has room: the
	// test frame is taller than the charged window on purpose (see render), so
	// the charged height has to be read from the app, not from the tester.
	a := testApp()
	wide(a)
	charged := 0
	a.Actions.ResizeTo = func(h int) { charged = h }
	ta := render(t, a)
	ta.Type("app")
	ta.Frame()

	results := a.Results()
	bar := actionBarIndex(results)
	if bar < 0 || bar != len(results)-1 {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	if bar <= rowBudget {
		t.Fatalf("the list is not long enough to overflow the window: %d rows", bar)
	}
	if charged <= 0 {
		t.Fatal("the app asked for no window height")
	}

	b := testApp()
	wide(b)
	b.Query = "app"
	frame := ui.NewTester(func(c *ui.Context) { b.View(c) }, InputWindowWidth, charged)
	frame.Frame()

	rect, ok := frame.Find("Run in shell")
	if !ok {
		t.Fatalf("the footer was not drawn at all: %q", frame.Texts())
	}
	bottom := float64(rect.Y + rect.H)
	if bottom > float64(charged) {
		t.Errorf("the footer is below the window's bottom edge: %.0f > %d", bottom, charged)
	}
	// Its band's own top edge: the footer is the last thing on the panel, so
	// nothing of the scroller is drawn under it.
	footerTop := float64(charged) - actionBarBand(b, results)
	if got := float64(rect.Y + rect.H); got < footerTop-4 {
		t.Errorf("the footer is not at the panel's bottom: it ends at %.0f, band top %.0f", got, footerTop)
	}
	// The scroller stops at the footer's top edge: the first match is drawn
	// above it, and the footer's band holds no match at all.
	r, ok := frame.Find("app-a")
	if !ok {
		t.Fatalf("the first match was not drawn: %q", frame.Texts())
	}
	if float64(r.Y+r.H) > footerTop+1 {
		t.Errorf("the first match runs under the footer: %.0f > %.0f", float64(r.Y+r.H), footerTop)
	}
	if bar > rowBudget && frame.HasText("app-n") {
		if last, ok := frame.Find("app-n"); ok && float64(last.Y+last.H) > footerTop+1 {
			t.Errorf("the last match runs under the footer: %.0f > %.0f", float64(last.Y+last.H), footerTop)
		}
	}
}

// actionBarBand is the band the bar rows take at the bottom of the panel, in
// pixels, as the view reserves it: one `actionBarUnits` step per bar row.
func actionBarBand(a *App, results []Item) float64 {
	bar := actionBarIndex(results)
	if bar < 0 {
		return 0
	}
	return a.Spacing * actionBarUnits * float64(len(results)-bar)
}
