// Package deadline asks for the one frame a timed line needs.
//
// The app draws several lines that change exactly once: a toast that clears
// itself, a confirmation that lapses, a debounce that flushes, a failure note
// that goes away. While such a line is up nothing about it moves, so the frames
// it needs are "one, when the clock runs out" — not one per display refresh.
// `Context.AnimationFrame` is the wrong tool for that: it asks for the next
// frame to *build the whole view again* (`mygo v0.7.0 ui/runtime.go:744-748`,
// `Context.AnimationFrame` → "build the view anew"), which at 120 Hz is 120
// rebuilds a second for a sentence that stayed the same. `Context.After` arms a
// single wake at a deadline instead, which is what the settings page's "copied"
// confirmations already did (`internal/settingsui/settingsui.go:3917`).
//
// The two motions that do move — the shell's panel entrance and its toast
// entrance — keep asking for frames: they animate their progress, and a frame is
// what draws the next step of it.
package deadline

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// wakeSettle is how far past a deadline the wake is armed. It is not a
// tolerance in the app's own clock: the timers that use this package all read
// "has it waited *longer* than the window" (`now.Sub(at) > window`,
// `now.After(until)`), and a wake at exactly the deadline reads as equal, so the
// line would find its clock still live, arm another wake of zero, and never wake
// again. One millisecond later the same comparison is true. Production timers
// only ever fire late, so this costs the deadline one frame at worst.
const wakeSettle = time.Millisecond

// Wake asks for a frame once wait has passed, and for none before it.
//
// Call it on every frame the line is up: each call re-arms the one wake, and
// when the deadline comes the frame it asks for is the one that finds the clock
// run out. A nil context — a caller outside a frame, as a test that expires a
// clock by hand — and a wait that has already run out both arm nothing: the
// frame being built sees the deadline for itself.
func Wake(c *ui.Context, wait time.Duration) {
	if c == nil || wait <= 0 {
		return
	}
	c.After(wait + wakeSettle)
}
