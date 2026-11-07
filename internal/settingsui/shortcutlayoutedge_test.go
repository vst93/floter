package settingsui

import (
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestShortcutLayoutEdges(t *testing.T) {
	for _, scenario := range []string{"saved-large", "draft-large", "warning", "draft-warning", "recording-large"} {
		for _, dark := range []bool{false, true} {
			t.Run(scenario+"/"+appearanceName(dark), func(t *testing.T) {
				a, entries := customStateApp(t)
				*entries = []settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say original"}}
				a.Store.Update(func(s *settings.Settings) {
					s.Language = "zh"
					if scenario == "saved-large" || scenario == "draft-large" || scenario == "recording-large" {
						s.UIScale = "large"
					}
				})
				a.Page = PageShortcuts
				tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(a.tokens(c).Theme); a.View(c) }, 720, 1000)
				tt.SetDark(dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				tt.Frame()
				tt.Scroll(400, 300, 0, 900)
				tt.Frame()
				copy := a.copy()
				if scenario == "warning" {
					a.CustomUnavailable = []CustomShortcutRejection{{Key: "Ctrl+Shift+B", Reason: CustomShortcutReasonOccupied}}
					tt.Frame()
				}
				if scenario == "draft-warning" || scenario == "draft-large" {
					ownershipClick(t, tt, copy.ShortcutsAdd)
					if scenario == "draft-warning" {
						tt.Key(ui.Ctrl|ui.Shift, ui.KeyB)
					} else {
						tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
					}
					if scenario == "draft-large" {
						ownershipChoose(t, tt, copy.ShortcutsActions[0].Label, true)
						if a.customAction != commandAction {
							t.Fatal("draft did not enter command mode")
						}
					}
				}
				if scenario == "recording-large" {
					ownershipClick(t, tt, copy.ShortcutsAdd)
				}
				ownershipDump(t, tt, scenario, dark)
				for _, label := range []string{copy.ShortcutsAction, copy.ShortcutsCommand, copy.ShortcutsRemove, copy.ShortcutsBind, copy.ShortcutsOccupied, copy.ShortcutsConflict, copy.ShortcutRecording} {
					if r, ok := tt.Find(label); ok {
						if r.X < 170 || r.X+r.W > 704 || r.W < 1 {
							t.Errorf("%s outside content: %+v", label, r)
						}
					}
				}
				if scenario == "saved-large" {
					trash := mustFind(t, tt, copy.ShortcutsRemove)
					picker := mustFind(t, tt, copy.ShortcutsAction)
					if trash.X+trash.W > picker.X {
						t.Errorf("saved picker overlaps remove: trash=%+v picker=%+v", trash, picker)
					}
				}
				if scenario == "draft-large" {
					r := mustFind(t, tt, copy.ShortcutsBind)
					// Hit test the existing key at the draft row, where the picker currently paints over it.
					tt.ClickAt(250, r.Y+r.H/2)
					tt.Frame()
					if !a.customRecording {
						t.Error("draft picker covers recorder hit target")
					}
				}
			})
		}
	}
}
