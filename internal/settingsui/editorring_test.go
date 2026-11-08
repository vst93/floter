package settingsui

import (
	"image/png"
	"math"
	"os"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func checkEditorRing(t *testing.T, name string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 5)
			a.toolParams[0].Kind = extensions.ParamSelect
			a.toolParams[0].Options = "北京\n上海"
			copy := a.copy()
			label := map[string]string{"label": copy.ParamsLabel, "id": copy.ParamsID,
				"placeholder": copy.ParamsPlaceholder, "default": copy.ParamsDefault,
				"flag": copy.ParamsFlag, "options": copy.ParamsOptions,
				"command": copy.LocalToolCommand, "version": copy.LocalToolVersion}[name]
			var target, viewport ui.Rect
			calls := 0
			a.Actions.EditCustomTool = func(string, CustomTool) { calls++ }
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				r := a.paramRefusalTargets(c, &a.toolParams[0])
				h := map[string]ui.Handle{"id": r.id, "default": r.defaultValue, "flag": r.flag, "options": r.options,
					"command": a.contentFocus(c, "command").source, "version": a.contentFocus(c, "version").source}[name]
				if name == "label" {
					h = r.id
				}
				if name == "placeholder" {
					h = r.defaultValue
				}
				target = c.Resolve(h).Bounds()
				viewport = c.Resolve(a.Body.Handle).Bounds()
			})
			for i := 0; i < 100 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatalf("cannot Tab to %s", label)
			}
			if name == "label" {
				target.X -= target.W + 8
			}
			if name == "placeholder" {
				target.X -= target.W + 8
			}
			// Re-enter from below after scrolling the current field above the
			// viewport, as a keyboard user returning through a long form does.
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			a.Body.Y += target.Y - viewport.Y + 8
			tt.Frame()
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Frame()
			if name == "label" || name == "placeholder" {
				target.X -= target.W + 8
			}
			if !tt.Focused(label) {
				t.Fatal("return lost input focus")
			}
			if tag := os.Getenv("FLOTER_P13KI_DUMP"); tag != "" {
				f, err := os.Create("/tmp/floter-P13-ki-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("%s rect=%+v scroll=%v", name, target, a.Body.Y)
			// Text editors paint their active 1px border inside the box;
			// unlike buttons they do not use the outer 2px keyboard ring.
			for side, y := range map[string]float32{"top": target.Y, "bottom": target.Y + target.H - 1} {
				x, iy := int(target.X+target.W/2), int(math.Round(float64(y)))
				r, _, b := rgbAt(tt.Image(), x, iy)
				t.Logf("%s %s RGB spread=%d", name, side, b-r)
				if b-r < 60 {
					t.Errorf("%s %s ring missing at [%d %d]: blue-red=%d", name, side, x, iy, b-r)
				}
			}
			// The page's 16px fade must not wash out the active editor.
			if target.Y < viewport.Y+16 || target.Y+target.H > viewport.Y+viewport.H-16 {
				t.Errorf("%s active border intersects 16px fade: rect=%+v viewport=%+v", name, target, viewport)
			}
			runVisibilityWheel(t, a, tt)
			want := map[string]string{"label": "新标签", "id": "new_id", "placeholder": "输入城市", "default": "北京", "flag": "--city", "options": "杭州\n深圳", "command": "forecast", "version": "2.3.4"}[name]
			expected, neighbor := a.toolParams[0], a.toolParams[1]
			switch name {
			case "label":
				expected.Label = want
			case "id":
				expected.ID = want
			case "placeholder":
				expected.Placeholder = want
			case "default":
				expected.Default = want
			case "flag":
				expected.Flag = want
			case "options":
				expected.Options = want
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type(want)
			tt.Frame()
			if a.toolParams[0] != expected || a.toolParams[1] != neighbor {
				t.Fatal("typing changed wrong parameter or field")
			}
			if name == "command" && a.toolCommand != want {
				t.Fatal("command edit lost")
			}
			if name == "version" && a.toolVersion != want {
				t.Fatal("version edit lost")
			}
			if calls != 0 {
				t.Fatal("editing unexpectedly saved")
			}
		})
	}
}

func TestEditorRingLabel(t *testing.T) { checkEditorRing(t, "label") }
func TestEditorRingID(t *testing.T)    { checkEditorRing(t, "id") }

func TestEditorRingPlaceholder(t *testing.T) { checkEditorRing(t, "placeholder") }
func TestEditorRingDefault(t *testing.T)     { checkEditorRing(t, "default") }

func TestEditorRingFlag(t *testing.T)    { checkEditorRing(t, "flag") }
func TestEditorRingOptions(t *testing.T) { checkEditorRing(t, "options") }

func TestEditorRingCommand(t *testing.T) { checkEditorRing(t, "command") }
func TestEditorRingVersion(t *testing.T) { checkEditorRing(t, "version") }
