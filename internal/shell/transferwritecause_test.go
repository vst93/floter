package shell

import (
	"os"
	"path/filepath"
	"testing"
)

func checkTransferWriteCause(t *testing.T, config bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		appearance := map[bool]string{false: "light", true: "dark"}[dark]
		t.Run(appearance, func(t *testing.T) {
			// A real directory cannot be overwritten as a file on any platform.
			dir := t.TempDir()
			destination := dir
			want := os.WriteFile(destination, nil, 0o600)
			if want == nil {
				t.Fatal("directory unexpectedly accepted a file write")
			}
			a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return destination, nil })
			name := "integrations-write"
			if config {
				name = "config-write"
			}
			run := func() error {
				if config {
					done := make(chan error, 1)
					a.Settings.Actions.ExportConfiguration("dev.floter.configured", transferFeedbackValues(), func(err error) { done <- err })
					return transferFeedbackAwait(t, done)
				}
				a.Settings.Actions.ExportIntegrations()
				waitForTransfer(t, a)
				return nil
			}
			clearNotices(a)
			err := run()
			if config && (err == nil || err.Error() != want.Error()) {
				t.Errorf("write callback=%v want=%v", err, want)
			}
			tt := transferFeedbackFrame(t, a, name, dark)
			configTargetDump(t, tt, name, appearance)
			if !tt.HasText(want.Error()) {
				t.Errorf("%s hides actual filesystem failure: %v", name, want)
			}
			if tt.HasText(a.SettingsCopy().IntegrationsTransferBad) {
				t.Errorf("%s wrongly reports an invalid import document", name)
			}
			if a.Settings.TransferBusy != "" {
				t.Error("write failure retained transfer lock")
			}
			clearNotices(a)
			destination = filepath.Join(dir, "retry.json")
			if err := run(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(destination)
			if err != nil || len(data) == 0 {
				t.Errorf("write retry did not produce a document: %v", err)
			}
		})
	}
}

func TestConfigExportWriteCause(t *testing.T)       { checkTransferWriteCause(t, true) }
func TestIntegrationsExportWriteCause(t *testing.T) { checkTransferWriteCause(t, false) }
