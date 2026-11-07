package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settingsui"
)

func TestIntegrationImportDocumentFailureExplainsCause(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, content := range []string{`{"version":99,"extensions":[]}`, `not a document`} {
			t.Run(map[bool]string{false: "light", true: "dark"}[dark]+"/"+content, func(t *testing.T) {
				a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
				path := filepath.Join(t.TempDir(), "backup.json")
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				_, want := extensions.ReadSync([]byte(content))
				a.openFileDialog = func() (string, error) { return path, nil }
				clearNotices(a)
				tt := transferCauseFrame(t, a, dark)
				a.Settings.Page = settingsui.PageIntegrations
				tt.Frame()
				tt.Frame()
				a.Settings.Actions.ImportIntegrations()
				waitForTransfer(t, a)
				tt.Frame()
				tt.Frame()
				name := "import-version"
				if content == "not a document" {
					name = "import-syntax"
				}
				transferCauseDump(t, tt, name, dark)
				if !tt.HasText(a.SettingsCopy().IntegrationsTransferBad) {
					t.Error("document refusal lost its localized heading")
				}
				if !tt.HasText(want.Error()) {
					t.Errorf("document refusal hides its cause: %s", want)
				}
				if a.Settings.TransferReport != nil || a.Settings.TransferBusy != "" {
					t.Error("invalid document fabricated report or retained busy")
				}
				if err := os.WriteFile(path, []byte(`{"version":2,"extensions":[]}`), 0o600); err != nil {
					t.Fatal(err)
				}
				a.Settings.Actions.ImportIntegrations()
				waitForTransfer(t, a)
				if a.Settings.TransferReport == nil {
					t.Error("corrected document could not be retried")
				}
			})
		}
	}
}

func TestIntegrationImportRepositoryFailureDoesNotClaimSkipped(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, paths := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
			original, err := os.ReadFile(paths.RepositoryFile)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "backup.json")
			if err := os.WriteFile(path, []byte(`{"version":2,"extensions":[{"id":"dev.floter.configured","enabled":true}]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			broken := []byte("broken repository")
			if err := os.WriteFile(paths.RepositoryFile, broken, 0o600); err != nil {
				t.Fatal(err)
			}
			_, want := extensions.LoadRepository(paths.RepositoryFile)
			a.openFileDialog = func() (string, error) { return path, nil }
			clearNotices(a)
			tt := transferCauseFrame(t, a, dark)
			a.Settings.Page = settingsui.PageIntegrations
			tt.Frame()
			tt.Frame()
			a.Settings.Actions.ImportIntegrations()
			waitForTransfer(t, a)
			tt.Frame()
			tt.Frame()
			transferCauseDump(t, tt, "import-repository", dark)
			if !tt.HasText(want.Error()) {
				t.Errorf("unreadable repository was treated as empty: missing %s", want)
			}
			if a.Settings.TransferReport != nil {
				t.Errorf("unreadable repository fabricated import report: %+v", a.Settings.TransferReport)
			}
			if a.Settings.TransferBusy != "" {
				t.Error("repository failure retained busy")
			}
			got, err := os.ReadFile(paths.RepositoryFile)
			if err != nil || !bytes.Equal(got, broken) {
				t.Error("failed import changed the repository")
			}
			if err := os.WriteFile(paths.RepositoryFile, original, 0o600); err != nil {
				t.Fatal(err)
			}
			a.Settings.Actions.ImportIntegrations()
			waitForTransfer(t, a)
			if report := a.Settings.TransferReport; report == nil || report.Succeeded != 1 || report.Skipped != 0 || len(report.Failed) != 0 {
				t.Errorf("repaired repository retry=%+v", report)
			}
		})
	}
}
