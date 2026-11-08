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

func runVisibilityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KE_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ke-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func runVisibilityApp(t *testing.T, dark bool) (*App, *Integration) {
	t.Helper()
	a := noticeApp(t, dark)
	in := &Integration{ID: "weather", Name: "天气查询", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
	for i := 0; i < 12; i++ {
		in.Params = append(in.Params, Param{ID: fmt.Sprintf("p%d", i), Label: fmt.Sprintf("参数 %d", i+1), Kind: extensions.ParamText, Default: "12"})
	}
	a.runOpen = map[string]bool{in.ID: true}
	a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure { t.Error("unexpected run"); return nil }
	a.Integrations = func() []Integration {
		rows := []Integration{*in}
		for i := 0; i < 4; i++ {
			rows = append(rows, Integration{ID: fmt.Sprintf("tail%d", i), Name: fmt.Sprintf("后续工具 %d", i+1), Enabled: true, RuntimeAvailable: true})
		}
		return rows
	}
	return a, in
}

func runVisible(t *testing.T, a *App, tt *ui.Tester, label string, viewport ui.Rect) {
	t.Helper()
	r := mustFind(t, tt, label)
	if !tt.Focused(label) || r.W <= 0 || r.H <= 0 || !toolVisiblePixels(r).In(toolVisiblePixels(viewport)) {
		t.Fatalf("run outlet outside viewport: label=%q focused=%v rect=%+v viewport=%+v scroll=%v", label, tt.Focused(label), r, viewport, a.Body.Y)
	}
}

func runVisibilityWheel(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	before := a.Body.Y
	dy := float32(140)
	if before == a.Body.MaxY {
		dy = -140
	}
	tt.Wheel(600, 280, 0, dy)
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	if a.Body.Y == before {
		t.Fatal("outlet reveal trapped subsequent scrolling")
	}
}

func checkRunFieldVisibility(t *testing.T, change string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			var viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { viewport = c.Resolve(a.Body.Handle).Bounds() })
			optionExitFocus(t, tt, "参数 1")
			a.Body.Y = 0
			if change == "kind" {
				a.Body.Y = a.Body.MaxY
			}
			tt.Frame()
			want := a.copy().RunCancel
			if change == "kind" {
				in.Params[0].Kind = extensions.ParamNumber
				want = "参数 1"
			} else {
				in.Params = in.Params[1:]
			}
			tt.Frame()
			tt.Frame()
			runVisibilityDump(t, tt, "field-"+change, dark)
			runVisible(t, a, tt, want, viewport)
			if a.runValues[in.ID]["p1"] != "12" {
				t.Fatal("refresh changed surviving answer")
			}
			if change == "kind" {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
				tt.Frame()
				if a.runValues[in.ID]["p0"] != "24" {
					t.Fatal("replacement input cannot edit")
				}
			}
			runVisibilityWheel(t, a, tt)
			if change == "removed" {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if a.runOpen[in.ID] {
					t.Fatal("removed field cannot cancel by keyboard")
				}
			}
		})
	}
}

func TestRunVisibilityRemovedField(t *testing.T) { checkRunFieldVisibility(t, "removed") }
func TestRunVisibilityChangedKind(t *testing.T)  { checkRunFieldVisibility(t, "kind") }

func checkRunActionVisibility(t *testing.T, unavailable bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			calls := 0
			a.Actions.RunWithParams = func(id string, values map[string]string) *extensions.ParamFailure {
				calls++
				if id != in.ID || len(values) != 12 || values["p0"] != "mine" {
					t.Error("run lost submitted answers")
				}
				return nil
			}
			var viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { viewport = c.Resolve(a.Body.Handle).Bounds() })
			a.runValues[in.ID]["p0"] = "mine"
			optionExitFocus(t, tt, a.copy().RunCancel)
			name, want := "form-cancel", a.copy().RunAction
			if unavailable {
				tt.Key(0, ui.KeyTab)
				if !tt.Focused(a.copy().RunLabel) {
					t.Fatal("Run did not own keyboard before becoming unavailable")
				}
				a.Body.Y = 0
				tt.Frame()
				in.Running = false
				name, want = "submit-unavailable", a.copy().RunCancel
			} else {
				a.Body.Y = a.Body.MaxY
				tt.Frame()
				tt.Key(0, ui.KeyEnter)
			}
			tt.Frame()
			tt.Frame()
			runVisibilityDump(t, tt, name, dark)
			runVisible(t, a, tt, want, viewport)
			if calls != 0 || a.runValues[in.ID]["p0"] != "mine" {
				t.Fatal("exit ran a command or replaced draft")
			}
			runVisibilityWheel(t, a, tt)
			if unavailable {
				in.Running = true
				tt.Frame()
				tt.Key(0, ui.KeyTab)
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if calls != 1 {
					t.Fatal("available Run cannot retry")
				}
			} else {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if !a.runOpen[in.ID] || a.runValues[in.ID]["p0"] != "mine" {
					t.Fatal("cancelled form cannot reopen original answers")
				}
			}
		})
	}
}

func TestRunVisibilitySubmitUnavailable(t *testing.T) { checkRunActionVisibility(t, true) }
func TestRunVisibilityCancel(t *testing.T)            { checkRunActionVisibility(t, false) }

func checkRunFormVisibility(t *testing.T, empty bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			calls := 0
			a.Actions.RunWithParams = func(id string, values map[string]string) *extensions.ParamFailure {
				calls++
				if !empty || id != in.ID || values != nil {
					t.Error("retired inputs reached direct Run")
				}
				return nil
			}
			var viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { viewport = c.Resolve(a.Body.Handle).Bounds() })
			optionExitFocus(t, tt, "参数 1")
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("mine")
			tt.Frame()
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			name := "form-closed"
			if empty {
				in.Params = nil
				name = "form-empty"
			} else {
				a.runOpen[in.ID] = false
			}
			tt.Frame()
			tt.Frame()
			runVisibilityDump(t, tt, name, dark)
			runVisible(t, a, tt, a.copy().RunAction, viewport)
			if calls != 0 || a.runValues[in.ID]["p0"] != "mine" {
				t.Fatal("form retirement changed session answers or ran command")
			}
			runVisibilityWheel(t, a, tt)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if empty {
				if calls != 1 {
					t.Fatal("direct Run not keyboard reachable")
				}
			} else if !a.runOpen[in.ID] || a.runValues[in.ID]["p0"] != "mine" {
				t.Fatal("closed form cannot reopen edited answer")
			}
		})
	}
}

func TestRunVisibilityFormClosed(t *testing.T)           { checkRunFormVisibility(t, false) }
func TestRunVisibilityAllParametersRemoved(t *testing.T) { checkRunFormVisibility(t, true) }

func checkConfigReturnVisibility(t *testing.T, discard bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			in.Params = nil
			a.Actions.RunWithParams = nil
			in.ConfigValues = map[string]any{}
			for i := 0; i < 12; i++ {
				key := fmt.Sprintf("c%d", i)
				in.Config = append(in.Config, ConfigField{Key: key, Label: fmt.Sprintf("设置 %d", i+1), Type: "text"})
				in.ConfigValues[key] = "stored"
			}
			a.configOpen = map[string]bool{in.ID: true}
			a.Actions.SaveConfiguration = func(string, map[string]any, func(error)) { t.Error("confirmation saved configuration") }
			var viewport ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { viewport = c.Resolve(a.Body.Handle).Bounds() })
			a.draft(*in).values["c0"] = "mine"
			optionExitFocus(t, tt, a.copy().ConfigOpen)
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if !tt.Focused(a.copy().ConfigCancel) {
				t.Fatal("configuration question did not focus Cancel")
			}
			name := "config-cancel"
			if discard {
				tt.Key(0, ui.KeyTab)
				if !tt.Focused(a.copy().ConfigDiscard) {
					t.Fatal("Discard not keyboard reachable")
				}
				name = "config-discard"
			}
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			runVisibilityDump(t, tt, name, dark)
			runVisible(t, a, tt, a.copy().ConfigOpen, viewport)
			if a.configDiscard[in.ID] || a.configOpen[in.ID] == discard {
				t.Fatal("configuration confirmation changed its close rule")
			}
			want := "mine"
			if discard {
				want = "stored"
			}
			if a.draft(*in).values["c0"] != want {
				t.Fatal("configuration confirmation changed draft ownership")
			}
			runVisibilityWheel(t, a, tt)
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if discard && !a.configOpen[in.ID] || !discard && !a.configDiscard[in.ID] {
				t.Fatal("configuration return cannot reopen by keyboard")
			}
		})
	}
}

func TestConfigReturnVisibilityCancel(t *testing.T)  { checkConfigReturnVisibility(t, false) }
func TestConfigReturnVisibilityDiscard(t *testing.T) { checkConfigReturnVisibility(t, true) }

func TestRunVisibilityRefreshDoesNotFollowUnfocusedForm(t *testing.T) {
	for _, change := range []string{"kind", "removed", "closed", "empty", "unavailable"} {
		for _, dark := range []bool{false, true} {
			t.Run(change+"/"+appearanceName(dark), func(t *testing.T) {
				a, in := runVisibilityApp(t, dark)
				a.Actions.Close = func() {}
				tt := toolVisibilityFrame(t, a, dark)
				optionExitFocus(t, tt, a.copy().Close)
				a.Body.Y = 100
				tt.Frame()
				switch change {
				case "kind":
					in.Params[0].Kind = extensions.ParamNumber
				case "removed":
					in.Params = in.Params[1:]
				case "closed":
					a.runOpen[in.ID] = false
				case "empty":
					in.Params = nil
				case "unavailable":
					in.Running = false
				}
				tt.Frame()
				tt.Frame()
				if !tt.Focused(a.copy().Close) || a.Body.Y != 100 {
					t.Fatalf("unfocused %s moved focus or scroll: %v", change, a.Body.Y)
				}
			})
		}
	}
}
