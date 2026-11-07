package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settingsui"
)

func checkImportInvalidText(t *testing.T, name string, data []byte, cause string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
			original, err := os.ReadFile(paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "workspace.json")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			a.openFileDialog = func() (string, error) { return path, nil }
			tt := transferCauseFrame(t, a, dark)
			a.Settings.Page = settingsui.PageIntegrations
			tt.Frame()
			a.Settings.Actions.ImportIntegrations()
			waitForTransfer(t, a)
			tt.Frame()
			tt.Frame()
			importResultDump(t, tt, name, dark)
			if !tt.HasText(a.SettingsCopy().IntegrationsTransferBad) || !tt.HasText(cause) {
				t.Errorf("invalid import lacks visible refusal: %s", cause)
			}
			if a.Settings.TransferReport != nil || a.Settings.TransferBusy != "" {
				t.Errorf("invalid document accepted or busy retained: %+v", a.Settings.TransferReport)
			}
			got, err := os.ReadFile(paths.RepositoryFile)
			if err != nil || !bytes.Equal(got, original) {
				t.Error("invalid document changed repository")
			}
			if _, err := os.Stat(filepath.Join(paths.Data, "dev.floter.configured", "config.json")); !os.IsNotExist(err) {
				t.Error("invalid document wrote configuration")
			}
			// Corrected bytes can be retried through exactly the same action.
			good := []byte(`{"version":2,"extensions":[{"id":"dev.floter.configured","enabled":true,"config":{"endpoint":"https://example.com"}}]}`)
			if err := os.WriteFile(path, good, 0o600); err != nil {
				t.Fatal(err)
			}
			a.Settings.Actions.ImportIntegrations()
			waitForTransfer(t, a)
			if r := a.Settings.TransferReport; r == nil || r.Succeeded != 1 || len(r.Failed) != 0 {
				t.Errorf("corrected import could not recover: %+v", r)
			}
			stored, err := extensions.LoadStoredConfiguration(paths.Data, "dev.floter.configured")
			if err != nil || stored.Values["endpoint"] != "https://example.com" {
				t.Errorf("corrected config=%+v err=%v", stored, err)
			}
		})
	}
}

func TestImportNegativeVersionRefusedBeforeWrite(t *testing.T) {
	checkImportInvalidText(t, "negative-version", []byte(`{"version":-1,"extensions":[{"id":"dev.floter.configured","enabled":false,"config":{"endpoint":"changed"}}]}`), "extensions: import version -1 is not supported")
}

func TestImportInvalidUTF8RefusedBeforeWrite(t *testing.T) {
	data := append([]byte(`{"version":2,"extensions":[{"id":"dev.floter.configured","enabled":true,"config":{"endpoint":"https://exa`), 0xff)
	data = append(data, []byte(`mple.com"}}]}`)...)
	checkImportInvalidText(t, "invalid-utf8", data, "extensions: import document contains invalid UTF-8")
}
