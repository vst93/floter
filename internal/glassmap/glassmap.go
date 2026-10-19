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
// least (0.2), but the veil is the material's own weight, not the fill's. What
// the stop *does* move is how much of that alpha the app has to paint itself:
// the window's material already lays a veil of its own over the desktop, and
// the tint is compensated for it (see MaterialVeil, Spec.Target), so one slider
// position shows one solidity on every stop. `Spec.Composite` is that number —
// the one the settings page prints — and `Spec.TintAlpha` is what the panel
// paints to reach it.
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
	// TintAlpha is the panel's fill alpha, 0..1: the alpha the panel *paints*,
	// which is the transparency slider less the veil the window's own material
	// already contributes (see Material). The `off` and effect stops share the
	// rule, so turning the glass on never makes the window denser.
	TintAlpha float32
	// Target is the alpha the frame is asked to **composite** to: the
	// transparency slider (floored for `off`, capped at SolidTop). TintAlpha is
	// the fill that composites with Material back to it, so the same slider
	// position shows the same solidity on every stop (「透明度与玻璃档位相互
	// 影响」). It is carried so the contrast preference can lift the visible
	// floor and re-derive the fill.
	Target float32
	// Material is the veil the window's backdrop material already lays over the
	// desktop, 0..1 (see MaterialVeil): 0 where the platform has no material of
	// its own, which is every stop on Linux and Windows in this build. It is the
	// amount TintAlpha was compensated against, so Composite is exact.
	Material float32
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

// OffFrameFloor is the readability floor the `off` stop carries: the stop
// really does turn the blur off (see MaterialVeil: no material on any
// platform), so at the slider's bottom end the text would sit on the raw
// desktop. The floor is the same *shape* as ContrastFrameFloor — a floor on
// the frame's own alpha, not a replacement for the slider — and it is the one
// rung the effect stops do not need, because their backdrop is blurred: a
// frosted pane stays readable much thinner than a plain tint does. It is why
// the `off` stop reads denser than a glass stop at the slider's bottom end,
// which is the honest answer: with the blur gone, the tint is the whole of the
// material.
const OffFrameFloor = 0.35

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

// MaterialVeil is the veil the window's own backdrop material lays over the
// desktop on its own, as a self-solidity 0..1, per platform and stop: the
// weight the panel's tint does *not* have to carry.
//
// macOS is the one platform with a material per stop (shell.vibrancyForStop):
// `under-page` is the thinnest (the frosted end), `under-window` the standard
// pane, `sidebar` the densest. Those three materials are what made the same
// 47% read three different ways — the material's own weight sat on top of a
// tint that was the same number on every stop (「同一个透明度在三档下观感
// 不同」) — so the mapping compensates for it instead of pretending it is not
// there. Windows 11's Acrylic is one backdrop for every stop and Linux has no
// material at all, so both carry 0 here: the tint is the whole of the glass and
// needs no compensation. (Acrylic does add a veil of its own; it is left at 0
// until the number can be calibrated against a real window, which is the same
// rule the three macOS numbers follow — one table, one place to tune.)
//
// The numbers are the materials' own weights, not a sample of a wallpaper: a
// material's opacity is a property of the material, so the table is what a
// screenshot against a flat canvas would confirm. They are deliberately modest
// — a veil of v caps the slider's low end at v (a tint cannot be negative),
// so a heavy reading would make the bottom of the slider inert.
func MaterialVeil(goos, step string) float32 {
	if goos != "darwin" {
		return 0
	}
	switch settings.NormalizeGlassStep(step) {
	case "off":
		return 0
	case "frosted":
		return 0.12 // under-page: the thinnest of the three
	case "liquid":
		return 0.34 // sidebar: the densest
	default:
		return 0.22 // under-window: the standard pane
	}
}

// compensate is the fill alpha that composites with a material veil back to
// target: `1-(1-fill)(1-veil) == target`. It is the same over-frame algebra the
// content pane and the sheen use, in the other direction: those two split one
// alpha between two layers the app paints itself, this one takes away the layer
// the *window server* already painted. A veil heavier than the target leaves no
// fill at all (the material is already denser than the slider asks), and the top
// is the frame's own SolidTop so a maxed slider still reads as glass.
func compensate(target, veil float32) float32 {
	if veil <= 0 {
		return min(SolidTop, target)
	}
	if veil >= 1 {
		return 0
	}
	fill := (target - veil) / (1 - veil)
	if fill < 0 {
		return 0
	}
	return min(SolidTop, fill)
}

// floatStep is the fixed lift a floating pane carries over the frame's tint,
// base.css's `--glass-float-alpha = var(--glass-tint-alpha) + 0.1`: a drawer,
// dialog, toast or resident note is always denser than the frame it floats
// over, but it still rides the slider.
const floatStep = 0.1

// SpecFor maps a stored glass_step and main_opacity onto the plugin's
// parameters for a window with no material of its own (Linux, and any render
// without a window). SpecForOS is the mapping the app uses, because the
// window's backdrop material is part of the answer.
//
// Both inputs are normalized first, so a hand-edited value behaves exactly as
// the loader would.
//
// The panel's fill is the frame's own alpha — the transparency slider's value,
// `min(--glass-solid-top, opacity)`, less the veil the window's material
// already contributes — on every stop. The old build's `--glass-tint-alpha`
// raised that with the stop's dimming layer, which is what made the glass-on
// frame denser than the `off` frame and the slider's low end inert
// (「关闭液态玻璃效果时透明底可以了，但是开启时又没有透明了」).
// The stop's haze (`HazeAlpha`) is still carried — it names how much veil the
// stop asks for, and the material choice reads it — but it never moves the
// fill: the slider is the frame's only alpha truth, exactly as it is on `off`.
//
// The stop's visible difference is therefore the **effect**, not the opacity:
// the OS material (shell.vibrancyForStop) and the control lens (LensScale),
// which is the rung the scene paints itself.
func SpecFor(step string, mainOpacity uint8) Spec {
	return SpecForOS("linux", step, mainOpacity)
}

// SpecForOS is SpecFor for a named platform: the stop's window material
// (shell.vibrancyForStop) has a weight of its own, and the tint is compensated
// against it (see MaterialVeil) so one slider position shows one solidity on
// every stop. The platform is a parameter rather than a read of runtime.GOOS so
// the three tables are one decision a test can read on any host.
func SpecForOS(goos, step string, mainOpacity uint8) Spec {
	normalized := settings.NormalizeGlassStep(step)
	opacity := settings.NormalizeOpacity(mainOpacity, settings.DefaultMainOpacity)
	target := min(SolidTop, float32(opacity)/100)

	if normalized == "off" {
		// No glass effect — no lens, no haze, no blur — but the frame's own
		// transparency still paints: the window stays translucent and the
		// desktop shows through the plain tint, unblurred. The `off` stop used
		// to take the near-solid stand-in here, which is what read as the
		// app's transparent bottom failing (「应用的透明底是失效的」).
		//
		// The stop carries no material, so the readability floor is the whole
		// of its compensation: with the blur gone, a 10% tint would put the
		// text straight on the wallpaper.
		alpha := max(OffFrameFloor, target)
		return Spec{Translucent: true, Glass: false, Style: glass.Clear, TintAlpha: alpha, Target: alpha}
	}
	stop, ok := stops[normalized]
	if !ok { // only reachable without the loader's normalization
		stop = stops["regular"]
	}
	veil := MaterialVeil(goos, normalized)
	return Spec{
		Translucent: true,
		Glass:       true,
		Style:       stop.style,
		HazeAlpha:   stop.dim,
		TintAlpha:   compensate(target, veil),
		Target:      target,
		Material:    veil,
		Blur:        stop.blur,
		Saturate:    stop.saturate,
		LensScale:   stop.lens,
	}
}

// Composite is the alpha the frame actually shows over the desktop: the fill
// the app paints, composited with the veil the window's material paints for it.
// It is the number the settings page prints (「有效不透明度」) and the number a
// test can assert the slider against — the slider is the ask, this is the
// answer.
func (s Spec) Composite() float32 {
	if !s.Translucent {
		return 1
	}
	return 1 - (1-s.Material)*(1-s.TintAlpha)
}

// Contrast is the spec a desktop that asked for more contrast gets: the
// frame's floor raised to near-solid on every stop, the haze dropped and the
// lens damped. It is base.css's `@media (prefers-contrast: more)` block
// applied to the material — the same three tokens it overrode
// (`--glass-frame-floor`, `--glass-step-dim`, `--glass-lens-scale`) are the
// three this changes, and the blur is deliberately left alone, exactly as the
// old build left it. An opaque spec (the stand-in for a window with no
// transparency at all) is returned unchanged.
//
// The floor is applied to the **visible** alpha and the fill re-derived from
// it, so the preference means the same thing on every stop: 0.86 of the frame,
// not 0.86 of a fill that the window's material then adds to.
func (s Spec) Contrast() Spec {
	if !s.Translucent {
		return s
	}
	target := max(ContrastFrameFloor, min(SolidTop, s.Composite()))
	s.Target = target
	s.TintAlpha = compensate(target, s.Material)
	s.HazeAlpha = 0
	s.LensScale *= ContrastLensScale
	return s
}

// SpecForPreferences is SpecFor plus the desktop's contrast preference, the
// pair every caller that owns a `*ui.Context` has: a desktop that asked for
// more contrast gets the material dialled back (see Spec.Contrast).
func SpecForPreferences(step string, mainOpacity uint8, highContrast bool) Spec {
	return SpecForPreferencesOS("linux", step, mainOpacity, highContrast)
}

// SpecForPreferencesOS is SpecForPreferences for a named platform: the
// app's own path, because the material it compensates for is the one the
// platform's window server paints.
func SpecForPreferencesOS(goos, step string, mainOpacity uint8, highContrast bool) Spec {
	spec := SpecForOS(goos, step, mainOpacity)
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
//
// The step is `0.09`, not the reference's *visible* 0.06 band: at a 47% frame a
// 0.06 ask is carried by a pane at 0.11 alpha, which over the frame's own tint
// and the list's hairlines is not a recess a viewer resolves — the panel read as
// the card with rows loose on it, one of the flatnesses behind 「整体 UI/UX 很
// 小气」. A 0.09 step paints the pane at 0.17 over the same frame: still one
// sheet, still the slider's tint, but now visibly one level below the field row
// it sits under. It is bounded the same way it was — the pane composites to
// `frame + step` and never to a second tint — so a maxed slider still lands on
// solid and a thin one still shows the desktop.
const contentStep = 0.09

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

// TerminalFrameShare is the share of the terminal's alpha the **card face**
// carries; the terminal's own background (its canvas: the palette's colour, or
// the window's for the shipped `inherit`) carries the rest. See
// Spec.TerminalFrameAlpha.
//
// The terminal has two backgrounds by design — the app's frame, which the
// title bar and the panel's margin sit on, and the canvas, which is the colour
// the user picked — and both used to paint the terminal slider, so 46% showed
// as `1-(1-0.46)^2` = 71% (「终端透明度滚轴说的和看到的不一致」). Splitting the
// alpha between the two layers keeps one visible number: the frame keeps the
// smaller share, because the canvas is the colour the user chose and it has to
// dominate, while the share still leaves the bar and the margin legible.
const TerminalFrameShare = 0.4

// TerminalFrameAlpha is the alpha the card face carries on the terminal
// surface: its share of the frame's alpha (see TerminalFrameShare). The
// terminal's canvas then paints CanvasAlpha over it, so the two composite back
// to TintAlpha — the terminal slider, once.
func (s Spec) TerminalFrameAlpha() float32 {
	if !s.Translucent {
		return 1
	}
	return s.TintAlpha * TerminalFrameShare
}

// CanvasAlpha is the alpha the terminal's own background carries over the card
// face: the fill that composites with TerminalFrameAlpha to the frame's alpha.
// It is the over-frame algebra ContentAlpha uses, so the terminal's canvas is
// one sheet with the panel it sits on rather than a second full tint.
func (s Spec) CanvasAlpha() float32 {
	if !s.Translucent {
		return 1
	}
	return overFrame(s.TerminalFrameAlpha(), s.TintAlpha)
}

// TransparencyAvailable reports whether the window can show the desktop at
// all: a Windows window without a system backdrop (before 11 22H2) has no
// material behind its content, so a translucent frame would show the window's
// own frame colour instead of the desktop and the card is painted opaque
// (shell.windowShowsMaterial). The transparency slider cannot do anything
// there, so the settings page disables it with the reason instead of leaving a
// knob that moves nothing (「滑块整条失效」). macOS and Linux always composite
// their own windows, so they always answer true.
func TransparencyAvailable(goos string, vibrancy bool) bool {
	return goos != "windows" || vibrancy
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
