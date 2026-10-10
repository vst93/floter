// Package theme turns floter's stored appearance settings into the concrete
// theme the views paint with.
//
// The old build had a token layer of its own: CSS custom properties in
// src/styles/base.css (surfaces, borders, radii, the glass tint) resolved
// from `theme` and scaled by `ui_scale`. This package is that layer's Go
// equivalent, a subset: the two appearance values the app ships, the radius
// scale, and the interface-size factor. The rest of the palette comes from
// the framework's light and dark themes, which already follow the desktop's
// accent and contrast preferences.
package theme

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// Appearance is the concrete look a surface paints with, after resolving the
// stored `theme` against the desktop.
type Appearance string

const (
	// Light and Dark are the two appearances the app paints.
	Light Appearance = "light"
	Dark  Appearance = "dark"
)

// Resolve resolves the stored theme against the desktop's appearance. It
// mirrors App.tsx's rule exactly: `auto` follows the system, an explicit
// `light` or `dark` wins, and anything else (only reachable without the
// loader's normalization) paints dark.
func Resolve(theme string, systemDark bool) Appearance {
	switch theme {
	case "auto":
		if systemDark {
			return Dark
		}
		return Light
	case "light":
		return Light
	default:
		return Dark
	}
}

// The radius scale from base.css, at scale 1: an inner chip, a control on a
// surface, a card holding controls, and the window or a full panel.
const (
	RadiusXS = 6
	RadiusSM = 9
	RadiusMD = 12
	RadiusLG = 14
)

// Tokens is the resolved theme: the appearance, the framework theme the
// widgets read, and the radius scale, all at the user's interface size.
type Tokens struct {
	// Appearance is the resolved light or dark look.
	Appearance Appearance
	// Theme is the theme to set on the view's context.
	Theme *ui.Theme
	// Scale is the interface-size factor the tokens were scaled by
	// (0.8/0.9/1.0/1.1; the shipped "small" step is 0.9).
	Scale float64

	RadiusXS float32
	RadiusSM float32
	RadiusMD float32
	RadiusLG float32

	// InputStroke is the one edge a card or a control draws (base.css's
	// `--input-stroke`), and InputStrokeActive its focused step — the lit
	// edge a focused card takes, as `.collapsed-card:focus-within` did.
	InputStroke       ui.Color
	InputStrokeActive ui.Color

	// Control is the resting plane a grouped card or a control sits on
	// (base.css's `--glass-control`), ControlEdge its hairline edge, and
	// Hairline the rule a row draws between itself and the next. The
	// settings panel's card language is these three plus TextStrong.
	Control     ui.Color
	ControlEdge ui.Color
	Hairline    ui.Color
	// TextStrong is a row's own label color, one step above the body text
	// (base.css's `--text-strong`).
	TextStrong ui.Color
}

// For resolves the appearance and scales the metrics for a settings value.
// The widgets' font, spacing and radius all follow the interface-size step,
// as the old build's CSS variables did.
func For(s settings.Settings, systemDark bool) Tokens {
	s = settings.Normalize(s)
	appearance := Resolve(s.Theme, systemDark)

	base := ui.LightTheme()
	if appearance == Dark {
		base = ui.DarkTheme()
	}
	// Copy so the shared theme values are never mutated in place.
	t := *base

	scale := settings.UIScaleFactor(s.UIScale)
	if scale <= 0 {
		scale = 1
	}
	f := float32(scale)
	t.FontSize = round1(t.FontSize * f)
	t.Spacing = round1(t.Spacing * f)
	t.Radius = round1(t.Radius * f)

	// The strokes and planes are alphas over white or black, per appearance,
	// exactly as base.css wrote them: a lit hairline in the dark, a dark one
	// in the light, and the focused step takes the accent.
	stroke, strokeActive := ui.RGBA(255, 255, 255, 0.08), ui.RGBA(143, 183, 255, 0.24)
	control, controlEdge, hairline := ui.RGBA(255, 255, 255, 0.055), ui.RGBA(255, 255, 255, 0.06), ui.RGBA(255, 255, 255, 0.07)
	textStrong := ui.RGBA(244, 245, 247, 0.96)
	if appearance == Light {
		stroke, strokeActive = ui.RGBA(0, 0, 0, 0.1), ui.RGBA(10, 108, 255, 0.28)
		control, controlEdge, hairline = ui.RGBA(0, 0, 0, 0.035), ui.RGBA(0, 0, 0, 0.07), ui.RGBA(0, 0, 0, 0.08)
		textStrong = ui.RGBA(23, 23, 26, 0.96)
	}
	return Tokens{
		Appearance:        appearance,
		Theme:             &t,
		Scale:             scale,
		RadiusXS:          round1(RadiusXS * f),
		RadiusSM:          round1(RadiusSM * f),
		RadiusMD:          round1(RadiusMD * f),
		RadiusLG:          round1(RadiusLG * f),
		InputStroke:       stroke,
		InputStrokeActive: strokeActive,
		Control:           control,
		ControlEdge:       controlEdge,
		Hairline:          hairline,
		TextStrong:        textStrong,
	}
}

// round1 rounds to one decimal, so a scaled metric stays a value a CSS
// author could have written and the tests can compare exactly.
func round1(v float32) float32 {
	return float32(int(v*10+0.5)) / 10
}
