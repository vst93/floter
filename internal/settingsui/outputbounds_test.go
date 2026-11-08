package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func outputBoundsDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KG_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kg-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkOutputBoundsControl(t *testing.T, open bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, _, tt, _ := outputRecoveryApp(t, dark, false)
			optionExitFocus(t, tt, a.copy().RunViewOutput)
			label, name := a.copy().RunViewOutput, "fold"
			if open {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				tt.Frame()
				label, name = a.copy().RunHideOutput, "open"
			}
			outputBoundsDump(t, tt, name, dark)
			r := mustFind(t, tt, label)
			// Read the native 2px ring, 3px outside the control. A box-only
			// reveal leaves these pixels clipped or under the page edge fade.
			for side, p := range map[string][2]int{
				"top":    {int(r.X + r.W/2), int(r.Y - 2)},
				"bottom": {int(r.X + r.W/2), int(r.Y + r.H + 1)},
				"left":   {int(r.X - 2), int(r.Y + r.H/2)},
				"right":  {int(r.X + r.W + 2), int(r.Y + r.H/2)},
			} {
				red, _, blue := rgbAt(tt.Image(), p[0], p[1])
				if blue-red < 20 {
					t.Errorf("%s ring %s missing at %v: blue-red=%d", name, side, p, blue-red)
				}
			}
		})
	}
}

func TestOutputBoundsFoldRing(t *testing.T) { checkOutputBoundsControl(t, false) }
func TestOutputBoundsOpenRing(t *testing.T) { checkOutputBoundsControl(t, true) }

func TestOutputBoundsNavigationProbe(t *testing.T) {
	a, in, tt, _ := outputRecoveryApp(t, false, true)
	text := outputRecoveryRead(t, a, in, tt)
	t.Logf("start body=%v box=%+v", a.Body.Y, mustFind(t, tt, text))
	tt.Key(ui.Ctrl, ui.KeyHome)
	tt.Frame()
	tt.Frame()
	t.Logf("home body=%v box=%+v", a.Body.Y, mustFind(t, tt, text))
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyEnd)
	tt.Frame()
	tt.Frame()
	outputBoundsDump(t, tt, "end", false)
	t.Logf("end body=%v inner=%+v box=%+v", a.Body.Y, a.runOutputScroll(in.ID), mustFind(t, tt, text))
	tt.Command("copy")
	if tt.Clipboard() != text {
		t.Fatal("navigation changed selection bytes")
	}
}
