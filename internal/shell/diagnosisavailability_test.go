package shell

import (
	"context"
	"testing"
)

func checkDiagnosisAvailability(t *testing.T, enabled bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := diagnosisCommandFixture(t, dark)
			f.entry.Enabled = !enabled
			if enabled {
				f.entry.State = "disabled"
			}
			f.write(t)
			f.a.RefreshIntegrations(context.Background())
			f.a.Settings.Actions.SetCommandEnabled(f.entry.ID, "checkping", true)
			initial := 1
			if enabled {
				initial = 0
			}
			if len(f.a.Launcher.Commands) != initial {
				t.Fatalf("initial commands=%d want=%d", len(f.a.Launcher.Commands), initial)
			}
			f.request(t, 0, "ok", nil, false)
			done := f.start(t)
			f.waitStarted(t, 0)
			f.entry.Enabled = enabled
			f.entry.State = map[bool]string{false: "disabled", true: "enabled"}[enabled]
			f.write(t) // The on-disk state changes before the next inventory refresh.
			f.release(t, 0)
			diagnosisWait(t, done)
			want, scenario := 0, "completion-disable"
			if enabled {
				want, scenario = 1, "completion-enable"
			}
			diagnosisCommandFrame(t, f, scenario, dark, want)
			row := f.a.integrationList()[0]
			if row.Enabled != enabled || row.Diagnosis != "ok" {
				t.Errorf("completion lost current availability or valid diagnosis: %+v", row)
			}
		})
	}
}

func TestDiagnosisExecutionCompletionDisablesCommands(t *testing.T) {
	checkDiagnosisAvailability(t, false)
}
func TestDiagnosisExecutionCompletionEnablesCommands(t *testing.T) {
	checkDiagnosisAvailability(t, true)
}
