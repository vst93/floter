package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func contentRecoveryDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IK_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ik-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkContentRecovery(t *testing.T, action string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			tool.Mode, tool.ScriptLanguage, tool.ScriptContent = routeScript, "javascript", "console.log('weather');\n"
			a.beginEdit("local.weather", tool)
			var copied, exported []string
			a.Actions.CopyText = func(value string) { copied = append(copied, value) }
			a.Actions.ExportScript = func(name, value string) { exported = append(exported, value) }
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			hint := mustFind(t, tt, copy.LocalToolScriptHint)
			tt.ClickAt(hint.X+100, hint.Y+hint.H+15)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(0, ui.KeyBackspace)
			tt.Frame()
			if a.toolScript != "" {
				t.Fatal("script did not clear")
			}
			tt.Type("console.log('restored');")
			tt.Frame()
			if a.toolScript != "console.log('restored');" {
				t.Errorf("empty script lost input: %q", a.toolScript)
			}
			label := copy.LocalToolCopyScript
			if action == "export" {
				label = copy.LocalToolExportScript
			}
			if err := tt.Click(label); err != nil {
				t.Error(err)
			} else {
				tt.Frame()
			}
			contentRecoveryDump(t, tt, "empty-"+action, dark)
			if action == "export" && (len(exported) != 1 || exported[0] != "console.log('restored');") {
				t.Errorf("empty script export=%q", exported)
			}
			if action == "copy" && (len(copied) != 1 || copied[0] != "console.log('restored');") {
				t.Errorf("empty script copy=%q", copied)
			}
		})
	}
}

func TestContentRecoveryEmptyScriptCopy(t *testing.T)   { checkContentRecovery(t, "copy") }
func TestContentRecoveryEmptyScriptExport(t *testing.T) { checkContentRecovery(t, "export") }

func checkPlanRecovery(t *testing.T, restore bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			if restore {
				tool.Command = ""
			}
			a.beginEdit("local.weather", tool)
			var copied []string
			a.Actions.CopyText = func(value string) { copied = append(copied, value) }
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.LocalToolCopyPlan); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			if restore {
				a.toolCommand = "weather"
			} else {
				a.toolCommand = ""
			}
			tt.Frame()
			tt.Frame()
			name := "plan-command-empty"
			if restore {
				name = "plan-command-restored"
			}
			contentRecoveryDump(t, tt, name, dark)
			if !tt.Focused(copy.LocalToolPlanCopied) {
				t.Error("command presence changed plan focus")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			want := executionPlanJSON(tool.Program, tool.Args)
			if len(copied) != 2 || copied[1] != want {
				t.Errorf("plan keyboard copied %q, want two plans", copied)
			}
		})
	}
}

func TestContentRecoveryPlanCommandEmpty(t *testing.T)    { checkPlanRecovery(t, false) }
func TestContentRecoveryPlanCommandRestored(t *testing.T) { checkPlanRecovery(t, true) }
