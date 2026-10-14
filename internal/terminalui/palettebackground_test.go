package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/settings"
)

// The layer the terminal surface paints behind the canvas is the palette's own
// background at the terminal slider's alpha - and nothing at all for `inherit`
// and for an id this build does not know, which is the renderer's own fallback
// too.
//
// The alpha is the *frame's* rule, not the slider read straight: the old
// build's canvas carried base.css's
// `min(--glass-solid-top, max(--glass-frame-floor, --terminal-opacity))`, so
// the canvas agrees with the veil glassmap paints under it, a fully opaque
// canvas cannot drop the material's sheen into invisibility, and a desktop that
// asked for more contrast lifts the low end.
func TestCanvasFillFollowsThePalette(t *testing.T) {
	cases := []struct {
		name         string
		opacity      uint8
		highContrast bool
		want         ui.Color
	}{
		{"inherit", 46, false, ui.Color{}},
		{"not-a-palette", 46, false, ui.Color{}},
		{"forest", 46, false, ui.RGB(0x0f, 0x1a, 0x14).Alpha(0.46)},
		{"forest", 10, false, ui.RGB(0x0f, 0x1a, 0x14).Alpha(0.1)},
		// The slider's top is 0.98: an opaque canvas would read as paint.
		{"forest", 100, false, ui.RGB(0x0f, 0x1a, 0x14).Alpha(glassmap.SolidTop)},
		// A desktop that asked for more contrast lifts the low end.
		{"forest", 46, true, ui.RGB(0x0f, 0x1a, 0x14).Alpha(glassmap.ContrastFrameFloor)},
		{"forest", 100, true, ui.RGB(0x0f, 0x1a, 0x14).Alpha(glassmap.SolidTop)},
		// The contrast floor never raises the near-solid top.
		{"paper", 0, true, ui.RGB(0xf4, 0xef, 0xe4).Alpha(glassmap.ContrastFrameFloor)},
	}
	for _, tc := range cases {
		if got := canvasFill(tc.name, tc.opacity, tc.highContrast); got != tc.want {
			t.Errorf("canvasFill(%q, %d, %v) = %v, want %v", tc.name, tc.opacity, tc.highContrast, got, tc.want)
		}
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
