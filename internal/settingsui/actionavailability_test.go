package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func availabilityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IJ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ij-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkToolActionWithdrawal(t *testing.T, action string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			if action == "script" || action == "export" {
				tool.Mode, tool.ScriptLanguage, tool.ScriptContent = routeScript, "javascript", "console.log('weather');\n"
			}
			a.beginEdit("local.weather", tool)
			copies, exports, picks := 0, 0, 0
			if action == "choose" {
				a.Actions.ChooseProgram = func() (string, error) { picks++; return "/usr/bin/weather", nil }
			}
			a.Actions.CopyText = func(string) { copies++ }
			a.Actions.ExportScript = func(name, content string) {
				exports++
				if name != "local.weather-script.js" || content != tool.ScriptContent {
					t.Errorf("export %q %q", name, content)
				}
			}
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			label, target := copy.LocalToolCopyCommand, copy.LocalToolCommand
			switch action {
			case "plan":
				label, target = copy.LocalToolCopyPlan, copy.LocalToolProgram
			case "script":
				label, target = copy.LocalToolCopyScript, copy.LocalToolScript
			case "export":
				label, target = copy.LocalToolExportScript, copy.LocalToolScript
			case "choose":
				label, target = copy.LocalToolChoose, copy.LocalToolProgram
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			// A real keyboard roundtrip makes the focus ring observable.
			tt.Key(0, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Frame()
			if action == "choose" {
				a.Actions.ChooseProgram = nil
			} else if action == "export" {
				a.Actions.ExportScript = nil
			} else {
				a.Actions.CopyText = nil
			}
			tt.Frame()
			tt.Frame()
			availabilityDump(t, tt, "withdraw-"+action, dark)
			if !tt.Focused(target) {
				t.Errorf("%s withdrawal lost source-field focus: want %s", action, target)
			}
			beforeCopies, beforeExports, beforePicks := copies, exports, picks
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if copies != beforeCopies || exports != beforeExports || picks != beforePicks {
				t.Error("withdrawal replayed an action")
			}
			// Restoring the host must not steal focus or run the action.
			a.Actions.CopyText = func(string) { copies++ }
			a.Actions.ExportScript = func(string, string) { exports++ }
			if action == "choose" {
				a.Actions.ChooseProgram = func() (string, error) { picks++; return "/usr/bin/weather", nil }
			}
			tt.Frame()
			if !tt.Focused(target) {
				t.Error("restored capability stole source focus")
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if action == "choose" {
				if picks != beforePicks+1 || copies != beforeCopies || exports != beforeExports {
					t.Error("program picker did not recover exactly once")
				}
			} else if action == "export" {
				if exports != beforeExports+1 || copies != beforeCopies {
					t.Error("export did not recover exactly once")
				}
			} else if copies != beforeCopies+1 || exports != beforeExports {
				t.Error("copy did not recover exactly once")
			}
		})
	}
}

func TestActionAvailabilityCommandWithdrawal(t *testing.T) { checkToolActionWithdrawal(t, "command") }
func TestActionAvailabilityPlanWithdrawal(t *testing.T)    { checkToolActionWithdrawal(t, "plan") }
func TestActionAvailabilityScriptWithdrawal(t *testing.T)  { checkToolActionWithdrawal(t, "script") }
func TestActionAvailabilityExportWithdrawal(t *testing.T)  { checkToolActionWithdrawal(t, "export") }

func checkLinkActionAvailability(t *testing.T, second, restore bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.Page = PageAbout
			var copied []string
			write := func(value string) { copied = append(copied, value) }
			if !restore {
				a.Actions.CopyText = write
			}
			tt := copyPageFrame(t, a, dark)
			a.FocusSidebar()
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			if second {
				tt.Key(0, ui.KeyTab)
			}
			tt.Frame()
			link, name := deepLinkConnectExample, "connect"
			if second {
				link, name = deepLinkRegisterExample, "register"
			}
			if restore {
				if !tt.Focused(link) {
					t.Fatal("link did not start focused")
				}
				a.Actions.CopyText = write
				name += "-restore"
			} else {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if len(copied) != 1 || copied[0] != link {
					t.Fatalf("initial link copy %q, want %q", copied, link)
				}
				a.Actions.CopyText = nil
				name += "-withdraw"
			}
			tt.Frame()
			tt.Frame()
			availabilityDump(t, tt, name, dark)
			if !tt.Focused(link) {
				t.Errorf("%s lost the current link's keyboard focus", name)
			}
			if restore {
				tt.Key(0, ui.KeyTab)
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if second {
					availabilityDump(t, tt, name+"-navigation", dark)
				}
				if len(copied) != 1 || copied[0] != link {
					t.Errorf("restored link Tab copied %q, want only %q", copied, link)
				}
			} else {
				tt.Command("selectAll")
				tt.Command("copy")
				if tt.Clipboard() != link {
					t.Errorf("fallback copied %q, want %q", tt.Clipboard(), link)
				}
			}
		})
	}
}

func TestActionAvailabilityConnectWithdrawal(t *testing.T) {
	checkLinkActionAvailability(t, false, false)
}
func TestActionAvailabilityRegisterWithdrawal(t *testing.T) {
	checkLinkActionAvailability(t, true, false)
}

func TestActionAvailabilityChooserWithdrawal(t *testing.T) { checkToolActionWithdrawal(t, "choose") }
func TestActionAvailabilityRegisterRestored(t *testing.T)  { checkLinkActionAvailability(t, true, true) }
