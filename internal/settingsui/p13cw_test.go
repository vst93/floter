package settingsui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The integrations card's health block is the one block on the card that was
// drawn at the card's own frame instead of in the card's label column, and its
// two warm banners leaned their heavier rule on the trailing edge.
//
// The reference draws the health block inside its `.extension-detail-block`,
// the same 12px padding its own `h4` heading takes (`extensions.css:1997-2002`),
// so the block's labels and its heading share one column. In this port the
// heading *is* the card row (`a.row(copy.IntegrationsHealth, …)`) and its label
// sits at `rowInset`; `healthDetail` was drawn as a sibling of that row with no
// horizontal inset at all, so `Checks` / `Failures` / the timestamp started at
// the card's first face pixel — 8px left of every label above them — and the
// probe durations ran out to the card's frame. Measured 720×1700, both
// appearances: rows at x=200, the block at x=192, durations ending at x=701
// (`docs/rounds/P13-cw-spec.md`).
//
// The warm rules are the other half: `.extension-health__failure`
// (`extensions.css:2200`) and `.extension-approval-record__stale`
// (`:2171-2179`) both say `border-left: 2px solid var(--text-warning)`, and
// this framework leaves the *uncovered* strip on the far side of a negative
// offset (`mygo/ui/paint.go:1254`), so `InsetShadow(-2, 0, 0, 0, …)` painted the
// 2px on the right.

// healthBlockPage renders the integrations detail card with everything this
// round measures in one frame: a stored probe report with one failing probe,
// and an approval record whose on-disk digest no longer matches.
func healthBlockPage(t *testing.T, dark bool) *ui.Tester {
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
			Health: "Degraded", HealthStatus: "degraded",
			HealthReport: testHealthReport(),
			Approval: &ApprovalRecord{
				At: 1760000000, ApprovedDigest: "sha256-aaaa", CurrentDigest: "sha256-bbbb",
				Enforced: []string{"network"},
			},
		}}
	}
	tt := render(t, a, 720, 1700)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

// findPrefixed returns the first label whose text starts with prefix. The
// stale banner's sentence is built from a long copy string, so matching it by
// its opening words keeps the test out of the translation table.
func findPrefixed(tt *ui.Tester, prefix string) (ui.Rect, bool) {
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, prefix) {
			return tt.Find(text)
		}
	}
	return ui.Rect{}, false
}

// cardFace reads the card's own face extent off one horizontal line that
// carries no ink: the first and last pixel equal to the face at x=400.
func cardFace(img *image.RGBA, y int) (left, right int, face [4]uint8) {
	c := img.RGBAAt(400, y)
	face = [4]uint8{c.R, c.G, c.B, c.A}
	left, right = -1, -1
	for x := 100; x < 400; x++ {
		if img.RGBAAt(x, y) == c {
			left = x
			break
		}
	}
	for x := 760; x > 400; x-- {
		if img.RGBAAt(x, y) == c {
			right = x
			break
		}
	}
	return left, right, face
}

// blockSpan finds a block's own extent along one line inside it: everything
// between the first and the last pixel that is not the card's face.
func blockSpan(img *image.RGBA, y, from, to int, face [4]uint8) (left, right int) {
	left, right = -1, -1
	for x := from; x <= to; x++ {
		c := img.RGBAAt(x, y)
		if [4]uint8{c.R, c.G, c.B, c.A} != face {
			if left < 0 {
				left = x
			}
			right = x
		}
	}
	return left, right
}

// warm reports whether a pixel is the warning ink rather than any of the
// faces it rides on: the token is a saturated warm (`#d97706` in both
// appearances), where the warm plane it sits on is a desaturated one and the
// neutral card face has no warm bias at all.
func warm(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r>>8 > 150 && (r>>8)-(b>>8) > 120 && (r>>8)-(g>>8) > 60
}

// warmRun counts the warning-ink pixels from x walking by step.
func warmRun(img *image.RGBA, x, y, step int) int {
	n := 0
	for ; x > 0 && x < 719 && warm(img.RGBAAt(x, y)); x += step {
		n++
	}
	return n
}

func TestTheHealthBlockSitsInTheCardsLabelColumn(t *testing.T) {
	const inset = 8 // the integration card's own `cardRowInset` (Space(2))
	for _, dark := range []bool{false, true} {
		tt := healthBlockPage(t, dark)
		img := tt.Image()
		health, ok := tt.Find("Health")
		if !ok {
			t.Fatalf("dark=%v: no Health row label", dark)
		}
		checks, ok := tt.Find("Checks")
		if !ok {
			t.Fatalf("dark=%v: no Checks block label", dark)
		}
		failures, ok := tt.Find("Failures")
		if !ok {
			t.Fatalf("dark=%v: no Failures block label", dark)
		}
		if checks.X != health.X || failures.X != health.X {
			t.Errorf("dark=%v: the health block's labels start at x=%.1f/%.1f, the row above them at x=%.1f: the block hangs %.1fpx left of the card's label column",
				dark, checks.X, failures.X, health.X, health.X-checks.X)
		}
		// One line of air between the row's label and the block's: no ink.
		y := int((health.Y + health.H + checks.Y) / 2)
		left, right, _ := cardFace(img, y)
		if left < 0 || right < 0 {
			t.Fatalf("dark=%v: could not read the card's face on y=%d", dark, y)
		}
		if got := checks.X - float32(left); got != inset {
			t.Errorf("dark=%v: the block starts %.1fpx inside the card's face (x=%.1f, face starts at %d), want %dpx",
				dark, got, checks.X, left, inset)
		}
		if got, want := checks.X+checks.W, float32(right)+1-inset; got != want {
			t.Errorf("dark=%v: the block ends at x=%.1f, want the card's content edge at x=%.1f (face ends at %d)",
				dark, got, want, right)
		}
	}
}

func TestTheWarmRuleLeansOnItsLeadingEdge(t *testing.T) {
	const inset = 8
	for _, dark := range []bool{false, true} {
		tt := healthBlockPage(t, dark)
		img := tt.Image()

		health, _ := tt.Find("Health")
		checks, ok := tt.Find("Checks")
		if !ok {
			t.Fatalf("dark=%v: no Checks block label", dark)
		}
		air := int((health.Y + health.H + checks.Y) / 2)
		cardLeft, cardRight, face := cardFace(img, air)

		// The failure card: the block's label, then 4u of air, then the
		// card's own 6u padding — y just inside its face and above its text.
		failures, ok := tt.Find("Failures")
		if !ok {
			t.Fatalf("dark=%v: no Failures block label", dark)
		}
		y := int(failures.Y + failures.H + 24)
		blockLeft, blockRight := blockSpan(img, y, cardLeft, cardRight, face)
		if blockLeft < 0 {
			t.Fatalf("dark=%v: no failure card on y=%d", dark, y)
		}
		lead := warmRun(img, blockLeft, y, 1)
		trail := warmRun(img, blockRight, y, -1)
		if lead != 2 || trail != 1 {
			t.Errorf("dark=%v failure card (x=%d..%d): leading edge %dpx / trailing edge %dpx of warning ink, want 2 / 1 — the reference's `border-left: 2px` is the heavier rule and belongs on the side that introduces the text",
				dark, blockLeft, blockRight, lead, trail)
		}
		if blockLeft != cardLeft+inset {
			t.Errorf("dark=%v failure card starts at x=%d, want the label column at x=%d", dark, blockLeft, cardLeft+inset)
		}

		// The stale banner: its sentence's own line carries no ink in the
		// 12u padding left of it, so the same walk reads the rule.
		banner, ok := findPrefixed(tt, "The manifest has changed")
		if !ok {
			t.Fatalf("dark=%v: no stale-approval banner", dark)
		}
		y = int(banner.Y + banner.H/2)
		blockLeft, blockRight = blockSpan(img, y, cardLeft, cardRight, face)
		if blockLeft < 0 {
			t.Fatalf("dark=%v: no stale banner on y=%d", dark, y)
		}
		if lead := warmRun(img, blockLeft, y, 1); lead != 2 {
			t.Errorf("dark=%v stale banner (x=%d..%d): leading edge %dpx of warning ink, want the reference's 2px",
				dark, blockLeft, blockRight, lead)
		}
		if trail := warmRun(img, blockRight, y, -1); trail != 0 {
			t.Errorf("dark=%v stale banner: trailing edge %dpx of warning ink, want none — the banner's only rule is its leading one",
				dark, trail)
		}
		_ = cardRight
	}
}
