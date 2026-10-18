package shell

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
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
	// failed is the sentence of a run of this window that never produced an
	// answer: the host refused to start it, it was killed, or it timed out
	// without printing anything. It is only ever drawn while the view has no
	// text of its own, because a bounded run that overran its deadline and said
	// something still has its output to show (see runCaptured).
	failed string
	// ran is whether this window has ever run its command: it is what separates
	// the "nothing to show" state (`launcher.externalEmpty`, after a run) from
	// the "no command pinned yet" one (`pluginWindow.idle`, before any).
	ran bool
	// generation counts content replacements, so a run the window started for
	// older content cannot overwrite the run the user pinned after replacing
	// it (the old build's newest-run-wins rule).
	generation int
	// loadingMore is whether a page fetch for a self-paging list is in flight,
	// so the scroll trigger asks once per page rather than once per frame.
	// An in-memory window has no fetch and never sets it.
	loadingMore bool
	// frames counts the body's layouts, so the scroll trigger can tell a body
	// that has not been measured yet (every `ScrollState` reads zero on its
	// first frame) from one whose content genuinely fits its box. Only a body
	// that fits its box *and* claims another page is at its end with `MaxY` at
	// zero, and that one still asks — the reference's `remaining <= 48` reads
	// zero there too.
	frames int
}

// detachLoadMoreGap is how close to the body's end the scroll has to be for
// the next page to be asked for: the reference's shared
// `PLUGIN_LOAD_MORE_THRESHOLD` (`main:src/launcher/plugin-mode.ts:496`, 48),
// which the detached window read through the same constant the launcher's own
// list did (`DetachedPluginApp.tsx:267-270`).
const detachLoadMoreGap = 48

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
		existing.loadingMore = false
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

// The detached window's chrome, in the reference's own units (`--u`, four to
// a `Space`): `.plugin-window__bar`'s `10u 14u` padding and its 8u gap, the
// bar's hairline floor, the body's 14u inset, and the list rows' 8u/10u plate
// with the 4u gap between them (`main:src/styles/launcher.css:1715-1818`).
const (
	detachedBarPadY      = 2.5 // 10u
	detachedBarPadX      = 3.5 // 14u
	detachedBarGap       = 2   // 8u
	detachedBarActionGap = 1.5 // 6u
	detachedBarButtonX   = 2.5 // 10u
	detachedBarButtonY   = 1   // 4u
	detachedBodyInset    = 3.5 // 14u
	detachedRowGap       = 1   // 4u
	detachedRowPadX      = 2.5 // 10u
	detachedRowPadY      = 2   // 8u
)

// detachedView draws one detached window: its bar (the command's name and its
// two controls), and the output itself.
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
	// not the panel's glass tint. The window is a plain column: the bar is
	// `flex: 0 0 auto` and the body takes the rest, so the body scrolls under a
	// bar that never moves (`launcher.css:1715-1723`).
	c.Root().Background(tokens.Opaque)

	ui.Column(c).Fill().Children(func() {
		a.detachedBar(c, detached, tokens, t, copy)
		a.detachedBodyView(c, detached, tokens, t, copy)
	})
}

// detachedBar is the reference's `.plugin-window__bar`: a slim header holding
// the command's name (muted, caption-sized) on the left and the two controls
// on the right, separated from the body by one `--hairline` rule
// (`DetachedPluginApp.tsx:284-320`, `launcher.css:1724-1745`).
func (a *App) detachedBar(c *ui.Context, detached *detachedOutput, tokens theme.Tokens, t *ui.Theme, copy i18n.Launcher) {
	bar := ui.Row(c).FillWidth().Shrink(0).
		Padding(t.Space(detachedBarPadY), t.Space(detachedBarPadX)).
		Gap(t.Space(detachedBarGap)).AlignItems(ui.Center)
	// The bar's own floor: `border-bottom: 1px solid var(--hairline)`. An inset
	// shadow rather than a border, so the bar's box does not grow by the line
	// the reference never charged.
	bar.InsetShadow(0, -1, 0, 0, tokens.Hairline)
	bar.Children(func() {
		// The title is the *request's* own label, muted and at the caption
		// step, single-line and ellipsized: `plugin-window__title`'s
		// `font-size: var(--text-caption); font-weight: 500; color:
		// var(--text-muted)`. The port drew it at 650 in the primary ink — the
		// window's loudest object, where the reference keeps it the quietest.
		ui.Text(c, detached.title).FontSize(tokens.Caption).FontWeight(500).
			LetterSpacing(tokens.Caption * 0.01).TextColor(t.TextMuted).Grow(1).
			SingleLine().Ellipsis("\u2026")
		actions := ui.Row(c).Shrink(0).Gap(t.Space(detachedBarActionGap)).AlignItems(ui.Center)
		actions.Children(func() {
			// The window's own controls are the reference's bar buttons: a
			// transparent mark that only lands on a pane while hovered. A
			// disabled one (the re-run key while a run is in flight) reports no
			// clicks and dims to the reference's 0.5 opacity — and, as its
			// `:hover:not(:disabled)` asks, never lights up either.
			barButton := func(label string, enabled bool, run func()) ui.Element {
				button := ui.Row(c).Focusable().Shrink(0).
					Padding(t.Space(detachedBarButtonY), t.Space(detachedBarButtonX)).
					Radius(tokens.RadiusSM).Label(label)
				if !enabled {
					button.Disabled(true)
					// The reference's `:disabled { opacity: 0.5 }`: the dim is the
					// *key's* own, so the framework's disabled flag alone (which
					// changes no ink) would leave the key looking live.
					button.Opacity(0.5)
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
	})
}

// detachedBodyView is the reference's `.plugin-window__body`: the run state
// machine's one line, the text arm's verbatim block, or the list arm's rows —
// scrolling under the bar inside one 14u inset (`DetachedPluginApp.tsx:322-350`,
// `launcher.css:1776-1818`).
func (a *App) detachedBodyView(c *ui.Context, detached *detachedOutput, tokens theme.Tokens, t *ui.Theme, copy i18n.Launcher) {
	detached.frames++
	body := ui.Scroll(c.Key("detached.output")).TrackScroll(&detached.scroll).Fill().
		Padding(t.Space(detachedBodyInset))
	body.Children(func() {
		// The state machine's own three lines come first: while a run is in
		// flight, after a refusal, and for a finished run with nothing to show
		// (`DetachedPluginApp.tsx:325-338`). The list and the text are the two
		// "done" arms, and only a list reaches the rows.
		switch {
		case detached.running:
			detachedStatus(c, copy.OutputWindowRunning, false, tokens, t)
		case detached.failed != "":
			detachedStatus(c, detached.failed, true, tokens, t)
		case len(detached.view.Text) > 0 && !detached.view.IsList:
			detachedText(c, detached, tokens, t)
		case detached.view.IsList:
			detachedList(c, detached.view.Rows, tokens, t)
			// R95 · the list arm pages exactly as the launcher's does: reaching
			// the end of what is loaded asks for the next page. The port drew the
			// first window only, so a plugin that emitted more than two pages had
			// rows in this window that nothing could reach — the reference's
			// scroll trigger (`DetachedPluginApp.tsx:248-270`) had no port at all.
			detachedAskForMore(a, detached)
		case detached.ran:
			detachedStatus(c, copy.OutputWindowEmpty, false, tokens, t)
		default:
			detachedStatus(c, copy.OutputWindowIdle, false, tokens, t)
		}
	})
}

// detachedAskForMore runs the window's scroll trigger: while the list has
// another page and the body is scrolled within the reference's threshold of
// its end, ask for it. The trigger reads the scroller's own geometry
// (`detached.scroll.MaxY` / `Y`), so a taller window needs no second budget and
// a resize cannot make it lie.
//
// `frames > 1` is the "the body has been measured" test: the container writes
// MaxY as it lays the content out, so a frame before the first layout reads 0
// and would otherwise look like a body already at its end (and pull a page on
// the window's very first frame).
func detachedAskForMore(a *App, detached *detachedOutput) {
	view := &detached.view
	if view.Page == nil || !view.Page.HasMore || detached.loadingMore {
		return
	}
	if detached.frames <= 1 {
		return
	}
	if detached.scroll.MaxY-detached.scroll.Y > detachLoadMoreGap {
		return
	}
	a.loadMoreDetachedPage(detached)
}

// loadMoreDetachedPage asks for the next page of the window's list, the same
// two arms the launcher's own output view has (`launcher.loadMoreOutputPage`):
// a self-paging plugin is re-run with the cursor from its last emission, and an
// in-memory window simply grows by one page. A complete list, or a fetch already
// in flight, is left alone.
func (a *App) loadMoreDetachedPage(detached *detachedOutput) {
	view := &detached.view
	if !view.IsList || view.Page == nil || !view.Page.HasMore || detached.loadingMore {
		return
	}
	if view.Declared {
		if view.Entry == nil || a.Launcher.Actions.LoadOutputPage == nil {
			return
		}
		detached.loadingMore = true
		generation := detached.generation
		entry := *view.Entry
		args := append([]string{}, view.Args...)
		cursor := view.Page.Cursor
		a.Launcher.Actions.LoadOutputPage(entry, args, cursor, func(run extensions.CapturedRun, err error) {
			if detached.generation != generation {
				return // the window's content was replaced while the page ran
			}
			detached.loadingMore = false
			emission, ok := extensions.ParseEmission(run.Stdout)
			if !ok {
				// The plugin stopped speaking the protocol: keep what we have and
				// stop asking, rather than looping on a page we cannot read.
				view.Page = &extensions.Page{}
			} else {
				view.Rows = extensions.MergeRows(view.Rows, emission.Rows)
				view.AllRows = view.Rows
				if emission.Page != nil {
					view.Page = emission.Page
				} else {
					view.Page = &extensions.Page{}
				}
			}
			if detached.win != nil {
				detached.win.Invalidate()
			}
		})
		return
	}
	view.PagesLoaded++
	rows, page := extensions.WindowRows(view.AllRows, view.PagesLoaded)
	view.Rows = rows
	if page.HasMore {
		view.Page = &page
	} else {
		view.Page = nil
	}
	if detached.win != nil {
		detached.win.Invalidate()
	}
}

// detachedStatus draws one line of the run state machine
// (`.plugin-window__status`): caption-sized, muted, and in the warning ink when
// the run was refused (`launcher.css:1783-1789`).
func detachedStatus(c *ui.Context, text string, failed bool, tokens theme.Tokens, t *ui.Theme) {
	ink := t.TextMuted
	if failed {
		ink = t.Warning
	}
	ui.Text(c, text).FontSize(tokens.Caption).TextColor(ink)
}

// detachedText draws the text arm verbatim in the terminal's monospace
// (`.launcher-plugin-text`, `launcher.css:2615-2637`): the body register at a
// 1.45 line, pre-wrapped, selectable — the output is text the user may want,
// and selecting it is what copies it. The port drew it at the caption step in
// the primary ink, one register quiet where the reference reads it as output.
func detachedText(c *ui.Context, detached *detachedOutput, tokens theme.Tokens, t *ui.Theme) {
	ui.Text(c, detached.view.Text).Font(detached.view.FontFamily).
		FontSize(tokens.Body).LineHeight(1.45).
		TextColor(tokens.TextSecondary).Wrap().Selectable()
}

// detachedList is the list arm: the reference's `.plugin-window__list`, a
// column of `.plugin-window__row` plates — the resting control fill, a 9u
// corner, 8u/10u of padding and a 4u gap between rows
// (`DetachedPluginApp.tsx:341-350`, `launcher.css:1792-1818`). A status row is
// information about the list rather than an entry in it, so it wears the muted
// status line instead of a plate (the same split the launcher's own list makes).
func detachedList(c *ui.Context, rows []extensions.Row, tokens theme.Tokens, t *ui.Theme) {
	ui.Column(c).FillWidth().Gap(t.Space(detachedRowGap)).Children(func() {
		for _, row := range rows {
			if row.Status {
				detachedStatus(c, row.Title, false, tokens, t)
				continue
			}
			plate := ui.Column(c).FillWidth().Gap(t.Space(0.5)).
				Padding(t.Space(detachedRowPadY), t.Space(detachedRowPadX)).
				Radius(tokens.RadiusSM).Background(tokens.Control)
			plate.Children(func() {
				ui.Text(c, row.Title).FontSize(tokens.Caption).FontWeight(500).
					TextColor(tokens.TextStrong).SingleLine().Ellipsis("\u2026")
				if row.Subtitle != "" {
					ui.Text(c, row.Subtitle).FontSize(tokens.Caption).
						TextColor(t.TextMuted).SingleLine().Ellipsis("\u2026")
				}
			})
		}
	})
}

// rerunDetached runs the window's command again, in the background, and puts
// the new output in the window. The window stays up through the run — it does
// not follow the panel, and it does not close on its own — and it says so while
// the run is in flight rather than showing the previous answer as if it were
// current.
func (a *App) rerunDetached(detached *detachedOutput) {
	if detached.entry == nil || detached.running {
		return
	}
	detached.running = true
	detached.failed = ""
	if detached.win != nil {
		detached.win.Invalidate()
	}
	entry := *detached.entry
	generation := detached.generation
	a.runCommandCaptured(entry, nil, func(run extensions.CapturedRun, err error) {
		a.applyDetachedRun(detached, generation, a.Launcher.OutputFor(run, err, &entry), err)
	})
}

// applyDetachedRun installs a finished run in the window, unless the window's
// content was replaced while the run was in flight — in that case the run is
// stale and the user's newer pin keeps the window. A run that never produced an
// answer — the host refused to start it, it was killed, or it timed out without
// printing anything — carries its sentence for the body to draw.
func (a *App) applyDetachedRun(detached *detachedOutput, generation int, view launcher.OutputView, runErr error) {
	if detached.generation != generation {
		return
	}
	detached.view = view
	detached.running = false
	detached.ran = true
	detached.failed = ""
	// A new answer starts its list at the first window again, as the reference's
	// run reset the page count (`DetachedPluginApp.tsx:126`).
	detached.loadingMore = false
	// A run that overran its deadline after saying something is not a refusal:
	// the reference read its output (see `extensions.runCaptured`).
	if runErr != nil && view.Text == "" {
		detached.failed = a.Launcher.RunStatus(extensions.CapturedRun{}, runErr)
		if detached.failed == "" {
			// The reference's own fallback when a failure carried nothing
			// readable (`runErrorMessage(...) || failure || externalFailed`).
			detached.failed = a.Launcher.Copy().OutputWindowFailed
		}
	}
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
