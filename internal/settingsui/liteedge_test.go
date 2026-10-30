package settingsui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P12-c · `docs/liquid-glass.md` §6.2 设置页：E1 的受光边与 E2 的冠线落到这一张表面。
//
// 这一支的材质不变量只有一条（§1 末尾）：**受光边严格亮于面**；elevation 的判据
// 三条（§2）：一个表面只有一个 E2、**E1 不许有外投影**、E2 必须两层影。E1 的落地
// 函数是 `theme.Tokens.LitFace`（顶 `LensRim` + 底 `LensEdge`，无影），E2 的冠线是
// `EdgeHighlight`。
//
// 五条 pin，每条都在深/浅两套外观上跑，全部量像素、不读注释：
//   - 侧栏选中页：E1（顶边亮于面，且块外没有投影）；
//   - 分组卡：E1（同上，且外圈仍是弱发丝，没有被换成控件边）；
//   - 字段：E1，且 `fieldFace` 与弹层的 `sheetFieldSlot` 是同一对线；
//   - 凹槽（开关）**反面**：凹 ≠ 浮，冠线不许落在顶上；
//   - 卸载对话框与保存告警：E2 的冠线是 `EdgeHighlight`，不是 E1 的 `LensRim`。
//
// 冠线怎么量：**不猜渲染器的合成算术**。`liteCrown` 把同一个面、同一个底色、
// 两条候选线各画一遍当参照，然后要求被量到的冠线离它「应该」那条更近。这样
// 一条 0.09 的线和一条 0.20 的线在两套外观上都分得开，而 pin 里没有一处写死
// 合成后的像素值——写死的那些会在下一次渲染器微调时变成噪声。

// liteFaceFloor is the face step an E1 block must show against the surface it
// sits on: §1 抬到 0.10/0.07 的理由是「面与底的差在 1x 屏上是 8~11 级」。
// 受光边在浅色底上是白压白（§3 末尾），所以**站起来**由面差负责，冠线是它上面
// 那一条光——两者都在，E1 才成立。
const liteFaceFloor = 8

// liteBrighter reports whether a is a strictly lighter pixel than b on all three
// channels: the lit edge is a neutral white line, so a real crown lifts all of
// them. A rim painted in one channel only would not be light.
func liteBrighter(a, b ui.Color) bool {
	return a.R > b.R && a.G > b.G && a.B > b.B
}

// liteDarker is liteBrighter's mirror, stated on the sum of the channels rather
// than one by one: the counter-side is a shade, and a shade over a *hued* fill
// (the on switch's accent track) lowers the two channels the hue carries while
// leaving the third where it was. The E1 lines themselves are neutral, so on
// those the sum test and the per-channel test agree.
func liteDarker(a, b ui.Color) bool {
	return int(a.R)+int(a.G)+int(a.B) < int(b.R)+int(b.G)+int(b.B)
}

// litePane is one reference pane: a fill on a ground, with one inset line along
// its top and one along its bottom.
type litePane struct {
	fill   ui.Color
	ground ui.Color
	crown  ui.Color
	shade  ui.Color
}

// liteMeasure draws a reference pane at the given ground and returns the two
// pixels a pane's own material is read from: the brightest of its first three
// inner rows (the 1px crown, plus the corner's antialiasing) and the darkest of
// its last three (the counter-side). Scanning rather than indexing is what lets
// one helper serve both appearances and every fill: a crown lands on a different
// row depending on how far the fill lifts it.
func liteMeasure(t *testing.T, tokens theme.Tokens, dark bool, p litePane) (crown, shade, face ui.Color) {
	t.Helper()
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(p.ground)
		ui.Column(c).Padding(20).Width(300).Children(func() {
			e := ui.Row(c).FillWidth().Height(48).Radius(tokens.RadiusSM).Background(p.fill)
			e.InsetShadow(0, 1, 0, 0, p.crown)
			e.InsetShadow(0, -1, 0, 0, p.shade)
			e.Bind(&ref)
		})
		box = c.Resolve(ref).Bounds()
	}, 400, 120)
	tt.SetDark(dark)
	tt.Frame()
	img := tt.Image()
	x := int(box.X) + int(box.W)/2
	face = imagePixel(img, x, int(box.Y+box.H/2))
	crown = imagePixel(img, x, int(box.Y))
	for dy := 1; dy < 3; dy++ {
		if q := imagePixel(img, x, int(box.Y)+dy); liteBrighter(q, crown) {
			crown = q
		}
	}
	shade = imagePixel(img, x, int(box.Y+box.H)-1)
	for dy := 1; dy < 3; dy++ {
		if q := imagePixel(img, x, int(box.Y+box.H)-1-dy); liteDarker(q, shade) {
			shade = q
		}
	}
	return crown, shade, face
}

// liteNearest says which of the two candidate lines a measured crown is, and
// fails when the two candidates are too close to tell apart — the same
// discriminating guard the card's edge pin uses, for the same reason: a
// comparison between two candidates that composite to the same pixel is a guard
// that can never fire.
func liteNearest(t *testing.T, what string, got, a, b ui.Color) (nearA bool) {
	t.Helper()
	gap := channelDelta(a, b)
	if gap < 3 {
		t.Fatalf("%s: the two candidate lines composite to %v and %v, %d level(s) apart — this pin cannot tell "+
			"them apart", what, a, b, gap)
	}
	return channelDelta(got, a) < channelDelta(got, b)
}

// liteRender draws the whole settings view at one appearance, settled past the
// rows' 120ms colour transition so a frame is never a mid-blend.
func liteRender(t *testing.T, dark bool, page int) (*ui.Tester, theme.Tokens) {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.selectPage(page)
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		a.View(c)
	}, 720, 900)
	tt.SetDark(dark)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	return tt, tokens
}

// liteTokens resolves the tokens of one appearance on a plain store.
func liteTokens(t *testing.T, dark bool) (theme.Tokens, ui.Color) {
	t.Helper()
	store := settings.NewStore(settings.Default())
	name := "light"
	if dark {
		name = "dark"
	}
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	return theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{}), ui.RGB(0, 0, 0)
}

// TestTheChosenPageStandsOnOneLitEdge pins §6.2 row 2-2: the sidebar's chosen
// page is E1 — its `RowSelected` face plus one 1px `LensRim` crown and the
// `LensEdge` counter-side — and it casts nothing onto the pane it lies on.
func TestTheChosenPageStandsOnOneLitEdge(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tt, tokens := liteRender(t, dark, PageGeneral)
			img := tt.Image()
			chosen := mustFind(t, tt, "General")
			// A column clear of both the row's rounded corner and its content:
			// the corner's antialiasing is brighter than any line inside it, and
			// the label's glyphs sit in the row's middle. The block's right end,
			// a few units in from the curve, is bare fill in every state.
			x := int(chosen.X+chosen.W) - 10
			face := imagePixel(img, x, int(chosen.Y+chosen.H/2))
			crown := imagePixel(img, x, int(chosen.Y))
			for dy := 1; dy < 3; dy++ {
				if q := imagePixel(img, x, int(chosen.Y)+dy); liteBrighter(q, crown) {
					crown = q
				}
			}
			// The block's last painted row: a rounded box's bottom edge is one
			// row below `Y+H` for this radius, so both are read and the darker
			// one is the counter-side.
			shade := imagePixel(img, x, int(chosen.Y+chosen.H)-1)
			for _, y := range []int{int(chosen.Y+chosen.H) - 1, int(chosen.Y + chosen.H), int(chosen.Y+chosen.H) - 2} {
				if q := imagePixel(img, x, y); liteDarker(q, shade) {
					shade = q
				}
			}
			pane := imagePixel(img, 2, int(chosen.Y+chosen.H/2))

			// ① The face rung: the block is visibly a 面 over the pane it sits
			// on — the whole of "standing up" on the light appearance.
			if step := channelDelta(face, pane); step < liteFaceFloor {
				t.Errorf("[%s] the chosen page's face %v sits %d levels over the pane %v, under the E1 floor %d: "+
					"the block reads as the ground it sits on", appearance.name, face, step, pane, liteFaceFloor)
			}
			// ② …plus one lit line on top and one shade under it. The crown is
			// compared against the same pane drawn with each candidate line, so
			// the pin does not have to re-derive the renderer's compositing.
			refRim, refShade, _ := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.RowSelected, ground: pane, crown: tokens.LensRim, shade: tokens.LensEdge,
			})
			refE2, _, _ := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.RowSelected, ground: pane, crown: tokens.EdgeHighlight, shade: tokens.LensEdge,
			})
			if !liteBrighter(crown, face) {
				t.Errorf("[%s] the chosen page's crown %v is not brighter than its own face %v: the lit edge is "+
					"missing (E1 = face + one 1px LensRim, liquid-glass.md §2)", appearance.name, crown, face)
			}
			if !liteNearest(t, "the chosen page's crown", crown, refRim, refE2) {
				t.Errorf("[%s] the chosen page's crown %v is nearer the E2 `EdgeHighlight` (%v) than the E1 `LensRim` "+
					"(%v): an E1 row wears the component rim", appearance.name, crown, refE2, refRim)
			}
			if !liteDarker(shade, face) {
				t.Errorf("[%s] the chosen page has no counter-side: its bottom inner row %v is not darker than its "+
					"face %v (顶亮一分、底暗一分，控件才站起来, §3)", appearance.name, shade, face)
			}
			if channelDelta(shade, refShade) > 2 {
				t.Errorf("[%s] the chosen page's counter-side = %v, want LensEdge over its face (%v)",
					appearance.name, shade, refShade)
			}
			// ③ E1 casts nothing (§2 judgement 2). The pixel just left of the
			// row, at its own height, must still be the pane.
			if side := imagePixel(img, 1, int(chosen.Y+chosen.H/2)); channelDelta(side, pane) > 4 {
				t.Errorf("[%s] the chosen page casts a shadow beside itself (%v against the pane %v): an E1 block "+
					"lies on the ground, it does not float over it", appearance.name, side, pane)
			}
		})
	}
}

// TestTheGroupedCardIsAnE1Pane pins §6.2 row 2-3: the grouped card keeps the
// contract's plane (`Control` + the *weak* hairline at radius 10) and gains the
// E1 crown — it is not a flat fill any more, and the crown is not the card's
// own edge.
func TestTheGroupedCardIsAnE1Pane(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tokens, _ := liteTokens(t, dark)
			// An opaque page under the card, so a token composited over it is
			// readable as the card's own face.
			page0 := ui.RGB(255, 255, 255)
			if dark {
				page0 = ui.RGB(0, 0, 0)
			}
			a := New(settings.NewStore(settings.Default()), Actions{})
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				c.Root().Background(page0)
				ui.Column(c).Padding(20).Children(func() {
					a.card(c, func() { a.row(c, "Row", "hint", func() {}) })
				})
			}, 400, 200)
			tt.Frame()
			img := tt.Image()
			page := imagePixel(img, 2, 2)
			// The card's own box, found by its extent: the outermost painted
			// columns and rows (the 20u page padding is the page colour on all
			// four sides).
			box := paintedBox(img, page)
			if box.W == 0 {
				t.Fatal("the card's box was not found")
			}
			x := int(box.X+box.W) - 8
			face := imagePixel(img, x, int(box.Y+box.H/2))
			crown := imagePixel(img, x, int(box.Y))
			for dy := 1; dy < 3; dy++ {
				if q := imagePixel(img, x, int(box.Y)+dy); liteBrighter(q, crown) {
					crown = q
				}
			}
			shade := imagePixel(img, x, int(box.Y+box.H)-1)
			for dy := 1; dy < 3; dy++ {
				if q := imagePixel(img, x, int(box.Y+box.H)-1-dy); liteDarker(q, shade) {
					shade = q
				}
			}
			// The card's own face: `Control` over the page, and it is *not* the
			// deprecated field rung — this pin still reads the material it names.
			if face == page {
				t.Fatalf("[%s] the card painted no face at all: %v", appearance.name, face)
			}
			refRim, _, refFace := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.Control, ground: page, crown: tokens.LensRim, shade: tokens.LensEdge,
			})
			refE2, _, _ := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.Control, ground: page, crown: tokens.EdgeHighlight, shade: tokens.LensEdge,
			})
			if channelDelta(face, refFace) > 3 {
				t.Errorf("[%s] the card's face = %v, want Control over the page (%v)", appearance.name, face, refFace)
			}
			if !liteBrighter(crown, face) {
				t.Errorf("[%s] the card's crown %v is not brighter than its face %v: the E1 crown is missing "+
					"(liquid-glass.md §6.2 第 2-3 行)", appearance.name, crown, face)
			}
			if !liteNearest(t, "the card's crown", crown, refRim, refE2) {
				t.Errorf("[%s] the card's crown %v is nearer the E2 `EdgeHighlight` (%v) than the E1 `LensRim` (%v)",
					appearance.name, crown, refE2, refRim)
			}
			if !liteDarker(shade, face) {
				t.Errorf("[%s] the card has no counter-side: its bottom inner row %v is not darker than its face %v",
					appearance.name, shade, face)
			}
			// The outer edge is still the *weak* hairline, not the control edge:
			// the crown is an inner line, it did not replace the card's outline.
			edge := imagePixel(img, x, int(box.Y))
			weak := panelOver(tokens.HairlineSoft, face)
			controlEdge := panelOver(tokens.ControlEdge, face)
			if channelDelta(weak, controlEdge) == 0 {
				t.Fatalf("[%s] HairlineSoft and ControlEdge composite to %v over this face: the pin can no longer tell "+
					"the two tiers apart", appearance.name, weak)
			}
			if channelDelta(edge, controlEdge) <= channelDelta(edge, weak) {
				t.Errorf("[%s] the card's edge = %v sits nearer the ControlEdge tier (%v) than the weak hairline (%v)",
					appearance.name, edge, controlEdge, weak)
			}
			// No cast: E1 lies on the ground (§2 judgement 2). The page outside
			// the card, at the card's own height, is untouched.
			if outside := imagePixel(img, int(box.X)-2, int(box.Y+box.H/2)); channelDelta(outside, page) > 4 {
				t.Errorf("[%s] the card casts a shadow beside itself (%v against the page %v): E1 draws no shadow",
					appearance.name, outside, page)
			}
		})
	}
}

// TestTheSettingsFieldIsOneLitSlot pins §6.2 row 2-4: the settings field and the
// overlay's field slot are the same E1 material — one face, one crown, one
// counter-side — so a config input and a plugin-sheet slot cannot drift into two
// materials.
func TestTheSettingsFieldIsOneLitSlot(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tokens, _ := liteTokens(t, dark)
			page0 := ui.RGB(255, 255, 255)
			if dark {
				page0 = ui.RGB(0, 0, 0)
			}
			a := New(settings.NewStore(settings.Default()), Actions{})
			value := ""
			var fieldBox, slotBox ui.Rect
			var fieldRef, slotRef ui.Handle
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				c.Root().Background(page0)
				ui.Column(c).Padding(24).Gap(16).Children(func() {
					a.textInput(c, &value).Bind(&fieldRef).Width(220).Height(48)
					a.sheetFieldSlot(c).Bind(&slotRef).Width(220).Height(48)
				})
				fieldBox = c.Resolve(fieldRef).Bounds()
				slotBox = c.Resolve(slotRef).Bounds()
			}, 400, 240)
			tt.Frame()
			img := tt.Image()
			page := imagePixel(img, 2, 2)
			refRim, refShade, refFace := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.Control, ground: page, crown: tokens.LensRim, shade: tokens.LensEdge,
			})
			refE2, _, _ := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.Control, ground: page, crown: tokens.EdgeHighlight, shade: tokens.LensEdge,
			})
			for _, b := range []struct {
				what string
				box  ui.Rect
			}{{"the settings field", fieldBox}, {"the overlay's field slot", slotBox}} {
				x := int(b.box.X) + 150
				face := imagePixel(img, x, int(b.box.Y+b.box.H/2))
				crown := imagePixel(img, x, int(b.box.Y))
				for dy := 1; dy < 3; dy++ {
					if q := imagePixel(img, x, int(b.box.Y)+dy); liteBrighter(q, crown) {
						crown = q
					}
				}
				shade := imagePixel(img, x, int(b.box.Y+b.box.H)-1)
				for dy := 1; dy < 3; dy++ {
					if q := imagePixel(img, x, int(b.box.Y+b.box.H)-1-dy); liteDarker(q, shade) {
						shade = q
					}
				}
				if channelDelta(face, refFace) > 3 {
					t.Errorf("[%s] %s's face = %v, want Control over the page (%v)", appearance.name, b.what, face, refFace)
				}
				if !liteBrighter(crown, face) {
					t.Errorf("[%s] %s has no E1 crown: its crown %v is not brighter than its face %v",
						appearance.name, b.what, crown, face)
				}
				if !liteNearest(t, b.what+"'s crown", crown, refRim, refE2) {
					t.Errorf("[%s] %s's crown %v is nearer the E2 `EdgeHighlight` (%v) than the E1 `LensRim` (%v)",
						appearance.name, b.what, crown, refE2, refRim)
				}
				if !liteDarker(shade, face) {
					t.Errorf("[%s] %s has no counter-side: its bottom inner row %v is not darker than its face %v "+
						"(顶亮一分、底暗一分，控件才站起来, §3)", appearance.name, b.what, shade, face)
				}
				if channelDelta(shade, refShade) > 2 {
					t.Errorf("[%s] %s's counter-side = %v, want LensEdge over its face (%v)",
						appearance.name, b.what, shade, refShade)
				}
			}
		})
	}
}

// TestTheRecessedGrooveHasNoCrown is the **counter-pin** to the four above: §6.2
// row 2-5. A groove is a recess, not a raised face — `LensInset` reads the same
// lens backwards (light pools along the bottom, the top is the dark one), and
// painting `LitFace` on it would light the wrong side and turn a hole into a
// button. `LensEdge`'s raise to 0.06 already reaches it through the shared lens
// family; this pin states that the *crown* must never arrive.
//
// The reading is taken on the **on** switch, at the end away from the thumb: an
// on switch's track is one uniform accent fill, so any line inside it is a line
// and not the state's own gradient. The claim is the direction of the recess —
// the top inner row is **not** brighter than the track it closes. A `LitFace`
// on the groove puts a 0.09 white line there, and that line lifts all three
// channels over both appearances (measured: dark +10, light +22 on the darkest
// channel), so the pin fires on the mutation rather than on a colour.
func TestTheRecessedGrooveHasNoCrown(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tokens, _ := liteTokens(t, dark)
			page0 := ui.RGB(255, 255, 255)
			if dark {
				page0 = ui.RGB(0, 0, 0)
			}
			a := New(settings.NewStore(settings.Default()), Actions{})
			on := true
			var ref ui.Handle
			var box ui.Rect
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				c.Root().Background(page0)
				ui.Column(c).Padding(24).Children(func() { a.switchControl(c, &on).Bind(&ref) })
				box = c.Resolve(ref).Bounds()
			}, 200, 120)
			tt.Frame()
			img := tt.Image()
			// The track's left end: the thumb of an on switch sits at the right.
			x := int(box.X) + 6
			top := imagePixel(img, x, int(box.Y)+1)
			body := imagePixel(img, x, int(box.Y+box.H/2))
			if liteBrighter(top, body) {
				t.Errorf("[%s] the switch's groove has a lit crown (%v over its own track %v): a recess is not a "+
					"raised face, `LensInset` lights the bottom (liquid-glass.md §6.2 第 2-5 行)",
					appearance.name, top, body)
			}
			// …and it really is the recessed lens, not a bare fill: the top inner
			// row is the *dark* side of the groove, measurably under the track.
			if !liteDarker(top, body) {
				t.Errorf("[%s] the switch's track has no recessed lens (top inner row %v, track %v): `LensInset` "+
					"must still read the lens backwards", appearance.name, top, body)
			}
		})
	}
}

// TestTheFloatingPanesWearTheE2Crown pins §6.2 rows 2-7 and 2-8: the two panes
// whose fill is `Float` — the uninstall dialog and the save-failure banner —
// wear the **E2** crown (`EdgeHighlight`), not the E1 one (`LensRim`). A float
// fill with a component rim is a pane that cannot say which storey it is on.
func TestTheFloatingPanesWearTheE2Crown(t *testing.T) {
	en := i18n.For("en").Settings

	// ① The uninstall dialog: `Float` + `ElevThrow` + `EdgeHighlight`.
	for _, appearance := range appearanceNames {
		t.Run("dialog/"+appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tokens, _ := liteTokens(t, dark)
			a := uninstallTestApp()
			a.Store = settings.NewStore(settings.Default())
			var ref ui.Handle
			var box ui.Rect
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				ui.Box(c).Fill().Background(ui.RGB(255, 255, 255))
				a.uninstallDialog(c, en, npmTool()).Bind(&ref)
				box = c.Resolve(ref).Bounds()
			}, 640, 480)
			tt.SetDark(dark)
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			img := tt.Image()
			// The pane's ground is the dialog's scrim: a fixed `rgba(0,0,0,0.34)`
			// over white (`uninstall.go`), which is what the reference panes below
			// are drawn on so the two crowns are measured on the same base.
			scrim := ui.RGB(168, 168, 168)
			// The pane's own padding, clear of the sentence: the middle of the
			// pane is text, and a glyph is not the face.
			x := int(box.X) + 6
			face := imagePixel(img, x, int(box.Y+box.H/2))
			crown := imagePixel(img, x, int(box.Y))
			for dy := 1; dy < 3; dy++ {
				if q := imagePixel(img, x, int(box.Y)+dy); liteBrighter(q, crown) {
					crown = q
				}
			}
			refE2, _, refFace := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.Float, ground: scrim, crown: tokens.EdgeHighlight, shade: tokens.LensEdge,
			})
			refE1, _, _ := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.Float, ground: scrim, crown: tokens.LensRim, shade: tokens.LensEdge,
			})
			if channelDelta(face, refFace) > 6 {
				t.Errorf("[%s] the dialog's face = %v, want Float over the scrim (%v)", appearance.name, face, refFace)
			}
			if !liteBrighter(crown, face) {
				t.Errorf("[%s] the dialog's crown %v is not brighter than its face %v", appearance.name, crown, face)
			}
			if !liteNearest(t, "the dialog's crown", crown, refE2, refE1) {
				t.Errorf("[%s] the dialog's crown %v is nearer the E1 `LensRim` (%v) than the E2 `EdgeHighlight` (%v): "+
					"a `Float` pane wears the floating rim", appearance.name, crown, refE1, refE2)
			}
		})
	}

	// ② The save-failure banner: `Float` + the E2 crown, and no cast (it rides
	// the page's scrolling column, so a broad throw would be clipped into a
	// hard edge — see the spec's 2-8 note).
	//
	// The band is drawn on a **mid-tone** page, not on white: `EdgeHighlight` on
	// the light appearance is 0.92 white, and a white crown over a white page is
	// 白压白 (§3 末尾) — unmeasurable rather than absent. A mid-tone ground makes
	// the same 1px line read on both appearances without changing the claim: the
	// crown is compared to the band's *own* face and to two reference panes on
	// that same ground.
	for _, appearance := range appearanceNames {
		t.Run("banner/"+appearance.name, func(t *testing.T) {
			dark := appearance.dark
			tokens, _ := liteTokens(t, dark)
			app := New(settings.NewStore(settings.Default()), Actions{})
			app.saveFailed = true
			app.saveAlertShown = true
			app.saveAlertProgress = 1
			ground := ui.RGB(128, 128, 128)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(20).Width(400).Children(func() {
					app.saveAlert(c, en)
				})
			}, 440, 120)
			tt.SetDark(dark)
			tt.Frame()
			img := tt.Image()
			box := paintedBox(img, ground)
			if box.W == 0 {
				t.Fatal("the banner's box was not found")
			}
			// The band's own padding, clear of the sentence: the middle of the
			// band is text, and a glyph is not the face.
			x := int(box.X) + 6
			face := imagePixel(img, x, int(box.Y+box.H/2))
			crown := imagePixel(img, x, int(box.Y))
			for dy := 1; dy < 3; dy++ {
				if q := imagePixel(img, x, int(box.Y)+dy); liteBrighter(q, crown) {
					crown = q
				}
			}
			refE2, _, refFace := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.Float, ground: ground, crown: tokens.EdgeHighlight, shade: tokens.LensEdge,
			})
			refE1, _, _ := liteMeasure(t, tokens, dark, litePane{
				fill: tokens.Float, ground: ground, crown: tokens.LensRim, shade: tokens.LensEdge,
			})
			if channelDelta(face, refFace) > 4 {
				t.Errorf("[%s] the save banner's face = %v, want Float over the page (%v)", appearance.name, face, refFace)
			}
			if !liteBrighter(crown, face) {
				t.Errorf("[%s] the save banner has no crown: its crown %v is not brighter than its face %v",
					appearance.name, crown, face)
			}
			if !liteNearest(t, "the save banner's crown", crown, refE2, refE1) {
				t.Errorf("[%s] the save banner's crown %v is nearer the E1 `LensRim` (%v) than the E2 `EdgeHighlight` (%v)",
					appearance.name, crown, refE1, refE2)
			}
			// No cast: the band rides the page's column, so its shadow would be
			// clipped by the scroll viewport into a hard edge (spec 2-8). The
			// page beside the band, at its own height, is untouched.
			if outside := imagePixel(img, int(box.X)-2, int(box.Y+box.H/2)); channelDelta(outside, ground) > 4 {
				t.Errorf("[%s] the save banner casts a shadow beside itself (%v against the page %v): this band is "+
					"not a floater yet, so it draws no throw (spec 2-8)", appearance.name, outside, ground)
			}
		})
	}
}

// TestDumpP12CLitEdgeFrames is the round's visual evidence: the settings page,
// the integrations page, the uninstall dialog and the save-failure banner in
// both appearances. Opt-in (`FLOTER_PNG=1`), like the other dumps: a test run
// should not litter /tmp. `FLOTER_PNG_TAG` names the side of the change
// (`before` / `after`) so the two sets sit side by side.
func TestDumpP12CLitEdgeFrames(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	en := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		dark := appearance.dark
		name := appearance.name

		// The settings pages: the sidebar's chosen page, the grouped cards and
		// the fields.
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		a.selectPage(PageGeneral)
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 900)
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P12-c-general-"+name+"-"+tag+".png"), tt)

		// The uninstall dialog: the E2 pane.
		a2 := uninstallTestApp()
		a2.Store = settings.NewStore(settings.Default())
		var ref ui.Handle
		tt2 := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.RGB(255, 255, 255))
			a2.uninstallDialog(c, en, npmTool()).Bind(&ref)
		}, 640, 480)
		tt2.SetDark(dark)
		tt2.Frame()
		tt2.Advance(200 * time.Millisecond)
		tt2.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P12-c-dialog-"+name+"-"+tag+".png"), tt2)

		// The save-failure banner: the other E2 pane.
		a3 := New(settings.NewStore(settings.Default()), Actions{})
		a3.saveFailed = true
		a3.saveAlertShown = true
		a3.saveAlertProgress = 1
		tt3 := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Padding(20).Width(680).Children(func() { a3.saveAlert(c, en) })
		}, 720, 200)
		tt3.SetDark(dark)
		tt3.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P12-c-banner-"+name+"-"+tag+".png"), tt3)
	}
}
