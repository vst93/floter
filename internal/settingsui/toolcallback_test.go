package settingsui

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func callbackTool(name string) CustomTool {
	return CustomTool{Name: name, Program: "/usr/bin/" + name, Command: name, Version: "1.2.3", Args: "--" + name, Platforms: []string{currentPlatform()}}
}

func toolCallbackFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark); s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() { a.extensionMetrics(func() { a.localToolSection(c, i18n.For("zh").Settings) }) })
	}, 720, 1100)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}

func checkToolPicker(t *testing.T, closed bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{ConnectLocalTool: func(CustomTool) {}, EditCustomTool: func(string, CustomTool) {}})
			a.beginEdit("local.old", callbackTool("oldtool"))
			calls := 0
			a.Actions.ChooseProgram = func() (string, error) {
				calls++
				if closed {
					a.toolOpen = false
					a.endEdit()
				} else {
					a.beginEdit("local.new", callbackTool("newtool"))
				}
				return "/usr/bin/selected-for-old", nil
			}
			tt := toolCallbackFrame(t, a, dark)
			if err := tt.Click(i18n.For("zh").Settings.LocalToolChoose); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			name := "picker-new"
			if closed {
				name = "picker-closed"
			}
			fetchDump(t, tt, name, dark)
			if calls != 1 {
				t.Fatalf("picker calls=%d", calls)
			}
			if closed {
				if a.toolOpen || a.toolEditing != "" || a.toolName != "" || a.toolProgram != "" || a.toolCommand != "" || a.toolArgs != "" {
					t.Errorf("closed picker resurrected draft: open=%v name=%q program=%q command=%q", a.toolOpen, a.toolName, a.toolProgram, a.toolCommand)
				}
				// A hidden stale draft becomes visible on the next disclosure.
				if err := tt.Click(i18n.For("zh").Settings.LocalToolAdd); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				fetchDump(t, tt, name, dark)
				if a.toolName != "" || a.toolProgram != "" {
					t.Error("fresh connection inherited closed picker values")
				}
			} else if !a.toolOpen || a.toolEditing != "local.new" || a.toolName != "newtool" || a.toolProgram != "/usr/bin/newtool" || a.toolCommand != "newtool" || a.toolArgs != "--newtool" {
				t.Errorf("picker overwrote new draft: editing=%q name=%q program=%q args=%q", a.toolEditing, a.toolName, a.toolProgram, a.toolArgs)
			}
			// An ordinary selection still updates the current form once.
			a.Actions.ChooseProgram = func() (string, error) { calls++; return "/usr/bin/current", nil }
			if err := tt.Click(i18n.For("zh").Settings.LocalToolChoose); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if a.toolProgram != "/usr/bin/current" || calls != 2 {
				t.Error("current picker result was not accepted exactly once")
			}
		})
	}
}

func TestToolCallbackPickerKeepsNewDraft(t *testing.T)         { checkToolPicker(t, false) }
func TestToolCallbackPickerKeepsClosedDraftEmpty(t *testing.T) { checkToolPicker(t, true) }

func checkToolSubmission(t *testing.T, editing bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{})
			calls := 0
			replace := func(tool CustomTool) {
				calls++
				if tool.Name != "oldtool" || tool.Program != "/usr/bin/oldtool" {
					t.Errorf("wrong submitted tool: %+v", tool)
				}
				a.beginEdit("local.new", callbackTool("newtool"))
			}
			a.Actions.ConnectLocalTool = replace
			a.Actions.EditCustomTool = func(id string, tool CustomTool) {
				if id != "local.old" {
					t.Errorf("wrong submitted id %q", id)
				}
				replace(tool)
			}
			id, name := "", "connect-new"
			label := i18n.For("zh").Settings.LocalToolConnect
			if editing {
				id, name, label = "local.old", "save-new", i18n.For("zh").Settings.LocalToolSave
			}
			a.beginEdit(id, callbackTool("oldtool"))
			tt := toolCallbackFrame(t, a, dark)
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			fetchDump(t, tt, name, dark)
			if calls != 1 {
				t.Fatalf("submissions=%d, want 1", calls)
			}
			if !a.toolOpen || a.toolEditing != "local.new" || a.toolName != "newtool" || a.toolProgram != "/usr/bin/newtool" || a.toolArgs != "--newtool" {
				t.Errorf("%s completion erased newer editor: open=%v id=%q name=%q program=%q", name, a.toolOpen, a.toolEditing, a.toolName, a.toolProgram)
				return
			}
			if !tt.HasText("local.new") {
				t.Error("new editor identity is not visible")
			}
			a.Actions.EditCustomTool = func(id string, tool CustomTool) {
				calls++
				if id != "local.new" || tool.Name != "newtool" {
					t.Error("new editor submitted old target")
				}
			}
			if err := tt.Click(i18n.For("zh").Settings.LocalToolSave); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 || a.toolOpen || a.toolEditing != "" {
				t.Error("ordinary successful save did not close its own form")
			}
		})
	}
}

func TestToolCallbackSaveKeepsNewEditor(t *testing.T)    { checkToolSubmission(t, true) }
func TestToolCallbackConnectKeepsNewEditor(t *testing.T) { checkToolSubmission(t, false) }
