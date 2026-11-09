package settingsui

import (
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func TestParamPrecedingLabel(t *testing.T) { checkParamPreceding(t, "label") }
func TestParamPrecedingID(t *testing.T)    { checkParamPreceding(t, "id") }

func TestParamPrecedingPlaceholder(t *testing.T) { checkParamPreceding(t, "placeholder") }
func TestParamPrecedingDefault(t *testing.T)     { checkParamPreceding(t, "default") }

func TestParamPrecedingFlag(t *testing.T) { checkParamPreceding(t, "flag") }
func TestParamPrecedingKind(t *testing.T) { checkParamPreceding(t, "kind") }

func TestParamPrecedingRemove(t *testing.T) { checkParamPreceding(t, "remove") }

func TestParamPrecedingRequired(t *testing.T) { checkParamPreceding(t, "required") }
func TestParamPrecedingOptions(t *testing.T)  { checkParamPreceding(t, "options") }

func checkParamPreceding(t *testing.T, field string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, change := range []string{"refusal", "options"} {
			t.Run(appearanceName(dark)+"/"+change, func(t *testing.T) {
				a := declarationVisibilityApp(t, dark, 2)
				for i := range a.toolParams {
					a.toolParams[i].Kind, a.toolParams[i].Options = extensions.ParamSelect, "北京\n上海"
				}
				if field == "kind" {
					a.toolParams[1].Kind = extensions.ParamText
				}
				copy := a.copy()
				label := map[string]string{"label": copy.ParamsLabel, "id": copy.ParamsID,
					"placeholder": copy.ParamsPlaceholder, "default": copy.ParamsDefault, "flag": copy.ParamsFlag,
					"kind": copy.ParamsKind, "required": copy.ParamsRequired, "options": copy.ParamsOptions, "remove": copy.ParamsRemove}[field]
				var viewport, control ui.Rect
				focus := true
				tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
					viewport = c.Resolve(a.Body.Handle).Bounds()
					if len(a.toolParams) < 2 {
						return
					}
					targets := a.paramRefusalTargets(c, &a.toolParams[1])
					handle := targets.id
					switch field {
					case "default":
						handle = targets.defaultValue
					case "flag", "required", "remove":
						handle = targets.flag
					case "options":
						handle = targets.options
					}
					control = c.Resolve(handle).Bounds()
					if focus && control.W > 0 {
						c.Resolve(handle).Focus().ScrollIntoView()
						focus = false
					}
				})
				switch field {
				case "label":
					tt.Key(ui.Shift, ui.KeyTab)
				case "kind", "required":
					tt.Key(0, ui.KeyTab)
				case "placeholder", "remove":
					tt.Key(0, ui.KeyTab)
					tt.Key(0, ui.KeyTab)
				}
				tt.Frame()
				var requiredHeight float32
				bounds := func() ui.Rect {
					r := control
					switch field {
					case "kind":
						r = mustFind(t, tt, copy.ParamsKindText)
						r.Y -= (30 - r.H) / 2
						r.H = 30
					case "placeholder":
						var ok bool
						r, ok = tt.TextCaret()
						if !ok {
							t.Fatal("placeholder caret missing")
						}
						r.Y -= (30 - r.H) / 2
						r.H = 30
					case "required":
						// Both required toggles have the same label and measured
						// height; the checkbox is centered in the 28px action row.
						h := mustFind(t, tt, label).H
						if h > 0 {
							requiredHeight = h
						}
						// The first row can be culled after expanding its options.
						h = requiredHeight
						if h <= 0 {
							t.Fatal("required checkbox was never measured")
						}
						r.Y += r.H + 6 + (28-h)/2
						r.H = h
					case "remove":
						r.Y += r.H + 6
						r.H = 28
					}
					return r
				}
				// Put the active control at its legal lower edge before feedback.
				r := bounds()
				a.Body.Y += r.Y + r.H + 19 - viewport.Y - viewport.H
				tt.Frame()
				for step, expanded := range []bool{true, false, true} {
					if change == "refusal" {
						a.toolParams[0].ID = "field0"
						if expanded {
							a.toolParams[0].ID = "bad id"
						}
					} else {
						a.toolParams[0].Options = "北京\n上海"
						if expanded {
							a.toolParams[0].Options = strings.Repeat("广州\n", 18) + "上海"
						}
					}
					tt.Frame()
					if field == "id" && change == "options" && step == 0 {
						if tag := os.Getenv("FLOTER_P13LF_DUMP"); tag != "" {
							f, err := os.Create("/tmp/floter-P13-lf-params-" + appearanceName(dark) + "-" + tag + ".png")
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
						t.Fatalf("focus left second %s", field)
					}
					r = bounds()
					pixel := func(v float32) int { return int(math.Round(float64(v))) }
					if r.W <= 0 || r.H <= 0 || pixel(r.Y-3) < pixel(viewport.Y+16) || pixel(r.Y+r.H+3) > pixel(viewport.Y+viewport.H-16) {
						t.Errorf("preceding %s hides %s: target=%+v viewport=%+v", change, field, r, viewport)
					}
				}
				first := a.toolParams[0]
				switch field {
				case "remove":
					tt.Key(0, ui.KeyEnter)
					tt.Frame()
					if len(a.toolParams) != 1 {
						t.Fatal("remove did not target second record")
					}
				case "kind":
					tt.Key(0, ui.KeyDown)
					tt.Key(0, ui.KeyEnd)
					tt.Key(0, ui.KeyEnter)
					tt.Frame()
					if a.toolParams[1].Kind != paramKinds[len(paramKinds)-1] {
						t.Error("kind changed wrong owner")
					}
				case "required":
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					if !a.toolParams[1].Required {
						t.Error("required changed wrong owner")
					}
				default:
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type("continued")
					tt.Frame()
					p := a.toolParams[1]
					value := map[string]string{"label": p.Label, "id": p.ID, "placeholder": p.Placeholder, "default": p.Default, "flag": p.Flag, "options": p.Options}[field]
					if value != "continued" {
						t.Errorf("%s edited wrong owner: %q", field, value)
					}
				}
				if a.toolParams[0] != first {
					t.Error("edited preceding record")
				}
				before := a.Body.Y
				dy := float32(140)
				if before == a.Body.MaxY {
					dy = -140
				}
				// Use the page gutter, outside the preceding textarea's own scroll.
				tt.Wheel(710, 280, 0, dy)
				tt.Advance(250 * time.Millisecond)
				tt.Frame()
				if a.Body.Y == before {
					t.Fatal("page gutter wheel did not scroll")
				}
				manual := a.Body.Y
				a.toolParams[0].Options = strings.ReplaceAll(a.toolParams[0].Options, "广州", "深圳")
				tt.Frame()
				tt.Frame()
				if a.Body.Y != manual {
					t.Error("same-height text reclaimed manual scrolling")
				}
			})
		}
	}
}
