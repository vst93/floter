package shell

import (
	"image/png"
	"os"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func dumpDiagnosisExecution(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JS_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-js-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkDiagnosisBusy(t *testing.T, competing bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			// An optional probe keeps the existing health-refresh outlet visible.
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: false, Args: []string{"-test.run=^TestHealthCompletionProvider$"}}}
			f.entry.State = "broken"
			f.write(t)
			f.a.Integrations.Refresh(t.Context())
			f.request(t, 0, "ok", nil, false)
			f.request(t, 1, "ok", nil, true)
			tt := healthRefreshFrame(t, f, "unused", dark)
			copy := i18n.For("zh").Settings
			settled := make(chan struct{}, 8)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			if err := tt.Click(copy.IntegrationsCheck); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			f.waitStarted(t, 0)
			calls := 0
			scenario := "repeat"
			if competing {
				scenario = "competing"
				f.a.Settings.Actions.ReprobeHealth = func(string) { calls++ }
				f.a.Settings.Actions.RepairIntegration = func(string, func(error)) { calls++ }
				f.a.Settings.Actions.UninstallIntegration = func(string, string, string, func(error)) { calls++ }
				tt.Frame()
				tt.Frame()
				for _, label := range []string{copy.IntegrationsHealthReprobe, copy.IntegrationsRecheck} {
					if err := tt.Click(label); err == nil {
						tt.Frame()
					}
				}
				if calls != 0 {
					t.Errorf("pending diagnosis permits competing actions: %d", calls)
				}
			} else {
				tt.Frame()
				if err := tt.Click(copy.IntegrationsCheck); err == nil {
					tt.Frame()
				}
				// Also cover duplicate dispatch outside a rendered click.
				f.a.Settings.Actions.DiagnoseIntegration(f.entry.ID)
				f.a.diagnosisMu.Lock()
				epoch := f.a.diagnosisEpochs[f.entry.ID]
				f.a.diagnosisMu.Unlock()
				if epoch != 1 {
					t.Errorf("pending diagnosis starts repeatedly: epoch=%d", epoch)
				}
			}
			tt.Frame()
			dumpDiagnosisExecution(t, tt, scenario, dark)
			f.release(t, 0)
			f.waitNotice(t)
			for i := 0; i < 2; i++ {
				select {
				case <-settled:
				case <-time.After(6 * time.Second):
					t.Error("diagnosis never entered and settled its row busy state")
					return
				}
			}
			f.a.Settings.Actions.Invalidate = nil
			tt.Frame()
			tt.Frame()
			// The original outlet must be reachable again after settlement.
			if competing {
				if err := tt.Click(copy.IntegrationsHealthReprobe); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if calls != 1 {
					t.Error("settled diagnosis leaves health refresh locked")
				}
			} else {
				f.a.Settings.Actions.DiagnoseIntegration = func(string) { calls++ }
				if err := tt.Click(copy.IntegrationsCheck); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if calls != 1 {
					t.Error("settled diagnosis leaves Check locked")
				}
			}
		})
	}
}

func TestDiagnosisExecutionRejectsRepeatedCheck(t *testing.T)  { checkDiagnosisBusy(t, false) }
func TestDiagnosisExecutionLocksCompetingActions(t *testing.T) { checkDiagnosisBusy(t, true) }
