package shell

import (
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
)

func TestIntegrationExportEncodingExplainsFailure(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "backup.json")
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return destination, nil })
			pkg := filepath.Join(paths.Extensions, "dev.floter.configured")
			if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(`{"schemaVersion":"2.0","id":"dev.floter.configured","name":"Configured","distribution":{"type":"local"},"runtime":{"type":"system","executableNames":["sh"]},"provider":{"type":"static-descriptor","descriptor":"description.json"}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			descriptor := filepath.Join(pkg, "description.json")
			if err := os.WriteFile(descriptor, []byte("broken descriptor"), 0o600); err != nil {
				t.Fatal(err)
			}
			document, err := extensions.ExportSync(paths)
			if err != nil {
				t.Fatal(err)
			}
			_, want := document.Marshal()
			if want == nil {
				t.Fatal("malformed descriptor did not reach the encoder")
			}
			if err := os.WriteFile(destination, []byte("existing backup"), 0o600); err != nil {
				t.Fatal(err)
			}
			clearNotices(a)
			a.Settings.Actions.ExportIntegrations()
			waitForTransfer(t, a)
			tt := transferCauseFrame(t, a, dark)
			transferCauseDump(t, tt, "export-encoding", dark)
			if !tt.HasText(want.Error()) {
				t.Errorf("encoding failure hides its cause: %s", want)
			}
			data, err := os.ReadFile(destination)
			if err != nil || string(data) != "existing backup" {
				t.Error("encoding failure overwrote the existing backup")
			}
			if a.Settings.TransferBusy != "" {
				t.Error("encoding failure retained transfer lock")
			}
			if err := os.WriteFile(descriptor, []byte(`{"protocolVersion":"1.0","provider":{"id":"dev.floter.configured","name":"Configured"}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			a.Settings.Actions.ExportIntegrations()
			waitForTransfer(t, a)
			data, err = os.ReadFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := extensions.ReadSync(data); err != nil {
				t.Fatalf("encoding retry failed: %v", err)
			}
		})
	}
}
