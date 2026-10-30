package settingsui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-i · 设置页剩下的十处控件面：`theme.Tokens.Lens` 的第四层环画在第一条
// `LensRim` 之后，而且落在同一个像素行上（mygo 的 `insetShadow` 把 hole 从
// padding edge 算起：`spread=1` 的环占 padding 第一行，`y=1,spread=0` 的 rim 也是
// 那一行，后加的画在上面）。所以 P12-a 抬到 0.09 的受光边在这一页**一次都没亮过**：
// 每个控件的顶边都是一条 `ControlEdge` 的暗线。P13-h 在启动器的 28u 图标板上抓到
// 同一个根因；这一轮是同一族在设置页上的十处。
//
// 三条像素 pin（每一条都在深/浅两套外观上跑，全部量像素、不读注释）：
//   - `TestTheActionPillIsAFaceNotARing`：`.extensions-action-button` 胶囊；
//   - `TestTheHoveredIconButtonIsAFaceNotARing`：30u 图标钮的 hover 臂；
//   - `TestTheUninstallPartCardIsAFaceNotARing`：卸载对话框的组件行卡片。
// 一条结构 pin（`TestNoSettingsSurfaceControlCallsTheFourLayerLens`）守住这一族
// 不许回到四层透镜。
//
// 为什么 pin 量得到东西：`Lens` 与 `LitFace` 在**顶边**上是同一个 token
// （`LensRim`），深色档里环（`ControlEdge` 0.07 白）与 rim（0.09 白）只差一级，
// 所以顶边不是可靠的判据。两者分得开的地方是：①**左右两侧的列**（环把它们涂成
// `ControlEdge`，`LitFace` 让它们回到面）；②**底侧**（深色档的环是白的，压在
// `LensEdge` 上，于是底内线反而比面亮）。所以主断言是这两条，顶边与「块外仍是地」
// 把 E1 的另外两条判据一起钉住。

// lensGround is the opaque plane every probe draws on. It is the same mid grey
// P13-e used for this surface: `Control` (white wash in dark, black wash in
// light) separates from it in both appearances, so one ground serves both.
func lensGround() ui.Color { return ui.RGB(128, 128, 128) }

// lensProbe renders one settings control, alone, centred on `lensGround`.
func lensProbe(t *testing.T, dark bool, w, h int, build func(c *ui.Context, a *App)) (*ui.Tester, ui.Color) {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
	a := New(store, Actions{})
	ground := lensGround()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		// The probe puts the control at (20, 20) rather than centring it: a
		// centred box lands on a fractional row whenever its height is odd, and
		// a 1px line on a half-covered row is a blend — the reading would then
		// grade the rasteriser, not the material.
		ui.Column(c).FillWidth().FillHeight().Background(ground).Padding(20).
			AlignItems(ui.Start).Justify(ui.Start).Children(func() {
			build(c, a)
		})
	}, w, h)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt, ground
}

// lensMean is the mean colour of one rectangle of the frame. Reading a band
// rather than one pixel is what makes these pins survive a layout that resolves
// to a fractional coordinate: a 1px line can land half-covered on two rows, and
// the rounded corners' antialiasing is a few columns at each end.
func lensMean(img *image.RGBA, x0, y0, x1, y1 int) ui.Color {
	var r, g, b, n int
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			p := imagePixel(img, x, y)
			r, g, b, n = r+int(p.R), g+int(p.G), b+int(p.B), n+1
		}
	}
	if n == 0 {
		return ui.Color{}
	}
	return ui.RGB(uint8(r/n), uint8(g/n), uint8(b/n))
}

// lensAssertE1 states what an E1 face must show: a lit line above, a shade
// below, **nothing on the sides** (the four-layer lens's ring painted both),
// and nothing cast outside it. Every reading is a two-pixel band, so a line that
// lands one row off still lands inside the reading.
func lensAssertE1(t *testing.T, appearance, what string, img *image.RGBA, box ui.Rect, faceY int, ground ui.Color) {
	t.Helper()
	x0, x1 := int(box.X+box.W*0.25), int(box.X+box.W*0.75)
	y0, y1 := int(box.Y+box.H*0.3), int(box.Y+box.H*0.7)
	// The face is read on the row the caller names: four units under the top
	// frame is above every control's content (a label's line box, a glyph, a
	// checkbox), so the reading grades the material and not the ink on it.
	face := lensMean(img, x0, faceY, x1, faceY)
	crown := lensMean(img, x0, int(box.Y)+1, x1, int(box.Y)+2)
	shade := lensMean(img, x0, int(box.Y+box.H)-3, x1, int(box.Y+box.H)-2)
	if !liteBrighter(crown, face) {
		t.Errorf("[%s] %s: the rows inside its top frame are %v, not brighter than its face %v — the face lost "+
			"its lit edge (liquid-glass.md §1/§2)", appearance, what, crown, face)
	}
	if !liteDarker(shade, face) {
		t.Errorf("[%s] %s: the rows inside its bottom frame are %v, not darker than its face %v — E1 is a pair "+
			"(§3: 顶亮一分、底暗一分); on the dark appearance the four-layer lens's white ring sat on top of this "+
			"shade", appearance, what, shade, face)
	}
	for _, side := range []struct {
		name string
		x0   int
		x1   int
	}{
		{"left", int(box.X) + 1, int(box.X) + 2},
		{"right", int(box.X+box.W) - 3, int(box.X+box.W) - 2},
	} {
		got := lensMean(img, side.x0, y0, side.x1, y1)
		if channelDelta(got, face) > 2 {
			t.Errorf("[%s] %s: its %s columns at mid height are %v but its face is %v — a ring is drawn around "+
				"the block: it is an E1 face, not the four-layer lens", appearance, what, side.name, got, face)
		}
	}
	if outside := lensMean(img, int(box.X)-4, y0, int(box.X)-3, y1); channelDelta(outside, ground) > 4 {
		t.Errorf("[%s] %s casts a shadow beside itself (%v against the ground %v): E1 adds none (§2 判据 2)",
			appearance, what, outside, ground)
	}
}

// TestTheActionPillIsAFaceNotARing pins §6.2's new 9-1 row: the shared
// `.extensions-action-button` pill (the integration card's actions, the
// discard bar's pills, the run form's Save) is an E1 face over its own
// `Border(1, ControlEdge)`.
//
// Mutation checks (each one measured before this pin landed):
//   - put `tokens.Lens(button)` back → the side columns carry `ControlEdge`
//     (light 115 against a 120 face; dark 150 against 141), so ① fails;
//   - keep only the lit row (drop `LitFace`'s counter-side) → ② fails;
//   - drop the whole `LitFace` call (a flat `Control` pane) → ③ fails;
//   - add a `Shadow(` → ④ fails.
func TestTheActionPillIsAFaceNotARing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			var ref ui.Handle
			var box ui.Rect
			tt, ground := lensProbe(t, appearance.dark, 400, 120, func(c *ui.Context, a *App) {
				a.actionButton(c, "Check").Bind(&ref)
				box = c.Resolve(ref).Bounds()
			})
			tt.Frame()
			if box.W == 0 {
				t.Fatal("the pill's box was not found")
			}
			lensAssertE1(t, appearance.name, "the action pill", tt.Image(), box, int(box.Y)+4, ground)
		})
	}
}

// TestTheHoveredIconButtonIsAFaceNotARing pins row 9-2 (and, by the same
// function, 9-3 / 9-4: the argument editor's 28u button, the settings header's
// two 28u buttons and the save alert's ✕ all take this arm). The icon button is
// transparent at rest and lights a `ControlHover` face under the pointer.
//
// Mutation: put `tokens.Lens(button)` back in the hover arm → the ring paints
// the two columns beside the frame, and on the dark appearance the white ring
// replaces the bottom shade.
func TestTheHoveredIconButtonIsAFaceNotARing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			tt, ground := lensProbe(t, appearance.dark, 200, 120, func(c *ui.Context, a *App) {
				a.extensionIconButton(c, "copy", "Copy", false)
			})
			// The pointer lands on the button's own centre: the hover arm is the
			// only rung that paints anything here.
			tt.Move(35, 35)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			tt.Frame()

			// The box is the painted one: the framework's `ButtonBase` insets its own
			// 30u slot by a hair, so the theme's nominal size is not where the face
			// lands. Nothing else is painted on this ground, so the scan is exact.
			img := tt.Image()
			box := paintedBox(img, ground)
			if box.W == 0 {
				t.Fatal("the hovered icon button painted nothing")
			}
			x0, x1 := int(box.X+box.W*0.25), int(box.X+box.W*0.75)
			if face := lensMean(img, x0, int(box.Y+box.H/2), x1, int(box.Y+box.H/2)); channelDelta(face, ground) < 2 {
				t.Fatalf("the hovered icon button painted no face (pixel %v is the ground %v)", face, ground)
			}
			lensAssertE1(t, appearance.name, "the hovered icon button", img, box, int(box.Y)+4, ground)
		})
	}
}

// TestTheUninstallPartCardIsAFaceNotARing pins rows 9-7/9-8/9-9: the three
// arms of `uninstallPartRow` (fixed / hover / resting) all wore the four-layer
// lens over the card's own keyline. The resting arm is the one measured here —
// it is the rung the dialog opens on, and the other two share its frame.
//
// Mutation: put `tokens.Lens(card)` back in the `default` arm → the columns
// beside the frame carry `ControlEdge` and the bottom shade is covered.
func TestTheUninstallPartCardIsAFaceNotARing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			on := true
			var ref ui.Handle
			var box ui.Rect
			tt, ground := lensProbe(t, appearance.dark, 500, 200, func(c *ui.Context, a *App) {
				ui.Column(c).Width(360).Children(func() {
					a.uninstallPartRow(c, uninstallPart{
						Label: "Host configuration", Hint: "Remove the host configuration.", Value: &on,
					}).Bind(&ref)
					box = c.Resolve(ref).Bounds()
				})
			})
			tt.Frame()
			if box.W == 0 || box.H == 0 {
				t.Fatalf("the component card did not lay out: %+v", box)
			}
			lensAssertE1(t, appearance.name, "the uninstall component card", tt.Image(), box, int(box.Y)+4, ground)
		})
	}
}

// TestNoSettingsSurfaceControlCallsTheFourLayerLens is the family guard: this
// surface's controls are E1 faces (`LitFace`), and the four-layer lens must not
// come back without the contract changing. The one remaining caller in the tree
// is named in `docs/liquid-glass.md` §6 (`control.SegmentedIndex`'s chosen slot,
// row 9-11: its own accent ring is the identity line, and the ring covers both
// the rim and the edge, so a swap is invisible in pixels) — the toast's action
// chip used to be the other one and was retired in P13-j (§6.6 第 6-3 行).
func TestNoSettingsSurfaceControlCallsTheFourLayerLens(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("the package's own sources were not found")
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name) // #nosec G304 -- the package's own sources
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "tokens.Lens(") {
			t.Errorf("%s still calls the four-layer lens on a control: this surface's faces wear "+
				"`theme.Tokens.LitFace` (顶受光边 + 底对侧暗线，无环) — the ring is the `ControlEdge` "+
				"frame painted a second time, and it buries the rim (docs/liquid-glass.md §2, P13-i)", name)
		}
	}
}

// TestDumpP13ILensFacePNGs is this round's visual evidence, in one image per
// appearance: the same pill drawn with the four-layer lens (before) and with
// E1's pair (after), on the surface's own ground. Opt-in (`FLOTER_PNG=1`) so the
// suite does not write files; the tag names the pair.
func TestDumpP13ILensFacePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-i-pill-<tag>-<appearance>.png")
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
		tokens := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
		ground := lensGround()
		var refs [2]ui.Handle
		var boxes [2]ui.Rect
		pill := func(c *ui.Context, i int, apply func(ui.Element) ui.Element, label string) {
			th := c.Theme()
			e := ui.Row(c).Height(th.Space(6)).Padding(0, th.Space(4)).
				Radius(tokens.RadiusSM).Border(1, tokens.ControlEdge).
				Background(tokens.Control).AlignItems(ui.Center).Justify(ui.Center)
			e = apply(e).Bind(&refs[i])
			e.Children(func() {
				ui.Text(c, label).FontSize(tokens.Caption).FontWeight(600).TextColor(tokens.TextStrong)
			})
			boxes[i] = c.Resolve(refs[i]).Bounds()
		}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			ui.Column(c).FillWidth().FillHeight().Background(ground).Padding(16).Gap(12).
				Justify(ui.Center).Children(func() {
				pill(c, 0, tokens.Lens, "Lens · before")
				pill(c, 1, tokens.LitFace, "LitFace · after")
			})
		}, 320, 140)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		img := tt.Image()
		path := filepath.Join("/tmp", "floter-P13-i-pill-"+tag+"-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		// The readings the round turns on, logged so the report can quote them:
		// the row inside each pill's frame (the lit line, or the ring), the side
		// column beside it, and the face itself.
		for i, name := range []string{"Lens before", "LitFace after"} {
			b := boxes[i]
			x0, x1 := int(b.X+b.W*0.25), int(b.X+b.W*0.75)
			t.Logf("%s %s: crown %v, side column %v, face %v",
				path, name, lensMean(img, x0, int(b.Y)+1, x1, int(b.Y)+1),
				lensMean(img, int(b.X)+1, int(b.Y+b.H/2), int(b.X)+2, int(b.Y+b.H/2)),
				lensMean(img, x0, int(b.Y+b.H/2), x1, int(b.Y+b.H/2)))
		}
	}
}
