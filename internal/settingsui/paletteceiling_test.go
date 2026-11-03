package settingsui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-fp（`docs/liquid-glass.md` §6.135）：P13-fo 把调色板 picker 从 `Width(220)`
// 改成 `MinWidth(220)`——「220 是地板不是上限」，长值不再被省略号切。那句话只说了
// 一半：这一行有**两个孩子**，picker 右边是那块缩微终端（`palettePreview`，固定
// `Space(37)`、`Shrink(0)`）。地板没有天花板，于是值一长，picker 就一路把预览块
// 顶出行外。
//
// 先量（真画法，行的内容盒 515 = 卡内宽；u=3.2/3.6/4.0/4.4 四台阶）：

//	台阶      值长   值墨         预览块 x    块右沿   行右沿   溢出
//	default   8     49…94        333.0      481.0    535.0    −54.0
//	default   35    49…250       333.0      481.0    535.0    −54.0
//	default   55    49…358       392.0      540.0    535.0     +5.0
//	default   75    49…479       511.0      659.0    535.0   +124.0
//	large     55    51…390       427.8      590.6    535.0    +55.6
//	large     75    51…522       558.8      721.6    535.0   +186.6
//
// 也就是说：出厂九个调色板名（最宽 `High contrast` 80px）永远碰不到这条线，但
// `paletteRow` 的标签表是**参数**（`i18n.Settings.Palettes`，调用方可以传任意
// `Option.Label`），一旦长过约 50 字，被推出去的是那块**预览**——它才是这一行
// 存在的理由（R43：让用户看见这个调色板长什么样）。
//
// 落地：picker 补 `Shrink(1)`。它先借标签列的地（§6.133 的地板不动），借完了就在
// 自己的盒子边收口（框架的省略号）；预览块 `Shrink(0)`，一个像素不挪。

// palettePreviewRect draws the palette row with the caller's label table and
// returns the preview block's rect.
func palettePreviewRect(t *testing.T, step, label string) ui.Rect {
	t.Helper()
	en := i18n.For("en").Settings
	copy := en
	copy.Palettes = []i18n.Option{{ID: "contrast", Label: label}}
	s := settings.Default()
	s.UIScale = step
	a := New(settings.NewStore(s), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		ui.Column(c).Padding(20).Width(rowContentWidth).Children(func() {
			a.paletteRow(c, copy, "contrast", func(string) {})
		})
	}, launcher.InputWindowWidth, 320)
	tt.SetDark(false)
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	block, ok := tt.Find(en.PalettePreview)
	if !ok {
		t.Fatalf("step=%s label=%d chars: the preview block is missing", step, len(label))
	}
	return block
}

// rowContentWidth is the settings card's row content box at the one width the
// surface is drawn at (720): the card's 703.8 border minus the row's own inset.
// Measured in `controlslot_test.go` (`contentRightAt720` − left inset).
const rowContentWidth float32 = 515

// TestThePickerFloorHasACeiling is the round's pin, in one sentence: **the
// preview block never moves.** A long palette value may take the label column's
// room (the floor), and past that it is cut inside its own box (the ceiling);
// the block beside it stays where it is at every step and every length.
func TestThePickerFloorHasACeiling(t *testing.T) {
	shipped := "High contrast"
	for _, step := range []string{"tiny", "small", "default", "large"} {
		base := palettePreviewRect(t, step, shipped)
		for _, n := range []int{30, 55, 75, 120} {
			label := strings.Repeat("wide value ", 20)[:n]
			got := palettePreviewRect(t, step, label)
			if got != base {
				t.Errorf("step=%s: a %d-char value moves the preview block %v → %v (%+.1fpx): "+
					"the picker's floor has no ceiling, so it pushes the block out of the row",
					step, n, base, got, got.X-base.X)
			}
		}
	}
}

// TestTheLongValueYieldsInsideItsOwnBox is the other half: when the value no
// longer fits, the picker's ink stops at the block's left edge — the value is
// cut by its own box (the framework's ellipsis), never painted over the preview.
func TestTheLongValueYieldsInsideItsOwnBox(t *testing.T) {
	for _, step := range []string{"small", "default", "large"} {
		label := strings.Repeat("wide value ", 20)[:120]
		tt, block := paletteRowFrame(t, step, label)
		value, ok := tt.Find(label)
		if !ok {
			t.Fatalf("step=%s: the value is missing", step)
		}
		_, inkRight := inkColumns(tt, value)
		if float32(inkRight) >= block.X {
			t.Errorf("step=%s: the value's ink ends at %d, at or past the preview block's left edge %.1f: "+
				"the value is painted over the preview instead of yielding inside its own box",
				step, inkRight, block.X)
		}
	}
}

// TestThePreviewBlockStillHoldsItsThreeLines answers P13-fo's open question
// (「先量它会不会在更小的台阶上切字」): the block's width is a literal
// `Space(37)`, and its three lines are fixed ink. Measured across the four
// shipped steps, the longest line (`floter --version`) ends 25.6…32.2px before
// the block's inner edge — so the block never clips its own content and needs no
// floor of its own.
func TestThePreviewBlockStillHoldsItsThreeLines(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		for _, step := range []string{"tiny", "small", "default", "large"} {
			s := settings.Default()
			s.UIScale = step
			a := New(settings.NewStore(s), Actions{
				SetIntegrationEnabled: func(string, bool, func(error)) {},
				SetCommandEnabled:     func(string, string, bool) {},
			})
			var tokens theme.Tokens
			tt := ui.NewTester(func(c *ui.Context) {
				tokens = a.tokens(c)
				c.SetTheme(tokens.Theme)
				ui.Column(c).Padding(20).Width(rowContentWidth).Children(func() {
					a.paletteRow(c, en, "inherit", func(string) {})
				})
			}, launcher.InputWindowWidth, 320)
			tt.SetDark(dark)
			tt.Advance(500 * time.Millisecond)
			tt.Frame()
			block, ok := tt.Find(en.PalettePreview)
			if !ok {
				t.Fatalf("%s/dark=%v: the preview block is missing", step, dark)
			}
			cmd, ok := tt.Find("floter --version")
			if !ok {
				t.Fatalf("%s/dark=%v: the preview's command line is missing", step, dark)
			}
			innerRight := block.X + block.W - tokens.Theme.Spacing*4 - 1
			if slack := innerRight - (cmd.X + cmd.W); slack < 8 {
				t.Errorf("%s/dark=%v: the command line ends %.1fpx before the block's inner edge "+
					"(block %.1f wide, inner padding %.1f): the block is clipping its own content",
					step, dark, slack, block.W, tokens.Theme.Spacing*4)
			}
		}
	}
}

// paletteRowFrame draws the row with one label and returns the frame and the
// preview block, for the ink reads.
func paletteRowFrame(t *testing.T, step, label string) (*ui.Tester, ui.Rect) {
	return paletteRowFrameIn(t, step, label, false)
}

// paletteRowFrameIn is `paletteRowFrame` in a chosen appearance.
func paletteRowFrameIn(t *testing.T, step, label string, dark bool) (*ui.Tester, ui.Rect) {
	t.Helper()
	en := i18n.For("en").Settings
	copy := en
	copy.Palettes = []i18n.Option{{ID: "contrast", Label: label}}
	s := settings.Default()
	s.UIScale = step
	a := New(settings.NewStore(s), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		ui.Column(c).Padding(20).Width(rowContentWidth).Children(func() {
			a.paletteRow(c, copy, "contrast", func(string) {})
		})
	}, launcher.InputWindowWidth, 320)
	tt.SetDark(dark)
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	block, ok := tt.Find(en.PalettePreview)
	if !ok {
		t.Fatalf("step=%s/dark=%v: the preview block is missing", step, dark)
	}
	return tt, block
}

// inkColumns is a rect's painted ink from its leftmost to its rightmost column.
func inkColumns(tt *ui.Tester, r ui.Rect) (int, int) {
	lo, ok := leftmostInk(tt, r)
	if !ok {
		return 0, 0
	}
	hi, _ := rightmostInk(tt, r)
	return lo, hi
}

// TestDumpP13FPPaletteCeiling is the round's visual evidence: the row holding a
// value long enough to reach the ceiling, at the shipped step and the largest
// one, both appearances. Opt-in so a normal run does not litter /tmp.
//
// The `before` arm is produced by running the same dump against the row as it
// stood before this round (`MinWidth(220)` without `Shrink(1)`) and naming the
// files with `FLOTER_P13FP_ARM=before` — the same code path, not a renamed copy
// of the after image.
func TestDumpP13FPPaletteCeiling(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	arm := os.Getenv("FLOTER_P13FP_ARM")
	if arm == "" {
		arm = "after"
	}
	const long = "High contrast, colour-blind safe and dimmed for long sessions"
	for _, a := range []struct {
		step, name string
		dark       bool
	}{
		{"small", "light", false},
		{"small", "dark", true},
		{"large", "light", false},
		{"large", "dark", true},
	} {
		tt, _ := paletteRowFrameIn(t, a.step, long, a.dark)
		writePNG(t, filepath.Join("/tmp",
			"floter-P13-fp-ceiling-"+a.step+"-"+a.name+"-"+arm+".png"), tt)
	}
}
