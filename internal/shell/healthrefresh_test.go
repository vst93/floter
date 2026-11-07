package shell

import (
	"context"
	"image/png"
	"os"
	"testing"

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
