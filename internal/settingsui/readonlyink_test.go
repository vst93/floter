package settingsui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The read-only rows' ink: the emphasis rung belongs to the answer.
//
// Two rows on the Integrations page offer nothing to press and still wore the
// settings row's emphasis step (`rowTitle`: `t.FontSize+1`, 580, `TextStrong`):
//
//   - the Detected list's path under the tool's own name. Measured, the path
//     resolved 208 levels against the plane — the same as the tool's name
//     (208), 87 levels above the description (121). On a list of programs the
//     binary path was the loudest thing on the card.
//   - the approval record's own title, and the `Permissions` line above its
//     pills. Measured, the title resolved 202 (light) / 198 (dark) — the same
//     as the three *values* it introduces (202 / 198), 80 levels above the
//     names of those values (122 / 115).
//
// P13-bt 重定基线（`docs/liquid-glass.md` §6.38）：a path is data and takes the
// rung this build already gives data (`rowLine{Mono: true}`: mono caption, 500,
// `TextMuted`); a block label takes the rung the block already gives its own
// group labels (`Enforced` / `Disclosure`: caption, 620, `TextSecondary`). The
// emphasis rung is left to the answer. These pins hold the resolved ink and the
// line box, not the requested token, because the contract is a claim about what
// a viewer reads.

// detectedPathApp is the Detected list's one row: a program the machine has,
// with a path under it.
func detectedPathApp() *App {
	return oneIntegrationApp(Integration{
		ID: "ok", Name: "Connected Tool", Enabled: true, Running: true, RuntimeAvailable: true,
	}, Actions{
		Detected: func() []DetectedTool {
			return []DetectedTool{{
				Name: "Ghost Tool", Path: "/usr/bin/ghost",
				Description: "A program the machine already has.", RuntimeAvailable: true,
			}}
		},
		ConnectDetected: func(DetectedTool) {},
	})
}

// detectedPathPage draws the Integrations page with the Detected row on it.
func detectedPathPage(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	tt, _ := renderIn(t, detectedPathApp(), 720, 1600, dark)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("open Integrations: %v", err)
	}
	tt.Frame()
	return tt
}

// TestTheDetectedPathIsQuieterThanTheTool pins the pair: the path under a
// detected program is data, so it sits one whole rung below the program's own
// name, on the card's text column, in the caption line box.
func TestTheDetectedPathIsQuieterThanTheTool(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := detectedPathPage(t, dark)
		name := mustFind(t, tt, "Ghost Tool")
		path := mustFind(t, tt, "/usr/bin/ghost")
		desc := mustFind(t, tt, "A program the machine already has.")
		nameInk, pathInk, descInk := inkSpan(tt, name), inkSpan(tt, path), inkSpan(tt, desc)

		if nameInk-pathInk < 60 {
			t.Errorf("%s: the path resolves %d levels and the tool's name %d — the path must sit a "+
				"whole rung below the name (§6.38)", appearance, pathInk, nameInk)
		}
		if pathInk > 150 {
			t.Errorf("%s: the path resolves %d levels — a path is data, not a title (§6.38)",
				appearance, pathInk)
		}
		// The path is never louder than the hint above it: both are "not the name".
		if pathInk > descInk {
			t.Errorf("%s: the path resolves %d levels and the description above it %d — a path "+
				"under a row is data, never the loudest line on the card (§6.38)",
				appearance, pathInk, descInk)
		}
		// The line box is the caption step, not the emphasis step: a reverted size
		// would grow it back to the 23px of a `t.FontSize+1` line.
		if path.H > 20 {
			t.Errorf("%s: the path's line box is %v tall, want the caption step (≤ 20px) — the row "+
				"kept the emphasis size (§6.38)", appearance, path.H)
		}
		if path.Y <= name.Y {
			t.Errorf("%s: the path sits at y=%v, not under the tool's name at y=%v",
				appearance, path.Y, name.Y)
		}
		// The path keeps the card's text column, left of the chip-indented name:
		// this round moved ink and size, not the row.
		if path.X >= name.X {
			t.Errorf("%s: the path starts at x=%v, not on the card's own column left of the "+
				"chip-indented name at x=%v", appearance, path.X, name.X)
		}
	}
}

// TestTheApprovalBlockLabelIsNotTheAnswer pins the ladder of one read-only
// block: the block's title and the `Permissions` line that heads its pills take
// the group-label rung (one rung, both of them), while the facts' values keep
// the emphasis rung they won in P13-ak.
func TestTheApprovalBlockLabelIsNotTheAnswer(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := approvalRecord(t, dark)
		head := mustFind(t, tt, en.ApprovalRecord)
		perms := mustFind(t, tt, en.ApprovalRecordPermissions)
		factName := mustFind(t, tt, en.ApprovalRecordApprovedAt)
		value := mustFind(t, tt, approvalApprovedAt(1700000000))
		headInk, permsInk := inkSpan(tt, head), inkSpan(tt, perms)
		valueInk := inkSpan(tt, value)

		if headInk > 150 {
			t.Errorf("%s: the block's title resolves %d levels — a title over a record is a label, "+
				"not the answer (§6.38)", appearance, headInk)
		}
		if valueInk-headInk < 40 {
			t.Errorf("%s: the title resolves %d levels and the fact's value %d — the title must not "+
				"out-shout (or match) the answer it introduces (§6.38)", appearance, headInk, valueInk)
		}
		// One role, one rung: the title and the permissions line are the same
		// label at the head of a block, so they resolve the same ink.
		if delta := permsInk - headInk; delta > 8 || delta < -8 {
			t.Errorf("%s: the block title resolves %d levels and the permissions label %d — both "+
				"are the same label over a block and take one rung (§6.38)", appearance, headInk, permsInk)
		}
		if head.Y >= factName.Y {
			t.Errorf("%s: the title sits at y=%v, not above the record's first name at y=%v",
				appearance, head.Y, factName.Y)
		}
		if head.X != factName.X {
			t.Errorf("%s: the title starts at x=%v but the record's names at x=%v — the label left "+
				"the block's text column", appearance, head.X, factName.X)
		}
	}
}

// TestTheEmptyApprovalPermissionsKeepsItsAnswerQuiet covers the no-permissions
// arm of the same row: its `None` line is the row's answer, so it stays on the
// quiet rung under the label instead of taking the emphasis step.
func TestTheEmptyApprovalPermissionsKeepsItsAnswerQuiet(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := New(newStore(t), Actions{})
		a.Integrations = func() []Integration {
			return []Integration{{
				ID: "dev.floter.bare", Name: "Bare Tool", Enabled: true, Running: true,
				Version: "1.0.0", Description: "A tool that asks for nothing.",
				Approval: &ApprovalRecord{At: 1700000000, ApprovedDigest: "sha256-aaaa",
					CurrentDigest: "sha256-aaaa"},
			}}
		}
		tt, _ := renderIn(t, a, 760, 1400, dark)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		label := mustFind(t, tt, en.ApprovalRecordPermissions)
		none := mustFind(t, tt, en.ApprovalRecordPermissionsNone)
		if ink := inkSpan(tt, none); ink > 150 {
			t.Errorf("%s: the `None` line resolves %d levels — the answer of a row stays off the "+
				"emphasis rung (§6.38)", appearance, ink)
		}
		if none.Y <= label.Y {
			t.Errorf("%s: the `None` line sits at y=%v, not under its label at y=%v",
				appearance, none.Y, label.Y)
		}
	}
}

// TestDumpP13BT writes the frames this round measured: the Detected list and
// the approval record block, both appearances. `FLOTER_P13BT_TAG` names the
// half (before / after) so one switch produces both sides of the comparison.
func TestDumpP13BT(t *testing.T) {
	if os.Getenv("FLOTER_P13BT_DUMP") == "" {
		t.Skip("set FLOTER_P13BT_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13BT_TAG")
	if tag == "" {
		tag = "after"
	}
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := detectedPathPage(t, dark)
		writePNG(t, filepath.Join("/tmp", "floter-P13-bt-detected-"+tag+"-"+appearance+".png"), tt)
		name, path := mustFind(t, tt, "Ghost Tool"), mustFind(t, tt, "/usr/bin/ghost")
		t.Logf("%s detected: name %v span=%d | path %v span=%d", appearance,
			name, inkSpan(tt, name), path, inkSpan(tt, path))

		at := approvalRecord(t, dark)
		writePNG(t, filepath.Join("/tmp", "floter-P13-bt-approval-"+tag+"-"+appearance+".png"), at)
		title, perms := mustFind(t, at, en.ApprovalRecord), mustFind(t, at, en.ApprovalRecordPermissions)
		fact, value := mustFind(t, at, en.ApprovalRecordApprovedAt), mustFind(t, at, approvalApprovedAt(1700000000))
		t.Logf("%s approval: title %v span=%d | perms %v span=%d | fact %v span=%d | value %v span=%d",
			appearance, title, inkSpan(at, title), perms, inkSpan(at, perms),
			fact, inkSpan(at, fact), value, inkSpan(at, value))
	}
}
