package shell

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func repairSearchFixture(t *testing.T, dark bool) (*diagnosisFixture, string) {
	t.Helper()
	f := newDiagnosisFixture(t, dark)
	program := f.entry.ExecutablePath
	f.entry.RuntimeOwnership = extensions.RuntimeOwnershipSystem
	f.manifest.Provider.Type, f.manifest.Provider.Descriptor = "static-descriptor", "provider-description.json"
	f.manifest.Provider.ArgsPrefix = nil
	f.write(t)
	commandReprobeDescriptor(t, f, "Recovered command")
	f.a.startDriftReprobe()
	f.a.RefreshIntegrations(context.Background())
	f.a.Settings.Actions.SetCommandEnabled(f.entry.ID, "root", true)
	f.a.onScreen = func() bool { return true }
	return f, program
}

func repairSearchDump(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JU_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-ju-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func repairSearchClick(t *testing.T, f *diagnosisFixture, tt *ui.Tester, repair bool) <-chan error {
	t.Helper()
	finished := make(chan error, 1)
	label := i18n.For("zh").Settings.IntegrationsReconnect
	if repair {
		label = i18n.For("zh").Settings.IntegrationsRecheck
		run := f.a.Settings.Actions.RepairIntegration
		f.a.Settings.Actions.RepairIntegration = func(id string, done func(error)) {
			run(id, func(err error) { done(err); finished <- err })
		}
	} else {
		run := f.a.Settings.Actions.ReconnectIntegration
		f.a.Settings.Actions.ReconnectIntegration = func(id string, done func(error)) {
			run(id, func(err error) { done(err); finished <- err })
		}
	}
	// Dispatch after Click has finished drawing, as a real window's event
	// queue does. Headless onMain otherwise runs the callback on the worker.
	var dispatch func()
	if repair {
		run := f.a.Settings.Actions.RepairIntegration
		f.a.Settings.Actions.RepairIntegration = func(id string, done func(error)) { dispatch = func() { run(id, done) } }
	} else {
		run := f.a.Settings.Actions.ReconnectIntegration
		f.a.Settings.Actions.ReconnectIntegration = func(id string, done func(error)) { dispatch = func() { run(id, done) } }
	}
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if dispatch == nil {
		t.Fatal("repair action did not dispatch")
	}
	dispatch()
	return finished
}

func repairSearchWait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(6 * time.Second):
		t.Fatal("repair action did not settle")
		return nil
	}
}

func checkRepairSearch(t *testing.T, repair bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f, program := repairSearchFixture(t, dark)
			scenario := "reconnect-search"
			if repair {
				scenario = "repair-search"
				f.entry.State = "broken"
				code := extensions.BindingCodeMissing
				f.entry.LastErrorCode = &code
			} else {
				f.entry.ExecutablePath = filepath.Join(f.dir, "gone")
				t.Cleanup(extensions.StubToolCandidates(func(...string) []extensions.ExecutableCandidate {
					return []extensions.ExecutableCandidate{{Name: "recovered", Path: program}}
				}))
			}
			f.write(t)
			f.a.RefreshIntegrations(context.Background())
			if len(f.a.Launcher.Commands) != 0 {
				t.Fatal("fixture still offers broken command")
			}
			tt := healthRefreshFrame(t, f, "unused", dark)
			if err := repairSearchWait(t, repairSearchClick(t, f, tt, repair)); err != nil {
				t.Fatal(err)
			}
			f.a.Open(SurfaceLauncher)
			f.a.Launcher.SetQuery("root")
			tt.Frame()
			tt.Frame()
			repairSearchDump(t, tt, scenario, dark)
			if len(f.a.Launcher.Commands) != 1 || !tt.HasText("Recovered command") {
				t.Errorf("%s completed but search stayed unavailable: commands=%d", scenario, len(f.a.Launcher.Commands))
			}
			if row := f.a.integrationList()[0]; !row.RuntimeAvailable || row.Broken {
				t.Error("repair did not restore current row")
			}
		})
	}
}

func TestRepairSearchReconnect(t *testing.T) { checkRepairSearch(t, false) }
func TestRepairSearchRecheck(t *testing.T)   { checkRepairSearch(t, true) }
