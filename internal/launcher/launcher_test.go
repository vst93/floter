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
					wantStyle := glass.Regular
					if step == "off" || step == "frosted" {
						wantStyle = glass.Clear
					}
					if got.Glass.Style != wantStyle {
						t.Errorf("%s: Glass.Style = %v, want %v", label, got.Glass.Style, wantStyle)
					}
				}
			}
		}
	}
}

func TestStringsForUnknownLanguageIsEnglish(t *testing.T) {
	if got := StringsFor("fr"); got != StringsFor("en") {
		t.Errorf("StringsFor(fr) = %+v, want the English copy", got)
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
