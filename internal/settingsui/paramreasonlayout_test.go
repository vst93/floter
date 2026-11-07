package settingsui

import (
	"strings"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func checkParamReasonLayout(t *testing.T, backend bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			label := strings.Repeat("等待检查的目标目录", 9)
			param := Param{ID: "target", Label: label, Kind: extensions.ParamText, Required: !backend}
			in := Integration{ID: "runner", Name: "Runner", Running: true, Enabled: true, Params: []Param{param}}
			a.runOpen = map[string]bool{in.ID: true}
			a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure {
				return &extensions.ParamFailure{Key: extensions.RunErrorParamInvalid, Param: "target"}
			}
			copy := a.copy()
			message, name := copy.RunParamRequired(label), "local-reason-wrap"
			if backend {
				message, name = copy.RunParamInvalid(label), "backend-reason-wrap"
			}
			ground := realPageGround(t, dark)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(20).Children(func() { a.extensionMetrics(func() { a.runForm(c, copy, in) }) })
			}, 720, 620)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			if backend {
				if err := tt.Click(copy.RunLabel); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
			}
			r := mustFind(t, tt, message)
			validationRefreshDump(t, tt, name, dark)
			if r.X < 20 || r.X+r.W > 700 || r.H <= 20 {
				t.Errorf("reason must wrap inside form: %+v", r)
			}
			if !backend {
				button := mustFind(t, tt, copy.RunLabel)
				if button.Y < r.Y+r.H {
					t.Errorf("error overlaps actions: reason=%+v run=%+v", r, button)
				}
				runLabel := mustFind(t, tt, label)
				tt.ClickAt(568, runLabel.Y+runLabel.H/2+9)
				tt.Type("corrected")
				tt.Frame()
				if tt.HasText(message) {
					t.Error("corrected field kept wrapped reason")
				}
			}
		})
	}
}

func TestParamReasonLayoutLocal(t *testing.T)   { checkParamReasonLayout(t, false) }
func TestParamReasonLayoutBackend(t *testing.T) { checkParamReasonLayout(t, true) }
