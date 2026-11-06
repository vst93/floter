package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func formOutletDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IC_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ic-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkFormOutlet(t *testing.T, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, elsewhere := range []bool{false, true} {
			place := "entry"
			if elsewhere {
				place = "elsewhere"
			}
			t.Run(appearanceName(dark)+"/"+place, func(t *testing.T) {
				a := noticeApp(t, dark)
				in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local",
					Config:       []ConfigField{{Key: "token", Label: "Token", Type: "text"}, {Key: "region", Label: "Region", Type: "text"}},
					ConfigValues: map[string]any{"token": "stored", "region": "local"}}
				a.Integrations = func() []Integration { return []Integration{in} }
				calls, opened, checked, closed := 0, 0, 0, 0
				var finish func(error)
				a.Actions.CopyConfiguration = func(id string, values map[string]any, done func(error)) {
					if id != in.ID || values["token"] != "stored" {
						t.Errorf("wrong transfer: %s %v", id, values)
					}
					calls++
					finish = done
				}
				a.Actions.ExportConfiguration = a.Actions.CopyConfiguration
				a.Actions.OpenConfiguration = func(string, []string) { opened++ }
				a.Actions.DiagnoseIntegration = func(string) { checked++ }
				a.Actions.Close = func() { closed++ }
				copy := a.copy()
				tt := noticeExitPage(t, a, dark)
				focus := func(label string) {
					t.Helper()
					for i := 0; i < 40 && !tt.Focused(label); i++ {
						tt.Key(0, ui.KeyTab)
						tt.Frame()
					}
					if !tt.Focused(label) {
						t.Fatalf("cannot reach %q", label)
					}
				}
				if err := tt.Click(copy.ConfigOpen); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				entry := "Token"
				switch change {
				case "copy-owner", "copy-fold", "copy-unavailable", "copy-both":
					entry = copy.ConfigCopy
				case "export-schema", "export-fold", "export-unavailable", "export-both":
					entry = copy.ConfigExport
				case "tool-command", "tool-action", "tool-owner", "tool-no-check":
					in.ConfigOwner, in.ConfigOpenCommand = "tool", []string{"settings"}
					tt.Frame()
					tt.Frame()
					entry = copy.ConfigOpenConfiguration
				}
				focus(entry)
				if change == "copy-owner" || change == "export-schema" || change == "copy-fold" || change == "export-fold" {
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					tt.Frame()
					if finish == nil || calls != 1 {
						t.Fatal("transfer did not start once")
					}
				}
				if elsewhere {
					focus(copy.ShowCommandsInSearch)
				}
				want := copy.ConfigOpen
				switch change {
				case "fold", "copy-fold", "export-fold":
					// The row itself is an existing disclosure target. Its label
					// does not take focus away from the editor before collapsing.
					r := mustFind(t, tt, copy.ConfigSection)
					tt.ClickAt(r.X+2, r.Y+r.H/2)
				case "field-remove":
					in.Config = in.Config[1:]
				case "field-type":
					in.Config[0].Type = "boolean"
				case "copy-owner":
					in.ConfigOwner, in.ConfigOpenCommand = "tool", []string{"settings"}
					want = copy.ConfigOpenConfiguration
				case "export-schema":
					in.Config = nil
					want = copy.IntegrationsCheck
				case "copy-unavailable":
					a.Actions.CopyConfiguration = nil
					want = copy.ConfigExport
				case "export-unavailable":
					a.Actions.ExportConfiguration = nil
					want = copy.ConfigCopy
				case "copy-both", "export-both":
					a.Actions.CopyConfiguration, a.Actions.ExportConfiguration = nil, nil
				case "tool-command":
					in.ConfigOpenCommand = nil
					want = copy.IntegrationsCheck
				case "tool-action":
					a.Actions.OpenConfiguration = nil
					want = copy.IntegrationsCheck
				case "tool-no-check":
					in.ConfigOpenCommand = nil
					a.Actions.DiagnoseIntegration = nil
					want = copy.Close
				case "tool-owner":
					in.ConfigOwner = "host"
					in.Config = nil
					want = copy.IntegrationsCheck
				default:
					t.Fatal("unknown change")
				}
				if elsewhere && change != "fold" && change != "copy-fold" && change != "export-fold" {
					want = copy.ShowCommandsInSearch
				}
				tt.Frame()
				tt.Frame()
				if !elsewhere {
					formOutletDump(t, tt, change, dark)
				}
				if !tt.Focused(want) {
					t.Errorf("%s lost usable focus %q", change, want)
				}
				if opened+checked+closed != 0 {
					t.Error("refresh dispatched action")
				}
				if finish != nil {
					finish(nil)
					tt.Frame()
					tt.Frame()
				}
				if !tt.Focused(want) {
					t.Errorf("%s callback lost focus %q", change, want)
				}
				if change == "fold" || change == "copy-fold" || change == "export-fold" {
					if a.configOpen[in.ID] || tt.HasText("Token") {
						t.Error("row did not fold form")
					}
				}
				if !elsewhere || change == "fold" || change == "copy-fold" || change == "export-fold" {
					beforeCalls, beforeOpen := calls, a.configOpen[in.ID]
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					tt.Frame()
					switch want {
					case copy.ConfigOpen:
						if a.configOpen[in.ID] == beforeOpen {
							t.Error("Configure outlet did not toggle")
						}
					case copy.ConfigCopy, copy.ConfigExport:
						if calls != beforeCalls+1 {
							t.Error("transfer outlet did not dispatch exactly once")
						}
					case copy.IntegrationsCheck:
						if checked != 1 {
							t.Error("Check outlet did not dispatch once")
						}
					case copy.Close:
						if closed != 1 {
							t.Error("header outlet did not close once")
						}
					case copy.ConfigOpenConfiguration:
						if opened != 1 {
							t.Error("tool outlet did not dispatch once")
						}
					}
					if change == "fold" || change == "copy-fold" || change == "export-fold" {
						if calls != beforeCalls {
							t.Error("reopen repeated transfer")
						}
						if !tt.HasText("Token") {
							t.Error("Space did not reopen folded form")
						}
						if !elsewhere {
							formOutletDump(t, tt, change, dark)
						}
					}
				}
			})
		}
	}
}

func TestFormOutletFieldRemove(t *testing.T)       { checkFormOutlet(t, "field-remove") }
func TestFormOutletFieldType(t *testing.T)         { checkFormOutlet(t, "field-type") }
func TestFormOutletCopyUnavailable(t *testing.T)   { checkFormOutlet(t, "copy-unavailable") }
func TestFormOutletExportUnavailable(t *testing.T) { checkFormOutlet(t, "export-unavailable") }
func TestFormOutletTransfersUnavailable(t *testing.T) {
	t.Run("copy", func(t *testing.T) { checkFormOutlet(t, "copy-both") })
	t.Run("export", func(t *testing.T) { checkFormOutlet(t, "export-both") })
}
func TestFormOutletCopyOwner(t *testing.T)    { checkFormOutlet(t, "copy-owner") }
func TestFormOutletExportSchema(t *testing.T) { checkFormOutlet(t, "export-schema") }
func TestFormOutletToolCommand(t *testing.T)  { checkFormOutlet(t, "tool-command") }
func TestFormOutletToolAction(t *testing.T)   { checkFormOutlet(t, "tool-action") }
func TestFormOutletToolNoCheck(t *testing.T)  { checkFormOutlet(t, "tool-no-check") }
func TestFormOutletToolOwner(t *testing.T)    { checkFormOutlet(t, "tool-owner") }
func TestFormOutletFold(t *testing.T)         { checkFormOutlet(t, "fold") }
func TestFormOutletCopyFold(t *testing.T)     { checkFormOutlet(t, "copy-fold") }
func TestFormOutletExportFold(t *testing.T)   { checkFormOutlet(t, "export-fold") }
