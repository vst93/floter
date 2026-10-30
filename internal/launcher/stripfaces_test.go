package launcher

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// P13-aa · 启动器两条「带」上的四个小面：各缺半套 E1。
//
// 前几轮把「悬停小格 / 控件面」收口成 E1（面 + 顶受光边 + 底对侧暗线，
// `docs/liquid-glass.md` §2/§3），但那一族的账本只扫**有边框**的面：材质 census 的
// 判据是「面 + 边 ⇒ 必须有一对线」，**没有边**（或边框写在变量里）的面根本不进视野。
// 这一页剩下的四块正好都在那个盲区里：
//
//   - 捕获输出页页头的两个 28u 方键（`output.go` 的 `control` 闭包）：悬停只有一层
//     `ControlHover` 洗色，一条线都没有——同一个启动器的 `rowControl`（20u）与
//     `fieldSettingsButton`（28u）自 P13-h 起就戴 `LitFace`；
//   - 电源确认条的 cancel 键：悬停是「面 + 一圈 `ControlEdge`」= §6.2b-2 判过的
//     「一块被画框圈住的平涂」；
//   - 同一条带的 execute 键：只有**顶**受光边，没有底对侧——§3 的「顶亮一分、
//     底暗一分」只做了前半句。
//
// 读法一律是相对的（线对面、面外对地面），pin 里没有一处写死渲染器算出来的像素值。

// stripFace is one face's four readings: its own plane, the row just inside its
// top edge, the row just inside its bottom edge, and the two side columns.
type stripFace struct {
	name   string
	face   color.RGBA
	top    color.RGBA
	bottom color.RGBA
	left   color.RGBA
	right  color.RGBA
}

// readFace samples one element box. inset is the number of pixels the material's
// own lines sit inside the box edge: 0 for a face with no border (its inner
// line is the first row of the box), 1 for a face whose `Border(1, …)` owns the
// outermost pixel.
func readFace(img *image.RGBA, box ui.Rect, inset int, name string) stripFace {
	x, y := int(box.X), int(box.Y)
	w, h := int(box.W), int(box.H)
	cx := x + w/2
	cy := y + h/2
	return stripFace{
		name:   name,
		face:   img.RGBAAt(x+inset+3, cy),
		top:    img.RGBAAt(cx, y+inset),
		bottom: img.RGBAAt(cx, y+h-1-inset),
		left:   img.RGBAAt(x+inset, cy),
		right:  img.RGBAAt(x+w-1-inset, cy),
	}
}

// TestTheOutputStripControlsAreE1Faces pins the capture-output page's two 28u
// header keys as E1 faces under the pointer: one lit line above, one shade
// below, **nothing on the sides** (a ring is the four-layer lens this contract
// retired), and the face itself lifted off the strip's own plane.
//
// Mutation checks:
//   - drop the `LitFace` call (the `ControlHover` wash alone) → 「顶内线亮于面」
//     fails;
//   - paint only the top line → the bottom assertion fails;
//   - wrap it in `tokens.Lens(...)` → the side columns carry `ControlEdge` and
//     the ring assertion fails.
func TestTheOutputStripControlsAreE1Faces(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			a := testApp()
			a.Actions.PinText = func(string, string) {}
			a.Actions.DetachOutput = func(OutputView) {}
			tt := runListOutput(t, a, `[{"id":"r0","title":"Alpha"}]`)
			tt.SetDark(appearance.dark)
			tt.Frame()

			key, ok := tt.Find(a.copy().HistoryPin)
			if !ok {
				t.Fatalf("the output strip's pin key is not drawn: %q", tt.Texts())
			}
			if key.W < 27.5 || key.W > 28.5 {
				t.Fatalf("the pin key's box is %.1f wide, want the strip's own 28u", key.W)
			}
			tt.Move(key.X+key.W/2, key.Y+key.H/2)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()

			img := tt.Image()
			got := readFace(img, key, 0, "pin")
			t.Logf("%s pin key: face %v top %v bottom %v left %v right %v",
				appearance.name, got.face, got.top, got.bottom, got.left, got.right)

			// The hover face has to be there at all, or the reading grades the
			// strip's own plane.
			strip := img.RGBAAt(int(key.X)-20, int(key.Y+key.H/2))
			if channelGap(got.face, strip) < 2 {
				t.Fatalf("the pin key painted no hover face (face %v, strip %v): the reading would grade the strip",
					got.face, strip)
			}
			if !brighter(got.top, got.face) {
				t.Errorf("the pin key's top inner line %v is not brighter than its own face %v: a hovered key is an "+
					"E1 face, not a flat wash (liquid-glass.md §2)", got.top, got.face)
			}
			if !darker(got.bottom, got.face) {
				t.Errorf("the pin key's bottom inner line %v is not darker than its own face %v: E1's counter-side "+
					"is missing (liquid-glass.md §3)", got.bottom, got.face)
			}
			for _, side := range []struct {
				what string
				got  color.RGBA
			}{{"left", got.left}, {"right", got.right}} {
				if gap := channelGap(side.got, got.face); gap > 1 {
					t.Errorf("the pin key's %s column at mid height is %v but its face is %v — a %d-level ring is "+
						"drawn around the box: the key is an E1 face, not the four-layer `Lens` (liquid-glass.md §2)",
						side.what, side.got, got.face, gap)
				}
			}
		})
	}
}

// TestThePowerChipsAreE1Faces pins the power confirmation's two chips. Both sit
// on the same 30u band as siblings, so they must wear the same material:
//
//   - the cancel chip is transparent at rest and **must stay unlit there** (a
//     white line on a transparent face is the 「类玻璃边」 §1 禁令 3 forbids);
//     under the pointer it takes a plane, and a plane wears the pair;
//   - the execute chip's warm raised pane already had the lit top rim; it was
//     missing the counter-side, which is what makes the chip stand up rather
//     than glow.
func TestThePowerChipsAreE1Faces(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			a := testApp()
			a.Actions.Power = func(string) {}
			tt := renderIn(t, a, appearance.dark)
			tt.Type("restart")
			tt.Frame()
			tt.TypeKey(0, ui.KeyEnter, "")
			tt.Frame()

			// ① The execute chip, at rest: the pair, both halves.
			execute, ok := tt.Find(a.copy().PowerRestartAction)
			if !ok {
				t.Fatalf("the execute chip is not drawn: %q", tt.Texts())
			}
			img := tt.Image()
			exec := readFace(img, execute, 1, "execute")
			t.Logf("%s execute chip: face %v top %v bottom %v left %v right %v",
				appearance.name, exec.face, exec.top, exec.bottom, exec.left, exec.right)
			if !brighter(exec.top, exec.face) {
				t.Errorf("the execute chip's top inner line %v is not brighter than its own face %v: the lit edge "+
					"is missing", exec.top, exec.face)
			}
			if !darker(exec.bottom, exec.face) {
				t.Errorf("the execute chip's bottom inner line %v is not darker than its own face %v: the raised "+
					"pane had only half the pair (liquid-glass.md §3)", exec.bottom, exec.face)
			}

			// ② The cancel chip at rest is transparent: no plane, no lit edge.
			cancel, ok := tt.Find(a.copy().PowerCancel)
			if !ok {
				t.Fatalf("the cancel chip is not drawn: %q", tt.Texts())
			}
			rest := readFace(tt.Image(), cancel, 1, "cancel-rest")
			t.Logf("%s cancel rest: face %v top %v bottom %v",
				appearance.name, rest.face, rest.top, rest.bottom)
			band := tt.Image().RGBAAt(int(cancel.X)-6, int(cancel.Y+cancel.H/2))
			if channelGap(rest.face, band) > 1 {
				t.Errorf("the cancel chip paints a plane at rest (%v against the band's %v): it is transparent "+
					"until the pointer lands on it", rest.face, band)
			}
			if channelGap(rest.top, band) > 1 {
				t.Errorf("the cancel chip wears a lit edge at rest (%v against the band's %v): a line on a "+
					"transparent face is the 「类玻璃边」 §1 禁令 3 forbids (liquid-glass.md §1)",
					rest.top, band)
			}

			// ③ The cancel chip under the pointer: a plane, and the pair with it.
			tt.Move(cancel.X+cancel.W/2, cancel.Y+cancel.H/2)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
			img = tt.Image()
			hover := readFace(img, cancel, 1, "cancel-hover")
			t.Logf("%s cancel hover: face %v top %v bottom %v left %v right %v",
				appearance.name, hover.face, hover.top, hover.bottom, hover.left, hover.right)
			if !brighter(hover.top, hover.face) {
				t.Errorf("the hovered cancel chip's top inner line %v is not brighter than its own face %v: a "+
					"hovered chip is an E1 face, not a fill inside a frame (liquid-glass.md §2)", hover.top, hover.face)
			}
			if !darker(hover.bottom, hover.face) {
				t.Errorf("the hovered cancel chip's bottom inner line %v is not darker than its own face %v: "+
					"E1's counter-side is missing (liquid-glass.md §3)", hover.bottom, hover.face)
			}
		})
	}
}

// TestDumpLauncherStripPNGs is this round's visual evidence: the capture-output
// page's header with the pointer on its pin key, and the power confirmation
// with the pointer on its cancel chip. Opt-in (`FLOTER_PNG=1`), and
// `FLOTER_PNG_TAG` names the pair (before/after).
func TestDumpLauncherStripPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-aa-strip-<tag>-<appearance>.png")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		// ① the output strip, pointer on the pin key.
		a := testApp()
		a.Actions.PinText = func(string, string) {}
		a.Actions.DetachOutput = func(OutputView) {}
		tt := runListOutput(t, a, `[{"id":"r0","title":"Alpha","group":"Group A"},`+
			`{"id":"r1","title":"Beta","group":"Group A"}]`)
		tt.SetDark(dark)
		tt.Frame()
		if key, ok := tt.Find(a.copy().HistoryPin); ok {
			tt.Move(key.X+key.W/2, key.Y+key.H/2)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
		}
		writeStripPNG(t, tt, "/tmp/floter-P13-aa-strip-"+tag+"-"+appearance+".png")

		// ② the power confirmation, pointer on the cancel chip.
		b := testApp()
		b.Actions.Power = func(string) {}
		pt := renderIn(t, b, dark)
		pt.Type("restart")
		pt.Frame()
		pt.TypeKey(0, ui.KeyEnter, "")
		pt.Frame()
		if cancel, ok := pt.Find(b.copy().PowerCancel); ok {
			pt.Move(cancel.X+cancel.W/2, cancel.Y+cancel.H/2)
			pt.Frame()
			pt.Advance(300 * time.Millisecond)
			pt.Frame()
		}
		writeStripPNG(t, pt, "/tmp/floter-P13-aa-power-"+tag+"-"+appearance+".png")
	}
}

func writeStripPNG(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}
