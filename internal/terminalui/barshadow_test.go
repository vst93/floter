package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// barShadowCanvas is the mid-grey the hover shadow is read against: the
// reference casts `--elev-bar` onto the content *under* the bar, so the frame
// needs a lit surface there rather than the black the ink tests use.
var barShadowCanvas = ui.RGBA(184, 184, 184, 1)

// barShadowFrame renders the bar over the grey canvas with the pointer on the
// bar only when hover is true, so the reference's hover shadow
// (`.terminal-bar:hover { box-shadow: var(--elev-bar) }`, `terminal.css:99-104`)
// can be read off the pixels just below the bar against the same frame with the
// pointer clear of it. It returns the bar's bottom edge, in pixels, so a caller
// samples the first row the shadow falls on.
func barShadowFrame(t *testing.T, a *App, hover bool) (*ui.Tester, theme.Tokens, float32) {
	t.Helper()
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	var tok theme.Tokens
	var floorY float32
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		floorY = c.Theme().Space(7) // the bar's own height
		c.Root().Background(barShadowCanvas)
		a.titleBar(c, copy)
	}, 900, 120)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if hover {
		tt.Move(450, 14) // over the bar
	} else {
		tt.Move(450, 100) // clear of it: no hover
	}
	tt.Frame()
	return tt, tok, floorY
}

// grey is the mean channel of a pixel; the canvas is grey, so the shadow is
// read as a drop in this value.
func grey(c ui.Color) int { return (int(c.R) + int(c.G) + int(c.B)) / 3 }

// `.terminal-bar:hover { box-shadow: var(--elev-bar) }` (`src/styles/terminal.css:
// 99-104`): the hover shadow is `0 6px 16px` at `0.18 * --elev-shadow-scale`,
// cast onto the content that scrolls under the bar. Read off the live bar: the
// row just below it is darker while the pointer is on the bar than in the same
// frame with the pointer clear, and the idle bar casts nothing at all — the row
// is the bare canvas.
func TestTheTerminalBarLiftsItsShadowOnHover(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, barTitle)

	idle, _, floorY := barShadowFrame(t, a, false)
	hovered, _, _ := barShadowFrame(t, a, true)

	// A column clear of the identity dot on the left and the action buttons on
	// the right: only the bar's own silhouette throws here.
	const col = 450
	row := int(floorY) // the first row under the bar's 7u height

	idlePix := sample(idle, col, row)
	hoverPix := sample(hovered, col, row)
	if grey(hoverPix) >= grey(idlePix) {
		t.Errorf("the hovered bar did not darken the row below it: idle %v, hovered %v", idlePix, hoverPix)
	}
	// The idle bar casts nothing: the row is the untouched canvas, so the drop
	// above is the shadow and not some other hover-dependent paint.
	if diff := grey(idlePix) - grey(barShadowCanvas); absInt(diff) > 4 {
		t.Errorf("the idle bar painted under itself (row %d = %v, canvas %v)", row, idlePix, barShadowCanvas)
	}
	// The shadow falls off with distance: well past the 6px throw plus the
	// 16px blur's reach it is gone, so it reads as the bar's own light shadow,
	// not a flat wash over the panel.
	far := row + 30
	if diff := grey(sample(idle, col, far)) - grey(sample(hovered, col, far)); diff > 2 {
		t.Errorf("the hover shadow still darkens the frame far below the bar: %d at row %d", diff, far)
	}
}
