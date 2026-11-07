package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
	"github.com/egoist/mygo/ui"
)

func diagnosisDetailFrame(t *testing.T, integration Integration, dark bool, scenario string) *ui.Tester {
	t.Helper()
	a := New(newStore(t), Actions{})
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := theme.ForPreferences(settings.Default(), dark, c.Preferences())
		c.SetTheme(tokens.Theme)
		ui.Column(c).Width(360).Children(func() { a.diagnosticsSection(c, i18n.For("zh").Settings, integration) })
	}, 400, 240)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	if tag := os.Getenv("FLOTER_P13JP_DUMP"); tag != "" {
		file, err := os.Create("/tmp/floter-P13-jp-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	return tt
}

func TestDiagnosisDetailEmptyMessage(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			for _, message := range []string{"", " \t "} {
				tt := diagnosisDetailFrame(t, Integration{DiagnosisStatus: "error", DiagnosisChecks: []DiagnosisCheck{{ID: "network.connection", Status: "error", Message: message}, {ID: "private-id", Status: "ok", Message: "连接已恢复"}}}, dark, "empty-message")
				if !tt.HasText("network.connection") {
					t.Error("empty check message hides the failing check identity")
				}
				if !tt.HasText("连接已恢复") || tt.HasText("private-id") {
					t.Error("nonempty check message lost its original content")
				}
			}
		})
	}
}

func TestDiagnosisDetailFailureInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			message := "连接检查失败，请重新连接"
			tt := diagnosisDetailFrame(t, Integration{Diagnosis: message, DiagnosisFailed: true}, dark, "failure-ink")
			r := mustFind(t, tt, message)
			colored := 0
			img := tt.Image()
			warning := theme.ForPreferences(settings.Default(), dark, ui.Preferences{ReduceMotion: true}).Theme.Warning
			warning = panelOver(warning, imagePixel(img, 1, int(r.Y)))
			for y := int(r.Y); y < int(r.Y+r.H); y++ {
				for x := int(r.X); x < int(r.X+r.W); x++ {
					p := imagePixel(img, x, y)
					if p.R == warning.R && p.G == warning.G && p.B == warning.B {
						colored++
					}
				}
			}
			if colored == 0 {
				t.Errorf("failed diagnosis summary has no warning ink: want=%+v", warning)
			}
		})
	}
}
