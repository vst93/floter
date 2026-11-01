package launcher

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-cp · 捕获输出页的便条行：图标板上叠了**两道淡**。
//
// `iconPlate(c, false, dim)` 在 `dim` 那一臂已经换掉材质——`DimFace`：面还在、
// 底对侧那条 `LensEdge` 还在，**受光边熄了**（`docs/liquid-glass.md` §3：受光边是
// 「这块面可以被碰」的记号，便条碰不动）。这是「一道淡」。
//
// 调用点（`output.go` 的 `drawOutputRow`）在材质之外**又**给整块板挂了
// `.Opacity(0.62)`——参考 CSS 给整行的那一道，被搬到了板上。两道乘起来，28u 的板
// 从「E1 的一个面」退成「地面上一块更浅的斑」：实测面差 18 → **11**（浅）、
// 24 → **15**（深），底对侧 14 → **9** / 10 → **5**。
//
// 这条 pin 钉的是**一道淡**：便条板的面与门板的面是同一个面差（差的是冠线与墨，
// 不是层高），而「便条」这个登记由两件东西说——熄掉的冠线，与低一档的字（标题
// `TextMuted`、字形 `TextTertiary`）。同一个应用里「禁用 / 只读」的材质只有一种
// 说法：P13-s / P13-aq / P13-u / P13-y 都是「只留一道淡」，这里是同一个算术。
func TestTheNoteRowPlateCarriesOneFadeNotTwo(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			door, note := notePlateProbe(t, dark)
			// ① 面差：便条板与门板是同一块面。差一档就说明还有第二道淡骑在上面。
			if step := levelGap(door.step - note.step); step > 1 {
				t.Errorf("the note plate's face steps %d levels off its ground and the door plate's steps %d: "+
					"two registers of one material (liquid-glass.md §6.2b-6 的「一道淡画两遍」)", note.step, door.step)
			}
			if note.step < 16 {
				t.Errorf("the note plate's face lifts only %d levels off its ground (%v over %v)", note.step, note.face, note.ground)
			}
			// ② 冠线：便条板不点灯。DimFace 只留底对侧，顶行就是面本身。
			if note.crown != 0 {
				t.Errorf("the note plate is lit: its top row lifts %d levels above its face — a note is not a door", note.crown)
			}
			if door.crown == 0 {
				t.Fatalf("the probe cannot see a crown at all: the door plate's top row equals its face, so ② proves nothing")
			}
			// ③ 底对侧：熄灯只熄顶边。底边那条 `LensEdge` 与门板同值。
			if gap := levelGap(door.foot - note.foot); gap > 1 {
				t.Errorf("the note plate's counter-side line is %d levels and the door plate's is %d: "+
					"DimFace keeps the shade and drops the light, not the other way round", note.foot, door.foot)
			}
			if note.foot == 0 {
				t.Errorf("the note plate has no counter-side line at all (%v over %v)", note.face, note.face)
			}
			// ④ 字形墨低一档：便条的记号是**墨**的一步，不是不透明度的一步。
			if drop := door.glyph - note.glyph; drop < 15 {
				t.Errorf("the note plate's glyph reads %d levels off its face and the door's %d (a step of %d): "+
					"the note's mark is one ink rung (TextMuted → TextTertiary), not a fade", note.glyph, door.glyph, drop)
			}
		})
	}
}

// plateReading is one 28u icon plate as the pixels report it.
type plateReading struct {
	ground, face color.RGBA
	step         int // face over ground
	crown        int // top row over face
	foot         int // bottom row under face
	glyph        int // the glyph's strongest step off the face
}

// notePlateProbe draws one runnable row and one note (`kind:"status"`) in the same
// captured-output list and reads both plates out of the frame. The runnable row is
// the second one, so neither plate sits on the cursor's `RowSelected` face — the
// two readings have to share one ground to be comparable.
func notePlateProbe(t *testing.T, dark bool) (door, note plateReading) {
	t.Helper()
	a := testApp()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	tt := runListOutput(t, a, doorAndNoteOutput())
	// 深色那一臂必须让**画布**也变深：tester 的底默认是白的，深色的窗口地面是
	// 半透明的，不切画布读到的是「深色面合成在白纸上」，那不是用户看见的那一帧。
	tt.SetDark(dark)
	tt.Frame()
	img := tt.Image()
	door = readPlate(t, tt, img, "Door2")
	note = readPlate(t, tt, img, "Note")
	return door, note
}

// doorAndNoteOutput is one runnable row followed by one note (`kind:"status"`),
// both carrying the same glyph so ④ compares ink and not a picture.
func doorAndNoteOutput() string {
	return strings.Join([]string{
		`[`,
		`{"id":"r0","title":"Door","icon":"file","action":{"type":"insert","text":"picked"}},`,
		`{"id":"r1","title":"Door2","icon":"file","action":{"type":"insert","text":"picked2"}},`,
		`{"id":"n2","title":"Note","icon":"file","kind":"status"}`,
		`]`,
	}, "")
}

// readPlate finds one row's icon plate in the frame and measures it.
//
// The plate is found by its **edges**, not by the theme's 28u arithmetic: a
// horizontal scan left of the row's title ink gives the box, and the box's own
// width is asserted to be the icon column's, so a plate that grew or shrank is a
// reading error rather than a silently different box.
func readPlate(t *testing.T, tt *ui.Tester, img *image.RGBA, name string) plateReading {
	t.Helper()
	r, ok := tt.Find(name)
	if !ok {
		t.Fatalf("%q was not drawn: %q", name, tt.Texts())
	}
	y := int(r.Y + r.H/2)
	ground := img.RGBAAt(int(r.X)-3, y)
	right := -1
	for x := int(r.X) - 3; x >= 0; x-- {
		if img.RGBAAt(x, y) != ground {
			right = x
			break
		}
	}
	if right < 0 {
		t.Fatalf("%q: no plate left of the title ink", name)
	}
	left := right
	for x := right; x >= 0; x-- {
		if img.RGBAAt(x, y) == ground {
			break
		}
		left = x
	}
	cx := left + 3 // a column clear of the 16u glyph inside the 28u box
	top, bottom := y, y
	for yy := y; yy >= 0; yy-- {
		if img.RGBAAt(cx, yy) == ground {
			break
		}
		top = yy
	}
	for yy := y; yy < img.Bounds().Max.Y; yy++ {
		if img.RGBAAt(cx, yy) == ground {
			break
		}
		bottom = yy
	}
	if w := right - left + 1; w != 28 {
		t.Fatalf("%q: the plate is %dpx wide, want the 28u icon column", name, w)
	}
	face := modeIn(img, image.Rect(left+3, top+3, right-2, bottom-2))
	return plateReading{
		ground: ground,
		face:   face,
		step:   channelGap(ground, face),
		crown:  channelGap(face, img.RGBAAt((left+right)/2, top)),
		foot:   channelGap(face, img.RGBAAt((left+right)/2, bottom)),
		glyph:  maxGapIn(img, face, ui.Rect{X: float32(left + 6), Y: float32(top + 6), W: float32(right - left - 11), H: float32(bottom - top - 11)}),
	}
}

// modeIn is a box's own face: the most common colour inside it, clear of its 1px
// lines and corner arcs. A face is the one colour that covers the interior, so
// the mode cannot be fooled by a single glyph pixel.
func modeIn(img *image.RGBA, r image.Rectangle) color.RGBA {
	seen := map[color.RGBA]int{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			seen[img.RGBAAt(x, y)]++
		}
	}
	var best color.RGBA
	n := 0
	for c, k := range seen {
		if k > n {
			best, n = c, k
		}
	}
	return best
}

// maxGapIn is the strongest step any pixel in the box takes off the given face,
// in either direction: on the light sheet a glyph is darker than its plate and on
// the dark one lighter, so a signed reading would only measure one appearance.
func maxGapIn(img *image.RGBA, face color.RGBA, r ui.Rect) int {
	best := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if g := channelGap(face, img.RGBAAt(x, y)); g > best {
				best = g
			}
		}
	}
	return best
}

func levelGap(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestDumpNotePlatePNGs writes the round's visual evidence: the same three-row
// captured-output list in both appearances, so the note row's plate can be looked
// at beside the two doors' plates. Opt-in (`FLOTER_PNG=1`), like every other dump
// in this package.
func TestDumpNotePlatePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tt := runListOutput(t, a, doorAndNoteOutput())
		tt.SetDark(dark)
		tt.Frame()
		path := "/tmp/floter-P13-cp-noteplate-" + appearance + suffix + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
