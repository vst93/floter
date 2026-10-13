package launcher

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// Strings is the launcher's copy for one language; the shared catalog's
// launcher section.
type Strings = i18n.Launcher

// ResolvedTheme is the concrete appearance the launcher paints with.
type ResolvedTheme = theme.Appearance

const (
	// ThemeLight and ThemeDark are the two resolved appearances.
	ThemeLight = theme.Light
	ThemeDark  = theme.Dark
)

// Surface is everything the view needs, resolved from the settings and the
// desktop's appearance. It is a plain value so the cross product the
// guardrails care about — four glass stops × two languages × three themes —
// is testable without a window.
type Surface struct {
	Settings settings.Settings
	Strings  Strings
	// Theme is the appearance after resolving settings.Theme against the
	// system: an explicit light/dark wins, `auto` follows the desktop.
	Theme ResolvedTheme
	// Tokens is the framework theme at the user's interface size.
	Tokens theme.Tokens
	// Glass is the plugin parameters for the stored step and opacity.
	Glass glassmap.Spec
}

// ResolveTheme resolves the stored theme against the desktop's appearance.
// It mirrors App.tsx's rule exactly: `auto` follows the system, an explicit
// `light` or `dark` wins, and anything else (only reachable without the
// loader's normalization) paints dark.
func ResolveTheme(name string, systemDark bool) ResolvedTheme {
	return theme.Resolve(name, systemDark)
}

// StringsFor returns the launcher's copy for a language, normalized the same
// way the settings loader does. An unknown language falls back to English.
func StringsFor(language string) Strings {
	return i18n.For(language).Launcher
}

// Resolve builds the surface for a settings value, the desktop's current
// appearance and the desktop's preferences. The settings are normalized
// first, so the caller may pass a value straight from Parse or a hand-built
// one. The preferences reach the look in two places, both of them base.css's
// `prefers-contrast: more` block: the stronger stroke tokens in the theme and
// the dialled-back material in the glass spec.
func Resolve(s settings.Settings, systemDark bool, prefs ui.Preferences) Surface {
	s = settings.Normalize(s)
	return Surface{
		Settings: s,
		Strings:  StringsFor(s.Language),
		Theme:    ResolveTheme(s.Theme, systemDark),
		Tokens:   theme.ForPreferences(s, systemDark, prefs),
		Glass:    glassmap.SpecForPreferences(s.GlassStep, s.MainOpacity, prefs.HighContrast),
	}
}
