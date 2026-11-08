package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func exportExitApp(t *testing.T, dark bool) *App {
	a := copyLifecycleApp(t, dark)
	tool := callbackTool("weather")
	tool.Mode, tool.ScriptLanguage, tool.ScriptContent = routeScript, "python", "print('sunny')\n"
	a.beginEdit("", tool)
	return a
}

func TestToolExportWaitAndRetry(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := exportExitApp(t, dark)
			var names, contents []string
			var completed []func()
			start := func(name, content string, done func()) {
				names, contents = append(names, name), append(contents, content)
				completed = append(completed, done)
			}
			a.Actions.ExportScript = func(n, s string) { start(n, s, nil) }
			a.Actions.ExportScriptAsync = start
			tt := toolRecoveryFrame(t, a, dark)
			copy := a.copy()
			before := mustFind(t, tt, copy.LocalToolExportScript)
			if err := tt.Click(copy.LocalToolExportScript); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			toolExitDump(t, tt, "export-wait", dark)
			after := mustFind(t, tt, copy.LocalToolExportScript)
			if before != after {
				t.Errorf("waiting export changed geometry: %+v -> %+v", before, after)
			}
			if err := tt.Click(copy.LocalToolExportScript); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(names) != 1 {
				t.Errorf("waiting export dispatched %d save dialogs", len(names))
			}
			// A different form cannot stack another native save dialog.
			a.beginEdit("", CustomTool{Name: "新的工具", Command: "forecast", Mode: routeScript, ScriptLanguage: "python", ScriptContent: "print('new')\n", Platforms: []string{currentPlatform()}})
			tt.Frame()
			if err := tt.Click(copy.LocalToolExportScript); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(names) != 1 {
				t.Errorf("new form bypassed export wait: %d", len(names))
			}
			for _, done := range completed {
				if done != nil {
					done()
				}
			}
			tt.Frame()
			if err := tt.Click(copy.LocalToolExportScript); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if len(names) != 2 || names[len(names)-1] != "forecast-script.py" || contents[len(contents)-1] != "print('new')\n" {
				t.Errorf("export retry lost current snapshot: %q / %q", names, contents)
			}
			if done := completed[len(completed)-1]; done != nil {
				done()
			}
		})
	}
}

func TestToolExportReturnsToScript(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, moved := range []bool{false, true} {
			t.Run(appearanceName(dark)+map[bool]string{false: "/editor", true: "/moved"}[moved], func(t *testing.T) {
				a := exportExitApp(t, dark)
				var done func()
				a.Actions.ExportScript = func(string, string) {}
				a.Actions.ExportScriptAsync = func(_, _ string, finish func()) { done = finish }
				tt := toolRecoveryFrame(t, a, dark)
				copy := a.copy()
				if err := tt.Click(copy.LocalToolExportScript); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if moved {
					localFormFocus(t, tt, copy.LocalToolName)
				}
				if done != nil {
					done()
				}
				tt.Frame()
				toolExitDump(t, tt, "export-return"+map[bool]string{false: "", true: "-moved"}[moved], dark)
				target := copy.LocalToolScript
				if moved {
					target = copy.LocalToolName
				}
				if !tt.Focused(target) {
					t.Errorf("export completion lost current keyboard exit: %s", target)
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("corrected")
				tt.Frame()
				got := a.toolScript
				if moved {
					got = a.toolName
				}
				if got != "corrected" {
					t.Errorf("export return dropped keyboard correction: %q", got)
				}
			})
		}
	}
}
