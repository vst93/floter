package launcher

import (
	"testing"

	"github.com/egoist/mygo/plugins/glass"

	"floter/internal/settings"
)

// TestSurfaceCrossProduct sweeps the P0 guardrail matrix: the four glass
// stops × the two languages × the three theme values, each against a light
// and a dark desktop. It pins the empty-state hint, the resolved appearance
// and the glass parameters the view will use.
func TestSurfaceCrossProduct(t *testing.T) {
	hints := map[string]string{
		"en": "Type to search",
		"zh": "输入以搜索",
	}
	steps := []string{"off", "frosted", "regular", "liquid"}
	languages := []string{"en", "zh"}
	themes := []string{"auto", "light", "dark"}

	// The effect ladder the plugin can carry, per stop: material and haze.
	type ladder struct {
		style glass.Style
		haze  float32
	}
	ladders := map[string]ladder{
		"off":     {glass.Clear, 0},
		"frosted": {glass.Clear, 0.6},
		"regular": {glass.Regular, 0.5},
		"liquid":  {glass.Regular, 0.2},
	}

	for _, step := range steps {
		for _, language := range languages {
			for _, theme := range themes {
				for _, systemDark := range []bool{false, true} {
					s := settings.Normalize(settings.Settings{
						Theme:           theme,
						GlassStep:       step,
						Language:        language,
						MainOpacity:     settings.DefaultMainOpacity,
						TerminalOpacity: settings.DefaultTerminalOpacity,
						UIScale:         settings.DefaultUIScale,
					})
					got := Resolve(s, systemDark)

					label := "step=" + step + " lang=" + language + " theme=" + theme
					if systemDark {
						label += " system=dark"
					} else {
						label += " system=light"
					}

					if got.Strings.Hint != hints[language] {
						t.Errorf("%s: Hint = %q, want %q", label, got.Strings.Hint, hints[language])
					}

					wantTheme := ThemeDark
					switch theme {
					case "auto":
						if !systemDark {
							wantTheme = ThemeLight
						}
					case "light":
						wantTheme = ThemeLight
					}
					if got.Theme != wantTheme {
						t.Errorf("%s: Theme = %q, want %q", label, got.Theme, wantTheme)
					}

					wantEnabled := step != "off"
					if got.Glass.Enabled != wantEnabled {
						t.Errorf("%s: Glass.Enabled = %v, want %v", label, got.Glass.Enabled, wantEnabled)
					}
					want := ladders[step]
					if got.Glass.Style != want.style {
						t.Errorf("%s: Glass.Style = %v, want %v", label, got.Glass.Style, want.style)
					}
					if got.Glass.HazeAlpha != want.haze {
						t.Errorf("%s: Glass.HazeAlpha = %v, want %v", label, got.Glass.HazeAlpha, want.haze)
					}
				}
			}
		}
	}
}

func TestStringsForUnknownLanguageIsEnglish(t *testing.T) {
	// The copy struct holds function fields (a sentence with a value in it),
	// so it is compared field by field rather than whole.
	if got, want := StringsFor("fr").Placeholder, StringsFor("en").Placeholder; got != want {
		t.Errorf("StringsFor(fr) placeholder = %q, want %q", got, want)
	}
	if got, want := StringsFor("fr").CommandQuit, StringsFor("en").CommandQuit; got != want {
		t.Errorf("StringsFor(fr) quit = %q, want %q", got, want)
	}
}

func TestResolveTheme(t *testing.T) {
	cases := []struct {
		theme      string
		systemDark bool
		want       ResolvedTheme
	}{
		{"auto", false, ThemeLight},
		{"auto", true, ThemeDark},
		{"light", false, ThemeLight},
		{"light", true, ThemeLight}, // explicit wins over the desktop
		{"dark", false, ThemeDark},
		{"dark", true, ThemeDark},
		// Only reachable without the loader's normalization; App.tsx paints
		// anything that is not auto/light dark.
		{"", false, ThemeDark},
		{"blue", true, ThemeDark},
	}
	for _, tc := range cases {
		if got := ResolveTheme(tc.theme, tc.systemDark); got != tc.want {
			t.Errorf("ResolveTheme(%q, %v) = %q, want %q", tc.theme, tc.systemDark, got, tc.want)
		}
	}
}

func TestWindowHeightUsesTheOldInputHeightAndScales(t *testing.T) {
	base := InputWindowHeight() + ResultsAreaHeight
	if got := WindowHeight("default"); got != base {
		t.Errorf("WindowHeight(default) = %v, want %v", got, base)
	}
	if got, want := WindowHeight("small"), InputWindowHeight()*0.9+ResultsAreaHeight; got != want {
		t.Errorf("WindowHeight(small) = %v, want %v", got, want)
	}
	if got, want := WindowHeight("large"), InputWindowHeight()*1.1+ResultsAreaHeight; got != want {
		t.Errorf("WindowHeight(large) = %v, want %v", got, want)
	}
	// An unknown step lands on the shipped default, never zero.
	if got := WindowHeight("huge"); got != InputWindowHeight()*0.9+ResultsAreaHeight {
		t.Errorf("WindowHeight(huge) = %v, want the small step", got)
	}
	if InputWindowWidth != 720 {
		t.Errorf("InputWindowWidth = %v, want 720", InputWindowWidth)
	}
}
