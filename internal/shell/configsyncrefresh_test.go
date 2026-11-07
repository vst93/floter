package shell

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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
func syncRefreshAddNeighbour(t *testing.T, a *App, paths extensions.Paths) {
	t.Helper()
	const first, second = "dev.floter.configured", "dev.floter.second"
	pkg := filepath.Join(paths.Extensions, second)
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"floter.extension.json", "tool.sh"} {
		data, err := os.ReadFile(filepath.Join(paths.Extensions, first, name))
		if err != nil {
			t.Fatal(err)
		}
		body := strings.ReplaceAll(strings.ReplaceAll(string(data), first, second), "Configured", "Second Tool")
		if err := os.WriteFile(filepath.Join(pkg, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := extensions.LoadRepository(paths.RepositoryFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := repo.Extensions[first]
	entry.ID, entry.Name, entry.ManifestPath = second, "Second Tool", filepath.Join(pkg, "floter.extension.json")
	repo.Extensions[second] = entry
	if err := extensions.SaveRepository(paths.RepositoryFile, repo); err != nil {
		t.Fatal(err)
	}
	a.RefreshIntegrations(context.Background())
	if len(a.Integrations.Inventory().Integrations) != 2 {
		t.Fatal("second installed tool missing")
	}
}

func checkConfigSyncRefresh(t *testing.T, scenario string) {
	t.Helper()
	failed := scenario == "cold-save-read-error"
	missing := strings.HasPrefix(scenario, "added-")
	neighbour := scenario == "added-neighbour-save"
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
			if missing {
				syncRefreshAddNeighbour(t, a, paths)
			}
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
			if missing {
				seed := computeStoredConfigs(items)
				if neighbour {
					delete(seed, "dev.floter.second")
				} else {
					delete(seed, id)
				}
				storedConfigMemoC.publish(seed, time.Now())
			}
			name, want := scenario, "https://imported.example"
			if failed || neighbour {
				want = "https://saved.example"
			}
			reads := 0
			got := refreshStoredConfigs(&storedConfigMemoC, items, func(given []extensions.Integration) map[string]storedConfig {
				old := computeStoredConfigs(given)
				reads++
				if reads != 1 {
					return old
				}
				if failed || neighbour {
					if failed {
						old[id] = storedConfig{key: storedConfigKey(paths, id)}
					}
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
			// Project this exact returned snapshot with the production mapper.
			// A later frame can repair the cache and hide a wrong first frame.
			projected := a.integrationList()
			for i := range projected {
				for _, integration := range items {
					if projected[i].ID == integration.Entry.ID {
						projected[i].Config = nil
						applyIntegrationConfig(a.Integrations, paths, &projected[i], integration, got[integration.Entry.ID])
					}
				}
			}
			a.Settings.Integrations = func() []settingsui.Integration { return projected }
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

func TestConfigSyncFirstReadCannotUndoImport(t *testing.T) { checkConfigSyncRefresh(t, "cold-import") }
func TestConfigSyncFirstFailureCannotBlankSavedForm(t *testing.T) {
	checkConfigSyncRefresh(t, "cold-save-read-error")
}
func TestConfigSyncAddedRowCannotUndoNeighbourSave(t *testing.T) {
	checkConfigSyncRefresh(t, "added-neighbour-save")
}
func TestConfigSyncAddedRowCannotUndoOwnImport(t *testing.T) {
	checkConfigSyncRefresh(t, "added-own-import")
}
