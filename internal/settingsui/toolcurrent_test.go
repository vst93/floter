package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func toolCurrentDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JZ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jz-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestToolCurrentPickerDraft(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, change := range []string{"mode", "fields", "clear"} {
			t.Run(appearanceName(dark)+"/"+change, func(t *testing.T) {
				a := copyLifecycleApp(t, dark)
				a.beginEdit("", callbackTool("weather"))
				calls := 0
				a.Actions.ChooseProgram = func() (string, error) {
					calls++
					if change == "mode" {
						a.toolMode, a.toolLanguage, a.toolScript = routeScript, "python", "print('new draft')\n"
					} else if change == "fields" {
						a.toolName, a.toolProgram, a.toolCommand = "新的天气工具", "/usr/bin/forecast", "forecast"
						a.toolArgs, a.toolVersionArgs, a.toolVersion, a.toolDescription = "--today", "-V", "2.0.0", "新的说明"
					} else {
						a.toolProgram = ""
					}
					return "/usr/bin/old-selection", nil
				}
				tt := toolRecoveryFrame(t, a, dark)
				if err := tt.Click(a.copy().LocalToolChoose); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				toolCurrentDump(t, tt, "picker-"+change, dark)
				if calls != 1 {
					t.Fatalf("picker calls=%d", calls)
				}
				switch change {
				case "mode":
					if a.toolMode != routeScript || a.toolLanguage != "python" || a.toolScript != "print('new draft')\n" || a.toolProgram != "/usr/bin/weather" {
						t.Errorf("picker rewound newer mode: mode=%q script=%q program=%q", a.toolMode, a.toolScript, a.toolProgram)
					}
				case "fields":
					if a.toolName != "新的天气工具" || a.toolProgram != "/usr/bin/forecast" || a.toolCommand != "forecast" || a.toolArgs != "--today" || a.toolVersionArgs != "-V" || a.toolVersion != "2.0.0" || a.toolDescription != "新的说明" {
						t.Errorf("picker rewound newer fields: name=%q program=%q command=%q args=%q", a.toolName, a.toolProgram, a.toolCommand, a.toolArgs)
					}
				case "clear":
					if a.toolProgram != "" {
						t.Errorf("picker refilled cleared path: %q", a.toolProgram)
					}
				}
			})
		}
	}
}

func TestToolCurrentPickerErrorKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.beginEdit("", callbackTool("weather"))
			warnings := 0
			a.Actions.NotifyWarning = func(string) { warnings++ }
			a.Actions.ChooseProgram = func() (string, error) { return "", os.ErrPermission }
			tt := toolRecoveryFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().LocalToolChoose)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			tt.Frame()
			toolCurrentDump(t, tt, "picker-error", dark)
			if warnings != 1 || !tt.Focused(a.copy().LocalToolProgram) {
				t.Errorf("picker error lost editable exit: warnings=%d programFocus=%v", warnings, tt.Focused(a.copy().LocalToolProgram))
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/usr/bin/retry")
			tt.Frame()
			if a.toolProgram != "/usr/bin/retry" {
				t.Errorf("picker error dropped keyboard correction: %q", a.toolProgram)
			}
		})
	}
}
