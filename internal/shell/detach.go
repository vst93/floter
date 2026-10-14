package shell

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// The detached output window: one captured run in a window of its own, which
// does not follow the launcher's summon and hide — that is the whole point of
// detaching (the old build's `pluginWindow`, R84/R90). It re-runs the command
// it was opened for, from its own header, so a command whose output the user
// wants to watch does not need the panel to stay up.
//
// R91 · a window belongs to its *content*, not to the press that opened it: a
// second detach of the same command raises the window already showing it (and
// replaces its contents — the R84-R90 behaviour) rather than opening a twin,
// and the window's own geometry is remembered under that content so the next
// detach opens where the user left it.

// detachedOutput is one open window's state: the content key it belongs to,
// the command it belongs to, the view it is showing, and its scroll offset.
type detachedOutput struct {
	win     *mygo.Window
	key     string
	entry   *extensions.CommandEntry
	view    launcher.OutputView
	title   string
	scroll  ui.ScrollState
	running bool
	// generation counts content replacements, so a run the window started for
	// older content cannot overwrite the run the user pinned after replacing
	// it (the old build's newest-run-wins rule).
	generation int
}

// detachContentKey is the identity of what a detached window holds: a command
// by its integration and its own id, and a run with no command behind it by
// its title. It is the old build's `content_key` — the NUL separator makes the
// integration/id split unambiguous — so `ab` + `c` never collides with `a` +
// `bc`, and it is what both the reuse check and the remembered geometry are
// keyed by.
func detachContentKey(entry *extensions.CommandEntry, title string) string {
	if entry != nil && entry.Command.ID != "" {
		return entry.IntegrationID + "\x00" + entry.Command.ID
	}
	return strings.TrimSpace(title)
}

// detachStateKey is the key the window's own geometry is remembered under, one
// per content (the old build kept `{width, height, x, y}` per `content_key`).
func detachStateKey(content string) string { return settings.DetachedStateKeyPrefix + content }

// detachWindowTitleSuffix is the old build's window-title tail. The detached
// React surface set the task-bar title to `${windowTitle} · floter`
// (`DetachedPluginApp.tsx:120-124`), so the OS window is identifiable by more
// than the command's own label; the U+00B7 middle dot is the reference's
// character. The header inside the window keeps the bare label.
const detachWindowTitleSuffix = " \u00b7 floter"

// detachWindowTitle is the window-manager title for a detached window: the
// content's own label plus the app tail.
func detachWindowTitle(label string) string { return label + detachWindowTitleSuffix }

// findDetached returns the open window already showing `key`, or nil.
func (a *App) findDetached(key string) *detachedOutput {
	for _, open := range a.detached {
		if open.key == key {
			return open
		}
	}
	return nil
}

// detachOutput opens a captured run in a window of its own, or raises the
// window already holding that content.
func (a *App) detachOutput(view launcher.OutputView) {
	copy := i18n.For(a.Store.Snapshot().Language).Launcher
	title := view.Title
	if title == "" {
		title = copy.OutputWindowFallback
	}
	key := detachContentKey(view.Entry, title)

	// The same content is already pinned out: replace what that window shows
	// and bring it forward, rather than opening a second window for one run
	// (the old build's R84-R90 rule, kept through R91).
	if existing := a.findDetached(key); existing != nil {
		existing.entry = view.Entry
		existing.view = view
		existing.title = title
		existing.running = false
		existing.generation++
		if existing.win != nil {
			existing.win.SetTitle(detachWindowTitle(title))
			existing.win.Show()
			existing.win.Focus()
			existing.win.Invalidate()
		}
		return
	}

	detached := &detachedOutput{key: key, entry: view.Entry, view: view, title: title}
	if a.openDetached != nil {
		detached.win = a.openDetached(view)
	} else {
		detached.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     detachWindowTitle(title),
			Width:     720,
			Height:    460,
			MinWidth:  420,
			MinHeight: 240,
			// Remembered per content, so the next detach of this command opens
			// where the user last left it (the old build's
			// `plugin-window-geometry.json`, keyed by `content_key`).
			StateKey: detachStateKey(key),
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
	surface := launcher.Resolve(a.Store.Snapshot(), c.Theme().Dark, c.Preferences())
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
			// transparent mark that only lands on a pane while hovered. A
			// disabled one (the re-run key while a run is in flight) reports no
			// clicks and dims to the reference's 0.5 opacity — and, as its
			// `:hover:not(:disabled)` asks, never lights up either.
			barButton := func(label string, enabled bool, run func()) ui.Element {
				button := ui.Row(c).Focusable().Shrink(0).Padding(t.Space(1), t.Space(2.5)).
					Radius(tokens.RadiusSM).Label(label)
				if !enabled {
					button.Disabled(true)
				} else if button.Hovered() {
					button.Background(tokens.ControlHover)
				}
				if button.Clicked() {
					run()
				}
				return button
			}
			// The re-run key is a text mark the reference disables for the whole
			// run (`DetachedPluginApp.tsx:293-306`).
			if detached.entry != nil {
				barButton(copy.OutputWindowRerun, !detached.running, func() {
					a.rerunDetached(detached)
				}).Children(func() {
					ui.Text(c, copy.OutputWindowRerun).FontSize(tokens.Caption).
						TextColor(tokens.TextTertiary)
				})
			}
			// The close key is the reference's icon key, not a word
			// (`DetachedPluginApp.tsx:307-315`): an X on the key face, with the
			// word kept for the accessible name only.
			barButton(copy.OutputWindowClose, true, func() {
				if detached.win != nil {
					detached.win.Close()
				}
				a.forgetDetached(detached)
			}).Children(func() {
				if glyph, ok := launcher.Glyph("close"); ok {
					ui.Icon(c, glyph).Size(t.Space(4), t.Space(4)).TextColor(tokens.TextTertiary)
				}
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
	generation := detached.generation
	a.runCommandCaptured(entry, nil, func(run extensions.CapturedRun, err error) {
		a.applyDetachedRun(detached, generation, a.Launcher.OutputFor(run, err, &entry))
	})
}

// applyDetachedRun installs a finished run in the window, unless the window's
// content was replaced while the run was in flight — in that case the run is
// stale and the user's newer pin keeps the window.
func (a *App) applyDetachedRun(detached *detachedOutput, generation int, view launcher.OutputView) {
	if detached.generation != generation {
		return
	}
	detached.view = view
	detached.running = false
	if detached.win != nil {
		detached.win.Invalidate()
	}
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
