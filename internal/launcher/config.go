package launcher

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/plugincfg"
)

// The plugin configuration overlay: the old build's R29 sheet, which takes the
// result list's place rather than opening a page of its own —
// 「配置页面通用化：点击设置之后，弹出一个基于通用规则的配置页面，而不是一个新的
// 完全独立的页面」. The schema, the values and the meaning of a change belong to
// the shell (the plugin's own settings); the launcher owns the band, the armed
// action and the one failure line.

// Config is the open overlay: the plugin's schema, the values its settings
// hold, and the overlay's own state.
type Config struct {
	// Schema is the plugin's declarative configuration.
	Schema *plugincfg.Schema
	// Values are the values as the plugin's settings hold them. A change
	// paints here first and is replaced by what the plugin actually stored.
	Values plugincfg.Values
	// Armed is the action key whose button has been pressed once, empty for
	// none.
	Armed string
	// Failed is the one line under the fields: the last *write* was refused
	// (`settings.saveFailed`). An action's own refusal is not a write failure
	// and has its own line — see ActionFailed.
	Failed bool
	// ActionFailed is the action key whose command was refused, empty for
	// none: that row prints the field's own failure line and disarms, the old
	// build's `ActionControl` keeping its own `failed` state rather than
	// borrowing the overlay's write line.
	ActionFailed string
}

// configOpen reports whether the overlay owns the list's band.
func (a *App) configOpen() bool { return a.config != nil && a.config.Schema != nil }

// OpenConfig opens a plugin's configuration sheet, for a caller outside (the
// shell's own tests, the deep-link handling): the same thing the gear does.
func (a *App) OpenConfig(plugin string) { a.openConfig(plugin) }

// ConfigOpen reports whether a plugin's configuration sheet is showing.
func (a *App) ConfigOpen() bool { return a.configOpen() }

// Config is the open sheet, nil when none is: the shell and the tests read the
// values the sheet is holding.
func (a *App) Config() *Config { return a.config }

// ConfigSchema is the open sheet's schema, nil when none is open: the shell
// sizes the window with it, exactly as the view does.
func (a *App) ConfigSchema() *plugincfg.Schema { return a.configSchema() }

// configSchema is the open sheet's schema, nil when none is open: the window's
// height is the sheet's own then, not the list's.
func (a *App) configSchema() *plugincfg.Schema {
	if !a.configOpen() {
		return nil
	}
	return a.config.Schema
}

// openConfig asks the shell for a plugin's configuration and shows it. A
// plugin with no declarative configuration (nil) leaves the list alone.
func (a *App) openConfig(plugin string) {
	if a.Actions.ConfigFor == nil {
		return
	}
	a.config = a.Actions.ConfigFor(plugin)
	a.FocusSearch()
}

// closeConfig hides the overlay and forgets its armed action.
func (a *App) closeConfig() {
	a.config = nil
}

// toggleConfig is the gear's own control: it opens a plugin's sheet, or closes
// it when that plugin's sheet is already open. The old build's R41 made the
// gear the one place to close what it opened — the sheet carries no close
// button of its own — so the control flips to an ✕ while the sheet shows.
func (a *App) toggleConfig(plugin string) {
	if a.configOpen() && a.config.Schema.Plugin == plugin {
		a.closeConfig()
		return
	}
	a.openConfig(plugin)
}

// ChangeConfig writes one field through the shell. The control paints
// immediately; only a write the plugin accepted replaces the values with what
// it actually stored (its own normalization has the last word), and a refused
// one rolls the control back to the last value the backend confirmed and
// reports the failure line — never keeping a value that was never stored (the
// old build's R113, `configChangeOutcome`).
//
// `a.config.Values` before the change is the committed snapshot: every path
// leaves it equal to what the backend last confirmed, so a refused write can
// restore it without a second field to drift out of step.
func (a *App) ChangeConfig(key string, value any) {
	if !a.configOpen() || a.Actions.ConfigChange == nil {
		return
	}
	committed := a.config.Values
	next := a.config.Schema.Apply(committed, key, value)
	a.config.Values = next
	a.config.Failed = false
	// A change does not touch the action row's own failure line: the old
	// build's `handleChange` and `ActionControl` are separate pieces of
	// state.
	stored := a.Actions.ConfigChange(a.config.Schema.Plugin, key, next[key])
	values, failed := plugincfg.ConfigChangeOutcome(next, committed, stored != nil)
	if failed {
		a.config.Values = values
		a.config.Failed = true
		return
	}
	// The plugin's own normalization has the last word: the sheet commits what
	// the settings actually hold, not what was asked for.
	a.config.Values = stored
}

// ArmConfig arms an action field: the first press of a destructive button only
// asks the question. The press also clears that row's failure line — the old
// build's `ActionControl` cleared `failed` as it armed, so a refused run does
// not leave the button looking broken after the user asks again.
func (a *App) ArmConfig(key string) {
	if a.configOpen() {
		a.config.Armed = key
		if a.config.ActionFailed == key {
			a.config.ActionFailed = ""
		}
	}
}

// UnarmConfig is the way back from an armed button.
func (a *App) UnarmConfig(key string) {
	if a.configOpen() && a.config.Armed == key {
		a.config.Armed = ""
	}
}

// RunConfig runs an armed action field's command. A refused run disarms and
// reports on the action's own line (`field.Failed`), never as a write failure:
// the overlay's `settings.saveFailed` line belongs to the write path (the old
// build's R113), and the old build's `ActionControl` kept the two apart.
func (a *App) RunConfig(key string) {
	if !a.configOpen() {
		return
	}
	a.config.Armed = ""
	a.config.ActionFailed = ""
	if a.Actions.ConfigAction == nil {
		// Nothing was wired to run: no command was attempted, so there is
		// nothing to report (the old build's `onRun?.()` no-op).
		return
	}
	if !a.Actions.ConfigAction(a.config.Schema.Plugin, key) {
		a.config.ActionFailed = key
	}
}

// syncConfig closes the overlay when its plugin's mode is no longer the one
// showing: leaving the mode leaves the sheet, as the old build's
// `exitPluginMode` did.
func (a *App) syncConfig() {
	if !a.configOpen() {
		return
	}
	if plugin := a.filterPluginID(); plugin != a.config.Schema.Plugin {
		a.closeConfig()
	}
}

// configBody draws the sheet in the band the list would have taken, below the
// field's row. The form itself is the settings pages' own language, which the
// shell builds (see `settingsui.PluginSheet`): one description of a plugin's
// fields, one renderer for it.
func (a *App) configBody(c *ui.Context, edge float32) {
	if a.Actions.DrawConfig != nil {
		ui.Box(c).FillWidth().Padding(edge, c.Theme().Space(1), 0, c.Theme().Space(1)).Children(func() {
			a.Actions.DrawConfig(c, a.config)
		})
	}
}

// configKeys are the overlay's own keys: Escape closes it (the old build's
// dismiss table closed the sheet before it left the mode), and the field keeps
// the keyboard.
func (a *App) configKeys(c *ui.Context) bool {
	if !a.configOpen() {
		return false
	}
	if c.Shortcut(0, ui.KeyEscape) {
		a.closeConfig()
		return true
	}
	return false
}
