package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func TestBuiltinRefusalSurvivesNeighborRecording(t *testing.T)  { checkBuiltinRefusalOwner(t, false) }
func TestBuiltinRefusalSurvivesNeighborAcceptance(t *testing.T) { checkBuiltinRefusalOwner(t, true) }

func checkBuiltinRefusalOwner(t *testing.T, accept bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{})
			writes := 0
			a.Actions.SetShortcut = func(id, key string) {
				writes++
				if err := a.Store.Update(func(s *settings.Settings) { s.SetShortcut(id, key) }); err != nil {
					t.Fatal(err)
				}
			}
			tt := shortcutRecoveryFrame(a, dark)
			owner, neighbor := settings.ShortcutNewCommand, settings.ShortcutOpenExternalTerminal
			original := a.binding(owner)
			ownershipClick(t, tt, shortcuts.Display(original))
			mods, key, ok := shortcuts.Parse(a.binding(settings.ShortcutPaste))
			if !ok {
				t.Fatal("paste shortcut unavailable")
			}
			tt.Key(mods, key)
			tt.Frame()
			if !tt.HasText(a.copy().ShortcutRejected) || a.shortcutWarning(owner, a.copy()) == "" {
				t.Fatal("duplicate did not create an owner refusal")
			}
			ownershipClick(t, tt, shortcuts.Display(a.binding(neighbor)))
			name := "builtin-refusal-start"
			if accept {
				name = "builtin-refusal-accepted"
				tt.Key(ui.Ctrl|ui.Alt, ui.KeyY)
				tt.Frame()
				tt.Frame()
				want, _ := shortcuts.FromKey(ui.Ctrl|ui.Alt, ui.KeyY)
				if a.binding(neighbor) != want || writes != 1 {
					t.Fatalf("neighbor did not accept: %s writes=%d", a.binding(neighbor), writes)
				}
			} else if !a.recording || writes != 0 {
				t.Fatal("neighbor did not start recording without writing")
			}
			if tag := os.Getenv("FLOTER_P13JB_DUMP"); tag != "" {
				f, err := os.Create("/tmp/floter-P13-jb/" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
			if !tt.HasText(a.copy().ShortcutRejected) || a.shortcutWarning(owner, a.copy()) != a.copy().ShortcutRejected {
				t.Error("neighbor recording erased the original row refusal")
			}
			if a.binding(owner) != original || a.shortcutWarning(neighbor, a.copy()) != "" {
				t.Fatal("neighbor changed refusal owner or original binding")
			}
			// Retrying the actual owner starts a fresh attempt and clears its warning.
			ownershipClick(t, tt, shortcuts.Display(original))
			if tt.HasText(a.copy().ShortcutRejected) {
				t.Error("owner retry retained its old refusal")
			}
			tt.Key(ui.Ctrl|ui.Alt, ui.KeyU)
			tt.Frame()
			want, _ := shortcuts.FromKey(ui.Ctrl|ui.Alt, ui.KeyU)
			if a.binding(owner) != want {
				t.Error("original row cannot retry")
			}
		})
	}
}
