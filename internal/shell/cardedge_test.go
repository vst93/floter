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
	resting := cardEdge(tokens, "windows", false, false)
	if len(resting) != 2 {
		t.Fatalf("windows card edge = %d strokes, want the rim's 2: %+v", len(resting), resting)
	}
	if resting[0].Y != 1 || resting[0].Color != tokens.EdgeHighlight {
		t.Errorf("windows top stroke = %+v, want inset 0 1px 0 EdgeHighlight", resting[0])
	}
	if resting[1].Y != -1 || resting[1].Color != tokens.EdgeShadow {
		t.Errorf("windows bottom stroke = %+v, want inset 0 -1px 0 EdgeShadow", resting[1])
	}
	if got := cardEdge(tokens, "darwin", false, false); got != nil {
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
			resting := cardEdge(tokens, goos, false, false)
			focused := cardEdge(tokens, goos, true, false)
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
		linux := cardEdge(tokens, "linux", true, false)
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
		for _, stroke := range cardEdge(tokens, "linux", false, false) {
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

// A *panel*'s edge is not the card's edge: base.css's platform blocks name two
// strokes for the settings card and the terminal panel, so the rim's two side
// whispers are gone there, and the Windows terminal panel wears the quieter
// keycap pair instead of the edge pair.
//
//	main:src/styles/settings.css:27 / terminal.css:41   → var(--glass-rim-shadow)
//	main:src/styles/base.css:1541-1556, 1609-1617       → the platform's two
//	main:src/styles/base.css:1567-1574                  → the keycap pair (terminal)
func TestAPanelEdgeIsThePlatformsTableNotTheRim(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tokens := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
		if tokens.KeycapHighlight == tokens.EdgeHighlight {
			t.Fatalf("dark=%v: the keycap crown is the edge's own", dark)
		}
		for _, goos := range []string{"windows", "linux"} {
			// The settings card: the edge pair, and no side whisper.
			settingsEdge := panelEdge(tokens, goos, false, false)
			if len(settingsEdge) != 2 {
				t.Errorf("dark=%v %s: the settings panel edge = %d strokes, want 2: %+v",
					dark, goos, len(settingsEdge), settingsEdge)
			}
			for _, stroke := range settingsEdge {
				if stroke.X != 0 {
					t.Errorf("dark=%v %s: a panel draws the rim's side whisper: %+v", dark, goos, stroke)
				}
			}
			if top, _ := panelTops(settingsEdge); top != tokens.EdgeHighlight {
				t.Errorf("dark=%v %s: the settings panel's crown is not EdgeHighlight", dark, goos)
			}
			// The Windows terminal panel is the one surface with its own pair.
			terminalEdge := panelEdge(tokens, goos, true, false)
			top, bottom := panelTops(terminalEdge)
			if goos == "windows" {
				if top != tokens.KeycapHighlight || bottom != tokens.KeycapShadow {
					t.Errorf("dark=%v windows: the terminal panel's rim = %v/%v, want KeycapHighlight/KeycapShadow",
						dark, top, bottom)
				}
			} else if top != tokens.EdgeHighlight || bottom != tokens.EdgeShadow {
				// Linux has no terminal-panel rule of its own: the group rule
				// (`:1609-1617`) is the whole of it.
				t.Errorf("dark=%v linux: the terminal panel's rim = %v/%v, want the edge pair", dark, top, bottom)
			}
		}
		// macOS keeps base.css's `--glass-rim-shadow`: four strokes.
		mac := panelEdge(tokens, "darwin", false, false)
		if len(mac) != 4 {
			t.Fatalf("dark=%v: the macOS panel edge = %d strokes, want the rim's 4: %+v", dark, len(mac), mac)
		}
		sides := 0
		for _, stroke := range mac {
			if stroke.X != 0 && stroke.Color == tokens.EdgeHighlightX {
				sides++
			}
		}
		if sides != 2 {
			t.Errorf("dark=%v: the macOS panel kept %d side whispers, want 2", dark, sides)
		}
	}
}

// The contrast rung: a desktop that asked for more contrast gets Windows' one
// ring in place of the pair, on the card and on both panels — the terminal's
// keycap pair included, because the reference's block names all three surfaces.
//
//	@media (prefers-contrast: more) {
//	  .platform-windows .collapsed-card,
//	  .platform-windows .settings-card,
//	  .platform-windows .terminal-panel {
//	    box-shadow: …, inset 0 0 0 1px var(--stroke-contrast);
//	  }
//	}
//
// (`main:src/styles/base.css:1576-1584`.) The ring's colour is the palette's
// stroke under the preference, which is `--stroke-contrast` (`theme.contrast()`)
// — the same token base.css swaps in, so one decision feeds both.
//
// The port had no contrast argument at all: a Windows desktop that asked for
// more contrast still got the resting pair, and the reference's stronger ring
// was never drawn on any of the three surfaces.
func TestTheContrastRungIsWindowsOneRing(t *testing.T) {
	resting := theme.ForPreferences(settings.Default(), true, ui.Preferences{})
	ringed := theme.ForPreferences(settings.Default(), true, ui.Preferences{HighContrast: true})
	if ringed.InputStroke == resting.InputStroke {
		t.Fatalf("the contrast palette's stroke is the resting one")
	}
	// Windows: one ring, on the card and on both panels, focused or not.
	for _, focused := range []bool{false, true} {
		strokes := cardEdge(ringed, "windows", focused, true)
		if len(strokes) != 1 || strokes[0].Spread != 1 || strokes[0].Color != ringed.InputStroke {
			t.Errorf("focused=%v: the contrast card edge = %+v, want one InputStroke ring", focused, strokes)
		}
	}
	for _, terminal := range []bool{false, true} {
		strokes := panelEdge(ringed, "windows", terminal, true)
		if len(strokes) != 1 || strokes[0].Spread != 1 || strokes[0].Color != ringed.InputStroke {
			t.Errorf("terminal=%v: the contrast panel edge = %+v, want one InputStroke ring", terminal, strokes)
		}
	}
	// Linux and macOS are untouched: their blocks have no contrast override.
	if got := cardEdge(ringed, "linux", false, true); len(got) != 2 {
		t.Errorf("linux contrast card edge = %d strokes, want the resting 2", len(got))
	}
	if got := cardEdge(ringed, "darwin", false, true); got != nil {
		t.Errorf("macOS drew a contrast card edge: %+v", got)
	}
	if got := panelEdge(ringed, "linux", true, true); len(got) != 2 {
		t.Errorf("linux contrast panel edge = %d strokes, want the resting 2", len(got))
	}
	if got := panelEdge(ringed, "darwin", false, true); len(got) != 4 {
		t.Errorf("macOS contrast panel edge = %d strokes, want the rim's 4", len(got))
	}
}

// panelTops is a panel edge's crown and its seat.
func panelTops(strokes []cardEdgeStroke) (top, bottom ui.Color) {
	for _, stroke := range strokes {
		switch {
		case stroke.Y == 1:
			top = stroke.Color
		case stroke.Y == -1:
			bottom = stroke.Color
		}
	}
	return top, bottom
}
