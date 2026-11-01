package settingsui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The health block's probe list is the page's last unpinned *shape*. P13-cu
// pinned the failure card's faces (the chip's double wash), but the list's
// punctuation — where the 1px hairline between two probes actually lands — had
// no pin at all, and it was landing in the wrong place.
//
// `healthProbeRow` used to wear `InsetShadow(0, -1, 0, 0, tokens.Hairline)`.
// In this framework a negative y offset leaves the uncovered strip on the
// box's **bottom** edge (`mygo/ui/shadow_test.go`'s `InsetShadow(0, 6, 0, 0)`
// case: a positive offset shadows the top), so the line was painted under the
// row's own text, not above it:
//
//	Checks      y 439..454
//	up          y 462..477
//	rule        y 480        (3px under "up", 8px above "extras")
//	extras      y 489..504
//	rule        y 507        (8px under "extras", then the next heading)
//	Failures    y 516
//
// Two things are wrong with that shape, and both are about space rather than
// ink: a divider that hugs one side by 5px reads as an underline of the row
// above, and a rule under the *last* row closes a list that the next heading
// already closes — the orphan line. The diagnostics grid on the same page
// (`settingsui.go` `diagnosisCheckRow`) is drawn the other way round: a 1px box
// *between* rows, dropped on the last. The probe list now uses that idiom.
//
// These pins measure the drawn frame, not the code: rules are found as rows
// that differ from the block's face, in a column that carries no glyph ink.

// probePage renders the Integrations page with one integration carrying the
// given stored probe report.
func probePage(t *testing.T, report *HealthReport, dark bool) *ui.Tester {
	t.Helper()
	store := newStore(t)
	if dark {
		if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
			t.Fatal(err)
		}
	}
	a := New(store, Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.probed", Name: "Probed Tool", Enabled: true,
			Health: "Degraded", HealthReport: report,
		}}
	}
	tt := render(t, a, 720, 1100)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

func channelSum(c ui.Color) int { return int(c.R) + int(c.G) + int(c.B) }

// ruleRows reports the rows inside [y0,y1) whose pixel at x differs from the
// block's face, skipping the y bands the probe text occupies.
func ruleRows(img *image.RGBA, x, y0, y1 int, skip [][2]int) []int {
	face := imagePixel(img, x, y0)
	var rows []int
	for y := y0; y < y1; y++ {
		inText := false
		for _, band := range skip {
			if y >= band[0] && y < band[1] {
				inText = true
				break
			}
		}
		if inText {
			continue
		}
		if imagePixel(img, x, y) != face {
			rows = append(rows, y)
		}
	}
	return rows
}

// TestTheProbeListSeparatesItsRowsAndDoesNotCloseItself is the shape pin: with
// N probes there are exactly N-1 one-pixel rules, each of them the same air
// away from the text above and the text below, and none of them closing the
// list.
func TestTheProbeListSeparatesItsRowsAndDoesNotCloseItself(t *testing.T) {
	for _, probes := range [][]HealthProbe{
		{{ID: "up", Passed: true, DurationMs: 12}, {ID: "extras", Passed: false, DurationMs: 30, ExitCode: 3}},
		{{ID: "up", Passed: true, DurationMs: 12}, {ID: "extras", Passed: false, DurationMs: 30, ExitCode: 3}, {ID: "paths", Passed: true, DurationMs: 9}},
	} {
		report := &HealthReport{CheckedAt: "2026-10-12T09:30:00Z", Probes: probes}
		tt := probePage(t, report, false)
		img := tt.Image()
		label := mustFind(t, tt, "Checks")
		failures := mustFind(t, tt, "Failures")
		const x = 320 // inside the list's width, clear of every probe id's glyphs
		var bands [][2]int
		for _, probe := range probes {
			box := mustFind(t, tt, probe.ID)
			bands = append(bands, [2]int{int(box.Y) - 2, int(box.Y+box.H) + 2})
		}
		rules := ruleRows(img, x, int(label.Y+label.H)+1, int(failures.Y), bands)
		if len(rules) != len(probes)-1 {
			t.Fatalf("%d probes draw %d rules, want %d (one *between* rows, none closing the list): %v",
				len(probes), len(rules), len(probes)-1, rules)
		}
		// Every rule is exactly one pixel: no fog, no double line.
		for i := 1; i < len(rules); i++ {
			if rules[i] == rules[i-1]+1 {
				t.Errorf("rows %d and %d at y=%d,y=%d are both off the face: a rule grew past 1px",
					i-1, i, rules[i-1], rules[i])
			}
		}
		// The air on both sides of a rule is the same: that is what turns a
		// row underline into a divider between two rows.
		for i, y := range rules {
			above := mustFind(t, tt, probes[i].ID)
			below := mustFind(t, tt, probes[i+1].ID)
			up := y - int(above.Y+above.H)
			down := int(below.Y) - (y + 1)
			if up != down {
				t.Errorf("the rule at y=%d sits %dpx under %q but %dpx above %q: it is not between them",
					y, up, probes[i].ID, down, probes[i+1].ID)
			}
		}
		// And the list ends in air: the last probe's row carries no rule, so
		// the band between its text and the next heading is clean.
		last := mustFind(t, tt, probes[len(probes)-1].ID)
		if tail := ruleRows(img, x, int(last.Y+last.H)+3, int(failures.Y), nil); len(tail) != 0 {
			t.Errorf("the list closes itself with a rule at %v, right above the next heading", tail)
		}
	}
}

// TestTheProbeRuleIsTheHairlineNotATextRule pins the ink of that separator: it
// is the same `Hairline` the card's own edges use (7 brightness levels darker on
// the light face, 19 lighter on the dark one — the sum unit is 3 per level),
// not a text-weight line and not a second surface. Measured on the drawn frame
// at the rule's own row.
func TestTheProbeRuleIsTheHairlineNotATextRule(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt := probePage(t, testHealthReport(), dark)
		img := tt.Image()
		label := mustFind(t, tt, "Checks")
		failures := mustFind(t, tt, "Failures")
		up := mustFind(t, tt, "up")
		extras := mustFind(t, tt, "extras")
		const x = 320
		face := imagePixel(img, x, int(up.Y+up.H)+2)
		bands := [][2]int{
			{int(up.Y) - 2, int(up.Y+up.H) + 2},
			{int(extras.Y) - 2, int(extras.Y+extras.H) + 2},
		}
		rules := ruleRows(img, x, int(label.Y+label.H)+1, int(failures.Y), bands)
		if len(rules) != 1 {
			t.Fatalf("dark=%v: %d rules between the two probes, want 1: %v", dark, len(rules), rules)
		}
		rule := imagePixel(img, x, rules[0])
		delta := channelSum(rule) - channelSum(face)
		low, high := -90, -40 // light face: the hairline reads darker
		if dark {
			low, high = 40, 90 // dark face: the hairline reads lighter
		}
		if delta < low || delta > high {
			t.Errorf("dark=%v: the rule is %d off the face (%v vs %v), outside the hairline's %d..%d",
				dark, delta, rule, face, low, high)
		}
	}
}
