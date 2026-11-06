package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func epochDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IG_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ig-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkRunEpoch(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "remembered"})
			a.Integrations = func() []Integration { return []Integration{*in} }
			copy := i18n.For("en").Settings
			calls, staleFrames := 0, 0
			a.Actions.RunWithParams = func(_ string, sent map[string]string) *extensions.ParamFailure {
				calls++
				if sent["target"] != "remembered" {
					t.Errorf("submitted %v", sent)
				}
				switch scenario {
				case "closed":
					a.runOpen[in.ID] = false
				case "replaced":
					in.Params = []Param{{ID: "fresh", Label: "Fresh", Kind: extensions.ParamText, Default: "current"}}
				}
				return &extensions.ParamFailure{Key: extensions.RunErrorParamInvalid, Param: "target"}
			}
			ground := realPageGround(t, dark)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(20).Children(func() { a.extensionMetrics(func() { a.runForm(c, copy, *in) }) })
				if calls > 0 && a.runError[in.ID] != "" {
					staleFrames++
				}
			}, 720, 520)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			if err := tt.Click(copy.RunLabel); err != nil {
				t.Fatal(err)
			}
			if scenario == "closed" {
				a.runOpen[in.ID] = true
			}
			tt.Frame()
			epochDump(t, tt, "run-"+scenario, dark)
			if staleFrames != 0 || tt.HasText(copy.RunParamInvalid("Target")) {
				t.Errorf("%s run published stale refusal in %d frames", scenario, staleFrames)
			}
			if calls != 1 {
				t.Errorf("runs=%d, want 1", calls)
			}
			key, want := "target", "remembered"
			if scenario == "replaced" {
				key, want = "fresh", "current"
			}
			if a.runValues[in.ID][key] != want {
				t.Errorf("current answer lost: %v", a.runValues[in.ID])
			}
		})
	}
}

func TestRunEpochClosedDuringCallback(t *testing.T)             { checkRunEpoch(t, "closed") }
func TestRunEpochParametersReplacedDuringCallback(t *testing.T) { checkRunEpoch(t, "replaced") }
