package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// P13-eb · opt-in dump：助记提示（`␣ clipboard history`）在**出货尺**上的帧。
//
// 这一轮产品像素差是亚像素的（9.8 → 9.9，同一个寄存器差 0.1pt），出图不是为了
// 「看新样子」，而是留证据：提示在 240px 宽的窗里被行挤到 105px 宽、一行、贴着
// 字段右沿，深浅各一张。`FLOTER_PNG=1 go test -run TestDumpP13EB ./internal/launcher/`
func TestDumpP13EBHintPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-eb-hint-<appearance>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := hintApp()
		a.Query = "clip"
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 240, int(WindowHeight("small")))
		if dark {
			tt.SetDark(true)
			tt.Advance(500 * time.Millisecond)
		}
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-eb-hint-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s: image %v", path, tt.Image().Bounds())
	}
}
