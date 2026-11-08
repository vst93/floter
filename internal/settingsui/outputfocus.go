package settingsui

import "github.com/egoist/mygo/ui"

// The page's scroll edge occupies 16px and the framework's keyboard ring
// extends 3px beyond a control. Reveal that ink too, without changing its
// layout footprint or following focus while the user scrolls afterwards.
func outputFocusOutlet(c *ui.Context, draw func(), reveal ...func() bool) {
	const clearance = float32(19)
	box := ui.Column(c).Padding(clearance, 0).Margin(-clearance, 0)
	box.Children(draw)
	was := ui.Local(box, "output-outlet-focused", func() bool { return false })
	focused := box.FocusWithin()
	if focused && !*was && (len(reveal) == 0 || reveal[0]()) {
		box.ScrollIntoView()
	}
	*was = focused
}
