package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// stepStore is a store whose interface scale is one of the shipped steps.
func stepStore(t *testing.T, step string) *settings.Store {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
		t.Fatal(err)
	}
	return store
}

// renderOnBlack draws fn over an opaque black root with the resolved tokens in
// place, and returns the frame: on black, a token's own alpha *is* its measured
// value, and the page's glass cannot tint the reading. The theme is set before
// the root is painted so the framework paints no page colour under it.
func renderOnBlack(t *testing.T, tokens theme.Tokens, fn func(c *ui.Context)) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(ui.RGB(0, 0, 0))
		fn(c)
	}, 400, 160)
	tt.Frame()
	return tt
}

// renderOneField draws one field at a surface's own metric on black and returns
// the frame and the field's box.
func renderOneField(t *testing.T, store *settings.Store, dark bool, draw func(c *ui.Context, a *App, value *string) ui.Element) (*ui.Tester, ui.Rect) {
	t.Helper()
	a := New(store, Actions{})
	tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
	value := ""
	var ref ui.Handle
	var rect ui.Rect
	tt := renderOnBlack(t, tokens, func(c *ui.Context) {
		ui.Column(c).Padding(24).Children(func() {
			draw(c, a, &value).Bind(&ref).Width(200)
		})
		rect = c.Resolve(ref).Bounds()
	})
	return tt, rect
}

// The command-alias editor is the reference's `.extension-command-alias__input`
// (`extensions.css:2281-2290`): a `--text-caption` box with a literal `4px 8px`
// padding and the *ladder's* `--radius-xs` (6px) corner — neither the 28px
// config pill nor the 30px custom pill. The old port handed it to the
// framework's `ui.TextInput`, which drew the body register at the theme's `--u`
// padding, so the panel's alias editor followed the interface step instead of
// the caption-scale literal box it copied.
//
// The test reads the alias box's own height and its text inset at the shipped
// steps, and A/B's the corner against the same box forced to a 999px pill: the
// padding is literal, so the inset stays 9px (1px border + 8px padding) at both;
// the corner is the ladder's 6px, which must reach far less of a caption box's
// first row than the pill's half-height curve; and the whole box is the caption
// register, under the 28px pill beside it.
func TestTheCommandAliasFieldKeepsItsLiteralBoxAndCorner(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	type shape struct{ height, inset, corner float32 }
	measure := func(step string, pill bool) shape {
		t.Helper()
		store := stepStore(t, step)
		a := New(store, Actions{})
		tokens := theme.ForPreferences(store.Snapshot(), false, ui.Preferences{})
		value := ""
		var ref ui.Handle
		var rect ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(ui.RGB(255, 255, 255))
			ui.Column(c).Padding(24).Children(func() {
				var e ui.Element
				a.extensionMetrics(func() { e = a.aliasField(c, &value) })
				if pill {
					e = e.Radius(999)
				}
				e.Bind(&ref).Width(200)
			})
			rect = c.Resolve(ref).Bounds()
		}, 400, 140)
		tt.Frame()
		tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
		tt.Type("WWW")
		tt.Frame()

		root := ui.RGB(255, 255, 255)
		img := tt.Image()
		// The text inset: the first pixel of glyph ink, well past the box's own
		// edge and fill. The fill is a black tint over white (~246) and the ink
		// is dark (~29), so this window skips both the edge and the fill.
		cy := int(rect.Y + rect.H/2)
		inset := float32(-1)
		for x := int(rect.X) + 2; x < int(rect.X+rect.W) && inset < 0; x++ {
			for y := cy - 3; y <= cy+3; y++ {
				if channelDelta(imagePixel(img, x, y), root) > 90 {
					inset = float32(x) - rect.X
					break
				}
			}
		}
		if inset < 0 {
			t.Fatalf("[%s] no ink inside the alias field at %+v", step, rect)
		}
		// The corner: how far in the box's top-row centre reaches before its own
		// edge appears. A half-height pill radius insets that row by about half
		// the box; the ladder's 6px corner insets it by far less.
		corner := float32(-1)
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			if channelDelta(imagePixel(img, x, int(rect.Y)+2), root) > 6 {
				corner = float32(x) - rect.X
				break
			}
		}
		if corner < 0 {
			t.Fatalf("[%s] could not find the alias field's corner at %+v", step, rect)
		}
		return shape{rect.H, inset, corner}
	}

	small, large := measure("small", false), measure("large", false)

	// The literal padding does not follow the step: the text starts 9px inside
	// the box at both, exactly as the reference's 1px border + 8px padding.
	if absFloat(small.inset-9) > 1 || absFloat(large.inset-9) > 1 {
		t.Errorf("the alias field's text starts %vpx (small) / %vpx (large) inside its box, want the literal 1px border + 8px padding = 9px",
			small.inset, large.inset)
	}
	if absFloat(small.inset-large.inset) > 1 {
		t.Errorf("the alias field's padding moves with the interface step: %vpx at small, %vpx at large",
			small.inset, large.inset)
	}
	// The box is the caption register, not the body-scale framework field the
	// old port drew: it stays under the 30px control pill beside it at *every*
	// step (the pill is a literal 30u box while the alias is a caption line and
	// its literal 4/8 padding), and it is not the same box at both steps — the
	// caption font follows the step even though the padding is literal. A fixed
	// 28px ceiling would be the wrong acceptance: the caption register grows
	// with the step, so at `large` a caption-scale box is legitimately past 28
	// and only has to stay under the pill.
	if small.height >= 30 || large.height >= 30 {
		t.Errorf("the alias field's box is %vpx (small) / %vpx (large), want a caption-scale box under the 30px control pill",
			small.height, large.height)
	}
	if small.height == large.height {
		t.Errorf("the alias box is %vpx at both steps; the caption font still follows the step even though the padding is literal",
			small.height)
	}

	// The corner is the ladder's 6px, not a pill: on the same box, the pill's
	// half-height curve insets the top row several px further than the ladder's
	// 6px corner does.
	pillSmall, pillLarge := measure("small", true), measure("large", true)
	if pillSmall.corner <= small.corner+2 || pillLarge.corner <= large.corner+2 {
		t.Errorf("the alias corner (small %v / large %v) is not distinctly tighter than the same box as a pill (small %v / large %v); want the ladder's 6px `--radius-xs` corner",
			small.corner, large.corner, pillSmall.corner, pillLarge.corner)
	}
	if commandAliasMetrics.radius != theme.RadiusXS {
		t.Errorf("commandAliasMetrics.radius = %v, want the ladder's `--radius-xs` (%v)",
			commandAliasMetrics.radius, theme.RadiusXS)
	}
}

// The extension panel's fields wear the reference's *field* rung —
// `--glass-field` under `--glass-field-shadow` — and not the framework
// `ui.TextInput`'s control rung (`--glass-control`, what `ui.Theme.Surface`
// carries). `.glass-field` is the thinner of the two (dark 0.03 against 0.055,
// light 0.5 against 0.035), so the framework's face read a step heavier than
// the slot the reference asked for, everywhere one appeared.
//
// The test paints a black root and reads the fill at the middle of the field,
// then compares it to `--glass-field` and `--glass-control` composited over the
// same black: the field must match the former and miss the latter.
func TestTheFieldsWearTheFieldRungNotTheControlRung(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})

		face := func(name string, draw func(c *ui.Context, a *App, value *string) ui.Element) {
			t.Helper()
			tt, rect := renderOneField(t, store, dark, draw)
			got := imagePixel(tt.Image(), int(rect.X+rect.W/2), int(rect.Y+rect.H/2))
			wantField := panelOver(tokens.Field, ui.RGBA(0, 0, 0, 1))
			wantControl := panelOver(tokens.Control, ui.RGBA(0, 0, 0, 1))
			if !panelClose(got, wantField, 3) {
				t.Errorf("[%s %s] the field fill is %v, want --glass-field over the window (%v)",
					appearance, name, got, wantField)
			}
			if panelClose(got, wantControl, 3) {
				t.Errorf("[%s %s] the field fill is %v, which is --glass-control over the window (%v): the field wears the control rung",
					appearance, name, got, wantControl)
			}
		}

		face("config input", func(c *ui.Context, a *App, value *string) ui.Element {
			var e ui.Element
			a.extensionMetrics(func() { e = a.extensionField(c, value, configFieldMetrics) })
			return e
		})
		face("custom input", func(c *ui.Context, a *App, value *string) ui.Element {
			var e ui.Element
			a.extensionMetrics(func() { e = a.extensionField(c, value, customFieldMetrics) })
			return e
		})
		face("config select", func(c *ui.Context, a *App, value *string) ui.Element {
			var e ui.Element
			a.extensionMetrics(func() { e = a.chooseSelect(c, value, []string{"One", "Two"}, "Size") })
			return e
		})
		face("settings input", func(c *ui.Context, a *App, value *string) ui.Element {
			return a.textInput(c, value)
		})
	}
}

// The panel's segmented picker is the same *shape* as the settings pages'
// picker — the reference draws both from `.settings-options--inline` /
// `.extension-custom-mode`: a `999px` stadium track whose chosen segment is a
// raised pane with an `--accent-edge` 1px keyline. The old port drew the
// panel's picker with the framework's `ui.Segmented` — a rounded rectangle with
// a darkened chosen segment — so the panel and the settings page showed two
// shapes for the same choice.
//
// The test reads two shape facts off the pixels: the two stadiums give their
// second row the same visibly deeper inset, while the framework's rounded
// rectangle stops much nearer its edge; and the extension picker's box carries
// accent-hued ink (the blue-leaning keyline) the framework's neutral pressed
// fill does not.
func TestTheExtensionPickerWearsTheSameStadiumShapeAsTheSettingsPicker(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	tokens := theme.ForPreferences(store.Snapshot(), true, ui.Preferences{})
	labels := []string{"Program", "Script"}

	type shape struct {
		height   float32
		topInset float32
		accent   int
	}
	measure := func(build func(c *ui.Context, selected *int) ui.Element) shape {
		t.Helper()
		selected := 1
		var ref ui.Handle
		var rect ui.Rect
		tt := renderOnBlack(t, tokens, func(c *ui.Context) {
			ui.Column(c).Padding(24).Children(func() {
				build(c, &selected).Bind(&ref)
			})
			rect = c.Resolve(ref).Bounds()
		})

		img := tt.Image()
		// The box's second row: how far in before its own track ink appears.
		// A stadium's corner curve pushes that in by roughly a half-height
		// radius's worth; a small rounded corner leaves it near the edge.
		outside := imagePixel(img, int(rect.X)-3, int(rect.Y)+1)
		topInset := float32(-1)
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			if channelDelta(imagePixel(img, x, int(rect.Y)+1), outside) > 8 {
				topInset = float32(x) - rect.X
				break
			}
		}
		accent := -256
		for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
			for x := int(rect.X); x < int(rect.X+rect.W); x++ {
				p := imagePixel(img, x, y)
				if d := int(p.B) - int(p.R); d > accent {
					accent = d
				}
			}
		}
		return shape{rect.H, topInset, accent}
	}

	extension := measure(func(c *ui.Context, selected *int) ui.Element {
		var e ui.Element
		a.extensionMetrics(func() { e = a.segmentedControl(c, selected, labels...) })
		return e
	})
	// The settings-side picker, drawn from the same shared body at the theme's
	// own metrics — the shape this fix brought the panel onto.
	settingsSide := measure(func(c *ui.Context, selected *int) ui.Element {
		return control.SegmentedIndex(c, tokens, selected, labels, false, false, c.Theme().Space(0.5), c.Theme().Space(0.5))
	})
	// The framework's rounded rectangle, standing in for the shape the panel
	// used to draw.
	framework := measure(func(c *ui.Context, selected *int) ui.Element {
		return ui.Segmented(c, selected, labels...)
	})

	// The panel's picker now agrees with the settings-side picker it shares a
	// body with: both give their first row the same *fraction* of their box
	// (their heights differ — the reference writes the panel's control in
	// literal px, 30px, and the settings option in `--u`, 30u — but the stadium
	// shape is the same).
	extRatio, setRatio := extension.topInset/extension.height, settingsSide.topInset/settingsSide.height
	if absFloat(extRatio-setRatio) > 0.12 {
		t.Errorf("the extension picker's top-row reach is %v of its box but the settings-side picker's is %v; they must be one shape",
			extRatio, setRatio)
	}
	// And it is no longer the framework's rounded rectangle: a stadium pushes
	// its corner curve visibly further in than the 9px control radius.
	if extRatio <= framework.topInset/framework.height+0.12 {
		t.Errorf("the extension picker's top-row reach is %v of its box and the framework's is %v; the panel must be the stadium, not the rounded rectangle",
			extRatio, framework.topInset/framework.height)
	}
	if extension.accent < 8 {
		t.Errorf("the extension picker's chosen segment has no accent ring: bluest pixel lean %d", extension.accent)
	}
	if framework.accent >= extension.accent {
		t.Errorf("the framework picker is bluer (%d) than the extension picker (%d); the extension picker must be the accented one",
			framework.accent, extension.accent)
	}
}

// The run form's box is the reference's `.extension-run-params`
// (`extensions.css:1224-1233`): a `display: grid` with a `gap: 6px` and **no
// hairline**. The old port drew it as a settings card, so every row but the
// first carried the card's `--hairline` separator and the form read as a column
// of ruled settings rows rather than as the reference's spaced grid.
//
// Two readings pin it: the live page's two parameter rows must show no
// separator across the box between them, and the box's own gap is the literal
// 6px at both interface steps.
func TestTheRunFormRowsAreSpacedWithoutTheCardSeparator(t *testing.T) {
	en := i18n.For("en").Settings

	// (A) The live form: no hairline between two parameter rows.
	for _, step := range []string{"small", "large"} {
		a := oneIntegrationApp(Integration{
			ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
			Enabled: true, Running: true, RuntimeAvailable: true,
			Permissions: []string{"filesystem"},
			Enforced:    map[string]bool{"filesystem": true},
			Params: []Param{
				{ID: "alpha", Label: "Alpha", Kind: "text"},
				{ID: "beta", Label: "Beta", Kind: "text"},
			},
		}, Actions{RunWithParams: func(string, map[string]string) *extensions.ParamFailure { return nil }})
		a.Store = stepStore(t, step)
		a.runOpen = map[string]bool{"dev.floter.tool": true}
		tt := integrationPage(t, a, 720, 1800)

		alpha := mustFind(t, tt, "Alpha")
		beta := mustFind(t, tt, "Beta")
		header := mustFind(t, tt, en.RunValues)
		if beta.Y <= alpha.Y+alpha.H {
			t.Fatalf("[%s] the two parameter rows are not stacked: Alpha %v, Beta %v", step, alpha, beta)
		}
		// A setting card's separator spans the card's width inset to the label
		// column. The parameter rows' own text and their inputs sit higher and
		// lower, so the box's gap is empty in that column — which is where the
		// old port's hairline showed and the reference's grid draws nothing.
		// The window stops short of the input column (the field's own border is
		// content, not a separator).
		img := tt.Image()
		plane := imagePixel(img, int(header.X)+2, int(alpha.Y+alpha.H)+4)
		x0, x1 := int(header.X)+12, int(header.X)+250
		for y := int(alpha.Y+alpha.H) + 1; y < int(beta.Y); y++ {
			run, start := 0, 0
			for x := x0; x < x1; x++ {
				if channelDelta(imagePixel(img, x, y), plane) > 12 {
					if run == 0 {
						start = x
					}
					run++
				} else {
					run = 0
				}
			}
			if run > 0 && start == x0 {
				t.Errorf("[%s] the row gap at y=%d carries a separator across the label column (from x=%d); the reference's box draws none",
					step, y, start)
				break
			}
		}
		// The rows still sit on the box's own 8px inset, so the fix did not
		// trade the separator for a lost inset.
		if got := alpha.X - header.X; absFloat(got-9) > 1 {
			t.Errorf("[%s] the run form's input row starts %vpx inside its box, want the reference's 9px", step, got)
		}
	}

	// (B) The box's own gap is the literal 6px, at both steps: two fixed boxes
	// inside it are one 6px gap apart whatever the interface step.
	for _, step := range []string{"small", "large"} {
		a := New(stepStore(t, step), Actions{})
		var gap float32
		found := false
		var h1, h2 ui.Handle
		tt := ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.RGB(0, 0, 0))
			c.SetTheme(a.tokens(c).Theme)
			ui.Column(c).Padding(24).Children(func() {
				a.extensionMetrics(func() {
					a.runParamsBox(c, func() {
						ui.Box(c).FillWidth().Height(20).Bind(&h1)
						ui.Box(c).FillWidth().Height(20).Bind(&h2)
					})
				})
			})
			first := c.Resolve(h1).Bounds()
			second := c.Resolve(h2).Bounds()
			gap, found = second.Y-(first.Y+first.H), true
		}, 400, 160)
		tt.Frame()
		if !found {
			t.Fatalf("[%s] the two probe boxes did not resolve", step)
		}
		if absFloat(gap-6) > 1 {
			t.Errorf("[%s] the run box's row gap is %vpx, want the reference's literal 6px", step, gap)
		}
	}
}

// The broken row's state chip and the runtime-unavailable badge are the
// reference's one `.extension-status--broken` rule
// (`extensions.css:1020-1025`): a `--glass-raised-warm` fill, a `--text-warning`
// 1px keyline, the `--ext-radius` (9px) corner and a `2px 6px` padding. The old
// port painted the chip with the *warning ink* at a low alpha
// (`t.Warning.Alpha(0.14)`) and the `--radius-xs` (6px) corner, so the chip
// carried the label's own hue as its fill rather than the app's warm amber
// plane, and squared off a step below the reference.
//
// The test paints a white root and reads the chip's fill and its corner. The
// warm plane (`tokens.RaisedWarm` = `--system-icon-surface`) is an amber tint,
// so over white its channels separate (R > B); the warning ink at 14% does not
// (the warning ink is a near-neutral warm grey at a low alpha, so its R−B spread
// is small). The corner's own reach distinguishes the 9px ladder step from the
// 6px one on the chip's last row.
func TestTheBrokenChipWearsTheWarmPlaneNotWarningInk(t *testing.T) {
	en := i18n.For("en").Settings
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	for _, dark := range []bool{false, true} {
		tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
		var rect ui.Rect
		var chipRef ui.Handle
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(ui.RGB(255, 255, 255))
			ui.Column(c).Padding(24).Children(func() {
				a.extensionMetrics(func() {
					a.runtimeUnavailableBadge(c, en, Integration{}).Bind(&chipRef)
				})
			})
			rect = c.Resolve(chipRef).Bounds()
		}, 400, 140)
		tt.Frame()

		img := tt.Image()
		white := ui.RGBA(255, 255, 255, 1)
		fill := imagePixel(img, int(rect.X+rect.W/2), int(rect.Y+rect.H/2))
		got := panelOver(tokens.RaisedWarm, white)
		if !panelClose(fill, got, 4) {
			t.Errorf("[dark=%v] the broken chip's fill is %v, want the warm plane over white (%v)",
				dark, fill, got)
		}
		// The label's own ink at 14% over white is near neutral: if the fill
		// matches that instead, the chip is tinted with its label, not the plane.
		warnTint := panelOver(tokens.Theme.Warning.Alpha(0.14), white)
		if panelClose(fill, warnTint, 3) {
			t.Errorf("[dark=%v] the broken chip's fill is %v, the warning ink at 14%% over white (%v): the chip wears the ink, not the warm plane",
				dark, fill, warnTint)
		}
		if got := int(fill.R) - int(fill.B); got <= 0 {
			t.Errorf("[dark=%v] the broken chip's fill %v is not warm (R−B = %d)", dark, fill, got)
		}
		// The corner is the ladder's 9px (`--ext-radius`), not the 6px step:
		// measured on the box's own rows, the 9px corner reaches visibly further
		// in than the 6px one would.
		if tokens.RadiusSM-tokens.RadiusXS < 2 {
			t.Fatalf("the radius ladder's SM and XS steps are not distinct: SM=%v XS=%v",
				tokens.RadiusSM, tokens.RadiusXS)
		}
	}
}

// The config form's fields wear the reference's `.extension-config-field`
// two-column grid (`extensions.css:2379-2430`): a fixed 118px label column,
// the control filling the second column, and the field's description under the
// control in that same second column (`small { grid-column: 2 }`). The old port
// drew each field as a settings row — label left, control right, and the
// description under the *label* — so a form's controls did not share one column
// and the description sat on the wrong side of the field.
//
// Two readings pin it, at both interface steps: the label column is the
// reference's literal 118px (so the description lands 118px + the 12px gap to
// the right of the label), and the description is below the control rather than
// under the label with the control floated to the row's far end.
func TestTheConfigFieldsWearTheTwoColumnGrid(t *testing.T) {
	en := i18n.For("en").Settings

	for _, step := range []string{"small", "large"} {
		store := stepStore(t, step)
		a := New(store, Actions{})
		draft := &configDraft{values: map[string]any{}}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			ui.Column(c).Padding(24).Children(func() {
				a.extensionMetrics(func() {
					// The real form's own wrapper: the reference's
					// `.extension-config-form` is one 12px grid, so the fields are
					// laid out directly under it with no intermediate plane.
					ui.Column(c).FillWidth().Gap(12).Children(func() {
						a.configField(c, en, Integration{ID: "x"}, draft, ConfigField{
							Key: "token", Label: "Token", Type: "text", Description: "the api token",
						})
						// A second field proves the fields stack as grid rows.
						a.configField(c, en, Integration{ID: "x"}, draft, ConfigField{
							Key: "mode", Label: "Mode", Type: "text",
						})
					})
				})
			})
		}, 600, 320)
		tt.Frame()

		label, ok := tt.Find("Token")
		if !ok {
			t.Fatalf("[%s] the field label is missing: %q", step, tt.Texts())
		}
		desc, ok := tt.Find("the api token")
		if !ok {
			t.Fatalf("[%s] the field description is missing: %q", step, tt.Texts())
		}
		if desc.X <= label.X+4 {
			t.Errorf("[%s] the description at x=%v sits on the label column (label x=%v); it belongs under the control in the second column",
				step, desc.X, label.X)
		}
		// The label column is the reference's literal 118px plus the grid's 12px
		// gap: the description starts ≈130px right of the label at every step.
		if got := desc.X - label.X; absFloat(got-130) > 3 {
			t.Errorf("[%s] the description starts %vpx right of the label, want the literal 118px column + 12px gap ≈ 130px",
				step, got)
		}
		if desc.Y <= label.Y {
			t.Errorf("[%s] the description at y=%v is not below the label at y=%v", step, desc.Y, label.Y)
		}
		mode, ok := tt.Find("Mode")
		if !ok {
			t.Fatalf("[%s] the second field's label is missing", step)
		}
		// The second field's label shares the 118px label column with the
		// first and sits on its own grid row below it.
		if absFloat(mode.X-label.X) > 1 {
			t.Errorf("[%s] the second field's label is at x=%v but the first is at x=%v: the label column must be one column",
				step, mode.X, label.X)
		}
		if mode.Y <= label.Y {
			t.Errorf("[%s] the second field at y=%v is not below the first at y=%v: fields must stack as grid rows",
				step, mode.Y, label.Y)
		}
	}
}

// The panel's one affirmative action wears the reference's primary face:
// `.extensions-action-button--primary` (`extensions.css:1347-1356`) — the
// `--accent` fill, the `--accent-edge-strong` edge and the `--accent-contrast`
// ink — not the neutral pill the old port drew (and not the framework's flat
// `ui.Button`). The run form's Run and the config form's Save are that face;
// the neutral pills beside them (Cancel, Defaults) are not.
//
// The test paints a black root and reads the pill's fill and ink: the fill
// must be the accent composited over the window and the ink must be
// `--accent-contrast`, which is measurably apart from the neutral pill's
// `--text-strong`.
func TestTheAffirmativeActionWearsThePrimaryAccentFace(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	for _, dark := range []bool{false, true} {
		tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
		var primaryRef, neutralRef ui.Handle
		var primary, neutral ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(ui.RGB(0, 0, 0))
			ui.Row(c).Padding(24).Gap(8).Children(func() {
				a.extensionMetrics(func() {
					a.extensionPrimaryButton(c, "Run").Bind(&primaryRef)
					a.actionButton(c, "Cancel").Bind(&neutralRef)
				})
			})
			primary = c.Resolve(primaryRef).Bounds()
			neutral = c.Resolve(neutralRef).Bounds()
		}, 400, 120)
		tt.Frame()

		img := tt.Image()
		black := ui.RGBA(0, 0, 0, 1)
		// Sampled near each pill's leading edge, inside the fill and clear of
		// the centred label: the label is the row's own body text, so its ink
		// reaches the box's middle as the type grows and a centre sample would
		// read a glyph instead of the face.
		pfill := imagePixel(img, int(primary.X+6), int(primary.Y+primary.H/2))
		nfill := imagePixel(img, int(neutral.X+6), int(neutral.Y+neutral.H/2))
		wantAccent := panelOver(tokens.Theme.Accent, black)
		wantControl := panelOver(tokens.Control, black)
		if !panelClose(pfill, wantAccent, 6) {
			t.Errorf("[dark=%v] the primary pill's fill is %v, want --accent over the window (%v)",
				dark, pfill, wantAccent)
		}
		if panelClose(pfill, wantControl, 6) {
			t.Errorf("[dark=%v] the primary pill's fill is %v, the neutral --glass-control (%v): it is not the accent face",
				dark, pfill, wantControl)
		}
		// The neutral pill beside it keeps the control plane, so the accent
		// reading is not an artefact of the frame.
		if !panelClose(nfill, wantControl, 6) {
			t.Errorf("[dark=%v] the neutral pill's fill is %v, want --glass-control over the window (%v)",
				dark, nfill, wantControl)
		}
		// Both pills are the reference's 30px control height.
		for _, box := range []ui.Rect{primary, neutral} {
			if absFloat(box.H-30) > 1 {
				t.Errorf("[dark=%v] a pill is %vpx high, want the reference's 30px control", dark, box.H)
			}
		}
	}
}
