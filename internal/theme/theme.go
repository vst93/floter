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

	// BodyUnits is the body text size in the old app's unit, `--text-body`:
	// 11px at scale 1. Every size in this codebase is in the same unit (a
	// `Space(n)` is four of them), so the framework theme's point size is set
	// from it rather than from the framework's own default.
	BodyUnits = 11
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
	// The old app's ladder is in its own unit: `--u: 1px * ui-scale` with
	// `--text-body: 11px * ui-scale`, so a body line is eleven units and every
	// `Space(n)` in this codebase is four of them (`--u * 4n`). The framework's
	// default is 13 or 14 points, which is a quarter larger than this design
	// and made every row taller than the one it copied; the body size is the
	// old app's eleven units, and the ladder around it (caption, emphasis,
	// title, display) follows from there.
	t.FontSize = round1(BodyUnits * f)
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

// ANSI is the sixteen program colours a command's own output is drawn with,
// per appearance: base.css's `--ansi-black` through `--ansi-bright-white`.
//
// A CLI's stdout carries SGR sequences — `git diff`, `ls --color` — and the
// text form has to read them as a terminal would (the old build's R65:
// 「有些是带格式的终端字符」). The colours are the app's own rather than the
// user's terminal palette, which is what the old build's CSS variables were:
// the sheet and the terminal each have their own sixteen, and neither drifts
// with the other's settings.
func ANSI(appearance Appearance) [16]ui.Color {
	if appearance == Light {
		return [16]ui.Color{
			ui.Hex("#1d1f21"), ui.Hex("#c72e3c"), ui.Hex("#19733a"), ui.Hex("#7a5c00"),
			ui.Hex("#1659b7"), ui.Hex("#9c2ca0"), ui.Hex("#007c86"), ui.Hex("#535762"),
			ui.Hex("#767b86"), ui.Hex("#d63d4b"), ui.Hex("#1a6e38"), ui.Hex("#7a5900"),
			ui.Hex("#246fd1"), ui.Hex("#ac38a9"), ui.Hex("#006e7a"), ui.Hex("#2c3037"),
		}
	}
	return [16]ui.Color{
		ui.Hex("#4b5563"), ui.Hex("#f38ba8"), ui.Hex("#a6e3a1"), ui.Hex("#f9e2af"),
		ui.Hex("#89b4fa"), ui.Hex("#f5c2e7"), ui.Hex("#94e2d5"), ui.Hex("#cdd6f4"),
		ui.Hex("#6b7280"), ui.Hex("#f38ba8"), ui.Hex("#a6e3a1"), ui.Hex("#f9e2af"),
		ui.Hex("#89b4fa"), ui.Hex("#f5c2e7"), ui.Hex("#94e2d5"), ui.Hex("#eef2ff"),
	}
}

// ANSIColor resolves one program colour for an appearance: the sixteen named
// ones by index, the 256-colour cube and 24-bit forms by value.
func ANSIColor(appearance Appearance, kind int, n uint8, r, g, b uint8) ui.Color {
	switch kind {
	case 1: // named
		palette := ANSI(appearance)
		if int(n) < len(palette) {
			return palette[n]
		}
		return ui.Transparent
	case 2: // the 256-colour cube
		return cubeColor(n)
	case 3: // truecolour
		return ui.RGBA(r, g, b, 1)
	}
	return ui.Transparent
}

// cubeColor resolves a 256-colour index: the first sixteen are the named
// colours, then a 6×6×6 cube, then a grey ramp.
func cubeColor(n uint8) ui.Color {
	if n < 16 {
		// The named colours again; the caller resolves them by appearance.
		return ui.Transparent
	}
	if n < 232 {
		index := int(n) - 16
		steps := [6]uint8{0, 95, 135, 175, 215, 255}
		return ui.RGBA(steps[index/36], steps[(index/6)%6], steps[index%6], 1)
	}
	grey := uint8(8 + 10*(int(n)-232))
	return ui.RGBA(grey, grey, grey, 1)
}
