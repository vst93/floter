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

// P13-m · 设置页三块「`Control` 平涂 + 一圈画框、没有内线」的面。
//
// P13-i（十处 `Lens`）与 P13-l（两处漏网）用的都是**单行** grep：
//
//	grep -n "tokens.Control)" internal/settingsui/*.go | grep "Border(1"
//
// 这三处的 `Border(...)` 与 `Background(tokens.Control)` 不在同一行，所以两轮都
// 照不到它们——不是「判过之后决定不做」，是从来没进过视野。它们停在 P12-c 第 2-4
// 行点名的那个状态：「一块被画框圈住的平涂」。
//
//   - `configMultiselectBox`（`config.go`）：66u 的多选字段箱，注释写着「与
//     `fieldFace` 同面」，而 `fieldFace` 自 P12-c 起就是 E1；
//   - `commandPlate`（`settingsui.go`）：扩展页命令列表里每一条命令的块；
//   - `aliasField`（`textinput.go`）：面板字段族里唯一没有那一对线的字段。
//
// 三条像素 pin（每条深/浅两套外观，走 `lensAssertE1` 的四条判据：顶亮 / 底暗 /
// 两侧 = 面 / 块外无影）。

// TestTheMultiselectBoxIsAFaceNotARing pins §6.2b-2 的 9-15 行。
//
// The probe passes an empty `fn`: the pin grades the material, not the tick rows
// inside it, so the reading rows are the box's own padding.
//
// Mutations: drop the `LitFace` call → the row inside the top frame is the face;
// swap it for `tokens.Lens` → the side columns carry the ring; add a `Shadow(`
// → the box casts beside itself.
func TestTheMultiselectBoxIsAFaceNotARing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			var scroll ui.ScrollState
			tt, ground := lensProbe(t, appearance.dark, 420, 180, func(c *ui.Context, a *App) {
				a.configMultiselectBox(c, &scroll, func() {})
			})
			img := tt.Image()
			box := paintedBox(img, ground)
			if box.W < 300 || box.H < 50 {
				t.Fatalf("the painted box %+v is not the multiselect field's 66u pane", box)
			}
			lensAssertE1(t, appearance.name, "the multiselect box", img, box, int(box.Y)+4, ground)
		})
	}
}

// TestTheCommandPlateIsAFaceNotARing pins row 9-16. The plate is the extension
// page's command list row — the same rung as the group card P12-c retired on
// (§6.2 第 2-3 行), which is why its outer frame is the weak `Hairline` rather
// than `ControlEdge`.
//
// Mutation: drop the `LitFace` call → the row inside the top frame is the face.
func TestTheCommandPlateIsAFaceNotARing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			tt, ground := lensProbe(t, appearance.dark, 460, 280, func(c *ui.Context, a *App) {
				a.extensionMetrics(func() {
					a.commandPlate(c, a.copy(), "ext.demo", Command{
						ID: "demo", Name: "Demo", Description: "A demo command.",
						Aliases: []string{"dm"}, Enabled: true, Declared: true,
						Available: true, Alias: "dm",
					})
				})
			})
			img := tt.Image()
			box := paintedBox(img, ground)
			if box.W < 300 || box.H < 40 {
				t.Fatalf("the painted box %+v is not the command plate", box)
			}
			lensAssertE1(t, appearance.name, "the command plate", img, box, int(box.Y)+4, ground)
		})
	}
}

// TestTheAliasFieldIsAFaceNotARing pins row 9-17. The face row is read three
// units under the top frame rather than four: the field is 20u tall with 4u of
// padding, so row four is where its text starts.
//
// Mutation: drop the `LitFace` call → the row inside the top frame is the face.
func TestTheAliasFieldIsAFaceNotARing(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			tt, ground := lensProbe(t, appearance.dark, 320, 120, func(c *ui.Context, a *App) {
				value := "k"
				a.extensionMetrics(func() {
					a.aliasField(c, &value).Width(240)
				})
			})
			img := tt.Image()
			box := paintedBox(img, ground)
			if box.W < 200 || box.H < 18 {
				t.Fatalf("the painted box %+v is not the alias field", box)
			}
			lensAssertE1(t, appearance.name, "the alias field", img, box, int(box.Y)+3, ground)
		})
	}
}

// TestDumpP13MFacePNGs is this round's visual evidence: one image per
// appearance with the before/after pair of all three frames — the two the app
// draws (`ControlEdge` for the field, `Hairline` for the two panel blocks).
// Opt-in (`FLOTER_PNG=1`) so the suite does not write files.
func TestDumpP13MFacePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-m-faces-<appearance>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
		ground := lensGround()
		const boxW, boxH, left, top, gap = 240, 40, 16, 16, 10
		probe := func(c *ui.Context, frame ui.Color, lit bool) {
			e := ui.Row(c).Width(boxW).Height(boxH).Padding(0, c.Theme().Space(2.5)).
				Radius(tokens.RadiusSM).Border(1, frame).
				Background(tokens.Control).AlignItems(ui.Center)
			if lit {
				e = tokens.LitFace(e)
			}
			e.Children(func() {
				label := "flat"
				if lit {
					label = "LitFace"
				}
				ui.Text(c, label).FontSize(tokens.Caption).TextColor(tokens.TextSecondary)
			})
		}
		rows := []struct {
			name  string
			frame ui.Color
			lit   bool
		}{
			{"multiselect · flat", tokens.ControlEdge, false},
			{"multiselect · LitFace", tokens.ControlEdge, true},
			{"command plate · flat", tokens.Hairline, false},
			{"command plate · LitFace", tokens.Hairline, true},
			{"alias field · flat", tokens.Hairline, false},
			{"alias field · LitFace", tokens.Hairline, true},
		}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			ui.Column(c).FillWidth().FillHeight().Background(ground).Padding(16).Gap(gap).
				AlignItems(ui.Start).Children(func() {
				for _, row := range rows {
					probe(c, row.frame, row.lit)
				}
			})
		}, 320, 360)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		img := tt.Image()
		path := filepath.Join("/tmp", "floter-P13-m-faces-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		for i, row := range rows {
			b := ui.Rect{X: left, Y: float32(top + i*(boxH+gap)), W: boxW, H: boxH}
			x0, x1 := int(b.X+b.W*0.25), int(b.X+b.W*0.75)
			t.Logf("%s %s: crown %v, side column %v, shade %v, face %v", path, row.name,
				lensMean(img, x0, int(b.Y)+1, x1, int(b.Y)+2),
				lensMean(img, int(b.X)+1, int(b.Y+b.H/2), int(b.X)+2, int(b.Y+b.H/2)),
				lensMean(img, x0, int(b.Y+b.H)-3, x1, int(b.Y+b.H)-2),
				lensMean(img, x0, int(b.Y)+4, x1, int(b.Y)+4))
		}
		// The three real components, drawn by the app on the same ground, so the
		// numbers below are the ones the product actually paints (the six rows
		// above are the controlled before/after pair: same size, same frame).
		real := func(name string, w, h, faceY int, build func(c *ui.Context, a *App)) {
			tt, ground := lensProbe(t, dark, w, h, build)
			img := tt.Image()
			b := paintedBox(img, ground)
			x0, x1 := int(b.X+b.W*0.25), int(b.X+b.W*0.75)
			t.Logf("%s %s: box %+v, crown %v, side column %v, shade %v, face %v", path, name, b,
				lensMean(img, x0, int(b.Y)+1, x1, int(b.Y)+2),
				lensMean(img, int(b.X)+1, int(b.Y+b.H/2), int(b.X)+2, int(b.Y+b.H/2)),
				lensMean(img, x0, int(b.Y+b.H)-3, x1, int(b.Y+b.H)-2),
				lensMean(img, x0, int(b.Y)+faceY, x1, int(b.Y)+faceY))
		}
		real("real multiselect box", 420, 180, 4, func(c *ui.Context, a *App) {
			var scroll ui.ScrollState
			a.configMultiselectBox(c, &scroll, func() {})
		})
		real("real command plate", 460, 280, 4, func(c *ui.Context, a *App) {
			a.extensionMetrics(func() {
				a.commandPlate(c, a.copy(), "ext.demo", Command{
					ID: "demo", Name: "Demo", Description: "A demo command.",
					Aliases: []string{"dm"}, Enabled: true, Declared: true,
					Available: true, Alias: "dm",
				})
			})
		})
		real("real alias field", 320, 120, 3, func(c *ui.Context, a *App) {
			value := "k"
			a.extensionMetrics(func() { a.aliasField(c, &value).Width(240) })
		})
	}
}
