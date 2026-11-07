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

func recoveryDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JI_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ji-" + name + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func recoveryWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func recoveryTransfer(t *testing.T, a *App, tt *ui.Tester, export bool) {
	t.Helper()
	label := a.SettingsCopy().IntegrationsImport
	called := false
	original := a.Settings.Actions.ImportIntegrations
	a.Settings.Actions.ImportIntegrations = func() { called = true; original() }
	defer func() { a.Settings.Actions.ImportIntegrations = original }()
	if export {
		label = a.SettingsCopy().IntegrationsExport
		exportOriginal := a.Settings.Actions.ExportIntegrations
		a.Settings.Actions.ExportIntegrations = func() { called = true; exportOriginal() }
		defer func() { a.Settings.Actions.ExportIntegrations = exportOriginal }()
	}
	if r, ok := tt.Find(label); ok && r.Y > 850 {
		tt.Scroll(500, 500, 0, r.Y-500)
		tt.Frame()
		tt.Frame()
	}
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !called {
		t.Fatal("transfer button did not dispatch its action")
	}
	waitForTransfer(t, a)
	tt.Frame()
	tt.Frame()
}

func checkImportAppliedFailure(t *testing.T, stage string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
			doc, err := extensions.ExportSync(paths)
			if err != nil {
				t.Fatal(err)
			}
			entry := doc.Extensions[0]
			id := entry.ID
			if stage != "enabled" {
				id = "dev.floter.imported"
				entry.Manifest = json.RawMessage(strings.ReplaceAll(string(entry.Manifest), entry.ID, id))
				entry.ScriptContent = strings.ReplaceAll(entry.ScriptContent, entry.ID, id)
				entry.ID = id
			}
			entry.Enabled = false
			entry.Config = map[string]any{"endpoint": "https://workspace.example"}
			dataDir := filepath.Join(paths.Data, id)
			if stage == "template" {
				var manifest map[string]any
				if err := json.Unmarshal(entry.Manifest, &manifest); err != nil {
					t.Fatal(err)
				}
				manifest["lifecycle"] = map[string]any{"configurationTemplates": []map[string]string{{"source": "tool.sh", "target": "defaults/tool.conf"}}}
				entry.Manifest, err = json.Marshal(manifest)
				if err != nil {
					t.Fatal(err)
				}
				recoveryWrite(t, dataDir, []byte("blocked data directory"))
			} else {
				recoveryWrite(t, filepath.Join(dataDir, "config.json"), []byte(`{"values":`))
			}
			doc.Extensions = []extensions.SyncEntry{entry}
			data, err := doc.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "workspace.json")
			recoveryWrite(t, path, data)
			a.openFileDialog = func() (string, error) { return path, nil }
			tt := transferCauseFrame(t, a, dark)
			recoveryTransfer(t, a, tt, false)
			recoveryDump(t, tt, "partial-"+stage, dark)
			repo, err := extensions.LoadRepository(paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := repo.Extensions[id]
			if !ok {
				t.Fatal("fixture did not commit the integration")
			}
			if stage != "template" && got.Enabled {
				t.Fatal("fixture did not commit enabled=false before failing")
			}
			r := a.Settings.TransferReport
			if r == nil || len(r.Failed) != 1 || r.Succeeded != 0 || r.Skipped != 0 {
				t.Fatalf("partial result=%+v", r)
			}
			want := "enabled state was set to false"
			if stage != "enabled" {
				want = "package was installed"
			}
			if !strings.Contains(r.Failed[0].Reason, want) {
				t.Errorf("partial %s hides committed state: %s", stage, r.Failed[0].Reason)
			}
			if !strings.Contains(strings.Join(tt.Texts(), "\n"), want) {
				t.Errorf("partial %s state is absent from the visible report", stage)
			}
			if a.Settings.TransferBusy != "" {
				t.Error("partial failure retained busy")
			}
			if stage == "template" {
				if !got.Enabled {
					t.Error("template failure unexpectedly applied the later disabled state")
				}
				if !strings.Contains(r.Failed[0].Reason, "configuration templates") {
					t.Error("template failure hides its stage")
				}
			} else {
				if !strings.Contains(r.Failed[0].Reason, "unexpected end of JSON input") {
					t.Error("partial failure lost original parse cause")
				}
				broken, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
				if err != nil || string(broken) != `{"values":` {
					t.Fatal("partial failure overwrote damaged configuration")
				}
				recoveryWrite(t, filepath.Join(dataDir, "config.json"), []byte(`{"values":{}}`))
				recoveryTransfer(t, a, tt, false)
				if r := a.Settings.TransferReport; r == nil || r.Succeeded != 1 || len(r.Failed) != 0 {
					t.Errorf("repaired import failed: %+v", r)
				}
				stored, err := extensions.LoadStoredConfiguration(paths.Data, id)
				if err != nil || stored.Values["endpoint"] != entry.Config["endpoint"] {
					t.Errorf("retry did not apply values: %+v %v", stored, err)
				}
			}
		})
	}
}

func checkTransferRecovery(t *testing.T, operation string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "backup.json")
			pickerError := error(nil)
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return destination, pickerError })
			tt := transferCauseFrame(t, a, dark)
			values := transferFeedbackValues()
			var copied string
			a.writeClipboard = func(p ClipboardPayload) { copied = p.Text }
			done := make(chan error, 1)
			path := filepath.Join(t.TempDir(), "import.json")
			a.openFileDialog = func() (string, error) { return path, nil }
			run := func() {
				switch operation {
				case "export":
					recoveryTransfer(t, a, tt, true)
				case "import":
					recoveryTransfer(t, a, tt, false)
				case "copy", "config-export":
					action := a.Settings.Actions.CopyConfiguration
					if operation == "config-export" {
						action = a.Settings.Actions.ExportConfiguration
					}
					action("dev.floter.configured", values, func(err error) { done <- err })
					transferFeedbackAwait(t, done)
				case "script":
					a.Settings.Actions.ExportScript("diagnose.py", "print('ready')\n")
					id := scriptExportNoticeID
					if pickerError != nil {
						id = "floter-notice"
					}
					waitForToast(t, a, id)
				}
				tt.Frame()
				tt.Frame()
			}
			original, err := os.ReadFile(paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "export":
				recoveryWrite(t, paths.RepositoryFile, []byte("broken repository"))
			case "import":
				recoveryWrite(t, path, []byte(`{"version":99}`))
			case "copy":
				values["endpoint"] = "https://broken\xff.example"
			default:
				pickerError = errors.New("file picker unavailable: restart the desktop portal")
			}
			run()
			before := strings.Join(tt.Texts(), "\n")
			cause := "file picker unavailable: restart the desktop portal"
			switch operation {
			case "export":
				cause = "invalid character"
			case "import":
				cause = "import version 99"
			case "copy":
				cause = "invalid UTF-8"
			}
			if !strings.Contains(before, cause) {
				t.Fatalf("fixture did not display the initial error %q: %s", cause, before)
			}
			pickerError = nil
			values["endpoint"] = "https://example.com"
			recoveryWrite(t, paths.RepositoryFile, original)
			recoveryWrite(t, path, []byte(`{"version":2,"extensions":[]}`))
			run()
			recoveryDump(t, tt, "retry-"+operation, dark)
			if strings.Contains(strings.Join(tt.Texts(), "\n"), cause) {
				t.Errorf("successful %s retry retains its resolved error", operation)
			}
			success := a.SettingsCopy().IntegrationsExported(1)
			switch operation {
			case "import":
				success = a.SettingsCopy().IntegrationsImportSummary(0, 0)
			case "copy":
				success = a.SettingsCopy().ConfigCopied
			case "config-export":
				success = a.SettingsCopy().ConfigExported
			case "script":
				success = a.SettingsCopy().LocalToolScriptExported
			}
			if !tt.HasText(success) {
				t.Errorf("retry lost its success result: %q", success)
			}
			if operation == "copy" {
				if !strings.Contains(copied, "https://example.com") || strings.Contains(copied, "private-token") {
					t.Error("successful copy lost data or leaked secret")
				}
			} else if operation != "import" {
				data, err := os.ReadFile(destination)
				if err != nil || len(data) == 0 {
					t.Errorf("retry did not write file: %v", err)
				}
			}
		})
	}
}

func TestImportEnabledFailureReportsCommittedState(t *testing.T) {
	checkImportAppliedFailure(t, "enabled")
}
func TestImportPackageFailureReportsCommittedState(t *testing.T) {
	checkImportAppliedFailure(t, "package")
}

func TestImportTemplateFailureReportsCommittedState(t *testing.T) {
	checkImportAppliedFailure(t, "template")
}
func TestIntegrationExportRetryRetiresResolvedError(t *testing.T) { checkTransferRecovery(t, "export") }

func TestConfigurationCopyRetryRetiresResolvedError(t *testing.T) { checkTransferRecovery(t, "copy") }
func TestConfigurationExportRetryRetiresResolvedError(t *testing.T) {
	checkTransferRecovery(t, "config-export")
}

func TestResolvedNoticeKeepsAnotherOperationsError(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, replacement := range []string{"external", "another-operation"} {
			t.Run(replacement+map[bool]string{false: "/painted", true: "/queued"}[pending], func(t *testing.T) {
				a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
				tt := transferCauseFrame(t, a, false)
				attempt := a.beginNotice("integrations-export")
				a.transferFailedFor(attempt, "initial export failure", nil)
				if !pending {
					tt.Frame()
				}
				if replacement == "external" {
					a.queueNotice("unrelated failure", refusalNotice)
				} else {
					a.transferFailedFor(a.beginNotice("another-operation"), "unrelated failure", nil)
				}
				if !pending {
					tt.Frame()
				}
				a.resolveNotice(attempt)
				tt.Frame()
				if !tt.HasText("unrelated failure") {
					t.Fatal("retry erased another operation's error")
				}
			})
		}
	}
}

func TestScriptExportRetryRetiresResolvedError(t *testing.T)      { checkTransferRecovery(t, "script") }
func TestIntegrationImportRetryRetiresResolvedError(t *testing.T) { checkTransferRecovery(t, "import") }

func TestOlderCompletionKeepsNewerFailure(t *testing.T) {
	a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
	tt := transferCauseFrame(t, a, false)
	older := a.beginNotice("script-export:diagnose.py")
	newer := a.beginNotice("script-export:diagnose.py")
	a.transferFailedFor(newer, "newer export failure", nil)
	tt.Frame()
	a.resolveNotice(older)
	tt.Frame()
	if !tt.HasText("newer export failure") {
		t.Fatal("older completion erased a newer failure")
	}
	a.resolveNotice(a.beginNotice("script-export:other.py"))
	tt.Frame()
	if !tt.HasText("newer export failure") {
		t.Fatal("another script erased this failure")
	}
	a.resolveNotice(a.beginNotice("script-export:diagnose.py"))
	tt.Frame()
	if tt.HasText("newer export failure") {
		t.Fatal("current retry did not resolve its old error")
	}
}
