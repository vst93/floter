package settingsui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestImportReportLongFailureStaysInItsColumn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			reason := "open /home/user/" + strings.Repeat("workspace", 20) + ".json: permission denied"
			a.TransferReport = &ImportReport{Failed: []TransferFailure{{ID: "dev.workspace", Reason: reason}}}
			body := a.copy().IntegrationsImportFailedDetail("dev.workspace: " + reason)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				ui.Column(c).Width(340).Padding(12).Gap(8).Children(func() { a.transferReport(c, a.copy()); ui.Text(c, "Next integration") })
			}, 360, 320)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			if tag := os.Getenv("FLOTER_P13JF_DUMP"); tag != "" {
				f, err := os.Create("/tmp/floter-P13-jf-report-bounds-" + appearanceName(dark) + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
			r := mustFind(t, tt, body)
			if r.X+r.W > 328 {
				t.Errorf("import failure overflows the report column: %+v right=328", r)
			}
			next := mustFind(t, tt, "Next integration")
			if r.H <= next.H {
				t.Errorf("long report did not wrap: %+v", r)
			}
			if next.Y < r.Y+r.H {
				t.Errorf("wrapped report overlaps following content: report=%+v next=%+v", r, next)
			}
		})
	}
}
