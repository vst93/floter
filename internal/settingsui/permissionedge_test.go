package settingsui

import (
	"image/png"
	"math"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func permissionEdgeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KK_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kk-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkPermissionEdge(t *testing.T, permission string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 5)
			a.toolAdvanced = true
			a.toolPermissions = map[string]bool{}
			calls := 0
			a.Actions.EditCustomTool = func(string, CustomTool) { calls++ }
			label := i18n.PermissionLabel(a.settings().Language, permission)
			var viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { viewport = c.Resolve(a.Body.Handle).Bounds() })
			for i := 0; i < 100 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatalf("cannot reach permission %s", permission)
			}
			caption := mustFind(t, tt, label)
			// Find returns the row caption first, not the switch with the
			// same accessible label. The 32.4x18 switch follows the caption
			// by the card's literal 12px gap, centered in its 26px control slot.
			target := ui.Rect{X: caption.X + caption.W + 12, Y: caption.Y + 4, W: 32.4, H: 18}
			t.Logf("first target=%+v viewport=%+v scroll=%v", target, viewport, a.Body.Y)
			absoluteY := target.Y + a.Body.Y
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			a.Body.Y = absoluteY - viewport.Y + 8
			tt.Frame()
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Frame()
			target.Y = absoluteY - a.Body.Y
			permissionEdgeDump(t, tt, permission, dark)
			if !tt.Focused(label) {
				t.Fatal("return lost permission focus")
			}
			t.Logf("return target=%+v viewport=%+v scroll=%v", target, viewport, a.Body.Y)
			for side, y := range map[string]float32{"top": target.Y - 2, "bottom": target.Y + target.H + 1} {
				x, iy := int(target.X+target.W/2), int(math.Round(float64(y)))
				r, _, b := rgbAt(tt.Image(), x, iy)
				if b-r < 60 {
					t.Errorf("%s %s ring missing at [%d %d]: blue-red=%d", permission, side, x, iy, b-r)
				}
			}
			// Compare occupied raster rows: independent float32 sums can put
			// the same painted row at 67.39996 and 67.4. No pixel tolerance.
			if math.Floor(float64(target.Y-3)) < math.Floor(float64(viewport.Y+16)) || math.Ceil(float64(target.Y+target.H+3)) > math.Ceil(float64(viewport.Y+viewport.H-16)) {
				t.Errorf("%s ring intersects fade: target=%+v viewport=%+v", permission, target, viewport)
			}
			runVisibilityWheel(t, a, tt)
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if !a.toolPermissions[permission] || len(a.toolPermissions) != 1 || calls != 0 {
				t.Fatalf("permission ownership lost: %v calls=%d", a.toolPermissions, calls)
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if a.toolPermissions[permission] || calls != 0 {
				t.Fatal("permission could not toggle back")
			}
		})
	}
}

func TestPermissionEdgeFilesystemRead(t *testing.T) {
	checkPermissionEdge(t, extensions.PermissionFilesystemRead)
}
func TestPermissionEdgeFilesystemWrite(t *testing.T) {
	checkPermissionEdge(t, extensions.PermissionFilesystemWrite)
}
func TestPermissionEdgeNetwork(t *testing.T) {
	checkPermissionEdge(t, extensions.PermissionNetworkFetch)
}
func TestPermissionEdgeProcess(t *testing.T) {
	checkPermissionEdge(t, extensions.PermissionProcessSpawn)
}
func TestPermissionEdgeClipboardRead(t *testing.T) {
	checkPermissionEdge(t, extensions.PermissionClipboardRead)
}
func TestPermissionEdgeClipboardWrite(t *testing.T) {
	checkPermissionEdge(t, extensions.PermissionClipboardWrite)
}

func checkScriptEditorEdge(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 5)
			a.toolMode, a.toolLanguage, a.toolScript = routeScript, "python", "print('sunny')"
			var target, viewport ui.Rect
			leave := false
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				if leave {
					c.Resolve(a.contentFocus(c, "command").source).Focus()
					leave = false
				}
				target = c.Resolve(a.contentFocus(c, "script").source).Bounds()
				viewport = c.Resolve(a.Body.Handle).Bounds()
			})
			label := a.copy().LocalToolScript
			for i := 0; i < 100 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatal("cannot reach script")
			}
			absoluteY := target.Y + a.Body.Y
			// The editor consumes Tab as indentation. Seed the following
			// field, then use a real Shift+Tab to return into the editor.
			leave = true
			tt.Frame()
			a.Body.Y = absoluteY - viewport.Y + 8
			tt.Frame()
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Frame()
			permissionEdgeDump(t, tt, "script", dark)
			t.Logf("script target=%+v viewport=%+v focused=%v", target, viewport, tt.Focused(label))
			if !tt.Focused(label) {
				t.Fatal("return did not reach script")
			}
			if target.Y < viewport.Y+16 || target.Y+target.H > viewport.Y+viewport.H-16 {
				t.Errorf("script first line intersects fade: target=%+v viewport=%+v", target, viewport)
			}
			runVisibilityWheel(t, a, tt)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("print('rain')")
			tt.Frame()
			if a.toolScript != "print('rain')" {
				t.Fatalf("script edit lost: %q", a.toolScript)
			}
		})
	}
}
