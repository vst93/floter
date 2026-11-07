package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/extensions"
)

func diagnosisFailedLifecycle(t *testing.T, dark bool) *diagnosisFixture {
	t.Helper()
	f := newDiagnosisFixture(t, dark)
	base := filepath.Join(f.dir, "lifecycle")
	f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true, Args: []string{"-test.run=^TestHealthCompletionProvider$", "--", "--floter-health", base}}}
	// Publish before launching the child; there is no partial-write race.
	if err := os.WriteFile(base+".release", []byte("7"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.write(t)
	f.a.Integrations.Refresh(context.Background())
	f.a.Open(SurfaceSettings)
	return f
}

func TestDiagnosisPhaseLiveHealth(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := diagnosisFailedLifecycle(t, dark)
			f.request(t, 0, "ok", nil, false)
			done := f.start(t)
			t.Cleanup(func() { f.release(t, 0); diagnosisWait(t, done) })
			f.waitStarted(t, 0) // Lifecycle has completed; provider is still waiting.
			diagnosisBoundaryFrame(t, f, "live-health", dark)
			row := f.a.integrationList()[0]
			if row.HealthStatus != extensions.HealthUnhealthy || !row.Broken || row.HealthReport == nil || len(row.HealthReport.Probes) != 1 || row.HealthReport.Probes[0].ExitCode != 7 {
				t.Errorf("completed lifecycle failure stays invisible while provider waits: health=%q broken=%v report=%+v", row.HealthStatus, row.Broken, row.HealthReport)
			}
		})
	}
}

func TestDiagnosisPhaseProtocolFailure(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := diagnosisFailedLifecycle(t, dark)
			diagnosisWrite(t, filepath.Join(f.dir, "request-0.json"), "invalid diagnosis response")
			f.release(t, 0)
			diagnosisWait(t, f.start(t))
			tt := diagnosisBoundaryFrame(t, f, "protocol-failure", dark)
			row := f.a.integrationList()[0]
			if !row.DiagnosisFailed || !strings.Contains(row.Diagnosis, extensions.HealthUnhealthy) || !strings.Contains(row.Diagnosis, "invalid diagnose answer") {
				t.Errorf("provider protocol failure erased lifecycle reason: %q", row.Diagnosis)
			}
			if !tt.HasText("Unhealthy; extensions: invalid diagnose answer: json: cannot unmarshal string into Go value of type extensions.Diagnosis") {
				t.Error("both diagnostic failure reasons are not visible in the summary")
			}
		})
	}
}
