package settingsui

import (
	"testing"
	"time"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func TestRunFormBooleanLocksBothEntrances(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, entrance := range []string{"switch", "label"} {
			t.Run(appearanceName(dark)+"/"+entrance, func(t *testing.T) {
				a, integration := runStateApp(Param{ID: "verbose", Label: "Verbose", Kind: extensions.ParamBoolean})
				a.repairBusy = map[string]bool{"runner": true}
				tt := runStateFrame(t, a, integration, dark)
				label := mustFind(t, tt, "Verbose")
				x, y := float32(660), label.Y+label.H/2
				if entrance == "label" {
					x = label.X + label.W/2
				}
				face := imagePixel(tt.Image(), 300, int(y))
				tt.ClickAt(x, y)
				tt.Advance(200 * time.Millisecond)
				tt.Frame()
				if hover := imagePixel(tt.Image(), 300, int(y)); hover != face {
					t.Errorf("locked boolean row lifted: %v -> %v", face, hover)
				}
				runStateDump(t, tt, "boolean-"+entrance, dark)
				if truthyValue(a.runValues["runner"]["verbose"]) {
					t.Errorf("busy boolean changed through %s", entrance)
				}
				delete(a.repairBusy, "runner")
				tt.Frame()
				tt.ClickAt(x, y)
				tt.Frame()
				if !truthyValue(a.runValues["runner"]["verbose"]) {
					t.Errorf("unlocked boolean did not change through %s", entrance)
				}
			})
		}
	}
}

func TestRunFormSelectHonorsRowLock(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, integration := runStateApp(Param{ID: "mode", Label: "Mode", Kind: extensions.ParamSelect, Default: "fast", Options: []string{"fast", "careful"}})
			a.repairBusy = map[string]bool{"runner": true}
			tt := runStateFrame(t, a, integration, dark)
			label := mustFind(t, tt, "Mode")
			face := imagePixel(tt.Image(), 300, int(label.Y+label.H/2))
			if err := tt.Click("careful"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyLeft)
			tt.Key(0, ui.KeyRight)
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			runStateDump(t, tt, "select", dark)
			if got := a.runValues["runner"]["mode"]; got != "fast" {
				t.Errorf("busy select changed to %q", got)
			}
			if hover := imagePixel(tt.Image(), 300, int(label.Y+label.H/2)); hover != face {
				t.Errorf("locked select row lifted: %v -> %v", face, hover)
			}
			delete(a.repairBusy, "runner")
			tt.Frame()
			if err := tt.Click("careful"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := a.runValues["runner"]["mode"]; got != "careful" {
				t.Errorf("unlocked select=%q, want careful", got)
			}
		})
	}
}
