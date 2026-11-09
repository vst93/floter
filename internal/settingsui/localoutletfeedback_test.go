package settingsui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"floter/internal/extensions"
	"floter/internal/scriptlang"
	"github.com/egoist/mygo/ui"
)

func TestLocalOutletCommand(t *testing.T) { checkLocalOutletFeedback(t, "command") }
func TestLocalOutletVersion(t *testing.T) { checkLocalOutletFeedback(t, "version") }
func TestLocalOutletIdentity(t *testing.T) {
	for _, field := range []string{"label", "id"} {
		t.Run(field, func(t *testing.T) { checkLocalOutletFeedback(t, field) })
	}
}
func TestLocalOutletValues(t *testing.T) {
	for _, field := range []string{"placeholder", "default", "flag"} {
		t.Run(field, func(t *testing.T) { checkLocalOutletFeedback(t, field) })
	}
}

func checkLocalOutletFeedback(t *testing.T, field string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := declarationVisibilityApp(t, dark, 2)
			a.toolParams[0].Kind, a.toolParams[0].Options = extensions.ParamSelect, "北京\n上海"
			a.Actions.ScriptRuntime = func(string) scriptlang.Check { t.Error("unexpected probe"); return scriptlang.Check{} }
			a.scriptRuntime = map[string]*runtimeProbe{"python": {done: make(chan struct{})}}
			a.toolLanguage = "python"
			copy := a.copy()
			label := map[string]string{"command": copy.LocalToolCommand, "version": copy.LocalToolVersion,
				"label": copy.ParamsLabel, "id": copy.ParamsID, "kind": copy.ParamsKind,
				"placeholder": copy.ParamsPlaceholder, "default": copy.ParamsDefault, "flag": copy.ParamsFlag,
				"required": copy.ParamsRequired, "add": copy.ParamsAdd, "remove": copy.ParamsRemove, "options": copy.ParamsOptions}[field]
			var viewport, control ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				viewport = c.Resolve(a.Body.Handle).Bounds()
				targets := a.paramRefusalTargets(c, &a.toolParams[0])
				handle := map[string]ui.Handle{"command": a.contentFocus(c, "command").source,
					"version": a.contentFocus(c, "version").source, "id": targets.id,
					"label": targets.id, "default": targets.defaultValue, "flag": targets.flag, "options": targets.options}[field]
				control = c.Resolve(handle).Bounds()
			})
			for i := 0; i < 80 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatalf("cannot Tab to %s", label)
			}
			// Enter the script branch only after Tab has reached the target:
			// the code editor deliberately consumes Tab as indentation.
			a.toolMode = routeScript
			tt.Frame()
			for step, expanded := range []bool{true, false, true} {
				probe := &runtimeProbe{done: make(chan struct{}), check: scriptlang.Check{Available: true, Version: strings.Repeat("Python release details ", 30)}}
				if expanded {
					close(probe.done)
				}
				a.scriptRuntime["python"] = probe
				tt.Frame()
				if field == "command" && step == 0 {
					if tag := os.Getenv("FLOTER_P13LE_DUMP"); tag != "" {
						f, err := os.Create("/tmp/floter-P13-le-local-" + appearanceName(dark) + "-" + tag + ".png")
						if err != nil {
							t.Fatal(err)
						}
						err = png.Encode(f, tt.Image())
						f.Close()
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				if !tt.Focused(label) {
					t.Fatalf("layout stole %s focus", field)
				}
				r := control
				switch field {
				case "placeholder", "kind":
					r = mustFind(t, tt, label)
					r.Y += r.H + 3
					r.H = 28
					if field == "kind" {
						r.H = 30
					}
				case "add", "remove", "required":
					r = mustFind(t, tt, label)
				}
				if r.W <= 0 || r.H <= 0 || r.Y-3 < viewport.Y+16 || r.Y+r.H+3 > viewport.Y+viewport.H-16 {
					t.Errorf("feedback hides %s: target=%+v viewport=%+v", field, r, viewport)
				}
			}
			switch field {
			case "kind":
				tt.Key(0, ui.KeyEnd)
				tt.Frame()
				if a.toolParams[0].Kind != paramKinds[len(paramKinds)-1] {
					t.Error("kind keyboard outlet stopped working")
				}
			case "required":
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if !a.toolParams[0].Required || a.toolParams[1].Required {
					t.Error("wrong required owner")
				}
			case "add", "remove":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				want := 3
				if field == "remove" {
					want = 1
				}
				if len(a.toolParams) != want {
					t.Errorf("list action: got %d want %d", len(a.toolParams), want)
				}
			default:
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("continued")
				tt.Frame()
				p := a.toolParams[0]
				value := map[string]string{"command": a.toolCommand, "version": a.toolVersion, "label": p.Label, "id": p.ID,
					"placeholder": p.Placeholder, "default": p.Default, "flag": p.Flag, "options": p.Options}[field]
				if value != "continued" {
					t.Errorf("%s edited wrong owner: %q", field, value)
				}
			}
			runVisibilityWheel(t, a, tt)
			manual := a.Body.Y
			a.toolDescription = "fixed-height draft"
			tt.Frame()
			tt.Frame()
			if a.Body.Y != manual {
				t.Error("unchanged layout reclaimed manual scrolling")
			}
		})
	}
}
