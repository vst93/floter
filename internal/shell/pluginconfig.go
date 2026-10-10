package shell

import (
	"log"
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"floter/internal/settingsui"
)

// The built-in plugins' configuration, as the launcher's overlay asks for it:
// the schema a plugin declares, the values its settings hold, and what a
// change means. The launcher owns the sheet (its band, its armed action, its
// failure line); the meaning of a field is here, where the settings are.
//
// The old build's R29 put the same knowledge in `plugins/config-schema.ts` and
// `plugins/config-persist.ts`: the schema is the contract, and the write path
// is one function per plugin, never the whole settings object.

// pluginConfig is a plugin's declarative configuration: its schema (the
// browser's target list filled from the machine) and its current values. Nil
// for a plugin that has no declarative configuration — there is nothing to
// show and no settings button.
func (a *App) pluginConfig(plugin string) *launcher.Config {
	s := a.Store.Snapshot()
	copy := i18n.For(s.Language).Settings
	var schema *plugincfg.Schema
	var values plugincfg.Values
	switch plugin {
	case settings.CustomPluginClipboard:
		schema, values = plugincfg.Clipboard(copy, settings.ClipboardOf(s))
	case settings.CustomPluginCalculator:
		schema, values = plugincfg.Calculator(copy, settings.CalculatorPluginOf(s))
	case settings.CustomPluginBrowser:
		schema, values = plugincfg.Browser(copy, settings.BrowserPluginOf(s), a.browserTargets())
	default:
		return nil
	}
	return &launcher.Config{Schema: schema, Values: values}
}

// changePluginConfig writes one field of a plugin's configuration, and answers
// with the values the plugin's settings hold afterwards. Nil means the write
// was refused: the sheet keeps the value it painted and says so, rather than
// pretending a change that was never stored.
func (a *App) changePluginConfig(plugin, key string, value any) plugincfg.Values {
	err := a.Store.Update(func(s *settings.Settings) {
		switch plugin {
		case settings.CustomPluginClipboard:
			state := settings.ClipboardOf(*s)
			switch key {
			case "enabled":
				state.Enabled = value == true
			case "max_items":
				state.MaxItems = int(asNumber(value))
			}
			s.SetClipboard(state)
		case settings.CustomPluginCalculator:
			state := settings.CalculatorPluginOf(*s)
			switch key {
			case "max_items":
				state.MaxItems = int(asNumber(value))
			case "retention_days":
				state.RetentionDays = int(asNumber(text(value)))
			case "copy_mode":
				state.CopyMode = text(value)
			}
			s.SetCalculatorPlugin(state)
		case settings.CustomPluginBrowser:
			state := settings.BrowserPluginOf(*s)
			switch key {
			case "enabled":
				state.Enabled = value == true
			case "target":
				state.Target = text(value)
			case "custom_base_dir":
				state.CustomBaseDir = strings.TrimSpace(text(value))
			case "history_days":
				state.HistoryDays = int(asNumber(value))
			case "sort_order":
				state.SortOrder = text(value)
			case "search_fields":
				state.SearchField = text(value)
			case "cdp_enabled":
				state.CDPEnabled = value == true
			case "cdp_port":
				state.CDPPort = int(asNumber(value))
			}
			s.SetBrowserPlugin(state)
		}
	})
	if err != nil {
		log.Printf("floter: could not save the %s configuration: %v", plugin, err)
		return nil
	}
	// The plugin's own normalization has the last word: the sheet is handed
	// what the settings actually hold, not what was asked for.
	if config := a.pluginConfig(plugin); config != nil {
		return config.Values
	}
	return nil
}

// runPluginAction runs an action field's command. The sheet's own two-step
// confirm has already asked (the old build's R38 kept the overlay out of the
// system dialog business), so the command runs directly.
func (a *App) runPluginAction(plugin, key string) bool {
	switch {
	case plugin == settings.CustomPluginClipboard && key == "clear_history":
		return a.clearClipboardHistory()
	case plugin == settings.CustomPluginCalculator && key == "clear_history":
		return a.clearCalculatorHistory()
	}
	return false
}

// clearClipboardHistory drops every entry that is not a favourite.
func (a *App) clearClipboardHistory() bool {
	if a.Clipboard == nil {
		return false
	}
	if err := a.Clipboard.Clear(); err != nil {
		log.Printf("floter: could not clear the clipboard history: %v", err)
		return false
	}
	a.invalidate()
	return true
}

// clearCalculatorHistory drops every calculation that is not a favourite.
func (a *App) clearCalculatorHistory() bool {
	if a.Calculator == nil {
		return false
	}
	if err := a.Calculator.Clear(); err != nil {
		log.Printf("floter: could not clear the calculator history: %v", err)
		return false
	}
	a.invalidate()
	return true
}

// invalidate asks for a frame, so a change the stores made shows.
func (a *App) invalidate() {
	if a.Win != nil {
		a.Win.Invalidate()
	}
}

// drawPluginConfig draws the open sheet: the settings pages' own card
// language, over the launcher's band.
func (a *App) drawPluginConfig(c *ui.Context, config *launcher.Config) {
	if config == nil {
		return
	}
	a.Settings.PluginSheet(c, settingsui.PluginSheet{
		Schema: config.Schema,
		Values: config.Values,
		Armed:  config.Armed,
		Failed: config.Failed,
		Change: a.Launcher.ChangeConfig,
		Arm:    a.Launcher.ArmConfig,
		Run:    a.Launcher.RunConfig,
		Unarm:  a.Launcher.UnarmConfig,
	})
}

// asNumber reads a value the sheet's bounded controls send: a slider and a
// number send a float, a select sends the text its option names.
func asNumber(value any) float64 {
	switch n := value.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		var out float64
		for _, r := range n {
			if r < '0' || r > '9' {
				return out
			}
			out = out*10 + float64(r-'0')
		}
		return out
	}
	return 0
}

// text reads a value the sheet's textual controls send.
func text(value any) string {
	value, _ = value.(string)
	out, _ := value.(string)
	return out
}
