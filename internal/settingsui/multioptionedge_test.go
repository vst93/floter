package settingsui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func checkMultiOptionEdge(t *testing.T, moved bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			options := []string{}
			for i := 0; i < 10; i++ {
				options = append(options, fmt.Sprintf("格式 %d", i+1))
			}
			selected := map[string]bool{}
			list := &ui.ScrollState{}
			rects := map[string]ui.Rect{}
			var inner ui.Rect
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				ui.Column(c).FillWidth().Padding(20).Background(c.Theme().Background).Children(func() {
					a.extensionMetrics(func() {
						a.configMultiselectBox(c, list, false, func() {
							for index, option := range options {
								on, changed := selected[option], false
								_, _, row := a.configOptionRow(c.Key(option), option, &on, &changed, false, index)
								rects[option] = row.Bounds()
								if changed {
									selected[option] = on
								}
							}
						})
					})
				})
				inner = c.Resolve(list.Handle).Bounds()
			}, 360, 150)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			name := "multi-option"
			if moved {
				name = "multi-moved"
			}
			runFieldEdgeDump(t, tt, name+"-rest", dark)
			optionExitFocus(t, tt, "格式 6")
			if moved {
				options = []string{"格式 1", "格式 6", "格式 2", "格式 3", "格式 4", "格式 5", "格式 7", "格式 8", "格式 9", "格式 10"}
				tt.Frame()
				tt.Frame()
			}
			runFieldEdgeDump(t, tt, name, dark)
			if !tt.Focused("格式 6") {
				t.Fatal("option lost focus")
			}
			r := rects["格式 6"]
			t.Logf("row=%+v inner=%+v scroll=%v max=%v", r, inner, list.Y, list.MaxY)
			if list.Y > 0 && r.Y-3 < inner.Y+12 || list.Y < list.MaxY && r.Y+r.H+3 > inner.Y+inner.H-12 {
				t.Errorf("option active ring enters inner fade: row=%+v viewport=%+v scroll=%v", r, inner, list.Y)
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if !selected["格式 6"] || len(selected) != 1 {
				t.Fatalf("option selection changed owner: %v", selected)
			}
			// Scrolling away after entry/reorder is still the user's choice.
			before := list.Y
			tt.Wheel(inner.X+inner.W/2, inner.Y+inner.H/2, 0, 50)
			tt.Frame()
			if list.Y == before {
				t.Fatal("option reveal trapped the wheel")
			}
		})
	}
}

func TestMultiOptionEdgeEntry(t *testing.T)   { checkMultiOptionEdge(t, false) }
func TestMultiOptionEdgeReorder(t *testing.T) { checkMultiOptionEdge(t, true) }
