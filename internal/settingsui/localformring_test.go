package settingsui

import (
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func localFormRingDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KH_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kh-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkLocalFormRing(t *testing.T, name string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 5)
			copy := a.copy()
			label := map[string]string{
				"add": copy.ParamsAdd, "remove": copy.ParamsRemove,
				"required": copy.ParamsRequired, "kind": copy.ParamsKind,
				"output":   copy.LocalToolTerminal,
				"advanced": copy.LocalToolAdvanced, "save": copy.LocalToolSave,
				"platforms": customPlatforms[0].Label,
			}[name]
			calls := 0
			a.Actions.EditCustomTool = func(string, CustomTool) { calls++ }
			tt := toolVisibilityFrame(t, a, dark)
			for i := 0; i < 100 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatalf("cannot Tab to %s", label)
			}
			localFormRingDump(t, tt, name, dark)
			r := mustFind(t, tt, label)
			if name == "kind" {
				// The repeated field caption is not the select's box. Its
				// current value is centered in the existing 30px control.
				r = mustFind(t, tt, paramKindLabels(copy)[0])
				r.Y -= (30 - r.H) / 2
				r.H = 30
			}
			if name == "save" {
				r.Y -= (30 - r.H) / 2
				r.H = 30
			}
			if name == "advanced" {
				// The row caption shares the action's word. Locate the action
				// using the existing row's 12px gap and the 44px capsule;
				// Find clips the subtitle at the viewport edge, so take its
				// full line height from the same body style higher in the form.
				sub := mustFind(t, tt, copy.LocalToolAdvancedPermissions)
				line := mustFind(t, tt, copy.LocalToolVersionArgsHint)
				r.Y = (r.Y + sub.Y + line.H - 30) / 2
				r.X, r.W = sub.X+sub.W+12, 44
				r.H = 30
			}
			if name == "platforms" {
				// ui.Checkbox paints the focus ring around its 16px box,
				// eight units before the caption, not around the caption.
				r.X -= 24
				r.Y += (r.H - 16) / 2
				r.W, r.H = 16, 16
			}
			t.Logf("%s rect=%+v scroll=%v", name, r, a.Body.Y)
			for side, p := range map[string][2]int{
				"top":    {int(r.X + r.W/2), int(math.Round(float64(r.Y - 2)))},
				"bottom": {int(r.X + r.W/2), int(math.Round(float64(r.Y + r.H + 1)))},
			} {
				red, _, blue := rgbAt(tt.Image(), p[0], p[1])
				if blue-red < 20 {
					t.Errorf("%s %s ring missing at %v: blue-red=%d", name, side, p, blue-red)
				}
			}
			if calls != 0 {
				t.Fatal("focus triggered save")
			}
			runVisibilityWheel(t, a, tt)
			switch name {
			case "add":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if len(a.toolParams) != 6 || !tt.Focused(copy.ParamsLabel) {
					t.Fatal("Add lost its original editing outlet")
				}
			case "remove":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if len(a.toolParams) != 4 || a.toolParams[0].ID != "field1" || !tt.Focused(copy.ParamsLabel) {
					t.Fatal("Remove lost next declaration outlet")
				}
			case "required":
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if !a.toolParams[0].Required || a.toolParams[1].Required {
					t.Fatal("Space changed wrong required value")
				}
			case "kind":
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnd)
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if a.toolParams[0].Kind != paramKinds[len(paramKinds)-1] {
					t.Fatal("Kind keyboard selection failed")
				}
			case "advanced":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if !a.toolAdvanced {
					t.Fatal("Advanced keyboard disclosure failed")
				}
			case "platforms":
				before := a.toolPlatforms[customPlatforms[0].ID]
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if a.toolPlatforms[customPlatforms[0].ID] == before {
					t.Fatal("platform checkbox did not toggle")
				}
			case "save":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if calls != 1 {
					t.Fatalf("Save dispatch=%d", calls)
				}
			case "output":
				tt.Key(0, ui.KeyRight)
				tt.Frame()
				if a.toolOutput != routeBackground {
					t.Fatal("output keyboard choice failed")
				}
			}
		})
	}
}

func TestLocalFormRingAdd(t *testing.T)    { checkLocalFormRing(t, "add") }
func TestLocalFormRingRemove(t *testing.T) { checkLocalFormRing(t, "remove") }

func TestLocalFormRingRequired(t *testing.T) { checkLocalFormRing(t, "required") }
func TestLocalFormRingKind(t *testing.T)     { checkLocalFormRing(t, "kind") }

func TestLocalFormRingOutput(t *testing.T) { checkLocalFormRing(t, "output") }
func TestLocalFormRingSave(t *testing.T)   { checkLocalFormRing(t, "save") }

func TestLocalFormRingAdvanced(t *testing.T)  { checkLocalFormRing(t, "advanced") }
func TestLocalFormRingPlatforms(t *testing.T) { checkLocalFormRing(t, "platforms") }
