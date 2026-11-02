package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// P13-ec · **默认取景不许和出货取景分家。**
//
// 这一包以前有两条取景：`render()`（框架库存主题，`Spacing 4.0` / `FontSize 14`）与
// `renderShipped()`（这一面自己的调色板，3.6 / 10.8）。两条并存的代价是 180 个调用点
// 量的是用户看不到的帧，而且「像素断言全绿」并不等于「量的是这个设计」。
//
// 这条 pin 钉的是那个连接处：`render()` 画出来的帧，**高度**就是出货尺的帧高
// （`themedFrameHeight`，439），并且字段块那条发丝落在出货尺的那一行（49 = 14u × 3.6 − 1
// 取整）。两把尺子的帧高必须不相等，否则这条 pin 会在「两边都是 488」里空过。
func TestTheDefaultFixtureDrawsTheShippedFrame(t *testing.T) {
	a := testApp()
	recentPage(a, 3)
	tt := render(t, a)
	if got, want := tt.Image().Bounds().Dy(), themedFrameHeight(maxResultSlots); got != want {
		t.Errorf("the default fixture draws a %dpx frame, want the shipped ruler's %dpx "+
			"(the stock ruler charges %dpx)", got, want, frameHeight(maxResultSlots))
	}
	if themedFrameHeight(maxResultSlots) == frameHeight(maxResultSlots) {
		t.Fatalf("the two rulers charge the same frame (%dpx): this pin cannot tell them apart",
			themedFrameHeight(maxResultSlots))
	}
	hairline := hairlineAbove(tt.Image(), int(a.listRect.Y))
	t.Logf("clip y=%v", a.listRect.Y)
	if want := 49.0; hairline != want {
		t.Errorf("the field block's hairline is at y=%v, want %v on the shipped ruler "+
			"(14u × 3.6 − 1, snapped to the painter's row); the stock ruler puts it at 55",
			hairline, want)
	}
}

// TestDumpP13ECRulersPNGs is this round's visual evidence: the same launcher frame on both
// rulers, both appearances. The left column of the diff is what the fixtures used to hold.
//
//	FLOTER_PNG=1 go test -run TestDumpP13ECRulers ./internal/launcher/
func TestDumpP13ECRulersPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-ec-<ruler>-<appearance>.png")
	}
	for _, ruler := range []struct {
		name    string
		shipped bool
	}{{"stock", false}, {"shipped", true}} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			// One App per frame: a single app bound in two windows cannot focus
			// (`ui: Handle.Focus needs a Context when bound in several windows`).
			a := testApp()
			recentPage(a, 3)
			height := frameHeight(maxResultSlots)
			build := func(c *ui.Context) { a.View(c) }
			if ruler.shipped {
				height = themedFrameHeight(maxResultSlots)
				build = func(c *ui.Context) {
					c.SetTheme(a.tokens(c).Theme)
					a.View(c)
				}
			}
			tt := ui.NewTester(build, InputWindowWidth, height)
			if dark {
				tt.SetDark(true)
				tt.Advance(500 * time.Millisecond)
			}
			tt.Frame()
			path := filepath.Join("/tmp", "floter-P13-ec-"+ruler.name+"-"+appearance+".png")
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
}
