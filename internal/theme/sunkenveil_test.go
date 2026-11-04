package theme

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/settings"
)

// P13-gc · 内容页那一档的 0.09：在 veil 之下，它必须仍是一步**看得见**的面差。
//
// `Sunken` 是画在「whatever 之上」的一层薄罩（§2.1 第 1 条：面档不随滑块缩放），
// 所以它的级数不靠 alpha 直接读，要靠**合成之后**的像素：面填在框的可见像素上，
// 差多少级 = `recess × |面的自有颜色 − 地面|`。上一轮量的是浮层的 0.10（§2.2），
// 这一轮是同一个 `overFrame` 的孪生：内容页的 0.09。
//
// 探针地面取中灰 `RGB(128,128,128)`（与 P13-ga §6.146 同一块底），判据是 §1 的
// 8 级下沿（`liteFaceFloor` 的口径：面与它压着的面差 ≥ 8 级）。
const veilFaceFloor = 8

// veilDesk is the probe ground: a mid-grey desktop. A wallpaper the colour of the
// fill itself would read 0 levels on every platform (that is `sunkennear_test.go`'s
// subject, and it is why the recess cannot be judged against the app's own colour).
var veilDesk = ui.RGB(128, 128, 128)

// veilCell is one platform/stop/slider cell of the probe.
type veilCell struct {
	goos    string
	step    string
	opacity uint8
	label   string
}

func veilCells() []veilCell {
	return []veilCell{
		{"linux", "liquid", 10, "linux liquid 10%"},
		{"linux", "liquid", 47, "linux liquid 47%"},
		{"darwin", "frosted", 47, "darwin frosted 47%"},
		{"darwin", "regular", 47, "darwin regular 47%"},
		{"darwin", "liquid", 90, "darwin liquid 90%"},
		{"darwin", "liquid", 47, "darwin liquid 47%"},
		// 最极端的一格：材质把 10% 的框托到 0.34，内容页的填充只有 0.09。
		{"darwin", "liquid", 10, "darwin liquid 10%"},
	}
}

const (
	veilCellW = 132
	veilCellH = 112
	veilGap   = 12
)

// veilRender draws one cell per entry: the frame's fill on the grey desk, the
// content pane's fill on the frame, and a control's fill on the pane. The three
// are painted as the tokens the app paints (translucent per-appearance fills),
// so the pixels the assertions read are the renderer's answer, not arithmetic.
func veilRender(t *testing.T, dark bool, cells []veilCell, paneAlpha func(i int, spec glassmap.Spec, tk Tokens) ui.Color) *image.RGBA {
	t.Helper()
	s := settings.Default()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.DarkTheme())
		ui.Row(c).FillWidth().FillHeight().Background(veilDesk).Padding(veilGap).Gap(veilGap).Children(func() {
			for i, cell := range cells {
				cell := cell
				st := s
				st.GlassStep = cell.step
				st.MainOpacity = cell.opacity
				tk := ForPlatform(st, dark, ui.Preferences{}, cell.goos)
				spec := glassmap.SpecForPreferencesOS(cell.goos, cell.step, cell.opacity, false)
				pane := paneAlpha(i, spec, tk)
				ui.Column(c).Width(veilCellW).Height(veilCellH).Radius(8).
					Background(tk.Theme.Background.Alpha(spec.Composite())).Padding(8).Children(func() {
					ui.Column(c).FillWidth().Height(72).Radius(6).Background(pane).Padding(8).Children(func() {
						ui.Box(c).FillWidth().Height(28).Radius(4).Background(tk.Control)
					})
				})
			}
		})
	}, len(cells)*(veilCellW+veilGap)+veilGap, veilCellH+2*veilGap)
	tt.SetDark(dark)
	tt.Frame()
	return tt.Image()
}

// veilAt reads one pixel as an opaque colour.
func veilAt(img *image.RGBA, x, y int) ui.Color {
	o := y*img.Stride + x*4
	return ui.RGBA(img.Pix[o], img.Pix[o+1], img.Pix[o+2], 255)
}

// veilStep is the level distance between two pixels on the channel that moved
// most: the fills are near-neutral, so the three channels move together and the
// largest is the visible step.
func veilStep(a, b ui.Color) int {
	best := 0
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		d := int(p[0]) - int(p[1])
		if d < 0 {
			d = -d
		}
		if d > best {
			best = d
		}
	}
	return best
}

// veilOldContentAlpha is the P13-gc baseline: `contentStep` added in **fill**
// space, `overFrame(TintAlpha, TintAlpha+0.09)`. It survives here only so the
// dump can paint the old pane beside the new one.
func veilOldContentAlpha(spec glassmap.Spec) float32 {
	if !spec.Translucent {
		return 1
	}
	target := spec.TintAlpha + 0.09
	if target > 1 {
		target = 1
	}
	if target <= spec.TintAlpha {
		return 0
	}
	if spec.TintAlpha >= 1 {
		return 1
	}
	return (target - spec.TintAlpha) / (1 - spec.TintAlpha)
}

// veilPaneNew is the shipped pane: `Tokens.Sunken` itself.
func veilPaneNew(i int, spec glassmap.Spec, tk Tokens) ui.Color { return tk.Sunken }

// veilPaneOld paints the same pane at the **fill-space** alpha P13-gc replaced.
// `ui.Color.Alpha` multiplies the fill's own alpha, so the byte is set outright.
func veilPaneOld(i int, spec glassmap.Spec, tk Tokens) ui.Color {
	a := veilOldContentAlpha(spec)
	return ui.RGBA(tk.Sunken.R, tk.Sunken.G, tk.Sunken.B, a)
}

// TestTheContentPaneStillReadsAsAFaceUnderTheMaterial is the 8-level check:
// the rendered pane is a face over the frame it sits on, and the rendered
// control is a face over that pane, on every platform/stop/slider cell and both
// appearances. The darwin `liquid` cells are the ones the old formula failed
// (6.6 / 6.8 levels).
func TestTheContentPaneStillReadsAsAFaceUnderTheMaterial(t *testing.T) {
	cells := veilCells()
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			img := veilRender(t, dark, cells, veilPaneNew)
			for i, cell := range cells {
				x := veilGap + i*(veilCellW+veilGap)
				y := veilGap
				// 三块各自的一条干净的列：框的下沿（面板里没有别的东西）、面的顶部
				// （控制块之上）、控制块的中间。
				frame := veilAt(img, x+veilCellW/2, y+veilCellH-4)
				pane := veilAt(img, x+veilCellW/2, y+8+4)
				control := veilAt(img, x+veilCellW/2, y+8+8+14)
				step := veilStep(pane, frame)
				t.Logf("%s %-22s frame=%v pane=%v (%d levels) control=%v (%d levels)",
					appearance, cell.label, frame, pane, step, control, veilStep(control, pane))
				if step < veilFaceFloor {
					t.Errorf("[%s] %s: the content pane %v sits %d levels above the frame %v, under the face floor %d "+
						"(the recess is a visible-alpha step — liquid-glass.md §2.3)",
						appearance, cell.label, pane, step, frame, veilFaceFloor)
				}
				if c := veilStep(control, pane); c < veilFaceFloor {
					t.Errorf("[%s] %s: the control %v sits %d levels above the pane %v, under the face floor %d",
						appearance, cell.label, control, c, pane, veilFaceFloor)
				}
			}
		})
	}
}

// The old formula's pane beside the new one, on the two cells where it mattered:
// the same frame, the same desk, only the alpha's space differs.
func TestDumpP13gcVeilPane(t *testing.T) {
	if os.Getenv("FLOTER_P13GC_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13GC_DUMP=1 to write /tmp/floter-P13-gc-veilpane-*.png")
	}
	base := []veilCell{
		{"linux", "liquid", 47, "linux liquid 47%"},
		{"darwin", "liquid", 47, "darwin liquid 47% (P13-gc)"},
		{"darwin", "liquid", 10, "darwin liquid 10% (P13-gc)"},
	}
	old := []veilCell{
		{"linux", "liquid", 47, "linux liquid 47%"},
		{"darwin", "liquid", 47, "darwin liquid 47% (old fill)"},
		{"darwin", "liquid", 10, "darwin liquid 10% (old fill)"},
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		// 左三格出厂算式（`Tokens.Sunken`），右三格旧填充空间那一笔。
		pane := func(i int, spec glassmap.Spec, tk Tokens) ui.Color {
			if i < len(base) {
				return veilPaneNew(i, spec, tk)
			}
			return veilPaneOld(i, spec, tk)
		}
		img := veilRender(t, dark, append(append([]veilCell{}, base...), old...), pane)
		// 一排六格：左三格是出厂算式，右三格是旧算式（同一张框、同一块地面）。
		y := veilGap + 8 + 4
		read := func(i int, label string) ui.Color {
			x := veilGap + i*(veilCellW+veilGap) + veilCellW/2
			c := veilAt(img, x, y)
			frame := veilAt(img, x, veilGap+veilCellH-4)
			t.Logf("%s %-32s pane=%v frame=%v (%d levels)", appearance, label, c, frame, veilStep(c, frame))
			return c
		}
		for i := range base {
			read(i, base[i].label)
		}
		for i := range old {
			read(i+len(base), old[i].label)
		}
		path := "/tmp/floter-P13-gc-veilpane-" + appearance + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}

// The settings body does not paint `Sunken` as a fill: it composites it opaque
// over the window's ground first (`settingsui.go:1303`, `FaceOver(地, Sunken)`).
// This pins what this round does to that byte: the pane token is now the same on
// every platform, so the opaque body on darwin lands on the **linux** byte
// (浅 247 → 246、深 14 → 15，各 1 级) instead of one level off it.
func TestTheOpaqueBodyLandsOnTheMaterialFreeByte(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		wantR := uint8(246)
		if dark {
			appearance = "dark"
			wantR = 15
		}
		s := settings.Default()
		s.GlassStep = "liquid"
		s.MainOpacity = 47
		darwin := ForPlatform(s, dark, ui.Preferences{}, "darwin")
		linux := ForPlatform(s, dark, ui.Preferences{}, "linux")
		body := FaceOver(darwin.Theme.Background, darwin.Sunken)
		ref := FaceOver(linux.Theme.Background, linux.Sunken)
		t.Logf("%s body=%v material-free=%v", appearance, body, ref)
		if body != ref {
			t.Errorf("[%s] the opaque body = %v, the material-free one = %v: the pane token is one rung, the fill it leaves behind is the same sheet", appearance, body, ref)
		}
		if body.R != wantR || body.G != wantR {
			t.Errorf("[%s] the opaque body's R/G = %d/%d, want %d/%d (the P13-gc literal)", appearance, body.R, body.G, wantR, wantR)
		}
	}
}
