package shell

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

// The test executable doubles as a portable tool. It only enters this branch
// for the fixture's explicit environment marker and its help arguments.
func init() {
	if os.Getenv("FLOTER_COMMAND_REPROBE_FIXTURE") != "1" || len(os.Args) < 2 {
		return
	}
	if os.Args[1] == "fresh" {
		fmt.Print("Options:\n  -o, --output <FILE>    Write output\n")
		os.Exit(0)
	}
	if os.Args[1] != "--help" {
		return
	}
	for n := 0; n < 8; n++ {
		base := fmt.Sprintf("help-%d", n)
		if os.Mkdir(base+".claimed", 0o700) != nil {
			continue
		}
		_ = os.WriteFile(base+".started", nil, 0o600)
		deadline := time.Now().Add(2500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(base + ".release"); err == nil {
				fmt.Print("Probe Tool - Gadgets\nAvailable Plugins\n📦 fresh 1.0\n")
				os.Exit(0)
			}
			time.Sleep(time.Millisecond)
		}
		os.Exit(4)
	}
	os.Exit(5)
}

func commandReprobeFixture(t *testing.T, dark bool) *diagnosisFixture {
	t.Helper()
	t.Setenv("FLOTER_COMMAND_REPROBE_FIXTURE", "1")
	f := newDiagnosisFixture(t, dark)
	f.entry.ManifestPath = filepath.Join(filepath.Dir(f.entry.ManifestPath), "integration", "floter.extension.json")
	f.entry.PublisherID, f.entry.ProviderKind = "local-user", "static-descriptor"
	f.manifest.Provider.Type, f.manifest.Provider.Descriptor = "static-descriptor", "provider-description.json"
	f.manifest.Provider.ArgsPrefix = nil
	f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection"}}
	f.write(t)
	commandReprobeDescriptor(t, f, "Original command")
	f.a.Integrations.Refresh(context.Background())
	f.a.Launcher.SetCommands(f.a.launcherCommands())
	// This test owns explicit probes; the unrelated version-drift background
	// loop must not launch another tool while it observes the completion.
	f.a.startDriftReprobe()
	f.a.onScreen = func() bool { return true }
	return f
}

func commandReprobeDescriptor(t *testing.T, f *diagnosisFixture, name string) {
	t.Helper()
	diagnosisWrite(t, filepath.Join(filepath.Dir(f.entry.ManifestPath), "provider-description.json"), map[string]any{
		"protocolVersion": "1.0", "provider": map[string]any{"id": f.entry.ID, "name": f.manifest.Name, "version": "1.0"},
		"commands": []any{map[string]any{"id": "root", "name": name, "description": "root", "execution": map[string]any{"program": "self", "mode": "pty", "argsPrefix": []string{}}}},
	})
}

func commandReprobeRelease(t *testing.T, f *diagnosisFixture) {
	t.Helper()
	for n := 0; n < 8; n++ {
		if err := os.WriteFile(filepath.Join(filepath.Dir(f.entry.ManifestPath), fmt.Sprintf("help-%d.release", n)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func commandReprobeStarted(t *testing.T, f *diagnosisFixture) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(filepath.Dir(f.entry.ManifestPath), "help-0.started")); err == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("command probe never reached its help gate")
}

func commandReprobeClick(t *testing.T, f *diagnosisFixture, tt *ui.Tester) {
	t.Helper()
	run, target := f.a.Settings.Actions.ReprobeCommands, ""
	f.a.Settings.Actions.ReprobeCommands = func(id string) { target = id }
	if err := tt.Click(i18n.For("zh").Settings.IntegrationsReprobe); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	f.a.Settings.Actions.ReprobeCommands = run
	if target != f.entry.ID {
		t.Fatalf("reprobe dispatched to %q, want %q", target, f.entry.ID)
	}
	run(target)
}

func dumpCommandReprobe(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JT_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-jt-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkCommandReprobeBusy(t *testing.T, competing bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := commandReprobeFixture(t, dark)
			tt := healthRefreshFrame(t, f, "unused", dark)
			settled := make(chan struct{}, 16)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			commandReprobeClick(t, f, tt)
			commandReprobeStarted(t, f)
			copy := i18n.For("zh").Settings
			scenario, calls := "repeat", 0
			tt.Frame()
			if competing {
				scenario = "competing"
				f.a.Settings.Actions.DiagnoseIntegration = func(string) { calls++ }
				f.a.Settings.Actions.ReprobeHealth = func(string) { calls++ }
				f.a.Settings.Actions.ReconnectIntegration = func(string, func(error)) { calls++ }
				for _, label := range []string{copy.IntegrationsCheck, copy.IntegrationsHealthReprobe} {
					if err := tt.Click(label); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
				}
				if calls != 0 {
					t.Errorf("pending command reprobe permits competing actions: %d", calls)
				}
			} else {
				if err := tt.Click(copy.IntegrationsReprobe); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				f.a.Settings.Actions.ReprobeCommands(f.entry.ID)
				f.a.diagnosisMu.Lock()
				epoch := f.a.diagnosisEpochs[f.entry.ID]
				f.a.diagnosisMu.Unlock()
				if epoch != 1 {
					t.Errorf("pending command reprobe starts repeatedly: epoch=%d", epoch)
				}
			}
			tt.Frame()
			dumpCommandReprobe(t, tt, scenario, dark)
			commandReprobeRelease(t, f)
			for i := 0; i < 2; i++ {
				select {
				case <-settled:
				case <-time.After(4 * time.Second):
					t.Error("command reprobe never entered and settled its row busy state")
					return
				}
			}
			tt.Frame()
			tt.Frame()
			f.a.Settings.Actions.ReprobeCommands = func(string) { calls++ }
			if err := tt.Click(copy.IntegrationsReprobe); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 {
				t.Errorf("completed command reprobe cannot be retried: calls=%d", calls)
			}
		})
	}
}

func TestCommandReprobeRejectsRepeatedClick(t *testing.T)  { checkCommandReprobeBusy(t, false) }
func TestCommandReprobeLocksCompetingActions(t *testing.T) { checkCommandReprobeBusy(t, true) }
