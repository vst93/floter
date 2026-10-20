package shell

import (
	"testing"
	"time"

	"floter/internal/launcher"

	"github.com/egoist/mygo/ui"
)

// frameAsks records which frame each resize request came from, so "one ask per
// frame" is a thing the test can assert: a settle draws several frames (the
// walk asks for the next), and counting calls alone cannot tell a frame that
// coalesced two rules from two frames that each had one.
type frameAsks struct {
	frame int
	at    []int
	asks  [][2]int
}

func (f *frameAsks) resize(width, height int) {
	f.asks = append(f.asks, [2]int{width, height})
	f.at = append(f.at, f.frame)
}

// reset forgets the asks recorded so far, keeping the frame counter: the
// counter must never restart, or a later frame would be mistaken for an earlier
// one and the "twice in a frame" check would read two phases as one.
func (f *frameAsks) reset() { f.at, f.asks = nil, nil }

// once fails if any frame asked for more than one size — the second
// synchronous SetWindowPos the coalescing exists to remove. A phase that asked
// nothing at all passes: "the frame asked for nothing" is a fact its own
// assertion covers, and for a settled launcher it is the wanted answer.
func (f *frameAsks) once(t *testing.T) {
	t.Helper()
	seen := make(map[int]bool, len(f.at))
	for _, frame := range f.at {
		if seen[frame] {
			t.Errorf("frame %d asked for two sizes (%v): every SetSize is a synchronous SetWindowPos",
				frame, f.asks)
		}
		seen[frame] = true
	}
}

// countedFrame is the launcher's View with a frame counter: one settle can draw
// more than one frame (an animation asks for the next), and the rule this file
// pins is *per frame*, so the assertions have to know how many they got.
func countedFrame(a *App, f *frameAsks) *ui.Tester {
	a.setWindowSize = f.resize
	return ui.NewTester(func(c *ui.Context) {
		f.frame++
		a.View(c)
	}, 720, 420)
}

// The frame's window size is one call, however many rules have an opinion.
//
// One frame can want the window at two sizes: the height walk steps toward the
// list's target every frame, and the same frame can conclude that the window came
// back wider than the 720-wide contract (holdContractWidth). Each used to be its
// own SetSize, and on Windows one SetSize is one *synchronous* SetWindowPos — two
// of them between a frame's layout and its paint is a round trip to the window
// manager the user pays per keystroke, which is the shape of the 「很多界面经常
// 卡死」 reports. The rule is pinned at both ends here: the collector's own
// semantics, and the frame that used to make the second call.
func TestOneFrameAsksForOneWindowSize(t *testing.T) {
	var r sizeRequest
	if _, _, ok := r.take(); ok {
		t.Fatal("an empty frame asked for a size")
	}
	r.ask(720, 300)
	r.ask(720, 340) // the newest ask wins: they describe one box
	if width, height, ok := r.take(); !ok || width != 720 || height != 340 {
		t.Fatalf("the frame's size = %dx%d (ok %v), want the last ask 720x340", width, height, ok)
	}
	if _, _, ok := r.take(); ok {
		t.Error("the frame's size was handed out twice: that is the second SetSize")
	}
	// A side that is not a size is not asked for: a surface that has not been
	// measured reports 0, and asking the platform for it would be a resize to
	// nothing.
	r.ask(0, 340)
	r.ask(720, 0)
	r.ask(-1, 340)
	if _, _, ok := r.take(); ok {
		t.Error("a zero or negative side was asked for")
	}
}

// The frame that used to make two synchronous resizes now makes one.
//
// A launcher frame that is both mid-walk (its height is still travelling to the
// list's target) and wider than the contract is exactly the frame that made two
// calls. Typing is what starts a walk in the shipped app — every keystroke can
// change the list's height — so that is the press this test walks.
func TestTheLauncherFrameResizesTheWindowOnce(t *testing.T) {
	a := appKeysApp()
	a.windowVisible = func() bool { return true }
	asks := &frameAsks{}
	tt := countedFrame(a, asks)
	tt.Frame()
	// A client wider than the 720 contract, so the pull has an opinion too.
	tt.SetSize(1000, 420)
	a.contractAt, a.contractTries = time.Time{}, 0
	asks.reset()

	tt.Type("term") // fewer rows: the walk has somewhere to go
	tt.Advance(40 * time.Millisecond)
	asks.once(t)
	if len(asks.asks) == 0 {
		t.Fatalf("a keystroke that changed the list asked the window for nothing")
	}
	for _, ask := range asks.asks {
		if ask[0] != launcher.InputWindowWidth {
			t.Errorf("a walking frame asked for width %d, want the contract's %d", ask[0], launcher.InputWindowWidth)
		}
		if ask[1] <= 0 {
			t.Errorf("a walking frame asked for height %d", ask[1])
		}
	}
	// A frame that has landed and sits inside the contract asks for nothing:
	// a launcher sitting still costs the window server nothing.
	tt.Advance(500 * time.Millisecond)
	tt.SetSize(launcher.InputWindowWidth, 420)
	asks.reset()
	tt.Frame()
	tt.Frame()
	if len(asks.asks) != 0 {
		t.Errorf("a settled launcher asked the window for %v", asks.asks)
	}
}

// And the pull's own ask is that frame's single call, not a second one.
func TestTheContractPullJoinsTheFrameSize(t *testing.T) {
	a := appKeysApp()
	a.windowVisible = func() bool { return true }
	asks := &frameAsks{}
	tt := countedFrame(a, asks)
	tt.Frame()
	// Wider than the contract with the walk landed: this is the pull's frame,
	// and the one that used to make a second synchronous resize beside it.
	tt.SetSize(1000, 420)
	a.lastH = a.launcherTarget
	a.contractAt, a.contractTries = time.Time{}, 0
	asks.reset()
	tt.Frame()
	asks.once(t)
	if len(asks.asks) == 0 {
		t.Fatal("the contract pull asked for no size on a window wider than 720")
	}
	if width := asks.asks[len(asks.asks)-1][0]; width != launcher.InputWindowWidth {
		t.Errorf("the pull asked for width %d, want the contract's %d", width, launcher.InputWindowWidth)
	}
	if a.lastW != launcher.InputWindowWidth || a.lastH != a.launcherTarget {
		t.Errorf("the app's own record is %dx%d after the pull, want the contract's %dx%d",
			a.lastW, a.lastH, launcher.InputWindowWidth, a.launcherTarget)
	}
	if a.contractTries == 0 {
		t.Error("the pull spent none of its budget on a window wider than the contract")
	}
}

// The landing frame is the one that really made two calls.
//
// The walk asks for the target and *records* it as `lastH`, so on that same frame
// `holdContractWidth`'s "a walking frame is left alone" guard no longer holds —
// the pull runs, in the same frame, and asks for the same window at the same
// height with the contract's width. That is the one frame where the two rules
// genuinely collide, and the one a user pays on every keystroke that changes the
// list while the platform has handed the window a client wider than the design's
// column. The frame is built by hand here (a view that runs both rules) because
// the tester's clock settles an animation to its target: that *is* the landing
// frame, which is exactly what this pins.
func TestTheLandingFrameAsksForOneSize(t *testing.T) {
	a := appKeysApp()
	asks := &frameAsks{}
	a.setWindowSize = asks.resize
	a.windowVisible = func() bool { return true }
	tt := ui.NewTester(func(c *ui.Context) {
		asks.frame++
		a.walkLauncherHeight(c)
		a.holdContractWidth(float32(launcher.InputWindowWidth) + 280)
		a.flushWindowSize()
	}, 1000, 420)
	// A walk one step from its target: this frame's step lands it.
	a.lastH, a.lastW = 300, launcher.InputWindowWidth
	a.launcherTarget = 304
	tt.Frame()
	if len(asks.asks) == 0 {
		t.Fatal("the landing frame asked for no size at all")
	}
	asks.once(t)
	width, height := asks.asks[len(asks.asks)-1][0], asks.asks[len(asks.asks)-1][1]
	if width != launcher.InputWindowWidth || height != 304 {
		t.Errorf("the landing frame asked for %dx%d, want the contract's 720x304", width, height)
	}
}

// A whole walk, on a window wider than the contract, never asks twice in one
// frame.
//
// This is the path a user pays per keystroke: every press that changes the list's
// height starts a walk, and every frame of it could also want the pull, so a
// frame with two calls is the queue that reads as a freeze. The asks are recorded
// per frame, and the walk is stepped by the tester's clock so the animation really
// travels.
func TestAWalkNeverAsksTwiceInOneFrame(t *testing.T) {
	a := appKeysApp()
	a.windowVisible = func() bool { return true }
	asks := &frameAsks{}
	tt := countedFrame(a, asks)
	tt.Frame()
	tt.SetSize(1000, 420) // wider than the 720 contract, all the way through
	a.contractAt, a.contractTries = time.Time{}, 0
	start := a.lastH

	asks.reset()
	tt.Type("s") // one more row: 293 -> 334, a walk upward
	tt.Frame()
	for i := 0; i < 4; i++ {
		tt.Advance(40 * time.Millisecond)
	}
	asks.once(t)
	if len(asks.asks) == 0 {
		t.Fatal("the walk asked for no height at all")
	}
	stepped := false
	for _, ask := range asks.asks {
		if ask[0] != launcher.InputWindowWidth {
			t.Errorf("a walking frame asked for width %d, want the contract's %d", ask[0], launcher.InputWindowWidth)
		}
		if ask[1] > start && ask[1] < a.launcherTarget {
			stepped = true
		}
	}
	if !stepped {
		t.Errorf("no ask was a step of the walk (%v, from %d to %d)", asks.asks, start, a.launcherTarget)
	}
	// Landing: the last frames ask the target and then stop. A window already at
	// its size costs the window server nothing, and no frame pays twice.
	asks.reset()
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	tt.Frame()
	asks.once(t)
	if a.lastH != a.launcherTarget {
		t.Errorf("the walk landed at %d, want the target %d", a.lastH, a.launcherTarget)
	}
	if got := len(asks.asks); got > 3 {
		t.Errorf("a settled launcher asked for %d sizes after landing: %v", got, asks.asks)
	}
}
