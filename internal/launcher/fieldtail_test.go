package launcher

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
)

// P13-bk · 字段行尾部那一格（`copy.CommandModeHint`）该属于谁。
//
// `i18n/i18n.go:117-119` 把它定义成「扩展命令的参数正在输入时」的提示。HEAD `f7d6a688`
// 上它的条件却是 `a.mode != nil || a.clipboard || a.browser`：剪贴板页与浏览器页也画它，
// 而那两页没有命令可跑，同一帧的 chips 行还写着「Tab to switch」——一个窗口对同一个键
// 两个相反的说法（实测：chips 行 `Tab to switch · Ctrl+D to favorite` ink 144/112，
// 字段行 `Enter runs it · Tab completes · Esc cancels` ink 116/164）。命令模式的空态上
// 这句话还被画了两遍（页面陈述 + 字段尾部，`tt.Texts()` 计数 = 2）。
//
// 本轮的 pin 分两半：**说什么**（哪一条文案出现在哪一页）与**怎么画**（同一族的五个
// 插件模式尾部必须是同一档墨、同一个位置）。

// tailPeak is the ink a text rectangle is drawn with: the pixel farthest (summed
// channels) from the plane it sits on. Reading the darkest pixel would read the
// plane itself in the dark appearance, where the ink is lighter than its ground
// (the P13-bj lesson, `appearance_test.go`'s `inkOn`).
func tailPeak(img *image.RGBA, r ui.Rect, ground color.RGBA) (color.RGBA, int) {
	var ink color.RGBA
	far := -1
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := img.RGBAAt(x, y)
			if d := channelDistance(c, ground); d > far {
				far, ink = d, c
			}
		}
	}
	return ink, far
}

// TestTheClipboardAndBrowserPagesNeverContradictTheirChipsRow 是 21-1 的 pin：
// 剪贴板页与浏览器页的 chips 行写着「Tab to switch」，所以字段行**不许**再写
// 「Tab completes」；这两页与计算器/文件页一样，尾部是字段自己的那句
// `copy.Hint`（有查询时是 ✕ 清除键）。
func TestTheClipboardAndBrowserPagesNeverContradictTheirChipsRow(t *testing.T) {
	pages := []struct {
		name string
		open func(a *App)
	}{
		{"clipboard", func(a *App) { a.EnterClipboard() }},
		{"browser", func(a *App) { a.EnterBrowser() }},
		{"calculator", func(a *App) { a.EnterCalculator() }},
		{"files", func(a *App) { a.EnterFiles() }},
	}
	forEachAppearance(t, func(t *testing.T, dark bool) {
		for _, page := range pages {
			t.Run(page.name, func(t *testing.T) {
				a := testApp()
				page.open(a)
				copy := StringsFor(a.settings().Language)
				tt := renderIn(t, a, dark)
				tt.Frame()
				if tt.HasText(copy.CommandModeHint) {
					t.Errorf("%s page draws the command-mode hint %q beside its own "+
						"`Tab to switch` chips hint: %v", page.name, copy.CommandModeHint, tt.Texts())
				}
				if !tt.HasText(copy.Hint) {
					t.Errorf("%s page's field tail says nothing: %v", page.name, tt.Texts())
				}

				// With a needle the tail is the clear key — the same control the
				// calculator and files pages have always had.
				b := testApp()
				page.open(b)
				b.SetQuery("zzzz")
				tt2 := renderIn(t, b, dark)
				tt2.Frame()
				if tt2.HasText(copy.CommandModeHint) {
					t.Errorf("%s page with a needle still claims %q: %v",
						page.name, copy.CommandModeHint, tt2.Texts())
				}
				if !tt2.HasText(copy.Clear) {
					t.Errorf("%s page with a needle has no clear key: %v", page.name, tt2.Texts())
				}
			})
		}
	})
}

// TestTheCommandModeSaysItsHintOnce 是 21-2 的 pin：命令模式的空态页面已经把这句话
// 当**陈述**画过（`emptyMessage` 在 `a.mode != nil` 返回同一条 `CommandModeHint`），
// 字段尾部不许再说第二遍。改前 `tt.Texts()` 里它出现两次（实测 2）。
func TestTheCommandModeSaysItsHintOnce(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		a := testApp()
		a.enterCommand(extensions.CommandEntry{Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}})
		a.SetQuery("zzzz")
		copy := StringsFor(a.settings().Language)
		tt := renderIn(t, a, dark)
		tt.Frame()

		n := 0
		for _, got := range tt.Texts() {
			if got == copy.CommandModeHint {
				n++
			}
		}
		if n != 1 {
			t.Errorf("the command-mode hint is drawn %d times, want 1: %v", n, tt.Texts())
		}
		r, ok := tt.Find(copy.CommandModeHint)
		if !ok {
			t.Fatalf("the hint is not on screen: %v", tt.Texts())
		}
		// The one copy is the page's statement, in the list band — not the field
		// row's tail (the row is the top 56u).
		if r.Y < 56 {
			t.Errorf("the surviving copy sits in the field row (y=%.0f), not on the page", r.Y)
		}
	})
}

// TestTheFourPluginModesWearOneFieldTail 是 21-3 的 pin（像素级）：五个插件模式里
// 尾部是**同一个物体**——同一条文案、同一档墨、同一个字阶（行盒同高）。改前剪贴板/浏览器
// 与计算器/文件是两种说法、两个宽度。
func TestTheFourPluginModesWearOneFieldTail(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		var want ui.Rect
		var wantInk color.RGBA
		for i, page := range []struct {
			name string
			open func(a *App)
		}{
			{"clipboard", func(a *App) { a.EnterClipboard() }},
			{"browser", func(a *App) { a.EnterBrowser() }},
			{"calculator", func(a *App) { a.EnterCalculator() }},
			{"files", func(a *App) { a.EnterFiles() }},
		} {
			a := testApp()
			page.open(a)
			copy := StringsFor(a.settings().Language)
			tt := renderIn(t, a, dark)
			tt.Frame()
			r, ok := tt.Find(copy.Hint)
			if !ok {
				t.Fatalf("%s: the tail is not on screen: %v", page.name, tt.Texts())
			}
			img := tt.Image()
			ground := img.RGBAAt(6, img.Rect.Dy()-6)
			ink, _ := tailPeak(img, r, ground)
			if i == 0 {
				want, wantInk = r, ink
				continue
			}
			if r.H != want.H {
				t.Errorf("%s: the tail's line box is %0.f, the clipboard page's is %0.f",
					page.name, r.H, want.H)
			}
			if d := channelDistance(ink, wantInk); d > 2 {
				t.Errorf("%s: the tail's ink is %v, the clipboard page's is %v (%d apart)",
					page.name, ink, wantInk, d)
			}
		}
	})
}

// TestTheFieldTailIsStrongerThanTheChipsAside 给尾部钉一条**绝对**判据：字段行的那句
// 指示（`copy.Hint`）与 chips 行的那句旁注（`copy.ClipboardFilterHint`）在同一帧里，
// 前者必须明显重于后者。两个都是「关于这一行的提示」，但字段行是这一页的输入面、chips
// 行是它下面的一个带子——同一档墨会让两句话读成一句话的两半。
//
// 实测（浅 253 / 深 23 地面，峰值对比）：尾部 **412 / 421**，chips 旁注 **328 / 267**。
// 这条 pin 也是「把尾部退到 `TextTertiary`」那种改动的唯一哨兵——四个页同时退一档时，
// 跨页比较（`TestTheFourPluginModesWearOneFieldTail`）看不出任何差别。
func TestTheFieldTailIsStrongerThanTheChipsAside(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		a := testApp()
		a.EnterClipboard()
		copy := StringsFor(a.settings().Language)
		tt := renderIn(t, a, dark)
		tt.Frame()
		img := tt.Image()
		ground := img.RGBAAt(6, img.Rect.Dy()-6)

		tail, ok := tt.Find(copy.Hint)
		if !ok {
			t.Fatalf("the field tail is not on screen: %v", tt.Texts())
		}
		aside, ok := tt.Find(copy.ClipboardFilterHint(a.favoriteBadge()))
		if !ok {
			t.Fatalf("the chips row's aside is not on screen: %v", tt.Texts())
		}
		_, tailDist := tailPeak(img, tail, ground)
		_, asideDist := tailPeak(img, aside, ground)
		if tailDist < asideDist+50 {
			t.Errorf("the field's own instruction is %d from the ground, the chips row's "+
				"aside is %d: one register for two roles", tailDist, asideDist)
		}
	})
}

// TestTheFieldTailIsGuardedByTheQuietPage 是 21-3 的结构 pin：重复守卫与空态列必须读
// **同一个**判据。两处各写一遍是本轮修掉的那种漂移（页面说一遍、字段再说一遍）。
func TestTheFieldTailIsGuardedByTheQuietPage(t *testing.T) {
	src, err := os.ReadFile("launcher.go") // #nosec G304 -- the package's own source
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if strings.Contains(body, "case a.mode != nil || a.clipboard || a.browser:") {
		t.Error("the field tail is still handed to the clipboard and browser pages")
	}
	if !strings.Contains(body, "case a.mode != nil:") {
		t.Error("the command mode's own arm is gone")
	}
	if !strings.Contains(body, "quietPage := a.output == nil && !a.configOpen() && len(results) == 0") {
		t.Error("the quiet-page predicate is not the one this round wrote")
	}
	if !strings.Contains(body, "} else if quietPage {") {
		t.Error("the empty column no longer reads the shared predicate")
	}
	if !strings.Contains(body, "if !quietPage {\n\t\t\t\t\tui.Text(c, copy.CommandModeHint)") {
		t.Error("the tail is not guarded by the quiet page")
	}
}

// The visual evidence: the clipboard page and the command mode's empty page,
// before and after, in both appearances. Opt-in for the same reason as the other
// dumps (`FLOTER_P13BK_DUMP=1`).
func TestDumpTheFieldTailPages(t *testing.T) {
	if os.Getenv("FLOTER_P13BK_DUMP") == "" {
		t.Skip("set FLOTER_P13BK_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13BK_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, page := range []struct {
			name string
			open func(a *App)
		}{
			{"clipboard", func(a *App) { a.EnterClipboard() }},
			{"browser", func(a *App) { a.EnterBrowser() }},
			{"commandmiss", func(a *App) {
				a.enterCommand(extensions.CommandEntry{Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}})
				a.SetQuery("zzzz")
			}},
		} {
			a := testApp()
			page.open(a)
			tt := renderIn(t, a, dark)
			tt.Frame()
			img := tt.Image()
			path := filepath.Join("/tmp", "floter-P13-bk-"+page.name+"-"+appearance+"-"+tag+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s %s (%dx%d) %s", page.name, appearance, img.Rect.Dx(), img.Rect.Dy(), path)
			t.Logf("  texts: %v", tt.Texts())
		}
	}
}
