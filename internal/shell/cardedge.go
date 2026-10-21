package shell

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The launcher card's inset edges, per platform — the reference's own table.
//
// `base.css` gives the card a *platform* edge, and it is not the same rule on
// each:
//
//	.platform-windows .collapsed-card,
//	.platform-linux   .collapsed-card {
//	  box-shadow: …,
//	    inset 0 1px 0 var(--edge-highlight),
//	    inset 0 -1px 0 var(--edge-shadow);
//	}
//	… :focus-within {
//	  box-shadow: …,
//	    inset 0 0 0 1px var(--accent-glow),      /* Linux swaps the keyline in */
//	    inset 0 1px 0 var(--edge-highlight-active),
//	    inset 0 -1px 0 var(--edge-shadow);
//	}
//
// (`main:src/styles/base.css:1541-1565` for Windows, `:1609-1632` for Linux; the
// resting tokens are at `:632-635`, the active one at `:633` dark / `:911`
// light, and `--accent-glow` at `:687` / `:955`.) macOS draws none of them: the
// window server lights and casts the frame there.
//
// The port drew the card as a bare 1px `Border` plus a cast shadow and no inset
// edge at all, so the top of the card never caught the light and its bottom
// never darkened — the two strokes that make a translucent pane read as a pane
// rather than as a rectangle of tint, on both platforms whose sheets ask for
// them.
//
// The table is a pure function of the platform and the keyboard so a test can
// read all three platforms on any host: a Windows-only or Linux-only branch is
// exactly the shape that goes unnoticed when only the running platform's suite
// reads it (the 10px/9px inset slip, see `insetForOS`).

// cardEdgeStroke is one inset stroke of the card's edge: the geometry as
// `inset <x> <y> 0 <spread>` reads, and the token it paints. An inset stroke's
// `blur` is always 0 here — every one of the reference's is.
type cardEdgeStroke struct {
	X, Y   float32
	Spread float32
	Color  ui.Color
}

// cardEdge is the card's inset edges for a platform and a focus state. An empty
// slice means the reference draws no edge there (macOS), not that none has been
// written yet.
//
// The order is the reference's: the accent keyline (Linux, focused only) before
// the two rim strokes, because `box-shadow` paints in declaration order and the
// keyline is the one that has to sit on top of the frame's own tint.
func cardEdge(tokens theme.Tokens, goos string, focused bool) []cardEdgeStroke {
	top := tokens.EdgeHighlight
	// The focused rung of the same stroke: the reference's `:focus-within`
	// rules swap `--edge-highlight` for `--edge-highlight-active` while the
	// stroke and the sheen move with it (`.collapsed-card:focus-within`,
	// `main:src/styles/launcher.css:136-144`, and see SheenAtFocus).
	if focused {
		top = tokens.EdgeHighlightActive
	}
	switch goos {
	case "windows":
		return []cardEdgeStroke{
			{Y: 1, Color: top},
			{Y: -1, Color: tokens.EdgeShadow},
		}
	case "linux":
		out := make([]cardEdgeStroke, 0, 3)
		if focused {
			// `inset 0 0 0 1px var(--accent-glow)`: the Linux card's keyboard
			// cue is the accent ring, not the brighter top stroke — the two
			// are drawn together, which is why the ring is first.
			out = append(out, cardEdgeStroke{Spread: 1, Color: tokens.AccentGlow})
		}
		return append(out,
			cardEdgeStroke{Y: 1, Color: top},
			cardEdgeStroke{Y: -1, Color: tokens.EdgeShadow},
		)
	}
	return nil
}

// drawCardEdge paints cardEdge onto the card.
func drawCardEdge(card ui.Element, tokens theme.Tokens, goos string, focused bool) {
	for _, stroke := range cardEdge(tokens, goos, focused) {
		card.InsetShadow(stroke.X, stroke.Y, 0, stroke.Spread, stroke.Color)
	}
}
