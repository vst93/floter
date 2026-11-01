package settingsui

import (
	"image/png"
	"os"
	"testing"
)

// P13-cw 的截图：集成卡上健康块那一屏（探针 / 失败卡 / 陈旧横幅），深浅两套。
//
// 这一轮改两件事：健康块从卡框收回卡的行列（左 192→200、右 701→693），
// 以及两条暖色 2px 重棱从尾边挪回前导边（参考的 `border-left`）。
//
//	FLOTER_P13CW_DUMP=1 FLOTER_P13CW_TAG=before go test -count=1 -run TestDumpP13cwHealthBlock ./internal/settingsui/
//
// 默认跳过：它是产物 dump，不是断言。
func TestDumpP13cwHealthBlock(t *testing.T) {
	if os.Getenv("FLOTER_P13CW_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13CW_DUMP=1 to write /tmp/floter-P13-cw-healthblock-*.png")
	}
	tag := os.Getenv("FLOTER_P13CW_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := healthBlockPage(t, dark)
		name := "/tmp/floter-P13-cw-healthblock-" + appearance + "-" + tag + ".png"
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
