package settingsui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The approval record's ink: a record is a value with a name, not a setting
// with a subtitle.
//
// The integration card's folded "approval record" block is four rows of facts
// the host recorded for the user (`批准时间`, `清单摘要`, `磁盘摘要`) — nothing on
// them can be pressed. They were drawn with the plain settings row, whose
// emphasis step belongs to the **setting's name** (`rowTitle`: +1pt at 580 in
// `TextStrong`) and whose second line is a subtitle (`t.TextMuted` at 500). On
// a record that is backwards: the name of the fact wore the loud ink and the
// fact itself — the timestamp, the digest, the thing the user opened the block
// to read — was the quietest text on the row.
//
// P13-ak 重定基线：记录行 (`factRow`) 把两档墨对调 —— 名字退到
// `TextSecondary`/caption、值升到 `TextStrong`/emphasis。 These pins hold the
// resolved ink, not the requested token, because the contract is a claim about
// what a viewer reads.

// inkSpan is the luma span a text box resolves: the distance between the
// lightest and the darkest pixel inside the box. Antialiased edges only ever
// sit *between* the plane and the ink, so the span is the plane-to-ink
// distance — the same number `bandInk` reads, without needing a plane sample
// (a record's key and its value share one face, so the two spans are already
// comparable).
func inkSpan(tt *ui.Tester, r ui.Rect) int {
	img := tt.Image()
	x0, y0 := int(r.X), int(r.Y)
	x1, y1 := x0+int(r.W), y0+int(r.H)
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > img.Bounds().Dx() {
		x1 = img.Bounds().Dx()
	}
	if y1 > img.Bounds().Dy() {
		y1 = img.Bounds().Dy()
	}
	lo, hi := 255, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			l := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			if l < lo {
				lo = l
			}
			if l > hi {
				hi = l
			}
		}
	}
	return hi - lo
}

// approvalFixture is one integration with a full approval record whose two
// digests differ, so each of the block's four rows draws a string no other row
// draws (the shared `auditFixture` uses one digest for both sides, which is the
// right picture for the audit block and an ambiguous one to find a box in).
func approvalFixture() []Integration {
	return []Integration{{
		ID: "dev.floter.audit", Name: "Audit Tool", Enabled: true, Running: true,
		Version:     "1.0.0",
		Description: "A tool that asks for one thing.",
		Permissions: []string{"clipboard-write"},
		Enforced:    map[string]bool{"clipboard-write": true},
		Approval: &ApprovalRecord{
			At: 1700000000, ApprovedDigest: "sha256-aaaa", CurrentDigest: "sha256-bbbb",
			Enforced: []string{"clipboard-write"},
		},
	}}
}

// approvalRecord renders the integration detail card with one full approval
// record open on it and returns the tester.
func approvalRecord(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	a := New(newStore(t), Actions{})
	a.Integrations = approvalFixture
	tt, _ := renderIn(t, a, 760, 1400, dark)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

// approvalFacts are the record's rows: the key string and the value string the
// English frame draws for one approved manifest.
func approvalFacts() [][2]string {
	en := i18n.For("en").Settings
	return [][2]string{
		{en.ApprovalRecordApprovedAt, approvalApprovedAt(1700000000)},
		{en.ApprovalRecordDigest, "sha256-aaaa"},
		{en.ApprovalRecordDigestCurrent, "sha256-bbbb"},
	}
}

// TestTheApprovalRecordPutsTheFactInTheEmphasisRung pins the pair: on every row
// of the record the fact out-inks its name by a visible step, the name stays on
// the quiet rung, and the two lines keep the row's own order and column (this
// round moved ink, not boxes).
func TestTheApprovalRecordPutsTheFactInTheEmphasisRung(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := approvalRecord(t, dark)
		for _, fact := range approvalFacts() {
			key, value := mustFind(t, tt, fact[0]), mustFind(t, tt, fact[1])
			keyInk, valueInk := inkSpan(tt, key), inkSpan(tt, value)
			if valueInk-keyInk < 60 {
				t.Errorf("%s: %q resolves %d levels and its value %q only %d — the fact must "+
					"out-ink the name of the fact by a whole rung (§6.10)",
					appearance, fact[0], keyInk, fact[1], valueInk)
			}
			if keyInk > 150 {
				t.Errorf("%s: the record's key %q resolves %d levels — a name on a record is "+
					"not a setting's title, it is the quiet caption over its value (§6.10)",
					appearance, fact[0], keyInk)
			}
			if value.Y <= key.Y {
				t.Errorf("%s: %q sits at y=%v, not below its name at y=%v — the record reads "+
					"name-then-value, the same order it is written in", appearance, fact[1], value.Y, key.Y)
			}
			if value.X != key.X {
				t.Errorf("%s: %q starts at x=%v but its name at x=%v — the pair left the "+
					"card's text column", appearance, fact[1], value.X, key.X)
			}
		}
	}
}

// TestDumpP13AKApprovalRecord writes the frames this round measured.
func TestDumpP13AKApprovalRecord(t *testing.T) {
	if os.Getenv("FLOTER_P13AK_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13AK_DUMP=1 to write /tmp/floter-P13-ak-*.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := approvalRecord(t, dark)
		for _, fact := range approvalFacts() {
			key, value := mustFind(t, tt, fact[0]), mustFind(t, tt, fact[1])
			t.Logf("%s %q ink %d / %q ink %d", appearance, fact[0], inkSpan(tt, key), fact[1], inkSpan(tt, value))
		}
		path := filepath.Join("/tmp", fmt.Sprintf("floter-P13-ak-approval-%s.png", appearance))
		writePNG(t, path, tt)
	}
}
