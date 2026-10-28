package terminalui

import (
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P11-au · **抽屉放不下时，最后几行要能被滚到。**
//
// P11-at 只修了「抽屉不让开浮条那条带」：面板比网格矮时，抽屉排在画布之下，
// 多出来的几行被**面板底边**裁掉（实测 640×360 / 720×360，`Safe paste` 只剩
// 3u 高，`Terminal palette` 整个在视口外）。要修得让条带**自己**报出一段可滚动
// 的范围——P11-at 试过把网格包进 `ui.Scroll`，`MaxY` 恒为 0，因为那个 scroll 在
// 横向 Row 里 `Grow(1)` 只吃宽度、高度仍等于内容的自然高，自己就和内容一样高。
//
// 本轮把**条带本身**做成滚动容器（面板列的直接孩子，`layout.go:723` 让滚动元素
// 在列里 `minMain` 自动为 0）。这条 pin 钉两件事：
//  1. 面板比条带矮时，条带如实报出 `MaxY > 0`（否则最后几行根本够不到）；
//  2. 滚到头以后，最后一个 appearance 轴（`Terminal palette`）完整落在条带的
//     视口里，不再被底边裁。
//
// 窗口够高时一点都不该变：`MaxY == 0`，偏移仍是 0。
func TestTheDrawerScrollsItsLastRowsIntoView(t *testing.T) {
	const eps = 0.5
	for _, lang := range []string{"en", "zh"} {
		scopy := i18n.For(lang).Settings
		for _, size := range [][2]int{
			{settings.MinTerminalWidth, settings.MinTerminalHeight},
			{720, settings.MinTerminalHeight},
		} {
			a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
			withSession(a, &terminal.Terminal{})
			a.Drawer = true
			if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
				t.Fatalf("%s %dx%d: %v", lang, size[0], size[1], err)
			}
			tt := render(t, a, size[0], size[1])
			strip, ok := tt.Find(scopy.GroupTerminal)
			if !ok {
				t.Fatalf("%s %dx%d: the drawer's strip is not on the frame: %q",
					lang, size[0], size[1], tt.Texts())
			}
			if a.DrawerScroll.MaxY <= 0 {
				t.Fatalf("%s %dx%d: the drawer reports no scroll range (MaxY=%.1f) — "+
					"the strip is %.1fu tall and its content does not fit, so the last rows "+
					"are clipped with no way to reach them",
					lang, size[0], size[1], a.DrawerScroll.MaxY, strip.H)
			}
			// Scroll to the end: the last appearance axis must be whole inside the
			// strip's viewport, not cut by its bottom edge.
			a.DrawerScroll.Y = math.MaxFloat32
			tt.Frame()
			strip, ok = tt.Find(scopy.GroupTerminal)
			if !ok {
				t.Fatalf("%s %dx%d: the strip left the frame after scrolling", lang, size[0], size[1])
			}
			last, ok := tt.Find(scopy.Palette)
			if !ok {
				t.Fatalf("%s %dx%d: the last appearance row (%q) is not on the frame even "+
					"after scrolling to the end: %q", lang, size[0], size[1], scopy.Palette, tt.Texts())
			}
			if last.Y < strip.Y-eps || last.Y+last.H > strip.Y+strip.H+eps {
				t.Errorf("%s %dx%d: the last row %+v still falls outside the strip's viewport %+v "+
					"after scrolling to the end", lang, size[0], size[1], last, strip)
			}
		}
	}
}

// The other half of the same rule: a window tall enough for the whole strip
// must not gain a scroll range, and its offset must stay at the start. A
// container that always reported a range would let the wheel move a drawer
// that fits.
func TestTheDrawerDoesNotScrollWhenItFits(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		a.Drawer = true
		tt := render(t, a, 720, 600)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		if a.DrawerScroll.MaxY != 0 {
			t.Errorf("dark=%v: a drawer that fits reports a scroll range (MaxY=%.1f)",
				dark, a.DrawerScroll.MaxY)
		}
		if a.DrawerScroll.Y != 0 {
			t.Errorf("dark=%v: a drawer that fits is scrolled (Y=%.1f)", dark, a.DrawerScroll.Y)
		}
	}
}

// TestDumpP11AUDrawerScrolledPNGs is the round's visual evidence: the terminal
// at its minimum height with the drawer scrolled to its end, both appearances.
// Opt-in (`FLOTER_PNG=1`) like the other dumps.
func TestDumpP11AUDrawerScrolledPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		a.Drawer = true
		tt := render(t, a, settings.MinTerminalWidth, settings.MinTerminalHeight)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		a.DrawerScroll.Y = math.MaxFloat32
		tt.Frame()
		path := "/tmp/floter-P11-au-drawer-end-" + appearance + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
