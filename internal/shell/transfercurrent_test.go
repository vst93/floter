package shell

import (
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func currentDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JJ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jj-" + name + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func currentTransfer(t *testing.T, a *App, tt *ui.Tester, export bool) {
	t.Helper()
	label := a.SettingsCopy().IntegrationsImport
	if export {
		label = a.SettingsCopy().IntegrationsExport
	}
	// The page may reflow after discovery. Re-read the button after scrolling
	// instead of clicking the previous frame's offscreen geometry.
	for n := 0; n < 4; n++ {
		tt.Frame()
		tt.Frame()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal("transfer button missing")
		}
		if r.Y >= 80 && r.Y+r.H < 900 {
			break
		}
		tt.Scroll(500, 500, 0, r.Y-500)
	}
	recoveryTransfer(t, a, tt, export)
}

func TestRepeatedImportUsesLatestEnabledState(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, initial := range []bool{false, true} {
			t.Run(map[bool]string{false: "light", true: "dark"}[dark]+"/"+map[bool]string{false: "disabled", true: "enabled"}[initial], func(t *testing.T) {
				a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
				const id = "dev.floter.configured"
				if err := a.Integrations.SetEnabled(id, initial); err != nil {
					t.Fatal(err)
				}
				doc := extensions.SyncDocument{Version: 2, Extensions: []extensions.SyncEntry{
					{ID: id, Enabled: !initial, Config: map[string]any{"endpoint": "https://first.example", "region": "eu"}},
					{ID: id, Enabled: initial, Config: map[string]any{"endpoint": "https://last.example"}},
				}}
				data, err := doc.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "workspace.json")
				recoveryWrite(t, path, data)
				a.openFileDialog = func() (string, error) { return path, nil }
				tt := transferCauseFrame(t, a, dark)
				currentTransfer(t, a, tt, false)
				currentDump(t, tt, "repeat-"+map[bool]string{false: "disabled", true: "enabled"}[initial], dark)
				repo, err := extensions.LoadRepository(paths.RepositoryFile)
				if err != nil {
					t.Fatal(err)
				}
				if repo.Extensions[id].Enabled != initial {
					t.Errorf("repeated import reports success but enabled=%v, want %v", repo.Extensions[id].Enabled, initial)
				}
				stored, err := extensions.LoadStoredConfiguration(paths.Data, id)
				if err != nil || stored.Values["endpoint"] != "https://last.example" || stored.Values["region"] != "eu" {
					t.Fatalf("ordered configuration merge changed: %+v %v", stored, err)
				}
				if r := a.Settings.TransferReport; r == nil || r.Succeeded != 2 || len(r.Failed) != 0 {
					t.Fatalf("entry report changed: %+v", r)
				}
			})
		}
	}
}

// Real picker callbacks finish out of order. Wait for the notification mutex
// after observing the picker return, then allow the worker to finish queuing.
// File exports have no completion callback, so the existing bounded waiter is
// used before assertions, never a fabricated toast in place of the action.
func checkLateTransferFailure(t *testing.T, config, recovered bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			type answer struct {
				path string
				err  error
			}
			requests := make(chan chan answer, 2)
			returned := make(chan struct{}, 2)
			a, _ := configuredIntegrationFixture(t, func(string) (string, error) {
				reply := make(chan answer, 1)
				requests <- reply
				v := <-reply
				returned <- struct{}{}
				return v.path, v.err
			})
			tt := transferCauseFrame(t, a, dark)
			done := make(chan error, 2)
			run := func() {
				if config {
					a.Settings.Actions.ExportConfiguration("dev.floter.configured", transferFeedbackValues(), func(err error) { done <- err })
				} else {
					a.Settings.Actions.ExportScript("diagnose.py", "print('ready')\n")
				}
			}
			run()
			older := <-requests
			defer close(older)
			run()
			newer := <-requests
			defer close(newer)
			path := filepath.Join(t.TempDir(), "backup.json")
			if recovered {
				newer <- answer{path: path}
			} else {
				newer <- answer{err: errors.New("current picker failure")}
			}
			<-returned
			if config {
				transferFeedbackAwait(t, done)
			} else {
				waitForToast(t, a, map[bool]string{false: "floter-notice", true: scriptExportNoticeID}[recovered])
			}
			tt.Frame()
			tt.Frame()
			older <- answer{err: errors.New("obsolete picker failure")}
			<-returned
			if config {
				transferFeedbackAwait(t, done)
			} else {
				waitForGoroutines()
			}
			tt.Frame()
			tt.Frame()
			name := map[bool]string{false: "script", true: "config"}[config] + "-late-" + map[bool]string{false: "failure", true: "success"}[recovered]
			currentDump(t, tt, name, dark)
			if tt.HasText("obsolete picker failure") {
				t.Errorf("%s: obsolete failure replaced current result", name)
			}
			if !recovered && !tt.HasText("current picker failure") {
				t.Error("latest failure disappeared")
			}
			if recovered {
				data, err := os.ReadFile(path)
				if err != nil || len(data) == 0 {
					t.Fatal("new export did not finish")
				}
			}
		})
	}
}

func TestScriptLateFailureKeepsCurrentFailure(t *testing.T) {
	checkLateTransferFailure(t, false, false)
}

func TestIntegrationExportFailureRetiresSuccess(t *testing.T) {
	checkTransferFailureRetiresSuccess(t, "export")
}

func TestConfigurationCopyFailureRetiresSuccess(t *testing.T) {
	checkTransferFailureRetiresSuccess(t, "copy")
}

func TestConfigurationExportFailureRetiresSuccess(t *testing.T) {
	checkTransferFailureRetiresSuccess(t, "config-export")
}

func TestScriptExportFailureRetiresSuccess(t *testing.T) {
	checkTransferFailureRetiresSuccess(t, "script")
}

func checkTransferFailureRetiresSuccess(t *testing.T, operation string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "backup.json")
			pickerError := error(nil)
			a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return path, pickerError })
			tt := transferCauseFrame(t, a, dark)
			values := transferFeedbackValues()
			done := make(chan error, 1)
			run := func() {
				switch operation {
				case "export":
					currentTransfer(t, a, tt, true)
				case "copy", "config-export":
					action := a.Settings.Actions.CopyConfiguration
					if operation == "config-export" {
						action = a.Settings.Actions.ExportConfiguration
					}
					action("dev.floter.configured", values, func(err error) { done <- err })
					transferFeedbackAwait(t, done)
				case "script":
					a.Settings.Actions.ExportScript("diagnose.py", "print('ready')\n")
					if pickerError == nil {
						waitForToast(t, a, scriptExportNoticeID)
					} else {
						waitForToast(t, a, "floter-notice")
					}
				}
				tt.Frame()
				tt.Frame()
			}
			run()
			success := a.SettingsCopy().IntegrationsExported(1)
			switch operation {
			case "copy":
				success = a.SettingsCopy().ConfigCopied
			case "config-export":
				success = a.SettingsCopy().ConfigExported
			case "script":
				success = a.SettingsCopy().LocalToolScriptExported
			}
			if !tt.HasText(success) {
				t.Fatalf("initial success missing: %q", success)
			}
			pickerError = errors.New("current export failed: picker unavailable")
			cause := "current export failed"
			if operation == "copy" {
				values["endpoint"] = "https://bad\xff.example"
				cause = "invalid UTF-8"
			}
			run()
			currentDump(t, tt, "failure-"+operation, dark)
			if tt.HasText(success) {
				t.Errorf("failed %s retains obsolete success", operation)
			}
			if !strings.Contains(strings.Join(tt.Texts(), "\n"), cause) {
				t.Fatalf("current failure missing: %q", tt.Texts())
			}
			if a.Settings.TransferBusy != "" {
				t.Fatal("failure retained busy")
			}
			if operation == "copy" {
				return
			}
			data, err := os.ReadFile(path)
			if err != nil || len(data) == 0 {
				t.Fatal("failed retry changed the completed backup")
			}
			if operation == "config-export" {
				var doc extensions.ConfigurationExportDocument
				if json.Unmarshal(data, &doc) != nil || doc.Values["token"] != extensions.PasswordPlaceholder {
					t.Error("backup redaction changed")
				}
			}
		})
	}
}
