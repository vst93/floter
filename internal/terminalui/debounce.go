package terminalui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/deadline"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// settingsDebounce is the reference's `SETTINGS_DEBOUNCE_MS`
// (`main:src/hooks/useSettings.ts:149`): a slider drag waits this long after the
// last change before the file is written once. The reference clears and restarts
// the timer on every change, so a drag that keeps moving never writes until it
// stops.
//
// It is the same window `internal/settingsui/debounce.go` keeps for the settings
// page's own sliders and alias field — two copies of one reference constant
// rather than one shared one, because neither package imports the other and a
// third package for a number would be a package per number.
const settingsDebounce = 180 * time.Millisecond

// preview applies mutate to the store's in-memory snapshot now, so the drawer's
// readout and the running session follow the pointer, and schedules one write
// for after the window. The store only writes on Commit.
//
// This is the terminal's half of the reference's debounce: its drawer's font
// size and line-height sliders are wired to `changeFontSize` and
// `changeTerminalLineHeight` (`main:src/App.tsx:3925-3926`), and both of those
// restart a 180ms timer instead of writing per tick (`useSettings.ts:444-500`,
// whose comment says a drag "would write the settings file dozens of times per
// gesture"). Every other drawer row is a discrete choice, and the reference
// writes those immediately (`changeGeneralSetting`), so those keep `set`.
func (a *App) preview(mutate func(*settings.Settings)) {
	a.Store.Preview(mutate)
	a.scheduleSettingsWrite()
}

// scheduleSettingsWrite restarts the debounce window. The reference clears the
// old timer before setting the new one, so the window is measured from the
// latest change.
func (a *App) scheduleSettingsWrite() {
	a.debouncing = true
	a.debounceAt = a.frameAt.Add(settingsDebounce)
}

// flushSettingsWrites writes the previewed value now: the flush path for
// leaving the terminal surface, hiding the window and quitting. A commit with
// nothing to write is a no-op, so a stray flush costs nothing.
func (a *App) flushSettingsWrites() {
	if !a.debouncing {
		return
	}
	a.debouncing = false
	a.debounceAt = time.Time{}
	if err := a.Store.Commit(); err != nil {
		a.ShowFeedback(i18n.For(a.settings().Language).Settings.SaveFailed)
	}
}

// expireSettingsWrites flushes the window once it has passed and arms one wake
// at its end while it runs, so a still terminal reaches the deadline without
// rebuilding the view on every refresh (P11-bq's rule: a line that changes once
// asks for one frame, not one per display refresh).
func (a *App) expireSettingsWrites(c *ui.Context) {
	if !a.debouncing {
		return
	}
	wait := a.debounceAt.Sub(c.Now())
	if wait <= 0 {
		a.flushSettingsWrites()
		return
	}
	deadline.Wake(c, wait)
}

// Flush writes any pending debounced appearance change immediately. The shell
// calls it when the terminal surface is left, the window hides, or the app
// quits: a hidden window draws no frames, so the frame-driven deadline would
// never arrive and the last drag would be silently lost.
func (a *App) Flush() { a.flushSettingsWrites() }
