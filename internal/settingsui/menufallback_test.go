package settingsui

import (
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
	"testing"
)

func checkMenuFallback(t *testing.T, palette bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{})
			copy := i18n.For("zh").Settings
			current, label, name := "say original", copy.ShortcutsAction, "command-refresh"
			if palette {
				current, label, name = "inherit", copy.Palette, "palette-unknown"
			}
			writes := 0
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				ui.Column(c).Padding(20).Children(func() {
					apply := func(next string) { current = next; writes++ }
					if palette {
						a.paletteRow(c, copy, current, apply)
					} else {
						a.customActionPicker(c, copy, current, apply)
					}
				})
			}, 720, 440)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			optionExitFocus(t, tt, label)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyDown)
			current = "say updated"
			if palette {
				current = "retired-palette"
			}
			want := current
			tt.Frame()
			tt.Frame()
			menuLifetimeDump(t, tt, name, dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if current != want || writes != 0 {
				t.Errorf("%s overwrote refreshed value: %q writes=%d", name, current, writes)
			}
			if !tt.Focused(label) {
				t.Error("refresh lost keyboard outlet")
			}
		})
	}
}

func TestMenuFallbackCommandRefresh(t *testing.T) { checkMenuFallback(t, false) }
func TestMenuFallbackPaletteUnknown(t *testing.T) { checkMenuFallback(t, true) }

func TestMenuBindingRefreshKeepsOwner(t *testing.T) {
	for _, remove := range []bool{false, true} {
		a, entries := customStateApp(t)
		tt := customStateFrame(t, a, true)
		optionExitFocus(t, tt, a.copy().ShortcutsAction)
		tt.Key(0, ui.KeyDown)
		tt.Key(0, ui.KeyHome)
		if remove {
			*entries = (*entries)[1:]
		} else {
			(*entries)[0], (*entries)[2] = (*entries)[2], (*entries)[0]
		}
		tt.Frame()
		tt.Frame()
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if remove && !a.customRecording {
			t.Errorf("removed open binding has no add outlet; rows=%v", *entries)
		}
		if !remove && (*entries)[2].Key != "Ctrl+Shift+A" {
			t.Error("reorder changed original key")
		}
	}
}
