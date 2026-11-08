package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"testing"
	"time"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func runVisibilityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KE_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ke-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func runVisibilityApp(t *testing.T, dark bool) (*App, *Integration) {
	t.Helper()
	a := noticeApp(t, dark)
	in := &Integration{ID: "weather", Name: "天气查询", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
	for i := 0; i < 12; i++ {
		in.Params = append(in.Params, Param{ID: fmt.Sprintf("p%d", i), Label: fmt.Sprintf("参数 %d", i+1), Kind: extensions.ParamText, Default: "12"})
	}
	a.runOpen = map[string]bool{in.ID: true}
	a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure { t.Error("unexpected run"); return nil }
	a.Integrations = func() []Integration {
		rows := []Integration{*in}
		for i := 0; i < 4; i++ {
			rows = append(rows, Integration{ID: fmt.Sprintf("tail%d", i), Name: fmt.Sprintf("后续工具 %d", i+1), Enabled: true, RuntimeAvailable: true})
		}
		return rows
	}
	return a, in
}

func runVisible(t *testing.T, a *App, tt *ui.Tester, label string, viewport ui.Rect) {
	t.Helper()
	r := mustFind(t, tt, label)
	if !tt.Focused(label) || r.W <= 0 || r.H <= 0 || !toolVisiblePixels(r).In(toolVisiblePixels(viewport)) {
		t.Fatalf("run outlet outside viewport: label=%q focused=%v rect=%+v viewport=%+v scroll=%v", label, tt.Focused(label), r, viewport, a.Body.Y)
	}
}

func runVisibilityWheel(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	before := a.Body.Y
	dy := float32(140)
	if before == a.Body.MaxY {
		dy = -140
	}
	tt.Wheel(600, 280, 0, dy)
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	if a.Body.Y == before {
		t.Fatal("outlet reveal trapped subsequent scrolling")
	}
}

func checkRunFieldVisibility(t *testing.T, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			var viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { viewport = c.Resolve(a.Body.Handle).Bounds() })
			optionExitFocus(t, tt, "参数 1")
			a.Body.Y = 0
			if change == "kind" {
				a.Body.Y = a.Body.MaxY
			}
			tt.Frame()
			want := a.copy().RunCancel
			if change == "kind" {
				in.Params[0].Kind = extensions.ParamNumber
				want = "参数 1"
			} else {
				in.Params = in.Params[1:]
			}
			tt.Frame()
			tt.Frame()
			runVisibilityDump(t, tt, "field-"+change, dark)
			runVisible(t, a, tt, want, viewport)
			if a.runValues[in.ID]["p1"] != "12" {
				t.Fatal("refresh changed surviving answer")
			}
			if change == "kind" {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
				tt.Frame()
				if a.runValues[in.ID]["p0"] != "24" {
					t.Fatal("replacement input cannot edit")
				}
			}
			runVisibilityWheel(t, a, tt)
			if change == "removed" {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if a.runOpen[in.ID] {
					t.Fatal("removed field cannot cancel by keyboard")
				}
			}
		})
	}
}

func TestRunVisibilityRemovedField(t *testing.T) { checkRunFieldVisibility(t, "removed") }
func TestRunVisibilityChangedKind(t *testing.T)  { checkRunFieldVisibility(t, "kind") }
