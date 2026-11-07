package settingsui

import (
	"image/png"
	"os"
	"testing"
	"time"
)

func TestImportReportRevealsItsSource(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, failed := range []bool{false, true} {
			t.Run(appearanceName(dark)+map[bool]string{false: "/success", true: "/failed"}[failed], func(t *testing.T) {
				a := noticeApp(t, dark)
				path := "/home/user/backups/workspace-integrations.json"
				a.TransferReport = &ImportReport{Path: path, Succeeded: 2, Skipped: 1}
				label := a.copy().IntegrationsImportSummary(2, 1)
				if failed {
					a.TransferReport.Failed = []TransferFailure{{ID: "dev.tools", Reason: "package missing"}}
					label = a.copy().IntegrationsImportFailedDetail("dev.tools: package missing")
				}
				tt := noticeExitPage(t, a, dark)
				r := mustFind(t, tt, label)
				tt.Move(r.X+r.W/2, r.Y+r.H/2)
				tt.Frame()
				tt.Advance(700 * time.Millisecond)
				tt.Frame()
				if tag := os.Getenv("FLOTER_P13JD_DUMP"); tag != "" {
					name := "report-source"
					if failed {
						name = "failed-report-source"
					}
					f, err := os.Create("/tmp/floter-P13-jd-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
					if err != nil {
						t.Fatal(err)
					}
					if err := png.Encode(f, tt.Image()); err != nil {
						t.Fatal(err)
					}
					f.Close()
				}
				if !tt.HasText(path) {
					t.Error("import report hides its source path on hover")
				}
				if next := mustFind(t, tt, label); next != r {
					t.Errorf("source tip moved report: %+v -> %+v", r, next)
				}
				tt.Move(0, 0)
				tt.Frame()
				tt.Frame()
				if tt.HasText(path) {
					t.Error("report source remains after pointer leaves")
				}
				a.TransferReport = nil
				tt.Frame()
				if tt.HasText(label) || tt.HasText(path) {
					t.Error("retired report retained content")
				}
			})
		}
	}
}
