package shell

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
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
	tt := f.frame(t, scenario, dark)
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
