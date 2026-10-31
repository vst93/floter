package launcher

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// P13-h · 启动器的**小面**：28u 图标板与两处悬停小格从四层控件透镜退成 E1 的一对面。
//
// `theme.Tokens.Lens` 是四层：顶 `LensRim` + 宽 `LensSheen` + 底 `LensEdge` +
// **四边 `ControlEdge` 的环**。第四层就是 `docs/liquid-glass.md` §2 要退掉的
// 「画框分层」——一块 28u 的板戴上它，读起来是「macOS 旧版本的工具条按钮」，
// 而这块板在启动器的**每一行**都出现（`launcher.go` 的结果行、`output.go` 的
// 捕获输出页行共用 `iconPlate` 这一个 draw point）。
//
// 两条 pin，每条都在深/浅两套外观上跑，全部量像素：
//   - `TestTheIconPlateIsAFaceNotARing`：板是 E1 的**面** —— 顶内线亮于面、
//     底内线暗于面、**左右两侧的列与面同色**（`Lens` 的环会让它们变一个色号）、
//     板外两像素仍是地（E1 无外投影，§2 判据 2）。
//   - `TestTheHoveredFieldControlIsAFaceNotARing`：字段行上悬停的 20u 小格同一条
//     规则、同一个函数（`rowControl` / `fieldSettingsButton` 的 `Hovered()` 臂）。
//
// 为什么这条 pin 量得到东西：`Lens` 与 `LitFace` 在**顶边与底边**上是同一对
// token，两版都亮、都暗；两者唯一分得开的地方是**左右两侧**与那条宽 sheen。
// 所以 pin 的主断言是侧列，顶/底两条只是把「面立起来」这件事一起钉住。

// plateGround is the opaque plane the isolated plate is drawn on. It is a fixed
// colour on purpose: the reading is entirely relative (lines against the plate's
// own face, the outside against the ground), so the ground only has to be a
// constant that the plate's translucent fill visibly separates from.
func plateGround(dark bool) color.RGBA {
	if dark {
		return color.RGBA{R: 28, G: 29, B: 33, A: 255}
	}
	return color.RGBA{R: 246, G: 246, B: 248, A: 255}
}

// plateFace renders one bare `iconPlate` — no children, so every pixel inside
// the box is either the plate's face or one of its 1px lines — centred on a
// constant ground.
//
// The box comes from the **theme's own** icon column (`Space(7)` = 28u), not from
// a scan of the frame: a scan silently grows with whatever the plate spills, and
// a cast shadow is exactly what assertion ④ has to catch.
type plateProbe struct {
	img    *image.RGBA
	tokens theme.Tokens
	box    image.Rectangle
	ground color.RGBA
}

func plateFace(t *testing.T, dark bool) plateProbe {
	t.Helper()
	a := testApp()
	ground := plateGround(dark)
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		if dark {
			c.SetTheme(ui.DarkTheme())
		} else {
			c.SetTheme(ui.LightTheme())
		}
		tokens = a.tokens(c)
		ui.Column(c).FillWidth().FillHeight().Background(ui.RGB(ground.R, ground.G, ground.B)).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			a.iconPlate(c, false, false)
		})
	}, 120, 80)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()

	img := tt.Image()
	size := int(ui.LightTheme().Space(7))
	cx, cy := img.Rect.Dx()/2, img.Rect.Dy()/2
	if got := img.RGBAAt(cx, cy); got == ground {
		t.Fatalf("the plate was not drawn at the centre of the probe frame (pixel %v is the ground %v)", got, ground)
	}
	return plateProbe{
		img:    img,
		tokens: tokens,
		box:    image.Rect(cx-size/2, cy-size/2, cx-size/2+size, cy-size/2+size),
		ground: ground,
	}
}

// TestTheIconPlateIsAFaceNotARing pins the 28u plate as E1: one lit line above,
// one shade below, **nothing on the sides**, nothing cast outside.
//
// Mutation checks (each one measured before this pin landed):
//   - put `tokens.Lens(plate)` back → the side columns carry `ControlEdge`, so
//     the first assertion fails (that is the whole point of the round);
//   - drop the `LitFace` call entirely (a flat `Control` box) → the top line is
//     the face, so 「顶内线亮于面」 fails;
//   - paint only the top line (the previous generation's half-pair) → the bottom
//     assertion fails;
//   - add a `Shadow(` to the plate → the outside pixel is no longer the ground.
func TestTheIconPlateIsAFaceNotARing(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			p := plateFace(t, appearance.dark)
			b := p.box
			cy := (b.Min.Y + b.Max.Y) / 2
			cx := (b.Min.X + b.Max.X) / 2
			// The face is read four pixels in from the left edge: clear of the
			// 1px line family, and inside the box whatever its width turns out
			// to be.
			face := p.img.RGBAAt(b.Min.X+4, cy)

			// ① No ring: the two side columns at mid height are the face itself.
			// This is the assertion that separates E1 from the four-layer lens.
			for _, side := range []struct {
				what string
				got  color.RGBA
			}{{"left", p.img.RGBAAt(b.Min.X, cy)}, {"right", p.img.RGBAAt(b.Max.X-1, cy)}} {
				if gap := channelGap(side.got, face); gap > 1 {
					t.Errorf("the plate's %s column at mid height is %v but its face is %v — a %d-level ring is "+
						"drawn around the whole box: the 28u plate is an E1 face, not the four-layer `Lens` "+
						"(liquid-glass.md §2 判据 / §6.1 第 1-4 行)", side.what, side.got, face, gap)
				}
			}
			// ② The crown: the top inner line is strictly brighter than the face
			// it closes. §1 的不变量（受光边严格亮于面）。
			if top := p.img.RGBAAt(cx, b.Min.Y); !brighter(top, face) {
				t.Errorf("the plate's top inner line %v is not brighter than its own face %v: the lit edge is "+
					"missing (E1 = face + one 1px LensRim, liquid-glass.md §2)", top, face)
			} else {
				// ②b …and it is the **component** line, not the floating one. The two
				// are told apart by compositing each candidate over the same face:
				// the reading has to sit nearer `LensRim` than `EdgeHighlight`. The
				// candidates are first checked to be far apart, or this comparison
				// could never fail.
				rim := inkOver(p.tokens.LensRim, face)
				floating := inkOver(p.tokens.EdgeHighlight, face)
				if channelGap(rim, floating) < 3 {
					t.Fatalf("the component rim %v and the floating rim %v are one line: E1 and E2 could not be told "+
						"apart", rim, floating)
				}
				if channelGap(top, floating) <= channelGap(top, rim) {
					t.Errorf("the plate's crown %v is nearer the floating `EdgeHighlight` (%v) than the component "+
						"`LensRim` (%v): an E1 block wears the component line, the E2 one belongs to panes that "+
						"press on other content (liquid-glass.md §2/§3)", top, floating, rim)
				}
			}
			// ③ The counter-side: §3 的「顶亮一分、底暗一分」。Without it a block
			// on the light appearance is 白压白 and reads as the ground.
			if bottom := p.img.RGBAAt(cx, b.Max.Y-1); !darker(bottom, face) {
				t.Errorf("the plate's bottom inner line %v is not darker than its own face %v: E1's counter-side "+
					"is missing (liquid-glass.md §3)", bottom, face)
			}
			// ④ E1 casts nothing (§2 判据 2): nothing is painted outside the 28u
			// box — the box is the theme's own icon column, so a cast shadow lands
			// on these pixels instead of growing the box away from the assertion.
			for _, out := range []struct {
				what string
				got  color.RGBA
			}{
				{"left of", p.img.RGBAAt(b.Min.X-1, cy)},
				{"two pixels left of", p.img.RGBAAt(b.Min.X-2, cy)},
				{"right of", p.img.RGBAAt(b.Max.X, cy)},
				{"above", p.img.RGBAAt(cx, b.Min.Y-1)},
				{"two pixels above", p.img.RGBAAt(cx, b.Min.Y-2)},
				{"below", p.img.RGBAAt(cx, b.Max.Y)},
			} {
				if out.got != p.ground {
					t.Errorf("the plate paints %s its own 28u box (%v against the ground %v): an E1 block lies "+
						"on the surface, it does not float over it and casts nothing (liquid-glass.md §2 判据 2)",
						out.what, out.got, p.ground)
				}
			}
			// …and the rim it wears is the component one, not the floating
			// storey's: E1 and E2 must not share an edge (§3).
			if p.tokens.LensRim == p.tokens.EdgeHighlight {
				t.Fatalf("the component rim and the floating rim are one value %v: E1 and E2 would wear the same edge",
					p.tokens.LensRim)
			}
		})
	}
}

// darker reports whether a is strictly darker than b on every channel: the
// counter-side of a lit edge is a neutral black line, so it must pull all three
// down. (The sibling `brighter` is the same test upwards.)
func darker(a, b color.RGBA) bool {
	return a.R < b.R && a.G < b.G && a.B < b.B
}

// TestTheHoveredFieldControlIsAFaceNotARing pins the same rule on the two small
// cells the launcher lights only under the pointer (`rowControl` 20u,
// `fieldSettingsButton` 28u): a hovered cell is E1's face — the `ControlHover`
// rung plus one lit line — not the four-layer lens.
//
// The reading is on the field row's `Clear` control, which is `rowControl`'s
// sibling draw point with the same hover arm. The control is transparent at
// rest, so this is the only state it can be graded in.
func TestTheHoveredFieldControlIsAFaceNotARing(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			a := testApp()
			a.Actions.OpenSessions = func() {}
			tt := renderIn(t, a, appearance.dark)
			tt.Type("x")
			tt.Frame()
			clear, ok := tt.Find("Clear")
			if !ok {
				t.Fatalf("the field's clear control was not drawn: %q", tt.Texts())
			}
			tt.Move(clear.X+clear.W/2, clear.Y+clear.H/2)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
			img := tt.Image()
			cy := int(clear.Y + clear.H/2)
			face := img.RGBAAt(int(clear.X)+1, cy)
			// The hover wash must be there at all: read against the field block
			// well clear of the control. A reading taken before the 120ms colour
			// transition would grade the field, not the control.
			field := img.RGBAAt(int(clear.X)-40, cy)
			if channelGap(face, field) < 2 {
				t.Fatalf("the clear control painted no hover face at all (face %v, field %v): the reading would "+
					"grade the field's own plane", face, field)
			}
			for _, side := range []struct {
				what string
				got  color.RGBA
			}{
				{"left", img.RGBAAt(int(clear.X), cy)},
				{"right", img.RGBAAt(int(clear.X+clear.W)-1, cy)},
			} {
				if gap := channelGap(side.got, face); gap > 1 {
					t.Errorf("the hovered control's %s column at mid height is %v but its face is %v — a ring is "+
						"drawn around the box: the hovered cell is an E1 face, not the four-layer `Lens` "+
						"(liquid-glass.md §2)", side.what, side.got, face)
				}
			}
		})
	}
}

// TestTheHoveredRowControlIsAFaceNotARing pins the other small cell of the same
// family: `rowControl` (the 20u trailing control on a clipboard row, 20u) wears
// the same hover arm as `fieldSettingsButton`. It is drawn here on its own, on
// the same constant ground the plate probe uses, because the row it lives on
// needs a clipboard fixture; the cell itself is the draw point under test.
func TestTheHoveredRowControlIsAFaceNotARing(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			a := testApp()
			ground := plateGround(appearance.dark)
			tt := ui.NewTester(func(c *ui.Context) {
				if appearance.dark {
					c.SetTheme(ui.DarkTheme())
				} else {
					c.SetTheme(ui.LightTheme())
				}
				ui.Column(c).FillWidth().FillHeight().Background(ui.RGB(ground.R, ground.G, ground.B)).
					Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					a.rowControl(c, "text", "Pin", func() {})
				})
			}, 120, 80)
			tt.SetDark(appearance.dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Move(60, 40)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()

			img := tt.Image()
			size := int(ui.LightTheme().Space(5)) // the control's own 20u box
			box := image.Rect(60-size/2, 40-size/2, 60-size/2+size, 40-size/2+size)
			cy := (box.Min.Y + box.Max.Y) / 2
			face := img.RGBAAt(box.Min.X+1, cy)
			if channelGap(face, ground) < 2 {
				t.Fatalf("the control painted no hover face at all (face %v, ground %v): the reading would grade "+
					"the ground", face, ground)
			}
			for _, side := range []struct {
				what string
				got  color.RGBA
			}{
				{"left", img.RGBAAt(box.Min.X, cy)},
				{"right", img.RGBAAt(box.Max.X-1, cy)},
			} {
				if gap := channelGap(side.got, face); gap > 1 {
					t.Errorf("the hovered 20u control's %s column at mid height is %v but its face is %v — a ring is "+
						"drawn around the box: the hovered cell is an E1 face, not the four-layer `Lens` "+
						"(liquid-glass.md §2)", side.what, side.got, face)
				}
			}
		})
	}
}

// TestDumpLauncherPlatePNGs is this round's visual evidence: the first screen of
// the launcher (every row leads with the plate) in both appearances. Opt-in
// (`FLOTER_PNG=1`), and `FLOTER_PNG_TAG` names the pair (before/after).
func TestDumpLauncherPlatePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-h-launcher-<tag>-<appearance>.png")
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
		a := testApp()
		recentPage(a, 3)
		tt := renderIn(t, a, dark)
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-h-launcher-"+tag+"-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		img := tt.Image()
		t.Logf("%s (%dx%d)", path, img.Rect.Dx(), img.Rect.Dy())
	}
}
