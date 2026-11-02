package launcher

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/settings"
)

// P13-w · 捕获输出页的折边退后（`docs/rounds/P13-w-spec.md`）。
//
// P13-v 把「折边之外的东西是退后，不是被切」落在启动器列表上，捕获输出页
// （同一张卡里的另一张列表）一直没接：滚到中间时实测 `Row 3` 的字母在 y=122
// 一刀断掉（y=111..121 有 `darkest = 25` 的墨，y=122 起是 235 的板）。
//
// 这一页的读数和那边不一样——40u 的行盒里字盒从 y+9 起，折边常常切在字盒的
// 前导里，可见的那几像素是**图标板的顶边**（实测 y=484..487 `darkest = 235`，
// 字一个都没有）。所以这些 pin 量的是**同一段像素在不同折边下的对比度**，
// 而不是「折边行比整行浅」——后者在改之前就成立（板本来就没字深），钉不住东西。
//
// 对比度按「离这一页自己的面有多远」算，不按「有多黑」：深色档里字比面亮，
// 拿 darkest 去量会量到面本身。

// faceIn is the surface the list draws on: the row block sits 8u in from the
// list's own edge, so the gutter beside it is the panel's face.
func faceIn(img *image.RGBA, viewport ui.Rect, y int) int {
	r, g, b, a := img.At(int(viewport.X)+2, y).RGBA()
	if a>>8 == 0 {
		return -1
	}
	return (int(r>>8) + int(g>>8) + int(b>>8)) / 3
}

// contrastIn is the strongest departure from the face inside box ∩ viewport:
// 0 means "nothing is drawn here", and a bigger number means more ink. It is
// the appearance-independent reading of "this row recedes" — the same number
// works on a light panel (ink darker than the face) and a dark one (ink
// lighter than it).
func contrastIn(img *image.RGBA, box, viewport ui.Rect) int {
	top := max(box.Y, viewport.Y)
	bottom := min(box.Y+box.H, viewport.Y+viewport.H)
	left := max(box.X, viewport.X)
	right := min(box.X+box.W, viewport.X+viewport.W)
	if bottom <= top || right <= left {
		return -1
	}
	face := faceIn(img, viewport, int(top+(bottom-top)/2))
	if face < 0 {
		return -1
	}
	best := 0
	for y := int(top); y < int(bottom); y++ {
		for x := int(left); x < int(right); x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a>>8 == 0 {
				continue
			}
			l := (int(r>>8) + int(g>>8) + int(b>>8)) / 3
			if d := l - face; d < 0 {
				if -d > best {
					best = -d
				}
			} else if d > best {
				best = d
			}
		}
	}
	return best
}

// clippedRow is the first row the fold cuts in this frame: the one with a box
// on screen whose bottom runs past the list's clip.
//
// P13-ef · 「切」按**像素**判，不按比例判。`visibleShare` 是 float32 除法：出货尺上
// 行盒 `y=100.4, h=36` 在 318.6 高的视口里算出来是 **0.99999997**，于是「整行都在
// 视口里」的第一行也会被当成「被折边切掉一条缝」——量到的是第一行的头带，而它的
// fold 是 1，两条读数一样，pin 就在「折边没打开」上误报。判据改成画的时候真的溢出：
// 行的最后一行像素在视口底之下。
func clippedRow(a *App) (int, ui.Rect, bool) {
	clipBottom := int(math.Round(float64(a.outputListRect.Y + a.outputListRect.H)))
	for i := 0; i < len(a.outputRowRects); i++ {
		box, seen := a.outputRowRects[i]
		if !seen || box.H <= 0 {
			continue
		}
		top := int(math.Round(float64(box.Y)))
		bottom := int(math.Round(float64(box.Y + box.H)))
		if top < clipBottom && bottom > clipBottom {
			return i, box, true
		}
	}
	return 0, ui.Rect{}, false
}

// headBand is the top of a row: the band the fold always shows, whatever the
// frame height. Measuring the same pixels at two folds is what separates
// "receded" from "clipped".
func headBand(box ui.Rect) ui.Rect {
	return ui.Rect{X: box.X, Y: box.Y, W: box.W, H: 8}
}

// runListOutputAt is `runListOutput` at a frame height of the test's own
// choosing, so the same rows can be read through two different folds.
func runListOutputAt(t *testing.T, a *App, stdout string, height int) *ui.Tester {
	t.Helper()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Command: []string{"tool"}, Success: true, Stdout: stdout}, nil)
	}
	tt := ui.NewTester(themedView(a), InputWindowWidth, height)
	tt.Frame()
	a.Query = "list"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	return tt
}

// The rule is a *scale*: the same pixels, seen through a tighter fold, carry
// less ink. Two frames, same row, 10 of its 40u on screen vs 30.
func TestTheFurtherTheOutputRowIsCutTheMoreItRecedes(t *testing.T) {
	inkAt := func(height int) (ink int, share float64) {
		a := testApp()
		tt := runListOutputAt(t, a, outputRowsWithActions(12), height)
		tt.Frame()
		if !a.InOutputView() || !a.output.IsList {
			t.Fatalf("the capture did not open as the list page: %+v", a.output)
		}
		i, box, ok := clippedRow(a)
		if !ok {
			t.Fatalf("no row is cut at height %d: %v", height, a.outputRowRects)
		}
		t.Logf("height %d: row %d box=%+v share=%.3f", height, i, box, visibleShare(box, a.outputListRect))
		return contrastIn(tt.Image(), headBand(box), a.outputListRect), visibleShare(box, a.outputListRect)
	}
	// The row the fold cuts at the shorter frame is the same row at the taller
	// one — the rows do not move when the frame grows, only the clip does.
	//
	// P13-ef · 两档都读出货尺（`themedFrameHeight`）。旧的两个高度是库存尺的 488/508；
	// 出货尺上行盒是 36px（40u × 0.9），出货帧 439 把第 10 行切在 14.6/36（实测
	// share 0.406），再高 20 的 459 只切掉 1px（share 0.989）—— 正是这条 pin 要的
	// 「同一行、两个折边」。
	tight, tightShare := inkAt(themedFrameHeight(maxResultSlots))
	loose, looseShare := inkAt(themedFrameHeight(maxResultSlots) + 20)
	if tight < 0 || loose < 0 {
		t.Fatalf("one of the two frames has no readable band: %d, %d", tight, loose)
	}
	if looseShare <= tightShare {
		t.Fatalf("the premise failed: the fold did not open (%.3f → %.3f)", tightShare, looseShare)
	}
	t.Logf("head band: %.3f visible → contrast %d | %.3f visible → contrast %d",
		tightShare, tight, looseShare, loose)
	if loose <= tight {
		t.Errorf("the row's head keeps its ink whether the fold shows 10 of its 40u "+
			"or 30: contrast %.3f visible = %d, %.3f visible = %d — the fold "+
			"clips the row instead of letting it recede", tightShare, tight, looseShare, loose)
	}
}

// The other half: the fold is a fold, not a global dim. A row wholly inside
// the viewport keeps the ink it always had.
func TestTheOutputRowsInsideTheViewportKeepTheirInk(t *testing.T) {
	a := testApp()
	tt := listOutputShipped(t, a, outputRowsWithActions(12), false)
	tt.Frame()
	tt.Frame()
	box, ok := a.outputRowRects[0]
	if !ok {
		t.Fatalf("the first row has no box: %v", a.outputRowRects)
	}
	// P13-dv：「整行都在视口里」是几何谓词，不是「比例 == 1」。比例在 float32
	// 上会落在 1 的下一格（实测 0.999999769：7.2 与 705.6 相加再相减丢了 1 ulp），
	// 而盒子的四条边是精确的——用它们问，前提才不靠容差带。
	clip := a.outputListRect
	if box.X < clip.X || box.X+box.W > clip.X+clip.W ||
		box.Y < clip.Y || box.Y+box.H > clip.Y+clip.H {
		t.Fatalf("the premise failed: the first row x=%.4f..%.4f y=%.4f..%.4f is not "+
			"wholly inside the viewport x=%.4f..%.4f y=%.4f..%.4f",
			box.X, box.X+box.W, box.Y, box.Y+box.H,
			clip.X, clip.X+clip.W, clip.Y, clip.Y+clip.H)
	}
	// 阈值取 150 而不是「非空」：实测整行可见的行在浅色档上是 228（字 25 对面板
	// 253），而变异 M3（整行也淡）把它压到 114——一个宽阈值会让那条变异静默通过，
	// 这正是本轮 M3 第一次跑时抓到的事（当时阈值是 40，M3 在启动器的 pin 上红了、
	// 在这一页的 pin 上没红）。
	// P13-dv：出货尺下同一行的静息墨实测 **219**（库存尺的旧注释记的是 228——
	// 帧的尺子变了，墨也跟着低 9 级）；150 这条线仍落在「整行」与「整行也淡
	// （114）」之间。
	if ink := contrastIn(tt.Image(), box, a.outputListRect); ink < 150 {
		t.Errorf("a row wholly inside the viewport has faded: contrast %d, want "+
			"the resting ink (the fold only dims what it cuts)", ink)
	} else {
		t.Logf("the first row's resting ink at the shipped ruler: %d", ink)
	}
}

// And the fold is not the footer's: the status strip below the list is pinned
// to the panel, not a row that scrolls under the clip (P13-v's M3 was exactly
// this mistake on the action bar).
func TestTheOutputFooterIsNotFaded(t *testing.T) {
	a := testApp()
	// A page in flight prints the footer's own band under the list.
	tt := runListOutput(t, a, listOutput(25))
	a.output.LoadingMore = true
	tt.Frame()
	tt.Frame()
	if outputFooterState(a.output) != footerLoading {
		t.Fatalf("the premise failed: this output has no footer band")
	}
	copy := StringsFor(a.settings().Language)
	footer, ok := tt.Find(copy.OutputLoading)
	if !ok {
		t.Fatalf("the footer band is not on screen: %q", tt.Texts())
	}
	if ink := contrastIn(tt.Image(), footer, ui.Rect{X: 0, Y: footer.Y, W: InputWindowWidth, H: footer.H}); ink < 20 {
		t.Errorf("the footer's band is faded: contrast %d — the fold reached a "+
			"band that is pinned to the panel, not a row", ink)
	}
}

// P13-w visual evidence: the captured-output page in both appearances, with
// the fold cutting a row. Opt-in for the same reason as the other dumps.
func TestDumpP13WOutputFoldPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tt := ui.NewTester(themedView(a), InputWindowWidth, themedFrameHeight(maxResultSlots))
		tt.SetDark(dark)
		tt.Frame()
		a.Query = "list"
		tt.Frame()
		tt.TypeKey(0, ui.KeyEnter, "")
		tt.Frame()
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-w-outputfold-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
}
