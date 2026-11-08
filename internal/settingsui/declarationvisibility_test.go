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

func declarationVisibilityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KC_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kc-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func declarationVisibilityApp(t *testing.T, dark bool, count int) *App {
	t.Helper()
	a := copyLifecycleApp(t, dark)
	a.beginEdit("local.weather", callbackTool("weather"))
	for i := 0; i < count; i++ {
		a.toolParams = append(a.toolParams, toolParam{ID: fmt.Sprintf("field%d", i), Label: fmt.Sprintf("字段%d", i), Kind: extensions.ParamText})
	}
	return a
}

func declarationVisible(t *testing.T, a *App, tt *ui.Tester, label string, rect, viewport ui.Rect) {
	t.Helper()
	if !tt.Focused(label) {
		t.Errorf("editing outlet lost focus: %s", label)
	}
	if rect.W <= 0 || rect.H <= 0 || !toolVisiblePixels(rect).In(toolVisiblePixels(viewport)) {
		t.Errorf("editing outlet outside viewport: rect=%+v viewport=%+v scroll=%v", rect, viewport, a.Body.Y)
	}
	before := a.Body.Y
	tt.Wheel(710, 280, 0, 140)
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	if before < a.Body.MaxY && a.Body.Y <= before {
		t.Error("outlet reveal trapped wheel scrolling")
	}
}

func checkDeclarationListVisibility(t *testing.T, action string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			count := 5
			if action == "only" {
				count = 1
			}
			a := declarationVisibilityApp(t, dark, count)
			copy := a.copy()
			// Make the departing tail's options long enough that its label and
			// the preceding record are genuinely above the removal viewport.
			if action != "add" {
				for i := range a.toolParams {
					a.toolParams[i].Kind = extensions.ParamSelect
					a.toolParams[i].Options = "北京\n上海\n广州\n深圳\n杭州\n南京\n成都\n重庆\n西安\n武汉\n长沙\n苏州\n天津\n青岛\n济南\n厦门\n福州\n郑州\n合肥\n南昌"
				}
			}
			var viewport, target ui.Rect
			var setup func(*ui.Context)
			probeIndex := 0
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				if setup != nil {
					f := setup
					setup = nil
					f(c)
				}
				viewport = c.Resolve(a.Body.Handle).Bounds()
				// ID shares a row with the label; both must be fully visible.
				if probeIndex >= 0 && probeIndex < len(a.toolParams) {
					target = c.Resolve(a.paramRefusalTargets(c, &a.toolParams[probeIndex]).id).Bounds()
				}
			})
			focusID := func(index int) {
				setup = func(c *ui.Context) {
					c.Resolve(a.paramRefusalTargets(c, &a.toolParams[index]).id).Focus().ScrollIntoView()
				}
				tt.Frame()
			}
			focusID(0)
			if action == "add" {
				// The heading precedes the first label and ID in the tab order.
				tt.Key(ui.Shift, ui.KeyTab)
				tt.Key(ui.Shift, ui.KeyTab)
				if !tt.Focused(copy.ParamsAdd) {
					t.Fatal("setup did not reach Add")
				}
				tt.Key(0, ui.KeyEnter)
				probeIndex = count
			} else if action == "withdraw" {
				focusID(3)
				a.toolParams = append(a.toolParams[:3], a.toolParams[4:]...)
			} else {
				remove := 1
				if action == "tail" || action == "only" {
					remove = count - 1
				}
				focusID(remove)
				for n := 0; n < 10 && !tt.Focused(copy.ParamsRemove); n++ {
					tt.Key(0, ui.KeyTab)
				}
				if !tt.Focused(copy.ParamsRemove) {
					t.Fatal("setup did not reach Remove")
				}
				if action == "next" {
					a.Body.Y = a.Body.MaxY
					tt.Frame()
				}
				tt.Key(0, ui.KeyEnter)
				probeIndex = remove
				if probeIndex >= len(a.toolParams) {
					probeIndex = len(a.toolParams) - 1
				}
			}
			tt.Frame()
			tt.Frame()
			declarationVisibilityDump(t, tt, "list-"+action, dark)
			if action == "only" || action == "withdraw" {
				r, ok := tt.Find(copy.ParamsAdd)
				if !ok {
					t.Fatal("Add disappeared")
				}
				declarationVisible(t, a, tt, copy.ParamsAdd, r, viewport)
				before := len(a.toolParams)
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if len(a.toolParams) != before+1 {
					t.Error("Add outlet cannot continue with Enter")
				}
			} else {
				declarationVisible(t, a, tt, copy.ParamsLabel, target, viewport)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("继续编辑")
				tt.Frame()
				if a.toolParams[probeIndex].Label != "继续编辑" {
					t.Errorf("wrong declaration receives edit: %+v", a.toolParams)
				}
			}
		})
	}
}

func TestDeclarationVisibilityAdd(t *testing.T) { checkDeclarationListVisibility(t, "add") }
