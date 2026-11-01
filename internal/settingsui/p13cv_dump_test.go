package settingsui

import (
	"image/png"
	"os"
	"testing"
)

// P13-cv 的截图：健康页「Checks」组在深/浅两套外观上的静息帧。
//
// 这一轮改的是探针列表的标点（分隔线从「每行底边一道」改成「两行之间一个 1px 盒子，
// 最后一行不画」），出图是为了给报告里的前后对照用：
//
//	FLOTER_P13CV_DUMP=1 go test -count=1 -run TestDumpP13cvProbeRules ./internal/settingsui/
//
// 默认跳过：它是产物 dump，不是断言。
func TestDumpP13cvProbeRules(t *testing.T) {
	if os.Getenv("FLOTER_P13CV_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13CV_DUMP=1 to write /tmp/floter-P13-cv-healthprobes-*.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := probePage(t, testHealthReport(), dark)
		name := "/tmp/floter-P13-cv-healthprobes-" + appearance + "-now.png"
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
