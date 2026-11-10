package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestParameterOptionsHonorReboundNewCommand(t *testing.T)    { checkEditorBinding(t, false) }
func TestConfigurationInputHonorsReboundNewCommand(t *testing.T) { checkEditorBinding(t, true) }

func checkEditorBinding(t *testing.T, config bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			if err := a.Store.Update(func(s *settings.Settings) {
				if !s.SetShortcut(settings.ShortcutNewCommand, "Ctrl+Backspace") {
					t.Fatal("fixture binding refused")
				}
			}); err != nil {
				t.Fatal(err)
			}
			a.toolParams = []toolParam{{ID: "target", Label: "Target", Kind: extensions.ParamSelect, Options: "files\napps"}}
			in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, Config: []ConfigField{{Key: "token", Label: "Token", Type: "text"}}, ConfigValues: map[string]any{"token": "stored"}}
			a.Integrations = func() []Integration { return []Integration{in} }
			a.configOpen = map[string]bool{in.ID: true}
			closed := 0
			a.Actions.Close = func() { closed++ }
			a.Page, a.entered = PageIntegrations, true
			prepareSettingsDetailFixture(a)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				if c.Shortcut(ui.Ctrl, ui.KeyBackspace) {
					a.Actions.Close()
				}
				if config {
					a.View(c)
				} else {
					ui.Column(c).Padding(20).Children(func() { a.extensionMetrics(func() { a.toolParamsEditor(c, a.copy()) }) })
				}
			}, 720, 900)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			label, name := a.copy().ParamsOptions, "options-shortcut"
			if config {
				label, name = "Token", "config-shortcut"
			}
			optionExitFocus(t, tt, label)
			tt.Key(ui.Ctrl, ui.KeyBackspace)
			tt.Frame()
			commandEditDump(t, tt, name, dark)
			if closed != 1 {
				t.Errorf("%s swallowed rebound new-command: closed=%d", name, closed)
			}
			if a.toolParams[0].Options != "files\napps" || a.draft(in).values["token"] != "stored" {
				t.Fatal("shortcut modified editor draft")
			}
			// Rebinding away restores the standard select/copy/Enter behavior.
			if err := a.Store.Update(func(s *settings.Settings) { s.SetShortcut(settings.ShortcutNewCommand, "Alt+N") }); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			want := "files\napps"
			if config {
				want = "stored"
			}
			if got := refreshRead(tt); got != want {
				t.Errorf("normal selection lost text: %q, want %q", got, want)
			}
			tt.Key(0, ui.KeyEnd)
			if !config {
				tt.Key(0, ui.KeyEnter)
				tt.Type("notes")
				tt.Frame()
				if a.toolParams[0].Options != "files\napps\nnotes" {
					t.Errorf("normal Enter lost multiline editing: %q", a.toolParams[0].Options)
				}
			}
			if closed != 1 {
				t.Fatal("old binding remained active after rebind")
			}
		})
	}
}
