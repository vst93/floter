package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The launcher card's inset edge is the reference's *platform* table, and it
// moves with the keyboard.
//
// `base.css` writes two inset strokes on the card for Windows and Linux —
// `inset 0 1px 0 var(--edge-highlight)` and `inset 0 -1px 0 var(--edge-shadow)`
// (`main:src/styles/base.css:1550-1556,1612-1617`) — and swaps the top one for
// `var(--edge-highlight-active)` on `:focus-within` (`:1558-1565,1625-1632`),
// with `inset 0 0 0 1px var(--accent-glow)` joining it on Linux. macOS draws
// none: the window server lights the frame there.
//
// The port drew a bare 1px border and no inset stroke, so the card's top never
// caught the light and its bottom never darkened — on both platforms whose
// sheets ask for them, which is why nothing caught it before.
func TestTheCardEdgeIsThePlatformsOwnTable(t *testing.T) {
	tokens := theme.ForPreferences(settings.Default(), true, ui.Preferences{})
	resting := cardEdge(tokens, "windows", false)
	if len(resting) != 2 {
		t.Fatalf("windows card edge = %d strokes, want the rim's 2: %+v", len(resting), resting)
	}
	if resting[0].Y != 1 || resting[0].Color != tokens.EdgeHighlight {
		t.Errorf("windows top stroke = %+v, want inset 0 1px 0 EdgeHighlight", resting[0])
	}
	if resting[1].Y != -1 || resting[1].Color != tokens.EdgeShadow {
		t.Errorf("windows bottom stroke = %+v, want inset 0 -1px 0 EdgeShadow", resting[1])
	}
	if got := cardEdge(tokens, "darwin", false); got != nil {
		t.Errorf("macOS draws a card edge: %+v (the window server lights the frame there)", got)
	}
}

// The focused card's top stroke is the brighter rung, and Linux adds the accent
// ring — the same rule as the stroke and the sheen, on the third half of it.
func TestTheCardEdgeBrightensWithTheKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tokens := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
		if tokens.EdgeHighlightActive == tokens.EdgeHighlight {
			t.Fatalf("dark=%v: the focused top edge is the resting one", dark)
		}
		for _, goos := range []string{"windows", "linux"} {
			resting := cardEdge(tokens, goos, false)
			focused := cardEdge(tokens, goos, true)
			if focusTop(t, focused) != tokens.EdgeHighlightActive {
				t.Errorf("dark=%v %s: the focused top stroke is not EdgeHighlightActive", dark, goos)
			}
			if focusTop(t, resting) != tokens.EdgeHighlight {
				t.Errorf("dark=%v %s: the resting top stroke is not EdgeHighlight", dark, goos)
			}
			// The bottom stroke is the same in both states: the reference's
			// focused rule keeps `inset 0 -1px 0 var(--edge-shadow)`.
			if focused[len(focused)-1] != resting[len(resting)-1] {
				t.Errorf("dark=%v %s: the bottom stroke moved on focus: %+v vs %+v",
					dark, goos, focused[len(focused)-1], resting[len(resting)-1])
			}
		}
		// Linux, focused: the accent ring is drawn, and it is the ring token
		// rather than either of the two others the accent has.
		linux := cardEdge(tokens, "linux", true)
		if len(linux) != 3 {
			t.Fatalf("dark=%v: the focused Linux card edge = %d strokes, want 3: %+v", dark, len(linux), linux)
		}
		if linux[0].Spread != 1 || linux[0].Color != tokens.AccentGlow {
			t.Errorf("dark=%v: the Linux focus ring = %+v, want inset 0 0 0 1px AccentGlow", dark, linux[0])
		}
		if tokens.AccentGlow == tokens.AccentEdge || tokens.AccentGlow == tokens.AccentHalo {
			t.Errorf("dark=%v: --accent-glow is not a rung of its own", dark)
		}
		// At rest Linux draws no ring: the reference's resting rule has none.
		for _, stroke := range cardEdge(tokens, "linux", false) {
			if stroke.Spread != 0 {
				t.Errorf("dark=%v: a resting Linux card draws a keyline: %+v", dark, stroke)
			}
		}
	}
}

// focusTop is the top rim stroke of an edge table (the one with Y = 1).
func focusTop(t *testing.T, strokes []cardEdgeStroke) ui.Color {
	t.Helper()
	for _, stroke := range strokes {
		if stroke.Y == 1 && stroke.Spread == 0 {
			return stroke.Color
		}
	}
	t.Fatalf("the edge table has no top stroke: %+v", strokes)
	return ui.Color{}
}
