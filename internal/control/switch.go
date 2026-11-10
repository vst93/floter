package control

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// SwitchOnFill is a surface color, independent of the accent used for small
// text on translucent backgrounds. It keeps a white thumb distinct in both
// appearances, including inactive windows.
func SwitchOnFill(dark bool) ui.Color {
	if dark {
		return ui.Hex("#2878D7")
	}
	return ui.Hex("#1769C2")
}

// Switch keeps the framework's input, accessibility and thumb motion while
// giving its on state a stable, saturated fill. Readability adjustments for
// text must not turn the track and its white thumb into the same color.
func Switch(c *ui.Context, on *bool) ui.Element {
	t := c.Theme()
	sw := ui.Switch(c, on)
	target := float32(0)
	if *on {
		target = 1
	}
	position := sw.Animate("switch.fill", target, 140*time.Millisecond)
	off := t.Border.Mix(t.Text, 0.15)
	return sw.Background(off.Mix(SwitchOnFill(t.Dark), position))
}
