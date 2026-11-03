package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// P13-fk（`docs/liquid-glass.md` §6.132）：`cardRow` 的控制槽写着 `Shrink(0)`——
// 「控件不收缩，让标签列先让」。这句话此前只是代码里的一个 flag，没人量过它到底
// 会不会把标签列吃光、会不会把控件顶出卡框。
//
// 实测（720×900，五个页面，深浅两套逐值相同；行内容盒 515.0，控制槽 0…220）：
//
//	页面         最宽控制槽   剩下给标签列
//	General        220.0         260.0     ← `ui.Select(...).Width(220)`（字体族）
//	General        200.0         280.0
//	Shortcuts       81.0         399.0
//	Integrations    88.0         400.0
//	About           67.8         412.2
//
// 五页里最宽的一枚控件占掉内容盒的 220/515 = **42.7%**，标签列还剩 260（= 72px，
// 是 `Font family` 那 66 列墨的 **3.9 倍**）。`over`（控制槽右沿 − 行内容右沿）
// 在每一行、每一页、两套外观上都是 **0.0**：槽永远贴着内容右沿站，不越出去。
//
// 所以 `Shrink(0)` 是**对的**，也是**没量过**的：这条 pin 把它钉成两句话——
// ① 没有任何一行把墨画到卡框外（槽不越界）；② 控件自己的宽度不随窗宽变，变的是
// 标签列（`Grow(1)`）。谁把这两句反过来（控件长成弹性的、或标签列不再让位），
// 这里会红。

// contentRightAt720 is the settings card's row-content right edge at the one
// width the surface is drawn at (`launcher.InputWindowWidth` = 720): the card's
// own right border at x=703.8 minus the row's 12.6u inset and its 1px rule slot
// → 691.2. Measured, not derived from a formula: the row's control slot ends at
// exactly this column on every page (the switches are right-aligned), and the
// pin below re-measures that it is still true.
const contentRightAt720 = 691.2

// settingsProbeApp is a settings app with one integration rich enough to draw
// every row kind the integrations page has.
func settingsProbeApp() *App {
	a := New(settings.NewStore(settings.Default()), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
			Enabled: true, Running: true, RuntimeAvailable: true,
			Permissions: []string{"filesystem-read", "environment"},
			Enforced:    map[string]bool{"environment": true},
			Commands: []Command{
				{ID: "c1", Name: "One command", Enabled: true, Available: true},
			},
		}}
	}
	return a
}

// rightmostInk is the rightmost column inside r that paints anything, and
// leftmostInk the leftmost. They reuse `lineInk`, which takes the box's median
// level as the face: a row is mostly plane, so the median is the plane.
func rightmostInk(img *ui.Tester, r ui.Rect) (int, bool) {
	cols := lineInk(img.Image(), r)
	if len(cols) == 0 {
		return 0, false
	}
	return cols[len(cols)-1].X, true
}

func leftmostInk(img *ui.Tester, r ui.Rect) (int, bool) {
	cols := lineInk(img.Image(), r)
	if len(cols) == 0 {
		return 0, false
	}
	return cols[0].X, true
}

// TestTheControlKeepsItsWidthWhileTheLabelColumnTakesTheSlack is the slot's
// second sentence: `Shrink(0)` on the control and `Grow(1)` on the label column
// mean the control is intrinsic and the column is elastic. Measured on the
// widest control the settings pages have (the 220u font-family select): its
// painted span is the same at 720 and at 1600, while the label column grows.
func TestTheControlKeepsItsWidthWhileTheLabelColumnTakesTheSlack(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		tt720 := renderInAt(t, PageGeneral, launcher.InputWindowWidth, dark)
		tt1600 := renderInAt(t, PageGeneral, 1600, dark)
		label720 := mustFind(t, tt720, en.FontFamily)
		label1600 := mustFind(t, tt1600, en.FontFamily)

		// The control sits right of the label; read its own painted extent in
		// the label's band, from the label's right edge to the content edge.
		span := func(tt *ui.Tester, label ui.Rect, right float32) (int, int) {
			band := ui.Rect{X: label.X + label.W + 1, Y: label.Y - 2, W: right - (label.X + label.W + 1), H: label.H + 4}
			lo, ok := leftmostInk(tt, band)
			if !ok {
				t.Fatalf("dark=%v: the control right of %q paints nothing", dark, en.FontFamily)
			}
			hi, _ := rightmostInk(tt, band)
			return lo, hi
		}
		lo720, hi720 := span(tt720, label720, contentRightAt720)
		lo1600, hi1600 := span(tt1600, label1600, 1600-28.8)
		// The label column is the elastic one: at 1600 the picker starts 880px
		// further right — exactly the width the window grew by — because the
		// 260u column took all of it.
		if delta := float32(lo1600 - lo720); absFloat(delta-880) > 2 {
			t.Errorf("dark=%v: the control starts %v further right at 1600, want the 880px the window grew by "+
				"(the label column, not the control, takes the slack)", dark, delta)
		}

		for _, arm := range []struct {
			width int
			hi    int
			edge  float32
		}{
			{launcher.InputWindowWidth, hi720, contentRightAt720},
			{1600, hi1600, 1600 - 28.8},
		} {
			// The picker's own 1px face inset leaves its last painted column
			// 1.2px inside the box edge — the same 1.2 at both widths, which is
			// what makes it a measurement of the edge rather than of a glyph.
			if absFloat(float32(arm.hi)-arm.edge) > 2 {
				t.Errorf("dark=%v width=%d: the picker's last painted column is x=%d, %.1f off the row's "+
					"content edge %.1f: the control column is not right-aligned",
					dark, arm.width, arm.hi, arm.edge-float32(arm.hi), arm.edge)
			}
		}
		if got, want := hi1600-lo1600, hi720-lo720; absFloat(float32(got-want)) > 1 {
			t.Errorf("dark=%v: the picker paints %d columns at 720 but %d at 1600: "+
				"the control is absorbing the window's extra width instead of the label column",
				dark, want, got)
		}
		// The label column is the widest control's leftover: 260u at 720 — and
		// the label is not the thing that gives, so its painted span is the same
		// as it is in the 1140u column at 1600.
		if absFloat(label720.W-260) > 1 {
			t.Errorf("dark=%v: the label column beside the 220u picker is %.1fu, want the measured 260u",
				dark, label720.W)
		}
		labelEnd720, _ := rightmostInk(tt720, label720)
		labelEnd1600, _ := rightmostInk(tt1600, label1600)
		if got, want := labelEnd720-int(label720.X), labelEnd1600-int(label1600.X); got != want {
			t.Errorf("dark=%v: %q paints %d columns at 720 but %d at 1600: the label is being cut in the "+
				"narrow column", dark, en.FontFamily, got, want)
		}
	}
}

// renderInAt draws one page at one width and one appearance.
func renderInAt(t *testing.T, page, width int, dark bool) *ui.Tester {
	t.Helper()
	a := settingsProbeApp()
	a.Page = page
	tt, _ := renderIn(t, a, width, 1400, dark)
	return tt
}
