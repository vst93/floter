package settingsui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

func localDraftFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() {
			a.extensionMetrics(func() { a.localToolSection(c, a.copy()) })
		})
	}, 720, 1500)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func localDraftApp() *App {
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(CustomTool) {},
		EditCustomTool:   func(string, CustomTool) {},
		CopyText:         func(string) {},
	})
	a.toolOpen = true
	return a
}

func localDraftDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HB_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hb-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLocalDraftEmptyScriptStaysEmpty(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := localDraftApp()
			a.toolMode, a.toolName, a.toolCommand = routeScript, "Weather", "weather"
			calls := 0
			a.Actions.ConnectLocalTool = func(CustomTool) { calls++ }
			tt := localDraftFrame(t, a, dark)
			copy := a.copy()
			hint := mustFind(t, tt, copy.LocalToolScriptHint)
			tt.ClickAt(hint.X+100, hint.Y+hint.H+15)
			tt.Frame()
			if !tt.Focused(copy.LocalToolScript) {
				t.Fatal("script did not take focus")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Key(0, ui.KeyBackspace)
			tt.Frame()
			tt.Frame()
			localDraftDump(t, tt, "empty-script", dark)
			if a.toolScript != "" {
				t.Errorf("deleted script was repopulated: %d bytes", len(a.toolScript))
			}
			if err := tt.Click(copy.LocalToolConnect); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 0 || !tt.HasText(copy.LocalToolScriptEmpty) {
				t.Errorf("empty script submit: calls=%d error=%v", calls, tt.HasText(copy.LocalToolScriptEmpty))
			}
		})
	}
}

func TestLocalDraftNewToolHasItsOwnUndoHistory(t *testing.T) {
	a := localDraftApp()
	first := CustomTool{Name: "First", Mode: routeScript, ScriptLanguage: "js", ScriptContent: "first_tool_secret\n"}
	a.beginEdit("first", first)
	tt := localDraftFrame(t, a, true)
	a.scriptEditor.state.Buffer.Set("first edited\n")
	tt.Frame()
	a.beginEdit("second", CustomTool{Name: "Second", Mode: routeScript, ScriptLanguage: "js", ScriptContent: "second_tool\n"})
	tt.Frame()
	if a.scriptEditor.state.Buffer.CanUndo() {
		a.scriptEditor.state.Buffer.Undo()
		tt.Frame()
		t.Errorf("new tool inherited undo: %q", a.toolScript)
	}
	if a.toolScript != "second_tool\n" {
		t.Errorf("new tool script=%q", a.toolScript)
	}
	a.scriptEditor.state.Buffer.Set("second edited\n")
	tt.Frame()
	if !a.scriptEditor.state.Buffer.Undo() {
		t.Fatal("current tool lost undo")
	}
	tt.Frame()
	if a.toolScript != "second_tool\n" || strings.Contains(a.toolScript, "first") {
		t.Errorf("undo escaped current tool: %q", a.toolScript)
	}
}

func TestLocalDraftNewConnectionDefaultsToTerminal(t *testing.T) {
	a := localDraftApp()
	a.toolName, a.toolProgram, a.toolCommand = "Weather", "/bin/weather", "weather"
	a.toolOutput = routeBackground
	var got CustomTool
	a.Actions.ConnectLocalTool = func(tool CustomTool) { got = tool }
	tt := localDraftFrame(t, a, true)
	if err := tt.Click(a.copy().LocalToolConnect); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got.Output != routeBackground {
		t.Fatalf("submitted output=%q", got.Output)
	}
	if err := tt.Click(a.copy().LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.toolOutput != "" {
		t.Errorf("fresh connection inherited output %q", a.toolOutput)
	}
}

func TestLocalDraftCopyConfirmationBelongsToTool(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := localDraftApp()
			tool := CustomTool{Name: "Weather", Program: "/bin/weather", Command: "weather"}
			a.beginEdit("first", tool)
			tt := localDraftFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText(copy.LocalToolCommandCopied) {
				t.Fatal("copy did not confirm")
			}
			a.beginEdit("second", tool)
			tt.Frame()
			localDraftDump(t, tt, "new-tool-copy", dark)
			if tt.HasText(copy.LocalToolCommandCopied) {
				t.Error("new tool inherited copied confirmation")
			}
			a.endEdit()
			if a.copiedContent != "" {
				t.Error("completed tool kept copied confirmation")
			}
		})
	}
}

func TestLocalDraftScriptLinesDoNotOverlap(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := localDraftApp()
			a.toolMode, a.toolScript = routeScript, "// 第一行 FIRST\n// 第二行 SECOND\n// 第三行 THIRD\n// 第四行 FOURTH\n"
			tt := localDraftFrame(t, a, dark)
			st := a.scriptEditor.state
			advance := st.LineTop(1) - st.LineTop(0)
			localDraftDump(t, tt, "script-lines", dark)
			if advance < float64(st.Font.Size*1.5) {
				t.Errorf("script line advance %.2f is below readable %.2f", advance, st.Font.Size*1.5)
			}
		})
	}
}

func TestLocalDraftCopyConfirmationMatchesCurrentContent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := localDraftApp()
			a.toolCommand = "weather"
			tt := localDraftFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText(copy.LocalToolCommandCopied) {
				t.Fatal("copy did not confirm")
			}
			a.toolCommand = "forecast"
			tt.Frame()
			localDraftDump(t, tt, "changed-content", dark)
			if tt.HasText(copy.LocalToolCommandCopied) {
				t.Error("changed content still claims copied")
			}
		})
	}
}
