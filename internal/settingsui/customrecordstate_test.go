package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/shortcuts"
)

func customStateApp(t *testing.T) (*App, *[]settings.CustomShortcut) {
	t.Helper()
	entries := []settings.CustomShortcut{
		{Key: "Ctrl+Shift+A", Action: "action:settings"},
		{Key: "Ctrl+Shift+B", Action: "say original"},
		{Key: "Ctrl+Shift+C", Action: "plugin:clipboard"},
	}
	a := New(newStore(t), Actions{
		CustomShortcuts: func() []settings.CustomShortcut { return entries },
		SetCustomShortcuts: func(list []settings.CustomShortcut) []CustomShortcutRejection {
			entries = list
			return nil
		},
	})
	return a, &entries
}

func customStateFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() { a.customSection(c, a.copy()) })
	}, 720, 440)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func TestCustomRecorderFollowsRemainingBinding(t *testing.T) {
	a, entries := customStateApp(t)
	a.customRecording, a.customRecordRow = true, 1
	tt := customStateFrame(t, a, true)
	a.removeCustomShortcut(0)
	tt.Frame()
	if !tt.HasText(a.copy().ShortcutRecording) {
		t.Error("remaining binding lost recorder")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyR)
	tt.Frame()
	want, _ := shortcuts.FromKey(ui.Cmd|ui.Shift, ui.KeyR)
	if (*entries)[0].Key != want || (*entries)[0].Action != "say original" || (*entries)[1].Key != "Ctrl+Shift+C" {
		t.Errorf("recording moved to another binding: %+v", *entries)
	}
}

func TestCustomRecorderStopsWhenItsBindingIsRemoved(t *testing.T) {
	a, entries := customStateApp(t)
	a.customRecording, a.customRecordRow = true, 0
	tt := customStateFrame(t, a, true)
	a.removeCustomShortcut(0)
	tt.Frame()
	if a.customRecording || tt.HasText(a.copy().ShortcutRecording) {
		t.Error("removed binding left recorder armed")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyR)
	tt.Frame()
	if (*entries)[0].Key != "Ctrl+Shift+B" || (*entries)[1].Key != "Ctrl+Shift+C" || a.customKey != "" {
		t.Errorf("removed recorder rewrote surviving binding: %+v draft=%q", *entries, a.customKey)
	}
}
