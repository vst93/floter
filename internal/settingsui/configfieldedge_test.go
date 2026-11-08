package settingsui

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func configFieldEdgeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KL_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kl-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkConfigFieldEdge(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			in.Params = nil
			a.Actions.RunWithParams = nil
			in.ConfigValues = map[string]any{}
			for i := 0; i < 12; i++ {
				key := fmt.Sprintf("c%d", i)
				in.Config = append(in.Config, ConfigField{Key: key, Label: fmt.Sprintf("配置 %d", i+1), Type: "text", Default: "stored"})
			}
			field := &in.Config[4]
			field.Type, field.Options = kind, []string{"North", "South"}
			switch kind {
			case "number":
				field.Default = float64(5)
			case "boolean":
				field.Default = false
			case "select":
				field.Default = "North"
			}
			a.configOpen = map[string]bool{in.ID: true}
			a.Actions.SaveConfiguration = func(string, map[string]any, func(error)) { t.Error("focus saved configuration") }
			var viewport, target ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				viewport = c.Resolve(a.Body.Handle).Bounds()
				grid := c.Resolve(a.noticeFocus(c, in.ID, "field").panel).Bounds()
				target = ui.Rect{X: grid.X + 130, Y: grid.Y, W: grid.W - 130, H: 28}
				if kind == "boolean" {
					target.W, target.H = 32.4, 18
				}
			})
			configFieldEdgeDump(t, tt, kind+"-rest", dark)
			for i := 0; i < 80 && !tt.Focused(field.Label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(field.Label) {
				t.Fatal("cannot reach configuration field")
			}
			absoluteY := target.Y + a.Body.Y
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			a.Body.Y = absoluteY - viewport.Y + 8
			tt.Frame()
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Frame()
			configFieldEdgeDump(t, tt, kind, dark)
			if !tt.Focused(field.Label) {
				t.Fatal("return lost field focus")
			}
			inset := float32(0)
			if kind == "select" || kind == "boolean" {
				inset = -2
			}
			t.Logf("%s target=%+v viewport=%+v scroll=%v", kind, target, viewport, a.Body.Y)
			for side, y := range map[string]float32{"top": target.Y + inset, "bottom": target.Y + target.H - 1 - inset} {
				r, _, b := rgbAt(tt.Image(), int(target.X+target.W/2), int(math.Round(float64(y))))
				if b-r < 60 {
					t.Errorf("%s %s active edge missing: blue-red=%d", kind, side, b-r)
				}
			}
			if target.Y+inset < viewport.Y+16 || target.Y+target.H-inset > viewport.Y+viewport.H-16 {
				t.Errorf("%s active edge intersects fade: target=%+v viewport=%+v", kind, target, viewport)
			}
			runVisibilityWheel(t, a, tt)
			want := any("continued")
			switch kind {
			case "boolean":
				tt.Key(0, ui.KeySpace)
				want = true
			case "select":
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnd)
				tt.Key(0, ui.KeyEnter)
				want = "South"
			default:
				tt.Key(ui.Cmd, ui.KeyA)
				if kind == "number" {
					tt.Type("12")
					want = float64(12)
				} else {
					tt.Type("continued")
				}
			}
			tt.Frame()
			values := a.draft(*in).values
			if values[field.Key] != want || values["c5"] != "stored" {
				t.Fatalf("edit ownership: %v", values)
			}
		})
	}
}

func checkRegistryFieldEdge(t *testing.T, version bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, _ := runVisibilityApp(t, dark)
			a.Actions.InstallFromRegistry = func(string, string) { t.Error("focus dispatched install") }
			a.installName, a.installVersion = "weather-tool", "^1.0.0"
			label, name := a.copy().IntegrationsPackageHint, "registry-name"
			if version {
				name = "registry-version"
			}
			var viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { viewport = c.Resolve(a.Body.Handle).Bounds() })
			configFieldEdgeDump(t, tt, name+"-rest", dark)
			for i := 0; i < 80 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatal("cannot reach registry field")
			}
			if version {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			bounds := func() ui.Rect {
				r := mustFind(t, tt, label)
				if version {
					r.X += r.W + 3.6
					r.W = 160
				}
				return r
			}
			target := bounds()
			originalWidth, absoluteY := target.W, target.Y+a.Body.Y
			wantWidth := float32(292)
			if version {
				wantWidth = 160
			}
			if originalWidth != wantWidth {
				t.Fatalf("registry width=%v want=%v", originalWidth, wantWidth)
			}
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			a.Body.Y = absoluteY - viewport.Y + 8
			tt.Frame()
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Frame()
			target = bounds()
			configFieldEdgeDump(t, tt, name, dark)
			t.Logf("%s target=%+v viewport=%+v scroll=%v", name, target, viewport, a.Body.Y)
			caret, editing := tt.TextCaret()
			if !editing || caret.X < target.X || caret.X > target.X+target.W || target.W != originalWidth {
				t.Fatalf("registry focus or width changed: %+v", target)
			}
			for side, y := range map[string]float32{"top": target.Y, "bottom": target.Y + target.H - 1} {
				r, _, b := rgbAt(tt.Image(), int(target.X+target.W/2), int(math.Round(float64(y))))
				if b-r < 60 {
					t.Errorf("%s %s active edge missing: blue-red=%d", name, side, b-r)
				}
			}
			if target.Y < viewport.Y+16 || target.Y+target.H > viewport.Y+viewport.H-16 {
				t.Errorf("%s active edge intersects fade: %+v", name, target)
			}
			runVisibilityWheel(t, a, tt)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("next")
			tt.Frame()
			if version && (a.installVersion != "next" || a.installName != "weather-tool") || !version && (a.installName != "next" || a.installVersion != "^1.0.0") {
				t.Fatal("registry edit ownership changed")
			}
		})
	}
}

func TestRegistryFieldEdgeName(t *testing.T)    { checkRegistryFieldEdge(t, false) }
func TestRegistryFieldEdgeVersion(t *testing.T) { checkRegistryFieldEdge(t, true) }

func TestConfigFieldEdgeText(t *testing.T)     { checkConfigFieldEdge(t, "text") }
func TestConfigFieldEdgePassword(t *testing.T) { checkConfigFieldEdge(t, "password") }

func TestConfigFieldEdgePath(t *testing.T)   { checkConfigFieldEdge(t, "path") }
func TestConfigFieldEdgeNumber(t *testing.T) { checkConfigFieldEdge(t, "number") }
