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

// TestTheSegmentBoxIsThirtyUnitsHigh pins the segment's own box to the
// reference's `.settings-option` floor.
//
// The reference (`main:src/styles/settings.css:1045`) writes the segment as
// `min-height: calc(var(--u) * 30)` — R43's tightened step — with
// `padding: 0 calc(var(--u) * 9)` (`:1055`). The height is the *floor*, not
// padding: a segment whose label is short is still one 30u box, so every
// segment in a track lines up whatever it says.
//
// This build drew the segment with a vertical padding pair instead
// (`Padding(Space(0.75), Space(2.25))`, no floor), which measured 22.4px at the
// shipped interface step where the reference's 30u floor is 27px — a segment
// roughly a sixth shorter than the one it copied, and one whose height moved
// with the label's line box. Mutation check: drop `MinHeight` (or restore the
// 0.75u vertical padding) and every assertion below goes red.
func TestTheSegmentBoxIsThirtyUnitsHigh(t *testing.T) {
	tt, theme := renderMeasured(t, []string{"Follow", "Long interface", "Tiny"}, 1)

	want := theme.Space(7.5) // 30u, four units a `Space` step.
	if want <= 0 {
		t.Fatalf("the theme has no unit ladder: Space(7.5) = %.2f", want)
	}

	var heights []float32
	for _, label := range []string{"Follow", "Long interface", "Tiny"} {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the segment %q is missing: %q", label, tt.Texts())
		}
		heights = append(heights, rect.H)
		if rect.H != want {
			t.Errorf("the segment %q is %.2f high, want the reference's 30u = %.2f",
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
