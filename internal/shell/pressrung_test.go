package shell

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-al · 指针的第三档落在 shell 的四个键上（`docs/rounds/P13-al-spec.md`）。
//
// 插件窗口 bar 的两个键（`detach.go` 的重跑键 / 关闭键）、便条窗口的 ✕
// （`pinned.go`）、toast 的 ✕ 与动作胶囊（`notices.go`）此前只有静息与悬停两档：
// 指针**按下去**的那一帧与悬停帧逐像素相同。P13-q 在设置页抓到同一个洞并补上了
// 第三档（面沉到 `ControlPress`、几何收一档），P13-y / P13-z 把这几枚键的**悬停**
// 面收成 E1 的一对面，但没有一轮把「按下」带到这一族来。§5 把这一档写死为
// `--glass-control-press` 的面加一次几何收档。
//
// 三条判据（深/浅两套外观各跑一遍，全部量像素）：
//
//	① 悬停真的画了面（悬停面 ≠ bar 自己的面）；
//	② 按下真的画了**另一个**面（按下面 ≠ 悬停面）——旧实现在这一条上逐像素相同，
//	   这是本轮的根因；
//	③ 按下的面在**静息与悬停之间**（第三档是「往静息的回退」，不是第三种颜色）；
//	④ 按下的盒子比悬停的窄（`Scale(0.92)` 是渲染变换，布局盒不动：量的就是
//	   「画出来的盒」）。
//
// 出图：`/tmp/floter-P13-al-<surface>-<light|dark>-<hover|press>.png`。

// pressSurface is one of the keys this round gives a third rung.
type pressSurface struct {
	name   string
	render func(t *testing.T, dark bool) *ui.Tester // 已经画好一帧
	label  string                                   // 键的无障碍名
	probe  int                                      // 从键盒子左边缘进去几 DIP 取列（字/图标之外）
}

// pressSurfaces are the three keys a test frame can reach: the two plugin-window
// bar keys and the toast's ✕ are the same shape (`notices.go` draws the same ✕
// on the same 24u key), the toast ✕ is covered by `internal/ci`'s arm census
// because a toast needs the whole app to render.
func pressSurfaces() []pressSurface {
	copy := i18n.For("en").Launcher
	return []pressSurface{
		{
			name: "output-rerun",
			render: func(t *testing.T, dark bool) *ui.Tester {
				tt, _ := barDetached(t, dark, false)
				return tt
			},
			label: copy.OutputWindowRerun,
			probe: 6,
		},
		{
			name: "output-close",
			render: func(t *testing.T, dark bool) *ui.Tester {
				tt, _ := barDetached(t, dark, false)
				return tt
			},
			label: copy.OutputWindowClose,
			probe: 4,
		},
		{
			name: "pinned-close",
			render: func(t *testing.T, dark bool) *ui.Tester {
				return barPinned(t, dark)
			},
			label: copy.OutputWindowClose,
			probe: 4,
		},
	}
}

// pressReading parks the pointer on one key and reads the pixel it paints there:
// `down` holds the button, `false` only hovers. The column is the key's own
// padding, clear of the glyph, so the pixel read is the material and not a
// stroke.
func pressReading(t *testing.T, surface pressSurface, dark, down bool) (ui.Color, ui.Rect) {
	t.Helper()
	tt := surface.render(t, dark)
	box := barKeyBox(t, tt, surface.label)
	x, y := int(box.X)+surface.probe, int(box.Y+box.H/2)
	tt.Move(float32(x), float32(y))
	tt.Frame()
	if down {
		tt.Press(float32(x), float32(y))
		// These keys carry no colour transition (`Transition` is not set on
		// them), so the frame after the press is already the settled press
		// face; the second frame only proves it is a state, not a one-frame
		// artefact of the dispatch.
		tt.Frame()
		tt.Frame()
	}
	return shellPixelAt(tt.Image(), x, y), box
}

// pressRest is the same pixel with no pointer on the bar: the ground the two
// faces are graded against.
func pressRest(t *testing.T, surface pressSurface, dark bool) ui.Color {
	t.Helper()
	tt := surface.render(t, dark)
	box := barKeyBox(t, tt, surface.label)
	x, y := int(box.X)+surface.probe, int(box.Y+box.H/2)
	tt.Move(4, 4) // the pointer starts here; the bar is at the top edge, so this is beside it
	tt.Frame()
	return shellPixelAt(tt.Image(), x, y)
}

// TestTheShellKeysSinkWhenThePointerGoesDown is the round's pin: ② and ③.
func TestTheShellKeysSinkWhenThePointerGoesDown(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, surface := range pressSurfaces() {
			rest := pressRest(t, surface, dark)
			hover, _ := pressReading(t, surface, dark, false)
			pressed, _ := pressReading(t, surface, dark, true)
			t.Logf("%s/%s: rest %v hover %v pressed %v", appearance, surface.name, rest, hover, pressed)
			if barChannelDelta(hover, rest) < 4 {
				t.Errorf("%s/%s: hovered face %v is the resting face %v — the hover arm painted nothing, so "+
					"this pin is grading the bar, not the key", appearance, surface.name, hover, rest)
				continue
			}
			if barChannelDelta(pressed, hover) < 2 {
				t.Errorf("%s/%s: the key under the pointer is %v whether or not the button is down (rest %v): "+
					"the press rung paints no face of its own — `--glass-control-press` is one step back toward "+
					"the resting face (docs/liquid-glass.md §5)", appearance, surface.name, pressed, rest)
				continue
			}
			// The rung is the ladder stepping *back down*: the pressed face sits
			// between the resting face and the hovered one, whichever way this
			// appearance's ladder runs (light: a weaker black wash, so lighter;
			// dark: a weaker white wash, so darker).
			if !barBetween(rest, pressed, hover) {
				t.Errorf("%s/%s: the pressed face %v is not between the resting %v and the hovered %v — the "+
					"third rung is a step back toward rest, not a third colour",
					appearance, surface.name, pressed, rest, hover)
			}
		}
	}
}

// TestTheShellKeysSinkTheirBox is ④: the press shrinks the painted key by the
// family's 0.92 while the layout box stays where it was — `Find` answers with
// the layout box, so a renderer that moved the key instead of scaling it would
// move the box under this pin.
func TestTheShellKeysSinkTheirBox(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, surface := range pressSurfaces() {
			hover := pressPaintedWidth(t, surface, dark, false)
			pressed := pressPaintedWidth(t, surface, dark, true)
			if hover <= 0 {
				t.Fatalf("%s/%s: the hovered key painted nothing along its own row", appearance, surface.name)
			}
			// 0.92 of the box, measured in whole DIPs along one row: demand a
			// visible narrowing (at least two pixels) rather than the exact
			// ratio, which a 28u box rounds away.
			if pressed > hover-2 {
				t.Errorf("%s/%s: the key paints %d DIP wide hovered and %d pressed — the press does not step the "+
					"box back (0.92 is the family's number, P13-q's icon keys)",
					appearance, surface.name, hover, pressed)
			}
		}
	}
}

// pressPaintedWidth counts the pixels along the key's middle row that are not
// the bar's own face: the key's painted box, read off the frame.
func pressPaintedWidth(t *testing.T, surface pressSurface, dark, down bool) int {
	t.Helper()
	tt := surface.render(t, dark)
	box := barKeyBox(t, tt, surface.label)
	x, y := int(box.X)+surface.probe, int(box.Y+box.H/2)
	// The bar's own face, taken well left of the key (the re-run key starts the
	// trailing group, so this is the bar's empty run).
	barFace := shellPixelAt(tt.Image(), int(box.X)-8, y)
	tt.Move(float32(x), float32(y))
	tt.Frame()
	if down {
		tt.Press(float32(x), float32(y))
		tt.Frame()
	}
	img := tt.Image()
	width := 0
	for px := int(box.X) - 8; px <= int(box.X+box.W)+8; px++ {
		if shellColorDistance(shellPixelAt(img, px, y), barFace) > 40 {
			width++
		}
	}
	return width
}

// barBetween reports whether mid's luma sits strictly between a and b.
func barBetween(a, mid, b ui.Color) bool {
	lo, hi := barLuma(a), barLuma(b)
	if lo > hi {
		lo, hi = hi, lo
	}
	return barLuma(mid) > lo && barLuma(mid) < hi
}

// TestDumpP13ALPressRungs is the opt-in frame dump: the same three keys, hovered
// and held.
func TestDumpP13ALPressRungs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") != "1" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-al-<surface>-<light|dark>-<hover|press>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, surface := range pressSurfaces() {
			for _, down := range []bool{false, true} {
				state := "hover"
				if down {
					state = "press"
				}
				tt := surface.render(t, dark)
				box := barKeyBox(t, tt, surface.label)
				x, y := int(box.X)+surface.probe, int(box.Y+box.H/2)
				tt.Move(float32(x), float32(y))
				tt.Frame()
				if down {
					tt.Press(float32(x), float32(y))
					tt.Frame()
					tt.Frame()
				}
				path := filepath.Join("/tmp", "floter-P13-al-"+surface.name+"-"+appearance+"-"+state+".png")
				writePressPNG(t, tt.Image(), path)
			}
		}
	}
}

// writePressPNG writes one frame.
func writePressPNG(t *testing.T, img image.Image, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("could not write %s: %v", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("could not encode %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
}
