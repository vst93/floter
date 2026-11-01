package settingsui

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// levelOf is one pixel's mean channel: the unit this round's numbers use
// (1 亮度级 = 3 in the rgb sum).
func levelOf(c color.RGBA) float64 { return float64(c.R+c.G+c.B) / 3 }

// levelOfColor is the same reading for a token that has not been painted yet.
func levelOfColor(c ui.Color) float64 { return float64(c.R+c.G+c.B) / 3 }

// healthFailureSample is the three pixels one failed, retryable probe's card
// is read from, on one appearance.
type healthFailureSample struct {
	ground, card, chip ui.Color
	// chipOverCard is the measured step the chip's own plane makes over the
	// card it sits in, in levels (negative: darker than its parent).
	chipOverCard float64
	// cardOverGround is the card's own step above the page.
	cardOverGround float64
}

// sampleHealthFailureCard draws one failed, retryable probe's card on the real
// settings-page ground and reads the three pixels.
func sampleHealthFailureCard(t *testing.T, dark bool) (healthFailureSample, theme.Tokens) {
	t.Helper()
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Width(360).Children(func() {
			a.healthFailureCard(c, a.copy(), HealthProbe{
				ID: "probe.git", Passed: false, ExitCode: 1, Retryable: true,
			})
		})
	}, 420, 220)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	image := tt.Image()
	chip, ok := tt.Find(a.copy().IntegrationsHealthRetryable)
	if !ok {
		t.Fatalf("the retryable chip is not drawn: %q", tt.Texts())
	}
	id, ok := tt.Find("probe.git")
	if !ok {
		t.Fatalf("the probe id is not drawn: %q", tt.Texts())
	}
	// The chip's own padding is `Space(1)` on the x axis, so two pixels left of
	// the word's box is the chip's blank plane; the card's is four pixels above
	// the head row, inside the card's own `Space(1.5)` padding.
	chipPx := image.RGBAAt(int(chip.X)-2, int(chip.Y+chip.H/2))
	cardPx := image.RGBAAt(int(id.X), int(id.Y)-4)
	tokens := theme.ForPreferences(a.settings(), dark, ui.Preferences{})
	return healthFailureSample{
		ground:         toUIColor(image.RGBAAt(2, 2)),
		card:           toUIColor(cardPx),
		chip:           toUIColor(chipPx),
		chipOverCard:   levelOf(chipPx) - levelOf(cardPx),
		cardOverGround: levelOf(cardPx) - levelOfColor(ground),
	}, tokens
}

func toUIColor(c color.RGBA) ui.Color {
	return ui.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}

// The retryable chip's plane is the card's own warm plane — **applied twice**.
//
// `healthFailureCard` paints `Background(tokens.RaisedWarm)` on the card and
// then the same token again on the `--retryable` chip inside it (P13-cu,
// `docs/liquid-glass.md` §6.64). Two passes of one straight-alpha token is not
// a rung: 0.13 → 0.169 in light and 0.115 → 0.197 in dark, i.e. the chip's
// step is whatever that arithmetic happens to land on.
//
// This pin is what makes that a *decision*: the composite is locked to the
// double wash on both appearances, and the two named candidates a reader would
// reach for are checked to be distinguishable from it — `Control` (the face
// `approvalPermissionPills` gives its chips one block away) and
// `IconSurfaceHover` (the warm family's next rung). Measured: chip over card
// −13.7 / +17.3 levels, card over ground −15.7 / +19.3, against Control
// −16.3 / +22.7 and IconSurfaceHover −22.7 / +31.0. So the chip is the
// **quietest** plane this card can hold: every named token would make a 4px
// word louder than the card it sits in. Hence: measured, not changed.
func TestTheRetryableChipPaintsTheCardPlaneTwice(t *testing.T) {
	for _, dark := range []bool{false, true} {
		sample, tokens := sampleHealthFailureCard(t, dark)
		want := panelOver(tokens.RaisedWarm, sample.card)
		control := panelOver(tokens.Control, sample.card)
		warmHover := panelOver(tokens.IconSurfaceHover, sample.card)
		if !panelClose(sample.chip, want, 2) {
			t.Errorf("%s: the chip's plane = %v, want the card's own warm plane applied twice %v",
				appearanceName(dark), sample.chip, want)
		}
		for name, candidate := range map[string]ui.Color{
			"Control":          control,
			"IconSurfaceHover": warmHover,
		} {
			if panelClose(sample.chip, candidate, 2) {
				t.Errorf("%s: the chip's plane reads %s %v, not the double wash %v",
					appearanceName(dark), name, candidate, want)
			}
		}
		// The step stays *inside* the card's own lift: a nested 4px word must
		// read quieter than the whole card it sits in.
		if got, card := absf(sample.chipOverCard), absf(sample.cardOverGround); got > card {
			t.Errorf("%s: the chip steps %.1f levels over its card, more than the card's own %.1f over the page",
				appearanceName(dark), got, card)
		}
		wantStep := levelOfColor(want) - levelOfColor(sample.card)
		if diff := absf(sample.chipOverCard - wantStep); diff > 2 {
			t.Errorf("%s: the chip's measured step = %.1f levels, want %.1f", appearanceName(dark), sample.chipOverCard, wantStep)
		}
	}
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// The health page's three status faces, before any pixel: a health *chip*
// carries one of three planes and the raw status decides which (P13-cu keeps
// the reference's split — `docs/liquid-glass.md` §6.64).
//
// This is the page's first structural pin. `healthStatusFace` had no test at
// all, so a chip could have drifted to any token without a word of complaint.
func TestTheHealthStatusChipFaces(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tokens := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
		th := tokens.Theme
		store := settings.NewStore(settings.Default())
		a := New(store, Actions{})
		for _, tc := range []struct {
			raw       string
			plane     ui.Color
			keyline   ui.Color
			ink       ui.Color
			transpare bool
		}{
			{"healthy", tokens.Control, tokens.ControlEdge, tokens.TextStrong, false},
			{"degraded", tokens.RaisedWarm, tokens.ControlEdge, th.Warning, false},
			{"unhealthy", tokens.RaisedWarm, tokens.ControlEdge, th.Warning, false},
			{"unknown", ui.Transparent, ui.Transparent, th.TextMuted, true},
		} {
			plane, keyline, ink := healthStatusFace(tokens, th, a.copy(), "", tc.raw)
			if plane != tc.plane || keyline != tc.keyline || ink != tc.ink {
				t.Errorf("%s/%s: face = %v/%v/%v, want %v/%v/%v",
					appearanceName(dark), tc.raw, plane, keyline, ink, tc.plane, tc.keyline, tc.ink)
			}
			if tc.transpare && plane.A != 0 {
				t.Errorf("%s/%s: `unknown` paints a plane %v; it is the word alone", appearanceName(dark), tc.raw, plane)
			}
		}
	}
}
