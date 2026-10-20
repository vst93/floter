package shell

import (
	"testing"

	"github.com/egoist/mygo"
)

// The two screens the tests walk: the primary one on the left, a wider second
// one to its right.
func summonTestDisplays() []mygo.Display {
	primary := mygo.Display{
		ID: 1, Primary: true,
		Bounds: mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080},
	}
	second := mygo.Display{
		ID:     2,
		Bounds: mygo.Rectangle{X: 1920, Y: 0, Width: 2560, Height: 1440},
	}
	return []mygo.Display{primary, second}
}

// The reference's own first strategy: the mouse decides. A panel summoned while
// the user works on the second screen opens there, not on the screen it was
// last left on — the port's whole bug (「三平台体验统一」, and
// `main:src-tauri/src/lib.rs:363-371`).
func TestThePointerPicksTheSummonDisplay(t *testing.T) {
	displays := summonTestDisplays()
	// The window says "first display"; the pointer says "second".
	got := summonDisplay(displays, mygo.Point{X: 2400, Y: 300}, true, 100, 100, 720, 400)
	if got.ID != 2 {
		t.Errorf("with the pointer on the second display the summon landed on %d, want 2", got.ID)
	}
	// And the other way round.
	got = summonDisplay(displays, mygo.Point{X: 300, Y: 300}, true, 2300, 300, 720, 400)
	if got.ID != 1 {
		t.Errorf("with the pointer on the first display the summon landed on %d, want 1", got.ID)
	}
}

// An untrusted reading is the reference's second strategy: the display the
// window is on (its *remembered monitor*), and the primary one only when the
// window is on neither.
func TestAnUntrustedPointerFallsBackToTheWindow(t *testing.T) {
	displays := summonTestDisplays()
	got := summonDisplay(displays, mygo.Point{}, false, 2300, 300, 720, 400)
	if got.ID != 2 {
		t.Errorf("an untrusted pointer over the second display landed on %d, want the window's 2", got.ID)
	}
	// A pointer reading that lands outside every display is the same case: a
	// stale coordinate must not place the panel nowhere.
	got = summonDisplay(displays, mygo.Point{X: -9000, Y: -9000}, true, 2300, 300, 720, 400)
	if got.ID != 2 {
		t.Errorf("a pointer off every display landed on %d, want the window's 2", got.ID)
	}
	// Nowhere at all: the primary display.
	got = summonDisplay(displays, mygo.Point{X: -9000, Y: -9000}, true, -9000, -9000, 720, 400)
	if got.ID != 1 {
		t.Errorf("a window on no display landed on %d, want the primary 1", got.ID)
	}
	// A machine that reports no displays answers an empty one rather than
	// indexing an empty slice.
	if got := summonDisplay(nil, mygo.Point{X: 10, Y: 10}, true, 0, 0, 720, 400); got.ID != 0 {
		t.Errorf("no displays answered %+v, want the zero display", got)
	}
}

// The pointer's containment rule is the window's own, against one point: the
// edges belong to the display they open.
func TestThePointerBelongsToOneDisplay(t *testing.T) {
	displays := summonTestDisplays()
	if got, ok := displayAt(displays, 1920, 0); !ok || got.ID != 2 {
		t.Errorf("the seam point resolved to %+v (%v), want the second display", got, ok)
	}
	if got, ok := displayAt(displays, 1919, 1079); !ok || got.ID != 1 {
		t.Errorf("the last point of the first display resolved to %+v (%v), want it", got, ok)
	}
	if _, ok := displayAt(displays, 0, 1080); ok {
		t.Error("a point below every display was claimed")
	}
}

// Wayland's (0, 0) is the framework's "I do not know", not the top-left corner
// (`main:src-tauri/src/lib.rs:399-404`): the reading is taken as unknown there
// and trusted everywhere else, including on X11, where (0, 0) really is a mouse
// in the corner.
func TestAZeroReadingIsUnknownOnlyOnWayland(t *testing.T) {
	wayland := func(key string) string {
		switch key {
		case "WAYLAND_DISPLAY":
			return "wayland-0"
		}
		return ""
	}
	sessionType := func(key string) string {
		if key == "XDG_SESSION_TYPE" {
			return "wayland"
		}
		return ""
	}
	none := func(string) string { return "" }

	cases := []struct {
		name    string
		x, y    int
		goos    string
		getenv  func(string) string
		trusted bool
	}{
		{"a real pointer is trusted on wayland", 640, 480, "linux", wayland, true},
		{"the zero is unknown on wayland", 0, 0, "linux", wayland, false},
		{"the zero is unknown via the session type", 0, 0, "linux", sessionType, false},
		{"x11's corner is a real pointer", 0, 0, "linux", none, true},
		{"windows can be in a corner", 0, 0, "windows", none, true},
		{"macos can be in a corner", 0, 0, "darwin", none, true},
		{"a wayland session on another os is not one", 0, 0, "darwin", wayland, true},
	}
	for _, tc := range cases {
		if got := cursorReadingTrusted(tc.x, tc.y, tc.goos, tc.getenv); got != tc.trusted {
			t.Errorf("%s: trusted = %v, want %v", tc.name, got, tc.trusted)
		}
	}
}
