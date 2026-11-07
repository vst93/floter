package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func transferDetailDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JG_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jg-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestImportReportLongSourceFitsTooltip(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			path := "/home/user/" + strings.Repeat("workspace", 12) + ".json"
			a.TransferReport = &ImportReport{Path: path, Succeeded: 1}
			tt := noticeExitPage(t, a, dark)
			label := a.copy().IntegrationsImportSummary(1, 0)
			r := mustFind(t, tt, label)
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Frame()
			tt.Advance(700 * time.Millisecond)
			tt.Frame()
			transferDetailDump(t, tt, "source-bounds", dark)
			p := mustFind(t, tt, path)
			if p.X < 0 || p.X+p.W > 720 || p.W > 320 {
				t.Errorf("report source escapes tooltip: %+v", p)
			}
			if p.H <= r.H {
				t.Errorf("long source did not wrap: %+v", p)
			}
		})
	}
}

func TestImportExcludedDetailsExposeFieldCounts(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			a.TransferReport = &ImportReport{Path: "/home/user/workspace.json", Succeeded: 2, ExcludedFields: []ExcludedField{{ExtensionID: "dev.workspace", FieldCount: 2}, {ExtensionID: "dev.build", FieldCount: 5}}}
			tt := noticeExitPage(t, a, dark)
			label := a.copy().IntegrationsImportExcludedFields("dev.workspace, dev.build")
			r := mustFind(t, tt, label)
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Frame()
			tt.Advance(700 * time.Millisecond)
			tt.Frame()
			transferDetailDump(t, tt, "excluded-details", dark)
			if !tt.HasText("dev.workspace: 2\ndev.build: 5") {
				t.Error("excluded-fields notice hides per-integration field counts")
			}
			if next := mustFind(t, tt, label); next != r {
				t.Errorf("details moved report: %+v -> %+v", r, next)
			}
			tt.Move(0, 0)
			tt.Frame()
			tt.Frame()
			if tt.HasText("dev.workspace: 2\ndev.build: 5") {
				t.Error("excluded details remain after pointer leaves")
			}
		})
	}
}

func TestImportFailureTailHasReadableDetails(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			a.TransferReport = &ImportReport{Path: "/home/user/workspace.json", Failed: []TransferFailure{
				{ID: "dev.workspace", Reason: "bad manifest"}, {ID: "dev.build", Reason: "missing script"},
				{ID: "dev.archive", Reason: "missing descriptor"}, {ID: "dev.notes", Reason: "permission denied"},
			}}
			tt := noticeExitPage(t, a, dark)
			label := a.copy().IntegrationsImportFailedDetail("dev.workspace: bad manifest; dev.build: missing script; dev.archive: missing descriptor") + " (" + a.copy().IntegrationsImportFailedMore(1) + ")"
			r := mustFind(t, tt, label)
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Frame()
			tt.Advance(700 * time.Millisecond)
			tt.Frame()
			transferDetailDump(t, tt, "failure-tail", dark)
			// P13-jg S: temporary hover-only tail -> persistent page detail.
			detail := "dev.notes: permission denied"
			if !tt.HasText(detail) {
				t.Error("failure tail cannot reveal the fourth integration or reason")
			}
			if next := mustFind(t, tt, label); next != r {
				t.Errorf("details moved report: %+v -> %+v", r, next)
			}
			tt.Move(0, 0)
			tt.Frame()
			tt.Frame()
			if !tt.HasText(detail) {
				t.Error("failure details depend on hover")
			}
		})
	}
}

func TestImportFailureTailAvailableInPageScroll(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			a.TransferReport = &ImportReport{Path: "/home/user/workspace.json"}
			for n := 0; n < 40; n++ {
				a.TransferReport.Failed = append(a.TransferReport.Failed, TransferFailure{ID: fmt.Sprintf("dev.workspace.%02d", n), Reason: "missing descriptor"})
			}
			tt := noticeExitPage(t, a, dark)
			// Necessary diagnostics must be available on the existing page,
			// including without a pointer and beyond a tooltip's height.
			label := "dev.workspace.39: missing descriptor"
			if !tt.HasText(label) {
				t.Fatal("failure tail is unavailable without hover")
			}
			tt.Scroll(400, 300, 0, 2000)
			tt.Frame()
			tt.Frame()
			last := mustFind(t, tt, label)
			if last.Y < 0 || last.Y+last.H > 900 {
				t.Errorf("last failure cannot be scrolled into view: %+v", last)
			}
			transferDetailDump(t, tt, "failure-scroll", dark)
		})
	}
}
