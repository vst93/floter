// Package glassmap maps floter's glass settings — the effect stop
// (`glass_step`) and the window transparency (`main_opacity`) — onto the
// panel's fill.
//
// The old app's material model (see src/glass-material.ts) had two
// independent axes:
//
//   - `glass_step`: off | frosted | regular | liquid — the *effect*
//     (blur, saturation, haze, control lens);
//   - `main_opacity`: 10-100 — the frame's tint alpha, the readability
//     control.
//
// The panel's fill is a **translucent tint**, not the glass plugin's
// material. `glass.Glass` maps what is under the element *in the scene* and
// its shader always writes an opaque result (`return vec4(c, 1.0)`), so a
// panel filled with it can never show the desktop — which is the whole point
// of the transparency slider. Instead the panel draws base.css's
// `--glass-tint`: the frame's own alpha (the slider) raised by the stop's
// dimming layer (`--glass-step-dim`), a single translucent colour the
// window's vibrancy shows through.
//
//   - **Blur (10/22/28px).** The desktop behind the window is blurred by the
//     window's vibrancy material — `mygo.VibrancyUnderWindow` on the panel's
//     window, which is one strength for the whole window (macOS's
//     NSVisualEffectView, Windows 11's system backdrops; Linux has none, and
//     there the tint's own alpha is the whole of the material). So the blur
//     ladder becomes the OS material, and the stop ladder keeps what the fill
//     can vary: the dimming layer.
//   - **Saturation (130/170/200%).** No knob, on the window or in the tint.
//
// `main_opacity` stays what it was: the frame's only alpha truth. The stop
// moves the dim, never the alpha truth — `frosted` dims the most (0.6),
// `liquid` the least (0.2), so a thin frame does not leave the desktop
// readable through the copy.
//
// `glass.ScrollEdge` (and `glass.Blur`) are used where they do have a
// counterpart: content scrolling *inside* the window, under a bar (the
// launcher's results under its field, the settings body under its header).
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
	// HazeAlpha is the stop's dimming layer (0.6/0.5/0.2). It is not painted
	// on its own: base.css folds it into the tint's alpha (`--glass-tint-alpha`),
	// which is the one fill the panel draws.
	HazeAlpha float32
	// TintAlpha is the panel's fill alpha, 0..1: the frame's own alpha raised
	// by the stop's dimming layer, as the old build's `--glass-tint-alpha`.
	TintAlpha float32
}

// The stop ladder, from GLASS_INTENSITY and GLASS_STEP_TOKENS in
// src/glass-material.ts: the old build's lens level (1/2/3) becomes the
// plugin's Clear/Regular, and its haze (0.6/0.5/0.2) is the dimming layer the
// tint's alpha is raised by. `off` is not a stop but the position that removes
// the material.
var stops = map[string]struct {
	style glass.Style
	dim   float32
}{
	"frosted": {glass.Clear, 0.6},
	"regular": {glass.Regular, 0.5},
	"liquid":  {glass.Regular, 0.2},
}

// SolidTop is base.css's `--glass-solid-top`: the maxed end of the
// transparency slider. It is 0.98, not 1, because a fully opaque frame drops
// the material's sheen and rim into invisibility and reads as paint. It caps
// the frame's own alpha, exactly as `--glass-frame-alpha = min(solid_top,
// max(frame_floor, opacity))` did.
const SolidTop = 0.98

// floatStep is the fixed lift a floating pane carries over the frame's tint,
// base.css's `--glass-float-alpha = var(--glass-tint-alpha) + 0.1`: a drawer,
// dialog, toast or resident note is always denser than the frame it floats
// over, but it still rides the slider.
const floatStep = 0.1

// SpecFor maps a stored glass_step and main_opacity onto the plugin's
// parameters. Both inputs are normalized first, so a hand-edited value
// behaves exactly as the loader would.
//
// The panel's fill is the reference's `--glass-tint-alpha`:
//
//	1 - (1 - dim*(1 - opacity)) * (1 - frame_alpha)
//
// with `frame_alpha = min(--glass-solid-top, opacity)`, so the transparency
// slider is the frame's own alpha and the stop's dimming layer raises it, so a
// thin `frosted` frame does not leave the desktop readable through the copy.
// The slider is still the only knob that moves the frame: the stop moves the
// dim, not the alpha truth.
func SpecFor(step string, mainOpacity uint8) Spec {
	normalized := settings.NormalizeGlassStep(step)
	opacity := settings.NormalizeOpacity(mainOpacity, settings.DefaultMainOpacity)
	alpha := float32(opacity) / 100

	if normalized == "off" {
		return Spec{Enabled: false, Style: glass.Clear, TintAlpha: 1}
	}
	stop, ok := stops[normalized]
	if !ok { // only reachable without the loader's normalization
		stop = stops["regular"]
	}
	frame := min(SolidTop, alpha)
	tint := 1 - (1-stop.dim*(1-alpha))*(1-frame)
	return Spec{
		Enabled:   true,
		Style:     stop.style,
		HazeAlpha: stop.dim,
		TintAlpha: tint,
	}
}

// ContentAlpha is the fill alpha of the content pane: the launcher's result
// list and the settings body (`--surface-sunken`). The pane is a second sheet
// of the frame's own tint over the frame, so it composites denser than the
// frame - the reference's readability contract - while still following the
// transparency slider.
//
// The reference pinned the pane to a fixed 0.62-0.80 band instead, which the
// slider barely moved and which hid the frame tint it sat on: the user read
// that as "the bottom is opaque and the transparency slider does nothing"
// (the frame tint moved 0.505 -> 0.98 behind a pane that stayed 0.64 -> 0.80).
// Riding the tint makes the app's own background the thing the slider moves.
// The `off` stop keeps an opaque pane, since it has no transparency at all.
func (s Spec) ContentAlpha() float32 {
	if !s.Enabled {
		return 1
	}
	return s.TintAlpha
}

// FloatAlpha is the fill alpha of a floating pane (`--glass-float`): the
// frame's tint plus floatStep, capped at opaque. A pane that floats over the
// frame is always denser than it, at every stop, and still follows the slider.
func (s Spec) FloatAlpha() float32 {
	if !s.Enabled {
		return 1
	}
	return min(1, s.TintAlpha+floatStep)
}

// Frame is the panel's own fill: the frame's tint at the reference's
// `--glass-tint-alpha`, translucent so what is behind the window shows
// through it. The window's vibrancy blurs that backdrop where the platform
// has one (macOS's NSVisualEffectView, Windows 11's backdrops); where it does
// not (Linux), the tint's own alpha is the whole of the glass, exactly as the
// old build's `backdrop-filter`-less fallback was.
//
// enabled is false for the `off` stop, where the caller paints the near-solid
// stand-in instead.
func Frame(spec Spec, theme *ui.Theme) (fill ui.Color, enabled bool) {
	if !spec.Enabled {
		return ui.Transparent, false
	}
	return theme.Background.Alpha(spec.TintAlpha), true
}
