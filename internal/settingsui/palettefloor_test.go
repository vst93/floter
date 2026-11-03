package settingsui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-fo（`docs/liquid-glass.md` §6.134）：同一张 Terminal appearance 卡上有两枚
// picker——字体族走 `chooseSelect`，调色板走 `paletteRow` 自己拼的一行。§6.133 把
// 前者从 `Width(220)` 改成 `MinWidth(220)`（220 是地板，值更长就借标签列的地），
// 而后者还留着字面 `Width(220)`：同一个盒子的宽度，一半来自「内容需要多少」，
// 一半来自一个常数。
//
// 出厂内容看不出差别（九个调色板名最宽 80px，都装得下），所以这条 pin 钉的是
// **规则**：两枚 picker 的盒子都由内容决定，出厂的每一个调色板名画出同一个盒子
// （右列是一条线），长标签也不被省略号切。
func TestThePalettePickerIsAFloorNotACeiling(t *testing.T) {
	en := i18n.For("en").Settings

	// A. 右列是一条线：九个出厂调色板名，逐台阶、两套外观，盒子相同。
	for _, dark := range []bool{false, true} {
		for _, step := range []string{"tiny", "small", "default", "large"} {
			_, base := palettePickerFrame(t, step, "inherit", dark)
			for _, option := range en.Palettes {
				_, box := palettePickerFrame(t, step, option.ID, dark)
				if box != base {
					t.Errorf("dark=%v step=%s: %q paints a %dpx box where %q paints %dpx: "+
						"the palette names are not one column", dark, step, option.Label, box, "Follow app", base)
				}
			}
		}
	}

	// B. 地板不是上限：一个比盒子长的标签让盒子长出来，而不是被 `SingleLine` 切掉。
	// 标签是设置文件/插件写进来的任意字符串（`terminal_theme` 只认 id，但这枚
	// picker 的标签表由 i18n 提供，长度不由本仓决定）。
	const long = "High contrast, colour-blind safe and dimmed for long sessions"
	for _, dark := range []bool{false, true} {
		for _, step := range []string{"small", "default", "large"} {
			_, short := palettePickerFrame(t, step, "inherit", dark)
			tt, box := palettePickerFrameWithLabels(t, step, dark, long)
			ink := inkWidth(tt, mustFind(t, tt, long))
			natural := naturalInk(t, step, long)
			if ink != int(natural) && ink+1 != int(natural) {
				t.Errorf("dark=%v step=%s: the long palette label paints %d of its %vpx natural ink: "+
					"it is being cut, not the box growing", dark, step, ink, natural)
			}
			if box <= short {
				t.Errorf("dark=%v step=%s: the box stayed %dpx (floor %d) for a %vpx label: "+
					"220 is being used as a ceiling", dark, step, box, short, natural)
			}
		}
	}
}

// palettePickerFrame draws the General page with one palette chosen and reads the
// palette picker's painted box off the frame. The label it reads back is the one
// that palette's own id maps to, so a chosen palette is found by its own name.
func palettePickerFrame(t *testing.T, step, palette string, dark bool) (*ui.Tester, int) {
	t.Helper()
	en := i18n.For("en").Settings
	label := ""
	for _, option := range en.Palettes {
		if option.ID == palette {
			label = option.Label
		}
	}
	if label == "" {
		t.Fatalf("no palette named %q", palette)
	}
	return palettePickerFrameLabels(t, step, palette, dark, label)
}

// palettePickerFrameWithLabels draws the row with a copy whose palette table is
// the caller's: the picker's labels come from `i18n.Settings.Palettes`, which is
// a parameter of `paletteRow`, so a long label can be drawn without touching the
// i18n tables (which are a red line).
func palettePickerFrameWithLabels(t *testing.T, step string, dark bool, label string) (*ui.Tester, int) {
	t.Helper()
	en := i18n.For("en").Settings
	shipped := i18n.For("en").Settings
	shipped.Palettes = []i18n.Option{{ID: "contrast", Label: label}}
	s := settings.Default()
	s.UIScale = step
	a := New(settings.NewStore(s), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	_ = en
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = a.tokens(c)
		c.SetTheme(tokens.Theme)
		ui.Column(c).Padding(20).Width(launcher.InputWindowWidth).Children(func() {
			a.paletteRow(c, shipped, "contrast", func(string) {})
		})
	}, launcher.InputWindowWidth, 240)
	tt.SetDark(dark)
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	value := mustFind(t, tt, label)
	_, _, box := pillBoxAt(tt.Image(), value, launcher.InputWindowWidth-20)
	return tt, box
}

func palettePickerFrameLabels(t *testing.T, step, palette string, dark bool, label string) (*ui.Tester, int) {
	t.Helper()
	s := settings.Default()
	s.UIScale = step
	if palette != "" {
		s.TerminalTheme = palette
	}
	a := New(settings.NewStore(s), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	a.Page = PageGeneral
	tt, _ := renderIn(t, a, launcher.InputWindowWidth, 1600, dark)
	value := mustFind(t, tt, label)
	_, _, box := pillBoxAt(tt.Image(), value, 691)
	return tt, box
}

// pillBoxAt is the pill's painted width in the value's own band: the band's most
// common colour is the face, and every column carrying it at any row belongs to
// the box.
func pillBoxAt(img *image.RGBA, value ui.Rect, right int) (int, int, int) {
	y0, y1 := int(value.Y)-2, int(value.Y+value.H)+2
	x0, x1 := int(value.X)-40, right
	if x0 < 0 {
		x0 = 0
	}
	if x1 > img.Bounds().Dx() {
		x1 = img.Bounds().Dx()
	}
	counts := map[color.RGBA]int{}
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			counts[img.RGBAAt(x, y)]++
		}
	}
	face := color.RGBA{}
	best := 0
	for c, n := range counts {
		if n > best {
			face, best = c, n
		}
	}
	lo, hi := x1, x0
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			if img.RGBAAt(x, y) == face {
				if x < lo {
					lo = x
				}
				if x > hi {
					hi = x
				}
				break
			}
		}
	}
	return lo, hi, hi - lo + 1
}

// TestDumpP13FOPaletteFloor is the round's visual evidence: the palette picker
// holding a label longer than its floor, at the shipped step and the largest one,
// both appearances. Opt-in so a normal run does not litter /tmp.
func TestDumpP13FOPaletteFloor(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	const long = "High contrast, colour-blind safe and dimmed for long sessions"
	for _, arm := range []struct {
		step, name string
		dark       bool
	}{
		{"small", "light", false},
		{"small", "dark", true},
		{"large", "light", false},
		{"large", "dark", true},
	} {
		tt, _ := palettePickerFrameWithLabels(t, arm.step, arm.dark, long)
		writePNG(t, filepath.Join("/tmp", "floter-P13-fo-palette-"+arm.step+"-"+arm.name+"-after.png"), tt)
	}
}
