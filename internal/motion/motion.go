// Package motion is the app's one rule for its own two ambient loops — the
// terminal bar's status dot and the launcher's paging ring — under the
// desktop's reduce-motion preference.
//
// The framework already follows the preference for the motion it drives: an
// Animate goes to its target at once and transitions collapse. Its Loop
// deliberately does not — a loop "shows that something is going on", so it goes
// on whatever the preference says. The old build drew the line the other way,
// in CSS: its `prefers-reduced-motion` block names both of these loops and
// turns them off (`animation: none`, the dot at rest at full opacity and the
// ring unrotated), and R161's wildcard block then collapses every duration the
// named block did not list. An app that wants that rule has to read the
// preference itself, which is what this package is.
package motion

import "github.com/egoist/mygo/ui"

// Value is an ambient loop's value this frame, in the loop's own 0..1 range,
// 0 at rest: the first keyframe of the old build's animation, where its loop
// starts and where a desktop that asked for less motion leaves it.
//
// phase is the reader that asks the framework for the loop's phase. It is
// called only when the desktop has not asked for less motion: a loop the
// framework has registered goes on asking for frames whether or not the view
// draws its value, so a reduce-motion frame must never register one.
func Value(c *ui.Context, phase func() float32) float32 {
	if c.Preferences().ReduceMotion {
		return 0
	}
	return phase()
}
