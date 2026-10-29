package settingsui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/i18n"
)

// P11-be: the integration card's two folded blocks each had their own answer to
// "where does this go?".
//
//   - The permission audit's switch was drawn **twice**: the row's trailing pill
//     (the card's one control slot, where every other row puts its control) and
//     a second, full-width button at the top of the folded block. `cardRow`
//     paints its `below` callbacks every frame (`rows.go:1035-1040`), so the
//     second one was on screen **while the audit was folded too** — three
//     "Permissions" words on one surface, one of them a 660u band.
//   - The diagnostics block was drawn as a bare `ui.Row`/`ui.Text`/`ui.Grid`
//     instead of a card row, so it had no separator, no row floor and no inset:
//     its label sat flush against the pills of the row above (reading as their
//     caption) and its check grid started at the card's own edge, one whole
//     inset left of every label above it.
//
// The two pins below are one per defect. The first is a source contract (the
// toggle is one call site); the second is geometry (both the block's label and
// its check rows land on the card's text column).

// auditFixture is the integration both pins draw: one enforced and one declared
// permission (so the audit has both tiers), an approval record (so the block
// above the diagnostics is a full one) and one probe check whose status word and
// message are unique in the frame, so `Tester.Find` cannot land on a twin.
func auditFixture() []Integration {
	return []Integration{{
		ID: "dev.floter.audit", Name: "Audit Tool", Enabled: true, Running: true,
		Version:     "1.0.0",
		Description: "A tool that asks for two things.",
		Permissions: []string{"clipboard-write", "filesystem-read"},
		Enforced:    map[string]bool{"clipboard-write": true},
		Approval: &ApprovalRecord{
			At: 1700000000, ApprovedDigest: "sha256-aaaa", CurrentDigest: "sha256-aaaa",
			Enforced: []string{"clipboard-write"}, Disclosure: []string{"filesystem-read"},
		},
		DiagnosisStatus: "healthy",
		DiagnosisChecks: []DiagnosisCheck{{ID: "version", Status: "probe-ok", Message: "probe answered"}},
	}}
}

// TestThePermissionAuditHasOneToggle is the first defect's pin, in two arms:
// the source has exactly one `actionButton(c, copy.IntegrationsPermissions)`
// call site, and the drawn frame shows the word twice — the row's label and the
// row's pill — instead of three times.
func TestThePermissionAuditHasOneToggle(t *testing.T) {
	source, err := os.ReadFile("settingsui.go") // #nosec G304 -- the package's own source
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(source), "actionButton(c, copy.IntegrationsPermissions)"); got != 1 {
		t.Errorf("the Permissions toggle has %d call sites in settingsui.go, want 1: "+
			"the row's control slot is the only place this card puts a control, and a second "+
			"button in the folded block is on screen even while the audit is closed", got)
	}
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	a.Integrations = auditFixture
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := countText(tt.Texts(), en.IntegrationsPermissions); got != 2 {
		t.Errorf("the frame shows %q %d times, want 2 (the row's label and the row's pill): %q",
			en.IntegrationsPermissions, got, tt.Texts())
	}
}

// TestTheDiagnosticsBlockSitsOnTheCardsTextColumn is the second defect's pin.
//
// The card's blocks are rows: their labels share one column (the row's inset
// plus its leading rule slot), and anything a block draws under its label sits
// in that same column. The diagnostics block drew neither — its label and its
// check grid both started at the card's own edge, which is why the label read as
// the caption of the permission pills above it.
//
// The reference's font is required: the probe compares text boxes inside one
// frame, and a machine without the shipped font lays the same view out
// differently.
func TestTheDiagnosticsBlockSitsOnTheCardsTextColumn(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	a.Integrations = auditFixture
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The card's own label column, read from a row the card draws with the
	// shared frame.
	row := mustFind(t, tt, en.ApprovalRecord)
	label := mustFind(t, tt, en.IntegrationsDiagnostics)
	if label.X != row.X {
		t.Errorf("the diagnostics block label starts at x=%v but the card's row labels at x=%v "+
			"(delta %v): the block is not on the card's text column", label.X, row.X, label.X-row.X)
	}
	// The block's own content — the check row's status word — is under the
	// label, so it must be on the same column too. It was flush with the card's
	// edge (`diagnosisCheckRow`'s `Padding(2, 0)`), which put it left of every
	// label above it.
	check := mustFind(t, tt, "Probe-ok")
	if check.X != row.X {
		t.Errorf("the check row starts at x=%v but the card's row labels at x=%v "+
			"(delta %v): the check grid is not on the card's text column", check.X, row.X, check.X-row.X)
	}
	// And it is *below* the label, not beside it.
	if check.Y <= label.Y {
		t.Errorf("the check row y=%v is not below the block label y=%v", check.Y, label.Y)
	}
}

// The P11-be visual evidence: the integrations card's permission audit and
// diagnostics block, in both appearances and in both fold states, because the
// round is about what those two blocks look like when they are and are not open.
//
// Opt-in for the same reason as the other dumps: a test run should not litter
// /tmp; the round that produced the report ran it once with the switch on.
func TestDumpP11BEAuditBlockPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, open := range []bool{false, true} {
			state := "closed"
			if open {
				state = "open"
			}
			a := New(newStore(t), Actions{})
			a.Integrations = auditFixture
			if open {
				a.auditOpen = map[string]bool{"dev.floter.audit": true}
			}
			tt, tokens := renderIn(t, a, 860, 1400, dark)
			if err := tt.Click("Integrations"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			row := mustFind(t, tt, en.ApprovalRecord)
			label := mustFind(t, tt, en.IntegrationsDiagnostics)
			check := mustFind(t, tt, "Probe-ok")
			t.Logf("%s %s: row-label x=%.0f y=%.0f | diagnostics label x=%.0f y=%.0f | check x=%.0f y=%.0f | Permissions x%d",
				appearance, state, row.X, row.Y, label.X, label.Y, check.X, check.Y,
				countText(tt.Texts(), en.IntegrationsPermissions))
			writePNG(t, filepath.Join("/tmp", "floter-P11-be-audit-"+appearance+"-"+state+".png"), tt)
			_ = tokens
		}
	}
}
