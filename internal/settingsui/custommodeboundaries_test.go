package settingsui

import (
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestCustomCommandReselectKeepsSavedBinding(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[1:2]
			tt := customStateFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("say unfinished")
			ownershipChoose(t, tt, a.copy().ShortcutsAction, true)
			ownershipDump(t, tt, "command-reselect", dark)
			if len(*entries) != 1 || (*entries)[0].Action != "say original" || a.customDraftOpen {
				t.Errorf("reselecting command mode removed binding: %v draft=%v", *entries, a.customDraftOpen)
				return
			}
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			if got := refreshRead(tt); got != "say unfinished" {
				t.Errorf("mode reselect lost edit: %q", got)
			}
		})
	}
}

func TestCustomDraftActionChangeKeepsRecorderFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = nil
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			ownershipClick(t, tt, copy.ShortcutsAdd)
			ownershipChoose(t, tt, copy.ShortcutsAction, true)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			ownershipDump(t, tt, "draft-mode-recording", dark)
			if a.customKey != "Ctrl+Shift+Z" {
				t.Errorf("mode switch lost active capture: key=%q", a.customKey)
			}
		})
	}
}

func TestCustomLongActionKeepsRecorderReachable(t *testing.T) {
	for _, scale := range []string{"compact", "default", "large"} {
		t.Run(scale, func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = []settings.CustomShortcut{{Key: "Ctrl+Alt+Shift+PageDown", Action: "action:open_external_terminal"}}
			a.Store.Update(func(s *settings.Settings) { s.UIScale = scale })
			a.Page = PageShortcuts
			tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(a.tokens(c).Theme); a.View(c) }, 720, 1000)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			tt.Scroll(400, 300, 0, 900)
			tt.Frame()
			ownershipDump(t, tt, "long-action-"+scale, false)
			p := mustFind(t, tt, a.copy().ShortcutsAction)
			trash := mustFind(t, tt, a.copy().ShortcutsRemove)
			if trash.X+trash.W > p.X {
				t.Errorf("long action covers trash: trash=%+v picker=%+v", trash, p)
			}
		})
	}
}
