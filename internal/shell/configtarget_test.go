package shell

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func configTargetDump(t *testing.T, tt *ui.Tester, name, appearance string) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JE_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-je-" + name + "-" + appearance + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

// Refresh the real executable provider between the last rendered form and the
// dispatched host action. The form intentionally retains that committed frame
// until its result has been observed: discovery and input need not arrive in
// the same order on a desktop.
func checkConfigTarget(t *testing.T, action, change string) {
	t.Helper()
	for _, appearance := range []string{"light", "dark"} {
		t.Run(appearance, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "export.json")
			picked, copied := 0, 0
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { picked++; return destination, nil })
			a.writeClipboard = func(ClipboardPayload) { copied++ }
			const id = "dev.floter.configured"
			desc, _ := a.Integrations.Description(id)
			if err := extensions.SaveConfiguration(paths, id, desc.Configuration.Schema, transferFeedbackValues()); err != nil {
				t.Fatal(err)
			}
			invalidateStoredConfigs()
			configPath := filepath.Join(paths.Data, id, "config.json")
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			visible := a.integrationList()
			a.Settings = settingsui.New(a.Store, settingsui.Actions{
				CopyConfiguration: a.copyConfiguration, ExportConfiguration: a.exportConfiguration,
				SaveConfiguration: a.saveIntegrationConfiguration,
			})
			a.Settings.Integrations = func() []settingsui.Integration { return visible }
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme, s.Language = appearance, "en" }); err != nil {
				t.Fatal(err)
			}
			a.Open(SurfaceSettings)
			a.Settings.Page = settingsui.PageIntegrations
			tt := ui.NewTester(a.View, 960, 1400)
			tt.SetDark(appearance == "dark")
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			openSettingsToolForTest(t, a, tt)
			if err := tt.Click(a.SettingsCopy().ConfigOpen); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			// Use a real editor change for Save so its ordinary dirty gate opens.
			if action == "save" {
				r, ok := tt.Find("endpoint")
				if !ok {
					t.Fatal("endpoint missing")
				}
				tt.ClickAt(r.X+240, r.Y+r.H/2)
				tt.Frame()
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("https://changed.example")
				tt.Frame()
			}
			script := filepath.Join(paths.Extensions, id, "tool.sh")
			original, err := os.ReadFile(script)
			if err != nil {
				t.Fatal(err)
			}
			refresh := func() {
				switch change {
				case "missing":
					desc.Configuration = nil
				case "tool":
					desc.Configuration.Owner = "tool"
				case "empty":
					desc.Configuration.Schema = nil
				case "disabled":
					if err := a.Integrations.SetEnabled(id, false); err != nil {
						t.Fatal(err)
					}
				}
				data, err := json.Marshal(desc)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(script, append(append([]byte("#!/bin/sh\ncat <<'JSONEOF'\n"), data...), []byte("\nJSONEOF\n")...), 0o755); err != nil {
					t.Fatal(err)
				}
				a.Integrations.Refresh(context.Background())
			}
			base, label := a.Settings.Actions.CopyConfiguration, a.SettingsCopy().ConfigCopy
			if action == "export" {
				base, label = a.Settings.Actions.ExportConfiguration, a.SettingsCopy().ConfigExport
			}
			if action == "save" {
				base, label = a.Settings.Actions.SaveConfiguration, a.SettingsCopy().ConfigSave
			}
			done := make(chan error, 1)
			run := func(id string, values map[string]any, finish func(error)) {
				refresh()
				defer func() {
					if p := recover(); p != nil {
						t.Errorf("stale %s panicked after %s refresh: %v", action, change, p)
						done <- nil
					}
				}()
				base(id, values, func(err error) { finish(err); done <- err })
			}
			switch action {
			case "copy":
				a.Settings.Actions.CopyConfiguration = run
			case "export":
				a.Settings.Actions.ExportConfiguration = run
			case "save":
				a.Settings.Actions.SaveConfiguration = run
			}
			clearNotices(a)
			if err := tt.Click(label); err != nil {
				t.Fatalf("%v; texts=%q", err, tt.Texts())
			}
			tt.Frame()
			failure := transferFeedbackAwait(t, done)
			tt.Frame()
			tt.Frame()
			configTargetDump(t, tt, action+"-"+change, appearance)
			if failure == nil {
				t.Errorf("stale %s silently succeeded after %s refresh", action, change)
			} else if !tt.HasText(failure.Error()) {
				t.Errorf("stale %s refusal is invisible: %v", action, failure)
			}
			if picked != 0 || copied != 0 {
				t.Errorf("stale %s reached output: picker=%d clipboard=%d", action, picked, copied)
			}
			after, err := os.ReadFile(configPath)
			if err != nil || string(after) != string(before) {
				t.Errorf("stale %s changed stored configuration: %v", action, err)
			}
			if action == "save" {
				return
			} // success save is covered end-to-end; no new lifetime policy.
			if err := os.WriteFile(script, original, 0o755); err != nil {
				t.Fatal(err)
			}
			if change == "disabled" {
				if err := a.Integrations.SetEnabled(id, true); err != nil {
					t.Fatal(err)
				}
			}
			a.Integrations.Refresh(context.Background())
			fresh := make(chan error, 1)
			base(id, transferFeedbackValues(), func(err error) { fresh <- err })
			if err := transferFeedbackAwait(t, fresh); err != nil {
				t.Fatal(err)
			}
			if picked+copied != 1 {
				t.Errorf("restored %s did not deliver exactly once", action)
			}
		})
	}
}

func TestConfigTargetCopyMissing(t *testing.T) {
	for _, change := range []string{"missing", "disabled", "empty"} {
		t.Run(change, func(t *testing.T) { checkConfigTarget(t, "copy", change) })
	}
}
func TestConfigTargetExportMissing(t *testing.T) {
	for _, change := range []string{"missing", "disabled", "empty"} {
		t.Run(change, func(t *testing.T) { checkConfigTarget(t, "export", change) })
	}
}

func TestConfigTargetCopyTool(t *testing.T)   { checkConfigTarget(t, "copy", "tool") }
func TestConfigTargetExportTool(t *testing.T) { checkConfigTarget(t, "export", "tool") }

func TestConfigTargetSaveMissing(t *testing.T) { checkConfigTarget(t, "save", "missing") }
func TestConfigTargetSaveTool(t *testing.T)    { checkConfigTarget(t, "save", "tool") }
