package terminalui

import (
	"os"
	"testing"

	"floter/internal/theme"
	"github.com/egoist/mygo/ui"
)

func TestTerminalOpacityFloorStudy(t *testing.T) {
	if os.Getenv("FLOTER_OPACITY_STUDY") == "" {
		t.Skip("set FLOTER_OPACITY_STUDY")
	}
	for _, level := range []float32{.50, .55, .60, .62, .64, .65, .66, .68, .70} {
		original, best := float64(21), float64(21)
		for name, p := range palettes {
			_ = name
			ink := ui.RGB(0, 0, 0)
			if p.dark {
				ink = ui.RGB(255, 255, 255)
			}
			for _, veil := range []float32{0, .12} {
				alpha := (level - veil) / (1 - veil)
				for _, desktop := range []ui.Color{ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)} {
					bg := packed(p.bg).Alpha(alpha).Over(desktop)
					original = min(original, theme.ContrastRatio(packed(p.fg), bg))
					best = min(best, theme.ContrastRatio(ink, bg))
				}
			}
		}
		t.Logf("terminal %.0f%% original=%.2f achievable=%.2f", level*100, original, best)
	}
}
