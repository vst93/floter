package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		theme      string
		systemDark bool
		want       Appearance
	}{
		{"auto", false, Light},
		{"auto", true, Dark},
		{"light", false, Light},
		{"light", true, Light}, // explicit wins over the desktop
		{"dark", false, Dark},
		{"dark", true, Dark},
		{"", false, Dark}, // App.tsx paints anything else dark
		{"blue", true, Dark},
	}
	for _, tc := range cases {
		if got := Resolve(tc.theme, tc.systemDark); got != tc.want {
			t.Errorf("Resolve(%q, %v) = %q, want %q", tc.theme, tc.systemDark, got, tc.want)
		}
	}
}

func TestForScalesByTheInterfaceStep(t *testing.T) {
	base := settings.Normalize(settings.Settings{
		Theme:           "light",
		GlassStep:       settings.DefaultGlassStep,
		Language:        settings.DefaultLanguage,
		MainOpacity:     settings.DefaultMainOpacity,
		TerminalOpacity: settings.DefaultTerminalOpacity,
		UIScale:         "default",
	})
	one := For(base, false)

	if one.Appearance != Light {
		t.Errorf("appearance = %q, want light", one.Appearance)
	}
	if one.Scale != 1 {
		t.Errorf("scale = %v, want 1", one.Scale)
	}
	if one.Theme.Dark {
		t.Errorf("theme is dark, want light")
	}

	small := For(settings.Settings{Theme: "auto", UIScale: "small"}, false)
	if small.Scale != 0.9 {
		t.Errorf("small scale = %v, want 0.9", small.Scale)
	}
	if got, want := small.Theme.FontSize, round1(one.Theme.FontSize*0.9); got != want {
		t.Errorf("small font = %v, want %v", got, want)
	}
	if got, want := small.RadiusMD, round1(RadiusMD*0.9); got != want {
		t.Errorf("small radius md = %v, want %v", got, want)
	}

	large := For(settings.Settings{Theme: "dark", UIScale: "large"}, false)
	if large.Appearance != Dark {
		t.Errorf("large appearance = %q, want dark", large.Appearance)
	}
	if got, want := large.RadiusLG, round1(RadiusLG*1.1); got != want {
		t.Errorf("large radius lg = %v, want %v", got, want)
	}
	// An unknown step lands on the shipped default's factor, never zero.
	if got := For(settings.Settings{Theme: "auto", UIScale: "huge"}, false).Scale; got != 0.9 {
		t.Errorf("unknown step scale = %v, want the shipped 0.9", got)
	}
}

func TestForDoesNotMutateTheSharedTheme(t *testing.T) {
	// Hold the value the framework handed out, then check For scaled only
	// its own copy of it.
	shared := ui.LightTheme()
	_ = For(settings.Settings{Theme: "light", UIScale: "large"}, false)
	if *shared != *ui.LightTheme() {
		t.Errorf("For mutated the shared light theme: %+v", *shared)
	}
}

// The strokes are the two appearances' edge colors, per appearance, as
// base.css wrote them: a lit hairline in the dark, a dark one in the light.
func TestStrokes(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	if dark.Appearance != Dark {
		t.Fatalf("appearance = %v", dark.Appearance)
	}
	if dark.InputStroke.A >= 255 {
		t.Errorf("dark stroke = %v, want translucent", dark.InputStroke)
	}
	light := For(settings.Settings{Theme: "light"}, false)
	if light.Appearance != Light {
		t.Fatalf("appearance = %v", light.Appearance)
	}
	if light.InputStroke.R != 0 {
		t.Errorf("light stroke = %v, want the dark-on-light one", light.InputStroke)
	}
	// The focused step takes the accent in both.
	if dark.InputStrokeActive == dark.InputStroke {
		t.Error("the active stroke is the idle one")
	}
}

// The settings content pane carries its own fill, and it rides the
// transparency slider as a single sheet: the pane is painted at the alpha that
// composites to the frame tint plus a small recess (glassmap.Spec.ContentAlpha),
// so the visible bottom is the frame the slider moves, not a second full tint
// stacked on it. The reference pinned `--surface-sunken` to a fixed 0.62 + 0.18
// band instead, which stayed ~0.64-0.80 no matter where the slider was and hid
// the frame tint it sat on - the user read that as "the bottom is opaque and the
// slider does nothing".
func TestSunkenFollowsTheOpacity(t *testing.T) {
	dim := settings.Settings{Theme: "dark", MainOpacity: 10}
	bright := settings.Settings{Theme: "dark", MainOpacity: 100}
	lowT := For(dim, false)
	highT := For(bright, false)
	if lowT.Sunken.A >= highT.Sunken.A {
		t.Errorf("the sunken fill did not rise with the slider: %v vs %v", lowT.Sunken.A, highT.Sunken.A)
	}
	// The pane is a recess over the frame, not a second sheet: at the low stop
	// its own alpha is small, because the frame underneath already carries the
	// tint the slider owns.
	if a := float64(lowT.Sunken.A) / 255; a > 0.55 {
		t.Errorf("the low stop alpha = %v, want a thin recess", a)
	}
	// At the top it reaches the near-solid face the frame itself owns.
	if a := float64(highT.Sunken.A) / 255; a < 0.97 {
		t.Errorf("the high stop alpha = %v, want the frame tint's ~0.98", a)
	}
	// A light appearance sunken fill is the light pane's own colour.
	lightT := For(settings.Settings{Theme: "light", MainOpacity: 100}, false)
	if a, b := float64(lightT.Sunken.A)/255, float64(highT.Sunken.A)/255; a != b {
		t.Errorf("the two appearances' alphas differ: %v vs %v", a, b)
	}
}

// The ladder scales once, by the step: the display register at the Large step
// is 17 points times 1.1, not "body plus six points" - the offset that grew
// with every step above the shipped one.
func TestLadderScalesWithTheStep(t *testing.T) {
	large := For(settings.Settings{Theme: "dark", UIScale: "large"}, false)
	if large.Display < 18.6 || large.Display > 18.8 {
		t.Errorf("display at Large = %v, want 17 x 1.1", large.Display)
	}
	if large.Title < 14.2 || large.Title > 14.4 {
		t.Errorf("title at Large = %v, want 13 x 1.1", large.Title)
	}
	if large.Body < 12.0 || large.Body > 12.2 {
		t.Errorf("body at Large = %v, want 11 x 1.1", large.Body)
	}
	// The shipped step: 0.9.
	small := For(settings.Settings{Theme: "dark"}, false)
	if small.Display < 15.2 || small.Display > 15.4 {
		t.Errorf("display at Small = %v, want 17 x 0.9", small.Display)
	}
}

// The palette is base.css's, not the framework's stock one: the pastel
// accent, the translucent control ladder and the near-black glass. These are
// the tokens the whole app is drawn against, so a regression here repaints
// every surface at once.
func TestPaletteComesFromBaseCSS(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	if dark.Theme.Accent != ui.Hex("#8fb7ff") {
		t.Errorf("dark accent = %v, want the pastel #8fb7ff", dark.Theme.Accent)
	}
	if dark.Theme.Background != ui.Hex("#121316") {
		t.Errorf("dark background = %v, want the glass tint #121316", dark.Theme.Background)
	}
	if dark.Theme.Text != ui.Hex("#f2f3f5") {
		t.Errorf("dark text = %v, want --text-primary", dark.Theme.Text)
	}
	// The control ladder is translucent white, not an opaque grey: the
	// framework's stock SurfaceHover would paint a solid block over the glass.
	if dark.Theme.SurfaceHover.A >= 255 {
		t.Errorf("dark hover = %v, want a translucent lift", dark.Theme.SurfaceHover)
	}
	if dark.Theme.AccentText != ui.Hex("#15171b") {
		t.Errorf("dark accent text = %v, want --accent-contrast", dark.Theme.AccentText)
	}

	light := For(settings.Settings{Theme: "light"}, false)
	if light.Theme.Accent != ui.Hex("#064aa6") {
		t.Errorf("light accent = %v, want the darker #064aa6", light.Theme.Accent)
	}
	if light.Theme.Background != ui.Hex("#fafafc") {
		t.Errorf("light background = %v, want #fafafc", light.Theme.Background)
	}
	if light.Theme.Text != ui.Hex("#1d1d1f") {
		t.Errorf("light text = %v, want --text-primary", light.Theme.Text)
	}
}

// The sidebar's chosen page is the opaque raised rung: the framework paints
// the accent behind a chosen list row, so a translucent lift would read as a
// blue pill, which is exactly what the reference's quiet pill is not.
func TestRaisedQuietIsOpaqueAndNeutral(t *testing.T) {
	for _, tc := range []struct {
		name string
		t    Tokens
	}{
		{"dark", For(settings.Settings{Theme: "dark"}, true)},
		{"light", For(settings.Settings{Theme: "light"}, false)},
	} {
		if tc.t.RaisedQuiet.A != 255 {
			t.Errorf("%s raised-quiet alpha = %d, want opaque", tc.name, tc.t.RaisedQuiet.A)
		}
		// A neutral lift over a near-neutral pane keeps R, G and B within a
		// few steps of each other; an accent-tinted pill would not.
		r, g, b := int(tc.t.RaisedQuiet.R), int(tc.t.RaisedQuiet.G), int(tc.t.RaisedQuiet.B)
		if spread := max(r, g, b) - min(r, g, b); spread > 12 {
			t.Errorf("%s raised-quiet = %v, want a neutral lift", tc.name, tc.t.RaisedQuiet)
		}
	}
}

// The control lens is three inset layers plus the pane edge: a control that
// carries it reads as a lens rather than as a flat tint with a border.
func TestLensTokensAreTranslucent(t *testing.T) {
	dark := For(settings.Settings{Theme: "dark"}, true)
	if dark.LensRim.A == 0 || dark.LensRim.A >= 255 {
		t.Errorf("lens rim = %v, want a translucent highlight", dark.LensRim)
	}
	if dark.LensEdge.R != 0 {
		t.Errorf("lens edge = %v, want a dark bottom shade", dark.LensEdge)
	}
}

// The glass stop scales the control lens (base.css's `--glass-lens-scale`:
// 0.5 / 1 / 1.6). It is the one rung of the old build's effect table the
// scene can paint on every platform, so it is what makes `frosted`, `regular`
// and `liquid` read apart even where the window has no material to blur the
// desktop with.
func TestLensScalesWithTheGlassStop(t *testing.T) {
	base := settings.Settings{Theme: "dark"}
	frosted := For(withStep(base, "frosted"), true)
	regular := For(withStep(base, "regular"), true)
	liquid := For(withStep(base, "liquid"), true)
	if !(frosted.LensRim.A < regular.LensRim.A && regular.LensRim.A < liquid.LensRim.A) {
		t.Errorf("lens rim did not rise with the stop: %d / %d / %d", frosted.LensRim.A, regular.LensRim.A, liquid.LensRim.A)
	}
	if !(frosted.LensSheen.A < regular.LensSheen.A && regular.LensSheen.A < liquid.LensSheen.A) {
		t.Errorf("lens sheen did not rise with the stop: %d / %d / %d", frosted.LensSheen.A, regular.LensSheen.A, liquid.LensSheen.A)
	}
	// The regular stop is the reference's own lens strength: the palette's
	// base value unchanged.
	palette := paletteFor(Dark)
	if regular.LensRim.A != palette.lensRim.A {
		t.Errorf("regular lens rim = %d, want the palette's %d", regular.LensRim.A, palette.lensRim.A)
	}
	// The off stop drops the lens entirely.
	if off := For(withStep(base, "off"), true); off.LensRim.A != 0 || off.LensSheen.A != 0 || off.LensEdge.A != 0 {
		t.Errorf("off kept a lens: %v / %v / %v", off.LensRim, off.LensSheen, off.LensEdge)
	}
}

// withStep is s with its glass stop set, for the lens tests.
func withStep(s settings.Settings, step string) settings.Settings {
	s.GlassStep = step
	return s
}

// A desktop that asked for more contrast gets base.css's
// `@media (prefers-contrast: more)` tokens: the card stroke, the grouped
// row's inner edge and the field placeholder are swapped per appearance. The
// rest of the palette is untouched, which is what makes the preference a
// change of *contrast* rather than a second look.
func TestHighContrastSwapsTheStrokeAndPlaceholderTokens(t *testing.T) {
	cases := []struct {
		name        string
		systemDark  bool
		stroke      ui.Color
		soft        ui.Color
		placeholder ui.Color
	}{
		{
			name: "light", systemDark: false,
			stroke:      ui.RGBA(0, 0, 0, 0.5),
			soft:        ui.RGBA(0, 0, 0, 0.36),
			placeholder: ui.RGBA(28, 28, 32, 0.68),
		},
		{
			name: "dark", systemDark: true,
			stroke:      ui.RGBA(255, 255, 255, 0.42),
			soft:        ui.RGBA(255, 255, 255, 0.34),
			placeholder: ui.RGBA(242, 243, 245, 0.58),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := settings.Settings{Theme: "auto", GlassStep: "regular"}
			plain := ForPreferences(s, tc.systemDark, ui.Preferences{})
			high := ForPreferences(s, tc.systemDark, ui.Preferences{HighContrast: true})

			if high.InputStroke != tc.stroke {
				t.Errorf("InputStroke = %v, want %v", high.InputStroke, tc.stroke)
			}
			if high.ControlEdge != tc.soft {
				t.Errorf("ControlEdge = %v, want %v", high.ControlEdge, tc.soft)
			}
			if high.Theme.TextMuted != tc.placeholder {
				t.Errorf("theme TextMuted = %v, want the placeholder contrast %v", high.Theme.TextMuted, tc.placeholder)
			}
			// The framework theme the widgets read is the same palette: its
			// Border is the card's stroke, so a button or a field the views
			// do not paint by hand gets the stronger edge too.
			if high.Theme.Border != tc.stroke {
				t.Errorf("theme Border = %v, want the contrast stroke %v", high.Theme.Border, tc.stroke)
			}
			// The preference has to change something, or it is an inert knob.
			if high.InputStroke == plain.InputStroke && high.ControlEdge == plain.ControlEdge && high.Theme.TextMuted == plain.Theme.TextMuted {
				t.Error("more contrast changed no token")
			}

			// Nothing else moved: blank the three contrast tokens, the
			// pointer and the three fields the material floor moves, and the
			// two resolutions have to be the same value.
			blank := func(tk Tokens) Tokens {
				tk.Theme = nil
				tk.InputStroke, tk.ControlEdge = ui.Color{}, ui.Color{}
				// The three the material floor and the lens damping move.
				tk.Sunken, tk.RaisedQuiet, tk.Float = ui.Color{}, ui.Color{}, ui.Color{}
				tk.LensRim, tk.LensSheen, tk.LensEdge = ui.Color{}, ui.Color{}, ui.Color{}
				tk.SheenTop, tk.SheenBottom = ui.Color{}, ui.Color{}
				return tk
			}
			if got, want := blank(high), blank(plain); got != want {
				t.Errorf("contrast moved tokens besides the three:\n got %+v\nwant %+v", got, want)
			}
			// The material floor rides through: the content pane and a
			// floating pane are painted at the raised frame's own alpha.
			if high.Sunken.A <= plain.Sunken.A {
				t.Errorf("the content pane did not follow the raised floor: %d vs %d", high.Sunken.A, plain.Sunken.A)
			}
			if high.Float.A <= plain.Float.A {
				t.Errorf("a floating pane did not follow the raised floor: %d vs %d", high.Float.A, plain.Float.A)
			}
		})
	}
}
