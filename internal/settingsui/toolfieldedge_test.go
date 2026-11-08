package settingsui

import (
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func checkToolFieldEdge(t *testing.T, name string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 5)
			if name == "language" {
				a.toolMode, a.toolLanguage, a.toolScript = routeScript, "python", "print('sunny')"
			}
			calls := 0
			a.Actions.ChooseProgram = func() (string, error) { calls++; return "", nil }
			a.Actions.EditCustomTool = func(string, CustomTool) { calls++ }
			copy := a.copy()
			label := map[string]string{"name": copy.LocalToolName, "program": copy.LocalToolProgram,
				"args": copy.LocalToolArgs, "versionargs": copy.LocalToolVersionArgs, "description": copy.LocalToolDescription,
				"mode": copy.LocalToolModeProgram, "choose": copy.LocalToolChoose, "language": copy.LocalToolLanguage}[name]
			var target, viewport ui.Rect
			focused := false
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				key := map[string]string{"name": "name", "program": "plan", "script": "script", "mode": "mode-program"}[name]
				h := a.contentFocus(c, key).source
				if name == "args" || name == "versionargs" || name == "description" {
					h = a.contentFocus(c, "mode-program").button
				}
				if name == "choose" {
					h = a.contentFocus(c, "choose").button
				}
				if name == "language" {
					h = a.contentFocus(c, "mode-script").button
				}
				focused = h.FocusWithin(c) && c.Resolve(h).Bounds().W > 0
				target, viewport = c.Resolve(h).Bounds(), c.Resolve(a.Body.Handle).Bounds()
			})
			for i := 0; i < 100 && !(tt.Focused(label) && focused); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) || !focused || target.W <= 0 || target.H <= 0 {
				t.Fatalf("cannot reach %s: %+v", name, target)
			}
			// Return through a real Tab event after the previous field has
			// scrolled above the visible body, rather than directly focusing it.
			absoluteY := target.Y + a.Body.Y
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			a.Body.Y = absoluteY - viewport.Y + 8
			tt.Frame()
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Frame()
			if !tt.Focused(label) || !focused {
				t.Fatal("return lost input focus")
			}
			if width := map[string]float32{"name": 240, "program": 240, "args": 220, "versionargs": 160, "description": 240, "mode": 92, "choose": 55, "language": 220}[name]; target.W != width {
				t.Fatalf("%s width changed: got %v want %v", name, target.W, width)
			}
			if name == "mode" {
				// The group has 3px padding; focus ink belongs to its first
				// 30px segment, not the 36px group or the gap between buttons.
				target.X += 3
				target.Y += 3
				target.W = (target.W - 6) / 2
				target.H -= 6
			}
			if tag := os.Getenv("FLOTER_P13KJ_DUMP"); tag != "" {
				f, err := os.Create("/tmp/floter-P13-kj-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("%s rect=%+v viewport=%+v scroll=%v", name, target, viewport, a.Body.Y)
			inset := float32(0)
			if name == "mode" || name == "choose" || name == "language" {
				inset = -2
			}
			for side, y := range map[string]float32{"top": target.Y + inset, "bottom": target.Y + target.H - 1 - inset} {
				x, iy := int(target.X+target.W/2), int(math.Round(float64(y)))
				r, _, b := rgbAt(tt.Image(), x, iy)
				if b-r < 60 {
					t.Errorf("%s %s active edge missing at [%d %d]: blue-red=%d", name, side, x, iy, b-r)
				}
			}
			if target.Y+inset < viewport.Y+16 || target.Y+target.H-inset > viewport.Y+viewport.H-16 {
				t.Errorf("%s active edge intersects 16px fade: rect=%+v viewport=%+v", name, target, viewport)
			}
			if calls != 0 {
				t.Fatal("focus invoked an action")
			}
			runVisibilityWheel(t, a, tt)
			if name != "mode" && name != "choose" && name != "language" {
				before := a.toolParams[0]
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("continued")
				tt.Frame()
				got := map[string]string{"name": a.toolName, "program": a.toolProgram, "args": a.toolArgs,
					"versionargs": a.toolVersionArgs, "description": a.toolDescription, "script": a.toolScript}[name]
				if got != "continued" || a.toolParams[0] != before || calls != 0 {
					t.Fatalf("edit ownership lost: %s=%q calls=%d", name, got, calls)
				}
			} else if name == "mode" {
				tt.Key(0, ui.KeyRight)
				tt.Frame()
				if a.toolMode != routeScript || calls != 0 {
					t.Fatal("mode selection failed")
				}
			} else if name == "choose" {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if calls != 1 {
					t.Fatalf("picker dispatch=%d", calls)
				}
			} else {
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnd)
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if a.toolLanguage == "python" || a.toolScript != "print('sunny')" || calls != 0 {
					t.Fatal("language change lost script or failed")
				}
			}
		})
	}
}

func TestToolFieldEdgeName(t *testing.T)    { checkToolFieldEdge(t, "name") }
func TestToolFieldEdgeProgram(t *testing.T) { checkToolFieldEdge(t, "program") }

func TestToolFieldEdgeArgs(t *testing.T)        { checkToolFieldEdge(t, "args") }
func TestToolFieldEdgeVersionArgs(t *testing.T) { checkToolFieldEdge(t, "versionargs") }

func TestToolFieldEdgeDescription(t *testing.T) { checkToolFieldEdge(t, "description") }
func TestToolFieldEdgeMode(t *testing.T)        { checkToolFieldEdge(t, "mode") }

func TestToolFieldEdgeChoose(t *testing.T)   { checkToolFieldEdge(t, "choose") }
func TestToolFieldEdgeLanguage(t *testing.T) { checkToolFieldEdge(t, "language") }
