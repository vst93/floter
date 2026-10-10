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
	// Failed is the one line under the fields: the last write or action was
	// refused.
	Failed bool
}

// configOpen reports whether the overlay owns the list's band.
func (a *App) configOpen() bool { return a.config != nil && a.config.Schema != nil }

// ConfigOpen reports whether a plugin's configuration sheet is showing.
func (a *App) ConfigOpen() bool { return a.configOpen() }

// Config is the open sheet, nil when none is: the shell and the tests read the
// values the sheet is holding.
func (a *App) Config() *Config { return a.config }

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
// one reports the failure line instead of keeping a value that was never
// saved.
func (a *App) ChangeConfig(key string, value any) {
	if !a.configOpen() || a.Actions.ConfigChange == nil {
		return
	}
	next := a.config.Schema.Apply(a.config.Values, key, value)
	a.config.Values = next
	a.config.Failed = false
	stored := a.Actions.ConfigChange(a.config.Schema.Plugin, key, next[key])
	if stored == nil {
		a.config.Failed = true
		return
	}
	a.config.Values = stored
}

// ArmConfig arms an action field: the first press of a destructive button only
// asks the question.
func (a *App) ArmConfig(key string) {
	if a.configOpen() {
		a.config.Armed = key
	}
}

// UnarmConfig is the way back from an armed button.
func (a *App) UnarmConfig(key string) {
	if a.configOpen() && a.config.Armed == key {
		a.config.Armed = ""
	}
}

// RunConfig runs an armed action field's command.
func (a *App) RunConfig(key string) {
	if !a.configOpen() {
		return
	}
	a.config.Armed = ""
	a.config.Failed = false
	if a.Actions.ConfigAction == nil {
		return
	}
	if !a.Actions.ConfigAction(a.config.Schema.Plugin, key) {
		a.config.Failed = true
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
		ui.Box(c).FillWidth().Padding(edge, 0, 0, 0).Children(func() {
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
