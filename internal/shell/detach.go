package shell

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
)

// The detached output window: one captured run in a window of its own, which
// does not follow the launcher's summon and hide — that is the whole point of
// detaching (the old build's `pluginWindow`, R84/R90). It re-runs the command
// it was opened for, from its own header, so a command whose output the user
// wants to watch does not need the panel to stay up.

// detachedOutput is one open window's state: the command it belongs to, the
// view it is showing, and its scroll offset.
type detachedOutput struct {
	win     *mygo.Window
	entry   *extensions.CommandEntry
	view    launcher.OutputView
	title   string
	scroll  ui.ScrollState
	running bool
}

// detachOutput opens a captured run in a window of its own.
func (a *App) detachOutput(view launcher.OutputView) {
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	title := view.Title
	if title == "" {
		title = copy.OutputWindowFallback
	}
	detached := &detachedOutput{entry: view.Entry, view: view, title: title}
	if a.openDetached != nil {
		detached.win = a.openDetached(view)
	} else {
		detached.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     title,
			Width:     720,
			Height:    460,
			MinWidth:  420,
			MinHeight: 240,
			Content: ui.View(func(c *ui.Context) {
				a.detachedView(c, detached)
			}),
		})
	}
	if detached.win != nil {
		detached.win.OnClosed(func() { a.forgetDetached(detached) })
	}
	a.detached = append(a.detached, detached)
}

// detachedView draws one detached window: its title, the two controls that
// belong to it (re-run and close), and the output itself.
func (a *App) detachedView(c *ui.Context, detached *detachedOutput) {
	// The window is its own Context: the shell sets the floter theme on the
	// panel's, and a detached window that did not would paint the framework's
	// stock palette instead.
	surface := launcher.Resolve(a.Store.Snapshot(), c.Theme().Dark)
	tokens := surface.Tokens
	c.SetTheme(tokens.Theme)
	t := c.Theme()
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	// A detached window is opaque (`.plugin-window`): the near-solid stand-in,
	// not the panel's glass tint.
	c.Root().Background(tokens.Opaque)

	ui.Column(c).Fill().Padding(t.Space(3)).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, detached.title).FontSize(t.FontSize).FontWeight(650).
				TextColor(t.Text).Grow(1)
			// The window's own controls are the reference's bar buttons: a
			// transparent mark that only lands on a pane while hovered.
			barButton := func(label string, run func()) {
				button := ui.Row(c).Focusable().Shrink(0).Padding(t.Space(1), t.Space(2.5)).
					Radius(tokens.RadiusSM).Label(label)
				if button.Hovered() {
					button.Background(tokens.ControlHover)
				}
				button.Children(func() {
					ui.Text(c, label).FontSize(tokens.Caption).TextColor(tokens.TextTertiary)
				})
				if button.Clicked() {
					run()
				}
			}
			if detached.entry != nil {
				barButton(copy.OutputWindowRerun, func() {
					if !detached.running {
						a.rerunDetached(detached)
					}
				})
			}
			barButton(copy.OutputWindowClose, func() {
				detached.win.Close()
				a.forgetDetached(detached)
			})
		})
		ui.Scroll(c.Key("detached.output")).TrackScroll(&detached.scroll).Fill().
			Border(1, tokens.ControlEdge).Radius(tokens.RadiusMD).Padding(t.Space(2)).Children(func() {
			text := detached.view.Text
			if text == "" {
				text = copy.OutputWindowIdle
			}
			ui.Text(c, text).FontSize(t.FontSize).Font(detached.view.FontFamily).
				TextColor(t.Text)
		})
	})
}

// rerunDetached runs the window's command again, in the background, and puts
// the new output in the window. The window stays up through the run — it does
// not follow the panel, and it does not close on its own.
func (a *App) rerunDetached(detached *detachedOutput) {
	if detached.entry == nil || detached.running {
		return
	}
	detached.running = true
	entry := *detached.entry
	a.runCommandCaptured(entry, nil, func(run extensions.CapturedRun, err error) {
		detached.view = a.Launcher.OutputFor(run, err, &entry)
		detached.running = false
		if detached.win != nil {
			detached.win.Invalidate()
		}
	})
}

// forgetDetached drops a window the user closed.
func (a *App) forgetDetached(detached *detachedOutput) {
	for i, open := range a.detached {
		if open == detached {
			a.detached = append(a.detached[:i], a.detached[i+1:]...)
			return
		}
	}
}

// closeDetached closes every detached window: the app is quitting, and a
// window that outlives its app would be a process nobody can reach.
func (a *App) closeDetached() {
	for _, detached := range a.detached {
		if detached.win != nil {
			detached.win.Close()
		}
	}
	a.detached = nil
}
