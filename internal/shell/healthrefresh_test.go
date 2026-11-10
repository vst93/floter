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
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func healthRefreshFrame(t *testing.T, f *diagnosisFixture, scenario string, dark bool) *ui.Tester {
	t.Helper()
	f.a.Open(SurfaceSettings)
	f.a.Settings.Page = settingsui.PageIntegrations
	f.a.Settings.Actions.Detected = nil
	tt := ui.NewTester(f.a.View, 960, 1600)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	openSettingsToolForTest(t, f.a, tt)
	dumpHealthRefresh(t, tt, scenario, dark)
	return tt
}

func dumpHealthRefresh(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JR_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-jr-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkHealthRefreshDeclaration(t *testing.T, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true}}
			f.write(t)
			f.a.Integrations.Refresh(context.Background())
			calls := 0
			f.a.Settings.Actions.ReprobeHealth = func(string) { calls++ }
			tt := healthRefreshFrame(t, f, "valid-"+change, dark)
			label := i18n.For("zh").Settings.IntegrationsHealthReprobe
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 {
				t.Fatal("valid health refresh cannot be reached")
			}
			switch change {
			case "removed":
				f.manifest.Lifecycle.Probes = nil
				f.write(t)
			case "identity":
				f.manifest.ID = "dev.floter.replacement"
				f.write(t)
			case "invalid":
				if err := os.WriteFile(f.entry.ManifestPath, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			f.a.Integrations.Refresh(context.Background())
			tt.Frame()
			tt.Frame()
			dumpHealthRefresh(t, tt, change, dark)
			if tt.HasText(label) {
				t.Error("current declaration cannot run probes but still offers health refresh")
			}
			if err := tt.Click(label); err == nil {
				tt.Frame()
			}
			if calls != 1 {
				t.Error("stale health refresh still dispatches a request")
			}
			if f.a.integrationList()[0].HealthReport == nil {
				t.Error("historical health report was discarded")
			}
			f.manifest.ID = f.entry.ID
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true}}
			f.write(t)
			f.a.Integrations.Refresh(context.Background())
			tt.Frame()
			tt.Frame()
			if err := tt.Click(label); err != nil {
				t.Fatal("restored declaration cannot refresh:", err)
			}
			tt.Frame()
			if calls != 2 {
				t.Error("restored declaration does not dispatch exactly once")
			}
		})
	}
}

func TestHealthRefreshRemovedDeclaration(t *testing.T) { checkHealthRefreshDeclaration(t, "removed") }
func TestHealthRefreshInvalidDeclaration(t *testing.T) {
	for _, change := range []string{"invalid", "identity"} {
		t.Run(change, func(t *testing.T) { checkHealthRefreshDeclaration(t, change) })
	}
}

func checkHealthRefreshFailure(t *testing.T, failure string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			program := f.entry.ExecutablePath
			base := filepath.Join(f.dir, "retry")
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true, Args: []string{"-test.run=^TestHealthCompletionProvider$", "--", "--floter-health", base}}}
			if failure == "binding" {
				f.entry.ExecutablePath = filepath.Join(f.dir, "missing-program")
			}
			f.write(t)
			f.a.Integrations.Refresh(context.Background())
			owner, _ := f.a.Integrations.Inventory().WithID(f.entry.ID)
			if failure == "repository" {
				if err := os.WriteFile(f.paths.RepositoryFile, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			f.a.onScreen = func() bool { return true }
			f.a.Open(SurfaceSettings)
			clearNotices(f.a)
			// Exercise the actual asynchronous action; the existing invalidation
			// hook acknowledges both entry and settlement without timing sleeps.
			settled := make(chan struct{}, 2)
			f.a.Settings.Actions.Invalidate = func() { settled <- struct{}{} }
			f.a.Settings.Actions.ReprobeHealth(f.entry.ID)
			for n := 0; n < 2; n++ {
				select {
				case <-settled:
				case <-time.After(6 * time.Second):
					t.Fatal("health refresh action never entered and settled its busy state")
				}
			}
			f.a.Settings.Actions.Invalidate = nil
			toast := queuedToast(f.a, "floter-notice")
			want := i18n.NotificationsFor("zh").IntegrationCheckFailed(f.manifest.Name)
			healthRefreshFrame(t, f, failure, dark)
			if toast.Type != "error" || toast.Title != want {
				t.Errorf("health refresh %s failure is silent: toast=%+v", failure, toast)
			}
			if !f.a.Settings.BeginHealthReprobe(f.entry.ID) {
				t.Fatal("failed health refresh leaves the row locked")
			}
			f.a.Settings.FinishHealthReprobe(f.entry.ID)
			if failure == "binding" {
				current, _ := f.a.Integrations.Inventory().WithID(f.entry.ID)
				if current.Entry.ProbeReport == nil || current.Entry.ProbeReport.CheckedAt != f.entry.ProbeReport.CheckedAt || current.Entry.LastErrorCode == nil {
					t.Error("binding failure lost history or its recorded reason")
				}
			}
			f.entry.ExecutablePath = program
			f.write(t)
			if err := os.WriteFile(base+".release", []byte("0"), 0o600); err != nil {
				t.Fatal(err)
			}
			f.a.Integrations.Refresh(context.Background())
			owner, _ = f.a.Integrations.Inventory().WithID(f.entry.ID)
			clearNotices(f.a)
			f.a.Settings.BeginHealthReprobe(f.entry.ID)
			f.a.runHealthReprobe(owner)
			if f.a.integrationList()[0].HealthStatus != extensions.HealthHealthy || len(noticeTitles(f.a)) != 0 {
				t.Error("repaired health refresh does not recover cleanly")
			}
			if !f.a.Settings.BeginHealthReprobe(f.entry.ID) {
				t.Error("successful health refresh leaves the row locked")
			}
			f.a.Settings.FinishHealthReprobe(f.entry.ID)
		})
	}
}

func TestHealthRefreshBindingFailure(t *testing.T)    { checkHealthRefreshFailure(t, "binding") }
func TestHealthRefreshRepositoryFailure(t *testing.T) { checkHealthRefreshFailure(t, "repository") }

func checkHealthRefreshCommands(t *testing.T, recover, disabled bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			base := filepath.Join(f.dir, "command-health")
			f.manifest.Lifecycle.Probes = []extensions.Probe{{ID: "connection", Required: true, Args: []string{"-test.run=^TestHealthCompletionProvider$", "--", "--floter-health", base}}}
			f.manifest.Provider = extensions.Provider{Type: "static-descriptor", Descriptor: "description.json"}
			if disabled {
				f.entry.Enabled = false
				f.entry.State = "disabled"
			}
			f.write(t)
			description := `{"protocolVersion":"1.0","provider":{"id":"dev.floter.check","name":"检查工具","version":"1.0.0"},"commands":[{"id":"healthping","name":"Health Ping","description":"检查连接","execution":{"program":"self","argsPrefix":["ping"],"mode":"external"}}]}`
			if err := os.WriteFile(filepath.Join(filepath.Dir(f.entry.ManifestPath), "description.json"), []byte(description), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(base+".release", []byte("7"), 0o600); err != nil {
				t.Fatal(err)
			}
			if recover {
				report, err := extensions.Reprobe(context.Background(), f.paths, f.entry.ID)
				if err != nil || report.Status != extensions.HealthUnhealthy {
					t.Fatalf("required failure fixture: report=%+v err=%v", report, err)
				}
				if err := os.WriteFile(base+".release", []byte("0"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			f.a.RefreshIntegrations(context.Background())
			f.a.Settings.Actions.SetCommandEnabled(f.entry.ID, "healthping", true)
			if err := f.a.Integrations.DescribeError(f.entry.ID); err != nil {
				t.Fatal("command descriptor fixture:", err)
			}
			initial := 1
			if recover || disabled {
				initial = 0
			}
			if len(f.a.Launcher.Commands) != initial {
				desc, _ := f.a.Integrations.Description(f.entry.ID)
				item, _ := f.a.Integrations.Inventory().WithID(f.entry.ID)
				_, bindingErr := extensions.ResolveRuntime(item)
				t.Fatalf("command fixture starts with %d commands, want %d; description=%+v state=%q binding=%v entries=%+v", len(f.a.Launcher.Commands), initial, desc, item.Entry.State, bindingErr, f.a.Integrations.CommandEntries())
			}
			owner, _ := f.a.Integrations.Inventory().WithID(f.entry.ID)
			f.a.Settings.BeginHealthReprobe(f.entry.ID)
			f.a.runHealthReprobe(owner)
			want := 0
			if recover && !disabled {
				want = 1
			}
			f.a.Open(SurfaceLauncher)
			f.a.Launcher.SetQuery("healthping")
			tt := ui.NewTester(f.a.View, 960, 600)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			scenario := "command-failure"
			if recover {
				scenario = "command-recovery"
			}
			if disabled {
				scenario += "-disabled"
			}
			dumpHealthRefresh(t, tt, scenario, dark)
			if len(f.a.Launcher.Commands) != want {
				t.Errorf("health refresh leaves launcher commands stale: got=%d want=%d", len(f.a.Launcher.Commands), want)
			}
			if tt.HasText("Health Ping") != (want == 1) {
				t.Errorf("health result and visible launcher command disagree: want command=%v texts=%q", want == 1, tt.Texts())
			}
		})
	}
}

func TestHealthRefreshWithdrawsFailedCommands(t *testing.T) {
	checkHealthRefreshCommands(t, false, false)
}
func TestHealthRefreshRestoresRecoveredCommands(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled", true: "disabled"}[disabled], func(t *testing.T) { checkHealthRefreshCommands(t, true, disabled) })
	}
}
