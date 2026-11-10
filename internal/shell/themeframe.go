package shell

import (
	"floter/internal/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Each native window owns its appearance history. A theme change settles all
// control colors together; ordinary hover transitions remain available.
type themeFrame struct {
	seen       bool
	appearance theme.Appearance
	ink        ui.Color
}

func (f *themeFrame) apply(c *ui.Context, tokens theme.Tokens) {
	if f.seen && (f.appearance != tokens.Appearance || f.ink != tokens.Theme.TextMuted) {
		c.DisableAnimations()
	}
	f.appearance, f.seen = tokens.Appearance, true
	f.ink = tokens.Theme.TextMuted
	c.SetTheme(tokens.Theme)
}

// Called on the UI thread after settings change, including while the main
// panel is hidden. Static output windows otherwise never ask for a new frame.
func (a *App) invalidateAppearance() {
	windows := []*mygo.Window{a.Win}
	a.pinMu.Lock()
	for win := range a.pins {
		windows = append(windows, win)
	}
	a.pinMu.Unlock()
	for _, detached := range a.detached {
		windows = append(windows, detached.win)
	}
	for _, win := range windows {
		if win != nil {
			win.Invalidate()
		}
	}
}
