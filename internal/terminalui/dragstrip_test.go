package terminalui

import (
	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestTerminalDragStripKeepsTheFirstLineSelectable(t *testing.T) {
	closed := 0
	a := New(newStore(), Actions{Close: func() { closed++ }}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, "First line selection")
	tt := renderThemed(t, a, 860, 600)
	copy := i18n.For("en").Terminal
	canvas, ok := tt.Find(copy.Input)
	if !ok || canvas.Y < terminalDragHeight {
		t.Fatalf("drag strip overlaps grid: %+v", canvas)
	}
	tt.Move(100, 5)
	tt.Frame()
	if tt.Cursor() != ui.CursorGrab {
		t.Fatalf("top strip missing drag cursor: %v", tt.Cursor())
	}
	tt.Press(100, 5)
	tt.Frame()
	if tt.Focused(copy.Input) || a.canvasSelecting {
		t.Fatal("window drag entered terminal selection")
	}
	tt.Release(100, 5)
	tt.Frame()
	// The revealed title passes through to the first row below it.
	tt.Move(70, canvas.Y+8)
	tt.Frame()
	tt.Press(70, canvas.Y+8)
	tt.Frame()
	if !tt.Focused(copy.Input) || !a.canvasSelecting {
		t.Fatal("title swallowed first-line selection")
	}
	closeRect, ok := tt.Find(copy.Close)
	if !ok {
		t.Fatal("close button disappeared")
	}
	tt.Move(closeRect.X+closeRect.W/2, closeRect.Y+closeRect.H/2)
	tt.Frame()
	if !a.canvasSelecting {
		t.Fatal("selection stopped on crossing toolbar")
	}
	tt.Release(closeRect.X+closeRect.W/2, closeRect.Y+closeRect.H/2)
	tt.Frame()
	if a.canvasSelecting || closed != 0 {
		t.Fatal("selection release activated toolbar or stayed latched")
	}
	// Shift lets selection begin under the toolbar's own buttons.
	tt.HoldModifiers(ui.Shift)
	tt.Frame()
	tt.Press(closeRect.X+closeRect.W/2, closeRect.Y+closeRect.H/2)
	tt.Frame()
	if !a.canvasSelecting || closed != 0 {
		t.Fatal("Shift did not yield toolbar to grid")
	}
	tt.Move(-20, -20)
	tt.Frame()
	tt.Release(-20, -20)
	tt.Frame()
	if a.canvasSelecting {
		t.Fatal("release outside window left toolbar hidden")
	}
	tt.HoldModifiers(0)
	tt.Frame()
	tt.Press(70, canvas.Y+8)
	tt.Frame()
	tt.CancelPress()
	tt.Frame()
	if a.canvasSelecting {
		t.Fatal("canceled selection left toolbar hidden")
	}
	// The same button remains usable once selection ends.
	tt.Move(closeRect.X+closeRect.W/2, closeRect.Y+closeRect.H/2)
	tt.Frame()
	tt.ClickAt(closeRect.X+closeRect.W/2, closeRect.Y+closeRect.H/2)
	tt.Frame()
	if closed != 1 {
		t.Fatalf("revealed close action count=%d", closed)
	}
}

func TestDumpP15TerminalInteractions(t *testing.T) {
	dir := os.Getenv("FLOTER_P15_DUMP")
	if dir == "" {
		t.Skip("optional visual evidence")
	}
	for _, appearance := range []string{"light", "dark"} {
		store := newStore()
		store.Preview(func(s *settings.Settings) { s.Theme = appearance })
		a := New(store, Actions{Close: func() {}, Settings: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		withTitle(a, "zsh · ~/Code/floter")
		tt := renderThemed(t, a, 860, 320)
		save := func(state string) {
			t.Helper()
			f, err := os.Create(filepath.Join(dir, "terminal-"+appearance+"-"+state+".png"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
		}
		save("rest")
		tt.Move(430, 5)
		tt.Frame()
		save("drag")
		tt.Move(70, 20)
		tt.Frame()
		tt.Press(70, 20)
		tt.Frame()
		save("selecting")
		tt.Release(70, 20)
	}
}
