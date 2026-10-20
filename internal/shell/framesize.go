package shell

// The window size a frame asks for, coalesced.
//
// One frame can want the window at two different sizes: the height walk
// (`walkLauncherHeight`) steps toward the list's target every frame, and the
// same frame can also conclude that the window came back wider than the design's
// 720-wide contract (`holdContractWidth`). Each of those used to be its own
// `SetSize`, and on Windows one `SetSize` is one *synchronous* `SetWindowPos`:
// two in a frame is a round trip to the window manager between the frame's
// layout and its paint. A user pressing a terminal shortcut repeatedly pays that
// per press, and a platform that is slow to answer reads as a freeze
// (「很多界面经常卡死」).
//
// Both are asking the *same* window for a size in the same frame, so the frame
// is entitled to make one call: the last ask wins (the walk's height and the
// pull's width describe one box, not two), and the platform sees one
// `SetWindowPos` per frame however many of the app's own rules had an opinion.
//
// The type is separate from App so the rule is a unit test: a frame's asks
// collapse, the newest wins, and taking the request empties it — so a frame that
// asked nothing calls the platform not at all.
type sizeRequest struct {
	width, height int
	pending       bool
}

// hasWindow reports whether there is a window for a frame's rules to size: the
// real one, or a test's stand-in for its resize (see setWindowSize). Every
// per-frame sizing rule reads this rather than `Win` alone, so a headless test
// can walk the same path a live app does.
func (a *App) hasWindow() bool { return a.Win != nil || a.setWindowSize != nil }

// ask records a size for this frame. A non-positive side is not a size (0 is
// how a surface that has not been measured reports itself), so it is ignored
// rather than asked for.
func (r *sizeRequest) ask(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	r.width, r.height, r.pending = width, height, true
}

// take hands out the frame's size, once.
func (r *sizeRequest) take() (width, height int, ok bool) {
	if !r.pending {
		return 0, 0, false
	}
	width, height = r.width, r.height
	*r = sizeRequest{}
	return width, height, true
}

// requestWindowSize records a size for the frame's single platform call.
//
// lastW and lastH are the numbers the app reasons about — the height walk's
// steady-state check and `resize`'s "already there" test read them — so they are
// updated here, at the moment the decision is made, not when the call lands: a
// second ask in the same frame must see the first one's height.
func (a *App) requestWindowSize(width, height int) {
	if !a.hasWindow() || width <= 0 || height <= 0 {
		return
	}
	a.lastW, a.lastH = width, height
	a.frameSize.ask(width, height)
}

// flushWindowSize applies the frame's size, once, at the end of the frame —
// after the layout that decided it, so the platform's synchronous resize is not
// interleaved with the paint it would invalidate.
func (a *App) flushWindowSize() {
	width, height, ok := a.frameSize.take()
	if !ok {
		return
	}
	if a.setWindowSize != nil {
		a.setWindowSize(width, height)
		return
	}
	if a.Win != nil {
		a.Win.SetSize(width, height)
	}
}
