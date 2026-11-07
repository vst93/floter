package shell

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func refreshResultDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JK_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jk-" + name + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func refreshResultWait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("background refresh did not finish")
	}
}

func checkStoredRefreshResult(t *testing.T, failed bool) {
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
			a.Settings = settingsui.New(a.Store, settingsui.Actions{
				ImportIntegrations: a.importIntegrations,
				SaveConfiguration:  a.saveIntegrationConfiguration,
			})
			a.Settings.Integrations = a.integrationList
			tt := transferCauseFrame(t, a, dark)
			tt = ui.NewTester(a.View, 960, 1800)
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
			old := computeStoredConfigs(items)
			if failed {
				old[id] = storedConfig{key: storedConfigKey(paths, id)}
			}
			entered, release, landed := make(chan struct{}), make(chan struct{}), make(chan struct{})
			released := false
			defer func() {
				if !released {
					close(release)
					refreshResultWait(t, landed)
				}
			}()
			_, computed, _ := storedConfigMemoC.snapshot()
			refreshStoredConfigs(&storedConfigMemoC, items, func([]extensions.Integration) map[string]storedConfig {
				close(entered)
				<-release
				return old
			}, func() { close(landed) }, computed.Add(storedConfigTTL+time.Millisecond))
			refreshResultWait(t, entered)
			name := "import-late-values"
			want := "https://imported.example"
			if failed {
				name, want = "save-late-read-error", "https://saved.example"
				values["endpoint"] = want
				done := make(chan error, 1)
				a.Settings.Actions.SaveConfiguration(id, values, func(err error) { done <- err })
				if err := transferFeedbackAwait(t, done); err != nil {
					t.Fatal(err)
				}
				waitForGoroutines()
				a.RefreshIntegrations(context.Background())
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
			// First draw the confirmed new generation, then let the old I/O finish.
			tt.Frame()
			tt.Frame()
			if got := a.integrationList()[0].ConfigValues["endpoint"]; got != want {
				t.Fatalf("new result never reached page: %v", got)
			}
			close(release)
			released = true
			refreshResultWait(t, landed)
			tt.Frame()
			tt.Frame()
			refreshResultDump(t, tt, name, dark)
			if got := a.integrationList()[0].ConfigValues["endpoint"]; got != want {
				t.Errorf("%s: retired read replaced current value: %v, want %s", name, got, want)
			}
			// Copy from the actual editor, so a correct cache with stale pixels fails too.
			r, ok := tt.Find("endpoint")
			if !ok {
				t.Fatalf("endpoint editor missing: %v", tt.Texts())
			}
			if r.Y > 850 {
				tt.Scroll(500, 500, 0, r.Y-500)
				tt.Frame()
				tt.Frame()
				r, _ = tt.Find("endpoint")
			}
			tt.ClickAt(r.X+240, r.Y+r.H/2)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(ui.Cmd, ui.KeyC)
			if got := tt.Clipboard(); got != want {
				t.Errorf("%s: editor displays %q, want %q", name, got, want)
			}
			stored, err := extensions.LoadStoredConfiguration(paths.Data, id)
			if err != nil || stored.Values["endpoint"] != want {
				t.Fatal("refresh changed durable data", err)
			}
		})
	}
}

func TestConfigRefreshOldReadCannotUndoImport(t *testing.T) { checkStoredRefreshResult(t, false) }
func TestConfigRefreshOldReadFailureCannotBlankSavedForm(t *testing.T) {
	checkStoredRefreshResult(t, true)
}
