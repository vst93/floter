package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/ansi"
	"floter/internal/theme"
)

// dimRun is the alpha `spanStyle`'s `opacity: 0.62` resolves to: 0.62 × 255,
// rounded the way the framework rounds an alpha byte.
const dimRun = uint8(158)

// TestDimFadesTheWholeRunNotJustTheDefaultColour pins `spanStyle`'s
// `if (style.dim) css.opacity = 0.62`: dim is the run's opacity, so a named
// colour and a named background fade with it, not only the surface default.
func TestDimFadesTheWholeRunNotJustTheDefaultColour(t *testing.T) {
	appearance := theme.Light
	base := ui.RGB(10, 20, 30)
	red := theme.ANSI(appearance)[1]
	blue := theme.ANSI(appearance)[4]

	t.Run("default foreground", func(t *testing.T) {
		got := resolveOutputSpan(appearance, ansi.Style{Dim: true}, base, 11, "x")
		if got.Color.A != dimRun {
			t.Fatalf("dim default run alpha = %d, want %d", got.Color.A, dimRun)
		}
		if got.Color.R != base.R || got.Color.G != base.G || got.Color.B != base.B {
			t.Fatalf("dim changed the default colour to %+v, want %+v", got.Color, base)
		}
	})

	t.Run("named foreground", func(t *testing.T) {
		got := resolveOutputSpan(appearance, ansi.Style{Dim: true, FG: ansi.Color{Kind: ansi.Named, N: 1}}, base, 11, "x")
		if got.Color != red.Alpha(dimAlpha) {
			t.Fatalf("dim named run = %+v, want %+v", got.Color, red.Alpha(dimAlpha))
		}
		if got.Color.A != dimRun {
			t.Fatalf("dim named run alpha = %d, want %d", got.Color.A, dimRun)
		}
	})

	t.Run("named background fades too", func(t *testing.T) {
		got := resolveOutputSpan(appearance, ansi.Style{
			Dim: true,
			FG:  ansi.Color{Kind: ansi.Named, N: 1},
			BG:  ansi.Color{Kind: ansi.Named, N: 4},
		}, base, 11, "x")
		if got.Background != blue.Alpha(dimAlpha) {
			t.Fatalf("dim background = %+v, want %+v", got.Background, blue.Alpha(dimAlpha))
		}
		if got.Background.A != dimRun {
			t.Fatalf("dim background alpha = %d, want %d", got.Background.A, dimRun)
		}
	})
}

// TestInverseSwapsOnlyTheSidesTheCommandNamed pins `spanStyle`'s inverse rule:
// the two colours are exchanged where the command named them, and a side it did
// not name stays the surface's own rather than a filled-in colour. The old
// build's bug was exactly the filled-in background (`c.Theme().Background`).
func TestInverseSwapsOnlyTheSidesTheCommandNamed(t *testing.T) {
	appearance := theme.Light
	base := ui.RGB(10, 20, 30)
	red := theme.ANSI(appearance)[1]
	blue := theme.ANSI(appearance)[4]

	t.Run("only the foreground is named", func(t *testing.T) {
		got := resolveOutputSpan(appearance, ansi.Style{
			Inverse: true,
			FG:      ansi.Color{Kind: ansi.Named, N: 1},
		}, base, 11, "x")
		if got.Color != base {
			t.Fatalf("inverse+fg colour = %+v, want the default %+v", got.Color, base)
		}
		if got.Background != red {
			t.Fatalf("inverse+fg background = %+v, want the named colour %+v", got.Background, red)
		}
	})

	t.Run("only the background is named", func(t *testing.T) {
		got := resolveOutputSpan(appearance, ansi.Style{
			Inverse: true,
			BG:      ansi.Color{Kind: ansi.Named, N: 4},
		}, base, 11, "x")
		if got.Color != blue {
			t.Fatalf("inverse+bg colour = %+v, want the named colour %+v", got.Color, blue)
		}
		if got.Background.A != 0 {
			t.Fatalf("inverse+bg background = %+v, want a transparent one", got.Background)
		}
	})

	t.Run("both sides are named", func(t *testing.T) {
		got := resolveOutputSpan(appearance, ansi.Style{
			Inverse: true,
			FG:      ansi.Color{Kind: ansi.Named, N: 1},
			BG:      ansi.Color{Kind: ansi.Named, N: 4},
		}, base, 11, "x")
		if got.Color != blue {
			t.Fatalf("inverse colour = %+v, want the background colour %+v", got.Color, blue)
		}
		if got.Background != red {
			t.Fatalf("inverse background = %+v, want the foreground colour %+v", got.Background, red)
		}
	})
}

// TestDimAndInverseComposeInSpanStylesOrder pins that dim is applied after the
// swap, as `spanStyle` does (`css.color`/`backgroundColor` are set from the
// swapped pair, then `opacity` covers both).
func TestDimAndInverseComposeInSpanStylesOrder(t *testing.T) {
	appearance := theme.Light
	base := ui.RGB(10, 20, 30)
	red := theme.ANSI(appearance)[1]
	blue := theme.ANSI(appearance)[4]

	got := resolveOutputSpan(appearance, ansi.Style{
		Dim:     true,
		Inverse: true,
		FG:      ansi.Color{Kind: ansi.Named, N: 1},
		BG:      ansi.Color{Kind: ansi.Named, N: 4},
	}, base, 11, "x")
	if got.Color != blue.Alpha(dimAlpha) || got.Background != red.Alpha(dimAlpha) {
		t.Fatalf("dim+inverse = %+v / %+v, want %+v / %+v",
			got.Color, got.Background, blue.Alpha(dimAlpha), red.Alpha(dimAlpha))
	}
}
