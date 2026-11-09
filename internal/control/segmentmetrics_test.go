package control

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// renderMeasured draws one segmented control and returns the tester plus the
// theme it was drawn with, so a test can read the reference's `--u` off it.
func renderMeasured(t *testing.T, labels []string, current int) (*ui.Tester, *ui.Theme) {
	t.Helper()
	tokens := theme.For(settings.Default(), true)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		Segmented(c, tokens, labels, current, func(int) {})
	}, 600, 200)
	tt.Frame()
	return tt, tokens.Theme
}

// Every segment keeps a usable hit area, independent of its label or selection.
func TestSegmentsKeepAComfortableHeight(t *testing.T) {
	tt, theme := renderMeasured(t, []string{"Follow", "Long interface", "Tiny"}, 1)

	want := max(float32(36), theme.Space(9))
	if want <= 0 {
		t.Fatalf("the theme has no unit ladder: Space(9) = %.2f", want)
	}

	var heights []float32
	for _, label := range []string{"Follow", "Long interface", "Tiny"} {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the segment %q is missing: %q", label, tt.Texts())
		}
		heights = append(heights, rect.H)
		if abs(rect.H-want) > 0.01 {
			t.Errorf("the segment %q is %.2f high, want the control floor %.2f",
				label, rect.H, want)
		}
	}
	// The chosen segment wears a raised pane, not a taller box: `.settings-option
	// --active` (`settings.css:1095`) changes the material only, so the floor is
	// the same for every segment.
	if heights[0] != heights[1] || heights[1] != heights[2] {
		t.Errorf("the segments are not one box high: %v", heights)
	}
}
