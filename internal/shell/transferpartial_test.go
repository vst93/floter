package shell

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func importResultDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JG_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jg-" + name + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func importResultFixture(t *testing.T, dark bool, entries []extensions.SyncEntry) (*App, *ui.Tester) {
	t.Helper()
	a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
	data, err := (extensions.SyncDocument{Version: 2, Extensions: entries}).Marshal()
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
	return a, tt
}

func TestImportPartialResultRetainsAppliedSummary(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, tt := importResultFixture(t, dark, []extensions.SyncEntry{
				{ID: "dev.floter.configured", Enabled: true},
				{ID: "dev.workspace.missing"},
				{ID: "dev.workspace.broken", Manifest: json.RawMessage(`{"id":"dev.workspace.other","name":"Other","distribution":{"type":"local"},"runtime":{"type":"system","executableNames":["sh"]},"provider":{"type":"executable"}}`)},
			})
			importResultDump(t, tt, "partial", dark)
			r := a.Settings.TransferReport
			if r == nil || r.Succeeded != 1 || r.Skipped != 1 || len(r.Failed) != 1 {
				t.Fatalf("mixed import result=%+v", r)
			}
			if !tt.HasText(a.SettingsCopy().IntegrationsImportSummary(1, 1)) {
				t.Error("partial failure hides the already-applied result")
			}
			if !tt.HasText(a.SettingsCopy().IntegrationsImportFailedDetail(r.Failed[0].ID + ": " + r.Failed[0].Reason)) {
				t.Error("partial result lost failure details")
			}
			if a.Settings.TransferBusy != "" {
				t.Error("partial result retained busy")
			}
		})
	}
}

func TestImportSkippedResultExplainsUnchangedEntry(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			a, tt := importResultFixture(t, dark, []extensions.SyncEntry{{ID: "dev.workspace.missing"}})
			importResultDump(t, tt, "skipped", dark)
			if !tt.HasText("dev.workspace.missing: not installed here, and the export carries no package") {
				t.Error("skipped import hides the integration and its reason")
			}
			if !tt.HasText(a.SettingsCopy().IntegrationsImportSummary(0, 1)) {
				t.Error("skipped result lost its summary")
			}
		})
	}
}
