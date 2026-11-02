package launcher

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/settings"
	"floter/internal/theme"
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
			door, note, tk := notePlateProbe(t, dark)
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
			// P13-du：不再用「差 ≥ 15 个亮度级」这个库尺魔法数（它是库存尺的产物：
			// 库存尺浅色实测 drop 29，出货尺只有 14）。改成**身份 pin**：门板的字形墨
			// 正好是 `TextMuted` 压在板面上，便条的正尧是 `TextTertiary`（深浅两套实测恒等）。
			if want := channelGap(door.face, over(tk.Theme.TextMuted, door.face)); door.glyph != want {
				t.Errorf("the door plate's glyph reads %d levels off its face, want TextMuted's %d: "+
					"the row's mark is the ink, not a fade", door.glyph, want)
			}
			if want := channelGap(note.face, over(tk.TextTertiary, note.face)); note.glyph != want {
				t.Errorf("the note plate's glyph reads %d levels off its face, want TextTertiary's %d: "+
					"the note's mark is one ink rung (TextMuted → TextTertiary), not a fade", note.glyph, want)
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
func notePlateProbe(t *testing.T, dark bool) (door, note plateReading, tk theme.Tokens) {
	t.Helper()
	a := testApp()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	// P13-du：取景换成出货尺帧（`c.SetTheme(a.tokens(c).Theme)` + `themedFrameHeight`）。
	// 旧的 `runListOutput` 画的是框架库存主题（`Spacing 4.0`），板子是 28×28 的
	// 整数盒；app 画的是 25.2，落 25…26 列（§6.86 的库存尺）。
	tt := listOutputShipped(t, a, doorAndNoteOutput(), dark)
	img := tt.Image()
	door = readPlate(t, tt, img, "Door2")
	note = readPlate(t, tt, img, "Note")
	// ④ 要比的是 app 自己发的那套墨（深浅各一套），不是框架库存主题的字色。
	tk = theme.ForPreferences(settings.Normalize(a.settings()), dark, ui.Preferences{})
	return door, note, tk
}

// listOutputShipped draws one captured-output list at the ruler the app ships.
//
// It is `runListOutput` with `renderShipped`'s two installation steps: the
// launcher's own palette on the context (otherwise every face below is the
// framework's stock ink) and the frame charged at `themedFrameHeight`.
func listOutputShipped(t *testing.T, a *App, stdout string, dark bool) *ui.Tester {
	t.Helper()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Command: []string{"tool"}, Success: true, Stdout: stdout}, nil)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, InputWindowWidth, themedFrameHeight(maxResultSlots))
	// 深色那一臂必须让**画布**也变深：tester 的底默认是白的，深色的窗口地面是
	// 半透明的，不切画布读到的是「深色面合成在白纸上」，那不是用户看见的那一帧。
	if dark {
		tt.SetDark(true)
		tt.Advance(500 * time.Millisecond)
	}
	tt.Frame()
	a.Query = "list"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	return tt
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
	// P13-du：板盒是 painter 的取整，不是 28 这个库存尺字面量。板列是
	// 8u 块内缩 + 8u 内边距 = 16u 起、+28u 图标列 = 44u 止；painter 填
	// `[round(x), round(x+w))`，所以首列 `round(16u)`、末列 `round(44u) − 1`。
	// 出货尺（3.6）下 16u = 14.4、44u = 39.6 → `left=14 right=39`，宽 **26**
	// （25.2 的盒子落 26 列，不是 28）。
	u := designUnit()
	plateLeft, plateRight := (8+8)*u, (8+8+28)*u
	if want := int(math.Round(plateLeft)); left != want {
		t.Fatalf("%q: the plate starts at x=%d, want the 28u icon column's left edge (round(16u) = %d)", name, left, want)
	}
	if want := int(math.Round(plateRight)) - 1; right != want {
		t.Fatalf("%q: the plate ends at x=%d, want the 28u icon column's right edge (round(44u) − 1 = %d)", name, right, want)
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

// P13-du · opt-in dump：便条板与行块的**出货尺**帧，供人眼复核。
//
// 这一轮产品像素零改动，出图不是为了「看新样子」，而是把两条 pin 量的框子
// 原样留下证据：板子 25.2 落 26 列（`14..39`），行块左右沟 `7` / `712`。
// 两个表面各占一帧，各自深浅一版：`FLOTER_PNG=1 go test -run TestDumpP13DU`.
func TestDumpP13DURoundingPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-du-<surface>-<appearance>.png")
	}
	dump := func(name string, tt *ui.Tester) {
		path := filepath.Join("/tmp", "floter-P13-du-"+name+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: image %v", path, tt.Image().Bounds())
	}
	forEachAppearance(t, func(t *testing.T, dark bool) {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		dump("plates-"+appearance, listOutputShipped(t, a, doorAndNoteOutput(), dark))

		b := testApp()
		recentPage(b, 2)
		dump("rowblock-"+appearance, renderShipped(t, b, dark))
	})
}
