package shell

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settingsui"
)

func checkExportMissingEssential(t *testing.T, descriptor bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "backup.json")
			original := []byte("existing good backup")
			if err := os.WriteFile(destination, original, 0o600); err != nil {
				t.Fatal(err)
			}
			pickers := 0
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { pickers++; return destination, nil })
			pkg := filepath.Join(paths.Extensions, "dev.floter.configured")
			source := filepath.Join(pkg, "tool.sh")
			content, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			name := "missing-script"
			if descriptor {
				name = "missing-descriptor"
				source = filepath.Join(pkg, "description.json")
				content = []byte(`{"protocolVersion":"1.0","provider":{"id":"dev.floter.configured","name":"Configured"}}`)
				manifest := []byte(`{"schemaVersion":"2.0","id":"dev.floter.configured","name":"Configured","distribution":{"type":"local"},"runtime":{"type":"system","executableNames":["sh"]},"provider":{"type":"static-descriptor","descriptor":"description.json"}}`)
				if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), manifest, 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Remove(source); err != nil {
				t.Fatal(err)
			}
			_, cause := os.ReadFile(source)
			if cause == nil {
				t.Fatal("fixture still has the essential file")
			}
			tt := transferCauseFrame(t, a, dark)
			a.Settings.Page = settingsui.PageIntegrations
			tt.Frame()
			clearNotices(a)
			a.Settings.Actions.ExportIntegrations()
			waitForTransfer(t, a)
			tt.Frame()
			tt.Frame()
			importResultDump(t, tt, name, dark)
			want := fmt.Sprintf("extensions: exporting dev.floter.configured: %v", cause)
			if !tt.HasText(want) {
				t.Errorf("missing essential was exported without visible cause: %s", want)
			}
			if pickers != 0 {
				t.Errorf("incomplete backup opened %d save pickers", pickers)
			}
			got, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(got, original) {
				t.Error("incomplete export replaced the existing backup")
			}
			if a.Settings.TransferBusy != "" {
				t.Error("failed export retained busy")
			}
			if err := os.WriteFile(source, content, 0o755); err != nil {
				t.Fatal(err)
			}
			a.Settings.Actions.ExportIntegrations()
			waitForTransfer(t, a)
			data, err := os.ReadFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := extensions.ReadSync(data)
			if err != nil || len(doc.Extensions) != 1 || pickers != 1 {
				t.Fatalf("restored export failed: doc=%+v pickers=%d err=%v", doc, pickers, err)
			}
			if descriptor {
				var compact bytes.Buffer
				if err := json.Compact(&compact, doc.Extensions[0].ProviderDescriptor); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(compact.Bytes(), content) {
					t.Errorf("restored descriptor changed: %s", compact.Bytes())
				}
			} else if doc.Extensions[0].ScriptContent != string(content) {
				t.Error("restored export omitted script")
			}
		})
	}
}

func TestExportMissingScriptPreservesBackup(t *testing.T)     { checkExportMissingEssential(t, false) }
func TestExportMissingDescriptorPreservesBackup(t *testing.T) { checkExportMissingEssential(t, true) }
