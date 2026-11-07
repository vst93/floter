package settingsui

import (
	"testing"
	"time"

	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func TestSavedShortcutKeyReadsInFull(t *testing.T) { checkShortcutKeyTip(t, false) }
func TestDraftShortcutKeyReadsInFull(t *testing.T) { checkShortcutKeyTip(t, true) }

func checkShortcutKeyTip(t *testing.T, draft bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			mods, key, ok := shortcuts.Parse("Ctrl+Alt+Shift+Super+PageDown")
			if !ok {
				t.Fatal("invalid long key fixture")
			}
			long, _ := shortcuts.FromKey(mods, key)
			label := shortcuts.Display(long)
			*entries = []settings.CustomShortcut{{Key: long, Action: "say original"}}
			if draft {
				*entries = nil
			}
			if err := a.Store.Update(func(s *settings.Settings) { s.Language = "en"; s.UIScale = "large" }); err != nil {
				t.Fatal(err)
			}
			a.Page = PageShortcuts
			writes := 0
			a.Actions.SetCustomShortcuts = func(list []settings.CustomShortcut) []CustomShortcutRejection { writes++; *entries = list; return nil }
			tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(a.tokens(c).Theme); a.View(c) }, 720, 1100)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			copy := a.copy()
			name := "saved-key-tip"
			if draft {
				name = "draft-key-tip"
				ownershipClick(t, tt, copy.ShortcutsAdd)
				tt.Key(mods, key)
				tt.Frame()
				ownershipChoose(t, tt, copy.ShortcutsAction, true)
				ownershipClick(t, tt, copy.ShortcutsCommand)
				tt.Type("say draft")
				tt.Frame()
			}
			if countOfText(tt, label) != 1 {
				t.Fatal("expected one resting chord label")
			}
			r := mustFind(t, tt, label)
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Advance(700 * time.Millisecond)
			tt.Frame()
			recorderDetailDump(t, tt, name, dark)
			if got := countOfText(tt, label); got != 2 {
				t.Errorf("truncated shortcut has no full hover text: got %d copies, want 2", got)
			}
			if writes != 0 {
				t.Error("reading key wrote a binding")
			}
			if next := mustFind(t, tt, label); next != r {
				t.Errorf("hover changed recorder geometry: %+v -> %+v", r, next)
			}
			tt.Move(0, 0)
			tt.Frame()
			tt.Frame()
			if countOfText(tt, label) != 1 {
				t.Error("shortcut tip remained after pointer left")
			}
			if draft {
				ownershipClick(t, tt, copy.ShortcutsBind)
			} else {
				ownershipClick(t, tt, label)
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyY)
				tt.Frame()
			}
			want := long
			if !draft {
				want, _ = shortcuts.FromKey(ui.Ctrl|ui.Shift, ui.KeyY)
			}
			if len(*entries) != 1 || (*entries)[0].Key != want || writes != 1 {
				t.Error("tooltip blocked the existing bind/record action")
			}
		})
	}
}
