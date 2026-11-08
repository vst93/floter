package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestToolRefusalVisibilityParameter(t *testing.T) {
	for _, field := range []string{"id", "duplicate", "options", "flag", "badflag", "number", "boolean", "select"} {
		for _, dark := range []bool{false, true} {
			t.Run(field+"/"+appearanceName(dark), func(t *testing.T) {
				a := copyLifecycleApp(t, dark)
				a.beginEdit("", callbackTool("weather"))
				bad := toolParam{ID: "second", Label: "需要修正", Kind: extensions.ParamText}
				fixed := "second"
				switch field {
				case "id":
					bad.ID = "bad id"
				case "duplicate":
					bad.ID = "first"
				case "options":
					bad.Kind, fixed = extensions.ParamSelect, "small\nlarge"
				case "flag":
					bad.Kind, fixed = extensions.ParamBoolean, "--enabled"
				case "badflag":
					bad.Flag, fixed = "--bad flag", "--value"
				case "number":
					bad.Kind, bad.Default, fixed = extensions.ParamNumber, "oops", "12"
				case "boolean":
					bad.Kind, bad.Default, bad.Flag, fixed = extensions.ParamBoolean, "maybe", "--enabled", "true"
				case "select":
					bad.Kind, bad.Default, bad.Options, fixed = extensions.ParamSelect, "missing", "small\nlarge", "large"
				}
				a.toolParams = []toolParam{
					{ID: "first", Label: "首项", Kind: extensions.ParamText}, bad,
					{ID: "third", Label: "日期", Kind: extensions.ParamText},
					{ID: "fourth", Label: "单位", Kind: extensions.ParamText},
					{ID: "fifth", Label: "地区", Kind: extensions.ParamText},
				}
				var submitted []CustomTool
				a.Actions.ConnectLocalTool = func(v CustomTool) { submitted = append(submitted, v) }
				var box, viewport ui.Rect
				focused := false
				tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
					if len(a.toolParams) < 2 {
						return
					}
					targets := a.paramRefusalTargets(c, &a.toolParams[1])
					h := targets.id
					switch field {
					case "options":
						h = targets.options
					case "flag", "badflag":
						h = targets.flag
					case "number", "boolean", "select":
						h = targets.defaultValue
					}
					box, viewport = c.Resolve(h).Bounds(), c.Resolve(a.Body.Handle).Bounds()
					focused = h.FocusWithin(c)
				})
				a.Body.Y = a.Body.MaxY
				tt.Frame()
				if err := tt.Click(a.copy().LocalToolConnect); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				toolVisibilityDump(t, tt, "parameter-"+field, dark)
				if len(submitted) != 0 || !focused {
					t.Fatal("parameter refusal lost target")
				}
				if box.H <= 0 || !toolVisiblePixels(box).In(toolVisiblePixels(viewport)) {
					t.Fatalf("parameter %s refusal remains outside viewport: rect=%+v viewport=%+v", field, box, viewport)
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type(fixed)
				tt.Frame()
				a.Body.Y = a.Body.MaxY
				tt.Frame()
				if err := tt.Click(a.copy().LocalToolConnect); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if len(submitted) != 1 || len(submitted[0].Params) != 5 || submitted[0].Params[0].Label != "首项" || submitted[0].Params[4].ID != "fifth" {
					t.Fatal("parameter correction lost declaration ownership or retry")
				}
			})
		}
	}
}

func TestToolAdvancedLabelCollapseKeepsKeyboardOutlet(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.beginEdit("", callbackTool("weather"))
			a.toolAdvanced = true
			tt := toolRecoveryFrame(t, a, dark)
			permission := extensions.AllPermissions[0]
			label := i18n.PermissionLabel("zh", permission)
			optionExitFocus(t, tt, label)
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if !tt.Focused(label) {
				t.Fatal("permission did not take focus")
			}
			selected := a.toolPermissions[permission]
			button := mustFind(t, tt, a.copy().LocalToolAdvanced)
			// The entire existing disclosure row is clickable. Its label is
			// a separate pointer path from the trailing keyboard button.
			tt.ClickAt(45, button.Y+button.H/2)
			tt.Frame()
			toolVisibilityDump(t, tt, "advanced-collapse", dark)
			if a.toolAdvanced {
				t.Fatal("label click did not collapse permissions")
			}
			if !tt.Focused(a.copy().LocalToolAdvanced) {
				t.Fatal("collapsed permission lost keyboard outlet")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if !a.toolAdvanced || a.toolPermissions[permission] != selected {
				t.Fatal("keyboard reopen changed declarations or failed")
			}
		})
	}
}
