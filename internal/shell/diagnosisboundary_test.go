package shell

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func diagnosisBoundaryFrame(t *testing.T, f *diagnosisFixture, scenario string, dark bool) *ui.Tester {
	t.Helper()
	f.a.Settings.Page = settingsui.PageIntegrations
	f.a.Settings.Actions.Detected = nil
	tt := ui.NewTester(f.a.View, 960, 2400)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	openSettingsToolForTest(t, f.a, tt)
	if tag := os.Getenv("FLOTER_P13JQ_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-jq-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	return tt
}

func TestDiagnosisBoundaryRemovedProbes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true}}
			f.write(t)
			f.a.Integrations.Refresh(context.Background())
			f.manifest.Lifecycle.Probes = nil
			f.write(t) // Disk changes before the UI's next inventory refresh.
			f.a.onScreen = func() bool { return true }
			f.a.Open(SurfaceSettings)
			clearNotices(f.a)
			f.request(t, 0, "error", []extensions.DiagnosisCheck{{ID: "old", Status: "error", Message: "过期检查仍被运行"}}, true)
			diagnosisWait(t, f.start(t))
			notices := noticeTitles(f.a)
			diagnosisBoundaryFrame(t, f, "removed-probes", dark)
			if _, err := os.Stat(filepath.Join(f.dir, "request-0.started")); err == nil {
				t.Error("removed lifecycle declaration still launches the stale provider")
			}
			row := f.a.integrationList()[0]
			if row.Diagnosis != "" || len(notices) != 0 {
				t.Errorf("removed probes published stale diagnosis/notice: %+v", row.Diagnosis)
			}
			current, _ := f.a.Integrations.Inventory().WithID(f.entry.ID)
			if len(current.Manifest.Lifecycle.Probes) != 0 {
				t.Error("removed probes were not refreshed into the current inventory")
			}
		})
	}
}

func TestDiagnosisBoundaryDiskOwner(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			f.a.onScreen = func() bool { return true }
			f.a.Open(SurfaceSettings)
			clearNotices(f.a)
			f.request(t, 0, "error", []extensions.DiagnosisCheck{{ID: "old", Status: "error", Message: "旧声明诊断失败"}}, false)
			done := f.start(t)
			f.waitStarted(t, 0)
			f.manifest.Provider.ArgsPrefix[3] = t.TempDir()
			f.write(t) // No lifecycle probes: completion must still reread disk.
			f.release(t, 0)
			diagnosisWait(t, done)
			notices := noticeTitles(f.a)
			tt := diagnosisBoundaryFrame(t, f, "disk-owner", dark)
			row := f.a.integrationList()[0]
			if row.Diagnosis != "" || tt.HasText("旧声明诊断失败") || len(notices) != 0 {
				t.Error("provider completed against stale disk owner and published old facts")
			}
			f.dir = f.manifest.Provider.ArgsPrefix[3]
			f.request(t, 0, "ok", []extensions.DiagnosisCheck{{ID: "new", Status: "ok", Message: "当前声明检查正常"}}, true)
			diagnosisWait(t, f.start(t))
			tt.Frame()
			tt.Frame()
			if !tt.HasText("当前声明检查正常") {
				t.Error("fresh diagnosis cannot recover after disk owner changes")
			}
		})
	}
}
