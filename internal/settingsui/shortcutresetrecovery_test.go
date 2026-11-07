package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func shortcutRecoveryFrame(a *App, dark bool) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		ui.Column(c).Padding(20).Children(func() { a.shortcuts(c, a.copy()) })
	}, 720, 900)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func shortcutRecoveryDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IV_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-iv-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResetSystemRefusalIsVisible(t *testing.T) { checkResetRefusalRecovery(t, "refused") }

func checkResetRefusalRecovery(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{})
			refused, calls := false, 0
			a.Actions.ShortcutRefused = func(id string) bool { return refused && id == settings.ShortcutToggleWindow }
			a.Actions.ResetShortcuts = func() {
				calls++
				if !refused {
					_ = a.Store.Update(func(s *settings.Settings) { s.SetExtra("shortcuts", map[string]any{}) })
				}
			}
			if scenario == "resolved" || scenario == "resolved-focus" {
				a.rejectedShortcut = settings.ShortcutToggleWindow
			} else {
				refused = true
			}
			tt := shortcutRecoveryFrame(a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsReset)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			shortcutRecoveryDump(t, tt, scenario, dark)
			switch scenario {
			case "refused":
				if !tt.HasText(a.copy().ShortcutRejected) || a.shortcutWarning(settings.ShortcutToggleWindow, a.copy()) == "" {
					t.Error("system-refused reset has no visible recovery feedback")
				}
			case "resolved":
				if tt.HasText(a.copy().ShortcutRejected) || a.rejectedShortcut != "" {
					t.Error("successful reset retained obsolete refusal")
				}
			default:
				// Isolate the focus contract even before the feedback wiring lands.
				if refused {
					a.rejectedShortcut = settings.ShortcutToggleWindow
				} else {
					a.rejectedShortcut = ""
				}
				tt.Frame()
				shortcutRecoveryDump(t, tt, scenario, dark)
				if !tt.Focused(a.copy().ShortcutsReset) {
					t.Errorf("%s lost reset keyboard focus", scenario)
				}
				refused = false
				tt.Key(0, ui.KeyEnter)
				if calls != 2 {
					t.Errorf("%s cannot retry from keyboard: calls=%d", scenario, calls)
				}
			}
		})
	}
}
