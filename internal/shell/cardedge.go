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
// Windows has one rung above those two: a desktop that asked for more contrast
// gets **one** ring instead of the pair —
//
//	@media (prefers-contrast: more) {
//	  .platform-windows .collapsed-card,
//	  .platform-windows .settings-card,
//	  .platform-windows .terminal-panel {
//	    box-shadow: 0 4px 16px …, 0 1px 4px …, inset 0 0 0 1px var(--stroke-contrast);
//	  }
//	}
//
// (`main:src/styles/base.css:1576-1584`), which is what this table's
// `contrast` argument selects. Linux has no such block: its two strokes stay,
// with `--stroke-contrast` already carried by the palette (`theme.contrast()`).
// macOS keeps drawing none.
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
// The order is the reference's: the two rim strokes, top before bottom, because
// `box-shadow` paints in declaration order.
//
// `contrast` is the desktop's `prefers-contrast: more` answer and only Windows
// branches on it: its block replaces the pair with one `--stroke-contrast`
// ring, so the resting and focused states are the *same* face there — the
// reference's contrast rule has no `:focus-within` counterpart.
func cardEdge(tokens theme.Tokens, goos string, focused, contrast bool) []cardEdgeStroke {
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
		if contrast {
			// … inset 0 0 0 1px var(--stroke-contrast), the pair gone.
			return []cardEdgeStroke{{Spread: 1, Color: tokens.InputStroke}}
		}
		return []cardEdgeStroke{
			{Y: 1, Color: top},
			{Y: -1, Color: tokens.EdgeShadow},
		}
	case "linux":
		// P11-z 重定基线（用户实测 F2）：这里曾加一圈 `inset 0 0 0 1px
		// AccentGlow`（base.css 的 Linux `:focus-within` keyline）。它回答的是
		// 「整个窗口有键盘焦点」——§5 的四类里没有这一类，用户读到的就是
		// 「整体红色线条太扎眼」。焦点现在只由上面那对描边（聚焦时顶边换
		// `EdgeHighlightActive`）与 `SheenAtFocus` 的亮面说，不再额外画一圈红。
		// （顺带查到的旧账：`theme.For` 从未给 `AccentGlow` 赋值，所以那一圈
		// 一直用零值颜色画，实际是隐形的；见 `theme.Tokens.AccentGlow`。）
		return []cardEdgeStroke{
			{Y: 1, Color: top},
			{Y: -1, Color: tokens.EdgeShadow},
		}
	}
	return nil
}

// drawCardEdge paints cardEdge onto the card.
func drawCardEdge(card ui.Element, tokens theme.Tokens, goos string, focused, contrast bool) {
	for _, stroke := range cardEdge(tokens, goos, focused, contrast) {
		card.InsetShadow(stroke.X, stroke.Y, 0, stroke.Spread, stroke.Color)
	}
}

// panelEdge is a *panel's* inset edges — the settings card and the terminal
// panel — for a platform. A panel is not the launcher card: it sits *on* a
// surface, so its resting rule is base.css's `--glass-rim-shadow` (four inset
// strokes), and the two platform blocks replace that with two.
//
//	/* settings.css:27, terminal.css:41 — the resting rule on every platform */
//	box-shadow: var(--glass-rim-shadow);
//	/* base.css:1541-1556 (Windows), :1609-1617 (Linux) — the platform's own */
//	… inset 0 1px 0 var(--edge-highlight),
//	  inset 0 -1px 0 var(--edge-shadow);
//
// So macOS keeps all four (the lit top, the dark bottom and the whisper on both
// sides) and Windows and Linux keep **two**: the platform rules name the top and
// the bottom only. The port drew the four-stroke rim on all three platforms, so
// the two side whispers `base.css` deliberately drops there were still painted —
// a 1px lit line inside each side edge of the settings card and the terminal
// panel, on the two platforms whose sheets ask for none.
//
// Windows adds one more distinction: its **terminal** panel is the one surface
// with its own pair. `.platform-windows .terminal-panel`
// (`main:src/styles/base.css:1567-1574`) writes
// `inset 0 1px 0 var(--keycap-highlight)` and
// `inset 0 -1px 0 var(--keycap-shadow)` — the keycap pair, a step quieter than
// the edge's own (0.07 against 0.15 in the dark appearance) — because a terminal
// panel is a keycap-shaped slab rather than a sheet lying on a surface. The port
// gave it the edge pair, so the Windows terminal's crown was twice as bright as
// its rule asks for.
//
// `terminal` selects that panel; the settings card leaves it false.
//
// The contrast rung is Windows' too, and it covers the terminal panel's keycap
// pair as well: `base.css:1576-1584` names all three surfaces, so the one ring
// replaces whichever pair the surface would have worn.
func panelEdge(tokens theme.Tokens, goos string, terminal, contrast bool) []cardEdgeStroke {
	top, bottom := tokens.EdgeHighlight, tokens.EdgeShadow
	if goos == "windows" && terminal {
		top, bottom = tokens.KeycapHighlight, tokens.KeycapShadow
	}
	switch goos {
	case "windows":
		if contrast {
			return []cardEdgeStroke{{Spread: 1, Color: tokens.InputStroke}}
		}
		// The two strokes the platform block names; the rim's side whispers
		// are not restated, so they are gone.
		return []cardEdgeStroke{
			{Y: 1, Color: top},
			{Y: -1, Color: bottom},
		}
	case "linux":
		// The two strokes the platform block names; the rim's side whispers
		// are not restated, so they are gone.
		return []cardEdgeStroke{
			{Y: 1, Color: top},
			{Y: -1, Color: bottom},
		}
	}
	// macOS: the base rule's `--glass-rim-shadow`, whole.
	return []cardEdgeStroke{
		{Y: 1, Color: top},
		{Y: -1, Color: bottom},
		{X: 1, Color: tokens.EdgeHighlightX},
		{X: -1, Color: tokens.EdgeHighlightX},
	}
}

// drawPanelEdge paints panelEdge onto a panel.
func drawPanelEdge(panel ui.Element, tokens theme.Tokens, goos string, terminal, contrast bool) {
	for _, stroke := range panelEdge(tokens, goos, terminal, contrast) {
		panel.InsetShadow(stroke.X, stroke.Y, 0, stroke.Spread, stroke.Color)
	}
}
