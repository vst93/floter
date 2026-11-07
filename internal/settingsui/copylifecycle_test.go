package settingsui

import (
	"image/png"
	"os"
	"testing"
	"time"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func copyLifecycleDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13II_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ii-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func copyLifecycleApp(t *testing.T, dark bool) *App {
	t.Helper()
	s := settings.Default()
	s.Language, s.Theme = "zh", appearanceName(dark)
	return New(settings.NewStore(s), Actions{ConnectLocalTool: func(CustomTool) {}, EditCustomTool: func(string, CustomTool) {}})
}

func TestCopyLifecycleNewEditorRejectsOldSuccess(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			a.beginEdit("local.first", tool)
			calls := 0
			a.Actions.CopyText = func(value string) {
				calls++
				if value != "weather" {
					t.Errorf("copied %q", value)
				}
				a.beginEdit("local.second", tool)
			}
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			copyLifecycleDump(t, tt, "copy-new-editor", dark)
			if a.toolEditing != "local.second" || !tt.HasText("local.second") || calls != 1 {
				t.Fatal("new editor was not retained")
			}
			if tt.HasText(copy.LocalToolCommandCopied) || a.copiedContent != "" {
				t.Error("old copy returned success into new editor")
			}
			a.Actions.CopyText = func(string) { calls++ }
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 || !tt.HasText(copy.LocalToolCommandCopied) {
				t.Error("fresh copy failed")
			}
			if copiedContentDuration != 1600*time.Millisecond {
				t.Error("copy window changed")
			}
		})
	}
}

func TestCopyLifecycleUnavailableContentIsInert(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.beginEdit("local.weather", callbackTool("weather"))
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			copyLifecycleDump(t, tt, "copy-unavailable", dark)
			if tt.Focused(copy.LocalToolCopyCommand) {
				t.Error("unavailable content copy still accepts focus")
			}
			if a.copiedContent != "" {
				t.Error("unavailable copy confirmed")
			}
			calls := 0
			a.Actions.CopyText = func(value string) {
				calls++
				if value != "weather" {
					t.Errorf("copied %q", value)
				}
			}
			tt.Frame()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 || !tt.HasText(copy.LocalToolCommandCopied) {
				t.Error("restored copy did not dispatch once")
			}
		})
	}
}

func checkCopyMode(t *testing.T, script bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		actions := []string{"command", "plan"}
		if script {
			actions = []string{"script"}
		}
		for _, action := range actions {
			t.Run(appearanceName(dark)+"/"+action, func(t *testing.T) {
				a := copyLifecycleApp(t, dark)
				tool := callbackTool("weather")
				tool.ScriptLanguage, tool.ScriptContent = "js", "console.log('weather');\n"
				if script {
					tool.Mode = routeScript
				}
				a.beginEdit("local.weather", tool)
				calls, exports := 0, 0
				a.Actions.CopyText = func(string) { calls++ }
				a.Actions.ExportScript = func(name, content string) {
					exports++
					if name != "local.weather-script.js" || content != tool.ScriptContent {
						t.Errorf("export changed: %q %q", name, content)
					}
				}
				tt := toolCallbackFrame(t, a, dark)
				copy := a.copy()
				label, done := copy.LocalToolCopyCommand, copy.LocalToolCommandCopied
				away, back := copy.LocalToolModeScript, copy.LocalToolModeProgram
				if action == "plan" {
					label, done = copy.LocalToolCopyPlan, copy.LocalToolPlanCopied
				}
				if script {
					label, done = copy.LocalToolCopyScript, copy.LocalToolScriptCopied
					away, back = back, away
				}
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if !tt.HasText(done) || calls != 1 {
					t.Fatal("initial copy failed")
				}
				if err := tt.Click(away); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if err := tt.Click(back); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				copyLifecycleDump(t, tt, "mode-"+action, dark)
				if tt.HasText(done) || a.copiedContent != "" {
					t.Errorf("%s copy success revived after mode roundtrip", action)
				}
				if calls != 1 || a.toolCommand != tool.Command || a.toolScript != tool.ScriptContent {
					t.Error("mode switch modified content or copied again")
				}
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if calls != 2 || !tt.HasText(done) {
					t.Error("current mode copy failed")
				}
				if script {
					deadline := a.copiedContentUntil
					if err := tt.Click(copy.LocalToolExportScript); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
					if exports != 1 || calls != 2 || a.copiedContentUntil != deadline || !tt.HasText(done) {
						t.Error("export changed the clipboard confirmation")
					}
				}
			})
		}
	}
}

func TestCopyLifecycleProgramModeRetiresSuccess(t *testing.T) { checkCopyMode(t, false) }
func TestCopyLifecycleScriptModeRetiresSuccess(t *testing.T)  { checkCopyMode(t, true) }

func copyPageFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		a.View(c)
	}, 960, 1600)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}

func checkCopyDeparture(t *testing.T, page bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.Page = PageIntegrations
			a.beginEdit("local.weather", callbackTool("weather"))
			calls := 0
			a.Actions.CopyText = func(string) { calls++ }
			var tt *ui.Tester
			if page {
				tt = copyPageFrame(t, a, dark)
			} else {
				tt = toolCallbackFrame(t, a, dark)
			}
			copy := a.copy()
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText(copy.LocalToolCommandCopied) {
				t.Fatal("initial copy failed")
			}
			name := "copy-fold"
			if page {
				name = "copy-page"
				if err := tt.Click(copy.PageIntegrations); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if !tt.HasText(copy.LocalToolCommandCopied) {
					t.Error("reselecting current page retired live success")
				}
				if err := tt.Click(copy.PageGeneral); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if a.Page != PageGeneral {
					t.Fatal("did not leave integrations")
				}
				if err := tt.Click(copy.PageIntegrations); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
			} else {
				if err := tt.Click(copy.LocalToolAdd); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if a.toolOpen {
					t.Fatal("form did not fold")
				}
				if err := tt.Click(copy.LocalToolAdd); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
			}
			copyLifecycleDump(t, tt, name, dark)
			if tt.HasText(copy.LocalToolCommandCopied) || a.copiedContent != "" {
				t.Errorf("%s revived old copy success", name)
			}
			if calls != 1 || !a.toolOpen || a.toolEditing != "local.weather" || a.toolCommand != "weather" || a.toolProgram != "/usr/bin/weather" {
				t.Error("departure changed draft or copied again")
			}
			if err := tt.Click(copy.LocalToolCopyCommand); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 || !tt.HasText(copy.LocalToolCommandCopied) {
				t.Error("returned form cannot copy")
			}
		})
	}
}

func TestCopyLifecycleFoldRetiresSuccess(t *testing.T) { checkCopyDeparture(t, false) }
func TestCopyLifecyclePageRetiresSuccess(t *testing.T) { checkCopyDeparture(t, true) }

func TestCopyLifecycleAboutUnavailableIsInert(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.Page = PageAbout
			tt := copyPageFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.DeepLinkCopy); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			copyLifecycleDump(t, tt, "about-unavailable", dark)
			if tt.Focused(copy.DeepLinkCopy) {
				t.Error("unavailable link copy still accepts focus")
			}
			if tt.HasText(copy.DeepLinkCopied) {
				t.Error("unavailable link copy confirmed")
			}
			// The value remains keyboard-selectable even without the host's
			// one-click action. Tab must have a useful destination off sidebar.
			a.FocusSidebar()
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			if !tt.Focused(deepLinkConnectExample) {
				t.Error("Tab cannot reach the link value")
			}
			tt.Command("selectAll")
			tt.Command("copy")
			if tt.Clipboard() != deepLinkConnectExample {
				t.Errorf("selected link copied %q", tt.Clipboard())
			}
			calls, notices := 0, 0
			a.Actions.CopyText = func(text string) {
				calls++
				if text != deepLinkConnectExample {
					t.Errorf("copied %q", text)
				}
			}
			a.Actions.NotifySuccess = func(text string) {
				notices++
				if text != copy.DeepLinkCopied {
					t.Errorf("notice %q", text)
				}
			}
			tt.Frame()
			if err := tt.Click(copy.DeepLinkCopy); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 || notices != 1 || !tt.HasText(copy.DeepLinkCopied) {
				t.Error("restored link copy did not confirm exactly once")
			}
			a.Actions.CopyText = nil
			tt.Frame()
			if a.copiedLink != "" || tt.HasText(copy.DeepLinkCopied) {
				t.Error("withdrawn clipboard kept link success")
			}
		})
	}
}

func TestCopyLifecycleAboutPageRetiresSuccess(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.Page = PageAbout
			calls, notices := 0, 0
			a.Actions.CopyText = func(string) { calls++ }
			a.Actions.NotifySuccess = func(string) { notices++ }
			tt := copyPageFrame(t, a, dark)
			copy := a.copy()
			if err := tt.Click(copy.DeepLinkCopy); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText(copy.DeepLinkCopied) {
				t.Fatal("initial link copy failed")
			}
			if err := tt.Click(copy.PageAbout); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText(copy.DeepLinkCopied) {
				t.Error("reselecting About retired live success")
			}
			if err := tt.Click(copy.PageGeneral); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if a.Page != PageGeneral {
				t.Fatal("did not leave About")
			}
			if err := tt.Click(copy.PageAbout); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			copyLifecycleDump(t, tt, "about-page", dark)
			if tt.HasText(copy.DeepLinkCopied) || a.copiedLink != "" {
				t.Error("About page revived old copy success")
			}
			if calls != 1 || notices != 1 {
				t.Error("page change repeated copy or notice")
			}
			if err := tt.Click(copy.DeepLinkCopy); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 || notices != 2 || !tt.HasText(copy.DeepLinkCopied) {
				t.Error("returned About cannot copy")
			}
			if linkCopiedDuration != 1600*time.Millisecond {
				t.Error("link copy window changed")
			}
		})
	}
}
