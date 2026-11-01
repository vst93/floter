package shell

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/launcher"
)

// P13-bp · 插件窗口列表臂的行盘：同一张列表，两处画法。
//
// `docs/liquid-glass.md` §2 把 E1 写死成「面 + 顶受光边 + 底对侧暗线」，
// §6.1 的 1-2 / 1-8、§6.2 的 2-3、§6.2b-2 的 9-16 把启动器的结果行、28u 图标板、
// 设置页的分组卡与命令块一个个收进这条判据。`shell/detach.go` 的 `detachedList`
// 这块行盘（`plugin-window__row`）是同级里唯一没进过账的：它在 `tokens.Control`
// 面上直接 `Children`，**一条线都没有**——同一份数据在启动器列表里有 E1 的一对线，
// 在这个窗口里是一块平涂。
//
// 这一族此前看不见，是因为两代 census 的判据都是「面 **+ 边**」：
// `facecensus_test.go`（P13-p）要 `Border(`/`InsetShadow(` 同时在，
// `litarm_contract_test.go`（P13-ab）只扫 `Hovered()`/`Pressed()` 臂——而这行盘
// 既没有边框，也没有指针臂（列表不可选）。它是两张账本的共同盲区。
//
// 本轮的 pin 就是 §2 判据落到这块面上：顶内线是一条受光边、底内线是一条暗线、
// 两侧回到面本身。候选线不写死像素，两条线都不是渲染器常量。

// plugindowPlate is what the pin reads off one row plate.
type plugindowPlate struct {
	ground, crown, face, shade, left, right ui.Color
}

// plugindowSum is a pixel's brightness: the three channels added, the same
// reading the other lit-edge pins use.
func plugindowSum(c ui.Color) int { return int(c.R) + int(c.G) + int(c.B) }

// plugindowAppearance names an appearance the way the dumps do.
func plugindowAppearance(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// plugindowView is the emitted list the window's list arm draws: two rows, the
// second without a subtitle.
func plugindowView() launcher.OutputView {
	var alpha, beta extensions.Row
	alpha.Title, alpha.Subtitle = "Alpha", "one"
	beta.Title = "Beta"
	var view launcher.OutputView
	view.Title = "jv"
	view.IsList = true
	view.Rows = []extensions.Row{alpha, beta}
	return view
}

// plugindowFrame renders the window with that list and returns the snapshot.
func plugindowFrame(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	a := barApp(t, dark)
	view := plugindowView()
	detached := &detachedOutput{}
	detached.title, detached.ran, detached.view = "jv", true, view
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.SetDark(dark)
	tt.Frame()
	return tt
}

// plugindowBounds finds the plate's own rectangle from the frame: a list plate
// is the one region whose fill is the control face, so the scan walks out from
// the title's box to where that fill returns to the window's ground. The
// geometry is read, never assumed — the probe must not depend on the padding
// constants it is meant to leave alone.
func plugindowBounds(t *testing.T, tt *ui.Tester, title ui.Rect) ui.Rect {
	t.Helper()
	img := tt.Image()
	cx := int(title.X + title.W/2)
	cy := int(title.Y + 4)
	ground := shellPixelAt(img, cx, int(title.Y)-24)
	same := func(c ui.Color) bool {
		return plugindowSum(c)-plugindowSum(ground) < 6 && plugindowSum(c)-plugindowSum(ground) > -6
	}
	top := cy
	for top > 0 && !same(shellPixelAt(img, cx, top-1)) {
		top--
	}
	bottom := cy
	for bottom < img.Rect.Dy()-1 && !same(shellPixelAt(img, cx, bottom+1)) {
		bottom++
	}
	left := int(title.X)
	for left > 0 && !same(shellPixelAt(img, left-1, cy)) {
		left--
	}
	right := int(title.X + title.W)
	for right < img.Rect.Dx()-1 && !same(shellPixelAt(img, right+1, cy)) {
		right++
	}
	var box ui.Rect
	box.X, box.Y = float32(left), float32(top)
	box.W, box.H = float32(right-left+1), float32(bottom-top+1)
	return box
}

// plugindowListProbe renders the window's list arm and returns the first row's
// plate box and the readings the pin uses.
func plugindowListProbe(t *testing.T, dark bool) (ui.Rect, plugindowPlate) {
	t.Helper()
	tt := plugindowFrame(t, dark)
	title, ok := tt.Find("Alpha")
	if !ok {
		t.Fatalf("the list row is missing: %q", tt.Texts())
	}
	plate := plugindowBounds(t, tt, title)
	img := tt.Image()
	at := func(x, y int) ui.Color { return shellPixelAt(img, x, y) }
	cx := int(plate.X + plate.W/2)
	var r plugindowPlate
	r.ground = at(cx, int(plate.Y)-12)
	r.crown = at(cx, int(plate.Y))
	for dy := 0; dy <= 2; dy++ {
		if q := at(cx, int(plate.Y)+dy); plugindowSum(q) > plugindowSum(r.crown) {
			r.crown = q
		}
	}
	r.face = at(cx, int(plate.Y)+4)
	r.shade = at(cx, int(plate.Y+plate.H)-1)
	for dy := -3; dy <= 0; dy++ {
		if q := at(cx, int(plate.Y+plate.H)+dy); plugindowSum(q) < plugindowSum(r.shade) {
			r.shade = q
		}
	}
	r.left = at(int(plate.X), int(plate.Y+plate.H/2))
	r.right = at(int(plate.X+plate.W)-1, int(plate.Y+plate.H/2))
	return plate, r
}

// TestThePluginWindowListRowWearsTheE1Pair is the round's verdict: a list plate
// is an E1 face — one lit edge on top and one counter-side shade at the bottom.
func TestThePluginWindowListRowWearsTheE1Pair(t *testing.T) {
	for _, dark := range []bool{false, true} {
		plate, r := plugindowListProbe(t, dark)
		name := plugindowAppearance(dark)
		if plate.W < 40 || plate.H < 20 {
			t.Fatalf("%s: measured a %v×%v plate, not a row", name, plate.W, plate.H)
		}
		f := plugindowSum(r.face)
		if dark {
			if plugindowSum(r.crown) < f {
				t.Errorf("%s: the plate's top inner row is a shade, not a lit edge: %v vs face %v",
					name, r.crown, r.face)
			}
		} else if plugindowSum(r.crown) <= f {
			t.Errorf("%s: the plate's top inner row is not lighter than the face: %v vs face %v (want the 0.09 rim)",
				name, r.crown, r.face)
		}
		if drop := f - plugindowSum(r.shade); drop < 9 {
			t.Errorf("%s: the plate's bottom inner row is only %d levels under the face (%v vs %v); want the counter-side shade",
				name, drop/3, r.shade, r.face)
		}
		// Neither side is a line: the pair is top and bottom only.
		if d := plugindowSum(r.left) - f; d > 6 || d < -6 {
			t.Errorf("%s: the plate's left column carries a line (%v vs face %v)", name, r.left, r.face)
		}
		if d := plugindowSum(r.right) - f; d > 6 || d < -6 {
			t.Errorf("%s: the plate's right column carries a line (%v vs face %v)", name, r.right, r.face)
		}
		// The face still steps off the window's own ground: the pair must not
		// have been bought by flattening the plate. The direction flips with the
		// appearance (the light plate is darker than its ground, the dark one
		// lighter), so the step is read as a magnitude.
		step := f - plugindowSum(r.ground)
		if step < 0 {
			step = -step
		}
		if step < 18 {
			t.Errorf("%s: the plate only steps %d levels off the window ground; want a face",
				name, step/3)
		}
	}
}

// TestDumpThePluginWindowListPlate writes both appearances for the report.
// With FLOTER_P13BP_TAG=before and the pair re-injected out of `detachedList` it
// produces the comparison pair.
func TestDumpThePluginWindowListPlate(t *testing.T) {
	if os.Getenv("FLOTER_P13BP_DUMP") == "" {
		t.Skip("set FLOTER_P13BP_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13BP_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		plate, r := plugindowListProbe(t, dark)
		t.Logf("%s (%s): plate %v ground %v crown %v face %v shade %v left %v right %v",
			plugindowAppearance(dark), tag, plate, r.ground, r.crown, r.face, r.shade, r.left, r.right)
		tt := plugindowFrame(t, dark)
		img := tt.Image()
		path := filepath.Join(os.TempDir(), "floter-P13-bp-windowlist-"+plugindowAppearance(dark)+"-"+tag+".png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
}
