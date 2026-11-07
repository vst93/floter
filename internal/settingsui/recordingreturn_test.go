package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func recordingReturnDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IQ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-iq-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuiltinRecorderCancelReturnsFocus(t *testing.T)  { checkRecordingReturn(t, false, "cancel") }
func TestCustomRecorderCancelReturnsFocus(t *testing.T)   { checkRecordingReturn(t, true, "cancel") }
func TestCustomRecorderAcceptedReturnsFocus(t *testing.T) { checkRecordingReturn(t, true, "accepted") }

func TestBuiltinRecorderOnlyArmsItsOwnRow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, _ := customStateApp(t)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				ui.Column(c).Padding(20).Children(func() {
					a.shortcutRow(c, a.copy())
					a.shortcutMapRow(c, a.copy(), settings.ShortcutNewCommand)
				})
			}, 720, 220)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			tt.Key(0, ui.KeyTab)
			tt.Key(0, ui.KeyEnter)
			if a.ShortcutID != settings.ShortcutNewCommand {
				t.Fatal("wrong recorder armed")
			}
			recordingReturnDump(t, tt, "builtin-owner", dark)
			count := 0
			for _, text := range tt.Texts() {
				if text == a.copy().ShortcutRecording {
					count++
				}
			}
			// Texts includes both the pane's accessible label and its text.
			if count != 2 {
				t.Errorf("one recording painted %d prompt entries, want 2 for one pane", count)
			}
			tt.Key(0, ui.KeyEscape)
			if a.recording {
				t.Error("Escape left recording armed")
			}
		})
	}
}

func checkRecordingReturn(t *testing.T, custom bool, outcome string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			writes := 0
			a.Actions.SetCustomShortcuts = func(list []settings.CustomShortcut) []CustomShortcutRejection { writes++; *entries = list; return nil }
			a.Actions.SetShortcut = func(id, key string) {
				writes++
				if err := a.Store.Update(func(s *settings.Settings) { s.SetShortcut(id, key) }); err != nil {
					t.Fatal(err)
				}
			}
			var tt *ui.Tester
			if custom {
				tt = customStateFrame(t, a, dark)
				optionExitFocus(t, tt, a.copy().ShortcutsCommand)
				for i := 0; i < 3; i++ {
					tt.Key(ui.Shift, ui.KeyTab)
				}
			} else {
				tt = ui.NewTester(func(c *ui.Context) {
					c.SetTheme(a.tokens(c).Theme)
					c.Root().Background(c.Theme().Background)
					ui.Column(c).Padding(20).Children(func() {
						a.shortcutMapRow(c, a.copy(), settings.ShortcutNewCommand)
						ui.Button(c, "Next control")
					})
				}, 720, 220)
				tt.SetDark(dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				tt.Key(0, ui.KeyTab)
			}
			tt.Key(0, ui.KeyEnter)
			if !tt.Focused(a.copy().ShortcutRecording) {
				t.Fatal("recorder not focused")
			}
			if outcome == "accepted" {
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyR)
			} else {
				tt.Key(0, ui.KeyEscape)
			}
			tt.Frame()
			name := "builtin-" + outcome
			if custom {
				name = "custom-" + outcome
			}
			recordingReturnDump(t, tt, name, dark)
			if !tt.Focused(a.copy().ShortcutRecord) {
				t.Errorf("%s lost recorder focus", name)
			}
			wantWrites := 0
			if outcome == "accepted" {
				wantWrites = 1
			}
			if writes != wantWrites {
				t.Fatalf("%s wrote %d bindings, want %d", outcome, writes, wantWrites)
			}
			tt.Key(0, ui.KeyEnter)
			if !tt.Focused(a.copy().ShortcutRecording) {
				t.Error("Enter cannot restart the original recorder")
			}
			if custom && a.customRecordRow != 1 {
				t.Errorf("restarted row %d, want 1", a.customRecordRow)
			}
		})
	}
}
