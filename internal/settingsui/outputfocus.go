package settingsui

import "github.com/egoist/mygo/ui"

// The page's scroll edge occupies 16px and the framework's keyboard ring
// extends 3px beyond a control. Reveal that ink too, without changing its
// layout footprint or following focus while the user scrolls afterwards.
func outputFocusOutlet(c *ui.Context, draw func(), reveal ...func() bool) ui.Element {
	const clearance = float32(19)
	box := ui.Column(c).Padding(clearance, 0).Margin(-clearance, 0)
	box.Children(draw)
	was := ui.Local(box, "output-outlet-focused", func() bool { return false })
	focused := box.FocusWithin()
	if focused && !*was && (len(reveal) == 0 || reveal[0]()) {
		box.ScrollIntoView()
	}
	*was = focused
	return box
}

// Window and body viewport dimensions can change without a focus event.
// Reveal the same envelope once on resize; scroll offsets and edited values
// are deliberately absent, so manual scrolling keeps its own position.
func (a *App) formViewportOutlet(c *ui.Context, draw func()) ui.Element {
	box := outputFocusOutlet(c, draw)
	viewport := c.Resolve(a.Body.Handle).Bounds()
	w, h := c.Size()
	size := [4]float32{viewport.W, viewport.H, w, h}
	previous := ui.Local(box, "form-viewport-size", func() [4]float32 { return size })
	if box.FocusWithin() && *previous != size {
		box.ScrollIntoView()
	}
	*previous = size
	return box
}

// Feedback can move an active control without resizing its viewport. Observe
// the preceding labels and notices, never the draft values or scroll offset.
// ScrollIntoView resolves against this frame's final layout.
func (a *App) formFeedbackOutlet(c *ui.Context, feedback string, draw func()) {
	box := a.formViewportOutlet(c, draw)
	previous := ui.Local(box, "form-feedback", func() string { return feedback })
	if box.FocusWithin() && *previous != feedback {
		box.ScrollIntoView()
	}
	*previous = feedback
}
