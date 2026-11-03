package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-fj · 空态的上下空气：区域自己那 24u 就是它的呼吸，floor 只是地板。
//
// 通用队列「空态满态」的最后一格（P13-fi「仍待」第 1 条）。§6.9 钉的是**部件之间**
// 的三对缝（8 / 12 / 23px），§6.13 钉的是出口的种类；**区域自己的上下空气从来没量过**
// ——它此前只是「`Padding(Space(6))` + `Center()`」这一句散文。
//
// 实测（真画法，720 宽，深浅两套逐值相同）：
//
//	arm            区域盒高   内容+2×24u   floor     下空气（末件底 → 区域底）
//	sessions       154.6      154.6        144.0     21.6
//	custom         147.8      147.8         82.8     21.6
//	integrations    84.6       84.6         82.8     21.6
//
// 三句话：
//
//	① 下空气在三个调用点上**都是 21.6**（= `Space(6)`，区域自己的内边距）：空态的密度
//	   不由调用点挑，由这一块区域挑一次。
//	② 上空气是同一个数：标记盒顶 = 区域顶 + 21.6（`title.Y − box.Y = 24u + 22u + 4u`
//	   = 45.0，逐十分位相等），所以整块是**上下等呼吸**。
//	③ 两个 floor 基本是**地板**而不是高度：只有集成页（`84.6` 对 `82.8`）差 1.8px 就
//	   生效；会话页的 160u（=144）被内容反超 10.6、自定义快捷键的 92u 被反超 65。
//	   写下来是为了下次别再以为「区域高 = floor」。
//
// 这一轮**零像素改动**：三个调用点、两套外观，空气都是同一个数，没有一处要抬。
// pin 守的是「密度只有一处决定」——把 `Padding(Space(6))` 改成 5u/7u、或给某个调用点
// 加一层 `Margin`，红的是它。

type emptyAirReading struct {
	box, title, last ui.Rect
	pad, mark, gap   float32 // the three numbers `emptyState` asks the theme for
}

// air is the space between the last part's bottom and the region's own bottom.
func (r emptyAirReading) air() float32 {
	return r.box.Y + r.box.H - (r.last.Y + r.last.H)
}

// top is the space between the region's top and the mark's box, derived from
// the title's measured offset: everything above the title is padding + mark +
// head gap, so the equation only holds if nothing was added above the mark.
func (r emptyAirReading) top() float32 {
	return r.title.Y - r.box.Y - r.mark - r.gap
}

// emptyAirProbe draws one of the region's three shipped arms through the
// product's own `emptyState` and returns the boxes the pins read.
func emptyAirProbe(t *testing.T, dark bool, arm string) emptyAirReading {
	t.Helper()
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	var ref, lastRef ui.Handle
	var out emptyAirReading
	var title string
	hasLast := false
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		th := c.Theme()
		out.pad, out.mark, out.gap = th.Space(6), th.Space(5.5), th.Space(1)
		var hint string
		var floor float32
		var plate bool
		var action func()
		switch arm {
		case "sessions":
			face := sessionsEmptyFaceFor(en, false, false)
			title, hint, floor, plate = face.Title, face.Hint, th.Space(40), true
			hasLast = true
			action = func() { a.pageButton(c, en.SessionsNew).Bind(&lastRef) }
		case "custom":
			title, hint = en.ShortcutsNone, en.ShortcutsEmptyHint
			hasLast = true
			action = func() { ui.Text(c, en.ShortcutsAddFirst).Bind(&lastRef) }
		case "integrations":
			title = en.IntegrationsEmpty
		default:
			t.Fatalf("unknown arm %q", arm)
		}
		e := a.emptyState(c, "keyboard", title, hint, 0, floor, plate, action)
		e.Bind(&ref)
		out.box = c.Resolve(ref).Bounds()
		if hasLast {
			out.last = c.Resolve(lastRef).Bounds()
		}
	}, 720, 420)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	tr, ok := tt.Find(title)
	if !ok {
		t.Fatalf("%s: the region's title is missing: %q", arm, tt.Texts())
	}
	out.title = tr
	if out.last.H == 0 {
		out.last = tr
	}
	return out
}

// TestAnEmptyRegionBreathesWithOneNumberOnBothSides: the air above the mark and
// the air under the last part are the same number at every call site, in both
// appearances — and that number is the region's own 24u padding.
func TestAnEmptyRegionBreathesWithOneNumberOnBothSides(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, arm := range []string{"sessions", "custom", "integrations"} {
			r := emptyAirProbe(t, dark, arm)
			if d := r.air() - r.pad; d > 0.6 || d < -0.6 {
				t.Errorf("%s (dark=%v): the air under the last part is %.1f, want the region's own padding %.1f",
					arm, dark, r.air(), r.pad)
			}
			if d := r.top() - r.pad; d > 0.6 || d < -0.6 {
				t.Errorf("%s (dark=%v): the air above the mark is %.1f, want the same padding %.1f (nothing may sit above the mark)",
					arm, dark, r.top(), r.pad)
			}
		}
	}
}

// TestTheEmptyRegionGrowsPastItsFloor: the floor is a floor. Two of the three
// call sites already stand taller than it, so a page that reads "the region is
// 160u" is reading a number the user never sees.
func TestTheEmptyRegionGrowsPastItsFloor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		sessions := emptyAirProbe(t, dark, "sessions")
		custom := emptyAirProbe(t, dark, "custom")
		if sessions.box.H <= 144 {
			t.Errorf("the sessions region is %.1f tall — the page-body floor (160u = 144) is now the height, not the floor",
				sessions.box.H)
		}
		if custom.box.H <= 82.8 {
			t.Errorf("the custom-shortcuts region is %.1f tall — the list floor (92u = 82.8) is now the height",
				custom.box.H)
		}
		// One density across surfaces: the two regions differ only by content.
		if d := sessions.air() - custom.air(); d > 0.6 || d < -0.6 {
			t.Errorf("dark=%v: the two regions breathe differently (%.1f vs %.1f)", dark, sessions.air(), custom.air())
		}
	}
}

// TestDumpP13FJEmptyAirPNGs is the opt-in evidence dump (FLOTER_P13FJ_DIR):
// the three arms, both appearances.
func TestDumpP13FJEmptyAirPNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13FJ_DIR")
	if dir == "" {
		t.Skip("set FLOTER_P13FJ_DIR to dump the empty-region evidence")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		a := New(newStore(t), Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			th := c.Theme()
			ui.Column(c).Gap(th.Space(2)).Children(func() {
				for _, arm := range []string{"sessions", "custom", "integrations"} {
					var title, hint string
					var floor float32
					var plate bool
					var action func()
					switch arm {
					case "sessions":
						face := sessionsEmptyFaceFor(en, false, false)
						title, hint, floor, plate = face.Title, face.Hint, th.Space(40), true
						action = func() { a.pageButton(c, en.SessionsNew) }
					case "custom":
						title, hint = en.ShortcutsNone, en.ShortcutsEmptyHint
						action = func() { ui.Text(c, en.ShortcutsAddFirst) }
					default:
						title = en.IntegrationsEmpty
					}
					a.emptyState(c, "keyboard", title, hint, 0, floor, plate, action)
				}
			})
		}, 720, 560)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		name := "light"
		if dark {
			name = "dark"
		}
		path := filepath.Join(dir, fmt.Sprintf("floter-P13-fj-emptyair-%s.png", name))
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
