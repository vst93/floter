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

// P13-ed · **外观取景（`renderIn`）也不许和出货取景分家。**
//
// `render()` 在 P13-ec 换到出货尺之后，这一包还剩第二条库存尺：`renderIn` 是 17 个文件、
// 40 个调用点用的「两套外观各画一帧」取景，它仍装着框架的库存主题（4.0 / 14）。于是那一族
// 的像素断言量的是用户看不到的帧——`gutter` 读 8 而 app 画 7、悬停小格的盒子落在半像素上、
// 电源行的图标板取样落在板外一个像素。
//
// 这条 pin 钉住连接处：`renderIn` 的帧高 = 出货尺的帧高，字段块发丝在 49，行块左沟在 7。
// 库存尺把这三个数写成 488 / 55 / 8，所以这条 pin 能分辨两把尺子。
func TestTheAppearanceFixtureDrawsTheShippedFrame(t *testing.T) {
	if got, want := themedFrameHeight(maxResultSlots), frameHeight(maxResultSlots); got == want {
		t.Fatalf("the two rulers charge the same frame (%dpx): this pin cannot tell them apart", got)
	}
	forEachAppearance(t, func(t *testing.T, dark bool) {
		a := testApp()
		recentPage(a, 3)
		tt := renderIn(t, a, dark)
		if got, want := tt.Image().Bounds().Dy(), themedFrameHeight(maxResultSlots); got != want {
			t.Errorf("the appearance fixture draws a %dpx frame, want the shipped ruler's %dpx "+
				"(the stock ruler charges %dpx)", got, want, frameHeight(maxResultSlots))
		}
		img := tt.Image()
		if got, want := hairlineAbove(img, int(a.listRect.Y)), 49.0; got != want {
			t.Errorf("the field block's hairline is at y=%v, want %v on the shipped ruler "+
				"(the stock ruler puts it at 55)", got, want)
		}
		row, ok := tt.Find("Recent 00")
		if !ok {
			t.Fatalf("the first row was not drawn: %q", tt.Texts())
		}
		panel := img.RGBAAt(1, int(row.Y+row.H/2))
		left := -1
		for x := 0; x < InputWindowWidth; x++ {
			if img.RGBAAt(x, int(row.Y+row.H/2)) != panel {
				left = x
				break
			}
		}
		if want := 7; left != want {
			t.Errorf("the row block starts at x=%d, want %d (8u × 0.9 = 7.20 on the shipped "+
				"ruler; the stock ruler paints 8)", left, want)
		}
	})
}

// TestDumpP13EDAppearancePNGs is this round's visual evidence: the appearance
// fixture's frame on both rulers, both appearances, on the clipboard page (the
// surface most of the `renderIn` pins read).
//
//	FLOTER_PNG=1 go test -run TestDumpP13EDAppearance ./internal/launcher/
func TestDumpP13EDAppearancePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-ed-<ruler>-<appearance>.png")
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
			a := testApp()
			a.EnterClipboard()
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
			path := filepath.Join("/tmp", "floter-P13-ed-"+ruler.name+"-"+appearance+".png")
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

// fixtureWindowSizes is the two windows the explicit-window audits draw at:
// the shipped card, and the shortest the window opens. They live here, beside
// the pin that checks them, so the audits cannot drift back to a frame the app
// never draws (`frameHeight`'s 488) while the pin still reads the shipped 439.
var fixtureWindowSizes = [][2]int{
	{InputWindowWidth, themedFrameHeight(maxResultSlots)},
	{InputWindowWidth, 360}, // settings.MinTerminalHeight, the shortest the window opens
}

// P13-ee · **显式开窗那一族（`renderAt`）装的也是出货主题。**
//
// 这一族与 `render()` / `renderIn()` 的差别只在**帧的尺寸由调用点给**（它把窗口本身当被测
// 对象：默认卡、以及窗口能缩到的最小高度 360），帧里的**墨**没有理由用第二个调色板。此前
// `renderAt` 不装主题，于是 13 个调用点量的是框架库存主题画的字与间距（4.0 / 14），而帧高
// 又按 `frameHeight` 记成 488 —— 一张用户看不到的卡。
//
// 这条 pin 钉住那个连接处，并且**两档窗口高度都钉**：发丝在 49（库存尺 55）、行块左沟在
// 7（库存尺 8）。帧高由调用点给，所以这里同时确认「给了多少就画多少」，不让一次换尺悄悄
// 变成换窗。
func TestTheExplicitWindowFixtureDrawsTheShippedInks(t *testing.T) {
	if got, want := themedFrameHeight(maxResultSlots), frameHeight(maxResultSlots); got == want {
		t.Fatalf("the two rulers charge the same frame (%dpx): this pin cannot tell them apart", got)
	}
	if got, want := fixtureWindowSizes[0][1], themedFrameHeight(maxResultSlots); got != want {
		t.Errorf("the explicit-window audits draw at %dpx, want the shipped frame %dpx "+
			"(the stock ruler charges %dpx)", got, want, frameHeight(maxResultSlots))
	}
	for _, size := range fixtureWindowSizes {
		a := clipboardModeApp(t)
		tt := renderAt(t, a, size[0], size[1], false)
		if got := tt.Image().Bounds().Dy(); got != size[1] {
			t.Errorf("%dx%d: the fixture drew a %dpx frame — the caller's height is the frame",
				size[0], size[1], got)
		}
		img := tt.Image()
		if got, want := hairlineAbove(img, int(a.listRect.Y)), 49.0; got != want {
			t.Errorf("%dx%d: the field block's hairline is at y=%v, want %v on the shipped ruler "+
				"(the stock theme puts it at 55)", size[0], size[1], got, want)
		}
		// The selected row's own face is the row block: on the plain page the
		// card is the frame and the block starts one gutter inside it. The
		// clipboard page has no selection at rest, so its card is one face all
		// the way across and there is no edge to read — the hairline above is
		// what tells the two rulers apart there.
		plain := testApp()
		recentPage(plain, 3)
		pt := renderAt(t, plain, size[0], size[1], false)
		pimg := pt.Image()
		row, ok := pt.Find("Recent 00")
		if !ok {
			t.Fatalf("%dx%d: the first row was not drawn: %q", size[0], size[1], pt.Texts())
		}
		mid := int(row.Y + row.H/2)
		panel := pimg.RGBAAt(1, mid)
		left := -1
		for x := 0; x < size[0]; x++ {
			if pimg.RGBAAt(x, mid) != panel {
				left = x
				break
			}
		}
		if want := 7; left != want {
			t.Errorf("%dx%d: the selected row's block starts at x=%d, want %d (8u × 0.9 = 7.20 "+
				"on the shipped ruler; the stock theme paints 8)", size[0], size[1], left, want)
		}
	}
}

// TestDumpP13EEWindowFixturePNGs is this round's visual evidence: the
// explicit-window fixture's card on both rulers, both appearances, on the
// clipboard page (the surface most of the `renderAt` call sites audit).
//
//	FLOTER_PNG=1 go test -run TestDumpP13EEWindowFixture ./internal/launcher/
func TestDumpP13EEWindowFixturePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-ee-<ruler>-<appearance>.png")
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
			a := clipboardModeApp(t)
			height := frameHeight(maxResultSlots)
			if ruler.shipped {
				height = themedFrameHeight(maxResultSlots)
			}
			var tt *ui.Tester
			if ruler.shipped {
				tt = renderAt(t, a, InputWindowWidth, height, dark)
			} else {
				tt = ui.NewTester(func(c *ui.Context) { a.View(c) }, InputWindowWidth, height)
				tt.SetDark(dark)
				tt.Frame()
				tt.Frame()
			}
			path := filepath.Join("/tmp", "floter-P13-ee-"+ruler.name+"-"+appearance+".png")
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
