package settingsui

import (
	"errors"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func contentVisibilityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KD_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kd-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkPickerReturnVisibility(t *testing.T, result string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 5)
			original := a.toolProgram
			calls, warnings := 0, 0
			a.Actions.NotifyWarning = func(string) { warnings++ }
			a.Actions.ChooseProgram = func() (string, error) {
				calls++
				if result == "error" {
					return "", errors.New("picker unavailable")
				}
				if result == "success" {
					return "/usr/bin/forecast", nil
				}
				return "", nil
			}
			var setup func(*ui.Context)
			var target, viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				if setup != nil {
					f := setup
					setup = nil
					f(c)
				}
				target = c.Resolve(a.contentFocus(c, "choose").source).Bounds()
				viewport = c.Resolve(a.Body.Handle).Bounds()
			})
			setup = func(c *ui.Context) { c.Resolve(a.contentFocus(c, "choose").source).Focus().ScrollIntoView() }
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			if !tt.Focused(a.copy().LocalToolChoose) {
				t.Fatal("picker setup did not reach its button")
			}
			// A focused control remains keyboard-operable after the user
			// scrolls to read declarations at the bottom of a long form.
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			if result == "withdraw" {
				a.Actions.ChooseProgram = nil
			} else {
				tt.Key(0, ui.KeyEnter)
			}
			tt.Frame()
			tt.Frame()
			contentVisibilityDump(t, tt, "picker-"+result, dark)
			declarationVisible(t, a, tt, a.copy().LocalToolProgram, target, viewport)
			wantCalls, wantWarnings, wantProgram := 1, 0, original
			if result == "withdraw" {
				wantCalls = 0
			} else if result == "error" {
				wantWarnings = 1
			} else if result == "success" {
				wantProgram = "/usr/bin/forecast"
			}
			if calls != wantCalls || warnings != wantWarnings || a.toolProgram != wantProgram {
				t.Errorf("picker return changed result: calls=%d warnings=%d program=%q", calls, warnings, a.toolProgram)
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/usr/bin/corrected")
			tt.Frame()
			if a.toolProgram != "/usr/bin/corrected" || calls != wantCalls {
				t.Error("picker outlet lost editing or reopened picker")
			}
		})
	}
}

func TestContentVisibilityPickerError(t *testing.T)  { checkPickerReturnVisibility(t, "error") }
func TestContentVisibilityPickerCancel(t *testing.T) { checkPickerReturnVisibility(t, "cancel") }
