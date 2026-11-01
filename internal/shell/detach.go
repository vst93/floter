package shell

import (
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/clipboard"
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
	// selectionText is the selection the body's copy-on-select gesture saw on
	// the previous frame, and selectionCopied the one it last wrote. The
	// reference copies on `mouseup` (`DetachedPluginApp.tsx:270-285`, R65); a
	// selection that is the same on two frames in a row has stopped moving, and
	// that is the release (see detachedCopyOnSelect). The port had no
	// copy-on-select at all — selecting a row's text and letting go copied
	// nothing.
	selectionText   string
	selectionCopied string
	selectionAt     time.Time
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

// The press rung's geometry: the family's own two numbers, taken from the
// reference's `:active:not(:disabled)` rules and already in force in the
// launcher's chips and in P13-q's settings controls — a capsule steps back to
// 0.97, an icon key to 0.92 (`docs/liquid-glass.md` §5 「按下」). One pair of
// numbers, not a third reading of the same gesture.
const (
	pressScaleChip = 0.97
	pressScaleKey  = 0.92
)

// detachedView draws one detached window: its bar (the command's name and its
// two controls), the run's verdict when there is a body to hold it, and the
// output itself.
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
		a.detachedVerdict(c, detached, tokens, t)
		a.detachedBodyView(c, detached, tokens, t, copy)
	})
}

// detachedHasBody reports whether the body draws **content** — the text arm or
// the list arm — rather than one of the four state lines. It is the band's gate
// and the body's own switch written once, so the two cannot drift: a run whose
// body says "the command wrote nothing" is already a conclusion, and a verdict
// band above it would say the same thing twice.
func detachedHasBody(view launcher.OutputView) bool {
	return view.IsList || len(view.Text) > 0
}

// detachedVerdict draws the run's **outcome** as a band of its own, between the
// window's bar and its body.
//
// P13-bi（`docs/liquid-glass.md` §6.28）：这一页此前只有 bar 和 body。一次失败、
// 一次超时和一次成功，在窗口上**逐像素相同**——差不是「小」，是 **0**：结局那句
// 话根本没有到达这个窗口。`launcher.OutputView.Status` 一直带着它，但 `Status`
// 只在启动器自己的条子上被画过（§6.26，P13-bg），插件窗口的正文只读 `Text` /
// `Rows` / `failed` 三样。实测（`ui.Tester` 720×460，文本臂）：三个臂的 `Texts()`
// 都是 `["jv" "Close" "hello"]`。
//
// **只在「有正文」的那两个臂画**：`running` / `failed` / 空输出 / 未运行四条状态行
// 本身就是这一页的结论（§6.24 的三档阶梯），再补一条结局带会让同一页说两遍结局。
// 所以这一行的门是 `ran && !running && failed == "" && view.Verdict != ""`——
// 有内容要展示，且这个内容来自一次跑完了的运行。
//
// 三档，与 §6.26 的前三档同一条阶梯（这一页的第三档是**事实**，不是键位图例）：
//
//	结局 `TextSecondary` + 580        浅 145 / 深 133   ← 这一次运行的结论
//	结局（失败）`t.Warning` + 580       浅 139 / 深 115   ← 色相是语义，不是强调（§6.19）
//	中点 `TextTertiary` 0.7
//	事实 `TextTertiary` + 500          浅 109 / 深  89   ← 时长与截断，这一页最轻的一件
func (a *App) detachedVerdict(c *ui.Context, detached *detachedOutput, tokens theme.Tokens, t *ui.Theme) {
	view := &detached.view
	if !detached.ran || detached.running || detached.failed != "" || view.Verdict == "" ||
		!detachedHasBody(*view) {
		return
	}
	verdict := tokens.TextSecondary
	if view.Failed {
		verdict = t.Warning
	}
	spans := []ui.Span{{Text: view.Verdict, Weight: 580, Color: verdict}}
	if view.Facts != "" {
		spans = append(spans,
			// The reference's muted `·` between two facts, the same reading §6.26
			// and the settings page's fact lines give it.
			ui.Span{Text: "  \u00b7  ", Color: tokens.TextTertiary.Alpha(0.7)},
			ui.Span{Text: view.Facts, Weight: 500, Color: tokens.TextTertiary},
		)
	}
	// The band's inset is the body's own 14u on both sides, so the verdict starts
	// on the same column as the text under it (the pinned window's text and this
	// window's text are the same block, `detachedBodyInset`). Its top is 14u too:
	// the bar's hairline needs a breath below it, and the body's own 14u top
	// padding is what separates the band from the text — one inset, given once.
	ui.Row(c).FillWidth().Shrink(0).
		Padding(t.Space(detachedBodyInset), t.Space(detachedBodyInset), 0, t.Space(detachedBodyInset)).
		AlignItems(ui.Center).Children(func() {
		ui.RichText(c, spans...).FontSize(tokens.Caption)
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
					// The reference's `:disabled { opacity: 0.5 }` is the framework's
					// one fade, not a second one: `ui/paint.go:140` multiplies a
					// disabled subtree by 0.5 and `ui/inline.go:109-113` dims its text
					// by the same half. This key is a **transparent face** — no pane,
					// no edge, only its ink — and `docs/liquid-glass.md` §1's table
					// gives that arm no material of its own: half the ink is the whole
					// statement. The port used to add `.Opacity(0.5)` on top of the
					// framework's 0.5, so a run in flight drew the key at an effective
					// **0.25** (measured: ink contrast 30217 → 1928 light, 24678 →
					// 1544 dark, i.e. a quarter, not a half — P13-y). Two fades
					// multiplied is one fade drawn twice.
					button.Disabled(true)
				} else if button.Pressed() {
					// P13-al · the third rung. This bar had rest and hover only, so the
					// frame the pointer was held down on was pixel-identical to the hover
					// frame (P13-q found the same hole in the settings page's actions and
					// filled it there): a key that does not move when pressed reads as a key
					// that did not take the click. The press rung is the ladder stepping
					// *back down* — `--glass-control-press` under the hover face — and the
					// box steps with it (0.92, the icon key's number: a bar key is a small
					// square in a 24u band, not a page-level capsule). The pair is drawn
					// again inside the arm: `LitFace` is per state, so the pressed key keeps
					// its lit crown and counter-side line rather than losing them exactly
					// when it is most obviously a solid object.
					button = tokens.LitFace(button.Background(tokens.ControlPress))
					button = button.Scale(pressScaleKey, pressScaleKey)
				} else if button.Hovered() {
					// P13-y · a hovered key on this bar is an E1 face: the face step
					// plus one lit edge on top and one counter-side line at the
					// bottom (`theme.Tokens.LitFace`), the same pair
					// `launcher.rowControl` (P13-h), `settingsui.headerButton`
					// (P13-i) and `terminalui.barButton` (P13-d) wear. It used to be a
					// flat `ControlHover` wash with no line at all: measured crown =
					// face = bottom (246/246/246 light, 20/20/20 dark). No ring and no
					// shadow: a key on a bar is E1, not E2 (§2 判据 2).
					button = tokens.LitFace(button.Background(tokens.ControlHover))
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
			detachedStateLine(c, copy.OutputWindowRunning, detachedInFlight, tokens, t)
		case detached.failed != "":
			detachedStateLine(c, detached.failed, detachedFault, tokens, t)
		case len(detached.view.Text) > 0 && !detached.view.IsList:
			a.detachedCopyOnSelect(c, detached, detachedText(c, detached, tokens, t))
		case detached.view.IsList:
			a.detachedCopyOnSelect(c, detached, detachedList(c, detached.view.Rows, tokens, t))
			// R95 · the list arm pages exactly as the launcher's does: reaching
			// the end of what is loaded asks for the next page. The port drew the
			// first window only, so a plugin that emitted more than two pages had
			// rows in this window that nothing could reach — the reference's
			// scroll trigger (`DetachedPluginApp.tsx:248-270`) had no port at all.
			detachedAskForMore(a, detached)
		case detached.ran:
			detachedStateLine(c, copy.OutputWindowEmpty, detachedStatement, tokens, t)
		default:
			detachedStateLine(c, copy.OutputWindowIdle, detachedStatement, tokens, t)
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

// detachedStateKind is which of the run state machine's four lines a caller is
// drawing. The reference gave all four one register (`.plugin-window__status`:
// `--text-caption` in `--text-muted`, `launcher.css:1783-1789`), and the port
// copied that literally — measured on the window's own ground the body's four
// lines and the bar's title landed on the **same** level (light 128–131 against
// the title's 131, dark 130–133 against 130). A window whose body says what
// happened in the same ink as its own chrome has no second rung: the page
// reads as "one bar plus one grey line under it" (P13-be).
type detachedStateKind int

const (
	// detachedStatement is the page's own sentence: why this window is empty,
	// or that the run produced nothing. It is the one thing the body has to
	// say, so it takes §4.7's statement register — the same pair
	// `settingsui.emptyState` draws (emphasis step, strong ink).
	//
	// P13-be 重定基线：旧值 `Caption` + `TextMuted`（对地面 115 / 110）→
	// 新值 `Emphasis` + `TextStrong`（实测 216 / 214）。
	detachedStatement detachedStateKind = iota
	// detachedInFlight is work happening now. It is the statement's **step
	// below**, not a second statement: same register, secondary ink, medium
	// weight, so the frame that says "Running…" never outranks the sentence
	// that will replace it.
	detachedInFlight
	// detachedFault is a run that could not start. It keeps the reference's
	// warning hue (a semantic colour, not an accent — `docs/raycast-design.md`
	// §6.19) and takes the statement's register, because a fault is the page's
	// conclusion as much as an empty result is.
	detachedFault
)

// detachedStateLine draws one line of the run state machine in its own
// register (see detachedStateKind).
func detachedStateLine(c *ui.Context, text string, kind detachedStateKind, tokens theme.Tokens, t *ui.Theme) {
	ink, weight := tokens.TextStrong, 580
	switch kind {
	case detachedInFlight:
		ink, weight = tokens.TextSecondary, 500
	case detachedFault:
		ink = t.Warning
	}
	ui.Text(c, text).FontSize(tokens.Emphasis).FontWeight(weight).TextColor(ink)
}

// detachedListNote draws one **status row inside the list** — the reference's
// `plugin-window__row` variant that carries information about the list rather
// than an entry in it.
//
// P13-be：它**故意不跟** `detachedStateLine` 一起抬。它说的不是这一页，是这个
// 列表（「4 条里的 3 条」）——从属的一行，而列表的盘子（`tokens.Control`）才是
// 那一档的主体。抬到陈述档会让一条列表的注与这一页的结论平起平坐，正是
// §4.7 的反面。所以它留在参考值上：`Caption` + `TextMuted`。
func detachedListNote(c *ui.Context, text string, tokens theme.Tokens, t *ui.Theme) {
	ui.Text(c, text).FontSize(tokens.Caption).TextColor(t.TextMuted)
}

// detachedText draws the text arm verbatim in the terminal's monospace
// (`.launcher-plugin-text`, `launcher.css:2615-2637`): the body register at a
// 1.45 line, pre-wrapped, selectable — the output is text the user may want,
// and selecting it is what copies it. The port drew it at the caption step in
// the primary ink, one register quiet where the reference reads it as output.
//
// The element is returned so the caller can run the copy-on-select gesture on
// it (see detachedCopyOnSelect).
func detachedText(c *ui.Context, detached *detachedOutput, tokens theme.Tokens, t *ui.Theme) ui.Element {
	return ui.Text(c, detached.view.Text).Font(detached.view.FontFamily).
		FontSize(tokens.Body).LineHeight(1.45).
		TextColor(tokens.TextSecondary).Wrap().Selectable()
}

// detachSelectionSettle is how long a selection has to stay unchanged before
// it is copied: the reference copied on `mouseup`, and a selection that has
// stopped changing is a selection whose drag is over. It is short enough to be
// invisible and long enough that a drag's intermediate frames — which move the
// selection every frame — never reach it.
const detachSelectionSettle = 120 * time.Millisecond

// detachedCopyOnSelect is R65's copy-on-select: the reference's list and text
// arms both write the selection to the clipboard on `mouseup`
// (`DetachedPluginApp.tsx:270-285` for the list arm, `PluginTextView`'s own
// gesture for the text arm), so letting go of a drag copies what it covered.
// The framework's Selectable already copies on the explicit chord; this is the
// gesture the reference added on top of it.
//
// A release is read as *a selection that stopped changing*, because the
// framework reports a press on the paragraph the selection started in, not on
// this container (and asking that paragraph whether it is pressed would take
// its selectability away — see `Pressed`'s flag). So the gesture is: the
// selection moving starts the clock, and the first frame that sees the same
// text `detachSelectionSettle` later writes it. The frame itself is asked for
// (`Context.After`), so a release that lands in the same frame as the last move
// still copies, without waiting for some unrelated reason to draw again.
func (a *App) detachedCopyOnSelect(c *ui.Context, detached *detachedOutput, container ui.Element) {
	text := selectedText(container)
	now := time.Now()
	if text == "" {
		detached.selectionText, detached.selectionCopied, detached.selectionAt = "", "", time.Time{}
		return
	}
	if text != detached.selectionText {
		// The selection moved: it is still being dragged. Remember it, and ask
		// for the frame that reads it once it rests.
		detached.selectionText, detached.selectionCopied = text, ""
		detached.selectionAt = now
		c.After(detachSelectionSettle)
		return
	}
	if text == detached.selectionCopied {
		return
	}
	if rest := now.Sub(detached.selectionAt); rest < detachSelectionSettle {
		c.After(detachSelectionSettle - rest)
		return
	}
	detached.selectionCopied = text
	// The watcher must not record what the app itself wrote, exactly as the
	// clipboard page's own copy does (see copyClipboardEntry).
	a.selfCopied = text
	if a.writeClipboard != nil {
		a.writeClipboard(ClipboardPayload{Kind: clipboard.KindText, Text: text})
	}
}

// selectedText is the text a Selectable container's selection covers, its
// paragraphs joined the way the framework's own copy joins them: newlines, in
// build order.
func selectedText(container ui.Element) string {
	var parts []string
	for _, part := range container.SelectedTexts() {
		runes := []rune(part.Text)
		start, end := part.Start, part.End
		if start < 0 {
			start = 0
		}
		if end > len(runes) {
			end = len(runes)
		}
		if start >= end {
			continue
		}
		parts = append(parts, string(runes[start:end]))
	}
	text := strings.Join(parts, "\n")
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return text
}

// detachedList is the list arm: the reference's `.plugin-window__list`, a
// column of `.plugin-window__row` plates — the resting control fill, a 9u
// corner, 8u/10u of padding and a 4u gap between rows
// (`DetachedPluginApp.tsx:341-350`, `launcher.css:1792-1818`). A status row is
// information about the list rather than an entry in it, so it wears the muted
// status line instead of a plate (the same split the launcher's own list makes).
//
// P13-bp（`docs/liquid-glass.md` §2 / §6.34）：这块盘此前只有 `Background`
// 一句，**一条线都没有**——同一份数据在启动器列表里是 E1（§6.1 第 1-2 行：
// 面 + 顶受光边 + 底对侧暗线），在这个窗口里是一块平涂。实测（720×460，窗口
// 自己的地面 `tokens.Opaque` 浅 246 / 深 20）：浅色 冠 / 面 / 底全 **229**、
// 深色全 **20**，也就是说这块盘除了「面」以外什么都没有，而它的四周就是窗口的
// 地面——浅色档它比地面**暗 17 级**、深色档与地面同值，那块盘既没有站起来，
// 也没有浮起来。它是这一族里最后一块没戴 §2 那一对线的面。
//
// 这一族此前看不见，是因为两代 census 的判据都是「面 **+ 边**」：
// `settingsui/facecensus_test.go`（P13-p）要 `Border(`/`InsetShadow(` 同时在，
// `ci/litarm_contract_test.go`（P13-ab）只扫 `Hovered()`/`Pressed()` 臂——而这个
// 行盘既没有边框，也没有指针臂（列表不可选）。`internal/ci` 的材质账本轮补了
// 一条**面级**判据把它们盖住（见 `flatface_contract_test.go`）。
func detachedList(c *ui.Context, rows []extensions.Row, tokens theme.Tokens, t *ui.Theme) ui.Element {
	return ui.Column(c).FillWidth().Gap(t.Space(detachedRowGap)).Selectable().Children(func() {
		for _, row := range rows {
			if row.Status {
				detachedListNote(c, row.Title, tokens, t)
				continue
			}
			plate := ui.Column(c).FillWidth().Gap(t.Space(0.5)).
				Padding(t.Space(detachedRowPadY), t.Space(detachedRowPadX)).
				Radius(tokens.RadiusSM).Background(tokens.Control)
			// E1: 面 + 顶受光边 + 底对侧暗线（`theme.Tokens.LitFace`）。几何一个
			// 字节不动：`LitFace` 只加两条 1px 内线，padding box 不变（同 §6.2b-2
			// 的 `extensionActionFace`、§6.1 的 1-2）。
			plate = tokens.LitFace(plate)
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
