package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func checkExportVisibility(t *testing.T, variants ...string) {
	t.Helper()
	for _, variant := range variants {
		for _, dark := range []bool{false, true} {
			t.Run(variant+"/"+appearanceName(dark), func(t *testing.T) {
				a := declarationVisibilityApp(t, dark, 5)
				a.toolMode, a.toolLanguage, a.toolScript = routeScript, "python", "print('sunny')\n"
				calls := 0
				var finish func()
				export := func(name, content string) {
					calls++
					if name != "local.weather-script.py" || content != "print('sunny')\n" {
						t.Errorf("export lost original bytes: %q / %q", name, content)
					}
				}
				a.Actions.ExportScript = export
				if variant == "async" {
					a.Actions.ExportScriptAsync = func(name, content string, done func()) {
						export(name, content)
						finish = done
					}
				}
				var setup func(*ui.Context)
				var target, viewport ui.Rect
				tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
					if setup != nil {
						f := setup
						setup = nil
						f(c)
					}
					target = c.Resolve(a.contentFocus(c, "export").source).Bounds()
					viewport = c.Resolve(a.Body.Handle).Bounds()
				})
				setup = func(c *ui.Context) { c.Resolve(a.contentFocus(c, "export").button).Focus().ScrollIntoView() }
				tt.Frame()
				tt.Key(0, ui.KeyTab)
				tt.Key(ui.Shift, ui.KeyTab)
				if !tt.Focused(a.copy().LocalToolExportScript) {
					t.Fatal("export setup did not reach its button")
				}
				a.Body.Y = a.Body.MaxY
				tt.Frame()
				wantCalls, wantScript := 0, a.toolScript
				switch variant {
				case "withdraw":
					a.Actions.ExportScript = nil
				case "blank":
					a.toolScript, wantScript = "  ", "  "
				default:
					wantCalls = 1
					tt.Key(0, ui.KeyEnter)
				}
				tt.Frame()
				tt.Frame()
				contentVisibilityDump(t, tt, "export-"+variant, dark)
				declarationVisible(t, a, tt, a.copy().LocalToolScript, target, viewport)
				if calls != wantCalls || a.toolScript != wantScript {
					t.Errorf("export exit changed draft/dispatch: calls=%d script=%q", calls, a.toolScript)
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("print('corrected')")
				tt.Frame()
				if a.toolScript != "print('corrected')" || calls != wantCalls {
					t.Error("export outlet lost editing or dispatched again")
				}
				if variant == "async" {
					if finish == nil || a.toolExportDone == nil {
						t.Fatal("export did not keep pending lifecycle")
					}
					finish()
					tt.Frame()
					if a.toolExportDone != nil || !tt.Focused(a.copy().LocalToolScript) || a.toolScript != "print('corrected')" {
						t.Error("export completion changed current editing")
					}
				}
			})
		}
	}
}

func TestContentVisibilityExportRetired(t *testing.T) { checkExportVisibility(t, "withdraw", "blank") }
func TestContentVisibilityExportReturn(t *testing.T)  { checkExportVisibility(t, "sync", "async") }
