package terminalui

import (
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/control"
	"floter/internal/settings"
)

// drawerStore is a settings store holding the shipped appearance.
func drawerStore(t *testing.T) *settings.Store {
	t.Helper()
	return settings.NewStore(settings.Default())
}

// drawerApp is a surface with one session, no library needed: the drawer is
// drawn from the settings and the canvas, not from a live terminal.
func drawerApp(t *testing.T) *App {
	t.Helper()
	a := New(drawerStore(t), Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	withSession(a, &terminal.Terminal{})
	a.Drawer = true
	return a
}

// TestTerminalDrawerShowsTheAppearanceRows: the drawer is the terminal's own
// appearance strip, docked at the panel's bottom edge — every axis this build
// can actually apply is on it, and the strip fits inside the window.
func TestTerminalDrawerShowsTheAppearanceRows(t *testing.T) {
	a := drawerApp(t)
	tt := render(t, a, 860, 600)
	tt.Frame()

	// The drawer is the surface that carries the *full* schema
	// (internal/control): every axis the schema names must have a row here.
	// Reading the list from the schema is what keeps the view and this test
	// from agreeing on a stale set.
	labels := map[control.AppearanceAxis]string{
		control.AxisFontFamily:  "Font family",
		control.AxisFontSize:    "Font size",
		control.AxisLineHeight:  "Line height",
		control.AxisPadding:     "Padding",
		control.AxisCursorShape: "Cursor shape",
		control.AxisCursorBlink: "Blinking cursor",
		control.AxisPalette:     "Terminal palette",
		control.AxisPasteSafe:   "Safe paste",
	}
	if len(labels) != len(control.AppearanceAxes()) {
		t.Fatalf("the test names %d axes, the schema has %d", len(labels), len(control.AppearanceAxes()))
	}
	if !tt.HasText("Terminal appearance") {
		t.Errorf("the drawer has no group title: %q", tt.Texts())
	}
	for _, axis := range control.AppearanceAxes() {
		label, ok := labels[axis]
		if !ok {
			t.Fatalf("the drawer owes the schema an axis with no row: %q", axis)
		}
		if !tt.HasText(label) {
			t.Errorf("the drawer is missing %q: %q", label, tt.Texts())
		}
	}
	// The sliders' live readouts, which the reference keeps beside the track
	// rather than at the end of the title line.
	for _, readout := range []string{"14 px", "1.20\u00d7"} {
		if !tt.HasText(readout) {
			t.Errorf("the drawer is missing the readout %q: %q", readout, tt.Texts())
		}
	}
	// The drawer's own dismiss, named by the same key the bar's X uses.
	if _, ok := tt.Find("Close terminal settings"); !ok {
		t.Errorf("the drawer has no close control: %q", tt.Texts())
	}

	// Every row is inside the window: the strip is docked, never clipped.
	for _, label := range []string{"Font size", "Cursor shape", "Terminal palette"} {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q is missing", label)
		}
		if rect.Y < 0 || rect.Y+rect.H > 600 {
			t.Errorf("%q sits at %v, outside the 600px window", label, rect)
		}
	}
}

// TestTerminalDrawerWritesEachSetting: every control writes the setting it
// draws, through the store the shell listens to — so the running session takes
// the change without the surface ever being left.
func TestTerminalDrawerWritesEachSetting(t *testing.T) {
	a := drawerApp(t)
	tt := render(t, a, 860, 600)
	tt.Frame()

	if err := tt.Click("Block"); err != nil {
		t.Fatalf("cursor shape: %v", err)
	}
	tt.Frame()
	if got := a.Store.Snapshot().CursorShape; got != "block" {
		t.Errorf("cursor shape = %q, want block", got)
	}

	if err := tt.Click("Relaxed"); err != nil {
		t.Fatalf("padding: %v", err)
	}
	tt.Frame()
	if got := a.Store.Snapshot().TerminalPadding; got != "relaxed" {
		t.Errorf("padding = %q, want relaxed", got)
	}

	if err := tt.Click("Blinking cursor"); err != nil {
		t.Fatalf("blink: %v", err)
	}
	tt.Frame()
	if a.Store.Snapshot().CursorBlink {
		t.Error("the blink switch did not turn off")
	}

	// The palette trigger shows the stored value; the drop-down picks another.
	if err := tt.Click("Follow app"); err != nil {
		t.Fatalf("palette trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Forest"); err != nil {
		t.Fatalf("palette option: %v", err)
	}
	tt.Frame()
	if got := a.Store.Snapshot().TerminalTheme; got != "forest" {
		t.Errorf("palette = %q, want forest", got)
	}
}

// TestTerminalDrawerClosesFromItself: the control that opens the drawer is not
// the only way out of it — the drawer carries its own close, and the gear in
// the bar becomes the same X while it is up (the reference's R41/R54 idiom).
func TestTerminalDrawerClosesFromItself(t *testing.T) {
	store := drawerStore(t)
	a := New(store, Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	a.Actions.Settings = a.ToggleDrawer
	a.Drawer = true
	tt := render(t, a, 860, 600)
	tt.Frame()

	// Both entries wear the same name while the drawer is open: the bar's
	// gear flipped to an X, and the drawer's own corner control.
	if got := strings.Count(strings.Join(tt.Texts(), "\n"), "Close terminal settings"); got != 2 {
		t.Errorf("the open drawer names its close control %d times, want 2: %q", got, tt.Texts())
	}
	if err := tt.Click("Close terminal settings"); err != nil {
		t.Fatalf("the drawer's close: %v", err)
	}
	tt.Frame()
	if a.Drawer {
		t.Error("the drawer's own close did not close it")
	}
	if tt.HasText("Terminal appearance") {
		t.Errorf("the closed drawer still draws its rows: %q", tt.Texts())
	}
}

// TestTerminalDrawerGearToggles: the bar's gear flips the drawer, and its own
// name follows the state.
func TestTerminalDrawerGearToggles(t *testing.T) {
	store := drawerStore(t)
	a := New(store, Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	a.Actions.Settings = a.ToggleDrawer
	tt := render(t, a, 860, 600)
	tt.Frame()
	if !tt.HasText("Terminal settings") {
		t.Fatalf("the closed gear is not named: %q", tt.Texts())
	}

	if err := tt.Click("Terminal settings"); err != nil {
		t.Fatalf("the gear: %v", err)
	}
	tt.Frame()
	if !a.Drawer {
		t.Fatal("the gear did not open the drawer")
	}
	if !tt.HasText("Close terminal settings") || tt.HasText("Terminal settings") {
		t.Errorf("the gear did not flip to its close name: %q", tt.Texts())
	}

	if err := tt.Click("Close terminal settings"); err != nil {
		t.Fatalf("the flipped gear: %v", err)
	}
	tt.Frame()
	if a.Drawer {
		t.Error("the flipped gear did not close the drawer")
	}
}

// TestTerminalDrawerLeavesTheCanvas: the drawer is a sibling of the canvas,
// never a layer over it — opening it does not take the session off the page,
// and the strip is drawn below the canvas's own top edge.
func TestTerminalDrawerLeavesTheCanvas(t *testing.T) {
	a := New(drawerStore(t), Actions{Close: func() {}}, nil)
	tt := render(t, a, 860, 600)
	tt.Frame()
	if !tt.HasText("New blank session") {
		t.Fatalf("the empty state is missing: %q", tt.Texts())
	}
	before, ok := tt.Find("New blank session")
	if !ok {
		t.Fatal("the empty state's control is missing")
	}

	a.Drawer = true
	tt.Frame()
	if !tt.HasText("New blank session") {
		t.Errorf("opening the drawer took the page away: %q", tt.Texts())
	}
	// The empty state reflows into the room the strip leaves rather than
	// sliding under it.
	after, ok := tt.Find("New blank session")
	if !ok {
		t.Fatal("the empty state's control vanished with the drawer open")
	}
	strip, ok := tt.Find("Terminal appearance")
	if !ok {
		t.Fatal("the strip is not on the page")
	}
	if strip.Y < before.Y && strip.Y < after.Y {
		t.Errorf("the strip at %v is not below the page it docks under", strip)
	}
	if after.Y+after.H > strip.Y+strip.H {
		t.Errorf("the page at %v is not held above the strip at %v", after, strip)
	}
}

// TestTerminalDrawerNeedsTheLanguage: the rows are the settings copy, so the
// drawer follows the interface language like every other surface.
func TestTerminalDrawerNeedsTheLanguage(t *testing.T) {
	a := drawerApp(t)
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	tt := render(t, a, 860, 600)
	tt.Frame()
	if !tt.HasText("字号") || !tt.HasText("关闭终端设置") {
		t.Errorf("the drawer did not follow the language: %q", tt.Texts())
	}
}
