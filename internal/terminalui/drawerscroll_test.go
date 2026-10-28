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

// P11-ax · **抽屉关闭再打开，从它自己的开头开始。**
//
// `DrawerScroll` 是 App 级的（像列表的 `ListState`），所以关闭再打开时上一轮
// 滚到的偏移原样留着——一份「新打开的抽屉」出现在上次滚到的位置。这与
// P11-ao（换设置页回顶）和 P11-ar（一份新的捕获输出从它自己的开头开始）是同一
// 条规则：一份新内容打开时从开头开始。归零只在 `ToggleDrawer` 的「打开」动作
// 上，逐帧路径不碰（否则滚轮每帧被清零）。
//
// 两条臂缺一不可：第一臂钉「重开回顶」，第二臂钉「开着时滚轮仍能滚」——后者
// 防止用「干脆把滚动关掉」来让第一臂变绿（那会让 P11-au 的「最后几行够得到」
// 重新坏掉）。
func TestReopeningTheDrawerStartsAtItsTop(t *testing.T) {
	a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	// Minimum height: the strip is taller than its viewport, so there is a
	// real scroll range to lose.
	tt := render(t, a, settings.MinTerminalWidth, settings.MinTerminalHeight)

	// Open it, scroll to the end, close it, open it again.
	a.ToggleDrawer()
	tt.Frame()
	if a.DrawerScroll.MaxY <= 0 {
		t.Fatalf("the drawer reports no scroll range (MaxY=%.1f) at %dx%d — the fixture "+
			"cannot exercise the reopen rule", a.DrawerScroll.MaxY,
			settings.MinTerminalWidth, settings.MinTerminalHeight)
	}
	a.DrawerScroll.Y = math.MaxFloat32
	tt.Frame()
	scrolled := a.DrawerScroll.Y
	if scrolled <= 0 {
		t.Fatalf("the drawer did not scroll to its end (Y=%.1f)", scrolled)
	}
	a.ToggleDrawer() // close
	tt.Frame()
	a.ToggleDrawer() // open again
	tt.Frame()
	if a.DrawerScroll.Y != 0 {
		t.Errorf("a reopened drawer starts scrolled (Y=%.1f, was %.1f at its end) — a fresh "+
			"drawer must start at its own top (P11-ax, the same rule as P11-ao/P11-ar)",
			a.DrawerScroll.Y, scrolled)
	}

	// Second arm: while the drawer is open, the wheel still moves it. A fix
	// that killed the scroll range would make the first arm pass for the wrong
	// reason and break P11-au's "the last rows are reachable".
	a.DrawerScroll.Y = math.MaxFloat32
	tt.Frame()
	if a.DrawerScroll.Y <= 0 {
		t.Errorf("the drawer no longer scrolls while it is open (Y=%.1f) — the reopen reset "+
			"must not take the scroll range away (P11-au)", a.DrawerScroll.Y)
	}
}

// TestDumpP11AXReopenPNGs is the round's visual evidence: the drawer scrolled
// to its end, then closed and reopened at its top, both appearances. Opt-in
// (`FLOTER_PNG=1`) like the other dumps.
func TestDumpP11AXReopenPNGs(t *testing.T) {
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
		tt := render(t, a, settings.MinTerminalWidth, settings.MinTerminalHeight)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		a.ToggleDrawer()
		tt.Frame()
		a.DrawerScroll.Y = math.MaxFloat32
		tt.Frame()
		scrolled := "/tmp/floter-P11-ax-drawer-" + appearance + "-scrolled.png"
		f, err := os.Create(scrolled)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		a.ToggleDrawer() // close
		tt.Frame()
		a.ToggleDrawer() // open again
		tt.Frame()
		reopened := "/tmp/floter-P11-ax-drawer-" + appearance + "-reopened.png"
		f, err = os.Create(reopened)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s (Y=%.1f) and %s (Y=%.1f)", scrolled, math.MaxFloat32, reopened, a.DrawerScroll.Y)
	}
}
