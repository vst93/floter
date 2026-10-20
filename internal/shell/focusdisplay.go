package shell

import (
	"os"
	"runtime"

	"github.com/egoist/mygo"
)

// Which display a summon belongs on.
//
// The reference answers this in `focused_monitor` (`main:src-tauri/src/lib.rs:
// 363-398`), and its first strategy is the mouse: a Spotlight-style panel
// appears on the screen the user is looking at, which is the screen the pointer
// is on, not the screen the panel happened to be left on. The port placed every
// summon on the display the *window* was already on (see windowDisplay), so a
// resident panel left on the primary monitor always came back there — a Mac or
// Windows user working on a second screen saw the launcher open across the room
// (`⓪ 三平台体验统一`).
//
// The two later reference strategies are the fallbacks here, in the same order:
// the display the window is on (the reference's *remembered monitor* — the panel
// was last dismissed from where it is, so it is the same answer on Wayland,
// where there is no cursor to read), then the primary display.
//
// Wayland is the awkward platform, again in the reference's words
// (`lib.rs:399-404`): `tao` returns a hardcoded `(0, 0)` there rather than an
// error, so a zero reading has to be taken as "unknown" instead of as the
// top-left corner. The same is true of GDK's pointer position under
// `gtk-layer-shell`-less Wayland, which is what mygo reads
// (`mygo@v0.5.5/internal/linux/system.go:253-261`,
// `gdkDeviceGetPosition`): the reading is silently (0, 0) wherever the
// compositor does not expose global pointer coordinates.

// cursorReadingTrusted reports whether a pointer reading is the mouse's real
// position. Wayland is the one platform that lies: a (0, 0) there is the
// framework's "I do not know" (`main:src-tauri/src/lib.rs:399-404`), and a
// summon placed on the primary monitor's top-left corner is worse than one
// placed where the window already is.
//
// `goos` and `getenv` are parameters so the rule is a unit test rather than a
// live desktop, exactly as `waylandSessionEnv` is (see panelresize.go).
func cursorReadingTrusted(x, y int, goos string, getenv func(string) string) bool {
	if x != 0 || y != 0 {
		return true
	}
	return !waylandSessionEnv(goos, getenv)
}

// displayAt is the display whose bounds contain a point, reporting whether one
// does. It is the pointer's counterpart of displayContaining (which asks about
// a window's centre): the same arithmetic, against a single point.
func displayAt(displays []mygo.Display, x, y int) (mygo.Display, bool) {
	for _, display := range displays {
		bounds := display.Bounds
		if x >= bounds.X && x < bounds.X+bounds.Width &&
			y >= bounds.Y && y < bounds.Y+bounds.Height {
			return display, true
		}
	}
	return mygo.Display{}, false
}

// summonDisplay answers which display a reveal belongs on: the one under the
// mouse where the platform reports it honestly, else the one the window is
// already on, else the primary one.
//
// Every input is a parameter — the displays, the pointer reading, whether that
// reading may be trusted, and the window's own box — so the order of the three
// strategies is a pure function a test can walk, rather than something only a
// multi-monitor desktop could show.
func summonDisplay(displays []mygo.Display, pointer mygo.Point, pointerTrusted bool, windowX, windowY, windowW, windowH int) mygo.Display {
	if len(displays) == 0 {
		return mygo.Display{}
	}
	if pointerTrusted {
		if found, ok := displayAt(displays, pointer.X, pointer.Y); ok {
			return found
		}
	}
	if found, ok := displayContaining(displays, windowX, windowY, windowW, windowH); ok {
		return found
	}
	for _, display := range displays {
		if display.Primary {
			return display
		}
	}
	return displays[0]
}

// pointerPoint is the mouse's own screen position, as mygo reports it, with the
// one platform rule applied: a reading that cannot be trusted comes back with
// ok false rather than as a corner that would misplace the panel.
//
// It is a variable so a test can answer for the desktop, and because the call
// must run on the main thread inside a live application (mygo's
// `Screen.CursorScreenPoint` goes through `onMainValue`).
var pointerPoint = func() (mygo.Point, bool) {
	point := mygo.Screen.CursorScreenPoint()
	return point, cursorReadingTrusted(point.X, point.Y, runtime.GOOS, os.Getenv)
}
