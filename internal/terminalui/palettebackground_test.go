package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The layer the terminal surface paints behind its content is the palette's own
// background — and the window's own for `inherit` and for an id this build does
// not know, which is the renderer's own fallback too.
//
// The alpha is the terminal slider's **less** the share the card's face already
// carries (glassmap.Spec.CanvasAlpha): the panel is one sheet, so 46% reads as
// 46% instead of the `1-(1-0.46)^2` = 71% two full tints composited to. The
// slider's own rule still holds underneath (the solid top, the contrast floor),
// because the spec is where those live now.
func TestThePanelFillFollowsThePalette(t *testing.T) {
	dark := theme.For(settings.Default(), true)
	spec := glassmap.SpecFor("regular", settings.DefaultTerminalOpacity)
	cases := []struct {
		name    string
		running bool
		opacity uint8
		want    ui.Color
	}{
		// A named palette paints its own colour, at the canvas's share of the
		// slider.
		{"forest", true, 46, ui.RGB(0x0f, 0x1a, 0x14).Alpha(spec.CanvasAlpha())},
		// `inherit` and an unknown id paint the window's own background, so an
		// inherit session looks exactly as it did.
		{"inherit", true, 46, dark.Theme.Background.Alpha(spec.CanvasAlpha())},
		{"not-a-palette", true, 46, dark.Theme.Background.Alpha(spec.CanvasAlpha())},
		// The empty page keeps the window's colour even with a light palette
		// selected: there is no session whose colour it would be, and the page's
		// own ink has to stay readable on it.
		{"paper", false, 46, dark.Theme.Background.Alpha(spec.CanvasAlpha())},
	}
	for _, tc := range cases {
		got := panelFill(tc.name, tc.running, glassmap.SpecFor("regular", tc.opacity), dark)
		if got != tc.want {
			t.Errorf("panelFill(%q, running=%v, %d%%) = %v, want %v", tc.name, tc.running, tc.opacity, got, tc.want)
		}
	}
	// The fill rides the slider: a thinner slider is a thinner panel, and the
	// top is the near-solid one rather than paint.
	thin := panelFill("forest", true, glassmap.SpecFor("regular", settings.MinWindowOpacity), dark)
	thick := panelFill("forest", true, glassmap.SpecFor("regular", settings.MaxWindowOpacity), dark)
	if thin.A >= thick.A {
		t.Errorf("the panel did not rise with the slider: %d vs %d", thin.A, thick.A)
	}
	if thick.A >= 255 {
		t.Errorf("the panel's top = %d, want the near-solid top rather than paint", thick.A)
	}
	// A desktop that asked for more contrast lifts the low end.
	high := panelFill("forest", true, glassmap.SpecForPreferences("regular", settings.MinWindowOpacity, true), dark)
	if high.A <= thin.A {
		t.Errorf("the contrast floor did not reach the panel: %d vs %d", high.A, thin.A)
	}
}

// A session never paints a background of its own, whatever palette the settings
// held when it started: `Transparent` is a creation-time option with no setter,
// so a session that read the palette there stayed wrong for the rest of its
// life - an `inherit` session never showed a palette picked later, and a
// palette session kept painting an opaque one after the user went back to
// `inherit`, which is the very thing the slider is for. The layer the palette
// colours is the view's own (canvasFill), so the palette reaches a running
// session on the next frame; the reference met the same two cases by rebuilding
// its renderer on every mount.
//
// The palette still reaches the plugin as a theme, whose colours - text,
// cursor, selection - are the ones a running session repaints with. Its
// background is opaque: the transparency is the view's layer (canvasFill), not
// the theme's.
func TestASessionNeverPaintsItsOwnBackground(t *testing.T) {
	newTerminal, options := captureTerminal(t)
	store := newStore()
	a := New(store, Actions{}, newTerminal)

	// The stored default is `inherit`.
	a.EnsureSession()
	if err := store.Update(func(s *settings.Settings) {
		s.TerminalTheme = "forest"
		s.TerminalOpacity = 80
	}); err != nil {
		t.Fatal(err)
	}
	a.NewSession()

	if len(*options) != 2 {
		t.Fatalf("sessions = %d, want 2", len(*options))
	}
	for i, o := range *options {
		if !o.Transparent {
			t.Errorf("session %d paints its own background", i)
		}
	}
	if o := (*options)[0]; o.Theme == nil || o.DarkTheme == nil {
		t.Errorf("the inherit session has no theme pair: %+v", o.Theme)
	} else if want := ui.RGB(0x8b, 0xd5, 0xca); o.DarkTheme.Cursor != want {
		t.Errorf("the inherit session's dark cursor = %v, want %v", o.DarkTheme.Cursor, want)
	}
	got := (*options)[1].Theme
	if got == nil {
		t.Fatal("the forest palette did not build")
	}
	if want := ui.RGB(0x0f, 0x1a, 0x14); got.Background != want {
		t.Errorf("forest background = %v, want %v", got.Background, want)
	}
	// A named palette is one fixed colour set, so it carries no dark half:
	// the plugin applies the same theme in either window appearance, as the
	// reference's canvas palettes did.
	if (*options)[1].DarkTheme != nil {
		t.Errorf("the forest palette carried a dark half: %+v", (*options)[1].DarkTheme)
	}
}

// The shipped `inherit` palette follows the window, and the one token it does
// not take from the plugin's defaults is the cursor: base.css reads
// `--terminal-cursor` (a teal), where a theme that leaves the cursor zero gets
// the foreground. An inherit session therefore arrives as the plugin's
// light/dark pair with only that token overridden - not as no theme at all,
// which is how it used to be handed over and why the cursor was the wrong
// colour.
func TestTheInheritCursorUsesTheShippedToken(t *testing.T) {
	light, dark := themeFor("inherit")
	if light == nil || dark == nil {
		t.Fatalf("inherit = (%v, %v), want a light/dark pair", light, dark)
	}
	if got, want := light.Cursor, ui.RGB(0x00, 0x7c, 0x86); got != want {
		t.Errorf("light inherit cursor = %v, want %v", got, want)
	}
	if got, want := dark.Cursor, ui.RGB(0x8b, 0xd5, 0xca); got != want {
		t.Errorf("dark inherit cursor = %v, want %v", got, want)
	}
	// The cursor is the *only* override: the text and the background stay the
	// plugin's own defaults, so `inherit` keeps meaning "follow the window".
	if light.Foreground != terminal.LightTheme().Foreground || light.Background != terminal.LightTheme().Background {
		t.Errorf("the light inherit theme moved the defaults: %+v", light)
	}
	if dark.Foreground != terminal.DarkTheme().Foreground || dark.Background != terminal.DarkTheme().Background {
		t.Errorf("the dark inherit theme moved the defaults: %+v", dark)
	}

	// The settings preview reads the same token, so the swatch and the live
	// terminal cannot disagree about the cursor colour.
	for _, isDark := range []bool{false, true} {
		colors, ok := PaletteColorsFor("inherit", isDark)
		if !ok {
			t.Fatalf("inherit preview missing for dark=%v", isDark)
		}
		want := inheritCursor(isDark)
		if colors.Cursor != want {
			t.Errorf("inherit preview cursor (dark=%v) = %v, want %v", isDark, colors.Cursor, want)
		}
	}

	// A named palette is still one fixed theme with its own cursor, and no
	// dark half.
	theme, pairDark := themeFor("forest")
	if theme == nil || pairDark != nil {
		t.Fatalf("forest = (%v, %v), want one fixed theme", theme, pairDark)
	}
	if got, want := theme.Cursor, ui.RGB(0x8b, 0xd4, 0x50); got != want {
		t.Errorf("forest cursor = %v, want %v", got, want)
	}
}
