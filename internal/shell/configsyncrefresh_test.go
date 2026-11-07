package shell

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func syncRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JL_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jl-" + name + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

// The read has already obtained its old answer when another action commits.
// Run that ordering through the existing compute seam, then the real settings
// list and editor; checking the return as well also catches a stale first frame
// even if an implementation protects the memo but returns the rejected map.
func checkConfigSyncRefresh(t *testing.T, failed bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
			const id = "dev.floter.configured"
			desc, _ := a.Integrations.Description(id)
			values := transferFeedbackValues()
			values["endpoint"] = "https://before.example"
			if err := extensions.SaveConfiguration(paths, id, desc.Configuration.Schema, values); err != nil {
				t.Fatal(err)
			}
			invalidateStoredConfigs()
			a.Settings = settingsui.New(a.Store, settingsui.Actions{ImportIntegrations: a.importIntegrations, SaveConfiguration: a.saveIntegrationConfiguration})
			a.Settings.Integrations = a.integrationList
			transferCauseFrame(t, a, dark)
			tt := ui.NewTester(a.View, 960, 1800)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			if err := tt.Click(a.SettingsCopy().ConfigOpen); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			items := a.Integrations.Inventory().Integrations
			invalidateStoredConfigs()
			name, want := "cold-import", "https://imported.example"
			if failed {
				name, want = "cold-save-read-error", "https://saved.example"
			}
			reads := 0
			got := refreshStoredConfigs(&storedConfigMemoC, items, func(given []extensions.Integration) map[string]storedConfig {
				old := computeStoredConfigs(given)
				reads++
				if reads != 1 {
					return old
				}
				if failed {
					old[id] = storedConfig{key: storedConfigKey(paths, id)}
					values["endpoint"] = want
					done := make(chan error, 1)
					a.Settings.Actions.SaveConfiguration(id, values, func(err error) { done <- err })
					if err := transferFeedbackAwait(t, done); err != nil {
						t.Fatal(err)
					}
				} else {
					doc := extensions.SyncDocument{Version: 2, Extensions: []extensions.SyncEntry{{ID: id, Enabled: true, Config: map[string]any{"endpoint": want}}}}
					data, err := doc.Marshal()
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(t.TempDir(), "workspace.json")
					recoveryWrite(t, path, data)
					a.openFileDialog = func() (string, error) { return path, nil }
					a.Settings.Actions.ImportIntegrations()
					waitForTransfer(t, a)
				}
				waitForGoroutines()
				return old
			}, nil, time.Now())
			tt.Frame()
			tt.Frame()
			syncRefreshDump(t, tt, name, dark)
			if got[id].values["endpoint"] != want {
				t.Errorf("%s: synchronous read returned retired value %v, want %s", name, got[id].values["endpoint"], want)
			}
			r, ok := tt.Find("endpoint")
			if !ok {
				t.Fatalf("endpoint missing: %v", tt.Texts())
			}
			tt.ClickAt(r.X+240, r.Y+r.H/2)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(ui.Cmd, ui.KeyC)
			if value := tt.Clipboard(); value != want {
				t.Errorf("%s: visible editor reverted to %q, want %q", name, value, want)
			}
			stored, err := extensions.LoadStoredConfiguration(paths.Data, id)
			if err != nil || stored.Values["endpoint"] != want {
				t.Fatal("durable configuration changed", err)
			}
		})
	}
}

func TestConfigSyncFirstReadCannotUndoImport(t *testing.T)        { checkConfigSyncRefresh(t, false) }
func TestConfigSyncFirstFailureCannotBlankSavedForm(t *testing.T) { checkConfigSyncRefresh(t, true) }
