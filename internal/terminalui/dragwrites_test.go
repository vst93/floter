package terminalui

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P11-cp · 一次拖拽 = 一次落盘。
//
// 抽屉的字号与行高两个滑轴以前每帧都走 `Store.Update`，而 `Update` 的成功路径
// **每次都真的写文件**：22 帧的一次拖拽产生 **17 次被接受的写**（P11-co 的测量件
// `FLOTER_MEASURE=1 go test -run TestMeasureDrawerDragWrites` 实测）。参考实现
// 对这两个滑轴本来就有 180ms 窗（`main:src/hooks/useSettings.ts:444-500` 的
// `changeFontSize` / `changeTerminalLineHeight`，抽屉接的就是它们，
// `App.tsx:3925-3926`），所以这是移植漏掉的一处。
//
// 这条 pin 读两个半边，因为只读一个都会被绕过：
//
//   - **值要实时**：拖拽期间 `Store.Version()` 必须一路涨（`Preview` 推进内存并
//     通知监听者，所以终端的字与行高随指针变）——只读写次数的话，「干脆不预览、
//     松手才写」也能绿，而那是另一种产品。
//   - **盘只写一次**：整次拖拽 `Store.Writes()` 恰好 +1，且盘上就是最后那一格。
func TestADrawerDragWritesTheFileOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := settings.OpenStore(path)
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
		t.Fatalf("the drawer's size readout was not drawn: %v", tt.Texts())
	}
	y := readout.Y + readout.H/2
	x0, x1 := readout.X-118, readout.X-8
	startVersion, startWrites := st.Version(), st.Writes()

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

	// The value followed the pointer: a preview advances the store's version, and
	// the frame that drew the drag read that value.
	previews := st.Version() - startVersion
	if previews < 3 {
		t.Fatalf("a %d-frame drag advanced the store %d times: the value did not follow the pointer",
			frames, previews)
	}
	// Nothing has been written yet: the window is still open.
	if got := st.Writes() - startWrites; got != 0 {
		t.Fatalf("the drag wrote the file %d times while its window was open, want 0", got)
	}

	// Let the window run out. The frame past its end is the one that writes, and
	// it writes the whole drag once.
	tt.Advance(2 * settingsDebounce)
	if got := st.Writes() - startWrites; got != 1 {
		t.Fatalf("the drag wrote the file %d times, want 1 (%d frames)", got, frames)
	}

	// The value on disk is the one the drag ended on, not an early tick.
	dragged := st.Snapshot().FontSize
	reloaded, err := settings.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.FontSize != dragged {
		t.Fatalf("the file holds font size %d, the drag ended on %d", reloaded.FontSize, dragged)
	}
	if dragged == settings.Default().FontSize {
		t.Fatalf("the drag never changed the font size (still the shipped %d): the pin proved nothing", dragged)
	}

	// A still drawer writes nothing more: the window closed with the write.
	before := st.Writes()
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if got := st.Writes() - before; got != 0 {
		t.Fatalf("an idle drawer wrote the file %d times, want 0", got)
	}
}

// TestTheDrawerWritesNothingWhileTheDragIsStillMoving is the same rule read the
// other way: the file stays untouched for the whole gesture, and the deadline
// is what ends it — not a frame count, not a release.
func TestTheDrawerWritesNothingWhileTheDragIsStillMoving(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	st, err := settings.OpenStore(path)
	if err != nil {
		t.Fatalf("the store did not open: %v", err)
	}
	a := New(st, Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	a.Drawer = true
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 900, 600)
	tt.Frame()

	readout, ok := tt.Find("14 px")
	if !ok {
		t.Fatalf("the drawer's size readout was not drawn: %v", tt.Texts())
	}
	y := readout.Y + readout.H/2
	x0, x1 := readout.X-118, readout.X-8
	tt.Press(x0, y)
	tt.Frame()
	// Half the window: a drag still in progress has not written anything, and the
	// file does not exist yet at all.
	tt.Advance(settingsDebounce / 2)
	tt.Move(x0+(x1-x0)/2, y)
	tt.Frame()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a drag inside its window created the file: %v", err)
	}
	if got := st.Writes(); got != 0 {
		t.Fatalf("a drag inside its window wrote %d times, want 0", got)
	}
	// Past the window the frame writes once.
	tt.Advance(settingsDebounce)
	tt.Frame()
	if got := st.Writes(); got != 1 {
		t.Fatalf("the frame past the window wrote %d times, want 1", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the flushed drag did not reach the file: %v", err)
	}
}

// TestAPreviewAndAWritePaintTheSameFrame backs the round's "zero pixel" claim:
// the two paths must produce the same frame, or debouncing the write would be a
// visual change in disguise. `Preview` and `Update` run the same clone → mutate
// → normalize → compare chain, so the value the frame reads is byte-identical;
// this pin holds that, and it fails if a preview ever skips normalization.
func TestAPreviewAndAWritePaintTheSameFrame(t *testing.T) {
	dump := func(preview bool) []byte {
		st := settings.NewStore(settings.Default())
		a := New(st, Actions{}, nil)
		withSession(a, &terminal.Terminal{})
		a.Drawer = true
		// One row reads the font size, the other the line height: both are
		// debounced sliders, and both are painted from the store's value.
		apply := func(s *settings.Settings) { s.FontSize = 18; s.TerminalLineHeight = 1.45 }
		if preview {
			st.Preview(apply)
		} else if err := st.Update(apply); err != nil {
			t.Fatalf("the write failed: %v", err)
		}
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 900, 600)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		var buf bytes.Buffer
		if err := png.Encode(&buf, tt.Image()); err != nil {
			t.Fatalf("the frame did not encode: %v", err)
		}
		return buf.Bytes()
	}
	a, b := dump(true), dump(false)
	if len(a) == 0 || len(b) == 0 {
		t.Fatalf("one of the frames was not exported (%d and %d bytes)", len(a), len(b))
	}
	if string(a) != string(b) {
		t.Fatalf("a previewed frame differs from a written one (%d vs %d bytes): "+
			"the debounce changed a pixel", len(a), len(b))
	}
	// The control: the same dump with the value left alone must NOT match, or
	// both arms above drew the shipped frame and the comparison proved nothing.
	untouched := func() []byte {
		st := settings.NewStore(settings.Default())
		app := New(st, Actions{}, nil)
		withSession(app, &terminal.Terminal{})
		app.Drawer = true
		f := ui.NewTester(func(c *ui.Context) { app.View(c) }, 900, 600)
		f.SetPreferences(ui.Preferences{ReduceMotion: true})
		f.Frame()
		var buf bytes.Buffer
		if err := png.Encode(&buf, f.Image()); err != nil {
			t.Fatalf("the control frame did not encode: %v", err)
		}
		return buf.Bytes()
	}()
	if string(untouched) == string(a) {
		t.Fatalf("the dragged frame equals the untouched one: neither arm drew font size 18, "+
			"so the comparison above proved nothing (%d bytes both)", len(a))
	}
}
