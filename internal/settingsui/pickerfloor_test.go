package settingsui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/terminalui"
	"floter/internal/theme"
)

// P13-fn（`docs/liquid-glass.md` §6.133）：设置页的 picker 盒子此前是一个**字面
// 220px**（`Width(220)`），而它里面的东西——内边距、字形、箭头——全部跟着界面
// 台阶走。于是同一个盒子里的空气在四个台阶上是 50% / 43% / 36% / 30%，而一个比
// 盒子长的值（字体族名由设置文件/插件写进来，可以是任意字符串）会被 `SingleLine`
// 用省略号切掉：实测 39 字的族名在出厂台阶上从第 189 列起被改画。
//
// 现在 220 是**地板**不是上限：值比它长，盒子就借标签列的地（标签列是弹性的，
// 且留着 ≥2× 自己的墨）。这三条 pin 钉的就是这句话。
func TestALongPickerValueGrowsTheBoxInsteadOfBeingCut(t *testing.T) {
	en := i18n.For("en").Settings
	const long = "Iosevka Term Extended Nerd Font Mono"
	steps := []string{"tiny", "small", "default", "large"}

	// A. 地板：出厂的每一个族名都画出同一个盒子——右列是一列，不是一条锯齿。
	for _, dark := range []bool{false, true} {
		_, _, _, shortBox := pickerFrame(t, "small", "Menlo", dark)
		for _, family := range terminalui.FallbackFontFamilies {
			_, _, _, box := pickerFrame(t, "small", family, dark)
			if box != shortBox {
				t.Errorf("dark=%v: %q paints a %dpx box where %q paints %dpx: the floor is not governing "+
					"the shipped names", dark, family, box, "Menlo", shortBox)
			}
		}
	}

	// B. 成长：值比盒子长多少，盒子就长多少；C. 让位的是标签列，但标签不被切。
	for _, dark := range []bool{false, true} {
		for _, step := range steps {
			_, _, tokens, shortBox := pickerFrame(t, step, "Menlo", dark)
			ttLong, label, _, box := pickerFrame(t, step, long, dark)
			labelInk := inkWidth(ttLong, label)
			// 盒子里的家具：两侧内边距 + 文字与箭头之间的空隙 + 箭头自己。
			u := tokens.Theme.Spacing
			furniture := 2*u*3.5 + u*1.5 + u*2.5
			room := float32(shortBox) - furniture
			overflow := naturalInk(t, step, long) - room
			if overflow < 0 {
				overflow = 0
			}
			if growth := float32(box - shortBox); absFloat(growth-overflow) > 6 {
				t.Errorf("dark=%v step=%s: the box grew %vpx for a value %vpx longer than its room "+
					"(box %d→%d, room %.1f, value %.1f): the value is being cut instead of the box growing",
					dark, step, growth, overflow, shortBox, box, room, naturalInk(t, step, long))
			}
			if labelInk <= 0 {
				t.Errorf("dark=%v step=%s: %q paints nothing", dark, step, en.FontFamily)
			}
			if got, want := labelInk, labelInkAt(t, step, "Menlo", dark); got != want {
				t.Errorf("dark=%v step=%s: %q paints %d columns with a long value but %d with a short one: "+
					"the label is the thing that gets cut", dark, step, en.FontFamily, got, want)
			}
			if label.W < 1.5*float32(labelInk) {
				t.Errorf("dark=%v step=%s: the label column is down to %.1fu for %d columns of ink: "+
					"a long value is eating the label", dark, step, label.W, labelInk)
			}
		}
	}
}

// naturalInk is a string's own painted width in the app's theme, drawn on a
// plane wide enough that nothing can cut it: the value's measure, to compare
// against the room the picker's box gives it.
func naturalInk(t *testing.T, step, text string) float32 {
	t.Helper()
	s := settings.Default()
	s.UIScale = step
	a := New(settings.NewStore(s), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	tt, _ := renderOn(t, 700, 60, false, a, func(c *ui.Context) {
		ui.Row(c).Padding(8, 8).Children(func() { ui.Text(c, text) })
	})
	r := ui.Rect{X: 0, Y: 0, W: float32(tt.Image().Bounds().Dx()), H: float32(tt.Image().Bounds().Dy())}
	return float32(inkWidth(tt, r))
}

// labelInkAt is the label column's own painted width on the General page.
func labelInkAt(t *testing.T, step, family string, dark bool) int {
	t.Helper()
	tt, label, _, _ := pickerFrame(t, step, family, dark)
	return inkWidth(tt, label)
}

// inkWidth is the painted width of one box.
func inkWidth(tt *ui.Tester, r ui.Rect) int {
	lo, ok := leftmostInk(tt, r)
	if !ok {
		return 0
	}
	hi, _ := rightmostInk(tt, r)
	return hi - lo
}

// pickerFrame draws the General page at one interface step with one font family
// chosen, and reads the picker's painted box off the frame: the columns of the
// row that carry the pill's own face colour.
func pickerFrame(t *testing.T, step, family string, dark bool) (*ui.Tester, ui.Rect, theme.Tokens, int) {
	t.Helper()
	en := i18n.For("en").Settings
	s := settings.Default()
	s.UIScale = step
	s.FontFamily = family
	a := New(settings.NewStore(s), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	a.Page = PageGeneral
	tt, tokens := renderIn(t, a, launcher.InputWindowWidth, 1400, dark)
	label := mustFind(t, tt, en.FontFamily)
	box := pillBox(tt.Image(), label)
	if box < 100 {
		t.Fatalf("step=%s family=%q dark=%v: the picker's face is only %dpx wide — the box could not be "+
			"read off the frame", step, family, dark, box)
	}
	return tt, label, tokens, box
}

// pillBox is the picker's painted width in the label's own row: the pill's face
// is the band's most common colour (the band starts at the label column's right
// edge, which is the pill's left edge), and every column that carries it at any
// row of the band belongs to the box.
func pillBox(img *image.RGBA, label ui.Rect) int {
	y0, y1 := int(label.Y)-2, int(label.Y+label.H)+2
	right := contentRightAt720
	x0, x1 := int(label.X+label.W), int(right)
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
	return hi - lo + 1
}

// The round's visual evidence: the picker holding a value longer than its
// column, at the shipped step and at the largest one, both appearances. Opt-in
// so a normal run does not litter /tmp.
func TestDumpP13FNPickerFloor(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	const long = "Iosevka Term Extended Nerd Font Mono"
	for _, arm := range []struct {
		step, name string
		dark       bool
	}{
		{"small", "light", false},
		{"small", "dark", true},
		{"large", "light", false},
		{"large", "dark", true},
	} {
		en := i18n.For("en").Settings
		s := settings.Default()
		s.UIScale = arm.step
		s.FontFamily = long
		a := New(settings.NewStore(s), Actions{
			SetIntegrationEnabled: func(string, bool, func(error)) {},
			SetCommandEnabled:     func(string, string, bool) {},
		})
		a.Page = PageGeneral
		tt, _ := renderIn(t, a, launcher.InputWindowWidth, 1400, arm.dark)
		label := mustFind(t, tt, en.FontFamily)
		writePNG(t, filepath.Join("/tmp", "floter-P13-fn-picker-"+arm.step+"-"+arm.name+"-after.png"), tt)
		_ = label
	}
}
