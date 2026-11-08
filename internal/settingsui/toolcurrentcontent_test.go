package settingsui

import "testing"

func TestToolCurrentCopyDraft(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, action := range []string{"command", "plan", "script"} {
			t.Run(appearanceName(dark)+"/"+action, func(t *testing.T) {
				a := copyLifecycleApp(t, dark)
				tool := callbackTool("weather")
				if action == "script" {
					tool.Mode, tool.ScriptLanguage, tool.ScriptContent = routeScript, "python", "print('old')\n"
				}
				a.beginEdit("", tool)
				copy := a.copy()
				label, done := copy.LocalToolCopyCommand, copy.LocalToolCommandCopied
				if action == "plan" {
					label, done = copy.LocalToolCopyPlan, copy.LocalToolPlanCopied
				} else if action == "script" {
					label, done = copy.LocalToolCopyScript, copy.LocalToolScriptCopied
				}
				var copied []string
				a.Actions.CopyText = func(value string) {
					copied = append(copied, value)
					a.toolName, a.toolCommand, a.toolProgram, a.toolArgs = "新的天气工具", "forecast", "/usr/bin/forecast", "--today"
					if action == "script" {
						a.toolLanguage, a.toolScript = "python", "print('new')\n"
					}
				}
				tt := toolRecoveryFrame(t, a, dark)
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				toolCurrentDump(t, tt, "copy-"+action, dark)
				if len(copied) != 1 || a.toolName != "新的天气工具" || a.toolCommand != "forecast" || a.toolProgram != "/usr/bin/forecast" || a.toolArgs != "--today" || (action == "script" && a.toolScript != "print('new')\n") {
					t.Errorf("%s copy rewound newer draft: name=%q command=%q script=%q", action, a.toolName, a.toolCommand, a.toolScript)
				}
				if a.copiedContent != "" || tt.HasText(done) {
					t.Errorf("%s copy confirmed stale bytes", action)
				}
				a.Actions.CopyText = func(value string) { copied = append(copied, value) }
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				want := "forecast"
				if action == "plan" {
					want = executionPlanJSON("/usr/bin/forecast", "--today")
				} else if action == "script" {
					want = "print('new')\n"
				}
				if len(copied) != 2 || copied[1] != want || !tt.HasText(done) {
					t.Errorf("fresh %s copy failed: values=%q", action, copied)
				}
			})
		}
	}
}

func TestToolCurrentExportDraft(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			tool.Mode, tool.ScriptLanguage, tool.ScriptContent = routeScript, "python", "print('old')\n"
			a.beginEdit("", tool)
			var names, contents []string
			a.Actions.ExportScript = func(name, content string) {
				names, contents = append(names, name), append(contents, content)
				a.toolCommand, a.toolScript = "forecast", "print('new')\n"
			}
			tt := toolRecoveryFrame(t, a, dark)
			if err := tt.Click(a.copy().LocalToolExportScript); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			toolCurrentDump(t, tt, "export-draft", dark)
			if a.toolCommand != "forecast" || a.toolScript != "print('new')\n" {
				t.Errorf("export rewound newer draft: command=%q script=%q", a.toolCommand, a.toolScript)
			}
			if err := tt.Click(a.copy().LocalToolExportScript); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(names) != 2 || names[0] != "weather-script.py" || contents[0] != "print('old')\n" || names[1] != "forecast-script.py" || contents[1] != "print('new')\n" {
				t.Errorf("exports lost click snapshots: names=%q contents=%q", names, contents)
			}
		})
	}
}
