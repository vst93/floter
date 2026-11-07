package shell

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func TestDiagnosisExecutionProvider(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "--floter-diagnosis-fixture" && i+2 < len(os.Args) && os.Args[i+2] == "describe" {
			fmt.Print(`{"protocolVersion":"1.0","provider":{"id":"dev.floter.check","name":"检查工具","version":"1.0"},"commands":[{"id":"checkping","name":"Check Ping","description":"检查连接","execution":{"program":"self","argsPrefix":["ping"],"mode":"external"}}]}`)
			os.Exit(0)
		}
	}
	TestDiagnosisCurrentProvider(t)
}

func diagnosisCommandFixture(t *testing.T, dark bool) *diagnosisFixture {
	t.Helper()
	f := newDiagnosisFixture(t, dark)
	f.manifest.Provider.ArgsPrefix[0] = "-test.run=^TestDiagnosisExecutionProvider$"
	return f
}

func diagnosisCommandFrame(t *testing.T, f *diagnosisFixture, scenario string, dark bool, want int) {
	t.Helper()
	f.a.Open(SurfaceLauncher)
	f.a.Launcher.SetQuery("checkping")
	tt := ui.NewTester(f.a.View, 960, 600)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	dumpDiagnosisExecution(t, tt, scenario, dark)
	if len(f.a.Launcher.Commands) != want {
		t.Errorf("diagnosis %s leaves launcher commands stale: got=%d want=%d", scenario, len(f.a.Launcher.Commands), want)
	}
	if tt.HasText("Check Ping") != (want == 1) {
		t.Errorf("diagnosis %s and visible search results disagree: want=%d", scenario, want)
	}
}

func checkDiagnosisLifecycleCommands(t *testing.T, recover, disabled bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := diagnosisCommandFixture(t, dark)
			base := filepath.Join(f.dir, "lifecycle-command")
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true, Args: []string{"-test.run=^TestHealthCompletionProvider$", "--", "--floter-health", base}}}
			if disabled {
				f.entry.Enabled, f.entry.State = false, "disabled"
			}
			f.write(t)
			if err := os.WriteFile(base+".release", []byte("7"), 0o600); err != nil {
				t.Fatal(err)
			}
			if recover {
				report, err := extensions.Reprobe(context.Background(), f.paths, f.entry.ID)
				if err != nil || report.Status != extensions.HealthUnhealthy {
					t.Fatalf("failure fixture: %+v %v", report, err)
				}
				if err := os.WriteFile(base+".release", []byte("0"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			f.a.RefreshIntegrations(context.Background())
			f.a.Settings.Actions.SetCommandEnabled(f.entry.ID, "checkping", true)
			initial := 1
			if recover || disabled {
				initial = 0
			}
			if len(f.a.Launcher.Commands) != initial {
				t.Fatalf("initial command count=%d want=%d", len(f.a.Launcher.Commands), initial)
			}
			f.request(t, 0, "ok", nil, false)
			done := f.start(t)
			t.Cleanup(func() { f.release(t, 0); diagnosisWait(t, done) })
			f.waitStarted(t, 0)
			want, scenario := 0, "lifecycle-failure"
			if recover {
				want, scenario = 1, "lifecycle-recovery"
			}
			if disabled {
				want, scenario = 0, scenario+"-disabled"
			}
			diagnosisCommandFrame(t, f, scenario, dark, want)
		})
	}
}

func TestDiagnosisExecutionWithdrawsFailedCommands(t *testing.T) {
	checkDiagnosisLifecycleCommands(t, false, false)
}
func TestDiagnosisExecutionRestoresRecoveredCommands(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled", true: "disabled"}[disabled], func(t *testing.T) { checkDiagnosisLifecycleCommands(t, true, disabled) })
	}
}
