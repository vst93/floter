package launcher

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// over lays a translucent color over an opaque one, the way the renderer
// composites a row's fill on the panel beneath it.
func over(over ui.Color, base color.RGBA) color.RGBA {
	a := float64(over.A) / 255
	mix := func(o, b uint8) uint8 {
		return uint8(float64(o)*a + float64(b)*(1-a) + 0.5)
	}
	return color.RGBA{mix(over.R, base.R), mix(over.G, base.G), mix(over.B, base.B), 255}
}

func closeRGB(a, b color.RGBA) bool {
	diff := func(x, y uint8) int {
		d := int(x) - int(y)
		if d < 0 {
			d = -d
		}
		return d
	}
	return diff(a.R, b.R) <= 2 && diff(a.G, b.G) <= 2 && diff(a.B, b.B) <= 2
}

// The selected result row's fill is the reference's `--accent-tint-quiet`, a
// *per-appearance* token: the dark appearance writes 0.085 of its pastel blue,
// the light one 0.065 of its darker blue (`main:src/styles/base.css:683,945`;
// `.launcher-result--selected`, `main:src/styles/launcher.css:1156`). The port
// hard-coded `Accent.Alpha(0.085)` for both appearances, so the light launcher
// painted its chosen row with the dark rung — a heavier wash over white than
// the reference asked for.
//
// The test renders the empty page in each appearance and reads the selected
// row's own band on its blank right side (past the text, where the row's fill
// shows with nothing over it): the pixel must equal the token composited over
// the *unselected* row's fill, so the assertion names the exact token rather
// than "tinted somehow".
func TestTheSelectedRowWearsThePerAppearanceQuietTint(t *testing.T) {
	rowFill := func(themeName string) (sel, rest color.RGBA, tokens theme.Tokens) {
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = themeName }); err != nil {
			t.Fatal(err)
		}
		tt := render(t, a)
		tt.Frame()
		img := tt.Image()
		first, ok := tt.Find("Open settings") // the empty page's selected first row
		if !ok {
			t.Fatalf("%s: the first result was not drawn: %q", themeName, tt.Texts())
		}
		second, ok := tt.Find("Open terminal") // the unselected row one band below
		if !ok {
			t.Fatalf("%s: the second result was not drawn: %q", themeName, tt.Texts())
		}
		// A blank x well to the right of both titles: the row's fill with no
		// glyph or text ink over it.
		const x = 600
		sel = img.RGBAAt(x, int(first.Y+first.H/2))
		rest = img.RGBAAt(x, int(second.Y+second.H/2))
		tokens = theme.ForPreferences(a.settings(), themeName == "dark", ui.Preferences{})
		return sel, rest, tokens
	}

	lightSel, lightRest, lightTok := rowFill("light")
	darkSel, darkRest, darkTok := rowFill("dark")

	// The two appearances' tokens must differ (that is the reference's own
	// split: dark 0.085, light 0.065).
	if lightTok.AccentTintQuiet == darkTok.AccentTintQuiet {
		t.Fatalf("the quiet tint is one value for both appearances: %v", lightTok.AccentTintQuiet)
	}

	for _, tc := range []struct {
		name       string
		sel, rest  color.RGBA
		wantColor  ui.Color
		otherColor ui.Color
	}{
		{"light", lightSel, lightRest, lightTok.AccentTintQuiet, darkTok.AccentTintQuiet},
		{"dark", darkSel, darkRest, darkTok.AccentTintQuiet, lightTok.AccentTintQuiet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := over(tc.wantColor, tc.rest)
			if !closeRGB(tc.sel, want) {
				t.Errorf("the selected row is %v, want %v (the quiet tint %v over the panel %v)",
					tc.sel, want, tc.wantColor, tc.rest)
			}
			// The *other* appearance's rung must not be what is painted: that
			// is exactly the bug this pins (one hard-coded alpha).
			other := over(tc.otherColor, tc.rest)
			if closeRGB(tc.sel, other) {
				t.Errorf("the %s selection is painted with the other appearance's rung %v (%v)",
					tc.name, tc.otherColor, other)
			}
		})
	}
}

// The power rows' warm plate carries its own ink. The reference's
// `.launcher-result__icon--system` (`main:src/styles/launcher.css:1232-1235`)
// paints `--system-icon-surface` with `--system-icon-text` — the amber ink
// (`base.css:705,963`) — **not** the warning colour, which is a near-neutral
// red-grey reserved for a label. The port drew the glyph in `t.Warning` on a
// plate filled with `t.Warning.Alpha(0.14)`: both the plane and its ink were
// the warning colour, which is the wrong hue on either appearance.
//
// The plate is read at its centre (the glyph covers it, so the *ink* is what
// shows): over the warm plane the glyph's own hue must lead red over blue, as
// the amber ink does and the warning colour does not.
func TestThePowerGlyphsWearTheSystemIconInk(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Frame()

	// The plate's darkest pixel is the glyph's own ink; it must match the
	// amber system-icon token, and must *not* match the warning colour.
	ink := darkestInCell(tt, "Shut down the computer")
	tokens := theme.ForPreferences(a.settings(), false, ui.Preferences{})
	want := opaque(tokens.SystemIconText)
	if !closeRGB(ink, want) {
		t.Errorf("the power glyph is %v, want the amber system-icon ink %v", ink, want)
	}
	if warn := opaque(tokens.Theme.Warning); closeRGB(ink, warn) {
		t.Errorf("the power glyph wears the warning colour %v, not the amber system-icon ink", warn)
	}
}

// darkestInCell returns the darkest pixel in a row's 28u icon cell: the glyph
// ink, on whatever plane it sits.
func darkestInCell(tt *ui.Tester, title string) color.RGBA {
	rect, ok := tt.Find(title)
	if !ok {
		panic("row not drawn: " + title)
	}
	img := tt.Image()
	best := img.RGBAAt(int(rect.X)-34, int(rect.Y))
	sum := func(c color.RGBA) int { return int(c.R) + int(c.G) + int(c.B) }
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X) - 34; x < int(rect.X)-12; x++ {
			if c := img.RGBAAt(x, y); sum(c) < sum(best) {
				best = c
			}
		}
	}
	return best
}

// opaque renders a translucent colour over white at full opacity, as the
// glyph ink is drawn once the plane behind it has been composited.
func opaque(c ui.Color) color.RGBA {
	a := float64(c.A) / 255
	mix := func(v uint8) uint8 { return uint8(float64(v)*a + 255*(1-a) + 0.5) }
	return color.RGBA{mix(c.R), mix(c.G), mix(c.B), 255}
}

// The power confirmation band is the reference's `.launcher-system-confirm`
// (`main:src/styles/launcher.css:756-770`): a `--system-icon-surface` fill —
// the warm amber plane a power entry wears — carrying the warning ink on its
// glyph and sentence. The port had filled the band with `t.Warning.Alpha(0.12)`,
// the *label's* colour as the pane's wash: the same mistake the launcher's
// feedback row made. The band's own fill is compared exactly against the warm
// token composited over the panel, and against the warning wash — the two
// candidate fills differ by a few units, so the assertion names the token
// rather than reading the pixel as "warm".
func TestThePowerConfirmBandWearsTheWarmPlane(t *testing.T) {
	a := testApp()
	a.Actions.Power = func(string) {}
	tt := render(t, a)
	tt.Type("restart")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if a.PowerPending() != PowerRestart {
		t.Fatalf("pending = %q", a.PowerPending())
	}
	msg, ok := tt.Find(a.copy().PowerConfirmMessage(PowerRestart))
	if !ok {
		t.Fatalf("the confirmation is not drawn: %q", tt.Texts())
	}
	// The gap between the glyph and the sentence is the band's own fill.
	band := tt.Image().RGBAAt(int(msg.X)-2, int(msg.Y+msg.H/2))
	tokens := theme.ForPreferences(a.settings(), false, ui.Preferences{})
	panel := color.RGBA{254, 254, 254, 255}
	want := over(tokens.IconSurface, panel)
	wash := over(tokens.Theme.Warning.Alpha(0.12), panel)
	if band != want {
		t.Errorf("the confirmation band is %v, want the warm plane %v over the panel (→ %v)", band, tokens.IconSurface, want)
	}
	if band == wash {
		t.Errorf("the confirmation band is the warning ink at 12%% (%v), not the warm plane", wash)
	}
}
