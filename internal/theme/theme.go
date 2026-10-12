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
	"math"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
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
	// ControlHover and ControlPress are the control ladder's two other
	// rungs (base.css's `--glass-control-hover` / `--glass-control-press`):
	// a pointer over a pane lifts it, a pointer down sinks it *below* the
	// resting face, and the settings sidebar's current page is the raised
	// quiet pane (`--glass-raised-quiet`, an alias of ControlHover).
	ControlHover ui.Color
	ControlPress ui.Color
	// Track is the groove a switch or a segmented control is looked into
	// (base.css's `--glass-track`), and Field the slot a text field cuts
	// into the glass (`--glass-field`).
	Track ui.Color
	Field ui.Color
	// Opaque is the near-solid stand-in for a surface whose glass has been
	// turned off or that must not show the desktop (base.css's
	// `--surface-opaque`): the `off` stop's card.
	Opaque ui.Color
	// FieldFill is the launcher's field block: `--glass-field` composited
	// over `--surface-opaque`, a near-solid surface one step brighter than
	// the card so the caret never sits over a ghost of the desktop
	// (`.collapsed-card__input-row`, R21).
	FieldFill ui.Color
	// TextStrong is a row's own label color, one step above the body text
	// (base.css's `--text-strong`), TextSecondary and TextTertiary the two
	// quieter steps below TextMuted.
	TextStrong    ui.Color
	TextSecondary ui.Color
	TextTertiary  ui.Color
	// The control lens family (base.css's `--glass-lens-*`): a control's top
	// rim, its broad sheen and its bottom shade. The framework draws no such
	// layers, and without them a translucent pane reads as a flat tint with
	// a border rather than as a lens; Lens stacks them as the one inset
	// shadow the old build's `--elev-0` was.
	LensRim   ui.Color
	LensSheen ui.Color
	LensEdge  ui.Color
	// SheenTop and SheenBottom are the card's top-down highlight (base.css's
	// `--sheen-top` / `--sheen-bottom`), the gradient that gives the panel's
	// glass a lit face rather than a flat tint.
	SheenTop    ui.Color
	SheenBottom ui.Color
	// ShadowAmbient and ShadowContact are the window's own cast shadow
	// (base.css's `--window-shadow-ambient` / `--window-shadow-contact`): the
	// broad throw and the tight contact edge a card carries where the platform
	// has no window-server shadow (Linux, Windows).
	ShadowAmbient ui.Color
	ShadowContact ui.Color
	// EdgeHighlight, EdgeShadow and EdgeHighlightX are the panel's rim (base.css's
	// `--edge-highlight` / `--edge-shadow` / `--edge-highlight-x`): the lit top
	// edge, the dark bottom edge and the whisper of a stroke on both sides that
	// `--glass-rim-shadow` composes. The launcher card has no rim (R72 removed
	// it — the card is its own floating window); the settings and terminal
	// panels, which sit *on* a surface, do.
	EdgeHighlight  ui.Color
	EdgeShadow     ui.Color
	EdgeHighlightX ui.Color
	// AccentEdge is the accent as a 1px keyline (base.css's `--accent-edge`):
	// the emphasis a chosen segment or a recommended badge carries without an
	// accent fill.
	AccentEdge ui.Color
	// Sunken is the settings content pane's own fill (base.css's
	// `--surface-sunken`): the settings panel is a reading surface, and it
	// carries a fill of its own so it reads one step denser than the frame.
	// Its alpha is the *over-frame* fill that composites to the frame tint
	// plus a small recess (glassmap.Spec.ContentAlpha), so the app's whole
	// background - the bottom included - stays the slider's single sheet.
	Sunken ui.Color
	// RaisedQuiet is the sidebar's chosen page: the control-hover lift
	// composited opaque over the content pane, so the framework's own accent
	// fill behind a chosen list row cannot bleed through as a blue pill.
	RaisedQuiet ui.Color
	// Float is a floating pane's own fill (base.css's `--glass-float`): the
	// frame's tint plus a fixed step, so a drawer, a dialog, a toast or the
	// terminal's resident note is always denser than the card it floats over.
	Float ui.Color

	// The text ladder, in points at the current step: base.css's registers,
	// which the codebase had been spelling as offsets off the body size (a
	// +6 that is 6.6 points at the Large step, not 6).
	Caption  float32
	Body     float32
	Emphasis float32
	Title    float32
	Display  float32
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

	// The whole palette is base.css's, not the framework's stock one. The
	// old build's tokens are what every surface was drawn against — the
	// pastel accent, the translucent control ladder, the near-black glass —
	// and the framework's defaults (a saturated blue, opaque grey hover
	// fills) are a different design. Setting them here means one place
	// decides the look and every widget the views do not style by hand —
	// buttons, inputs, switches, focus rings, the scrollbar — inherits it.
	p := paletteFor(appearance)
	t.Background = p.background
	t.Text = p.text
	t.TextMuted = p.textMuted
	t.Accent = p.accent
	t.AccentHover = p.accentHover
	t.AccentPressed = p.accentPressed
	t.AccentText = p.accentText
	t.Border = p.stroke
	t.Surface = p.control
	t.SurfaceHover = p.controlHover
	t.SurfacePressed = p.controlPress
	t.Selection = p.selection
	t.Focus = p.focus
	t.Warning = p.warning
	t.Scrollbar = p.scrollbar
	t.ScrollbarWidth = round1(6 * f)
	// The framework's control radius is the ladder's `--radius-sm` (9px),
	// not its own 6px stock value: a button or a field on a surface is the
	// ladder's second step, and the card and window keep their own steps.
	t.Radius = round1(RadiusSM * f)

	// The strokes and planes the views read directly are the same palette
	// values, per appearance, exactly as base.css wrote them.
	stroke, strokeActive := p.stroke, p.strokeActive
	control, controlEdge, hairline := p.control, p.controlEdge, p.hairline
	textStrong := p.textStrong
	controlHover, controlPress := p.controlHover, p.controlPress
	track, field := p.track, p.field
	textSecondary, textTertiary := p.textSecondary, p.textTertiary
	// The content pane rides the frame's own tint (the transparency slider)
	// as a single sheet: the pane is only the recess it needs to composite to
	// `frame + step`, so the app's background - top, middle and bottom - is
	// what the slider moves. The reference pinned `--surface-sunken` to a fixed
	// 0.62-0.80 band (which the slider barely moved), and an earlier rewrite
	// then painted the pane at the frame's own alpha, stacking two full tints
	// into a near-opaque bottom. See glassmap.Spec.ContentAlpha.
	spec := glassmap.SpecFor(s.GlassStep, s.MainOpacity)
	sunken := spec.ContentAlpha()
	floatAlpha := spec.FloatAlpha()
	// The panel's sheen is the glass's lit face. The off stop has no glass, so
	// it paints the plain tint with no sheen - base.css's `[data-glass="off"]
	// .shell` dropped the gradient along with the material.
	sheen := spec.TintAlpha
	if !spec.Glass {
		sheen = 0
	}
	baseFill := t.Background
	if appearance == Dark {
		baseFill = ui.RGB(17, 18, 20)
	} else {
		baseFill = ui.RGB(243, 244, 247)
	}
	// The sidebar's chosen page is the reference's `--glass-raised-quiet`,
	// but the framework marks a chosen list row by painting the accent
	// *behind* the row, and a translucent lift over that reads as a blue
	// pill — the opposite of the quiet one the reference asks for. So this
	// rung is the opaque composite of the control-hover lift over the pane
	// it sits on: it hides the accent and keeps the lift.
	raisedQuiet := blendOpaque(baseFill, p.controlHover)
	// A floating pane's fill: the frame's tint plus the reference's fixed step
	// (`--glass-float-alpha = var(--glass-tint-alpha) + 0.1`), in the float
	// palette's own slightly lighter colour, so a drawer, dialog or toast is
	// always denser than the frame it floats over and still follows the slider.
	floatFill := ui.RGB(24, 25, 29)
	if appearance == Light {
		floatFill = ui.RGB(252, 252, 254)
	}

	// The ladder, at the step: base.css's own registers (caption 10, body 11,
	// emphasis 12, title 13, display 17), each scaled once here rather than
	// spelled as an offset wherever a register is needed.
	caption := round1(10 * f)
	body := round1(11 * f)
	emphasis := round1(12 * f)
	title := round1(13 * f)
	display := round1(17 * f)

	return Tokens{
		Appearance:        appearance,
		Theme:             &t,
		Sunken:            baseFill.Alpha(sunken),
		RaisedQuiet:       raisedQuiet,
		Float:             floatFill.Alpha(floatAlpha),
		Caption:           caption,
		Body:              body,
		Emphasis:          emphasis,
		Title:             title,
		Display:           display,
		Scale:             scale,
		RadiusXS:          round1(RadiusXS * f),
		RadiusSM:          round1(RadiusSM * f),
		RadiusMD:          round1(RadiusMD * f),
		RadiusLG:          round1(RadiusLG * f),
		InputStroke:       stroke,
		InputStrokeActive: strokeActive,
		Control:           control,
		ControlHover:      controlHover,
		ControlPress:      controlPress,
		ControlEdge:       controlEdge,
		Track:             track,
		Field:             field,
		Opaque:            p.opaque,
		FieldFill:         p.fieldFill,
		LensRim:           scaleAlpha(p.lensRim, spec.LensScale),
		LensSheen:         scaleAlpha(p.lensSheen, spec.LensScale),
		LensEdge:          scaleAlpha(p.lensEdge, spec.LensScale),
		SheenTop:          scaleAlpha(p.sheenTop, sheen),
		SheenBottom:       scaleAlpha(p.sheenBottom, sheen),
		ShadowAmbient:     p.shadowAmbient,
		ShadowContact:     p.shadowContact,
		EdgeHighlight:     p.edgeHighlight,
		EdgeShadow:        p.edgeShadow,
		EdgeHighlightX:    p.edgeHighlightX,
		AccentEdge:        p.accentEdge,
		Hairline:          hairline,
		TextStrong:        textStrong,
		TextSecondary:     textSecondary,
		TextTertiary:      textTertiary,
	}
}

// palette is base.css's token set for one appearance, as the framework
// theme's fields plus the control-ladder rungs the views read by name.
// Everything here is the old build's value: the pastel accent, the
// translucent white (dark) or black (light) control ladder, the content
// recess's own colour and the near-solid stand-in.
func paletteFor(appearance Appearance) palette {
	if appearance == Light {
		return palette{
			background:     ui.Hex("#fafafc"),
			text:           ui.Hex("#1d1d1f"),
			textStrong:     ui.RGBA(23, 23, 26, 0.96),
			textMuted:      ui.RGBA(60, 60, 67, 0.78),
			textSecondary:  ui.RGBA(60, 60, 67, 0.86),
			textTertiary:   ui.RGBA(60, 60, 67, 0.72),
			accent:         ui.Hex("#064aa6"),
			accentHover:    ui.Hex("#0a5ecb"),
			accentPressed:  ui.Hex("#05408f"),
			accentText:     ui.Hex("#f7f9ff"),
			stroke:         ui.RGBA(0, 0, 0, 0.1),
			strokeActive:   ui.RGBA(10, 108, 255, 0.28),
			control:        ui.RGBA(0, 0, 0, 0.035),
			controlHover:   ui.RGBA(0, 0, 0, 0.06),
			controlPress:   ui.RGBA(0, 0, 0, 0.022),
			controlEdge:    ui.RGBA(0, 0, 0, 0.07),
			track:          ui.RGBA(0, 0, 0, 0.05),
			field:          ui.RGBA(255, 255, 255, 0.5),
			hairline:       ui.RGBA(0, 0, 0, 0.08),
			selection:      ui.RGBA(6, 74, 166, 0.1),
			focus:          ui.RGBA(6, 74, 166, 0.82),
			warning:        ui.Hex("#9c1f18"),
			scrollbar:      ui.RGBA(60, 60, 67, 0.24),
			opaque:         ui.RGBA(250, 250, 252, 0.995),
			fieldFill:      ui.RGBA(252, 252, 254, 0.997),
			lensRim:        ui.RGBA(255, 255, 255, 0.9),
			lensSheen:      ui.RGBA(255, 255, 255, 0.5),
			lensEdge:       ui.RGBA(0, 0, 0, 0.08),
			sheenTop:       ui.RGBA(255, 255, 255, 0.66),
			sheenBottom:    ui.RGBA(255, 255, 255, 0.22),
			shadowAmbient:  ui.RGBA(0, 0, 0, 0.18),
			shadowContact:  ui.RGBA(0, 0, 0, 0.12),
			edgeHighlight:  ui.RGBA(255, 255, 255, 0.92),
			edgeShadow:     ui.RGBA(0, 0, 0, 0.07),
			edgeHighlightX: ui.RGBA(255, 255, 255, 0.6),
			accentEdge:     ui.RGBA(6, 74, 166, 0.36),
		}
	}
	return palette{
		background:     ui.Hex("#121316"),
		text:           ui.Hex("#f2f3f5"),
		textStrong:     ui.RGBA(244, 245, 247, 0.96),
		textMuted:      ui.RGBA(238, 239, 242, 0.55),
		textSecondary:  ui.RGBA(238, 239, 242, 0.74),
		textTertiary:   ui.RGBA(238, 239, 242, 0.48),
		accent:         ui.Hex("#8fb7ff"),
		accentHover:    ui.Hex("#a9c8ff"),
		accentPressed:  ui.Hex("#7aa7ff"),
		accentText:     ui.Hex("#15171b"),
		stroke:         ui.RGBA(255, 255, 255, 0.08),
		strokeActive:   ui.RGBA(143, 183, 255, 0.24),
		control:        ui.RGBA(255, 255, 255, 0.055),
		controlHover:   ui.RGBA(255, 255, 255, 0.085),
		controlPress:   ui.RGBA(255, 255, 255, 0.038),
		controlEdge:    ui.RGBA(255, 255, 255, 0.06),
		track:          ui.RGBA(255, 255, 255, 0.07),
		field:          ui.RGBA(255, 255, 255, 0.03),
		hairline:       ui.RGBA(255, 255, 255, 0.07),
		selection:      ui.RGBA(143, 183, 255, 0.13),
		focus:          ui.RGBA(122, 167, 255, 0.78),
		warning:        ui.RGBA(255, 178, 164, 0.82),
		scrollbar:      ui.RGBA(238, 239, 242, 0.15),
		opaque:         ui.RGBA(26, 27, 30, 0.99),
		fieldFill:      ui.RGBA(33, 34, 37, 0.99),
		lensRim:        ui.RGBA(255, 255, 255, 0.15),
		lensSheen:      ui.RGBA(255, 255, 255, 0.06),
		lensEdge:       ui.RGBA(0, 0, 0, 0.15),
		sheenTop:       ui.RGBA(255, 255, 255, 0.046),
		sheenBottom:    ui.RGBA(255, 255, 255, 0.01),
		shadowAmbient:  ui.RGBA(0, 0, 0, 0.3),
		shadowContact:  ui.RGBA(0, 0, 0, 0.2),
		edgeHighlight:  ui.RGBA(255, 255, 255, 0.15),
		edgeShadow:     ui.RGBA(0, 0, 0, 0.28),
		edgeHighlightX: ui.RGBA(255, 255, 255, 0.04),
		accentEdge:     ui.RGBA(143, 183, 255, 0.3),
	}
}

// palette is one appearance's token set, kept private: Tokens is the public
// shape, and the framework theme's fields are filled from here.
type palette struct {
	background, text, textStrong, textMuted, textSecondary, textTertiary ui.Color
	accent, accentHover, accentPressed, accentText                       ui.Color
	stroke, strokeActive                                                 ui.Color
	control, controlHover, controlPress, controlEdge                     ui.Color
	track, field, hairline, selection, focus, warning, scrollbar         ui.Color
	opaque, fieldFill                                                    ui.Color
	lensRim, lensSheen, lensEdge                                         ui.Color
	sheenTop, sheenBottom                                                ui.Color
	shadowAmbient, shadowContact                                         ui.Color
	edgeHighlight, edgeShadow, edgeHighlightX                            ui.Color
	accentEdge                                                           ui.Color
}

// round1 rounds to one decimal, so a scaled metric stays a value a CSS
// author could have written and the tests can compare exactly.
func round1(v float32) float32 {
	return float32(int(v*10+0.5)) / 10
}

// scaleAlpha multiplies a token's own alpha by a factor, keeping its colour.
// The card's sheen is a fixed highlight in base.css, but a fixed highlight
// over a thin frame props the whole background to a density the transparency
// slider cannot move - the light appearance's 0.66 sheen left the bottom at
// 0.81 at the slider's low end. Scaling it by the frame's own tint makes the
// highlight fade with the face, so the slider owns the background again.
func scaleAlpha(c ui.Color, factor float32) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, float32(c.A)/255*factor)
}

// blendOpaque lays over on base at over's alpha and returns the opaque
// result, for the one lift that has to cover a color the framework paints
// behind it.
func blendOpaque(base, over ui.Color) ui.Color {
	a := float64(over.A) / 255
	mix := func(b, o uint8) uint8 {
		return uint8(math.Round(float64(b)*(1-a) + float64(o)*a))
	}
	return ui.RGBA(mix(base.R, over.R), mix(base.G, over.G), mix(base.B, over.B), 1)
}

// Lens draws the control lens on a pane: the top rim, the broad sheen and the
// bottom shade, then the shared pane edge — base.css's `--glass-lens-stack`,
// which every control on a surface carried as its `--elev-0`. The framework
// leaves a control as a flat tint plus a border; these four inset layers are
// what make the same box read as a pane of glass. They stack in the order
// added, so the rim lands last, over the sheen's fade, as the old rule's
// declaration order had it.
func (t Tokens) Lens(e ui.Element) ui.Element {
	e.InsetShadow(0, 1, 0, 0, t.LensRim)
	e.InsetShadow(0, 6, 10, -6, t.LensSheen)
	e.InsetShadow(0, -1, 0, 0, t.LensEdge)
	e.InsetShadow(0, 0, 0, 1, t.ControlEdge)
	return e
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
