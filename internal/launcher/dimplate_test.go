package launcher

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-bb · 捕获输出页那一行「不是门」的板，不该戴着受光边。
//
// `row.Status` / `row.Disabled` 的行在捕获输出页里是**注释**，不是可以打开的门
// （`output.go` 的 `case row.Status || row.Disabled:` 不抬面、不 hover）。可它前面的
// 28u 图标板一直是 `iconPlate` 画出来的，而那块板从头到尾戴着 `LitFace` 的一对线
// ——顶边 `LensRim` 的受光边是「这块面可以被碰」的记号（`docs/liquid-glass.md` §3），
// 于是整行里唯一还亮着灯的东西恰好是那一小格，读起来像「这行可以点，只是字淡了」。
// 同族的 P13-s / P13-ar / P13-aq / P13-ba 已经把这盏灯从禁用胶囊、不可用的卸载卡、
// accent 的 Save、行忙那一簇键上摘掉了；这一轮是启动器侧最后一个亮着的地方。
//
// 量的是**这一块板自己的两行像素**：顶内那行（受光边落在这里，`InsetShadow(0,1,0,0)`
// 从 padding edge 算起，`spread=0` 正好占第一行）与它下面第三行的面。0.62 那道淡是
// 参考 CSS 自己的 dim 档（`spanStyle` 的 `opacity: 0.62`，同一个数），两臂走同一条
// 淡，所以差只可能来自那条边。
const dimPlateFade = 0.62

// dimPlateProbe renders one `iconPlate` on a fixed ground and returns the tester
// plus the plate's box on the last frame.
func dimPlateProbe(t *testing.T, dark, dim bool) (*ui.Tester, ui.Rect) {
	t.Helper()
	a := testApp()
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		if dark {
			c.SetTheme(ui.DarkTheme())
		} else {
			c.SetTheme(ui.LightTheme())
		}
		ground := ui.RGB(28, 29, 33)
		if !dark {
			ground = ui.RGB(247, 247, 248)
		}
		ui.Column(c).FillWidth().FillHeight().Background(ground).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			plate := a.iconPlate(c, false, dim)
			if dim {
				// exactly what `outputBody` does to a note row's plate
				plate.Opacity(dimPlateFade)
			}
			box = plate.Bounds()
		})
	}, 120, 80)
	tt.Frame()
	tt.Frame()
	return tt, box
}

func platePixel(tt *ui.Tester, x, y float32) color.RGBA {
	img := tt.Image()
	i := img.PixOffset(int(x), int(y))
	return color.RGBA{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// The pin: on the plate the pointer can take, the top inner row is *lighter* than
// the face by at least one level (the lit edge); on a note's plate the same two
// rows are the same colour, and the counter-side line at the bottom is still there
// (the plate is a face, it is just not a door).
func TestANoteRowsPlateDoesNotWearTheLitEdge(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		liveTT, live := dimPlateProbe(t, dark, false)
		dimTT, dim := dimPlateProbe(t, dark, true)
		if live.W == 0 || dim.W == 0 {
			t.Fatalf("a plate did not get a box: live %v dim %v", live, dim)
		}
		read := func(tt *ui.Tester, box ui.Rect) (crown, face, bottom color.RGBA) {
			// The plate is located by scanning: `Bounds()` is the previous
			// frame's border box and the column centres it on a sub-pixel
			// origin, so the drawn face starts a row below the box's Y.
			// centre column: the plate wears a `RadiusXS` corner, so a column two
			// pixels in reads the corner's anti-aliasing on the top row rather
			// than the edge itself (the glyph sits from +6, so +3 is still face).
			x := box.X + box.W/2
			ground := platePixel(tt, x, 0)
			top, last := -1, -1
			for y := int(box.Y) - 3; y <= int(box.Y+box.H)+3; y++ {
				if y < 0 || y >= int(tt.Image().Rect.Dy()) {
					continue
				}
				if platePixel(tt, x, float32(y)) != ground {
					if top < 0 {
						top = y
					}
					last = y
				}
			}
			return platePixel(tt, x, float32(top)), platePixel(tt, x, float32(top+3)), platePixel(tt, x, float32(last))
		}
		liveCrown, liveFace, liveBottom := read(liveTT, live)
		dimCrown, dimFace, dimBottom := read(dimTT, dim)
		t.Logf("live: crown %v face %v bottom %v", liveCrown, liveFace, liveBottom)
		t.Logf("dim:  crown %v face %v bottom %v", dimCrown, dimFace, dimBottom)
		level := func(c color.RGBA) int { return int(c.R) + int(c.G) + int(c.B) }
		if level(liveCrown) <= level(liveFace) {
			t.Errorf("the live plate lost its lit edge: crown %v is not over the face %v", liveCrown, liveFace)
		}
		if level(dimCrown) != level(dimFace) {
			t.Errorf("the note's plate kept a lit edge: crown %v vs face %v (Δ %d)",
				dimCrown, dimFace, level(dimCrown)-level(dimFace))
		}
		if level(dimBottom) == level(dimFace) {
			t.Errorf("the note's plate lost its counter-side line too: bottom %v == face %v", dimBottom, dimFace)
		}
		dumpDimPlate(t, dark, liveTT, dimTT)
	})
}

// dumpDimPlate writes the two plates side by side for a human eye
// (opt-in: `FLOTER_P13BB_DUMP=1`).
func dumpDimPlate(t *testing.T, dark bool, liveTT, dimTT *ui.Tester) {
	t.Helper()
	if os.Getenv("FLOTER_P13BB_DUMP") == "" {
		return
	}
	live, dim := liveTT.Image(), dimTT.Image()
	b := live.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()*2+b.Dx()/6, b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.Set(x, y, live.At(b.Min.X+x, b.Min.Y+y))
			out.Set(x+b.Dx()+b.Dx()/6, y, dim.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	path := fmt.Sprintf("/tmp/floter-P13-bb-dimplate-%s.png", appearance)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}
