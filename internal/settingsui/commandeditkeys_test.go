package settingsui

import (
	"fmt"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestCommandAliasKeepsVerticalArrows(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, key := range []ui.Key{ui.KeyUp, ui.KeyDown} {
			t.Run(appearanceName(dark)+"/"+fmt.Sprint(key), func(t *testing.T) {
				a := noticeApp(t, dark)
				in := commandListIntegration()
				in.Commands = in.Commands[:1]
				a.Integrations = func() []Integration { return []Integration{in} }
				a.Actions.SetCommandAlias = func(string, string, string) {}
				tt := noticeExitPage(t, a, dark)
				label := a.copy().IntegrationsCommandAliasFor("JSON Viewer")
				optionExitFocus(t, tt, label)
				tt.Type("draft")
				tt.Frame()
				tt.Key(0, key)
				tt.Frame()
				commandEditDump(t, tt, "alias-arrow-"+fmt.Sprint(key), dark)
				if a.Page != PageIntegrations || !tt.Focused(label) {
					t.Fatalf("alias arrow left editor: page=%v focused=%v", a.Page, tt.Focused(label))
				}
				if got := refreshRead(tt); got != "draft" {
					t.Errorf("arrow changed alias: %q", got)
				}
			})
		}
	}
}

// The shell registers the chord at the window before drawing Settings. An
// editor must honor the user's binding even when it also edits text with it.
func TestCommandAliasHonorsReboundNewCommand(t *testing.T) {
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
			in := commandListIntegration()
			in.Commands = in.Commands[:1]
			in.Commands[0].Alias = "saved"
			a.Integrations = func() []Integration { return []Integration{in} }
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
				a.View(c)
			}, 720, 900)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			optionExitFocus(t, tt, a.copy().IntegrationsCommandAliasFor("JSON Viewer"))
			tt.Key(ui.Ctrl, ui.KeyBackspace)
			tt.Frame()
			commandEditDump(t, tt, "alias-shortcut", dark)
			if closed != 1 {
				t.Errorf("alias swallowed rebound new-command: closed=%d", closed)
			}
			if len(a.pendingAliases) != 0 {
				t.Fatal("shortcut changed alias")
			}
		})
	}
}
