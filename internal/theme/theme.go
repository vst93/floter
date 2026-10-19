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
	"runtime"

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

	// WindowsWindowRadius is the window's own corner on Windows: DWM rounds
	// every window at its ~8px preset (`DWMWCP_ROUND`), so base.css pins
	// `--window-radius` to that same figure there (`base.css:1492`) instead of
	// the 14px the other two platforms use (`base.css:66`). The card draws
	// *inward* from the window's own edge, so a 14px card corner inside an 8px
	// window corner leaves the DWM background showing as a crescent at each
	// corner — one of the pieces the user read as 「窗口周边外圈还有一圈额外的
	// 底色」. See windowRadiusFor.
	WindowsWindowRadius = 8

	// BodyUnits is the body text size in this codebase's own unit, `--text-body`
	// one rung up: 12px at scale 1. Every size in this file's unit (a
	// `Space(n)` is four of them), so the framework theme's point size is set
	// from it rather than from the framework's own default.
	//
	// The reference's own register is 11px *at its scale 1*; the shipped
	// interface step is `small` (0.9), so the ladder is written one rung up and
	// the shipped step lands on the reference's physical sizes (see the ladder
	// in ForPlatform). Reading the design off the shipped step instead means
	// every user sees the design at its own size without first finding
	// 「界面大小」 — which is what made the whole app read as 小气.
	BodyUnits = 12
)

// windowRadiusFor is the corner a surface that *is* the window draws. Windows
// is the one platform whose window is rounded by the system at its own preset,
// and a card whose corner is larger than the window's shows the system's frame
// color in the four crescents between the two arcs (see
// WindowsWindowRadius). macOS's 14px is the window server's own corner there
// and Linux's is the compositor's; the card matches it. It takes the platform
// as a parameter so all three answers are one table a test reads on any host.
func windowRadiusFor(goos string) float32 {
	if goos == "windows" {
		return WindowsWindowRadius
	}
	return RadiusLG
}

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
	// RadiusLG is the window or a full panel's corner off Windows; on Windows
	// the window itself is what decides the corner, and that is WindowRadius
	// (see WindowsWindowRadius).
	RadiusLG float32
	// WindowRadius is the corner a surface that *is* the window draws: the
	// launcher card, the settings card and the terminal panel. It is RadiusLG
	// everywhere but Windows, which rounds the window at 8px.
	WindowRadius float32

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
	// (base.css's `--glass-track`), TrackEdge the quieter hairline that groove
	// draws on its recessed lens (`--glass-track-edge`), and Field the slot a
	// text field cuts into the glass (`--glass-field`).
	Track     ui.Color
	TrackEdge ui.Color
	Field     ui.Color
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
	// glass a lit face rather than a flat tint. They are the reference's
	// **fixed** tokens — the same on every effect stop, and dropped by `off` —
	// capped at the frame's own alpha. The fill under them is `FaceFill`'s
	// compensated alpha, so the two composite back to the frame's own alpha
	// (see `FaceFill`).
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
	// accent fill. AccentHalo is the accent as a soft ring (base.css's
	// `--accent-halo`): the one non-elevation shadow in the sheets, the live
	// status dot's glow (`--elev-halo` = `0 0 0 2px var(--accent-halo)`), the
	// accent at a fixed low alpha so it never reads as a second accent fill.
	AccentEdge ui.Color
	// AccentEdgeStrong is the heavier accent keyline (base.css's
	// `--accent-edge-strong`): the 1px edge the *primary* action pill carries
	// over its accent fill (`.extensions-action-button--primary`, its
	// `--accent-edge-strong` border), where `--accent-edge` is the keyline a
	// chosen slot wears without a fill. One accent, two weights: the fill's
	// edge is the strong one so the pill reads as a control, not as a tint.
	AccentEdgeStrong ui.Color
	AccentHalo       ui.Color
	// AccentTintQuiet is the accent at the quietest rung of its tint family
	// (base.css's `--accent-tint-quiet`): the fill a launcher result row's
	// selection wears, where the emphasis is the row's own type — the title at
	// 700 — rather than the fill. It is one rung below `--accent-tint` and it
	// is a *per-appearance* token (dark 0.085, light 0.065): the same pastel
	// blue over near-black gains contrast, while the same alpha over white
	// barely moves the pixel, so the light appearance needs the lighter rung.
	// Quoting one number for both appearances painted the light row's
	// selection with the dark rung (the launcher's `Accent.Alpha(0.085)`).
	AccentTintQuiet ui.Color
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
	// RaisedWarm is the warm branch of the raised pane (base.css's
	// `--glass-raised-warm`, an alias of `--system-icon-surface`): the amber
	// fill a broken integration row paints, where the accent's blue would read
	// as "this opens something" rather than as a warning.
	RaisedWarm ui.Color
	// Float is a floating pane's own fill (base.css's `--glass-float`): the
	// frame's tint plus a fixed step, so a drawer, a dialog, a toast or the
	// terminal's resident note is always denser than the card it floats over.
	Float ui.Color
	// IconSurface and IconSurfaceHover are the warm system-icon plane (base.css's
	// `--system-icon-surface` / `--system-icon-surface-hover`): the amber tint a
	// semantic icon chip sits on. `--glass-raised-warm` is an alias of the first,
	// which is what the launcher's own feedback row and the settings sheet's
	// danger pills paint — it is a warm *surface*, not the warning ink, and
	// quoting the warning colour's alpha here painted the row's fill with the
	// label's own hue (the row read pink-red instead of warm orange).
	IconSurface      ui.Color
	IconSurfaceHover ui.Color
	// SystemIconText is the ink a warm system-icon plane carries (base.css's
	// `--system-icon-text`): the amber ink the launcher's power rows draw their
	// glyph in. It is a token of its own, not the warning ink: the dark
	// appearance reads it as a pale warm peach and the light one as a burnt
	// orange (`#a24d15`), while the warning colour is a near-neutral red-grey
	// reserved for a *label*. The port painted the warm plate's glyph with
	// `t.Warning`, which is the wrong hue on both appearances.
	SystemIconText ui.Color

	// The text ladder, in points at the current step: base.css's registers,
	// which the codebase had been spelling as offsets off the body size (a
	// +6 that is 6.6 points at the Large step, not 6).
	Caption  float32
	Body     float32
	Emphasis float32
	Title    float32
	Display  float32
}

// For resolves the appearance and scales the metrics for a settings value,
// assuming a desktop with no preferences of its own. It is what a render
// without a desktop context uses (the segmented-control tests); a caller with
// a `*ui.Context` should use ForPreferences, because the desktop's contrast
// preference lives there and the old build's `prefers-contrast: more` block
// is a real part of the look.
func For(s settings.Settings, systemDark bool) Tokens {
	return ForPreferences(s, systemDark, ui.Preferences{})
}

// ForPreferences resolves the appearance and scales the metrics for a settings
// value and the desktop's preferences. The widgets' font, spacing and radius
// all follow the interface-size step, as the old build's CSS variables did,
// and a desktop that asked for more contrast gets base.css's
// `prefers-contrast: more` tokens: the stronger strokes and the readable
// placeholder, plus the material changes (glassmap.Spec.Contrast).
//
// It is ForPlatform for the window this build is running in: the glass fill is
// compensated for the platform's own material (glassmap.MaterialVeil), so the
// tokens of a macOS window and a Linux one are not the same numbers. A caller
// that wants the other platform's look — a test reading all three tables on one
// host — asks ForPlatform directly.
func ForPreferences(s settings.Settings, systemDark bool, prefs ui.Preferences) Tokens {
	return ForPlatform(s, systemDark, prefs, runtime.GOOS)
}

// ForPlatform is ForPreferences for a named platform (the `goos` strings the
// rest of the build uses). The platform reaches the look in exactly one place:
// the glass spec, whose fill is the transparency slider less the veil the
// window's material already contributes (glassmap.SpecForPreferencesOS).
func ForPlatform(s settings.Settings, systemDark bool, prefs ui.Preferences, goos string) Tokens {
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
	if prefs.HighContrast {
		p = p.contrast(appearance)
	}
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
	// base.css writes the whole radius ladder in literal px (`--radius-xs:
	// 6px; --radius-sm: 9px; --radius-md: 12px; --radius-lg: 14px`,
	// base.css:93-96), so a corner does not follow `--ui-scale` the way a
	// font or a `--u` metric does: the ladder is the one set of numbers the
	// old build kept fixed while the interface grew. See `Tokens.RadiusSM`.
	t.Radius = RadiusSM

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
	spec := glassmap.SpecForPreferencesOS(goos, s.GlassStep, s.MainOpacity, prefs.HighContrast)
	sunken := spec.ContentAlpha()
	floatAlpha := spec.FloatAlpha()
	// The panel's sheen is the glass's lit face. The off stop has no glass, so
	// it paints the plain tint with no sheen - base.css's `[data-glass="off"]
	// .shell` dropped the gradient along with the material. The effect stops
	// paint the reference's **fixed** `--sheen-top` / `--sheen-bottom`
	// (base.css:628-629 dark 0.046/0.01, :906-907 light 0.66/0.22): those
	// tokens do not vary with `[data-glass]` — the only rung a stop moves on
	// the frame's own face is nothing at all; `--glass-lens-scale` moves the
	// **control** lens (LensRim/LensSheen/LensEdge below), not the card. The
	// three stops therefore read apart on their controls (P7-em) and share one
	// lit face; grading the sheen by the stop was this build's own invention
	// from when the stops had nothing else to show on Linux.
	//
	// The sheen is still capped at the frame's own alpha: a sheen heavier than
	// the frame would make the top of a very thin frame denser than the slider
	// asked for, and `FaceFill`'s compensation cannot ask for a negative fill
	// (see `baseAlpha`). The cap is what keeps the frame's *alpha* the slider's
	// on every stop — `FaceFill` gives the tint the alpha that composites with
	// the sheen back to the slider's, so the fixed sheen changes the lit face's
	// colour, not the fill's truth (the sheen used to sit over the frame's tint
	// and add its own alpha — 「开玻璃透明底失效」).
	sheenTop, sheenBottom := ui.Color{}, ui.Color{}
	if spec.Glass {
		sheenTop = capAlpha(p.sheenTop, spec.TintAlpha)
		sheenBottom = capAlpha(p.sheenBottom, spec.TintAlpha)
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

	// The ladder, at the step: the reference's own registers (caption 10, body
	// 11, emphasis 12, title 13, display 17) one rung up (11/12/13/14/19), each
	// scaled once here rather than spelled as an offset wherever a register is
	// needed.
	//
	// One rung up is what makes the **shipped** step the reference's own sizes:
	// `small` is 0.9, and 11/12/13/14/19 x 0.9 = 9.9/10.8/11.7/12.6/17.1, i.e.
	// base.css's 10/11/12/13/17 within a tenth of a point. The ladder used to
	// be the base numbers themselves, so the shipped step drew the design at
	// 90% — a 9pt caption, a 10.8pt emphasis, a 15.3pt display — and every
	// surface read cramped against the window it sat in (「整体 UI/UX 很
	// 小气」). A step above the shipped one now draws *larger* than the
	// reference rather than the reference shrinking below it: the ladder is a
	// named size, the step is the user's magnification of it.
	caption := round1(11 * f)
	body := round1(12 * f)
	emphasis := round1(13 * f)
	title := round1(14 * f)
	display := round1(19 * f)

	return Tokens{
		Appearance:  appearance,
		Theme:       &t,
		Sunken:      baseFill.Alpha(sunken),
		RaisedQuiet: raisedQuiet,
		RaisedWarm:  p.iconSurface,
		Float:       floatFill.Alpha(floatAlpha),
		Caption:     caption,
		Body:        body,
		Emphasis:    emphasis,
		Title:       title,
		Display:     display,
		Scale:       scale,
		// The radius ladder is literal px, as base.css wrote it: a corner is
		// not a `--u` metric and does not move with the interface step.
		RadiusXS:          RadiusXS,
		RadiusSM:          RadiusSM,
		RadiusMD:          RadiusMD,
		RadiusLG:          RadiusLG,
		WindowRadius:      windowRadiusFor(goos),
		InputStroke:       stroke,
		InputStrokeActive: strokeActive,
		Control:           control,
		ControlHover:      controlHover,
		ControlPress:      controlPress,
		ControlEdge:       controlEdge,
		Track:             track,
		TrackEdge:         p.trackEdge,
		Field:             field,
		Opaque:            p.opaque,
		FieldFill:         p.fieldFill,
		LensRim:           scaleAlpha(p.lensRim, spec.LensScale),
		LensSheen:         scaleAlpha(p.lensSheen, spec.LensScale),
		LensEdge:          scaleAlpha(p.lensEdge, spec.LensScale),
		// The frame's lit face is the reference's fixed token, capped at the
		// frame's own alpha (see the `sheenTop` note above); the stop does not
		// grade it.
		SheenTop:         sheenTop,
		SheenBottom:      sheenBottom,
		ShadowAmbient:    p.shadowAmbient,
		ShadowContact:    p.shadowContact,
		EdgeHighlight:    p.edgeHighlight,
		EdgeShadow:       p.edgeShadow,
		EdgeHighlightX:   p.edgeHighlightX,
		AccentEdge:       p.accentEdge,
		AccentEdgeStrong: p.accentEdgeStrong,
		AccentHalo:       p.accentHalo,
		AccentTintQuiet:  p.accentTintQuiet,
		Hairline:         hairline,
		TextStrong:       textStrong,
		TextSecondary:    textSecondary,
		TextTertiary:     textTertiary,
		// The warm plane is a fixed per-appearance token in base.css — unlike
		// the frame's tint it does not follow the transparency slider.
		IconSurface:      p.iconSurface,
		IconSurfaceHover: p.iconSurfaceHover,
		SystemIconText:   p.systemIconText,
	}
}

// palette is base.css's token set for one appearance, as the framework
// theme's fields plus the control-ladder rungs the views read by name.
// Everything here is the old build's value: the pastel accent, the
// translucent white (dark) or black (light) control ladder, the content
// recess's own colour and the near-solid stand-in.
//
// The surface ladder is the one set of numbers this build raised over the
// reference's: `stroke` > `controlEdge` > `hairline` > `control`, the four
// rungs that say a card, its rows and the planes on them are different depths.
// The old build drew all of them under a 0.08 alpha (dark: 0.08/0.06/0.07/
// 0.055), which over a translucent dark frame is one or two levels of grey —
// below what a 1x display resolves — so a card was a card only because of its
// shadow, its rows had no rule and its controls no edge: every surface read as
// one flat pane, and the window read as pinched however generous its padding
// was (「整体 UI/UX 很 小气」). The rungs keep their order and their ratios, one
// step brighter: the card's edge clears the row's hairline, the hairline clears
// a control plane. They stay light-handed — 0.1 white over a dark frame is a
// quarter of the contrast the contrast preference's own tokens use
// (`contrast()`'s 0.42/0.34) — so the ladder buys depth without becoming a
// border-on-every-box design.
func paletteFor(appearance Appearance) palette {
	if appearance == Light {
		return palette{
			background:    ui.Hex("#fafafc"),
			text:          ui.Hex("#1d1d1f"),
			textStrong:    ui.RGBA(23, 23, 26, 0.96),
			textMuted:     ui.RGBA(60, 60, 67, 0.78),
			textSecondary: ui.RGBA(60, 60, 67, 0.86),
			textTertiary:  ui.RGBA(60, 60, 67, 0.72),
			accent:        ui.Hex("#064aa6"),
			accentHover:   ui.Hex("#0a5ecb"),
			accentPressed: ui.Hex("#05408f"),
			accentText:    ui.Hex("#f7f9ff"),
			stroke:        ui.RGBA(0, 0, 0, 0.12),
			strokeActive:  ui.RGBA(10, 108, 255, 0.28),
			control:       ui.RGBA(0, 0, 0, 0.05),
			controlHover:  ui.RGBA(0, 0, 0, 0.075),
			controlPress:  ui.RGBA(0, 0, 0, 0.032),
			controlEdge:   ui.RGBA(0, 0, 0, 0.1),
			track:         ui.RGBA(0, 0, 0, 0.06),
			trackEdge:     ui.RGBA(0, 0, 0, 0.08),
			field:         ui.RGBA(255, 255, 255, 0.5),
			// One step under the control edge, as the dark palette's is: the
			// ladder says a row's hairline is *quieter* than a control's edge
			// (`stroke > controlEdge > hairline > control`), and the light
			// palette had the two at the same 0.1, so a row rule and a control
			// edge drew the same line — the last rung of the ladder with no
			// step in it.
			hairline:         ui.RGBA(0, 0, 0, 0.09),
			selection:        ui.RGBA(6, 74, 166, 0.1),
			focus:            ui.RGBA(6, 74, 166, 0.82),
			warning:          ui.Hex("#9c1f18"),
			scrollbar:        ui.RGBA(60, 60, 67, 0.24),
			opaque:           ui.RGBA(250, 250, 252, 0.995),
			fieldFill:        ui.RGBA(252, 252, 254, 0.997),
			lensRim:          ui.RGBA(255, 255, 255, 0.9),
			lensSheen:        ui.RGBA(255, 255, 255, 0.5),
			lensEdge:         ui.RGBA(0, 0, 0, 0.08),
			sheenTop:         ui.RGBA(255, 255, 255, 0.66),
			sheenBottom:      ui.RGBA(255, 255, 255, 0.22),
			shadowAmbient:    ui.RGBA(0, 0, 0, 0.18),
			shadowContact:    ui.RGBA(0, 0, 0, 0.12),
			edgeHighlight:    ui.RGBA(255, 255, 255, 0.92),
			edgeShadow:       ui.RGBA(0, 0, 0, 0.07),
			edgeHighlightX:   ui.RGBA(255, 255, 255, 0.6),
			accentEdge:       ui.RGBA(6, 74, 166, 0.36),
			accentEdgeStrong: ui.RGBA(6, 74, 166, 0.68),
			accentHalo:       ui.RGBA(6, 74, 166, 0.24),
			// The launcher selection's own rung: the light appearance steps the
			// tint down (base.css `--accent-tint-quiet: rgba(6, 74, 166, 0.065)`)
			// because the dark appearance's 0.085 reads heavier over white.
			accentTintQuiet: ui.RGBA(6, 74, 166, 0.065),
			// The warm system-icon plane, the light appearance's amber (base.css
			// `--system-icon-surface` / `--system-icon-surface-hover`).
			iconSurface:      ui.RGBA(214, 118, 44, 0.13),
			iconSurfaceHover: ui.RGBA(214, 118, 44, 0.21),
			systemIconText:   ui.Hex("#a24d15"),
		}
	}
	return palette{
		background:       ui.Hex("#121316"),
		text:             ui.Hex("#f2f3f5"),
		textStrong:       ui.RGBA(244, 245, 247, 0.96),
		textMuted:        ui.RGBA(238, 239, 242, 0.55),
		textSecondary:    ui.RGBA(238, 239, 242, 0.74),
		textTertiary:     ui.RGBA(238, 239, 242, 0.48),
		accent:           ui.Hex("#8fb7ff"),
		accentHover:      ui.Hex("#a9c8ff"),
		accentPressed:    ui.Hex("#7aa7ff"),
		accentText:       ui.Hex("#15171b"),
		stroke:           ui.RGBA(255, 255, 255, 0.11),
		strokeActive:     ui.RGBA(143, 183, 255, 0.24),
		control:          ui.RGBA(255, 255, 255, 0.075),
		controlHover:     ui.RGBA(255, 255, 255, 0.105),
		controlPress:     ui.RGBA(255, 255, 255, 0.05),
		controlEdge:      ui.RGBA(255, 255, 255, 0.1),
		track:            ui.RGBA(255, 255, 255, 0.09),
		trackEdge:        ui.RGBA(255, 255, 255, 0.075),
		field:            ui.RGBA(255, 255, 255, 0.03),
		hairline:         ui.RGBA(255, 255, 255, 0.09),
		selection:        ui.RGBA(143, 183, 255, 0.13),
		focus:            ui.RGBA(122, 167, 255, 0.78),
		warning:          ui.RGBA(255, 178, 164, 0.82),
		scrollbar:        ui.RGBA(238, 239, 242, 0.15),
		opaque:           ui.RGBA(26, 27, 30, 0.99),
		fieldFill:        ui.RGBA(33, 34, 37, 0.99),
		lensRim:          ui.RGBA(255, 255, 255, 0.15),
		lensSheen:        ui.RGBA(255, 255, 255, 0.06),
		lensEdge:         ui.RGBA(0, 0, 0, 0.15),
		sheenTop:         ui.RGBA(255, 255, 255, 0.046),
		sheenBottom:      ui.RGBA(255, 255, 255, 0.01),
		shadowAmbient:    ui.RGBA(0, 0, 0, 0.3),
		shadowContact:    ui.RGBA(0, 0, 0, 0.2),
		edgeHighlight:    ui.RGBA(255, 255, 255, 0.15),
		edgeShadow:       ui.RGBA(0, 0, 0, 0.28),
		edgeHighlightX:   ui.RGBA(255, 255, 255, 0.04),
		accentEdge:       ui.RGBA(143, 183, 255, 0.3),
		accentEdgeStrong: ui.RGBA(143, 183, 255, 0.72),
		accentHalo:       ui.RGBA(143, 183, 255, 0.18),
		// The launcher selection's own rung (base.css `--accent-tint-quiet:
		// rgba(143, 183, 255, 0.085)`).
		accentTintQuiet: ui.RGBA(143, 183, 255, 0.085),
		// The warm system-icon plane, the dark appearance's amber (base.css
		// `--system-icon-surface` / `--system-icon-surface-hover`), and its
		// own ink (`--system-icon-text`): a pale warm peach, not the warning
		// red-grey.
		iconSurface:      ui.RGBA(255, 176, 132, 0.115),
		iconSurfaceHover: ui.RGBA(255, 176, 132, 0.2),
		systemIconText:   ui.RGBA(255, 209, 178, 0.82),
	}
}

// contrast returns the palette a desktop that asked for more contrast gets:
// base.css's `@media (prefers-contrast: more)` block, which swaps three
// tokens per appearance. `--stroke-contrast` is the card and floating-chrome
// border (the old build's `.collapsed-card`, `.settings-card`,
// `.extensions-drawer`, `.app-toast` …), `--stroke-contrast-soft` the inner
// edge of a grouped row (`.settings-option`, `.settings-section__card`), and
// `--text-placeholder-contrast` the field placeholder.
//
// The placeholder token lands on textMuted, which is the one lever the
// framework exposes for it: its editor paints a placeholder in
// `LineNumber`, and it derives that as `TextMuted.Alpha(0.75)`
// (mygo's ui/editor/theme.go). The same step also strengthens every quiet
// label one rung, which is what the old build's `color: var(--text-primary)`
// on the toast/notice copies asked for.
func (p palette) contrast(app Appearance) palette {
	if app == Light {
		p.stroke = ui.RGBA(0, 0, 0, 0.5)
		p.controlEdge = ui.RGBA(0, 0, 0, 0.36)
		p.textMuted = ui.RGBA(28, 28, 32, 0.68)
	} else {
		p.stroke = ui.RGBA(255, 255, 255, 0.42)
		p.controlEdge = ui.RGBA(255, 255, 255, 0.34)
		p.textMuted = ui.RGBA(242, 243, 245, 0.58)
	}
	return p
}

// palette is one appearance's token set, kept private: Tokens is the public
// shape, and the framework theme's fields are filled from here.
type palette struct {
	background, text, textStrong, textMuted, textSecondary, textTertiary    ui.Color
	accent, accentHover, accentPressed, accentText                          ui.Color
	stroke, strokeActive                                                    ui.Color
	control, controlHover, controlPress, controlEdge                        ui.Color
	track, trackEdge, field, hairline, selection, focus, warning, scrollbar ui.Color
	opaque, fieldFill                                                       ui.Color
	lensRim, lensSheen, lensEdge                                            ui.Color
	sheenTop, sheenBottom                                                   ui.Color
	shadowAmbient, shadowContact                                            ui.Color
	edgeHighlight, edgeShadow, edgeHighlightX                               ui.Color
	accentEdge                                                              ui.Color
	accentEdgeStrong                                                        ui.Color
	accentHalo                                                              ui.Color
	accentTintQuiet                                                         ui.Color
	iconSurface, iconSurfaceHover                                           ui.Color
	systemIconText                                                          ui.Color
}

// round1 rounds to one decimal, so a scaled metric stays a value a CSS
// author could have written and the tests can compare exactly.
func round1(v float32) float32 {
	return float32(int(v*10+0.5)) / 10
}

// scaleAlpha multiplies a token's own alpha by a factor, keeping its colour.
// It is the `--glass-lens-scale` rung: base.css writes every lens layer as
// `calc(var(--glass-lens-*-base) * var(--glass-lens-scale))`, so the stop
// grades the control lens and nothing else.
func scaleAlpha(c ui.Color, factor float32) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, float32(c.A)/255*factor)
}

// capAlpha lowers a colour's alpha to at most max, keeping its colour: the
// sheen may not be heavier than the frame it sits on, or a thin frame's top
// would composite denser than the slider's own alpha.
func capAlpha(c ui.Color, max float32) ui.Color {
	if a := float32(c.A) / 255; a > max {
		return ui.RGBA(c.R, c.G, c.B, max)
	}
	return c
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

// FaceFill is the card's face at the frame's own alpha: the tint colour as the
// two stops of its top-down gradient, at the alpha that **composites with the
// stop's sheen back to the frame's alpha**. The sheen is painted over it as
// the second layer (`Tokens.SheenTop` / `SheenBottom`, `View`), and the base
// is chosen so `1-(1-base)(1-sheen) == alpha`: a plain tint under a sheen
// layer would composite to `1-(1-alpha)(1-sheen)`, denser than the slider's
// own alpha — in the light appearance up to +0.11 in the result band at the
// default 47%, which is what the user read as 「开玻璃透明底失效」. Here the
// frame's alpha is the transparency slider's on every stop, while the stop
// still grades the lit face's colour (the sheen the scene paints itself).
func (t Tokens) FaceFill(alpha float32) (top, bottom ui.Color) {
	bg := ui.RGB(18, 19, 22)
	if t.Theme != nil {
		bg = t.Theme.Background
	}
	sheenTop, sheenBottom := t.SheenAt(alpha)
	return ui.RGBA(bg.R, bg.G, bg.B, baseAlpha(alpha, sheenTop)),
		ui.RGBA(bg.R, bg.G, bg.B, baseAlpha(alpha, sheenBottom))
}

// SheenAt is the frame's lit face capped at `alpha`, the alpha the face is
// actually painted at: the layer FaceFill compensates for. The tokens are
// already capped at the *frame's* alpha (see the `sheenTop` note in
// ForPlatform), which is the right cap for the window's own card; a surface
// whose face carries a thinner alpha — the terminal's, which shares the
// terminal slider with its panel (glassmap.Spec.TerminalFrameAlpha) — has to
// cap it again, or a light appearance's 0.66 sheen would paint the whole face
// at its own alpha and the frame would be denser than the slider asked for.
func (t Tokens) SheenAt(alpha float32) (top, bottom ui.Color) {
	return capAlpha(t.SheenTop, alpha), capAlpha(t.SheenBottom, alpha)
}

// baseAlpha is the fill alpha a sheen of alpha s composites over to frame:
// `1-(1-base)(1-s) == frame`, clamped so a sheen heavier than the frame at the
// slider's bottom end cannot ask for a negative fill.
func baseAlpha(frame float32, sheen ui.Color) float32 {
	s := float32(sheen.A) / 255
	if s >= 1 {
		return 0
	}
	base := (frame - s) / (1 - s)
	if base < 0 {
		return 0
	}
	if base > frame {
		return frame
	}
	return base
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

// LensInset draws the recessed lens on a groove: base.css's
// `--glass-lens-stack-inset`, the *same* lens read backwards. The light pools
// along the bottom edge of the groove and the top edge is the dark one — the
// exact opposite of Lens — with the inner slot shadow crouched under the top
// edge and the track's own quieter hairline closing the loop. It is what makes
// a switch or a segmented control's track read as a recess in the same sheet
// of glass the raised panes sit on, rather than as a flat darker box, and
// `--glass-lens-scale` grades it with the glass stop exactly as Lens is.
//
// The layers are added last-on-top (see `InsetShadow`), so the lit rim goes in
// last: base.css lists it first, and CSS paints the first shadow on top.
func (t Tokens) LensInset(e ui.Element) ui.Element {
	e.InsetShadow(0, 0, 0, 1, t.TrackEdge)
	e.InsetShadow(0, 1, 0, 0, t.LensEdge)
	e.InsetShadow(0, 1, 2, 0, t.ShadowContact)
	e.InsetShadow(0, -1, 0, 0, t.LensRim)
	return e
}

// FieldSlot draws the inset a text or number field is cut into the glass with:
// base.css's `--glass-field-shadow` (`:524-527` dark, `:887-888` light) — the
// window's contact inset crouched under the top edge plus the lens's own rim
// (and, on dark glass, its bottom shade). It is the one layer the plugin
// configuration sheet's 28u controls (`.plugin-config-text` /
// `.plugin-config-number`, `plugin-config.css:97-118`) carry that a bare
// `--glass-field` fill does not: without it the slot reads as a flat brighter
// pane rather than as a slot cut *into* the glass. The stack consumes the lens
// family, so `[data-glass]` grades it exactly as it grades Lens and LensInset.
//
// The layers append one at a time through InsetShadow, so the first layer
// lands under the later ones; the reference lists the contact shadow first and
// CSS paints the first shadow on top, so it goes in last — the same reversal
// LensInset does. Light glass has no bottom shade in its field shadow, and its
// rim is a fixed near-opaque white rather than the graded lens token.
func (t Tokens) FieldSlot(e ui.Element) ui.Element {
	if t.Appearance == Light {
		e.InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.85))
		e.InsetShadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))
		return e
	}
	e.InsetShadow(0, -1, 0, 0, t.LensEdge)
	e.InsetShadow(0, 1, 0, 0, t.LensRim)
	e.InsetShadow(0, 1, 2, 0, t.ShadowContact)
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
