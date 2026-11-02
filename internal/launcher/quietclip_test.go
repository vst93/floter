package launcher

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-bl · 空态那句话在最窄的窗口里**没有**被切——这条 pin 把这个边界钉住。
//
// 这一页的句子是面板里唯一一条「句子级」文本：`Emphasis` 字阶、`SingleLine()`、
// 居中摆在列表本该占据的那条带里（`launcher.go` 的 `quietPage` 臂）。它没有
// `Ellipsis`，所以**一旦**句子的自然宽度超过带子，`SingleLine` 会直接把它切掉，
// 两端都不留记号——一句话被切成半截，读起来是界面坏了，不是「话太长」。
//
// 本轮先量：最长的一条（浏览器插件关掉的那句，英文 74 个字符）在窗口最窄的
// 520u 上，墨只占 422u（x=49..471），带子内边距是 12u——两端各留 37u / 37u。
// 也就是说今天**还没有**被切；这条 pin 锁的是「以后改文案/加语言也不许切」，
// 而不是修一个已经发生的缺陷（报告里记作 `故意不做`：不改成 `Wrap()`，因为
// 换行会把居中一句话变成两行、动到带子的视觉重心，而今天没有这个需要）。
//
// 量的是**墨的两端**：把带子按行切成「有墨的行段」，句子自己那一段的左右端点
// 必须留在带子的 16u 内边距之内。切了的话它会一直顶到卡片边。

// inkRun is one vertical run of rows that carry ink, with the horizontal extent
// of everything drawn in it.
type inkRun struct {
	Top, Bottom int
	Left, Right int
}

// lumaAt is the pixel's own level, or -1 where nothing was painted.
func lumaAt(img *image.RGBA, x, y int) int {
	r, g, b, a := img.At(x, y).RGBA()
	if a>>8 == 0 {
		return -1
	}
	return (int(r>>8) + int(g>>8) + int(b>>8)) / 3
}

// inkRuns splits the band into the runs of rows that carry ink, skipping the
// card's own frame (the outer 4u on each side), so a hairline is not read as
// the sentence.
func inkRuns(img *image.RGBA, box ui.Rect) []inkRun {
	face := lumaAt(img, int(box.X)+6, int(box.Y+box.H/2))
	if face < 0 {
		return nil
	}
	var runs []inkRun
	cur := -1
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		left, right := -1, -1
		for x := int(box.X) + 4; x < int(box.X+box.W)-4; x++ {
			l := lumaAt(img, x, y)
			if l < 0 {
				continue
			}
			if d := l - face; d > 8 || d < -8 {
				if left < 0 {
					left = x
				}
				right = x
			}
		}
		if left < 0 {
			cur = -1
			continue
		}
		if cur < 0 {
			runs = append(runs, inkRun{Top: y, Left: left, Right: right})
			cur = len(runs) - 1
			continue
		}
		runs[cur].Bottom = y + 1
		if left < runs[cur].Left {
			runs[cur].Left = left
		}
		if right > runs[cur].Right {
			runs[cur].Right = right
		}
	}
	for i := range runs {
		if runs[i].Bottom == 0 {
			runs[i].Bottom = runs[i].Top + 1
		}
	}
	return runs
}

// browserOffApp is the browser page with the plugin switched off: the longest
// sentence the quiet page can be asked to draw.
func browserOffApp(t *testing.T) *App {
	t.Helper()
	a := testApp()
	a.Actions.SearchBrowser = func(query string, done func(BrowserResults)) {
		done(BrowserResults{Disabled: true})
	}
	a.EnterBrowser()
	return a
}

func TestTheQuietPagesSentenceIsNeverCut(t *testing.T) {
	for _, width := range []int{520, InputWindowWidth} {
		a := browserOffApp(t)
		tt := renderAt(t, a, width, themedFrameHeight(maxResultSlots), false)
		img := tt.Image()
		if len(tt.Texts()) == 0 {
			t.Fatalf("%d: the quiet page drew nothing", width)
		}
		band := ui.Rect{X: 0, Y: 56, W: float32(width), H: float32(themedFrameHeight(maxResultSlots)) - 56}
		runs := inkRuns(img, band)
		// The chips row is the first run; the sentence is the next one, and it
		// is the only other thing in the band.
		if len(runs) < 2 {
			t.Fatalf("%d: expected the chips row and the sentence, got %d runs: %v", width, len(runs), runs)
		}
		sentence := runs[len(runs)-1]
		t.Logf("%d: %d runs, sentence %+v", width, len(runs), sentence)
		if sentence.Left < 16 {
			t.Errorf("%d: the sentence's ink starts at x=%d, inside the band's 16u margin — it is cut at the card's edge, not wrapped", width, sentence.Left)
		}
		if sentence.Right > width-16 {
			t.Errorf("%d: the sentence's ink ends at x=%d, inside the band's 16u margin — it is cut at the card's edge, not wrapped", width, sentence.Right)
		}
	}
}
