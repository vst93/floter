package shell

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func dumpDiagnosisRecovery(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JP_DUMP"); tag != "" {
		path := "/tmp/floter-P13-jp-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png"
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func diagnosisRecoveryFrame(t *testing.T, f *diagnosisFixture, scenario string, dark bool) *ui.Tester {
	t.Helper()
	f.a.Open(SurfaceSettings)
	f.a.Settings.Page = settingsui.PageIntegrations
	f.a.Settings.Actions.Detected = nil
	tt := ui.NewTester(f.a.View, 960, 2400)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	openSettingsToolForTest(t, f.a, tt)
	dumpDiagnosisRecovery(t, tt, scenario, dark)
	return tt
}

func waitDiagnosisMarker(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("subprocess did not reach %s", path)
}

// The repository can advance before the UI inventory gets its next refresh.
// A discarded lifecycle check must not become a provider diagnosis or toast.
func TestDiagnosisRecoverySupersededHealth(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			base := filepath.Join(f.dir, "probe")
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true, TimeoutMs: 5000, Args: []string{"-test.run=^TestHealthCompletionProvider$", "--", "--floter-health", base}}}
			f.write(t)
			f.a.Integrations.Refresh(context.Background())
			f.a.onScreen = func() bool { return true }
			f.a.Open(SurfaceSettings)
			clearNotices(f.a)
			f.request(t, 0, "ok", nil, true)
			done := f.start(t)
			waitDiagnosisMarker(t, base+".started")
			f.entry.ProbeReport.CheckedAt = "2026-11-08T08:00:00Z"
			f.write(t) // deliberately do not refresh the in-memory inventory
			if err := os.WriteFile(base+".release", []byte("7"), 0o600); err != nil {
				t.Fatal(err)
			}
			diagnosisWait(t, done)
			titles := noticeTitles(f.a)
			tt := diagnosisRecoveryFrame(t, f, "superseded-health", dark)
			row := f.a.integrationList()[0]
			if len(titles) != 0 || row.Diagnosis != "" || row.DiagnosisFailed {
				t.Errorf("discarded lifecycle check published a diagnosis or notice: diagnosis=%q failed=%v notices=%q", row.Diagnosis, row.DiagnosisFailed, titles)
			}
			if row.HealthReport.CheckedAt != f.entry.ProbeReport.CheckedAt {
				t.Error("discarded check did not refresh the current health report")
			}
			_ = tt
		})
	}
}

func checkDiagnosisRuntimeChange(t *testing.T, change string) {
	t.Helper()
	for _, pending := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			t.Run(map[bool]string{false: "complete", true: "pending"}[pending]+"/"+map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
				f := newDiagnosisFixture(t, dark)
				f.request(t, 0, "error", []extensions.DiagnosisCheck{{ID: "runtime", Status: "error", Message: "旧运行版本检查失败"}}, !pending)
				done := f.start(t)
				f.waitStarted(t, 0)
				if !pending {
					diagnosisWait(t, done)
				}
				switch change {
				case "version":
					f.entry.CurrentVersion = "2.0.0"
				case "root":
					root := t.TempDir()
					f.entry.RuntimeRoot = &root
				}
				f.write(t)
				f.a.Integrations.Refresh(context.Background())
				if pending {
					f.release(t, 0)
					diagnosisWait(t, done)
				}
				tt := diagnosisRecoveryFrame(t, f, "runtime-"+change+"-"+map[bool]string{false: "complete", true: "pending"}[pending], dark)
				row := f.a.integrationList()[0]
				if row.Diagnosis != "" || row.DiagnosisFailed || tt.HasText("旧运行版本检查失败") {
					t.Errorf("changed runtime %s inherited stale diagnosis: %q", change, row.Diagnosis)
				}
				// A new check of this runtime must still publish normally.
				f.request(t, 1, "ok", []extensions.DiagnosisCheck{{ID: "runtime", Status: "ok", Message: "当前运行版本正常"}}, true)
				diagnosisWait(t, f.start(t))
				tt.Frame()
				tt.Frame()
				if !tt.HasText("当前运行版本正常") {
					t.Error("current runtime check was lost")
				}
			})
		}
	}
}

func TestDiagnosisRecoveryRuntimeVersion(t *testing.T) { checkDiagnosisRuntimeChange(t, "version") }
func TestDiagnosisRecoveryRuntimeRoot(t *testing.T)    { checkDiagnosisRuntimeChange(t, "root") }

func TestDiagnosisRecoveryFailedChecksOverrideOK(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			f.request(t, 0, "ok", []extensions.DiagnosisCheck{{ID: "account", Status: "error", Message: "需要重新登录"}}, true)
			diagnosisWait(t, f.start(t))
			tt := diagnosisRecoveryFrame(t, f, "failed-checks", dark)
			row := f.a.integrationList()[0]
			if !row.DiagnosisFailed || row.DiagnosisStatus != "ok" || !tt.HasText("需要重新登录") || tt.HasText("Ok") {
				t.Errorf("failed provider checks still display an OK summary: failed=%v status=%q", row.DiagnosisFailed, row.DiagnosisStatus)
			}
		})
	}
}

func TestDiagnosisRecoveryLifecycleFailureSurvivesProviderOK(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			base := filepath.Join(f.dir, "failed-lifecycle")
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true, Args: []string{"-test.run=^TestHealthCompletionProvider$", "--", "--floter-health", base}}}
			if err := os.WriteFile(base+".release", []byte("7"), 0o600); err != nil {
				t.Fatal(err)
			}
			f.write(t)
			f.a.Integrations.Refresh(context.Background())
			f.request(t, 0, "ok", []extensions.DiagnosisCheck{{ID: "configuration", Status: "ok", Message: "配置检查正常"}}, true)
			diagnosisWait(t, f.start(t))
			tt := diagnosisRecoveryFrame(t, f, "lifecycle-failure", dark)
			row := f.a.integrationList()[0]
			if !row.DiagnosisFailed || row.Diagnosis != extensions.HealthUnhealthy || row.DiagnosisStatus != "ok" || !tt.HasText("Unhealthy") || !tt.HasText("配置检查正常") {
				t.Errorf("provider OK erased a real lifecycle failure: diagnosis=%q failed=%v status=%q", row.Diagnosis, row.DiagnosisFailed, row.DiagnosisStatus)
			}
		})
	}
}
