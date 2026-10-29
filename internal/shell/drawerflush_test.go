package shell

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/terminalui"
)

// P11-cp · 拖到一半就离开终端页，那一格必须落盘。
//
// 抽屉的字号/行高滑轴现在走 180ms 窗（与设置页同一条参考规则），窗是靠**帧**到点
// 关的（`deadline.Wake`）。而离开终端表面、隐藏窗口、退出这三处都会让帧停下来：
// 隐藏的窗口不再画，`deadline` 的唤醒永远不到，那次拖拽就**静默丢了**——用户松手
// 前换了页面，字号变了一格、盘上还是旧值。
//
// 修前红：`shell.Open` 只 flush 设置页（`shell.go` 的 `a.Settings.Flush()`），
// 终端那一半没有 flush，所以这条 pin 在旧代码上报「盘上还是 14」。
func TestLeavingTheTerminalSurfaceWritesTheDraggedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatalf("the store did not open: %v", err)
	}
	a := New(Options{Store: store})
	// A session the drawer can be docked to: a terminal the plugin never started
	// is a marker, as the sessions list's own fixtures are.
	a.Terminal.Sessions = []*terminalui.Session{{Title: "zsh", Term: &terminal.Terminal{}, Created: time.Now()}}
	a.Terminal.Active = 0
	a.Terminal.Drawer = true
	a.Surf = SurfaceTerminal

	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 900, 600)
	tt.Frame()
	readout, ok := tt.Find("14 px")
	if !ok {
		t.Fatalf("the drawer's size readout was not drawn: %v", tt.Texts())
	}
	// The track's live span in the shell's own frame: measured, the slider answers
	// the pointer from 100u left of the readout to 20u (the standalone drawer's
	// wider cells put its track further out, so this is not the same pair).
	y := readout.Y + readout.H/2
	x0, x1 := readout.X-100, readout.X-20

	// Drag, but do not wait for the window: the gesture is still in flight.
	tt.Press(x0, y)
	tt.Frame()
	tt.Move(x0+(x1-x0)/2, y)
	tt.Frame()
	tt.Release(x0+(x1-x0)/2, y)
	tt.Frame()
	if got := store.Writes(); got != 0 {
		t.Fatalf("the drag wrote %d times before its window closed, want 0", got)
	}
	dragged := store.Snapshot().FontSize
	if dragged == settings.Default().FontSize {
		t.Fatalf("the drag never moved the font size off the shipped %d", dragged)
	}

	// Leave the surface. The frame-driven deadline can no longer arrive, so the
	// flush on the way out is the only thing that writes this value.
	a.Open(SurfaceLauncher)

	if got := store.Writes(); got != 1 {
		t.Fatalf("leaving the terminal wrote %d times, want 1", got)
	}
	reloaded, err := settings.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.FontSize != dragged {
		t.Fatalf("the file holds font size %d, the drag ended on %d: the write was lost",
			reloaded.FontSize, dragged)
	}
}

// The same rule for the window hiding: a hidden window draws no frames, so its
// deadline would never arrive either.
func TestHidingTheWindowWritesTheDraggedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatalf("the store did not open: %v", err)
	}
	a := New(Options{Store: store})
	a.Terminal.Sessions = []*terminalui.Session{{Title: "zsh", Term: &terminal.Terminal{}, Created: time.Now()}}
	a.Terminal.Active = 0
	a.Terminal.Drawer = true
	a.Surf = SurfaceTerminal
	a.hideWindow = func() {}

	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 900, 600)
	tt.Frame()
	readout, ok := tt.Find("14 px")
	if !ok {
		t.Fatalf("the drawer's size readout was not drawn: %v", tt.Texts())
	}
	y := readout.Y + readout.H/2
	x0, x1 := readout.X-100, readout.X-20
	tt.Press(x0, y)
	tt.Frame()
	tt.Move(x0+(x1-x0)/2, y)
	tt.Frame()
	tt.Release(x0+(x1-x0)/2, y)
	tt.Frame()
	dragged := store.Snapshot().FontSize
	if dragged == settings.Default().FontSize {
		t.Fatalf("the drag never moved the font size off the shipped %d", dragged)
	}

	a.Hide()

	reloaded, err := settings.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.FontSize != dragged {
		t.Fatalf("hiding left the file at %d, the drag ended on %d: the write was lost",
			reloaded.FontSize, dragged)
	}
}
