package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func checkRunDefaultRefresh(t *testing.T, unknown, during bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			in := Integration{ID: "runner", Name: "Runner", Running: true, Enabled: true, Params: []Param{{ID: "target", Label: "目标", Kind: extensions.ParamText, Default: "old"}}}
			a.Integrations = func() []Integration { return []Integration{in} }
			a.runOpen = map[string]bool{in.ID: true}
			// Empty answers use the current declaration's fallback at runtime.
			a.runValues = map[string]map[string]string{in.ID: {"target": ""}}
			copy := a.copy()
			failure := extensions.ParamFailure{Key: extensions.RunErrorParamInvalid, Param: "target"}
			message, name := copy.RunParamInvalid("目标"), "known"
			if unknown {
				failure = extensions.ParamFailure{Key: extensions.RunErrorParamUnknown, Param: "retired"}
				message, name = copy.RunParamUnknown("retired"), "unknown"
			}
			if during {
				name += "-callback"
			} else {
				name += "-later"
			}
			calls, staleFrames := 0, 0
			a.Actions.RunWithParams = func(_ string, sent map[string]string) *extensions.ParamFailure {
				calls++
				if sent["target"] != "" {
					t.Errorf("changed session answer: %v", sent)
				}
				if during {
					in.Params[0].Default = "fresh"
				}
				return &failure
			}
			ground := realPageGround(t, dark)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(20).Children(func() { a.extensionMetrics(func() { a.runForm(c, copy, in) }) })
				if during && calls > 0 && a.runError[in.ID] != "" {
					staleFrames++
				}
			}, 720, 520)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			if err := tt.Click(copy.RunLabel); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !during {
				if !tt.HasText(message) {
					t.Fatal("initial refusal absent")
				}
				tt.Frame()
				if !tt.HasText(message) {
					t.Fatal("unchanged refusal retired")
				}
				in.Params[0].Default = "fresh"
			}
			tt.Frame()
			validationRefreshDump(t, tt, name, dark)
			if tt.HasText(message) || a.runError[in.ID] != "" || staleFrames != 0 {
				t.Errorf("%s retained obsolete default refusal: staleFrames=%d error=%q", name, staleFrames, a.runError[in.ID])
			}
			if calls != 1 || a.runValues[in.ID]["target"] != "" {
				t.Errorf("refresh dispatched or replaced answer: calls=%d values=%v", calls, a.runValues[in.ID])
			}
			a.Actions.RunWithParams = func(_ string, sent map[string]string) *extensions.ParamFailure {
				calls++
				args, err := extensions.ParamArguments([]extensions.ParamDefinition{{ID: "target", Kind: extensions.ParamText, Default: in.Params[0].Default}}, sent, false)
				if err != nil || len(args) != 1 || args[0] != "fresh" {
					t.Errorf("retry did not use refreshed default: %v %v", args, err)
				}
				return nil
			}
			if err := tt.Click(copy.RunLabel); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 {
				t.Errorf("retry calls=%d", calls)
			}
		})
	}
}

func TestRunDefaultRefreshKnownLater(t *testing.T)      { checkRunDefaultRefresh(t, false, false) }
func TestRunDefaultRefreshUnknownLater(t *testing.T)    { checkRunDefaultRefresh(t, true, false) }
func TestRunDefaultRefreshKnownCallback(t *testing.T)   { checkRunDefaultRefresh(t, false, true) }
func TestRunDefaultRefreshUnknownCallback(t *testing.T) { checkRunDefaultRefresh(t, true, true) }
