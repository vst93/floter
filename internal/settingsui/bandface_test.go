package settingsui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-l · 设置页最后两块「`Control` 平涂 + 一圈 `ControlEdge` 画框」的面。
//
// 这一族在 P12-c 第 2-4 行就被点过名（`fieldFace` 当时「只停在了一块被画框圈住
// 的平涂」），P13-i 又把十处同形状的控件面逐个换掉——但这两处漏在框外，因为它们
// 不是 `Lens` 的调用点，`glasscensus_test.go` 的「透镜被故意退掉」账本也就照不到
// 它们：
//
//   - `shortcutRecorder`（`settingsui.go`）：快捷键页与自定义工具表单共用的
//     82u × 27u 录制钮，静息面 `Control` + `ControlEdge`，**没有内线**；
//   - `noticeBand` 的**非危险臂**：同一个组件，`danger` 那条叠了一圈警告环，
//     非危险那条只剩发丝——`disableConfirmation` 的带子因此比它坐着的分组卡
//     （E1，P12-c 第 2-3 行）还平。
//
// 三条像素 pin（每条都在深/浅两套外观上跑）：
//   - `TestTheShortcutRecorderIsAFaceNotARing`；
//   - `TestTheNoticeBandIsAFaceNotARing`（非危险臂）；
//   - `TestTheDangerNoticeBandKeepsItsWarningRing`（危险臂的**反面** pin：它那条
//     身份线是警告环，不是 E1 的受光边，所以 `LitFace` 故意不加在那一臂上）。

// lensWarmth is how far a colour leans amber (r − b). The warning ink is warm on
// both appearances (`#9C1F18` light, `rgba(255,178,164,.82)` dark) while every
// neutral ink — the face, `LensRim`'s white and its shade — sits at r ≈ b, so
// one subtraction tells the two identity lines apart without naming a value.
func lensWarmth(c ui.Color) int { return int(c.R) - int(c.B) }

// recorderLabel is the chord the recorder's probe draws. It is a plain string so
// the pin grades the material, not the shortcut table.
const recorderLabel = "⌘⇧K"

// TestTheShortcutRecorderIsAFaceNotARing pins §6.2b's 9-13 row: the recorder is
// an E1 face over its own `ControlEdge` frame. It is the same shape P12-c
// retired on the field and P13-i retired on ten controls.
//
// Mutation checks (each one measured before this pin landed):
//   - put `tokens.Lens(button)` back → the side columns carry `ControlEdge` and
//     the bottom shade is covered by the ring's white row (dark);
//   - drop the `LitFace` call → the row inside the top frame is the face;
//   - add a `Shadow(` → the block casts beside itself.
func TestTheShortcutRecorderIsAFaceNotARing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			var ref ui.Handle
			var box ui.Rect
			tt, ground := lensProbe(t, appearance.dark, 240, 120, func(c *ui.Context, a *App) {
				// The recorder returns nothing to bind, so it is wrapped in a
				// box that shrinks to it: the wrapper's bounds are the button's.
				ui.Column(c).Bind(&ref).Children(func() {
					a.shortcutRecorder(c, "settings.shortcut.record", recorderLabel, func() {})
				})
				box = c.Resolve(ref).Bounds()
			})
			tt.Frame()
			img := tt.Image()
			if box.W < 60 || box.H < 20 || box.H > 34 {
				t.Fatalf("the resolved box %+v is not the recorder's 82u × 27u pane", box)
			}
			lensAssertE1(t, appearance.name, "the shortcut recorder", img, box, int(box.Y)+4, ground)
		})
	}
}

// TestTheNoticeBandIsAFaceNotARing pins row 9-14: the neutral arm of the notice
// band. `disableConfirmation` is the caller — the band under an integration's
// switch, which appears when the switch is turned off.
//
// Mutation: delete the `else { tokens.LitFace(band) }` arm → the row inside the
// band's top frame is the face, and the band reads as a flat rectangle again.
func TestTheNoticeBandIsAFaceNotARing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			tt, ground := lensProbe(t, appearance.dark, 460, 140, func(c *ui.Context, a *App) {
				a.noticeBand(c, false, "Stop the integration? Its data stays.", func(*ui.Element) {})
			})
			tt.Frame()
			img := tt.Image()
			box := paintedBox(img, ground)
			if box.W == 0 || box.H == 0 {
				t.Fatalf("the notice band painted nothing on the ground %v", ground)
			}
			lensAssertE1(t, appearance.name, "the notice band", img, box, int(box.Y)+4, ground)
			// The arm's identity is a neutral pane: no warning ink on its top
			// inner row (the danger arm's pin reads the other side of this).
			if warm := lensWarmth(lensMean(img, int(box.X+box.W*0.25), int(box.Y)+1,
				int(box.X+box.W*0.75), int(box.Y)+1)); warm > 12 {
				t.Errorf("[%s] the neutral notice band's top inner row is warm (r-b = %d): only the danger arm "+
					"carries the warning ring", appearance.name, warm)
			}
		})
	}
}

// TestTheDangerNoticeBandKeepsItsWarningRing is the reverse pin for the same
// component: the `danger` arm says one thing — its amber ring — and `LitFace` is
// deliberately **not** added there. `InsetShadow(0,0,0,1,…)`'s ring occupies the
// first padding row, which is the same row `LitFace`'s `y=1` rim would paint, and
// the later layer wins: adding E1 there washes the ring 9% white on top and
// darkens it 6% below (measured: light top `156,31,24` → `165,51,45`, bottom
// `156,31,24` → `147,29,23`). The ring stops being one colour.
//
// The judgement is therefore **symmetry**, not one pixel's value: a 1px ring is
// the same colour on all four sides, while E1's pair is by definition brighter
// above and darker below. Comparing the top row with the bottom row survives a
// layout that lands the ring on half-covered rows (both sides blend the same
// way); the warmth check on top of it says which ring it is.
//
// Mutation: add `tokens.LitFace(band)` to the `danger` arm → top and bottom
// drift apart (the pair lands on the ring) and this pin fails.
func TestTheDangerNoticeBandKeepsItsWarningRing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			tt, ground := lensProbe(t, appearance.dark, 460, 140, func(c *ui.Context, a *App) {
				a.noticeBand(c, true, "Discard the edits?", func(*ui.Element) {})
			})
			tt.Frame()
			img := tt.Image()
			box := paintedBox(img, ground)
			if box.W == 0 || box.H == 0 {
				t.Fatalf("the danger notice band painted nothing on the ground %v", ground)
			}
			x0, x1 := int(box.X+box.W*0.25), int(box.X+box.W*0.75)
			top := lensMean(img, x0, int(box.Y)+1, x1, int(box.Y)+1)
			bottom := lensMean(img, x0, int(box.Y+box.H)-2, x1, int(box.Y+box.H)-2)
			if warm := lensWarmth(top); warm < 25 {
				t.Errorf("[%s] the danger band's top inner row is %v (r-b = %d), not the warning ring: the "+
					"band's identity line is the amber keyline (liquid-glass.md §6.2b 第 9-14 行)",
					appearance.name, top, warm)
			}
			if delta := channelDelta(top, bottom); delta > 4 {
				t.Errorf("[%s] the danger band's ring is %v on top but %v below: E1's lit pair is painted "+
					"over the warning ring. A ring is one colour on all four sides — that is why `LitFace` is "+
					"deliberately not added to this arm (§6.2b 第 9-14b 行)", appearance.name, top, bottom)
			}
		})
	}
}

// TestDumpP13LFacePNGs is this round's visual evidence: one image per
// appearance with the before/after pair of both faces, drawn on the surface's
// own ground. Opt-in (`FLOTER_PNG=1`) so the suite does not write files.
func TestDumpP13LFacePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-l-faces-<appearance>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
		ground := lensGround()
		// Every row is the same 82u × 27u box at a fixed place, so the four
		// readings grade the material and not the layout: the only thing that
		// differs between a before/after pair is the line inside the frame.
		const boxW, boxH, top, gap = 110, 27, 16, 10
		probe := func(c *ui.Context, label string, apply func(ui.Element) ui.Element) {
			e := ui.Row(c).Width(boxW).Height(boxH).Padding(0, c.Theme().Space(2.5)).
				Radius(tokens.RadiusSM).Border(1, tokens.ControlEdge).
				Background(tokens.Control).AlignItems(ui.Center).Justify(ui.Center)
			e = apply(e)
			e.Children(func() {
				ui.Text(c, label).Font(deepLinkValueFont).FontSize(tokens.Caption).
					TextColor(tokens.TextSecondary)
			})
		}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			ui.Column(c).FillWidth().FillHeight().Background(ground).Padding(16).Gap(gap).
				AlignItems(ui.Start).Children(func() {
				probe(c, "recorder · Lens", tokens.Lens)
				probe(c, "recorder · LitFace", tokens.LitFace)
				probe(c, "band · flat", func(e ui.Element) ui.Element { return e })
				probe(c, "band · LitFace", tokens.LitFace)
			})
		}, 320, 210)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		img := tt.Image()
		path := filepath.Join("/tmp", "floter-P13-l-faces-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		for i, name := range []string{"recorder Lens", "recorder LitFace", "band flat", "band LitFace"} {
			b := ui.Rect{X: top, Y: float32(top + i*(boxH+gap)), W: boxW, H: boxH}
			x0, x1 := int(b.X+b.W*0.25), int(b.X+b.W*0.75)
			t.Logf("%s %s: crown %v, side column %v, shade %v, face %v", path, name,
				lensMean(img, x0, int(b.Y)+1, x1, int(b.Y)+1),
				lensMean(img, int(b.X)+1, int(b.Y+b.H/2), int(b.X)+2, int(b.Y+b.H/2)),
				lensMean(img, x0, int(b.Y+b.H)-2, x1, int(b.Y+b.H)-2),
				lensMean(img, x0, int(b.Y)+4, x1, int(b.Y)+4))
		}
	}
}
