package settingsui

import (
	"testing"

	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func TestCustomRekeyKeepsUnsubmittedCommand(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[1:2]
			tt := customStateFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("say unfinished")
			ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+B"))
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			ownershipDump(t, tt, "rekey-command", dark)
			if got := refreshRead(tt); got != "say unfinished" {
				t.Errorf("changing key discarded command draft: %q", got)
			}
			if (*entries)[0].Action != "say original" {
				t.Fatal("rekey submitted draft")
			}
			tt.Key(0, ui.KeyEnter)
			if (*entries)[0].Action != "say unfinished" {
				t.Error("new key cannot submit retained command")
			}
		})
	}
}

func TestCustomCommandRefreshUpdatesUntouchedEditor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[1:2]
			tt := customStateFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			(*entries)[0].Action = "say refreshed"
			tt.Frame()
			ownershipDump(t, tt, "refresh-command", dark)
			if got := refreshRead(tt); got != "say refreshed" {
				t.Errorf("untouched focused command stale: %q", got)
			}
			tt.Key(0, ui.KeyEnter)
			if (*entries)[0].Action != "say refreshed" {
				t.Error("stale editor overwrote refreshed command")
			}
		})
	}
}

func TestCustomCommandRowsStayInsideSettings(t *testing.T) {
	for _, scenario := range []string{"saved", "draft", "long-key", "long-action"} {
		t.Run(scenario, func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = []settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say original"}}
			if scenario == "long-key" {
				(*entries)[0].Key = "Ctrl+Alt+Shift+Super+PageDown"
			}
			if scenario == "long-action" {
				(*entries)[0].Action = "action:open_external_terminal"
			}
			a.Page = PageShortcuts
			tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(a.tokens(c).Theme); a.View(c) }, 720, 1000)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			tt.Scroll(400, 300, 0, 900)
			tt.Frame()
			if scenario == "draft" {
				ownershipClick(t, tt, a.copy().ShortcutsAdd)
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
				// Saved command and draft picker have duplicate labels; use draft row coordinates.
				r, _ := tt.Find(a.copy().ShortcutsBind)
				p, _ := tt.Find(a.copy().ShortcutsAction)
				tt.ClickAt(p.X+p.W/2, r.Y+r.H/2)
				tt.Frame()
				tt.Key(0, ui.KeyEnd)
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
			}
			ownershipDump(t, tt, "bounds-"+scenario, false)
			for _, label := range []string{a.copy().ShortcutsAction, a.copy().ShortcutsCommand, a.copy().ShortcutsRemove, a.copy().ShortcutsBind} {
				if r, ok := tt.Find(label); ok {
					t.Logf("%s: %+v", label, r)
					if r.X < 0 || r.X+r.W > 720 {
						t.Errorf("%s outside window: %+v", label, r)
					}
				}
			}
			if scenario == "long-key" {
				key := mustFind(t, tt, shortcuts.Display((*entries)[0].Key))
				trash := mustFind(t, tt, a.copy().ShortcutsRemove)
				if key.X+key.W > trash.X {
					t.Errorf("key text overlaps remove: key=%+v trash=%+v", key, trash)
				}
			}
		})
	}
}
