package shell

import (
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
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
