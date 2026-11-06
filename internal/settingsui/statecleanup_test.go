package settingsui

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/shortcuts"
)

// P13-hk · state that outlives the surface that owns it.
//
// The four surfaces below (the two shortcut recorders, the custom command
// editor, the config list box and the uninstall dialog) all keep a piece of
// editor state on the App so a redraw does not lose it. The bugs are the same
// shape in each: the state has no owner once the control that created it is
// gone, so a later gesture reads an answer the user never gave.

func TestBuiltInRecorderGivesUpAfterFocusMoves(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Shortcut, a.ShortcutID = "Alt+Space", "toggle_window"
	tt := render(t, a, 720, 620)
	if err := tt.Click(a.copy().PageShortcuts); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(shortcuts.Display("Alt+Space")); err != nil {
		t.Fatalf("the recorder button: %v", err)
	}
	tt.Frame()
	if !a.recording {
		t.Fatal("the built-in recorder did not arm")
	}
	// Move the keyboard off the capture pane without answering it.
	if err := tt.Click(a.copy().PageGeneral); err != nil {
		t.Fatalf("the General page: %v", err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.recording {
		t.Error("a bare Escape left the built-in recorder armed after the focus moved")
	}
}

func TestCustomRecorderGivesUpAfterFocusMoves(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 1000)
	if err := tt.Click(a.copy().PageShortcuts); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()
	if err := tt.Click(a.copy().ShortcutsAdd); err != nil {
		t.Fatalf("the custom recorder: %v", err)
	}
	tt.Frame()
	if !a.customRecording {
		t.Fatal("the custom recorder did not arm")
	}
	if err := tt.Click(a.copy().PageGeneral); err != nil {
		t.Fatalf("the General page: %v", err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.customRecording {
		t.Error("a bare Escape left the custom recorder armed after the focus moved")
	}
}

func TestCustomCommandNoOpEnterDropsTheDraft(t *testing.T) {
	a, _ := customStateApp(t)
	tt := customStateFrame(t, a, false)
	copy := a.copy()
	r := mustFind(t, tt, copy.ShortcutsCommand)
	tt.ClickAt(r.X+r.W-8, r.Y+r.H/2)
	tt.Frame()
	if !tt.Focused(copy.ShortcutsCommand) {
		t.Fatal("command did not take focus")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	// The stored action, plus whitespace: the host trims it and the row has
	// nothing new to store.
	tt.Type("say original ")
	tt.Frame()
	if _, ok := a.customEdits["Ctrl+Shift+B"]; !ok {
		t.Fatal("the edit was not drafted")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if draft, ok := a.customEdits["Ctrl+Shift+B"]; ok {
		t.Errorf("a no-op Enter kept the draft %q over the stored action", draft)
	}
}

func TestClosingTheConfigFormForgetsItsListOffsets(t *testing.T) {
	b, _ := configFormApp(t,
		[]ConfigField{{Key: "tags", Label: "Tags", Type: "multiselect", Options: listBoxOptions(12)}},
		map[string]any{})
	tt := integrationPage(t, b, 900, 1400)
	state := b.configListScroll("dev.floter.tool", "tags")
	if state.MaxY <= 0 {
		t.Fatalf("the long option list has nowhere to scroll (max %v)", state.MaxY)
	}
	top, ok := tt.Find("option-a")
	if !ok {
		t.Fatalf("the box's first option is missing: %q", tt.Texts())
	}
	tt.Scroll(top.X+4, top.Y+2, 0, 40)
	tt.Frame()
	if state.Y == 0 {
		t.Fatal("the wheel did not move the list box's options")
	}
	b.closeConfig("dev.floter.tool")
	if _, ok := b.configListScrolls["dev.floter.tool\x00tags"]; ok {
		t.Error("closing the form kept the list box's old offset")
	}
}

func uninstallReopen(t *testing.T, close func(tt *ui.Tester)) *App {
	t.Helper()
	en := i18n.For("en").Settings
	a := uninstallTestApp()
	a.uninstallOpen = map[string]bool{}
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		if a.uninstallOpen["npm"] {
			a.uninstallDialog(c, en, npmTool()).Bind(&ref)
		}
	}, 720, 640)
	tt.Frame()
	a.uninstallOpen["npm"] = true
	tt.Frame()
	tt.Frame()
	if err := tt.Click(en.UninstallHostConfig); err != nil {
		t.Fatalf("untick host config: %v", err)
	}
	tt.Frame()
	if a.uninstallDraft["npm"].RemoveHostConfig {
		t.Fatal("the host-config row did not untick")
	}
	close(tt)
	tt.Frame()
	tt.Frame()
	if a.uninstallOpen["npm"] {
		t.Fatal("the dialog did not close")
	}
	a.uninstallOpen["npm"] = true
	tt.Frame()
	tt.Frame()
	return a
}

func TestUninstallCancelForgetsThePartialSelection(t *testing.T) {
	a := uninstallReopen(t, func(tt *ui.Tester) {
		if err := tt.Click(i18n.For("en").Settings.PermissionsCancel); err != nil {
			t.Fatalf("cancel: %v", err)
		}
	})
	draft := a.uninstallDraft["npm"]
	if !draft.RemoveHostConfig || !draft.RemoveToolData || !draft.RemoveArtifacts {
		t.Errorf("a cancelled dialog reopened with the abandoned ticks: %+v", draft)
	}
}

func TestUninstallEscapeForgetsThePartialSelection(t *testing.T) {
	a := uninstallReopen(t, func(tt *ui.Tester) {
		tt.Key(0, ui.KeyEscape)
	})
	draft := a.uninstallDraft["npm"]
	if !draft.RemoveHostConfig || !draft.RemoveToolData || !draft.RemoveArtifacts {
		t.Errorf("an escaped dialog reopened with the abandoned ticks: %+v", draft)
	}
}

// Probe for the plugin sheet's editor drafts across a close/reopen. The sheet's
// App outlives the overlay (the shell keeps one settings App), so the test
// drives the same App with the sheet drawn and not drawn.
func TestP13HKPluginDraftAcrossClose(t *testing.T) {
	a := New(newStore(t), Actions{})
	schema := &plugincfg.Schema{Plugin: "plug.text", Title: "Text", Fields: []plugincfg.Field{
		{Key: "path", Kind: plugincfg.Text, Label: "Path"},
	}}
	values := plugincfg.Values{"path": "stored"}
	sheet := PluginSheet{Schema: schema, Values: values}
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		if open {
			a.PluginSheet(c, sheet)
		}
	}, 720, 300)
	tt.Frame()
	tt.Frame()
	r := mustFind(t, tt, "Path")
	tt.ClickAt(r.X+40, r.Y+r.H+12)
	tt.Frame()
	if !tt.Focused("Path") {
		t.Fatalf("text input did not take focus: %q", tt.Texts())
	}
	tt.Type("draft")
	tt.Frame()
	key := "plug.text\x00path"
	if _, ok := a.pluginTextDrafts[key]; !ok {
		t.Fatal("the edit was not drafted")
	}
	open = false
	a.ClosePluginSheet()
	tt.Frame()
	tt.Frame()
	open = true
	tt.Frame()
	if draft, ok := a.pluginTextDrafts[key]; ok {
		t.Errorf("reopening the sheet still holds the abandoned draft %q", draft)
	}
}

func TestP13HKPluginNumberDraftAcrossClose(t *testing.T) {
	a := New(newStore(t), Actions{})
	schema := &plugincfg.Schema{Plugin: "plug.num", Title: "Num", Fields: []plugincfg.Field{
		{Key: "size", Kind: plugincfg.Number, Label: "Size", Min: 0, Max: 100, Step: 1, Unit: "items"},
	}}
	values := plugincfg.Values{"size": float64(2)}
	sheet := PluginSheet{Schema: schema, Values: values, Change: func(string, any) {}}
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		if open {
			a.PluginSheet(c, sheet)
		}
	}, 720, 240)
	tt.Frame()
	tt.Frame()
	unit := mustFind(t, tt, "items")
	tt.ClickAt(unit.X-30, unit.Y+unit.H/2)
	tt.Frame()
	if !tt.Focused("Size") {
		t.Fatalf("number input did not take focus: %q", tt.Texts())
	}
	tt.Type("9")
	tt.Frame()
	key := "plug.num\x00size"
	if _, ok := a.pluginNumberDrafts[key]; !ok {
		t.Fatal("the edit was not drafted")
	}
	open = false
	a.ClosePluginSheet()
	tt.Frame()
	tt.Frame()
	open = true
	tt.Frame()
	if draft, ok := a.pluginNumberDrafts[key]; ok {
		t.Errorf("reopening the sheet still holds the abandoned number draft %q", draft)
	}
}

func TestClosingThePluginSheetForgetsItsScroll(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.PluginScroll.Y = 42
	a.sheetPlugin = "plug.text"
	a.ClosePluginSheet()
	if a.PluginScroll.Y != 0 {
		t.Errorf("the closed sheet kept its scroll offset: Y = %v", a.PluginScroll.Y)
	}
	if a.sheetPlugin != "" {
		t.Errorf("the closed sheet kept its plugin id: %q", a.sheetPlugin)
	}
}

// A failed disable leaves its sentence on the band, which is right while the
// question is up. Once the question is answered "cancel", that failure belongs
// to a question that no longer exists: asking again must open on the question,
// not on last time's error.
func TestAFreshDisableQuestionDropsTheOldFailure(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{
		SetIntegrationEnabled: func(id string, enabled bool, done func(error)) {
			if done != nil {
				done(errors.New("disk is full"))
			}
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "v", Name: "V Tools", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Enabled"); err != nil {
		t.Fatalf("the enable switch: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Disable integration"); err != nil {
		t.Fatalf("the confirm: %v", err)
	}
	tt.Frame()
	if !tt.HasText("disk is full") {
		t.Fatalf("the failure is not on the band: %q", tt.Texts())
	}
	if err := tt.Click(en.IntegrationsCancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Enabled"); err != nil {
		t.Fatalf("the switch again: %v", err)
	}
	tt.Frame()
	question := en.DisableTitle("V Tools") + " " + en.DisableDescription
	if !tt.HasText(question) {
		t.Errorf("the fresh question is missing: %q", tt.Texts())
	}
	if tt.HasText("disk is full") {
		t.Errorf("the fresh question still carries the previous failure: %q", tt.Texts())
	}
}

// The removal bar is the same family and the same rule.
func TestAFreshRemovalQuestionDropsTheOldFailure(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{
		UninstallIntegration: func(id, name, kind string, done func(error)) {
			if done != nil {
				done(errors.New("permission denied"))
			}
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "pkg", Name: "V Tools", DistributionSource: "local"}}
	}
	tt := render(t, a, 720, 1200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(en.IntegrationsRemovePackage); err != nil {
		t.Fatalf("the removal button: %v", err)
	}
	tt.Frame()
	cancel := mustFind(t, tt, en.IntegrationsCancel)
	tt.ClickAt(cancel.X+cancel.W+30, cancel.Y+cancel.H/2)
	tt.Frame()
	if !tt.HasText("permission denied") {
		t.Fatalf("the failure is not on the band: %q", tt.Texts())
	}
	if err := tt.Click(en.IntegrationsCancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tt.Frame()
	if err := tt.Click(en.IntegrationsRemovePackage); err != nil {
		t.Fatalf("the removal button again: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Remove V Tools?") {
		t.Errorf("the fresh question is missing: %q", tt.Texts())
	}
	if tt.HasText("permission denied") {
		t.Errorf("the fresh question still carries the previous failure: %q", tt.Texts())
	}
}
