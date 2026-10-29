package terminalui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P11-co · 抢救 stash 的回收件（`9aff92d6db`「floter orphan 2026-10-30 05:37」）。
//
// 这条 stash 的树与 HEAD 逐字节相同（`git diff --quiet 9aff92d6db HEAD` 通过），
// 唯一没被后续轮次覆盖的是它的**未跟踪**载荷：本文件。它是一条测量探针，
// 不是一个已经接上的修复，所以按测量件保留：opt-in、只报数字、不断言。
//
// 它量的是「一次抽屉拖拽往设置文件写了几次」——本仓的设置页有 debounce
// （`settingsui/debounce.go`），抽屉的滚轴走的是另一条路。P11-co 实测：
// **22 帧的一次拖拽产生了 17 次被接受的写**（每次写都会落盘，
// `Store.Update` 成功路径 `store.go`）。这不该被打包成一条「测试」，它是一条
// 待做的轮次，已写进 `.pending-bugs.md` 的「仍待」。
func TestMeasureDrawerDragWrites(t *testing.T) {
	if os.Getenv("FLOTER_MEASURE") == "" {
		t.Skip("set FLOTER_MEASURE=1 to measure a drawer drag's setting writes")
	}
	st, err := settings.OpenStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatalf("the store did not open: %v", err)
	}
	a := New(st, Actions{Close: func() {}, NewCommand: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	a.Drawer = true
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 900, 600)
	tt.Frame()
	readout, ok := tt.Find("14 px")
	if !ok {
		t.Fatal("the drawer's size readout was not drawn")
	}
	y := readout.Y + readout.H/2
	x0, x1 := readout.X-118, readout.X-8
	before := st.Version()
	tt.Press(x0, y)
	tt.Frame()
	frames := 1
	for i := 0; i < 20; i++ {
		x := x0 + float32(i+1)*(x1-x0)/20
		tt.Move(x, y)
		tt.Frame()
		frames++
	}
	tt.Release(x1, y)
	tt.Frame()
	frames++
	t.Logf("the drawer: %d frames of one drag produced %d accepted setting writes",
		frames, st.Version()-before)
}
