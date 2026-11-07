package settingsui

import (
	"errors"
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
)

func checkConfigTransferRowBusy(t *testing.T, export bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Broken: true,
				RuntimeOwnership: "system", FailureCode: extensions.BindingCodeMissing,
				Config:       []ConfigField{{Key: "endpoint", Label: "Endpoint", Type: "text"}},
				ConfigValues: map[string]any{"endpoint": "https://workspace.example"}}
			a.Integrations = func() []Integration { return []Integration{in} }
			var answer func(error)
			a.Actions.RepairIntegration = func(_ string, done func(error)) { answer = done }
			calls := 0
			a.Actions.CopyConfiguration = func(id string, values map[string]any, done func(error)) {
				calls++
				if id != in.ID || values["endpoint"] != in.ConfigValues["endpoint"] {
					t.Error("transfer lost its target or draft")
				}
				done(nil)
			}
			a.Actions.ExportConfiguration = a.Actions.CopyConfiguration
			tt := noticeExitPage(t, a, dark)
			if err := tt.Click(a.copy().ConfigOpen); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			label, name := a.copy().ConfigCopy, "copy-row-busy"
			if export {
				label, name = a.copy().ConfigExport, "export-row-busy"
			}
			if err := tt.Click(a.copy().IntegrationsRecheck); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if answer == nil || !a.rowBusy(in.ID) {
				t.Fatal("real repair action did not claim row")
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if tag := os.Getenv("FLOTER_P13JE_DUMP"); tag != "" {
				f, err := os.Create("/tmp/floter-P13-je-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
			// The existing option-exit contract keeps draft transfers reachable
			// while the integration itself is busy.
			if calls != 1 {
				t.Errorf("busy row lost %s keyboard outlet: calls=%d", name, calls)
			}
			answer(errors.New("repair retry"))
			tt.Frame()
			tt.Frame()
			if a.rowBusy(in.ID) {
				t.Fatal("repair failure retained busy lock")
			}
			before := calls
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != before+1 {
				t.Errorf("settled row could not retry %s", name)
			}
		})
	}
}

func TestConfigTransferCopyRemainsRowBusyOutlet(t *testing.T)   { checkConfigTransferRowBusy(t, false) }
func TestConfigTransferExportRemainsRowBusyOutlet(t *testing.T) { checkConfigTransferRowBusy(t, true) }
