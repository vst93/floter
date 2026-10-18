package shell

import (
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

	_, right, _, left := a.inset()
	cardLeft := left + 1 // the card's own 1px stroke sits inside its box
	cardRight := float32(720) - right - 1

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
	_, right, _, left := a.inset()
	panel := img.RGBAAt(int(left)+2, int(row.Y+row.H/2))
	tinted := 0
	for x := int(left) + 2; x < 720-int(right)-2; x++ {
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
