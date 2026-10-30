package settingsui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-p · 材质 census，不是函数名 census。
//
// 前三轮（P13-i / P13-l / P13-m）把设置页的控件面一块块退成 E1 的一对面，但三轮
// 用的都是 grep：先是数 `tokens.Lens(`，后来是数 `Border(1,` 与 `tokens.Control)`
// 在不在**同一行**。于是这一族还剩三块面没戴那一对线，而它们对两种 grep 都不可见：
//
//	A  `rows.go` `sessionIconButton` 的悬停臂：它从来没有 `Lens` 可数——可 grep 的是
//	   「`Lens` 调用」，不是材质。
//	B  `savealert.go` `shortcutRejectedAlert`：`Background(tokens.Control)` 与
//	   `Border(1, tokens.ControlEdge)` 分两行写。
//	C  `pluginsheet.go` `sheetActionButton`：面是变量 `plane`，不是 `tokens.Control`。
//
// 三处都只差两条 1px 内线，几何一个字节不动。量法与 P13-n 的 `counterPane` 同一套：
// 取**真页面的地面**（`realPageGround`）当底，画**真的控件**（不是照着画的替身），
// 读顶内线 / 面 / 底内线三个像素。
//
// 为什么浅色档要求「顶线严格更亮」而深色档只要求「不更暗」：浅色档的外圈是
// `ControlEdge`（黑 0.06），受光边是白 0.09，两者在白面上方向相反——一条白线
// 换回 `ControlEdge` 会立刻变成暗线，所以浅色档量得出「换错线」。深色档的外圈是
// 白 0.07、受光边是白 0.09，两条白线在 1x 屏上分不开（P13-n 的结论），那里能读出来
// 的是**对侧**：`LensEdge` 0.20 把面压下去 8~13 级。

// litSum is the brightness a pixel reads as: the three channels added, the same
// sum the other lit-edge pins use (a hue's white line lifts two channels and
// lowers the third, so the sum is what says "lighter").
func litSum(c ui.Color) int { return int(c.R) + int(c.G) + int(c.B) }

// readLitPair reads the three pixels of an E1 face off a rendered frame: the
// brightest of the first two inner rows (the lit edge), the darkest of the last
// two (the counter-side), and the face between them. Two rows, not one: a 28u
// control is laid out on a fractional y, so its 1px inset line straddles two
// device rows.
func readLitPair(img *image.RGBA, box ui.Rect) (crown, shade, face ui.Color) {
	// The middle column, four rows in from the top: the label's ink sits on the
	// box's centre **row**, so the face is read from the padding band above it
	// (4 rows clears the 1px frame and stays above every glyph), while the two
	// inner lines are read where the pair actually paints them.
	x := int(box.X + box.W/2)
	crown = imagePixel(img, x, int(box.Y)+1)
	for dy := 1; dy <= 2; dy++ {
		if q := imagePixel(img, x, int(box.Y)+dy); litSum(q) > litSum(crown) {
			crown = q
		}
	}
	face = imagePixel(img, x, int(box.Y)+4)
	shade = imagePixel(img, x, int(box.Y+box.H)-2)
	for dy := -3; dy <= -2; dy++ {
		if q := imagePixel(img, x, int(box.Y+box.H)+dy); litSum(q) < litSum(shade) {
			shade = q
		}
	}
	return crown, shade, face
}

// assertLitPair is the one rule all three pins share: the top inner row is a
// lit line (strictly lighter on the light appearance, never darker on the dark
// one) and the bottom inner row is a shade (clearly darker than the face).
func assertLitPair(t *testing.T, what string, dark bool, box ui.Rect, crown, shade, face ui.Color) {
	t.Helper()
	if box.W < 20 || box.H < 20 {
		t.Fatalf("%s: measured a %v×%v box, not a control", what, box.W, box.H)
	}
	if dark {
		if litSum(crown) < litSum(face) {
			t.Errorf("%s (dark): the top inner row is a shade, not a lit edge: %v vs face %v",
				what, crown, face)
		}
	} else if litSum(crown) <= litSum(face) {
		t.Errorf("%s (light): the top inner row is not lighter than the face: %v vs face %v (want the 0.09 rim)",
			what, crown, face)
	}
	if drop := litSum(face) - litSum(shade); drop < 9 {
		t.Errorf("%s (%s): the bottom inner row is only %d levels under the face (%v vs %v); want the counter-side shade",
			what, appearanceName(dark), drop, shade, face)
	}
}

// boundSquare is the probe's handle on a void draw function: the button is a
// 28u square, so a 28u wrapper has exactly the button's box (the button is its
// only child and fills it).
func boundSquare(c *ui.Context, fn func()) ui.Element {
	t := c.Theme()
	row := ui.Row(c).Width(t.Space(7)).Height(t.Space(7)).Shrink(0)
	row.Children(fn)
	return row
}

func appearanceName(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// litFrame renders one real widget on the settings page's own ground and
// returns the frame and the widget's box. The widget is built by the product's
// own draw function, so a pin cannot stay green while the product draws
// something else. `hover` moves the pointer onto the box before the second
// frame, for controls whose pair only exists under the pointer.
func litFrame(t *testing.T, dark, hover bool, build func(a *App, c *ui.Context) ui.Element) (*ui.Tester, ui.Rect) {
	t.Helper()
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(24).Width(320).Children(func() {
			build(a, c).Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 400, 200)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if hover {
		tt.Move(box.X+box.W/2, box.Y+box.H/2)
		tt.Frame()
	}
	return tt, box
}

// litProbe is litFrame plus the three pixels the pair is read from.
func litProbe(t *testing.T, dark, hover bool, build func(a *App, c *ui.Context) ui.Element) (ui.Rect, ui.Color, ui.Color, ui.Color) {
	t.Helper()
	tt, box := litFrame(t, dark, hover, build)
	crown, shade, face := readLitPair(tt.Image(), box)
	return box, crown, shade, face
}

// A. The sessions page's 28u icon button, hovered. Its three siblings (the 30u
// extension icon button, the 28u param editor icon button, the header's 28u
// buttons) all wear the pair since P13-i; this one was invisible to that sweep
// because it never had a `Lens` call to find.
func TestTheSessionIconButtonWearsTheLitPair(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		box, crown, shade, face := litProbe(t, dark, true, func(a *App, c *ui.Context) ui.Element {
			return boundSquare(c, func() {
				a.sessionIconButton(c, "play", en.SessionsResume, false, true, false, func() {})
			})
		})
		assertLitPair(t, "the session icon button", dark, box, crown, shade, face)
	}
}

// B. The Shortcuts page's refusal band. It is the same shape as `noticeBand`'s
// non-danger arm (§6.2b 9-14) — a `Control` pane under a `ControlEdge` frame on
// the page's ground — and that one has worn the pair since P13-l.
func TestTheShortcutRefusalBandWearsTheLitPair(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		box, crown, shade, face := litProbe(t, dark, false, func(a *App, c *ui.Context) ui.Element {
			a.rejectedShortcut = settings.ShortcutToggleWindow
			// The band is drawn into the page column, not returned, so the probe
			// binds the column it fills: a FillWidth child's box is its parent's.
			var wrap ui.Handle
			col := ui.Column(c).FillWidth().Bind(&wrap)
			col.Children(func() { a.shortcutRejectedAlert(c, en) })
			return col
		})
		assertLitPair(t, "the shortcut refusal band", dark, box, crown, shade, face)
	}
}

// C. The plugin sheet's 28u action pill (neutral arm). Everything inside a
// float is E1 (§2 判据 1), and the pill's three siblings (the extension action
// pill 9-1, the notice band's `barButton` 9-5, the uninstall dialog's capsule
// 9-6) all wear the pair.
func TestThePluginSheetPillWearsTheLitPair(t *testing.T) {
	for _, dark := range []bool{false, true} {
		box, crown, shade, face := litProbe(t, dark, false, func(a *App, c *ui.Context) ui.Element {
			return a.sheetActionButton(c, "Clear history", false, func() {})
		})
		assertLitPair(t, "the plugin sheet's action pill", dark, box, crown, shade, face)
	}
}

// The danger arm is the reverse pin: its face is transparent at rest, so a lit
// edge there would be a white line on nothing — §1 禁令 3's 「类玻璃边」. It
// keeps its one warning hairline and nothing else.
//
// The pin reads the **counter-side** half, not the crown: on a transparent pill
// the top inner row is the warning hairline itself (the border straddles two
// device rows at a fractional y), so the only row that can prove a face is
// missing is the bottom one — `LitFace` would paint a 0.20/0.06 shade there.
func TestThePluginSheetDangerPillKeepsItsOwnLine(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, box := litFrame(t, dark, false, func(a *App, c *ui.Context) ui.Element {
			return a.sheetActionButton(c, "Delete", true, func() {})
		})
		img := tt.Image()
		// The rows the pair would occupy, read down the pill's middle column (the
		// label sits on the centre row, not on these two), against the ground
		// outside the pill: a lit edge is +0.09 of white, a counter-side shade is
		// 0.06/0.20 of black, so either one moves a channel by more than 2.
		ground := imagePixel(img, 2, 2)
		x := int(box.X + box.W/2)
		top := imagePixel(img, x, int(box.Y)+1)
		bottom := imagePixel(img, x, int(box.Y+box.H)-2)
		if diff := channelDiff(top, ground); diff > 2 {
			t.Errorf("the danger pill (%s) grew a lit edge: top row %v over ground %v (%d levels)",
				appearanceName(dark), top, ground, diff)
		}
		if diff := channelDiff(bottom, ground); diff > 2 {
			t.Errorf("the danger pill (%s) grew a counter-side shade: bottom row %v over ground %v (%d levels)",
				appearanceName(dark), bottom, ground, diff)
		}
		if box.W < 20 {
			t.Errorf("the danger pill measured %v wide", box.W)
		}
	}
}

// channelDiff is the largest per-channel difference between two pixels.
func channelDiff(a, b ui.Color) int {
	d := absInt(int(a.R) - int(b.R))
	if v := absInt(int(a.G) - int(b.G)); v > d {
		d = v
	}
	if v := absInt(int(a.B) - int(b.B)); v > d {
		d = v
	}
	return d
}

// The opt-in dump: the three surfaces on both appearances, before and after
// (set FLOTER_PNG_TAG=before while the fix is reverted, as the mutation plan
// does), so the report can show the same pixels the pins read.
func TestDumpP13PLitPairCensus(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	en := i18n.For("en").Settings
	surfaces := []struct {
		name  string
		build func(a *App, c *ui.Context) ui.Element
		hover bool
	}{
		{"sessionbutton", func(a *App, c *ui.Context) ui.Element {
			return boundSquare(c, func() {
				a.sessionIconButton(c, "play", en.SessionsResume, false, true, false, func() {})
			})
		}, true},
		{"refusalband", func(a *App, c *ui.Context) ui.Element {
			a.rejectedShortcut = settings.ShortcutToggleWindow
			var wrap ui.Handle
			col := ui.Column(c).FillWidth().Bind(&wrap)
			col.Children(func() { a.shortcutRejectedAlert(c, en) })
			return col
		}, false},
		{"sheetpill", func(a *App, c *ui.Context) ui.Element {
			return a.sheetActionButton(c, "Clear history", false, func() {})
		}, false},
	}
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		for _, surface := range surfaces {
			var box ui.Rect
			var ref ui.Handle
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(24).Width(320).Children(func() {
					surface.build(a, c).Bind(&ref)
					box = c.Resolve(ref).Bounds()
				})
			}, 400, 160)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			if surface.hover {
				tt.Move(box.X+box.W/2, box.Y+box.H/2)
				tt.Frame()
			}
			crown, shade, face := readLitPair(tt.Image(), box)
			t.Logf("%s %s %s: crown %v shade %v face %v", surface.name, appearanceName(dark), tag, crown, shade, face)
			name := filepath.Join(os.TempDir(), "floter-P13-p-"+surface.name+"-"+appearanceName(dark)+"-"+tag+".png")
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
