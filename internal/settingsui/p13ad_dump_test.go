package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-ad 的截图：卸载对话框的组件卡在深/浅两套外观上的静息帧。
//
// 这一轮**没有静态墨改**——改的是指针的颜色步长（`theme.ColorStep`，120ms）。
// 出这两张图是为了给出「面一个字节都没动」的证据，而不是为了看差别：
//
//	FLOTER_P13AD_DUMP=1 go test -count=1 -run TestDumpP13adColorStep ./internal/settingsui/
//
// 默认跳过：它是产物 dump，不是断言。
func TestDumpP13adColorStep(t *testing.T) {
	if os.Getenv("FLOTER_P13AD_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13AD_DUMP=1 to write /tmp/floter-P13-ad-uninstallcard-*.png")
	}
	on := true
	part := uninstallPart{Label: "Host configuration", Hint: "Remove the host configuration.", Value: &on}
	a := uninstallTestApp()
	for _, dark := range []bool{true, false} {
		ground := ui.RGB(247, 247, 249)
		name := "/tmp/floter-P13-ad-uninstallcard-light.png"
		if dark {
			ground = ui.RGB(24, 25, 29)
			name = "/tmp/floter-P13-ad-uninstallcard-dark.png"
		}
		dark := dark
		tt := ui.NewTester(func(c *ui.Context) {
			if dark {
				c.SetTheme(ui.DarkTheme())
			} else {
				c.SetTheme(ui.LightTheme())
			}
			ui.Column(c).Fill().Background(ground).Padding(40).Children(func() {
				a.uninstallPartRow(c, part)
			})
		}, 500, 200)
		tt.Frame()
		tt.Frame()
		f, err := os.Create(name)
		if err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatalf("encoding %s: %v", name, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("closing %s: %v", name, err)
		}
	}
}
