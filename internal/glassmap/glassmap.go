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
// of the transparency slider. Instead the panel draws the frame's own alpha:
// the transparency slider, `min(--glass-solid-top, opacity)`, a single
// translucent colour the window's vibrancy shows through. The stop's dimming
// layer (`--glass-step-dim`) is carried in the Spec but never moves the fill,
// so the `off` and effect stops share one transparency floor.
//
//   - **Blur (10/22/28px).** The desktop behind the window is blurred by the
//     window's vibrancy material, chosen per stop (shell.vibrancyForStop):
//     macOS's NSVisualEffectView materials have a weight per stop, Windows 11
//     has only the one backdrop that blurs (Acrylic), and Linux has none —
//     there the tint's own alpha is the whole of the material. So the blur
//     ladder becomes the OS material.
//   - **Saturation (130/170/200%).** No knob, on the window or in the tint;
//     carried in the Spec for the record and the material choice.
//   - **Control lens (0.5/1/1.6).** `--glass-lens-scale` scales the theme's
//     LensRim/LensSheen/LensEdge, the rung the scene paints itself, so a stop
//     switch is visible on every control even where the window has no
//     material.
//
// `main_opacity` stays what it was: the frame's only alpha truth. The stop
// never moves it — `frosted` asks for the most veil (0.6) and `liquid` the
// least (0.2), but the veil is the material's own weight, not the fill's.
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
	// Translucent is true for every stop, including `off`: the opacity slider
	// is the window's transparency truth, and turning the glass effect off
	// must not turn the window opaque (「关闭玻璃效果的时候，应用的透明底是
	// 失效的」).
	Translucent bool
	// Glass is true for the effect stops (frosted/regular/liquid): they carry
	// the blur ladder, the haze and the control lens. `off` carries none of
	// them, but it still paints the frame's own translucent tint.
	Glass bool
	// Style is the plugin's material style: the lens ladder.
	Style glass.Style
	// HazeAlpha is the stop's dimming layer (0.6/0.5/0.2): how much veil the
	// stop asks for over a blurred backdrop. It is **not** painted into the
	// fill — the transparency slider is the frame's only alpha truth — but it
	// is kept so the material choice and the report can name it.
	HazeAlpha float32
	// TintAlpha is the panel's fill alpha, 0..1: the frame's own alpha, the
	// transparency slider, on every stop. The `off` and effect stops share it,
	// so turning the glass on never makes the window denser.
	TintAlpha float32
	// Blur is the stop's backdrop blur in DIPs, base.css's `--glass-step-blur`
	// (10/22/28). The panel cannot run its own `backdrop-filter`, so this is
	// what the window's vibrancy material is chosen from: the blur ladder is
	// the OS material (see shell.vibrancyForStop).
	Blur float32
	// Saturate is the stop's backdrop saturation in percent, base.css's
	// `--glass-step-saturate` (130/170/200). The framework exposes no
	// saturation knob on the window material, so it is carried for the
	// record and for the material choice; it never moves the fill's alpha.
	Saturate float32
	// LensScale multiplies the control-lens family (`--glass-lens-scale`:
	// 0.5/1/1.6), the visible "how much lens the controls carry" rung. The
	// theme scales LensRim/LensSheen/LensEdge by it, so a stop switch is a
	// visible change on every control, not only a tint shift.
	LensScale float32
}

// The stop ladder, from GLASS_INTENSITY and GLASS_STEP_TOKENS in
// src/glass-material.ts: the old build's lens level (1/2/3) becomes the
// plugin's Clear/Regular, and its haze (0.6/0.5/0.2) is the dimming layer the
// tint's alpha is raised by. `off` is not a stop but the position that removes
// the material.
//
// The blur/saturate/lens columns are the rest of the old build's effect
// table (base.css's `[data-glass]` blocks). They are the rungs that make a
// stop change visible — a tint shift alone left `frosted` and `regular`
// reading the same (「看起来好像没什么区别」).
var stops = map[string]struct {
	style    glass.Style
	dim      float32
	blur     float32
	saturate float32
	lens     float32
}{
	"frosted": {glass.Clear, 0.6, 10, 130, 0.5},
	"regular": {glass.Regular, 0.5, 22, 170, 1},
	"liquid":  {glass.Regular, 0.2, 28, 200, 1.6},
}

// SolidTop is base.css's `--glass-solid-top`: the maxed end of the
// transparency slider. It is 0.98, not 1, because a fully opaque frame drops
// the material's sheen and rim into invisibility and reads as paint. It caps
// the frame's own alpha, exactly as `--glass-frame-alpha = min(solid_top,
// max(frame_floor, opacity))` did.
const SolidTop = 0.98

// ContrastFrameFloor is base.css's `--glass-frame-floor` under
// `@media (prefers-contrast: more)`: the low end of the transparency slider is
// lifted to near-solid, on every stop. The old build's comment says why it is
// the material and not only the borders: "At the low end of the transparency
// slider the frame is genuinely see-through — that is the point of a thin
// material — but a user who has asked the system for contrast must not have
// to also go find the slider." In normal use the same token is `0`, so the
// floor is inert until the desktop asks.
const ContrastFrameFloor = 0.86

// ContrastLensScale is base.css's `--glass-lens-scale` under the same block:
// the lens sheen is a decorative refraction cue, so it is damped to a
// near-flat pane while the pane keeps its stronger edge.
const ContrastLensScale = 0.6

// floatStep is the fixed lift a floating pane carries over the frame's tint,
// base.css's `--glass-float-alpha = var(--glass-tint-alpha) + 0.1`: a drawer,
// dialog, toast or resident note is always denser than the frame it floats
// over, but it still rides the slider.
const floatStep = 0.1

// SpecFor maps a stored glass_step and main_opacity onto the plugin's
// parameters. Both inputs are normalized first, so a hand-edited value
// behaves exactly as the loader would.
//
// The panel's fill is the frame's own alpha — the transparency slider's value,
// `min(--glass-solid-top, opacity)` — on every stop. The old build's
// `--glass-tint-alpha` raised that with the stop's dimming layer, which is
// what made the glass-on frame denser than the `off` frame and the slider's
// low end inert (「关闭液态玻璃效果时透明底可以了，但是开启时又没有透明了」).
// The stop's haze (`HazeAlpha`) is still carried — it names how much veil the
// stop asks for, and the material choice reads it — but it never moves the
// fill: the slider is the frame's only alpha truth, exactly as it is on `off`.
//
// The stop's visible difference is therefore the **effect**, not the opacity:
// the OS material (shell.vibrancyForStop) and the control lens (LensScale),
// which is the rung the scene paints itself.
func SpecFor(step string, mainOpacity uint8) Spec {
	normalized := settings.NormalizeGlassStep(step)
	opacity := settings.NormalizeOpacity(mainOpacity, settings.DefaultMainOpacity)
	alpha := float32(opacity) / 100
	frame := min(SolidTop, alpha)

	if normalized == "off" {
		// No glass effect — no lens, no haze, no blur — but the frame's own
		// transparency still paints: the window stays translucent and the
		// desktop shows through the plain tint, unblurred. The `off` stop used
		// to take the near-solid stand-in here, which is what read as the
		// app's transparent bottom failing (「应用的透明底是失效的」).
		return Spec{Translucent: true, Glass: false, Style: glass.Clear, TintAlpha: frame}
	}
	stop, ok := stops[normalized]
	if !ok { // only reachable without the loader's normalization
		stop = stops["regular"]
	}
	return Spec{
		Translucent: true,
		Glass:       true,
		Style:       stop.style,
		HazeAlpha:   stop.dim,
		TintAlpha:   frame,
		Blur:        stop.blur,
		Saturate:    stop.saturate,
		LensScale:   stop.lens,
	}
}

// Contrast is the spec a desktop that asked for more contrast gets: the
// frame's floor raised to near-solid on every stop, the haze dropped and the
// lens damped. It is base.css's `@media (prefers-contrast: more)` block
// applied to the material — the same three tokens it overrode
// (`--glass-frame-floor`, `--glass-step-dim`, `--glass-lens-scale`) are the
// three this changes, and the blur is deliberately left alone, exactly as the
// old build left it. An opaque spec (the stand-in for a window with no
// transparency at all) is returned unchanged.
func (s Spec) Contrast() Spec {
	if !s.Translucent {
		return s
	}
	s.TintAlpha = max(ContrastFrameFloor, min(SolidTop, s.TintAlpha))
	s.HazeAlpha = 0
	s.LensScale *= ContrastLensScale
	return s
}

// SpecForPreferences is SpecFor plus the desktop's contrast preference, the
// pair every caller that owns a `*ui.Context` has: a desktop that asked for
// more contrast gets the material dialled back (see Spec.Contrast).
func SpecForPreferences(step string, mainOpacity uint8, highContrast bool) Spec {
	spec := SpecFor(step, mainOpacity)
	if highContrast {
		spec = spec.Contrast()
	}
	return spec
}

// contentStep is the recess the content pane carries over the frame: the
// pane is one step denser than the frame it sits on, never a second sheet of
// the frame's own tint. The old code painted the pane at the *frame's* alpha
// (`ContentAlpha() == TintAlpha`), which stacked two full tints and left the
// visible bottom at `1-(1-t)^2` - 0.76 at the slider's bottom against the
// frame's 0.51 - so the bottom read as a near-opaque bar and the slider moved
// it over half the band it moves the frame. The pane is now painted at the
// alpha that *composites* to `frame + contentStep` over the frame, so the app's
// background (top, middle and bottom) is the frame tint the slider owns.
const contentStep = 0.06

// overFrame returns the alpha a pane must carry to composite to target over
// a frame of alpha frame: `1 - (1-frame)(1-pane) == target`.
func overFrame(frame, target float32) float32 {
	if target <= frame {
		return 0
	}
	if target >= 1 {
		return 1
	}
	return (target - frame) / (1 - frame)
}

// ContentAlpha is the fill alpha of the content pane: the launcher's result
// list and the settings body (`--surface-sunken`). It is the alpha the pane
// carries *over the frame*, chosen so the visible recess composites to the
// frame tint plus {@link contentStep} - a single sheet the transparency slider
// moves, not a second full tint that halves the slider's effect. Every stop
// rides the slider, `off` included: the pane is as translucent as the frame
// it sits on.
func (s Spec) ContentAlpha() float32 {
	if !s.Translucent {
		return 1
	}
	return overFrame(s.TintAlpha, min(1, s.TintAlpha+contentStep))
}

// FloatAlpha is the fill alpha of a floating pane (`--glass-float`): the
// frame's tint plus floatStep, capped at opaque. A pane that floats over the
// frame is always denser than it, at every stop, and still follows the slider.
func (s Spec) FloatAlpha() float32 {
	if !s.Translucent {
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
// The fill is built for every stop, `off` included: the `off` stop drops the
// effect (blur, haze, lens) but keeps the frame's own transparency, so the
// window is never made opaque by turning the glass off. The caller still gets
// the second return to know whether the stop carries the glass material
// (`spec.Glass`).
func Frame(spec Spec, theme *ui.Theme) (fill ui.Color, translucent bool) {
	if !spec.Translucent {
		return ui.Transparent, false
	}
	return theme.Background.Alpha(spec.TintAlpha), true
}
