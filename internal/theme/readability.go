package theme

import (
	"math"

	"floter/internal/glassmap"
	"github.com/egoist/mygo/ui"
)

// Background returns the neutral canvas color used by this appearance.
func Background(appearance Appearance) ui.Color { return paletteFor(appearance).background }

// readable bounds text contrast against both desktop extremes, including
// the frame's sheen and the surfaces on which labels actually appear. The
// native material is deliberately not credited: its tint is OS dependent.
// Colors are stable while moving the window; only theme/opacity changes
// rebuild them through Memo.
func (p palette) readable(app Appearance, spec glassmap.Spec, highContrast bool) palette {
	// Thin glass needs quieter white washes, otherwise a hovered control
	// cancels some of the dark backing that makes its label readable.
	if app == Dark {
		strength := float32(0.675) + 0.325*max(0, spec.Composite()-glassmap.ReadableFloor)/(glassmap.SolidTop-glassmap.ReadableFloor)
		p.control = scaleAlpha(p.control, strength)
		p.controlHover = scaleAlpha(p.controlHover, strength)
		p.rowHover = scaleAlpha(p.rowHover, strength)
		p.rowSelected = scaleAlpha(p.rowSelected, strength)
		p.rowPressed = scaleAlpha(p.rowPressed, strength)
		p.thumb = scaleAlpha(p.thumb, strength)
	}
	backs := p.readabilityBackdrops(app, spec)
	// All backgrounds stay on the same side of the text's luminance. Only
	// the darkest light surface / brightest dark surface can set the limit.
	// Resolve it once instead of testing every surface in every search step.
	worst := backs[0]
	for _, bg := range backs[1:] {
		if (app == Dark && srgbLuminance(bg) > srgbLuminance(worst)) ||
			(app == Light && srgbLuminance(bg) < srgbLuminance(worst)) {
			worst = bg
		}
	}
	backs = []ui.Color{worst}
	p.textMuted = p.textMuted.Over(p.background)
	toward := ui.RGB(0, 0, 0)
	if app == Dark {
		toward = ui.RGB(255, 255, 255)
	}
	boost := float64(0)
	if highContrast {
		boost = 1
	}
	p.text = readableInk(p.text, toward, 6+boost, backs)
	p.textStrong = p.text
	p.textSecondary = readableInk(p.textSecondary, toward, 5+boost, backs)
	p.textMuted = readableInk(p.textMuted, toward, 4.7+boost, backs)
	p.textTertiary = readableInk(p.textTertiary, toward, 4.5+boost, backs)
	p.accent = readableInk(p.accent, toward, 5.5, backs)
	// Hover/pressed are opaque button fills with AccentText on top. Keep
	// distinct feedback after adapting the accent used for text and icons.
	p.accentHover = p.accent.Mix(ui.RGB(255, 255, 255), 0.12)
	p.accentPressed = p.accent.Mix(ui.RGB(0, 0, 0), 0.12)
	p.warning = readableInk(p.warning, toward, 5.5, backs)
	p.systemIconText = readableInk(p.systemIconText, toward, 4.5, backs)
	return p
}

func (p palette) readabilityBackdrops(app Appearance, spec glassmap.Spec) []ui.Color {
	sunken := ui.RGB(17, 18, 20)
	if app == Light {
		sunken = ui.RGB(243, 244, 247)
	}
	var backs []ui.Color
	for _, desktop := range []ui.Color{ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)} {
		for _, sheen := range []ui.Color{{}, p.sheenTop, p.sheenBottom, p.sheenTopActive, p.sheenBottomActive} {
			if !spec.Glass {
				sheen = ui.Color{}
			}
			face := p.background.Alpha(baseAlpha(spec.TintAlpha, sheen)).Over(desktop)
			face = sheen.Over(face)
			pane := sunken.Alpha(spec.ContentAlpha()).Over(face)
			backs = append(backs, face, p.controlHover.Over(face), p.rowPressed.Over(pane),
				p.thumb.Over(p.control.Over(pane)), p.keycapFace.Over(pane), p.selection.Over(pane))
		}
	}
	return backs
}

// readableInk makes the smallest move toward the appearance's ink extreme.
// The original hue is retained by interpolation; text itself stays opaque.
func readableInk(ink, toward ui.Color, ratio float64, backs []ui.Color) ui.Color {
	ink.A = 255
	passes := func(c ui.Color) bool {
		for _, bg := range backs {
			if ContrastRatio(c, bg) < ratio {
				return false
			}
		}
		return true
	}
	if passes(ink) {
		return ink
	}
	lo, hi := float32(0), float32(1)
	for range 12 {
		mid := (lo + hi) / 2
		if passes(ink.Mix(toward, mid)) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return ink.Mix(toward, hi)
}

// ReadableOnGlass adapts default terminal text/cursor colors to both desktop
// extremes, retaining their hue. Explicit program ANSI colors remain intact.
func ReadableOnGlass(ink, background ui.Color, alpha float32, ratio float64) ui.Color {
	toward := ui.RGB(0, 0, 0)
	if srgbLuminance(background) < 0.5 {
		toward = ui.RGB(255, 255, 255)
	}
	return readableInk(ink, toward, ratio, []ui.Color{
		background.Alpha(alpha).Over(ui.RGB(0, 0, 0)),
		background.Alpha(alpha).Over(ui.RGB(255, 255, 255)),
	})
}

// ContrastRatio is WCAG's sRGB luminance ratio. Callers composite translucent
// surfaces first; this function compares the resulting opaque colors.
func ContrastRatio(a, b ui.Color) float64 {
	x, y := srgbLuminance(a), srgbLuminance(b)
	return (max(x, y) + .05) / (min(x, y) + .05)
}

func srgbLuminance(c ui.Color) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return .2126*linear(c.R) + .7152*linear(c.G) + .0722*linear(c.B)
}
