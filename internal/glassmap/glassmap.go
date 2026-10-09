// Package glassmap maps floter's two glass settings — the effect stop
// (`glass_step`) and the window transparency (`main_opacity`) — onto the
// parameters the mygo glass plugin actually exposes.
//
// The old app's material model (see src/glass-material.ts) had two
// independent axes:
//
//   - `glass_step`: off | frosted | regular | liquid — the *effect*
//     (blur, saturation, haze, control lens);
//   - `main_opacity`: 10-100 — the frame's tint alpha, the readability
//     control.
//
// The plugin (github.com/egoist/mygo/plugins/glass) exposes
//
//	glass.Glass{Style: glass.Regular | glass.Clear, Tint: ui.Color, Interactive: bool}
//
// plus a separate `glass.Blur` backdrop blur. There is no intensity knob:
// `Regular` is the only Liquid Glass material and `Clear` is the lighter
// one. So P0 maps
//
//	off      -> no material at all
//	frosted  -> glass.Clear   (the plugin's lighter material)
//	regular  -> glass.Regular
//	liquid   -> glass.Regular (the plugin has no heavier one)
//
// and spends `main_opacity` on the material's `Tint` alpha, the only
// opacity-shaped parameter the plugin has. A fully faithful mapping of the
// three effect stops needs a plugin intensity parameter (or stacked
// `glass.Blur`), which is a P2 item — recorded in the R163 report.
package glassmap

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// Spec is the plugin parameters a settings pair maps to, as plain values so
// the mapping is testable without a window.
type Spec struct {
	// Enabled is false for the `off` stop: the panel gets no material.
	Enabled bool
	// Style is the plugin's material style.
	Style glass.Style
	// TintAlpha is the tint's alpha, 0..1, from main_opacity.
	TintAlpha float32
}

// SpecFor maps a stored glass_step and main_opacity onto the plugin's
// parameters. Both inputs are normalized first, so a hand-edited value
// behaves exactly as the loader would.
func SpecFor(step string, mainOpacity uint8) Spec {
	normalized := settings.NormalizeGlassStep(step)
	opacity := settings.NormalizeOpacity(mainOpacity, settings.DefaultMainOpacity)
	alpha := float32(opacity) / 100

	switch normalized {
	case "off":
		return Spec{Enabled: false, Style: glass.Clear, TintAlpha: alpha}
	case "frosted":
		return Spec{Enabled: true, Style: glass.Clear, TintAlpha: alpha}
	default: // regular, liquid
		return Spec{Enabled: true, Style: glass.Regular, TintAlpha: alpha}
	}
}

// Material builds the plugin material for a spec, tinted toward the theme's
// background so the tint reads as the frame's own fill (light in a light
// theme, dark in a dark one), as the old app's frame alpha did. It returns
// nil for the `off` stop, where the caller should leave the element's
// background alone instead of calling Material.
func Material(spec Spec, theme *ui.Theme) ui.Material {
	if !spec.Enabled {
		return nil
	}
	tint := theme.Background.Alpha(spec.TintAlpha)
	return glass.Glass{Style: spec.Style, Tint: tint}
}
