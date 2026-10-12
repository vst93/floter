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
// transparency slider: the pane is a second sheet of the frame's own tint, so
// it composites denser than the frame (the reference's readability contract)
// while the slider still moves it. The reference pinned `--surface-sunken` to
// a fixed 0.62 + 0.18 band instead, which stayed ~0.64-0.80 no matter where
// the slider was and hid the frame tint it sat on - the user read that as
// "the bottom is opaque and the slider does nothing".
func TestSunkenFollowsTheOpacity(t *testing.T) {
	dim := settings.Settings{Theme: "dark", MainOpacity: 10}
	bright := settings.Settings{Theme: "dark", MainOpacity: 100}
	lowT := For(dim, false)
	highT := For(bright, false)
	if lowT.Sunken.A >= highT.Sunken.A {
		t.Errorf("the sunken fill did not rise with the slider: %v vs %v", lowT.Sunken.A, highT.Sunken.A)
	}
	// It rides the frame tint, so the low stop is the frame's own 0.505, not
	// the fixed band's 0.638 that hid it.
	if a := float64(lowT.Sunken.A) / 255; a > 0.55 {
		t.Errorf("the low stop alpha = %v, want the frame tint's ~0.505", a)
	}
	// And it reaches the near-solid top the slider owns, not a second ceiling.
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
