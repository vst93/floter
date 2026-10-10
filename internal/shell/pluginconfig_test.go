package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/settingsui"
)

// pluginConfigApp is a shell whose clipboard history is on, so the clipboard
// mode has a gear to open its configuration with.
func pluginConfigApp(t *testing.T) *App {
	t.Helper()
	return New(Options{Store: settings.NewStore(settings.Default())})
}

// The launcher's gear opens the plugin's configuration *in the launcher*: the
// schema the plugin declares, the values its settings hold, drawn in the
// settings pages' own card language (the old build's R29 overlay). A change
// goes through the plugin's own settings, and what the store then holds is
// what the sheet shows — the plugin's normalization has the last word.
func TestPluginConfigSheetWritesThroughTheStore(t *testing.T) {
	a := pluginConfigApp(t)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 1400)
	tt.Frame()
	// Into the clipboard mode, then the gear.
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if err := tt.Click(i18n.For("en").Launcher.ConfigurePlugin); err != nil {
		t.Fatalf("the gear: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if !a.Launcher.ConfigOpen() {
		t.Fatal("the sheet did not open")
	}
	copy := i18n.For("en").Settings
	for _, want := range []string{copy.ClipboardPlugin, copy.ClipboardEnabled, copy.ClipboardClear} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	// The gear is the sheet's only close control while it is open.
	if !tt.HasText(i18n.For("en").Launcher.ConfigurePluginClose) {
		t.Errorf("the gear did not flip: %q", tt.Texts())
	}

	// The switch writes the plugin's own block, through the store.
	if err := tt.Click(copy.ClipboardEnabled); err != nil {
		t.Fatalf("the switch: %v", err)
	}
	tt.Frame()
	if state := settings.ClipboardOf(a.Store.Snapshot()); state.Enabled {
		t.Errorf("the switch did not land: %+v", state)
	}
	// And the sheet now shows what the store holds.
	if a.Launcher.Config().Values["enabled"] != false {
		t.Errorf("the sheet kept %v", a.Launcher.Config().Values["enabled"])
	}
	// Escape closes the sheet and leaves the mode.
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if a.Launcher.ConfigOpen() {
		t.Error("Escape left the sheet open")
	}
	if !a.Launcher.InClipboardMode() {
		t.Error("Escape left the mode")
	}
}

// The clear-history action runs the store's own clear, after the sheet's
// two-step confirm: the first press only arms it.
func TestPluginConfigActionClearsTheHistory(t *testing.T) {
	a := pluginConfigApp(t)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 1400)
	tt.Frame()
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if err := tt.Click(i18n.For("en").Launcher.ConfigurePlugin); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	copy := i18n.For("en").Settings
	if err := tt.Click(copy.ClipboardClear); err != nil {
		t.Fatalf("the action's first press: %v", err)
	}
	tt.Frame()
	if a.Launcher.Config().Armed != "clear_history" {
		t.Fatalf("armed = %q", a.Launcher.Config().Armed)
	}
	// The second press runs it: the history is emptied (it is empty in a
	// test, so the call is what is asserted — it reports success).
	if err := tt.Click(copy.ClipboardClearButton); err != nil {
		t.Fatalf("the action's second press: %v", err)
	}
	tt.Frame()
	if a.Launcher.Config().Armed != "" {
		t.Errorf("the button stayed armed: %q", a.Launcher.Config().Armed)
	}
	if a.Launcher.Config().Failed {
		t.Error("clearing an empty history reported a failure")
	}
}

// A plugin without a declarative configuration has no sheet: the gear is not
// drawn at all for an extension command's own list (its options are the
// manifest's).
func TestPluginConfigForAnUnknownPluginIsNil(t *testing.T) {
	a := pluginConfigApp(t)
	if config := a.pluginConfig("nonsense"); config != nil {
		t.Errorf("an unknown plugin got a sheet: %+v", config)
	}
	for _, plugin := range []string{
		settings.CustomPluginClipboard,
		settings.CustomPluginBrowser,
		settings.CustomPluginCalculator,
	} {
		config := a.pluginConfig(plugin)
		if config == nil || config.Schema == nil {
			t.Errorf("%s has no schema", plugin)
			continue
		}
		if config.Schema.Plugin != plugin {
			t.Errorf("%s's schema names %q", plugin, config.Schema.Plugin)
		}
		if config.Schema.Title == "" {
			t.Errorf("%s's sheet has no title", plugin)
		}
	}
}

// The browser sheet's target list starts with "automatic", which stays valid
// after a browser is uninstalled, and the discovered browsers follow it (the
// discovery itself is the machine's, and the schema takes whatever it is
// handed).
func TestBrowserSheetTargetsStartWithAutomatic(t *testing.T) {
	a := pluginConfigApp(t)
	config := a.pluginConfig(settings.CustomPluginBrowser)
	field, ok := config.Schema.Field("target")
	if !ok {
		t.Fatal("the browser schema has no target field")
	}
	if len(field.Options) == 0 || field.Options[0].Value != "auto" {
		t.Fatalf("target options = %+v", field.Options)
	}
	// The sheet is the settings pages' own language: the same rows and cards,
	// one renderer (see settingsui.PluginSheet).
	_ = settingsui.IntegrationsPage()
}
