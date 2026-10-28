package launcher

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// TestTheAccentRulerFollowsTheToken is P11-am's pin, and it freezes the shape of
// the ruler rather than one colour.
//
// The two 「这块面没有 accent」 pins (`TestTheOutputSelectedRowIsTheNeutralChosenRung`,
// `TestTheLauncherListCarriesNoAccent`) read `accentRuler`. Its predecessor was
// the accent's *red* spelled out (`R−G > 24 && R−B > 24 && |G−B| ≤ 8`), which was
// true of `#E5484D` and of nothing in the reference's blue: after P11-al both
// pins counted zero accent pixels on any frame, i.e. neither could fail again.
//
// So the property pinned here is not "the accent is blue" but "the ruler answers
// for its token": for four unrelated hues, the ruler built from one hue must see
// that hue at every alpha a surface actually uses (a fill, a keyline, the quiet
// tint) and must not see the other three — nor a grey. A probe that freezes a
// hue into its condition fails this, whatever that hue is.
func TestTheAccentRulerFollowsTheToken(t *testing.T) {
	// The panel the washes are composited over (the light appearance's own).
	panel := color.RGBA{R: 0xF7, G: 0xF7, B: 0xF8, A: 0xFF}
	hues := []ui.Color{ui.Hex("#064AA6"), ui.Hex("#E5484D"), ui.Hex("#0F7A3D"), ui.Hex("#7A1FA8")}

	for _, hue := range hues {
		ruler := newAccentRuler(hue, panel)
		if ruler.norm == 0 {
			t.Fatalf("the ruler built from %v cannot answer: a neutral accent leaves it nothing to measure", hue)
		}

		// ① The accent itself, and the same accent at the alphas the tokens use:
		//    a fill (1.0), a keyline (0.36) and the quiet tint (0.065).
		for _, alpha := range []float32{1, 0.36, 0.065} {
			p := over(hue.Alpha(alpha), panel)
			share, off := ruler.measure(p)
			if !ruler.carries(p) {
				t.Errorf("accent %v at alpha %.3f reads as %v, which the ruler says is not the accent "+
					"(share %.3f, off the hue line %.1f levels): the probe is blind to its own token",
					hue, alpha, p, share, off)
			}
		}

		// ② A grey, the same hue's sibling at a low alpha over a *hued* surface,
		//    and every other hue: none of these is this accent.
		if ruler.carries(panel) {
			t.Errorf("accent %v: the neutral panel %v reads as the accent", hue, panel)
		}
		for _, other := range hues {
			if other == hue {
				continue
			}
			for _, alpha := range []float32{1, 0.36} {
				if p := over(other.Alpha(alpha), panel); ruler.carries(p) {
					t.Errorf("accent %v: a different hue's pixel %v (alpha %.2f) reads as this accent — "+
						"the ruler follows one hardcoded hue, not the token", hue, p, alpha)
				}
			}
		}
	}
}

// TestTheLiveTokensClearTheRuler is the same ruler read against what the product
// actually paints: both appearances' accent, its quiet tint and its keyline carry;
// the two warm surfaces the launcher legitimately draws (the warning ink and the
// system-icon plate) and the neutral planes do not.
func TestTheLiveTokensClearTheRuler(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := settings.Settings{Theme: "auto", GlassStep: "regular"}
			tokens := theme.ForPreferences(s, tc.dark, ui.Preferences{})
			panel := color.RGBA{
				R: tokens.Theme.Background.R, G: tokens.Theme.Background.G,
				B: tokens.Theme.Background.B, A: 0xFF,
			}
			ruler := newAccentRuler(tokens.Theme.Accent, panel)
			if ruler.norm == 0 {
				t.Fatalf("the accent token %v is neutral: the ruler cannot answer", tokens.Theme.Accent)
			}

			carries := []struct {
				name string
				p    color.RGBA
			}{
				{"the accent fill", over(tokens.Theme.Accent, panel)},
				{"the accent keyline", over(tokens.AccentEdge, panel)},
				{"the quiet tint", over(tokens.AccentTintQuiet, panel)},
				{"an antialiased glyph in the accent", over(tokens.Theme.Accent.Alpha(0.35), panel)},
			}
			for _, c := range carries {
				if !ruler.carries(c.p) {
					share, off := ruler.measure(c.p)
					t.Errorf("%s (%v) is not read as the accent (token %v): share %.3f, off the hue line %.1f",
						c.name, c.p, tokens.Theme.Accent, share, off)
				}
			}

			neutral := []struct {
				name string
				p    color.RGBA
			}{
				{"the panel", panel},
				{"a chosen row", over(tokens.RowSelected, panel)},
				{"the warning ink", over(tokens.Theme.Warning, panel)},
				{"the warm system-icon plate", over(tokens.IconSurface, panel)},
			}
			for _, c := range neutral {
				if ruler.carries(c.p) {
					t.Errorf("%s (%v) is read as the accent %v: the ruler counts a colour of its own as accent spend",
						c.name, c.p, tokens.Theme.Accent)
				}
			}
		})
	}
}
