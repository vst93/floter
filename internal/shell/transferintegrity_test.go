package shell

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func integrityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JH_DUMP"); tag != "" {
		appearance := map[bool]string{false: "light", true: "dark"}[dark]
		f, err := os.Create("/tmp/floter-P13-jh-" + name + "-" + appearance + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfigurationTransferPreservesInvalidText(t *testing.T) {
	for _, export := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			name := map[bool]string{false: "copy", true: "config-export"}[export]
			t.Run(name+"/"+map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
				destination := filepath.Join(t.TempDir(), "config.json")
				original := []byte("existing good configuration backup")
				if err := os.WriteFile(destination, original, 0o600); err != nil {
					t.Fatal(err)
				}
				pickers, copies := 0, 0
				a, _ := configuredIntegrationFixture(t, func(string) (string, error) { pickers++; return destination, nil })
				var clipboardText string
				a.writeClipboard = func(payload ClipboardPayload) { copies++; clipboardText = payload.Text }
				run := a.Settings.Actions.CopyConfiguration
				if export {
					run = a.Settings.Actions.ExportConfiguration
				}
				tt := transferCauseFrame(t, a, dark)
				clearNotices(a)
				values := transferFeedbackValues()
				values["endpoint"] = "https://exa\xffmple.com"
				done := make(chan error, 1)
				run("dev.floter.configured", values, func(err error) { done <- err })
				err := transferFeedbackAwait(t, done)
				tt.Frame()
				tt.Frame()
				integrityDump(t, tt, name, dark)
				if err == nil || !strings.Contains(err.Error(), "UTF-8") {
					t.Errorf("configuration transfer silently repaired invalid text: %v", err)
				}
				text := strings.Join(tt.Texts(), "\n")
				if !strings.Contains(text, "UTF-8") || !strings.Contains(text, "endpoint") {
					t.Error("configuration transfer hides the damaged field")
				}
				if copies != 0 || pickers != 0 {
					t.Errorf("invalid configuration escaped: copies=%d pickers=%d", copies, pickers)
				}
				got, err := os.ReadFile(destination)
				if err != nil || !bytes.Equal(got, original) {
					t.Error("invalid configuration replaced the old backup")
				}
				// A valid replacement character is real user text, not evidence of corruption.
				values["endpoint"] = "https://例子.example/�"
				run("dev.floter.configured", values, func(err error) { done <- err })
				if err := transferFeedbackAwait(t, done); err != nil {
					t.Fatalf("corrected configuration cannot be retried: %v", err)
				}
				data := []byte(clipboardText)
				if export {
					data, err = os.ReadFile(destination)
					if err != nil {
						t.Fatal(err)
					}
				}
				var doc extensions.ConfigurationExportDocument
				if err := json.Unmarshal(data, &doc); err != nil || doc.Values["endpoint"] != values["endpoint"] || doc.Values["token"] != extensions.PasswordPlaceholder {
					t.Errorf("retry changed text or lost redaction: %v", err)
				}
			})
		}
	}
}

func checkBackupIntegrity(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "workspace.json")
			original := []byte("existing restorable backup")
			if err := os.WriteFile(destination, original, 0o600); err != nil {
				t.Fatal(err)
			}
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return destination, nil })
			pkg := filepath.Join(paths.Extensions, "dev.floter.configured")
			source := filepath.Join(pkg, "tool.sh")
			good, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			var broken []byte
			cause := "UTF-8"
			switch kind {
			case "stored-text":
				source = filepath.Join(paths.Data, "dev.floter.configured", "config.json")
				good = []byte(`{"endpoint":"https://例子.example/�"}`)
				broken = []byte("{\"endpoint\":\"https://exa\xffmple.com\"}")
				if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
					t.Fatal(err)
				}
			case "manifest":
				source = filepath.Join(pkg, "floter.extension.json")
				good, err = os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				broken = []byte("{broken manifest")
				cause = "manifest"
			case "empty-script":
				cause = "empty"
			case "script-text":
				broken = []byte("#!/bin/sh\necho 'caf\xff'\n")
			case "empty-descriptor", "descriptor-text":
				manifest := []byte(`{"schemaVersion":"2.0","id":"dev.floter.configured","name":"Configured","distribution":{"type":"local"},"runtime":{"type":"system","executableNames":["sh"]},"provider":{"type":"static-descriptor","descriptor":"description.json"}}`)
				if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), manifest, 0o600); err != nil {
					t.Fatal(err)
				}
				source = filepath.Join(pkg, "description.json")
				good = []byte(`{"protocolVersion":"1.0","provider":{"id":"dev.floter.configured","name":"Configured"}}`)
				if kind == "descriptor-text" {
					broken = bytes.Replace(good, []byte("Configured"), []byte("caf\xff"), 1)
				} else {
					cause = "empty"
				}
			case "oversize":
				// The source fits below 4 MiB, but JSON escaping makes the actual
				// file exceed it. The limit must use the final encoded bytes.
				broken = []byte(strings.Repeat("\t", (2<<20)+1))
				cause = "4194304"
			default:
				t.Fatalf("unknown fixture %s", kind)
			}
			if err := os.WriteFile(source, broken, 0o600); err != nil {
				t.Fatal(err)
			}
			tt := transferCauseFrame(t, a, dark)
			clearNotices(a)
			a.Settings.Actions.ExportIntegrations()
			waitForTransfer(t, a)
			tt.Frame()
			tt.Frame()
			integrityDump(t, tt, kind, dark)
			text := strings.Join(tt.Texts(), "\n")
			if !strings.Contains(text, cause) || (kind != "oversize" && !strings.Contains(text, filepath.Base(source))) {
				t.Errorf("%s backup hides the damaged source and cause", kind)
			}
			got, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(got, original) {
				t.Errorf("%s export replaced a good backup with unrestorable content", kind)
			}
			if a.Settings.TransferBusy != "" || noticeSays(a, a.SettingsCopy().IntegrationsExported(1)) {
				t.Error("failed backup retained busy or claimed success")
			}
			if err := os.WriteFile(source, good, 0o755); err != nil {
				t.Fatal(err)
			}
			a.Settings.Actions.ExportIntegrations()
			waitForTransfer(t, a)
			got, err = os.ReadFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := extensions.ReadSync(got)
			if err != nil || len(doc.Extensions) != 1 {
				t.Fatalf("repaired %s backup cannot be imported: %v", kind, err)
			}
			entry := doc.Extensions[0]
			if kind == "stored-text" && entry.Config["endpoint"] != "https://例子.example/�" {
				t.Error("corrected stored text changed in backup")
			}
			if (kind == "empty-script" || kind == "script-text") && entry.ScriptContent != string(good) {
				t.Error("repaired script changed in backup")
			}
			if strings.Contains(kind, "descriptor") {
				var compact bytes.Buffer
				if err := json.Compact(&compact, entry.ProviderDescriptor); err != nil || !bytes.Equal(compact.Bytes(), good) {
					t.Error("repaired descriptor changed in backup")
				}
			}
		})
	}
}

func TestBackupRejectsInvalidStoredText(t *testing.T) { checkBackupIntegrity(t, "stored-text") }

func TestBackupRejectsBrokenDeclaredManifest(t *testing.T) { checkBackupIntegrity(t, "manifest") }
func TestBackupRejectsEmptyScript(t *testing.T)            { checkBackupIntegrity(t, "empty-script") }

func TestBackupRejectsInvalidScriptText(t *testing.T) { checkBackupIntegrity(t, "script-text") }
func TestBackupRejectsEmptyDescriptor(t *testing.T)   { checkBackupIntegrity(t, "empty-descriptor") }
