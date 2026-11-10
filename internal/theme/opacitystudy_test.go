package theme

import (
	"os"
	"testing"

	"floter/internal/glassmap"
	"github.com/egoist/mygo/ui"
)

// Opt-in measurement of candidate floors, including the least favorable
// native veil compensation, both desktop extremes, and active controls.
func TestOpacityFloorStudy(t *testing.T) {
	if os.Getenv("FLOTER_OPACITY_STUDY") == "" {
		t.Skip("set FLOTER_OPACITY_STUDY")
	}
	for _, app := range []Appearance{Light, Dark} {
		for _, level := range []float32{.50, .55, .60, .65, .68, .70, .71, .72, .74, .75, .78} {
			worst := float64(21)
			var tertiary ui.Color
			for _, step := range []string{"off", "frosted", "liquid"} {
				spec := glassmap.SpecForOS("darwin", step, 86)
				spec.Target = level
				spec.TintAlpha = (level - spec.Material) / (1 - spec.Material)
				p := paletteFor(app).readable(app, spec, false)
				tertiary = p.textTertiary
				for _, bg := range p.readabilityBackdrops(app, spec) {
					for _, ink := range []ui.Color{p.text, p.textSecondary, p.textMuted, p.textTertiary, p.accent, p.warning} {
						worst = min(worst, ContrastRatio(ink, bg))
					}
				}
			}
			t.Logf("%s %.0f%% worst=%.2f tertiary=%v", app, level*100, worst, tertiary)
		}
	}
}
