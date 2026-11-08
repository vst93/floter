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

func outputRecoveryRead(t *testing.T, a *App, in *Integration, tt *ui.Tester) string {
	t.Helper()
	optionExitFocus(t, tt, a.copy().RunHideOutput)
	a.Body.Y += 180
	tt.Frame()
	result, _ := a.LastRun(in.ID)
	text := result.Text()
	r := mustFind(t, tt, text)
	if r.W == 0 || r.H == 0 {
		t.Fatal("output reader not visible")
	}
	tt.ClickAt(r.X+5, r.Y+5)
	tt.Command("selectAll")
	tt.Command("copy")
	if tt.Clipboard() != text || !tt.Focused(text) {
		t.Fatal("cannot focus and copy existing result")
	}
	return text
}

func checkOutputReaderRetirement(t *testing.T, empty bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in, tt, viewport := outputRecoveryApp(t, dark, true)
			old := outputRecoveryRead(t, a, in, tt)
			name, want := "external-fold", a.copy().RunViewOutput
			if empty {
				name, want = "empty-result", a.copy().RunHideOutput
				a.SetRunResult(in.ID, RunResult{Success: false})
			} else {
				a.setRunOutputOpen(in.ID, false)
			}
			tt.Frame()
			tt.Frame()
			outputRecoveryDump(t, tt, name, dark)
			runVisible(t, a, tt, want, *viewport)
			result, _ := a.LastRun(in.ID)
			if empty && result.Text() != "" || !empty && result.Text() != old {
				t.Fatal("retirement changed stored output")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if a.outputOpen[in.ID] == empty {
				t.Fatal("retired reader lost keyboard disclosure")
			}
		})
	}
}

func TestOutputRecoveryExternalFold(t *testing.T) { checkOutputReaderRetirement(t, false) }
func TestOutputRecoveryEmptyResult(t *testing.T)  { checkOutputReaderRetirement(t, true) }

func checkOutputOwnerRetirement(t *testing.T, removed bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in, tt, _ := outputRecoveryApp(t, dark, true)
			closed := 0
			a.Actions.Close = func() { closed++ }
			tt.Frame()
			old := outputRecoveryRead(t, a, in, tt)
			name := "orphan"
			if removed {
				name = "removed"
				a.Integrations = func() []Integration { return nil }
			} else {
				in.Orphan = true
			}
			tt.Frame()
			tt.Frame()
			outputRecoveryDump(t, tt, name, dark)
			if !tt.Focused(a.copy().Close) {
				t.Fatal("retired output lost page keyboard exit")
			}
			r := mustFind(t, tt, a.copy().Close)
			if r.W <= 0 || r.H <= 0 {
				t.Fatal("page keyboard exit is clipped")
			}
			if tt.HasText(old) {
				t.Fatal("retired output remained visible")
			}
			result, ok := a.LastRun(in.ID)
			if !ok || result.Text() != old {
				t.Fatal("view retirement erased stored result")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if closed != 1 {
				t.Fatal("page keyboard exit cannot close")
			}
		})
	}
}

func TestOutputRecoveryOrphan(t *testing.T)  { checkOutputOwnerRetirement(t, false) }
func TestOutputRecoveryRemoved(t *testing.T) { checkOutputOwnerRetirement(t, true) }
