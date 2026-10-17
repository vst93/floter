package settingsui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The integrations page's health / approval blocks were the last region of the
// settings panel with no field-by-field comparison against the reference:
// `main:src/ExtensionsPanel.tsx:2892-2975` draws an `.extension-health` block
// and an `.extension-approval-record` block, and `main:src/styles/extensions.css`
// names each of their pieces. The port had flattened every one of them into a
// plain ruled card row with a trailing word:
//
//	.extension-health__status      the verdict *chip* (four faces), not a word
//	.extension-health__probe       11px Check/X, monospace id, right-aligned ms
//	.extension-health__failure     the warm failure card, 2px warning edge
//	.extension-approval-record__permission   the permission *pill* family
//	.extension-approval-record__stale        the changed-manifest banner
//
// These pin the faces. The frame is drawn over the panel's own plane, so a
// token's composited value is the readable target (see `panelOver`).

// integrationWithHealth renders one integration card carrying a stored probe
// report, and returns the frame plus the app so the chip's own plane can be
// read.
func integrationWithHealth(t *testing.T, report *HealthReport, approval *ApprovalRecord) (*ui.Tester, *App) {
	t.Helper()
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.probed", Name: "Probed Tool", Enabled: true,
			Health: "Degraded", HealthReport: report, Approval: approval,
		}}
	}
	tt := render(t, a, 720, 1100)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt, a
}

func testHealthReport() *HealthReport {
	return &HealthReport{
		CheckedAt: "2026-10-12T09:30:00Z",
		Probes: []HealthProbe{
			{ID: "up", Passed: true, DurationMs: 12, ExitCode: 0},
			{ID: "extras", Passed: false, DurationMs: 30, ExitCode: 3, Stderr: "no extras here", Retryable: true},
		},
	}
}

// The verdict is a *chip* wearing the warm raised plane and the warning
// keyline (`extensions.css:2184-2188`), not a bare grey word trailing a ruled
// row: a degraded/unhealthy run is a warning the block must read as one.
func TestTheHealthVerdictWearsItsOwnChip(t *testing.T) {
	for _, dark := range []bool{false, true} {
		store := newStore(t)
		if dark {
			if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
				t.Fatal(err)
			}
		}
		a := New(store, Actions{})
		a.Integrations = func() []Integration {
			return []Integration{{ID: "dev.floter.bad", Name: "Bad Tool", Enabled: true, Health: "Degraded"}}
		}
		tt := render(t, a, 720, 900)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()

		word, ok := tt.Find("Degraded")
		if !ok {
			t.Fatalf("the verdict word is missing: %q", tt.Texts())
		}
		// Read the chip's plane just inside its leading edge, on the word's
		// own centre line (the padding pair puts the fill before the glyphs).
		img := tt.Image()
		cy := int(word.Y + word.H/2)
		got := imagePixel(img, int(word.X)-5, cy)
		// It is *not* the plain page plane a bare word sat on: the chip grew
		// a fill of its own...
		plain := imagePixel(img, 200, cy)
		if panelClose(got, plain, 3) {
			t.Errorf("the verdict word still sits on the plain row (%v): it did not grow a chip", got)
		}
		// ...and that fill is the warm pane (red heavier than blue), which is
		// `--glass-raised-warm`'s whole point. The chip is what says "degraded"
		// as a state rather than as a trailing adjective.
		if !(got.R > got.B) {
			t.Errorf("the verdict chip's plane is not warm: %v", got)
		}
	}
}

// A probe line is the reference's compact row, not a ruled card row: the id is
// the *monospace* register (`extensions.css:2197` — a probe id is an API name)
// and the duration is pushed to the trailing edge. Measuring the drawn boxes is
// what tells the two apart: a ruled card row would give the id the body
// emphasis face and put the duration tight under it.
func TestProbeLinesAreTheReferenceCompactRows(t *testing.T) {
	tt, _ := integrationWithHealth(t, testHealthReport(), nil)

	id, ok := tt.Find("up")
	if !ok {
		t.Fatalf("the passing probe id is missing: %q", tt.Texts())
	}
	dur, ok := tt.Find("12ms")
	if !ok {
		t.Fatalf("the passing probe duration is missing: %q", tt.Texts())
	}
	// The duration is the trailing fact: it sits to the *right* of the id and
	// on the same line.
	if dur.X <= id.X {
		t.Errorf("the duration is not trailing the id: id x=%.0f, duration x=%.0f", id.X, dur.X)
	}
	if abs(float64(dur.Y-id.Y)) > 1 {
		t.Errorf("the duration is not on the probe's own line: id y=%.0f, duration y=%.0f", id.Y, dur.Y)
	}
	// The probe line is the caption step, not the body emphasis a card row's
	// label wears: the drawn text box is shorter than the row's own label.
	// The group label above it ("Checks") is the same caption step as the
	// probe id — it is the reference's `__label`, one voice with the block —
	// so the comparison is against a *card row's* label, which is what the
	// old ruled-row layout would have given the id.
	label, ok := tt.Find("Health")
	if !ok {
		t.Fatalf("the Health row label is missing: %q", tt.Texts())
	}
	if id.H >= label.H {
		t.Errorf("the probe id is drawn at the label's own step: id h=%.0f, label h=%.0f", id.H, label.H)
	}
	// The monospace face is measurably narrower per glyph than the interface
	// face for the same caption size: two characters of an api id like "up"
	// cannot show it, so the *shape* is pinned by the drawn mark instead — the
	// passing mark is the reference's literal 11px Check/X, not a text glyph.
	img := tt.Image()
	if !hasInk(img, int(id.X)-14, int(id.Y), int(id.X)-2, int(id.Y+id.H)) {
		t.Errorf("no mark is drawn in the probe line's leading column")
	}
}

// The failure card is the reference's warm raised pane under a 2px warning
// leading rule (`extensions.css:2200-2203`), never a plain ruled row whose
// second line happens to be the exit code.
func TestTheFailureCardWearsTheWarmRasiedPlane(t *testing.T) {
	tt, _ := integrationWithHealth(t, testHealthReport(), nil)
	code, ok := tt.Find("exit: 3")
	if !ok {
		t.Fatalf("the failure's exit code is missing: %q", tt.Texts())
	}
	img := tt.Image()
	// The card fills the row's whole width, so its plane is read on the exit
	// code's own line but *left* of the code's text (which starts ~198): the
	// block's fill is there and nowhere else.
	cy := int(code.Y + code.H/2)
	got := imagePixel(img, 260, cy)
	// The plain row plane is the same column on the Health row's own line — a
	// ruled card row, which is what the block used to be.
	health := mustFind(t, tt, "Health")
	plain := imagePixel(img, 260, int(health.Y+health.H/2))
	if panelClose(got, plain, 3) {
		t.Errorf("the failure block is the plain row plane (%v): it did not grow the warm card", got)
	}
	// The plane is warm (red heavier than blue), which is the whole point of
	// `--glass-raised-warm`.
	if !(got.R > got.B) {
		t.Errorf("the failure card's plane is not warm: %v", got)
	}
}

// The recorded permissions are the reference's pill family
// (`extensions.css:2149-2168`): each id is its own chip, an enforced one
// wearing the raised-quiet + accent keyline its twin in the audit list has, a
// disclosure one the plain icon-surface face. The port drew them as one ruled
// row per id with a trailing tier word, which put a second line's emphasis on
// an id and gave the pill family no face at all.
func TestTheApprovedPermissionsAreTheReferencePills(t *testing.T) {
	record := &ApprovalRecord{
		At: 1700000000, ApprovedDigest: "sha256-aaaabbbbcccc", CurrentDigest: "sha256-ddddeeeeffff",
		Enforced: []string{"clipboard-write"}, Disclosure: []string{"filesystem-read"},
	}
	for _, dark := range []bool{false, true} {
		store := newStore(t)
		appearance := "light"
		if dark {
			appearance = "dark"
			if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
				t.Fatal(err)
			}
		}
		a := New(store, Actions{})
		a.Integrations = func() []Integration {
			return []Integration{{
				ID: "dev.floter.probed", Name: "Probed Tool", Enabled: true, Approval: record,
			}}
		}
		tt := render(t, a, 720, 1100)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()

		// The tier labels name the two groups; the pills are drawn under each
		// heading, on the card's own row (the pill line is a separate draw, so
		// its leading edge is the row inset, not the heading's start).
		mustFind(t, tt, "Floter enforces")
		enforcedPill := mustFind(t, tt, "Write the clipboard")
		mustFind(t, tt, "Declared, not blocked")
		disclosurePill := mustFind(t, tt, "Read files")

		// Each pill's own leading edge band is a chip plane — a fill distinct
		// from the plain row it sits on (the block above the group is a row).
		img := tt.Image()
		// The plane the pills sit on is the card's own row; sample it at the
		// same line, well to the right of both pills.
		plain := imagePixel(img, 600, int(enforcedPill.Y+enforcedPill.H/2))
		// The pill's left padding is 8px, so its plane starts ~8px before the
		// label text; sample there, at the pill's own mid-line.
		enforcedPlane := imagePixel(img, int(enforcedPill.X)-5, int(enforcedPill.Y+enforcedPill.H/2))
		disclosurePlane := imagePixel(img, int(disclosurePill.X)-5, int(disclosurePill.Y+disclosurePill.H/2))
		// The enforced pill's own face is what "floter refused" must read as:
		// it is not the plain row plane, it is the raised-quiet + accent
		// keyline its twin in the audit list wears. (The disclosure pill's fill
		// is the reference's `--glass-control`, which over a dark card is a
		// near-invisible 0.035 lift by design — its 1px `--glass-control-edge`
		// hairline is what draws it, so the fill is not pinned here.) The two
		// tiers keep the reference's *recognisable shape difference*: the
		// enforced pill's face is not the disclosure pill's, so the two groups
		// read apart in the record exactly as they do in the review list.
		if panelClose(enforcedPlane, plain, 3) {
			t.Errorf("[%s enforced] the permission has no pill plane: it is still a bare label", appearance)
		}
		if panelClose(enforcedPlane, disclosurePlane, 3) {
			t.Errorf("[%s] the two permission tiers wear one face: enforced %v, disclosure %v",
				appearance, enforcedPlane, disclosurePlane)
		}

		// The changed-manifest sentence is the reference's own banner, drawn in
		// the block rather than as a nameless row with a lone `!`.
		if !tt.HasText("The manifest has changed since this approval. Re-approve to cover the current version.") {
			t.Errorf("[%s] the changed-manifest banner is missing: %q", appearance, tt.Texts())
		}
	}
}

// abs is the positive difference of two floats, for the line-alignment check.
func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// hasInk reports whether any pixel in the box differs from the plane behind it,
// which is what "a mark is drawn here" means on a rendered frame.
func hasInk(img *image.RGBA, x0, y0, x1, y1 int) bool {
	base := imagePixel(img, 2, y0)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if !panelClose(imagePixel(img, x, y), base, 8) {
				return true
			}
		}
	}
	return false
}

// The Health block's groups are the reference's `__label` lines, not ruled
// card rows: `ExtensionsPanel.tsx:2945-2969` opens the checks group with the
// label and then lists the probe lines *inside it*, so "Checks" is a caption
// at 620 on the secondary ink (`extensions.css:2191`) sitting directly above
// its own content — not a 44u settings row carrying a 44u row of detail below
// it. The difference is measurable: a ruled row would put the card's own
// separator between the label and the row, and the label would sit on the
// card's row floor with the detail block folded under it.
func TestTheHealthGroupsAreLabelledNotRuledRows(t *testing.T) {
	tt, _ := integrationWithHealth(t, testHealthReport(), nil)
	label, ok := tt.Find("Checks")
	if !ok {
		t.Fatalf("the checks label is missing: %q", tt.Texts())
	}
	probe, ok := tt.Find("up")
	if !ok {
		t.Fatalf("the first probe line is missing: %q", tt.Texts())
	}
	// The label leads its own content: the probe line starts to its *left*
	// (the label is the block's heading, the probe carries the id column) and
	// begins on the line below it.
	if probe.Y <= label.Y {
		t.Errorf("the probe line is not below the label: label y=%.0f, probe y=%.0f",
			label.Y, probe.Y)
	}
	// The label is the caption step: a card row's own label is the emphasis
	// step (`t.FontSize + 1`), which the block label must not wear.
	row, ok := tt.Find("Health")
	if !ok {
		t.Fatalf("the Health row label is missing: %q", tt.Texts())
	}
	if label.H >= row.H {
		t.Errorf("the group label is drawn at a card row's step: label h=%.0f, row label h=%.0f",
			label.H, row.H)
	}
}

// The diagnostics block capitalises both words the way the reference's
// `text-transform: capitalize` does (`extensions.css:2331,2341`): the summary
// word and every check's own status, first letter up and the rest left as the
// provider spelled it. A provider's lowercase `ok` therefore reads `Ok`.
func TestTheDiagnosticsWordsAreCapitalised(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "dev.floter.dx", Name: "Dx Tool", Enabled: true,
			DiagnosisStatus: "ok",
			DiagnosisChecks: []DiagnosisCheck{{ID: "version", Status: "ok", Message: "answered"}},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Ok") {
		t.Errorf("the verdict word was not capitalised: %q", tt.Texts())
	}
	if !tt.HasText("Diagnostics") {
		t.Errorf("the diagnostics block label is missing: %q", tt.Texts())
	}
	// The check row prints its status word and its message — the reference's
	// own two cells (`ExtensionsPanel.tsx:2899`). The check's *id* is the
	// React key there, never a third printed fact, so it is not a row.
	if !tt.HasText("answered") {
		t.Errorf("the check's own message is missing: %q", tt.Texts())
	}
}
