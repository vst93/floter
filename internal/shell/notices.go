package shell

import (
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/theme"
)

// The app's one notice: a sentence raised by an external trigger, shown where
// the user is even when the launcher is not the surface on screen.
//
// The old build's toast stack was that surface — a sibling of the mode shell,
// so it stayed put across a surface switch — and a refusal raised by a
// `floter://` link is exactly the case the docked launcher row cannot serve:
// the link lands on the settings page, and that row is drawn under a list the
// user is not looking at. The framework draws these
// (`ui.Context.AddToast`), so the app keeps no second toast stack of its own.

// refusalWindow is how long one refusal stays quiet after it has been raised
// once: the old build's FAILURE_NOTIFY_DEDUP_MS. The trigger is external and
// can repeat without anybody pressing anything — a page retrying a broken
// link, a shell loop — so one bad link must not fill the window with copies of
// the same sentence, while the same refusal coming back later is news again.
const refusalWindow = 30 * time.Second

// refusalKey is the one key the refusals share: the old build sent a single
// dictionary key (`settings.deepLinkRejected`), so every refused link is the
// same failure and coalesces onto the same window.
const refusalKey = "deep-link-rejected"

// refusalNotice is how long the refusal sentence stays readable: the old
// build's error-toast window (its `TOAST_DISMISS_MS.error`), longer than the
// framework's four-second default because it carries something the user may
// want to act on.
const refusalNotice = 8 * time.Second

// warningNotice is the advisory window: the old build's
// `TOAST_DISMISS_MS.warning`, one step shorter than the error's because a
// warning says what will not happen yet rather than what failed.
const warningNotice = 6 * time.Second

// refusalGate coalesces an automatic, externally triggered failure per key,
// the rule the old build's `createFailureDeduper` implemented. The clock is
// injectable so a test can drive a burst without waiting wall-clock time.
type refusalGate struct {
	mu     sync.Mutex
	window time.Duration
	now    func() time.Time
	at     map[string]time.Time
}

func newRefusalGate(window time.Duration, now func() time.Time) *refusalGate {
	return &refusalGate{window: window, now: now, at: map[string]time.Time{}}
}

// allow reports whether key may be raised now, and if so marks it raised: the
// second and later calls inside the window return false, the first call after
// it elapses returns true again.
func (g *refusalGate) allow(key string) bool {
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if last, ok := g.at[key]; ok && now.Sub(last) < g.window {
		return false
	}
	g.at[key] = now
	return true
}

// queueNotice remembers a toast for the next frame. The view cannot be built
// from a background goroutine, so a notice is queued and drawn by View; the
// one id means a second notice replaces the first rather than stacking beside
// it.
func (a *App) queueNotice(message string, window time.Duration) {
	a.noticeMu.Lock()
	a.notices = append(a.notices, ui.Toast{
		ID:      "floter-notice",
		Title:   message,
		Type:    "error",
		Timeout: window,
	})
	a.noticeMu.Unlock()
	if a.Win != nil {
		a.Win.Invalidate()
	}
}

// warnNotice is the app's advisory notice: the old build's
// `onNotify("warning", …)`, raised for something the user should know about
// but that did not fail. The extension form's missing toolchain is the case
// that needed it — the entry is saved either way, so the sentence has to
// arrive wherever the user is, not in the launcher's docked row. It carries
// no `Type`: the framework's default toast look paints one tone whatever the
// type says, and the old build's warning tone has nowhere to land in it.
func (a *App) warnNotice(message string) {
	if message == "" {
		return
	}
	a.queueNotice(message, warningNotice)
}

// flushNotices hands the queued notices to the framework's own toast stack,
// once, at the start of a frame (the engine rebuilds a frame up to three
// times, so clearing the queue here is what keeps one notice from being added
// again on the next pass).
func (a *App) flushNotices(c *ui.Context) {
	a.noticeMu.Lock()
	pending := a.notices
	a.notices = nil
	a.noticeMu.Unlock()
	for _, toast := range pending {
		c.AddToast(toast)
	}
}

// toastTone is the color a toast's mark and its sentence wear, by the kind its
// raiser tagged it with — the old build's `.app-toast--error` / `--warning` /
// `--success`.
//
// Error and warning share the amber band: the reference's own R9-1 note on the
// CSS says so ("it shares the error band's amber *label*"), and the kind earns
// its keep by the *icon* and the accessibility role, not a second colour.
// Success wears the accent; an untagged toast wears the theme's ordinary text,
// so a caller that sets no `Type` still reads as plain information.
func toastTone(t *ui.Theme, kind string) ui.Color {
	switch kind {
	case "error", "warning":
		return t.Warning
	case "success":
		return t.Accent
	default:
		return t.Text
	}
}

// toastMark is the icon each kind leads with: the old build's `ToastStack` drew
// lucide's AlertCircle, TriangleAlert and Check at 15px. An untagged toast has
// no mark.
func toastMark(kind string) string {
	switch kind {
	case "error":
		return "alert"
	case "warning":
		return "triangle-alert"
	case "success":
		return "check"
	default:
		return ""
	}
}

// toastViewport draws the app's notices with the old build's own look, in place
// of the framework's theme one.
//
// The framework's default paints every toast the same inverted pill: its
// `buildToasts` takes `t.inverse()` and reads nothing of `Toast.Type`, so a
// refused deep link and an advisory warning arrive indistinguishable. The old
// build's `.app-toast` was a floating pane on `--glass-float` with a 1px edge
// and the kind's own mark and tone. The framework offers exactly one seam for
// that — `ui.ToastViewportBase`, which replaces the theme's viewport for the
// frame — and this is it: the `Float` token is the old `--glass-float`, the
// `InputStroke` its `--stroke-control`, and the tone and mark come from the
// kind above.
func (a *App) toastViewport(c *ui.Context) {
	// The old build's stack had one geometry per surface
	// (`src/styles/extensions.css:210-290`): the two full-height surfaces
	// (settings, the terminal, a plugin page) hang it from the top right, under
	// the 56u card header; the collapsed launcher — a ~58px strip where no
	// placement is overlap-free — pins its *newest* toast to a 20u band on the
	// bottom-left and drops the ones queued behind it. `ui.ToastViewportBase` is
	// the one seam that replaces the theme's own bottom-centre pill.
	collapsed := a.Surf == SurfaceLauncher
	width, _ := c.Size()
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		t := c.Theme()
		tokens := theme.ForPreferences(a.Store.Snapshot(), t.Dark, c.Preferences())
		copy := i18n.For(a.Store.Snapshot().Language).Settings
		if collapsed {
			// One toast, newest wins. Earlier ones are dropped from layout, not
			// just hidden, so the thin window never reserves a second row.
			if len(toasts) > 1 {
				toasts = toasts[len(toasts)-1:]
			}
			viewport.Bottom(t.Space(1.5)).Left(t.Space(1.5)).
				AlignItems(ui.Start).Gap(t.Space(2)).PassThrough()
		} else {
			viewport.Top(t.Space(16)).Right(t.Space(4)).
				AlignItems(ui.End).Justify(ui.Start).Gap(t.Space(2)).PassThrough()
		}
		for _, toast := range toasts {
			p := ui.ToastBase(c, toast)
			if !p.Root.Valid() {
				continue
			}
			tone := toastTone(t, toast.Type)
			// The column's own width: `min(360px, 100% - 32px)` on the full-height
			// surfaces, `min(300px, 100% - 12px)` on the launcher strip.
			want, room := t.Space(90), width-t.Space(8)
			if collapsed {
				want, room = t.Space(75), width-t.Space(3)
			}
			p.Root.Row().AlignItems(ui.Center).Gap(t.Space(2)).
				Width(min(want, room)).MaxWidth(room).
				Padding(t.Space(2), t.Space(3)).Radius(tokens.RadiusMD).
				Background(tokens.Float).Border(1, tokens.InputStroke).TextColor(tone)
			dismiss := t.Space(6)
			mark := t.Space(3.25)
			if collapsed {
				// The strip: one forced line at the caption step, and a slimmer
				// dismiss control so the 20u band still leaves a hit target.
				p.Root.Height(t.Space(5)).Padding(0, t.Space(2)).Gap(t.Space(1.5))
				dismiss = t.Space(4.5)
			}
			p.Root.Children(func() {
				if name := toastMark(toast.Type); name != "" {
					if glyph, ok := launcher.Glyph(name); ok {
						ui.Icon(c, glyph).Size(t.Space(3.75), t.Space(3.75)).TextColor(tone)
					}
				}
				sentence := ui.Text(c, toast.Title).TextColor(tone).Grow(1)
				if collapsed {
					sentence.FontSize(tokens.Caption).SingleLine().Ellipsis("\u2026")
				} else {
					sentence.FontSize(t.FontSize)
				}
				// The optional action chip (a page failure's retry), the old
				// `.app-toast__action`: a quiet control-material chip the same
				// height as the dismiss control. Pressing it runs the caller's
				// closure and closes the toast, so a retry never leaves a stale
				// one behind.
				if toast.Action != "" {
					action := ui.Row(c).Focusable().Shrink(0).Height(t.Space(5.5)).
						Padding(0, t.Space(2)).Radius(tokens.RadiusSM).
						AlignItems(ui.Center).Justify(ui.Center).
						Background(tokens.Control).Label(toast.Action)
					// The reference's `--elev-0` lens on the chip: without it the
					// action is a flat tint beside the floating card, and the
					// glass stop never grades it.
					tokens.Lens(action)
					if action.Hovered() {
						action.Background(tokens.ControlHover)
					}
					action.Children(func() {
						ui.Text(c, toast.Action).FontSize(tokens.Caption).FontWeight(600).
							TextColor(tokens.TextStrong)
					})
					if action.Clicked() {
						if toast.OnAction != nil {
							toast.OnAction()
						}
						c.CloseToast(toast.ID)
					}
				}
				button := ui.Row(c).Focusable().Shrink(0).Size(dismiss, dismiss).
					Radius(tokens.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
					Label(copy.DismissNotice)
				if button.Hovered() {
					button.Background(tokens.ControlHover)
				}
				button.Children(func() {
					ui.Icon(c, launcherGlyph("close")).Size(mark, mark).
						TextColor(tokens.TextTertiary)
				})
				if button.Clicked() {
					c.CloseToast(toast.ID)
				}
			})
		}
	})
}

// launcherGlyph is launcher.Glyph with the fallback the app's other glyph reads
// use: the names here are program constants, so a missing one is a bug rather
// than a runtime condition.
func launcherGlyph(name string) *ui.SVG {
	glyph, ok := launcher.Glyph(name)
	if !ok {
		glyph, _ = launcher.Glyph("alert")
	}
	return glyph
}

// refuseLink is what a refused `floter://` link does: it brings the panel
// forward without changing what it is showing (the old build's `focus_main`),
// and raises the one refusal sentence — unless the same refusal is still inside
// its window, in which case one attempt's worth of noise is enough. Safe to
// call from a background goroutine (a `connect` that failed after the review).
func (a *App) refuseLink() {
	if a.refusals == nil || !a.refusals.allow(refusalKey) {
		return
	}
	message := i18n.For(a.Store.Snapshot().Language).Settings.DeepLinkRejected
	a.onMain(func() {
		a.Show()
		a.queueNotice(message, refusalNotice)
	})
}
