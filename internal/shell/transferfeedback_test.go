package shell

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"floter/internal/clipboard"
	"floter/internal/extensions"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func transferFeedbackValues() map[string]any {
	return map[string]any{"endpoint": "https://example.com", "token": "private-token", "region": "eu"}
}

func transferFeedbackAwait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("transfer did not settle")
		return nil
	}
}

func transferFeedbackFrame(t *testing.T, a *App, name string, dark bool) *ui.Tester {
	t.Helper()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	a.Open(SurfaceSettings)
	tt := ui.NewTester(a.View, 720, 520)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	if tt.HasText(a.SettingsCopy().IntegrationsTransfer) {
		settingsSectionForTest(t, a, tt, a.SettingsCopy().IntegrationsTransfer)
	}
	if tag := os.Getenv("FLOTER_P13JD_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jd-" + name + "-" + appearance + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	return tt
}

func TestConfigCopyWaitsForClipboardDelivery(t *testing.T) {
	a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
	entered, release, done := make(chan ClipboardPayload, 1), make(chan struct{}, 1), make(chan error, 1)
	defer close(release)
	a.writeClipboard = func(payload ClipboardPayload) { entered <- payload; <-release }
	clearNotices(a)
	a.Settings.Actions.CopyConfiguration("dev.floter.configured", transferFeedbackValues(), func(err error) { done <- err })
	select {
	case payload := <-entered:
		if payload.Kind != clipboard.KindText || strings.Contains(payload.Text, "private-token") || !strings.Contains(payload.Text, "[REDACTED]") {
			t.Errorf("copy backend received invalid or unredacted payload: kind=%s", payload.Kind)
		}
	case <-time.After(time.Second):
		t.Error("configuration copy bypassed the configured clipboard backend")
	}
	if noticeSays(a, a.SettingsCopy().ConfigCopied) {
		t.Error("configuration copy announced success before clipboard delivery")
	}
	select {
	case <-done:
		t.Error("configuration copy completed before clipboard delivery")
		return
	default:
	}
	// Unblock explicitly, with the deferred close retained for failed assertions.
	release <- struct{}{}
	if err := transferFeedbackAwait(t, done); err != nil {
		t.Fatal(err)
	}
	if !noticeSays(a, a.SettingsCopy().ConfigCopied) {
		t.Error("delivered configuration copy has no success notice")
	}
}

func checkConfigValidationFeedback(t *testing.T, export bool) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, _ := configuredIntegrationFixture(t, func(string) (string, error) { t.Error("invalid draft opened a file picker"); return "", nil })
			a.writeClipboard = func(ClipboardPayload) { t.Error("invalid draft reached clipboard") }
			clearNotices(a)
			values := transferFeedbackValues()
			delete(values, "endpoint")
			desc, _ := a.Integrations.Description("dev.floter.configured")
			_, want := extensions.ExportConfiguration("dev.floter.configured", desc.Configuration.ConfigVersion, desc.Configuration.Schema, values)
			if want == nil {
				t.Fatal("invalid fixture accepted")
			}
			done := make(chan error, 1)
			run, name := a.Settings.Actions.CopyConfiguration, "copy-validation"
			if export {
				run, name = a.Settings.Actions.ExportConfiguration, "export-validation"
			}
			run("dev.floter.configured", values, func(err error) { done <- err })
			if err := transferFeedbackAwait(t, done); err == nil || err.Error() != want.Error() {
				t.Errorf("validation callback=%v want=%v", err, want)
			}
			tt := transferFeedbackFrame(t, a, name, dark)
			if !tt.HasText(want.Error()) {
				t.Errorf("configuration validation hid its actionable cause: want %q", want.Error())
			}
			if tt.HasText(a.SettingsCopy().IntegrationsTransferBad) {
				t.Error("configuration validation incorrectly accuses an integrations file")
			}
		})
	}
}

func TestConfigCopyReportsValidationCause(t *testing.T)   { checkConfigValidationFeedback(t, false) }
func TestConfigExportReportsValidationCause(t *testing.T) { checkConfigValidationFeedback(t, true) }

func checkTransferDialogFailure(t *testing.T, kind string) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			failure := errors.New("file picker unavailable: restart the desktop portal")
			a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return "", failure })
			a.openFileDialog = func() (string, error) { return "", failure }
			clearNotices(a)
			run := func() {
				if kind == "config" {
					done := make(chan error, 1)
					a.Settings.Actions.ExportConfiguration("dev.floter.configured", transferFeedbackValues(), func(err error) { done <- err })
					if err := transferFeedbackAwait(t, done); !errors.Is(err, failure) {
						t.Errorf("picker callback=%v", err)
					}
				} else {
					if kind == "export" {
						a.Settings.Actions.ExportIntegrations()
					} else {
						a.Settings.Actions.ImportIntegrations()
					}
					waitForTransfer(t, a)
					if a.Settings.TransferBusy != "" {
						t.Error("picker failure retained busy state")
					}
				}
			}
			run()
			tt := transferFeedbackFrame(t, a, kind+"-dialog-error", dark)
			if !tt.HasText(failure.Error()) {
				t.Errorf("%s picker failure is invisible", kind)
			}
			clearNotices(a)
			failure = nil
			run()
			if pendingNotices(a) != 0 {
				t.Error("cancelled picker raised a notice")
			}
		})
	}
}

func TestConfigExportPickerFailureIsVisible(t *testing.T) { checkTransferDialogFailure(t, "config") }
func TestIntegrationsExportPickerFailureIsVisible(t *testing.T) {
	checkTransferDialogFailure(t, "export")
}
func TestIntegrationsImportPickerFailureIsVisible(t *testing.T) {
	checkTransferDialogFailure(t, "import")
}

func TestConfigTransferSuccessPreservesOtherRefusal(t *testing.T) {
	for _, export := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			t.Run(map[bool]string{false: "copy", true: "export"}[export]+"/"+map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
				destination := filepath.Join(t.TempDir(), "config.json")
				a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return destination, nil })
				a.writeClipboard = func(ClipboardPayload) {}
				clearNotices(a)
				a.queueNotice("another tool needs permission", refusalNotice)
				done := make(chan error, 1)
				run, success, name := a.Settings.Actions.CopyConfiguration, a.SettingsCopy().ConfigCopied, "copy-preserves-refusal"
				if export {
					run, success, name = a.Settings.Actions.ExportConfiguration, a.SettingsCopy().ConfigExported, "export-preserves-refusal"
				}
				run("dev.floter.configured", transferFeedbackValues(), func(err error) { done <- err })
				if err := transferFeedbackAwait(t, done); err != nil {
					t.Fatal(err)
				}
				tt := transferFeedbackFrame(t, a, name, dark)
				if !tt.HasText(success) || !tt.HasText("another tool needs permission") {
					t.Error("configuration success replaced the unrelated refusal")
				}
			})
		}
	}
}
