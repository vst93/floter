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
			if name == "mode" || name == "choose" {
				// Existing integrations can place the entire form below the
				// first viewport; mode must work there as well as on an empty page.
				a.Integrations = func() []Integration {
					return []Integration{{ID: "clock", Name: "时间工具", Enabled: true, RuntimeAvailable: true}, {ID: "notes", Name: "便条工具", Enabled: true, RuntimeAvailable: true}}
				}
			}
			copy := a.copy()
			label := map[string]string{
				"add": copy.ParamsAdd, "remove": copy.ParamsRemove,
				"required": copy.ParamsRequired, "kind": copy.ParamsKind,
				"mode": copy.LocalToolModeProgram, "output": copy.LocalToolTerminal,
				"advanced": copy.LocalToolAdvanced, "save": copy.LocalToolSave, "choose": copy.LocalToolChoose,
			}[name]
			calls := 0
			a.Actions.EditCustomTool = func(string, CustomTool) { calls++ }
			if name == "choose" {
				a.Actions.ChooseProgram = func() (string, error) { calls++; return "/usr/bin/forecast", nil }
			}
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
			if name == "advanced" || name == "save" || name == "choose" {
				r.Y -= (30 - r.H) / 2
				r.H = 30
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
			case "save":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if calls != 1 {
					t.Fatalf("Save dispatch=%d", calls)
				}
			case "choose":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if calls != 1 || a.toolProgram != "/usr/bin/forecast" || !tt.Focused(copy.LocalToolProgram) {
					t.Fatal("program picker lost result or keyboard outlet")
				}
			case "mode":
				tt.Key(0, ui.KeyRight)
				tt.Frame()
				if a.toolMode != routeScript {
					t.Fatal("mode keyboard choice failed")
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
