package theme

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-at · `FaceOver` is the opaque answer to "what colour is this face,
// really?" — the one a fade laid over a translucent face must use, or the
// fade's own alpha gets multiplied by the face's.
func TestFaceOverReturnsAnOpaqueComposite(t *testing.T) {
	base := ui.RGB(240, 240, 240)
	thin := ui.RGBA(0, 0, 0, 0.102)
	got := FaceOver(base, thin)
	if got.A != 255 {
		t.Errorf("FaceOver returned %v, want an opaque colour (A = %d)", got, got.A)
	}
	want := uint8(math.Round(240 * (1 - 26.0/255)))
	if got.R != want || got.G != want || got.B != want {
		t.Errorf("FaceOver(%v, %v) = %v, want %d in every channel", base, thin, got, want)
	}
	// A chain composites innermost last: the block's own face over the card's
	// over the pane's over the ground, and the result is a single opaque
	// colour — no layer of the chain may survive as alpha, because
	// `ui.Color.Alpha` multiplies whatever is laid on top of it.
	chain := FaceOver(FaceOver(FaceOver(base, ui.RGBA(10, 11, 12, 0.157)), ui.RGBA(0, 0, 0, 0.071)), ui.RGBA(243, 244, 247, 0.169))
	if chain.A != 255 {
		t.Errorf("the composited chain returned %v, want an opaque colour", chain)
	}
	if chain == FaceOver(base, ui.RGBA(10, 11, 12, 0.157)) {
		t.Errorf("the chain collapsed to its first layer (%v): the later layers were dropped", chain)
	}
	// An opaque face over anything is itself: the identity that makes a fade
	// laid on it invisible.
	if got := FaceOver(base, base); got != base {
		t.Errorf("FaceOver(%v, %v) = %v, want the base back", base, base, got)
	}
}
