package launcher

import (
	"floter/internal/glassmap"
	"floter/internal/settings"
)

// ResolvedTheme is the concrete appearance the launcher paints with.
type ResolvedTheme string

const (
	ThemeLight ResolvedTheme = "light"
	ThemeDark  ResolvedTheme = "dark"
)

// Surface is everything the view needs, resolved from the settings and the
// desktop's appearance. It is a plain value so the cross product the P0
// guardrails care about — four glass stops × two languages × three themes —
// is testable without a window.
type Surface struct {
	Settings settings.Settings
	Strings  Strings
	// Theme is the appearance after resolving settings.Theme against the
	// system: an explicit light/dark wins, `auto` follows the desktop.
	Theme ResolvedTheme
	// Glass is the plugin parameters for the stored step and opacity.
	Glass glassmap.Spec
}

// ResolveTheme resolves the stored theme against the desktop's appearance.
// It mirrors App.tsx's rule exactly: `auto` follows the system, an explicit
// `light` or `dark` wins, and anything else (only reachable without the
// loader's normalization) paints dark.
func ResolveTheme(theme string, systemDark bool) ResolvedTheme {
	switch theme {
	case "auto":
		if systemDark {
			return ThemeDark
		}
		return ThemeLight
	case "light":
		return ThemeLight
	default:
		return ThemeDark
	}
}

// Resolve builds the surface for a settings value and the desktop's current
// appearance. The settings are normalized first, so the caller may pass a
// value straight from Parse or a hand-built one.
func Resolve(s settings.Settings, systemDark bool) Surface {
	s = settings.Normalize(s)
	return Surface{
		Settings: s,
		Strings:  StringsFor(s.Language),
		Theme:    ResolveTheme(s.Theme, systemDark),
		Glass:    glassmap.SpecFor(s.GlassStep, s.MainOpacity),
	}
}
