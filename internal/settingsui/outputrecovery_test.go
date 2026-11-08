package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func outputRecoveryApp(t *testing.T, dark, open bool) (*App, *Integration, *ui.Tester, *ui.Rect) {
	t.Helper()
	a := copyLifecycleApp(t, dark)
	in := &Integration{ID: "output-reader", Name: "天气查询", Enabled: true, Running: true, RuntimeAvailable: true}
	a.Integrations = func() []Integration {
		rows := []Integration{*in}
		for i := 0; i < 5; i++ {
			rows = append(rows, Integration{ID: fmt.Sprintf("tail%d", i), Name: fmt.Sprintf("后续工具 %d", i+1), Enabled: true, RuntimeAvailable: true})
		}
		return rows
	}
	a.SetRunResult(in.ID, RunResult{Stdout: strings.Repeat("杭州：晴，温度 24°C\n", 60), Success: true})
	a.setRunOutputOpen(in.ID, open)
	viewport := new(ui.Rect)
	tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { *viewport = c.Resolve(a.Body.Handle).Bounds() })
	return a, in, tt, viewport
}

func outputRecoveryDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KF_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kf-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkOutputDisclosureRecovery(t *testing.T, open bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in, tt, viewport := outputRecoveryApp(t, dark, open)
			from, to, name := a.copy().RunViewOutput, a.copy().RunHideOutput, "expand"
			if open {
				from, to, name = to, from, "collapse"
			}
			optionExitFocus(t, tt, from)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			tt.Frame()
			outputRecoveryDump(t, tt, name, dark)
			if a.outputOpen[in.ID] == open {
				t.Fatal("disclosure did not change state")
			}
			runVisible(t, a, tt, to, *viewport)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if a.outputOpen[in.ID] != open {
				t.Fatal("disclosure cannot reverse with keyboard")
			}
		})
	}
}

func TestOutputRecoveryExpand(t *testing.T)   { checkOutputDisclosureRecovery(t, false) }
func TestOutputRecoveryCollapse(t *testing.T) { checkOutputDisclosureRecovery(t, true) }
