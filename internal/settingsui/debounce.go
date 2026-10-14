package settingsui

import (
	"sort"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// settingsDebounce is the reference's `SETTINGS_DEBOUNCE_MS`
// (`src/hooks/useSettings.ts:149`): a slider drag or a typed command alias
// waits this long after the last change before the file is written once. The
// reference clears and restarts the timer on every change, so a drag that
// keeps moving never writes until it stops.
const settingsDebounce = 180 * time.Millisecond

// aliasEdit is one command's latest alias draft, held while the debounce
// window runs. The map key is the extension and command ids, so a later
// keystroke on the same field replaces the earlier one instead of writing
// twice.
type aliasEdit struct {
	extensionID string
	commandID   string
	alias       string
}

// previewSetting applies mutate to the store's in-memory snapshot now, so the
// slider's readout and the frame follow the pointer, and schedules one write
// for after the window. The store only writes on Commit.
func (a *App) previewSetting(mutate func(*settings.Settings)) {
	a.Store.Preview(mutate)
	a.scheduleSettingsWrite()
}

// debounceAlias records one command's latest alias and schedules the write.
// The alias editor is a text field, so one save per pause is the reference's
// own rule — the same 180ms window as the sliders.
func (a *App) debounceAlias(extensionID, commandID, alias string) {
	if a.pendingAliases == nil {
		a.pendingAliases = map[string]aliasEdit{}
	}
	a.pendingAliases[extensionID+"\x00"+commandID] = aliasEdit{
		extensionID: extensionID,
		commandID:   commandID,
		alias:       alias,
	}
	a.scheduleSettingsWrite()
}

// scheduleSettingsWrite restarts the debounce window. The reference clears the
// old timer before setting the new one (`changeOpacity`), so the window is
// measured from the latest change.
func (a *App) scheduleSettingsWrite() {
	a.debouncing = true
	a.debounceAt = a.now().Add(settingsDebounce)
}

// flushSettingsWrites writes the pending debounced changes now: alias edits
// through their shell action, then the previewed settings through Commit. It
// is the flush path for page switches, leaving the surface, hiding the window
// and quitting, so the last drag is never lost.
//
// Alias edits run first because the shell action writes through Store.Update;
// that commits the previewed settings too, and the following Commit is then a
// no-op. With no alias pending the commit is the one write.
func (a *App) flushSettingsWrites() {
	if !a.debouncing {
		return
	}
	a.debouncing = false
	a.debounceAt = time.Time{}
	if len(a.pendingAliases) > 0 {
		edits := a.pendingAliases
		a.pendingAliases = nil
		keys := make([]string, 0, len(edits))
		for key := range edits {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			edit := edits[key]
			if a.Actions.SetCommandAlias != nil {
				a.Actions.SetCommandAlias(edit.extensionID, edit.commandID, edit.alias)
			}
		}
	}
	if err := a.Store.Commit(); err != nil {
		a.noteSaveFailed()
	}
}

// expireSettingsWrites flushes the window once it has passed and keeps frames
// coming while it runs, so a still page reaches the deadline.
func (a *App) expireSettingsWrites(c *ui.Context) {
	if !a.debouncing {
		return
	}
	if !a.now().Before(a.debounceAt) {
		a.flushSettingsWrites()
		return
	}
	if c != nil {
		c.AnimationFrame()
	}
}

// Flush writes any pending debounced settings change immediately. The shell
// calls it when the settings surface is left, the window hides, or the app
// quits.
func (a *App) Flush() { a.flushSettingsWrites() }
