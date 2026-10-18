package shell

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The search page's left/right column, measured on the *shipped* frame.
//
// The user's first report is 「搜索页面左右两边各有一大块空白」, and the port's
// first fix for it (`5c2d60d`) was a unit error: `Space(n)` is four reference
// units (`--u * 4n`), so the reference's `padding: 0 16u`
// (`main:src/styles/launcher.css:212-215`) is `Space(4)`, not `Space(16)`.
//
// Every check of that fix until now rendered the launcher *alone*, on the
// framework's stock theme (14px type, a 4px unit). The shipped frame is the
// shell's: the card sits inside the platform's own inset
// (`main:src/styles/base.css:1591-1607`: 10u a side on Linux, `4u 10u 12u 4u` on
// Windows, none on macOS) and the theme is `theme.ForPreferences`, i.e. the
// interface step the user picked — `small`, 0.9, by default. So the reference's
// 16u is 14.4px there and 54u is 48.6px, and a check written against literal
// pixels would have called a correct frame wrong.
//
// This asserts the two numbers the design actually names, in *units*, so it
// holds at every interface step: the query's text starts on 16u and a result
// row's title on 54u (4u of panel gutter + the row's 12u padding + its 28u icon
// column + the 10u gap), both measured from the card's own content edge — and
// the row's painted box keeps the reference's 4u gutter on either side.
func TestTheSearchColumnIsTheReferenceUnitsAtTheShippedStep(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Frame()

	// The reference's `--u`: the port's Space(n) is 4n of them.
	if a.Launcher.Spacing <= 0 {
		t.Fatalf("the launcher did not report its unit (spacing %v)", a.Launcher.Spacing)
	}
	unit := float32(a.Launcher.Spacing / 4)

	cardLeft, cardRight := cardEdges(a)

	field, ok := tt.Find("Search")
	if !ok {
		t.Fatalf("the field was not drawn: %q", tt.Texts())
	}
	if want := cardLeft + 16*unit; absf(field.X-want) > 1.5 {
		t.Errorf("the query starts at x=%.1f, want the reference's 16u (%.1f) from the card's %.1f edge",
			field.X, want, cardLeft)
	}

	row, ok := tt.Find("Open settings")
	if !ok {
		t.Fatalf("the first result was not drawn: %q", tt.Texts())
	}
	if want := cardLeft + 54*unit; absf(row.X-want) > 1.5 {
		t.Errorf("the result's title starts at x=%.1f, want the reference's 54u (%.1f)", row.X, want)
	}

	// The row's own box: `calc(100% - 8u)` centred in the panel
	// (`launcher.css:840-843`), i.e. 4u of panel on either side of it. The
	// selected row is the one whose tint is painted, so it is the box read here.
	img := tt.Image()
	y := int(row.Y + row.H/2)
	panel := img.RGBAAt(int(cardLeft)+1, y)
	boxLeft, boxRight := -1, -1
	for x := int(cardLeft) + 1; x < int(cardRight); x++ {
		if img.RGBAAt(x, y) != panel {
			if boxLeft < 0 {
				boxLeft = x
			}
			boxRight = x
		}
	}
	if want := iround(cardLeft + 4*unit); absi(boxLeft-want) > 1 {
		t.Errorf("the row's box starts at x=%d, want the reference's 4u gutter (x=%d)", boxLeft, want)
	}
	if want := iround(cardRight - 4*unit); absi(boxRight-want) > 1 {
		t.Errorf("the row's box ends at x=%d, want the mirrored 4u gutter (x=%d)", boxRight, want)
	}
}

// The held modifier's row on the shipped frame: 「按住时默认加到底部」 is a
// position fact (the row is the list's last line, over a query as well as on
// the empty page), 「松手就消失」 a presence fact, and 「应该有选中样式」 the
// painted band — the row wears the selection tint across the same column the
// rows above it do, not the panel's own fill.
func TestTheHeldRowIsTheLastLineOfTheShippedList(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Type("ls")
	tt.Frame()

	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	results := a.Launcher.Results()
	if len(results) < 2 || results[len(results)-1].ID != "system-terminal-bare" {
		t.Fatalf("the held row is not the list's last line: %+v", results)
	}
	if a.Launcher.Selected != len(results)-1 {
		t.Errorf("the selection is on %d, want the held row %d", a.Launcher.Selected, len(results)-1)
	}
	row, ok := tt.Find("Open Terminal")
	if !ok {
		t.Fatalf("the held row was not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	cardLeft, cardRight := cardEdges(a)
	panel := img.RGBAAt(int(cardLeft)+1, int(row.Y+row.H/2))
	tinted := 0
	for x := int(cardLeft) + 1; x < int(cardRight); x++ {
		if img.RGBAAt(x, int(row.Y+row.H/2)) != panel {
			tinted++
		}
	}
	if tinted < 60 {
		t.Errorf("the held row's band is not tinted: %d differing pixels across the row's line", tinted)
	}

	// Released: the row is gone and the selection is back on the page's default.
	tt.HoldModifiers(0)
	tt.Frame()
	if hasLauncherItem(a, "system-terminal-bare") {
		t.Errorf("the held row survived the release: %+v", a.Launcher.Results())
	}
	if tt.HasText("Open Terminal") {
		t.Errorf("the released row is still on screen: %q", tt.Texts())
	}
}

// The user's second report, in one frame: the terminal *surface*'s row and the
// modifier-held *action* row are two different rows.
//
// The reference keeps the two registers apart in copy (`main:src/i18n.ts:886-887`
// `system.terminal` / `system.terminalSubtitle` for the catalog row, `:59-60`
// `launcher.terminalRow` / `launcher.terminalRowSubtitle` for the held row) and
// in place (the catalog row is an ordinary match, the held row is the list's
// last line and takes the selection). This build had used the held row's
// 「Open Terminal」 as the catalog row's own title, so a typed query drew the
// *action* word as an ordinary result — the report 「输入框中有输入内容时出现的
// 「在终端中打开」项：现在固定在第一项」. With the surface named 「Terminal」
// again, the action word is back to being one row, at the bottom, only while
// the modifier is down.
func TestTheTerminalSurfaceAndTheHeldActionAreTwoRows(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Type("terminal")
	tt.Frame()

	// The surface's row: an ordinary match, and no held row yet.
	surfaceRow, ok := tt.Find("Terminal")
	if !ok {
		t.Fatalf("the terminal surface's row is not drawn: %q", tt.Texts())
	}
	if tt.HasText("Open Terminal") {
		t.Errorf("the held row's action word is on the page with the modifier up: %q", tt.Texts())
	}
	results := a.Launcher.Results()
	if len(results) == 0 || results[0].ID != "terminal" {
		t.Fatalf("the query did not reach the terminal surface: %+v", results)
	}

	// Held: the action row joins as the list's last line and takes the
	// selection, below the surface row rather than above it.
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	heldRow, ok := tt.Find("Open Terminal")
	if !ok {
		t.Fatalf("the held row was not drawn: %q", tt.Texts())
	}
	results = a.Launcher.Results()
	if last := results[len(results)-1].ID; last != "system-terminal-bare" {
		t.Fatalf("the held row is not the list's last line: %+v", results)
	}
	if a.Launcher.Selected != len(results)-1 {
		t.Errorf("the selection is on %d, want the held row %d", a.Launcher.Selected, len(results)-1)
	}
	if heldRow.Y <= surfaceRow.Y {
		t.Errorf("the held row is drawn above the surface's own row: held y=%.0f, surface y=%.0f",
			heldRow.Y, surfaceRow.Y)
	}
}

// The card is the window less the platform's own inset — the second half of
// the user's first report.
//
// 「搜索页面左右两边各有一大块空白」 can only be a *third* band of blank: a card
// narrower than the window it was given. The reference has exactly one such
// band and it is the platform's: `.collapsed-card { width: 100% }`
// (`main:src/styles/launcher.css:8-13`) inside `.platform-linux
// .collapsed-shell { padding: calc(var(--u) * 10) }` (`base.css:1591-1607`) —
// the room the card's cast shadow lands in, 10u a side on Linux and 4u/10u on
// Windows, none on macOS. Its field row is full-bleed across that card
// (`:212-215`, the 16u is *padding*, not a narrower block) and so is the panel
// under it. So: the only unpainted columns are the platform's own, and the
// field's fill reaches the card's stroke on both sides.
func TestTheCardIsTheWindowLessThePlatformsInset(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Frame()

	field, ok := tt.Find("Search")
	if !ok {
		t.Fatalf("the field was not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	cardLeft, cardRight := cardEdges(a)
	y := int(field.Y + field.H/2)

	// The field block's own fill, sampled at the card's middle: a narrow block
	// would move this column's colour too, so the sample cannot be taken from
	// a column the block's edge might have passed over.
	fill := img.RGBAAt(int((cardLeft+cardRight)/2), y)
	runLeft, runRight := -1, -1
	for x := 0; x < 720; x++ {
		if img.RGBAAt(x, y) == fill {
			if runLeft < 0 {
				runLeft = x
			}
			runRight = x
		}
	}
	if runLeft < 0 {
		t.Fatalf("the field block was not painted at y=%d", y)
	}
	// The window's inset is all that is left of the frame, and the field block
	// runs from the card's own first pixel to its last: the stroke is painted
	// *inside* the card's box, not beside it.
	if want := int(cardLeft); runLeft != want {
		t.Errorf("the field block starts at x=%d, want x=%d: the card is narrower than the window's inset allows (a blank band inside the card)",
			runLeft, want)
	}
	if want := int(cardRight); runRight != want {
		t.Errorf("the field block ends at x=%d, want x=%d: the card is narrower than the window's inset allows (a blank band inside the card)",
			runRight, want)
	}
}

// cardEdges is the card's own content box on the shipped frame, in DIPs: the
// window (720) less the platform's inset — `App.inset` answers in the
// reference's units, so it is handed through the theme's own `Space` — and the
// card's 1px stroke, which is painted inside the card's box. The two numbers
// are the first and the last pixel the card's content owns.
func cardEdges(a *App) (left, right float32) {
	_, rightUnits, _, leftUnits := a.inset()
	spacing := float32(a.Launcher.Spacing)
	return leftUnits*spacing + 1, float32(720) - rightUnits*spacing - 2
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func iround(v float32) int {
	if v < 0 {
		return int(v - 0.5)
	}
	return int(v + 0.5)
}

func absi(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// The platform's inset is the reference's unit ladder, not a pixel constant.
//
// `base.css` writes the shell's padding as `calc(var(--u) * 10)` a side on
// Linux (`:1591-1607`), and `--u` is `1px * ui-scale` (`:53`) — so the card's
// distance to the window edge is nine pixels at the shipped `small` step, ten
// at `medium` and eleven at `large`, and the card is 720 less twice that at
// every one of them. The port had the pixel reading (10 a side) hard-coded, so
// the card was two pixels narrower than the design at the default step and two
// wider at `large` — the one horizontal band the launcher's own rules do not
// account for, and the only thing 「搜索页面左右大空白」 could ever have been.
// This pins the drawn card against the step's own unit.
func TestThePlatformInsetIsTheReferenceUnitLadder(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the ladder's Linux reading (10u a side) is the one asserted")
	}
	for _, step := range []string{"small", "medium", "large"} {
		a := appKeysAppAtScale(step)
		tt := appKeysRender(t, a)
		tt.Frame()

		unit := float32(a.Launcher.Spacing / 4)
		left, right := cardEdges(a)
		if want := 10*unit + 1; absf(left-want) > 0.5 {
			t.Errorf("%s: the card's content starts at %.1f, want the reference's 10u (%.1f)", step, left, want)
		}
		if want := float32(720) - 10*unit - 2; absf(right-want) > 0.5 {
			t.Errorf("%s: the card's content ends at %.1f, want the mirrored 10u (%.1f)", step, right, want)
		}

		// The drawn frame agrees: the field block is full-bleed across the
		// card, so the run of its own fill is the card's content box.
		field, ok := tt.Find("Search")
		if !ok {
			t.Fatalf("%s: the field was not drawn: %q", step, tt.Texts())
		}
		img := tt.Image()
		y := int(field.Y + field.H/2)
		fill := img.RGBAAt(int((left+right)/2), y)
		runLeft, runRight := -1, -1
		for x := 0; x < 720; x++ {
			if img.RGBAAt(x, y) == fill {
				if runLeft < 0 {
					runLeft = x
				}
				runRight = x
			}
		}
		if runLeft != int(left) || runRight != int(right) {
			t.Errorf("%s: the field block spans %d..%d, want the card's content box %d..%d",
				step, runLeft, runRight, int(left), int(right))
		}
	}
}
