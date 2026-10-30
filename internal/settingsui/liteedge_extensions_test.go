package settingsui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-e · `docs/liquid-glass.md` §6.4 扩展页：E1 的受光边落到集成行与动作胶囊。
//
// 这一页此前**已经**有两级：「底」是页面（`Sunken` + 外层 tint），「面」是分组卡
// （P12-c 的 2-3 给 `cardGeometry` 戴上了 `LitFace`）。缺的是第三级——**被选中的
// 那一行**。它只是一层 `RowSelected` 平涂，与它脚下那张 E1 卡只差一档 alpha，
// 读起来是「卡里又刷了一块漆」。本轮的 pin 就钉这一对线，并反手钉住三件不该变的
// 事：破损行**不拿**冠线（它那条线已被警告色内环占住）、胶囊的冠线是 E1 的
// `LensRim` 而不是 E2 的 `EdgeHighlight`、E1 一律不画投影（§2 判据 2）。
//
// 怎么量：与 P12-c 同一套机制——`liteMeasure` 把同一个面、同一个底色、两条候选线
// 各画一遍当参照，再要求量到的冠线离它「应该」那条更近。pin 里没有一处写死渲染器
// 合成出来的像素值。

// liteRowFrame draws one row face on a constant ground and finds its painted
// extent: the row is the only thing drawn on that ground, so its own fill is
// every pixel the ground does not own at the scan column. Scanning rather than
// computing from the layout is what lets the pin survive a row that grows with
// its content (the row's 44px floor is a *minimum*).
type liteRowFrame struct {
	tt     *ui.Tester
	x      int
	top    int
	bottom int
	ground ui.Color
}

// drawLiteRow renders one `cardRow` wearing the face `face` returns, on a
// constant ground, and resolves the row's own box.
func drawLiteRow(t *testing.T, tokens theme.Tokens, dark bool, ground ui.Color, face func(c *ui.Context) rowFace) *liteRowFrame {
	t.Helper()
	var labelRef ui.Handle
	var labelBox ui.Rect
	var inset float32
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Width(300).Children(func() {
			a := New(settings.NewStore(settings.Default()), Actions{})
			a.cardRows = 0 // the row is the card's first: no separator above it
			inset = c.Theme().Space(3.5)
			a.cardRow(c, ui.Center, nil, face(c), func() {}, func() {
				ui.Text(c, "V Tools").Bind(&labelRef)
			})
		})
		labelBox = c.Resolve(labelRef).Bounds()
	}, 400, 160)
	tt.SetDark(dark)
	tt.Frame()

	frame := &liteRowFrame{tt: tt, ground: ground}
	// A column inside the row but clear of the leading-rule slot (1px) and of
	// the label's glyphs: the row's own fill is bare there in every state.
	frame.x = int(labelBox.X-inset) + 6
	img := tt.Image()
	frame.top, frame.bottom = -1, -1
	for y := 0; y < img.Rect.Dy(); y++ {
		if channelDelta(imagePixel(img, frame.x, y), ground) <= 2 {
			continue
		}
		if frame.top < 0 {
			frame.top = y
		}
		frame.bottom = y
	}
	if frame.top < 0 {
		t.Fatalf("no row at x=%d: the column is the bare ground %v", frame.x, ground)
	}
	return frame
}

// face is the pixel in the middle of the block: the row's own rung.
func (f *liteRowFrame) face() ui.Color {
	return imagePixel(f.tt.Image(), f.x, (f.top+f.bottom)/2)
}

// crown is the brightest of the block's first three inner rows (the 1px lit
// edge plus the corner's antialiasing).
func (f *liteRowFrame) crown() ui.Color {
	img := f.tt.Image()
	c := imagePixel(img, f.x, f.top)
	for dy := 1; dy < 3; dy++ {
		if q := imagePixel(img, f.x, f.top+dy); liteBrighter(q, c) {
			c = q
		}
	}
	return c
}

// shade is the darkest of the block's last three rows: the counter-side.
func (f *liteRowFrame) shade() ui.Color {
	img := f.tt.Image()
	s := imagePixel(img, f.x, f.bottom)
	for dy := 1; dy < 3; dy++ {
		if q := imagePixel(img, f.x, f.bottom-dy); liteDarker(q, s) {
			s = q
		}
	}
	return s
}

// TestTheNamedIntegrationRowStandsOnOneLitEdge pins §6.4 row 4-1: the row a
// `floter://register` link named is an **E1 block** — its `RowSelected` face
// plus one 1px `LensRim` crown and one 1px `LensEdge` counter-side — and it
// casts nothing onto the ground it lies on.
//
// Mutation M1 (drop `Lit: true` from the Highlighted arm) turns the crown
// assertion red; M2 (paint `EdgeHighlight` instead of the component rim) turns
// the "nearest candidate" assertion red.
func TestTheNamedIntegrationRowStandsOnOneLitEdge(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tokens, _ := liteTokens(t, dark)
			// A mid-tone ground: `RowSelected` is a white wash on the dark
			// appearance and a black one on the light, so a mid grey is the one
			// base on which the face stands off in *both* (a white page would be
			// 白压白 on the light side, §3 末尾).
			ground := ui.RGB(128, 128, 128)
			f := drawLiteRow(t, tokens, dark, ground, func(c *ui.Context) rowFace {
				a := New(settings.NewStore(settings.Default()), Actions{})
				return a.integrationRowFace(c, Integration{ID: "dev.floter.reg", Highlighted: true})
			})
			face, crown, shade := f.face(), f.crown(), f.shade()

			// ① The face is the contract's neutral selection rung over the ground,
			// compared against the same wash drawn in a reference pane.
			refRim, refShade, refFace := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.RowSelected, ground: ground, crown: tokens.LensRim, shade: tokens.LensEdge,
			})
			refE2, _, _ := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.RowSelected, ground: ground, crown: tokens.EdgeHighlight, shade: tokens.LensEdge,
			})
			if channelDelta(face, refFace) > 6 {
				t.Errorf("[%s] the named row's face = %v, want RowSelected over the ground (%v)",
					appearance.name, face, refFace)
			}
			// ② …plus the two E1 lines. The crown is compared against the same face
			// drawn with each candidate line, so the pin never re-derives the
			// renderer's compositing.
			if !liteBrighter(crown, face) {
				t.Errorf("[%s] the named row's crown %v is not brighter than its own face %v: the row is a wash "+
					"of paint, not a block (E1 = face + one 1px LensRim, liquid-glass.md §2 / §6.4 第 4-1 行)",
					appearance.name, crown, face)
			}
			if !liteNearest(t, "the named row's crown", crown, refRim, refE2) {
				t.Errorf("[%s] the named row's crown %v is nearer the E2 `EdgeHighlight` (%v) than the E1 "+
					"`LensRim` (%v): an E1 block wears the component rim", appearance.name, crown, refE2, refRim)
			}
			if !liteDarker(shade, face) {
				t.Errorf("[%s] the named row has no counter-side: its bottom inner row %v is not darker than its "+
					"face %v (顶亮一分、底暗一分，控件才站起来, §3)", appearance.name, shade, face)
			}
			if channelDelta(shade, refShade) > 2 {
				t.Errorf("[%s] the named row's counter-side = %v, want LensEdge over its face (%v)",
					appearance.name, shade, refShade)
			}
			// ③ E1 casts nothing (§2 判据 2): the pixels immediately above and below
			// the block, at the same column, are still the ground.
			img := f.tt.Image()
			if above := imagePixel(img, f.x, f.top-1); channelDelta(above, ground) > 4 {
				t.Errorf("[%s] the named row casts a shadow above itself (%v against the ground %v): an E1 block "+
					"lies on the ground, it does not float over it", appearance.name, above, ground)
			}
			if below := imagePixel(img, f.x, f.bottom+1); channelDelta(below, ground) > 4 {
				t.Errorf("[%s] the named row casts a shadow below itself (%v against the ground %v)",
					appearance.name, below, ground)
			}
		})
	}
}

// TestTheBrokenRowKeepsItsWarningLineAndGetsNoCrown pins §6.4 row 4-4: a broken
// row already spends its one line on the warning keyline (`Inset`, four sides),
// so it must **not** also wear the E1 crown — a second line in the same 1px row
// would wash 「这块坏了」 into 「这块被选中了」.
//
// Mutation M3 (`Lit: true` on the Broken arm) turns the structural assertion
// red, and the crown assertion with it.
func TestTheBrokenRowKeepsItsWarningLineAndGetsNoCrown(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tokens, _ := liteTokens(t, dark)
			ground := ui.RGB(128, 128, 128)

			// The structural half: the face asks for the warm pane and the warning
			// keyline, and for no lit edge.
			var face rowFace
			a := New(settings.NewStore(settings.Default()), Actions{})
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				face = a.integrationRowFace(c, Integration{ID: "dev.floter.broken", Broken: true})
			}, 60, 40)
			tt.Frame()
			if face.Lit {
				t.Errorf("[%s] the broken row asks for the E1 crown (Lit = true); its one line is the warning "+
					"keyline (liquid-glass.md §6.4 第 4-4 行)", appearance.name)
			}
			if face.Face != tokens.RaisedWarm || face.Inset.A == 0 {
				t.Errorf("[%s] the broken row's face %v / keyline %v are not the warm pane under the warning ring",
					appearance.name, face.Face, face.Inset)
			}

			// The pixel half: whatever the warning keyline composites to on the
			// top row, `LitFace`'s neutral white line must not be there. A white
			// crown lifts all three channels; the warm keyline does not, so the
			// absence is stated as "the top row is not the neutral lit axis".
			f := drawLiteRow(t, tokens, dark, ground, func(c *ui.Context) rowFace {
				b := New(settings.NewStore(settings.Default()), Actions{})
				return b.integrationRowFace(c, Integration{Broken: true})
			})
			crown, plane := f.crown(), f.face()
			if liteBrighter(crown, plane) && crown.R == crown.B && crown.G == crown.B {
				t.Errorf("[%s] the broken row's top line %v is a neutral lit crown over its face %v: its one line "+
					"belongs to the warning ring", appearance.name, crown, plane)
			}
		})
	}
}

// TestTheExtensionActionPillWearsTheE1Crown pins §6.4 row 4-3 ("只需复核"):
// the panel's `.extensions-action-button` pill carries the component rim
// (`Lens`'s first layer is `LensRim`, re-based to 0.09 by P12-a) and **not** the
// floating `EdgeHighlight` — and, being an E1 control, it casts nothing.
//
// Mutation M4 (drop `tokens.Lens(button)` from `extensionActionFace`) turns the
// crown assertion red.
func TestTheExtensionActionPillWearsTheE1Crown(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tokens, _ := liteTokens(t, dark)
			ground := ui.RGB(128, 128, 128)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(20).Children(func() {
					a := New(settings.NewStore(settings.Default()), Actions{})
					a.actionButton(c, "Check")
				})
			}, 400, 120)
			tt.SetDark(dark)
			tt.Frame()
			img := tt.Image()
			box := paintedBox(img, ground)
			if box.W == 0 {
				t.Fatal("the pill's box was not found")
			}
			// The pill's own 1px border is the outermost painted row; the crown is
			// the row just inside it, and the face is the middle of the pill.
			x := int(box.X) + 8
			crown := imagePixel(img, x, int(box.Y)+1)
			for dy := 2; dy < 5; dy++ {
				if q := imagePixel(img, x, int(box.Y)+dy); liteBrighter(q, crown) {
					crown = q
				}
			}
			plane := imagePixel(img, x, int(box.Y+box.H/2))
			// The two candidate pills: the same four-layer stack the control wears
			// (`Control` face, the `ControlEdge` hairline, `LensSheen`, `LensEdge`)
			// with **only** the lit line swapped. Holding every other layer constant
			// is what makes the comparison a comparison: the broad sheen sits in the
			// same rows in all three, so the difference the pin reads is the rim's
			// own alpha and nothing else.
			refRim := litePillCrown(t, tokens, dark, tokens.LensRim)
			refE2 := litePillCrown(t, tokens, dark, tokens.EdgeHighlight)
			refFace, _ := liteTokens(t, dark)
			_ = refFace
			if !liteBrighter(crown, plane) {
				t.Errorf("[%s] the pill has no crown: its top inner row %v is not brighter than its face %v "+
					"(§6.4 第 4-3 行: the resting pill wears the component rim)", appearance.name, crown, plane)
			}
			if !liteNearest(t, "the pill's crown", crown, refRim, refE2) {
				t.Errorf("[%s] the pill's crown %v is nearer the floating `EdgeHighlight` (%v) than the E1 "+
					"`LensRim` (%v): only a `Float` pane wears the E2 line (P12-c 的 2-7/2-8)",
					appearance.name, crown, refE2, refRim)
			}
			// No cast: an E1 control adds no outer shadow (§2 判据 2). The ground
			// just left of the pill, at its own height, is untouched.
			if outside := imagePixel(img, int(box.X)-2, int(box.Y+box.H/2)); channelDelta(outside, ground) > 4 {
				t.Errorf("[%s] the pill casts a shadow beside itself (%v against the ground %v)",
					appearance.name, outside, ground)
			}
		})
	}
}

// litePillCrown draws the action pill's own four-layer stack with its lit line
// replaced by `crown`, and returns the pixel that line paints: the brightest of
// the pill's first inner rows, the same way the real pill is read. Everything
// else in the stack is the control's own (`Control` face, `ControlEdge` hairline,
// `LensSheen`, `LensEdge`), so the only variable left is which line is lit.
func litePillCrown(t *testing.T, tokens theme.Tokens, dark bool, crown ui.Color) ui.Color {
	t.Helper()
	ground := ui.RGB(128, 128, 128)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() {
			pill := ui.ButtonBase(c).Height(27).Padding(0, 10).Radius(13.5).
				Border(1, tokens.ControlEdge).Background(tokens.Control)
			pill.InsetShadow(0, 1, 0, 0, crown)
			pill.InsetShadow(0, 6, 10, -6, tokens.LensSheen)
			pill.InsetShadow(0, -1, 0, 0, tokens.LensEdge)
			pill.InsetShadow(0, 0, 0, 1, tokens.ControlEdge)
			pill.Children(func() { ui.Text(c, "Check") })
		})
	}, 400, 120)
	tt.SetDark(dark)
	tt.Frame()
	img := tt.Image()
	box := paintedBox(img, ground)
	if box.W == 0 {
		t.Fatal("the reference pill's box was not found")
	}
	x := int(box.X) + 8
	got := imagePixel(img, x, int(box.Y)+1)
	for dy := 2; dy < 5; dy++ {
		if q := imagePixel(img, x, int(box.Y)+dy); liteBrighter(q, got) {
			got = q
		}
	}
	return got
}

// TestDumpP13ELitEdgeFrames is the round's visual evidence: the Integrations
// page (with one highlighted and one plain integration) and the action pill, in
// both appearances. Opt-in (`FLOTER_PNG=1`) like the other dumps.
// `FLOTER_PNG_TAG` names the side of the change (`before` / `after`).
func TestDumpP13ELitEdgeFrames(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, appearance := range appearanceNames {
		dark := appearance.dark
		name := appearance.name

		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		a.Integrations = func() []Integration {
			return []Integration{
				{ID: "plain", Name: "Plain Tool", Enabled: true, Running: true, RuntimeAvailable: true,
					Description: "A tool nothing linked to."},
				{ID: "dev.floter.reg", Name: "Register Me", Enabled: true, Running: true,
					RuntimeAvailable: true, Highlighted: true,
					Description: "The row a floter://register link named."},
			}
		}
		a.selectPage(PageIntegrations)
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 900)
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-e-integrations-"+name+"-"+tag+".png"), tt)

		tt2 := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ui.RGB(128, 128, 128))
			ui.Column(c).Padding(20).Children(func() {
				b := New(settings.NewStore(settings.Default()), Actions{})
				ui.Row(c).Gap(8).Children(func() {
					b.actionButton(c, "Check")
					b.actionButton(c, "Re-scan")
				})
			})
		}, 400, 120)
		tt2.SetDark(dark)
		tt2.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-e-pill-"+name+"-"+tag+".png"), tt2)
	}
}
