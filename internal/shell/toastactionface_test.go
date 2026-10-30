package shell

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-j · toast 的动作 chip（`.app-toast__action`）：`docs/liquid-glass.md` §6.6
// 第 6-3 行，全仓 `Lens` 的最后一个「未开始」调用点（另一个是
// `control.SegmentedIndex`，9-11 判为 `故意不做`——accent 环就是它的身份线）。
//
// 根因与 P13-h / P13-i 同族：`theme.Tokens.Lens` 的四层按 rim → sheen → edge →
// 四边 `ControlEdge` 环追加，环画在最后、且与 `y=1` 的 rim 落在同一个像素行
// （mygo 的 `insetShadow` 把 hole 从 padding edge 算起）。于是 P12-a 抬到 0.09 的
// 受光边在这枚 chip 上**一次都没亮过**：顶内线是环的 `ControlEdge`。
//
// 这一行挂着的问题还有第二个：这枚 chip **没有自己的 `Border`**——环就是它唯一的
// 边界。本轮的答案是「不补边框」：它坐在 toast 的 `Float` 面上，面差本身已经把它
// 分开（下面 ⑤ 量的就是这个数），而补 `Border(1)` 会把 padding box 缩 1px
// （`insetShadow` 从 padding edge 算起），那是几何改动，越出「只改墨与面」的红线，
// 也正是 §1 禁掉的「画框分层」。
//
// 五条判据（每条都在深/浅两套外观上跑，全部量像素、不读注释）：
//  ① 顶内行亮于面（E1 的受光边）；
//  ② 底内行暗于面（E1 的对侧暗线）；
//  ③ 左右两侧的列 = 面（没有环——环会把它们涂成 `ControlEdge`）；
//  ④ chip 外面仍是 toast 的面（E1 不投影，§2 判据 2）；
//  ⑤ chip 的面与 toast 的面至少差 4 级（这就是「不补边框」的全部理由）。

type chipAppearance struct {
	name string
	dark bool
}

var chipAppearances = []chipAppearance{{"light", false}, {"dark", true}}

// chipPixel reads one pixel of the frame.
func chipPixel(img *image.RGBA, x, y int) ui.Color {
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		return ui.Color{}
	}
	p := img.RGBAAt(x, y)
	return ui.RGB(p.R, p.G, p.B)
}

// chipMean is the mean colour of one rectangle, corners included: every reading
// here is a band, so a 1px line that lands a row off still lands inside it.
func chipMean(img *image.RGBA, x0, y0, x1, y1 int) ui.Color {
	var r, g, b, n int
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			p := chipPixel(img, x, y)
			r, g, b, n = r+int(p.R), g+int(p.G), b+int(p.B), n+1
		}
	}
	if n == 0 {
		return ui.Color{}
	}
	return ui.RGB(uint8(r/n), uint8(g/n), uint8(b/n))
}

// chipDelta is the largest per channel difference between two readings.
func chipDelta(a, b ui.Color) int {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return max(d(a.R, b.R), max(d(a.G, b.G), d(a.B, b.B)))
}

// chipBrighter / chipDarker compare two readings by summed luminance, which is
// what "一分亮" means on a nearly neutral face.
func chipBrighter(a, b ui.Color) bool {
	return int(a.R)+int(a.G)+int(a.B) > int(b.R)+int(b.G)+int(b.B)
}

func chipDarker(a, b ui.Color) bool { return chipBrighter(b, a) }

// toastActionShot renders one settled toast carrying an action chip on the
// full-height arm (the settings page, where the pane hangs from the top right)
// and returns the frozen frame plus the chip's box.
//
// The box comes from `ui.Tester.Find`: the chip is a `Shrink(0)` row, so its
// own box *is* the box the label is found at (it grows to its text plus 2u of
// padding). `chipBox` re-checks that against the frame before anything is
// graded.
func toastActionShot(t *testing.T, dark bool) (*image.RGBA, ui.Rect) {
	t.Helper()
	store := settings.NewStore(settings.Default())
	a := New(Options{Store: store})
	now := time.Unix(0, 0)
	a.toastClock = func() time.Time { return now }
	a.Open(SurfaceSettings)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()

	a.noticeMu.Lock()
	a.notices = append(a.notices, ui.Toast{
		ID:      "floter-retry",
		Title:   "Plugin page failed",
		Type:    "error",
		Action:  "Retry",
		Timeout: refusalNotice,
	})
	a.noticeMu.Unlock()
	tt.Frame()
	// Step past `--dur-4` so the pane has landed: the entrance scales and
	// translates it, and a half-travelled pane is not the material.
	now = now.Add(toastEnter)
	tt.Frame()

	label, ok := tt.Find("Retry")
	if !ok {
		t.Fatalf("the toast's action chip is not drawn: %q", tt.Texts())
	}
	if !tt.HasText(i18n.For("en").Settings.DismissNotice) {
		t.Fatalf("the toast has no dismiss control: %q", tt.Texts())
	}
	return shellFrameCopy(tt.Image()), label
}

// chipBox verifies the box `ui.Tester.Find` reported: the pixel just inside its
// left edge must not be the pane's face, and the one just outside it must be.
// A shifted rect then fails loudly instead of grading the wrong pixels.
func chipBox(t *testing.T, img *image.RGBA, label ui.Rect) ui.Rect {
	t.Helper()
	if label.W < 8 || label.H < 8 {
		t.Fatalf("the chip's box is degenerate: %+v", label)
	}
	mid := int(label.Y + label.H/2)
	inside := chipPixel(img, int(label.X)+2, mid)
	outside := chipPixel(img, int(label.X)-3, mid)
	if chipDelta(inside, outside) < 4 {
		t.Fatalf("the chip's box was not found: the pixel inside its left edge is %v and the one outside is %v "+
			"(label rect %+v)", inside, outside, label)
	}
	return label
}

// TestTheToastActionChipIsAFaceNotARing pins §6.6's 6-3 row.
func TestTheToastActionChipIsAFaceNotARing(t *testing.T) {
	for _, appearance := range chipAppearances {
		t.Run(appearance.name, func(t *testing.T) {
			img, label := toastActionShot(t, appearance.dark)
			box := chipBox(t, img, label)
			mid := int(box.Y + box.H/2)
			// The box the layout resolved is fractional (66.7..86.5 on this
			// frame) and the rasteriser snaps it: the painted rows are 67..86. The
			// crown and shade bands are read off those, not off the floor of the
			// box, or the top band would straddle the pane and the lit row.
			top := int(box.Y + 0.5)
			bottom := int(box.Y+box.H+0.5) - 1

			// The chip's own face: read in its left padding, where no glyph lands.
			face := chipMean(img, int(box.X)+2, mid-3, int(box.X)+5, mid+3)
			crown := chipMean(img, int(box.X)+10, top, int(box.X+box.W)-11, top+1)
			shade := chipMean(img, int(box.X)+10, bottom-1, int(box.X+box.W)-11, bottom)
			// The side bands hug the painted edge on both sides: the box is
			// fractional (622.6..665.0), so the columns are snapped the same way the
			// rasteriser snaps them — otherwise the right band sits a pixel inside
			// the ring and only the left one catches it.
			left := chipMean(img, int(box.X+0.5)+1, mid-4, int(box.X+0.5)+2, mid+4)
			right := chipMean(img, int(box.X+box.W+0.5)-2, mid-4, int(box.X+box.W+0.5)-1, mid+4)
			// The pane the chip sits on, read on the chip's own rows: the pane's
			// face carries a vertical sheen, so a reading taken at the pane's top
			// would be grading the gradient and not the chip's shadow. Near (4px
			// out) and far (24px out) are the same row on a flat pane.
			near := chipMean(img, int(box.X)-4, mid-4, int(box.X)-3, mid+4)
			pane := chipMean(img, int(box.X)-24, mid-4, int(box.X)-23, mid+4)

			t.Logf("[%s] box=%+v face=%v crown=%v shade=%v left=%v right=%v pane=%v near=%v",
				appearance.name, box, face, crown, shade, left, right, pane, near)

			if !chipBrighter(crown, face) {
				t.Errorf("[%s] the chip's top rows inside its frame are %v, not brighter than its face %v — it lost "+
					"its lit edge (liquid-glass.md §1/§2; the four-layer lens's ring painted over it)", appearance.name, crown, face)
			}
			if !chipDarker(shade, face) {
				t.Errorf("[%s] the chip's bottom rows inside its frame are %v, not darker than its face %v — E1 is a "+
					"pair (§3: 顶亮一分、底暗一分)", appearance.name, shade, face)
			}
			for _, side := range []struct {
				name string
				got  ui.Color
			}{{"left", left}, {"right", right}} {
				if chipDelta(side.got, face) > 2 {
					t.Errorf("[%s] the chip's %s columns at mid height are %v but its face is %v — a ring is drawn "+
						"around the block: it is an E1 face, not the four-layer lens", appearance.name, side.name, side.got, face)
				}
			}
			if chipDelta(near, pane) > 4 {
				t.Errorf("[%s] the chip casts a shadow beside itself (%v against the pane 24px out %v): E1 adds none "+
					"(§2 判据 2)", appearance.name, near, pane)
			}
			if step := chipDelta(face, pane); step < 4 {
				t.Errorf("[%s] the chip's face %v is only %d levels off the pane's face %v — the face step is the "+
					"only thing bounding this chip (it has no border by design), so a step this small leaves it "+
					"unbounded (§6.6 第 6-3 行)", appearance.name, face, step, pane)
			}
		})
	}
}

// TestDumpP13JToastActionPNGs writes the before/after sheet: the same chip over
// the same `Float` pane, once wearing the four-layer lens and once wearing E1's
// pair. Opt-in — `FLOTER_PNG=1 go test -run TestDumpP13JToastActionPNGs
// ./internal/shell/`.
func TestDumpP13JToastActionPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the before/after sheet")
	}
	for _, appearance := range chipAppearances {
		appearance := appearance
		t.Run(appearance.name, func(t *testing.T) {
			store := settings.NewStore(settings.Default())
			tokens := theme.ForPreferences(store.Snapshot(), appearance.dark, ui.Preferences{})
			chip := func(c *ui.Context, name string, lens bool) ui.Element {
				row := ui.Row(c).Shrink(0).Height(tokens.Theme.Space(5.5)).Padding(0, tokens.Theme.Space(2)).
					Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
					Background(tokens.Control)
				if lens {
					tokens.Lens(row)
				} else {
					tokens.LitFace(row)
				}
				row.Children(func() {
					ui.Text(c, name).FontSize(tokens.Caption).FontWeight(600).TextColor(tokens.TextStrong)
				})
				return row
			}
			var handles [2]ui.Handle
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				ui.Column(c).FillWidth().FillHeight().Background(tokens.Float).Padding(24).
					Gap(tokens.Theme.Space(4)).AlignItems(ui.Start).Justify(ui.Start).Children(func() {
					chip(c, "Retry · Lens", true).Bind(&handles[0])
					chip(c, "Retry · LitFace", false).Bind(&handles[1])
				})
			}, 320, 180)
			tt.SetDark(appearance.dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			// The sheet's geometry is integral (padding 24, a 5.5u chip, one Space(4)
			// gap), so the two chips' rows are known without asking the layout back:
			// x starts at 24, the first chip's top is 24, the second's is one chip
			// plus one gap lower. The face is read in the chip's left padding and the
			// crown across its middle, where no glyph lands.
			h := tokens.Theme.Space(5.5)
			gap := tokens.Theme.Space(4)
			for i, name := range []string{"Retry · Lens", "Retry · LitFace"} {
				top := int(24 + float32(i)*(h+gap) + 0.5)
				mid := top + int(h/2)
				t.Logf("[%s] %s: face=%v crown=%v left=%v", appearance.name, name,
					chipMean(tt.Image(), 26, mid-3, 29, mid+3),
					chipMean(tt.Image(), 34, top, 60, top+1),
					chipMean(tt.Image(), 25, mid-4, 26, mid+4))
			}
			path := "/tmp/floter-P13-j-chip-" + appearance.name + ".png"
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", path)
		})
	}
}
