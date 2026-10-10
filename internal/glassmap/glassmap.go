// Package glassmap maps floter's glass settings — the effect stop
// (`glass_step`) and the window transparency (`main_opacity`) — onto the
// parameters the mygo glass plugin exposes.
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
//	glass.Blur{Radius, Mask}       // a backdrop blur, no tint, rim or shadow
//
// and nothing else on the effect axis. Two of the old numbers have no
// counterpart, and this package's tests pin that as a fact rather than
// hiding it:
//
//   - **Blur (10/22/28px).** `glass.Blur` blurs *the pixels of the scene
//     under the element* (scene.BackdropOf reads the frame's own buffer),
//     not the desktop behind the window: over the launcher's transparent
//     root it would blur transparency and change nothing. The desktop
//     behind the window is blurred by the window's vibrancy material
//     (mygo.VibrancyUnderWindow, see internal/shell), which is one
//     strength for the whole window. So the blur ladder becomes the OS
//     material, and the stop ladder keeps what the plugin can vary.
//   - **Saturation (130/170/200%).** The plugin has no saturation knob.
//
// What survives per stop is the glass *material* — `Clear` for the thin
// stop, `Regular` (the full lens) for the heavier two — and the haze veil
// `dim` from GLASS_STEP_TOKENS: a strong veil at the frosted end so thin
// glass stays readable, none at the liquid end where the lens does the
// work. `main_opacity` stays what it was: the tint's alpha, the frame's
// only alpha truth.
//
// `glass.Blur` and `glass.ScrollEdge` are used where they do have the
// desktop's counterpart: content scrolling *inside* the window, under a
// bar (the launcher's results under its field, see internal/launcher).
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
	// Style is the plugin's material style: the lens ladder.
	Style glass.Style
	// HazeAlpha is the veil of the theme's background painted under the
	// glass, 0..1, from the stop's `dim`.
	HazeAlpha float32
	// TintAlpha is the tint's alpha, 0..1, from main_opacity.
	TintAlpha float32
}

// The stop ladder, from GLASS_INTENSITY and GLASS_STEP_TOKENS in
// src/glass-material.ts: the old build's lens level (1/2/3) becomes the
// plugin's Clear/Regular, and its haze (0.6/0.5/0.2) is carried over as the
// veil. `off` is not a stop but the position that removes the material.
var stops = map[string]struct {
	style glass.Style
	haze  float32
}{
	"frosted": {glass.Clear, 0.6},
	"regular": {glass.Regular, 0.5},
	"liquid":  {glass.Regular, 0.2},
}

// SpecFor maps a stored glass_step and main_opacity onto the plugin's
// parameters. Both inputs are normalized first, so a hand-edited value
// behaves exactly as the loader would.
func SpecFor(step string, mainOpacity uint8) Spec {
	normalized := settings.NormalizeGlassStep(step)
	opacity := settings.NormalizeOpacity(mainOpacity, settings.DefaultMainOpacity)
	alpha := float32(opacity) / 100

	if normalized == "off" {
		return Spec{Enabled: false, Style: glass.Clear, TintAlpha: alpha}
	}
	stop, ok := stops[normalized]
	if !ok { // only reachable without the loader's normalization
		stop = stops["regular"]
	}
	return Spec{
		Enabled:   true,
		Style:     stop.style,
		HazeAlpha: stop.haze,
		TintAlpha: alpha,
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
	return glass.Glass{Style: spec.Style, Tint: theme.Background.Alpha(spec.TintAlpha)}
}

// Haze is the veil painted under the glass: the theme's background at the
// stop's dim alpha, or Transparent when there is none. It is a color rather
// than a material so the caller can lay it under the glass pane, as the old
// `--glass-step-dim` composited under `--glass-tint-alpha`.
//
// The veil exists to keep text readable over a *blurred backdrop* — the
// desktop the old build's `backdrop-filter` smeared. A platform with no
// backdrop (Linux, and Windows before 11: `Context.Vibrancy` is false there)
// has no blur under the panel, so the veil is not covering a smear; it is
// only opacity, and half a screen of it turns glass into a solid slab. The
// veil is dropped there and the tint's own alpha — the user's setting — is
// what shows the desktop through, which is the honest reading of "glass"
// when the renderer cannot blur what is behind the window.
func Haze(spec Spec, theme *ui.Theme, backdrop bool) ui.Color {
	if !spec.Enabled || spec.HazeAlpha <= 0 || !backdrop {
		return ui.Transparent
	}
	return theme.Background.Alpha(spec.HazeAlpha)
}
