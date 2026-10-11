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

// The settings content pane carries its own fill: 0.62 + 0.18 of the main
// opacity, as `--glass-content-alpha` had it - more solid than the launcher,
// rising with the transparency slider without ever reaching it.
func TestSunkenFollowsTheOpacity(t *testing.T) {
	dim := settings.Settings{Theme: "dark", MainOpacity: 10}
	bright := settings.Settings{Theme: "dark", MainOpacity: 100}
	lowT := For(dim, false)
	highT := For(bright, false)
	if lowT.Sunken.A >= highT.Sunken.A {
		t.Errorf("the sunken fill did not rise with the slider: %v vs %v", lowT.Sunken.A, highT.Sunken.A)
	}
	// The formula's own floor: at 10% the pane is 0.638, not transparent.
	if a := float64(lowT.Sunken.A) / 255; a < 0.6 || a > 0.7 {
		t.Errorf("the low stop alpha = %v", a)
	}
	// A light appearance sunken fill is the light pane's own colour.
	lightT := For(settings.Settings{Theme: "light", MainOpacity: 100}, false)
	if a, b := float64(lightT.Sunken.A)/255, float64(highT.Sunken.A)/255; a != b {
		t.Errorf("the two appearances' alphas differ: %v vs %v", a, b)
	}
}
