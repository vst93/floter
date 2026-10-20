package shell

import (
	"testing"
	"time"
)

// The panel arrives the way the reference's `shell-enter` says it does.
//
// `main:src/styles/base.css:1244-1256` animates every surface shell on mount —
// `animation: shell-enter var(--dur-4) var(--spring)` over the 220ms `--dur-4`
// (`:349`) with `from { opacity: 0; transform: scale(0.97) translateY(6px) }`
// and `transform-origin: center top`. The port had no arrival at all on any of
// the three platforms: a summon swapped the window's contents into place, which
// is the part of 「界面效果」 the direction of 2026-10-18 puts ahead of pixel
// detail.
//
// The travel is read through the frame itself, at the three instants that
// matter: the settled frame, the reveal's first frame, and the frame the 220ms
// lands on.
func TestThePanelLandsInsteadOfAppearing(t *testing.T) {
	a := appKeysApp()
	// A reveal needs a window to happen in; there is none in a headless test,
	// so the seam stands in for one and the clock is the app's own (which is
	// what lets a test stand inside the 220ms).
	a.arrivalSeen = func() bool { return true }
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	tt := appKeysRender(t, a)

	// Settled: the frame the rest of the suite already pins.
	a.enterLive = false
	tt.Frame()
	settled := bandImage(tt)
	bounds := settled.Bounds()

	// A reveal starts the travel, and its first frame is *not* the settled one:
	// 0 of the way through, the pane is at 97% and lifted 6px.
	a.startShellEnter()
	if !a.enterLive {
		t.Fatalf("the reveal started no arrival")
	}
	tt.Frame()
	start := bandImage(tt)
	if start.Bounds() != bounds {
		t.Fatalf("the arrival changed the frame's own size: %v vs %v", start.Bounds(), bounds)
	}
	if bandDiff(start, settled, 0, 0, bounds.Dx(), bounds.Dy()) == 0 {
		t.Fatalf("the frame a reveal draws is identical to the settled one: the panel appeared instead of arriving")
	}

	// Halfway through, the travel has moved on from its start but has not
	// landed.
	now = now.Add(shellEnter / 2)
	tt.Frame()
	middle := bandImage(tt)
	if bandDiff(middle, start, 0, 0, bounds.Dx(), bounds.Dy()) == 0 {
		t.Errorf("the frame is still at the arrival's first step halfway through it")
	}

	// The 220ms lands exactly on the settled frame, and the arrival stops
	// asking for animation frames.
	now = now.Add(shellEnter / 2)
	tt.Frame()
	if diff := bandDiff(bandImage(tt), settled, 0, 0, bounds.Dx(), bounds.Dy()); diff != 0 {
		t.Errorf("the arrival did not land on the settled frame: %d pixels differ", diff)
	}
	if a.enterLive {
		t.Errorf("the arrival is still live after its 220ms")
	}
}

// The arrival is a nicety of a reveal, never a condition of a frame: a panel
// with no window never starts one, and a panel that never arrived reports
// itself settled.
//
// This is what keeps the arrival from moving any other assertion: the card's
// edges, the search column's units, the wide-window and held-row checks all read
// `View` on an app with no window, and a travel applied there would have shifted
// every one of them.
func TestOnlyAWindowArrives(t *testing.T) {
	a := appKeysApp()
	a.enterLive = false
	a.startShellEnter()
	if a.enterLive {
		t.Errorf("a panel with no window started an arrival")
	}
	if got := a.shellEnterProgress(); got != 1 {
		t.Errorf("a panel that never arrived reports progress %v, want 1 (settled)", got)
	}

	// The reference's own numbers, so the shape cannot drift from the design:
	// `--dur-4` is 220ms (`base.css:349`).
	if shellEnter != 220*time.Millisecond {
		t.Errorf("the arrival lasts %v, want the reference's --dur-4 (220ms)", shellEnter)
	}

	// A window that is there, and a reveal: the travel runs from 0 and stops
	// asking after the 220ms.
	a.arrivalSeen = func() bool { return true }
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	// Fresh state: the previous start was refused, so nothing is live.
	a.startShellEnter()
	if !a.enterLive {
		t.Fatalf("a reveal onto a window started no arrival")
	}
	if got := a.shellEnterProgress(); got > 0.001 {
		t.Errorf("the arrival's first frame is at %v, want 0", got)
	}
	now = now.Add(shellEnter)
	if got := a.shellEnterProgress(); got != 1 {
		t.Errorf("the arrival is at %v after its 220ms, want 1", got)
	}
	if a.enterLive {
		t.Errorf("the arrival is still live after its 220ms")
	}
}

// The points the travel is *drawn* at: the rest shape at 0 and at 1, the
// reference's 6px lift at 0 and none at 1, and the same `--spring` overshoot
// the toasts already travel on.
func TestTheArrivalTravelsOnTheSpringCurve(t *testing.T) {
	if got := toastSpring(0); got > 0.001 {
		t.Errorf("the arrival starts at %v, want 0", got)
	}
	if got := toastSpring(1); got < 0.999 {
		t.Errorf("the arrival ends at %v, want 1", got)
	}
	// `cubic-bezier(0.34, 1.56, 0.64, 1)` passes its target before it settles;
	// a curve that only ever approaches 1 would read as a plain ease-out.
	peak := float32(0)
	for p := float32(0); p <= 1; p += 0.01 {
		if v := toastSpring(p); v > peak {
			peak = v
		}
	}
	if peak <= 1.001 {
		t.Errorf("the arrival's peak is %v: the spring's overshoot is gone", peak)
	}
	// The opening frame's shape, as `shell-enter` writes it: scale 0.97, 6px
	// down. Drawn from the same expression the View uses, so a change to either
	// number would have to change this line too.
	settle := toastSpring(0)
	scale := shellEnterScale + (1-shellEnterScale)*settle
	lift := shellEnterLift * (1 - settle)
	if absf(scale-0.97) > 0.001 {
		t.Errorf("the arrival opens at scale %v, want the reference's 0.97", scale)
	}
	if absf(lift-6) > 0.001 {
		t.Errorf("the arrival opens %vpx down, want the reference's 6px", lift)
	}
	// And it lands on the rest shape: scale 1 and no lift.
	settle = toastSpring(1)
	if absf(shellEnterScale+(1-shellEnterScale)*settle-1) > 0.01 {
		t.Errorf("the arrival lands at scale %v, want 1", shellEnterScale+(1-shellEnterScale)*settle)
	}
	if absf(shellEnterLift*(1-settle)) > 0.05 {
		t.Errorf("the arrival lands %vpx down, want 0", shellEnterLift*(1-settle))
	}
	// The transform grows from `transform-origin: center top` (`base.css:1243`),
	// which is what makes the panel hang from its top edge rather than swell
	// from its middle: the two fractions the View hands `TransformOrigin`.
	if shellEnterOriginX != 0.5 || shellEnterOriginY != 0 {
		t.Errorf("the arrival's origin is (%v, %v), want the reference's center top",
			shellEnterOriginX, shellEnterOriginY)
	}
}
