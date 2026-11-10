package settingsui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const scrollbarIdle = 600 * time.Millisecond
const scrollbarFade = 160 * time.Millisecond

// Keep the framework's thumb, hit testing and drag behavior; only its ink
// fades. Hovering the content does not keep a scrollbar alive. Approaching
// the narrow gutter reveals it so a mouse can still grab it without scrolling.
type scrollbarActivity struct {
	x, y     float32
	activeAt time.Time
}

func (s *scrollbarActivity) theme(c *ui.Context, scroll ui.Element, state *ui.ScrollState) ui.Theme {
	t := *c.Theme()
	t.ScrollbarWidth = max(3, t.Space(1))
	now := c.Now()
	x, _, over := scroll.PointerPosition()
	atEdge := over && x >= scroll.Bounds().W-t.Space(1.5)-t.ScrollbarWidth-9
	moved := state.X != s.x || state.Y != s.y
	s.x, s.y = state.X, state.Y
	if moved || atEdge {
		s.activeAt = now
	}
	opacity := float32(0)
	if !s.activeAt.IsZero() {
		elapsed := now.Sub(s.activeAt)
		switch {
		case elapsed < scrollbarIdle:
			opacity = 1
			c.After(scrollbarIdle - elapsed)
		case elapsed < scrollbarIdle+scrollbarFade && !c.Preferences().ReduceMotion:
			opacity = 1 - float32(elapsed-scrollbarIdle)/float32(scrollbarFade)
			c.AnimationFrame()
		}
	}
	t.Scrollbar.A = uint8(float32(t.Scrollbar.A) * opacity)
	return t
}
