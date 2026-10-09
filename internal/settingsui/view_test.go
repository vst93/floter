package settingsui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

func newStore(t *testing.T) *settings.Store {
	t.Helper()
	return settings.NewStore(settings.Default())
}

// newStoreFor is newStore for a benchmark, which has no *testing.T.
func newStoreFor(b *testing.B) *settings.Store {
	b.Helper()
	return settings.NewStore(settings.Default())
}

func render(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
	}, w, h)
	tt.Frame()
	return tt
}

func TestSettingsShowsThePagesAndTheGeneralControls(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 580)

	for _, want := range []string{
		"Settings", "General", "Sessions", "Shortcuts", "Plugins", "Integrations", "About",
		"Appearance", "Language", "Liquid glass effect", "App transparency",
		"Terminal transparency", "Interface size",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestSettingsControlsWriteThroughTheStore(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 580)

	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().Theme; got != "dark" {
		t.Errorf("theme = %q, want dark", got)
	}

	if err := tt.Click("Liquid Max"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().GlassStep; got != "liquid" {
		t.Errorf("glass step = %q, want liquid", got)
	}

	if err := tt.Click("Large"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().UIScale; got != "large" {
		t.Errorf("ui scale = %q, want large", got)
	}

	// The language switch is last: it relabels every control.
	if err := tt.Click("中文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().Language; got != "zh" {
		t.Errorf("language = %q, want zh", got)
	}
	if !tt.HasText("设置") {
		t.Errorf("the panel did not switch language: %q", tt.Texts())
	}
}

func TestSettingsOpacityShowsAndFollowsTheStore(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 580)

	if !tt.HasText("47%") || !tt.HasText("46%") {
		t.Errorf("opacity values missing: %q", tt.Texts())
	}
	if err := store.Update(func(s *settings.Settings) { s.MainOpacity = 80 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("80%") {
		t.Errorf("the control did not follow the store: %q", tt.Texts())
	}
}

func TestSetWritesThroughAndClamps(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	a.set(func(s *settings.Settings) { s.MainOpacity = 250 })
	if got := store.Snapshot().MainOpacity; got != settings.MaxWindowOpacity {
		t.Errorf("opacity = %d, want the %d ceiling", got, settings.MaxWindowOpacity)
	}
	a.set(func(s *settings.Settings) { s.MainOpacity = 1 })
	if got := store.Snapshot().MainOpacity; got != settings.MinWindowOpacity {
		t.Errorf("opacity = %d, want the %d floor", got, settings.MinWindowOpacity)
	}
	for _, tc := range []struct {
		in   float64
		want uint8
	}{{47.4, 47}, {47.6, 48}, {0, settings.MinWindowOpacity}, {250, settings.MaxWindowOpacity}} {
		if got := clampPercent(tc.in); got != tc.want {
			t.Errorf("clampPercent(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSettingsSidebarRoutesAndEscapeCloses(t *testing.T) {
	closed := 0
	a := New(newStore(t), Actions{Close: func() { closed++ }})
	tt := render(t, a, 720, 580)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Page != PageIntegrations {
		t.Fatalf("page = %d, want integrations", a.Page)
	}
	if !tt.HasText("No integrations are installed.") {
		t.Errorf("the empty integrations state did not show: %q", tt.Texts())
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if closed != 1 || a.Page != PageIntegrations {
		t.Errorf("Escape closed %d times, page %d", closed, a.Page)
	}
}

func TestTerminalAppearanceControls(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 620)

	for _, want := range []string{
		"Terminal appearance", "Font size", "Font family", "Cursor shape",
		"Blinking cursor", "Line height", "Padding", "Terminal palette",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// The terminal group sits below the fold: scroll the body to reach it.
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()

	if err := tt.Click("Block"); err != nil {
		t.Fatalf("cursor shape: %v", err)
	}
	tt.Frame()
	if got := store.Snapshot().CursorShape; got != "block" {
		t.Errorf("cursor shape = %q, want block", got)
	}

	if err := tt.Click("Blinking cursor"); err != nil {
		t.Fatalf("blink: %v", err)
	}
	tt.Frame()
	if store.Snapshot().CursorBlink {
		t.Error("the blink switch did not turn off")
	}

	if err := tt.Click("Relaxed"); err != nil {
		t.Fatalf("padding: %v", err)
	}
	tt.Frame()
	if got := store.Snapshot().TerminalPadding; got != "relaxed" {
		t.Errorf("padding = %q, want relaxed", got)
	}

	if err := tt.Click("Terminal palette"); err != nil {
		t.Fatalf("palette trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Forest"); err != nil {
		t.Fatalf("palette option: %v", err)
	}
	tt.Frame()
	if got := store.Snapshot().TerminalTheme; got != "forest" {
		t.Errorf("palette = %q, want forest", got)
	}
}

func TestSessionsPage(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 580)

	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("No sessions are running.") {
		t.Errorf("missing the empty session state: %q", tt.Texts())
	}

	closed := 0
	a.Actions.CloseSession = func() { closed++ }
	a.Sessions = func() []Session {
		return []Session{{Title: "zsh — floter", Running: true}}
	}
	tt.Frame()
	if !tt.HasText("zsh — floter") || !tt.HasText("Running") {
		t.Errorf("the session did not show: %q", tt.Texts())
	}
	if err := tt.Click("Close session"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if closed != 1 {
		t.Errorf("close session ran %d times, want 1", closed)
	}
}

func TestShortcutsPage(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Shortcut = "Alt+Space"
	tt := render(t, a, 720, 580)

	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Show / hide floter") || !tt.HasText("Alt + Space") {
		t.Errorf("the shortcut did not show: %q", tt.Texts())
	}
	if !tt.HasText("Record") {
		t.Errorf("the recorder is missing: %q", tt.Texts())
	}
	if !tt.HasText("The global keys floter answers, and the ones you bind yourself.") {
		t.Errorf("the hint did not show: %q", tt.Texts())
	}
}

func TestAboutPage(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.About = About{
		Name:         "floter",
		Version:      "0.3.14",
		Framework:    "mygo 0.3.5",
		Scheme:       "floter://",
		SettingsPath: "/Users/v/Library/Application Support/floter/settings.json",
		RepoURL:      "https://github.com/vst93/floter",
	}
	tt := render(t, a, 720, 580)

	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"floter 0.3.14", "0.3.14", "mygo 0.3.5", "floter://",
		"/Users/v/Library/Application Support/floter/settings.json",
		"https://github.com/vst93/floter",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	if err := tt.Click("https://github.com/vst93/floter"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != "https://github.com/vst93/floter" {
		t.Errorf("opened URLs = %v", urls)
	}
}

func TestWindowBehaviourControls(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 620)

	tt.Scroll(400, 300, 0, 400)
	tt.Frame()

	if err := tt.Click("Hide when focus is lost"); err != nil {
		t.Fatalf("hide on blur: %v", err)
	}
	tt.Frame()
	if store.Snapshot().HideOnBlur {
		t.Error("the hide-on-blur check box did not turn off")
	}

	// The residency control shows what is stored; a custom value the
	// presets do not name shows as itself, so the control never lies.
	if err := store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 45 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("45 s") {
		t.Errorf("the custom residency did not show: %q", tt.Texts())
	}
	if err := store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 120 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("120 s") {
		t.Errorf("the preset residency did not show: %q", tt.Texts())
	}
}

func TestResidencyOptions(t *testing.T) {
	copy := i18n.For("en").Settings
	options := residencyOptions(copy, 10)
	if got := options[1].Label; got != "10 s" {
		t.Errorf("first preset = %q", got)
	}
	last := options[len(options)-1]
	if last.ID != strconv.FormatUint(uint64(settings.SurfaceResidencyNever), 10) || last.Label != "Never" {
		t.Errorf("last option = %+v", last)
	}

	// A custom value is prepended, and parsing round-trips every id.
	custom := residencyOptions(copy, 45)
	if custom[0].Label != "45 s" {
		t.Errorf("custom first option = %+v", custom[0])
	}
	for _, option := range custom {
		seconds, ok := parseResidency(option.ID)
		if !ok {
			t.Errorf("option id %q did not parse", option.ID)
		}
		if option.ID == custom[0].ID && seconds != 45 {
			t.Errorf("custom id parsed as %d", seconds)
		}
	}
	if _, ok := parseResidency("soon"); ok {
		t.Error("a non-numeric id parsed")
	}
}

func TestIntegrationsPage(t *testing.T) {
	toggled := ""
	toggledTo := false
	removedID, removedName := "", ""
	a := New(newStore(t), Actions{
		SetIntegrationEnabled: func(id string, enabled bool) { toggled, toggledTo = id, enabled },
		UninstallIntegration:  func(id, name string) { removedID, removedName = id, name },
	})
	a.Integrations = func() []Integration {
		return []Integration{
			{
				ID: "io.github.vst93.v", Name: "V Tools", Description: "Developer tools",
				Publisher: "vst", Version: "0.0.12", ToolVersion: "0.0.12",
				Enabled: true, Running: true,
			},
			{
				ID: "dev.floter.static", Name: "Static Tools", Publisher: "floter",
				Version: "1.0.0", Enabled: false,
			},
			{
				ID: "bad.tool", Name: "Bad", Enabled: true, Broken: true, Error: "provider exploded",
			},
			{ID: "orphan.pkg", Name: "orphan.pkg", Orphan: true},
		}
	}
	tt := render(t, a, 720, 620)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"V Tools", "vst · 0.0.12 · io.github.vst93.v", "Running",
		"Static Tools", "Disabled", "Bad", "Broken", "provider exploded",
		"orphan.pkg", "Installed on disk, not recorded",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// The Running row's check box turns the integration off.
	if err := tt.Click("Enabled"); err != nil {
		t.Fatalf("the enable check box is missing: %v", err)
	}
	tt.Frame()
	if toggled != "io.github.vst93.v" || toggledTo {
		t.Errorf("toggled %q to %v", toggled, toggledTo)
	}
	// The first row's Uninstall button asks the shell to remove it.
	if err := tt.Click("Uninstall"); err != nil {
		t.Fatalf("the uninstall button is missing: %v", err)
	}
	tt.Frame()
	if removedID != "io.github.vst93.v" || removedName != "V Tools" {
		t.Errorf("uninstall recorded %q/%q", removedID, removedName)
	}

	// An orphan has no switch: clicking its name does nothing.
	before := toggled
	if err := tt.Click("orphan.pkg"); err == nil {
		tt.Frame()
	}
	if toggled != before {
		t.Errorf("an orphan toggled %q", toggled)
	}
}

func TestLaunchAtStartupControl(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 620)

	tt.Scroll(400, 300, 0, 400)
	tt.Frame()
	if !tt.HasText("Launch at startup") {
		t.Fatalf("the control is missing: %q", tt.Texts())
	}
	if err := tt.Click("Launch at startup"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !store.Snapshot().LaunchAtStartup {
		t.Error("the switch did not write the setting")
	}
}

func TestInstallFromRegistryControl(t *testing.T) {
	installed := ""
	constraint := ""
	a := New(newStore(t), Actions{
		InstallFromRegistry: func(name, version string) { installed, constraint = name, version },
	})
	a.Integrations = func() []Integration { return nil }
	tt := render(t, a, 720, 620)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The field's contents are the app's own state, so a test can set them
	// the way typing would.
	a.installName = "@vst93/floter-v"
	a.installVersion = "^1.0.0"
	tt.Frame()

	if err := tt.Click("Install"); err != nil {
		t.Fatalf("the install button is missing: %v", err)
	}
	tt.Frame()
	if installed != "@vst93/floter-v" || constraint != "^1.0.0" {
		t.Errorf("install recorded %q / %q", installed, constraint)
	}
	// The fields clear for the next install.
	if a.installName != "" || a.installVersion != "" {
		t.Errorf("the fields were not cleared: %q / %q", a.installName, a.installVersion)
	}

	// An empty name asks for nothing.
	if err := tt.Click("Install"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if constraint != "^1.0.0" {
		t.Errorf("an empty request was sent: %q", installed)
	}
}

func TestAppIconReader(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})

	// The shipped icon is dark, and anything unknown lands on it.
	if got := a.appIcon(); got != "dark" {
		t.Errorf("default = %q, want dark", got)
	}
	if err := store.Update(func(s *settings.Settings) { s.SetExtra("app_icon", "light") }); err != nil {
		t.Fatal(err)
	}
	if got := a.appIcon(); got != "light" {
		t.Errorf("stored = %q, want light", got)
	}
	if err := store.Update(func(s *settings.Settings) { s.SetExtra("app_icon", "purple") }); err != nil {
		t.Fatal(err)
	}
	if got := a.appIcon(); got != "dark" {
		t.Errorf("unknown = %q, want dark", got)
	}

	// The control is on the page, with both appearances offered.
	tt := render(t, a, 720, 620)
	if !tt.HasText("App icon") {
		t.Errorf("the control is missing: %q", tt.Texts())
	}
	if !tt.HasText("The icon shown in the menu bar / tray and on the taskbar. Dark is the default.") {
		t.Errorf("the hint is missing: %q", tt.Texts())
	}
}

func TestIntegrationPermissionsLine(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Permissions: []string{"filesystem-read", "process-spawn"},
			Enforced:    map[string]bool{"process-spawn": true},
		}}
	}
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The line names every permission, marking the enforced one.
	if !tt.HasText("Read files") {
		t.Errorf("a permission is missing: %q", tt.Texts())
	}
	if !tt.HasText("Start processes (Floter enforces)") {
		t.Errorf("the enforced permission is not marked: %q", tt.Texts())
	}
	// The dialog copy is localized too.
	zh := New(settings.NewStore(settings.Settings{Language: "zh"}), Actions{})
	if got := zh.permissionLine(Integration{Permissions: []string{"process-spawn"}, Enforced: map[string]bool{"process-spawn": true}}, i18n.For("zh").Settings); !strings.Contains(got, "启动进程") || !strings.Contains(got, "强制拦截") {
		t.Errorf("zh line = %q", got)
	}
}

func TestShortcutRecorder(t *testing.T) {
	recorded := ""
	a := New(newStore(t), Actions{
		SetShortcut: func(id, accelerator string) { recorded = id + "=" + accelerator },
	})
	a.Shortcut, a.ShortcutID = "Alt+Space", "toggle_window"
	tt := render(t, a, 720, 620)

	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The row shows the current key, and Record starts the recorder.
	if !tt.HasText("Alt + Space") {
		t.Fatalf("the current shortcut is missing: %q", tt.Texts())
	}
	if err := tt.Click("Record"); err != nil {
		t.Fatalf("the record button is missing: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Press keys…") {
		t.Fatalf("the recorder did not start: %q", tt.Texts())
	}

	// A combination with a modifier is recorded...
	tt.Key(ui.Super, ui.KeyG)
	tt.Frame()
	if recorded != "toggle_window=Cmd+G" {
		t.Errorf("recorded %q", recorded)
	}
	if !a.recording == false {
		t.Error("the recorder stayed on")
	}

	// ...and Escape cancels without recording.
	recorded = ""
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if recorded != "" {
		t.Errorf("Escape recorded %q", recorded)
	}
	if !tt.HasText("Record") {
		t.Errorf("the recorder did not close: %q", tt.Texts())
	}

	// A key without a modifier is not a shortcut, and the recorder waits.
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyG)
	tt.Frame()
	if recorded != "" || !a.recording {
		t.Errorf("a bare key recorded %q, recording=%v", recorded, a.recording)
	}
}

// The Plugins page shows the built-in plugins' settings and writes every
// change through the store.
func TestPluginsPageWritesThroughTheStore(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 620)

	if err := tt.Click("Plugins"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"Browser history", "Clipboard history", "Search browser data", "History window", "Order"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// The browser switch writes the plugin's block.
	if err := tt.Click("Search browser data"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if plugin := settings.BrowserPluginOf(store.Snapshot()); plugin.Enabled {
		t.Errorf("the browser switch did not land: %+v", plugin)
	}

	// The order picker opens on its trigger and stores the choice.
	if err := tt.Click("Order"); err != nil {
		t.Fatalf("order trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Most visited"); err != nil {
		t.Fatalf("order option: %v", err)
	}
	tt.Frame()
	if plugin := settings.BrowserPluginOf(store.Snapshot()); plugin.SortOrder != "visits" {
		t.Errorf("sort order = %q", plugin.SortOrder)
	}

	// The clipboard group sits below the fold: scroll the body to reach it.
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()
	if err := tt.Click("Keep a clipboard history"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if state := settings.ClipboardOf(store.Snapshot()); state.Enabled {
		t.Errorf("the clipboard switch did not land: %+v", state)
	}
	tt.Scroll(400, 300, 0, -900)
	tt.Frame()

	// The target picker offers the automatic choice plus the discovered
	// browsers, and stores the id.
	a.Actions.BrowserTargets = func() []i18n.Option {
		return []i18n.Option{{ID: "brave", Label: "Brave"}}
	}
	tt.Frame()
	if !tt.HasText("Automatic") {
		t.Fatalf("the automatic target is missing: %q", tt.Texts())
	}
	if err := tt.Click("Automatic"); err != nil {
		t.Fatalf("target trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Brave"); err != nil {
		t.Fatalf("target option: %v", err)
	}
	tt.Frame()
	if plugin := settings.BrowserPluginOf(store.Snapshot()); plugin.Target != "brave" {
		t.Errorf("target = %q", plugin.Target)
	}
}

// The Integrations page lists one switch per command and reports a toggle.
func TestIntegrationCommandSwitches(t *testing.T) {
	store := newStore(t)
	toggled := [][3]string{}
	a := New(store, Actions{
		SetCommandEnabled: func(extensionID, commandID string, enabled bool) {
			toggled = append(toggled, [3]string{extensionID, commandID, strconv.FormatBool(enabled)})
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Commands: []Command{
				{ID: "jv", Name: "Just run", Enabled: true, Available: true},
				{ID: "build", Name: "Build", Enabled: false},
			},
		}}
	}
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Just run") || !tt.HasText("Build") {
		t.Fatalf("the command switches = %q", tt.Texts())
	}
	// A command whose runtime does not resolve says so rather than lying
	// about being ready.
	if !tt.HasText("Build  \u00b7  runtime unavailable") {
		t.Errorf("the unavailable note is missing: %q", tt.Texts())
	}
	if err := tt.Click("Build  \u00b7  runtime unavailable"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(toggled) != 1 || toggled[0] != [3]string{"io.github.vst93.v", "build", "true"} {
		t.Errorf("toggled = %v", toggled)
	}
}

// Switching pages records the choice, so the next visit reopens the page.
func TestPageSelectionIsRemembered(t *testing.T) {
	pages := []string{}
	a := New(newStore(t), Actions{SetPage: func(name string) { pages = append(pages, name) }})
	tt := render(t, a, 720, 580)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(pages) != 1 || pages[0] != "about" {
		t.Errorf("pages = %v", pages)
	}
	if page, ok := PageByName("plugins"); !ok || page != PagePlugins {
		t.Errorf("PageByName(plugins) = %d, %v", page, ok)
	}
	if _, ok := PageByName("nope"); ok {
		t.Error("an unknown page id resolved")
	}
	if got := PageName(PageIntegrations); got != "integrations" {
		t.Errorf("PageName = %q", got)
	}
}

// The Shortcuts page lists the custom shortcuts and can add, retarget and
// remove them.
func TestCustomShortcutsPage(t *testing.T) {
	store := newStore(t)
	entries := []settings.CustomShortcut{{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}}
	rejections := []CustomShortcutRejection{}
	var committed [][]settings.CustomShortcut
	a := New(store, Actions{
		CustomShortcuts: func() []settings.CustomShortcut { return entries },
		SetCustomShortcuts: func(list []settings.CustomShortcut) []CustomShortcutRejection {
			committed = append(committed, list)
			entries = list
			return rejections
		},
	})
	tt := render(t, a, 720, 620)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Custom shortcuts") || !tt.HasText("Cmd + Shift + P") {
		t.Fatalf("the custom section = %q", tt.Texts())
	}

	// Recording a key fills the new row, and Add persists the binding.
	if err := tt.Click("Record key"); err != nil {
		t.Fatalf("record: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Press keys\u2026") {
		t.Fatalf("the recorder did not open: %q", tt.Texts())
	}
	tt.Key(ui.Super, ui.KeyJ)
	tt.Frame()
	if len(committed) != 0 {
		t.Fatalf("a half-made row was persisted: %+v", committed)
	}
	if err := tt.Click("Add"); err != nil {
		t.Fatalf("add: %v", err)
	}
	tt.Frame()
	if len(committed) != 1 || len(committed[0]) != 2 {
		t.Fatalf("committed = %+v", committed)
	}
	if committed[0][1].Action != "plugin:clipboard" {
		t.Errorf("the new row's action = %q", committed[0][1].Action)
	}

	// Retargeting a row writes the new action; removing one drops it.
	if err := tt.Click("Clipboard history"); err != nil {
		t.Fatalf("action trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Browser history"); err != nil {
		t.Fatalf("action option: %v", err)
	}
	tt.Frame()
	last := committed[len(committed)-1]
	if last[0].Action != "plugin:browser" {
		t.Errorf("retargeted action = %q", last[0].Action)
	}
	if err := tt.Click("Remove"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	tt.Frame()
	last = committed[len(committed)-1]
	if len(last) != 1 {
		t.Errorf("after a removal = %+v", last)
	}

	// A key the system refused is reported rather than looking bound.
	rejections = []CustomShortcutRejection{{Key: "Cmd+J", Reason: "taken"}}
	if err := tt.Click("Remove"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Could not bind Cmd+J: taken") {
		t.Errorf("the rejection is missing: %q", tt.Texts())
	}
}

// The Plugins page carries the calculator's card and writes its three fields
// through the store.
func TestCalculatorCardWritesThrough(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 640)
	if err := tt.Click("Plugins"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()
	for _, want := range []string{"Calculator", "History size", "Keep for", "Enter copies"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// The copy mode is a segmented choice, so it writes on click.
	if err := tt.Click("Result only"); err != nil {
		t.Fatalf("copy mode: %v", err)
	}
	tt.Frame()
	if plugin := settings.CalculatorPluginOf(store.Snapshot()); plugin.CopyMode != settings.CalculatorCopyResult {
		t.Errorf("copy mode = %q", plugin.CopyMode)
	}

	// The retention picker opens on its trigger and stores the window.
	if err := tt.Click("30 days"); err != nil {
		t.Fatalf("retention trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("7 days"); err != nil {
		t.Fatalf("retention option: %v", err)
	}
	tt.Frame()
	if plugin := settings.CalculatorPluginOf(store.Snapshot()); plugin.RetentionDays != 7 {
		t.Errorf("retention = %d", plugin.RetentionDays)
	}
	// The picker offers every window, including the never-expire one.
	if err := tt.Click("7 days"); err != nil {
		t.Fatalf("retention trigger: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Never expire") || !tt.HasText("1 day") {
		t.Errorf("the windows are missing from the picker: %q", tt.Texts())
	}
}

// An orphan package directory is listed with the two operations it can take,
// and nothing that needs a repository record.
func TestOrphanRowsOfferAdoptAndDelete(t *testing.T) {
	adopted := []string{}
	deleted := []string{}
	a := New(newStore(t), Actions{
		AdoptIntegration: func(id string) { adopted = append(adopted, id) },
		DeleteOrphan:     func(id string) { deleted = append(deleted, id) },
	})
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "dev.floter.installed", Name: "Installed", Enabled: true, Running: true},
			{ID: "dev.floter.orphan", Name: "dev.floter.orphan", Orphan: true},
		}
	}
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("dev.floter.orphan") {
		t.Fatalf("the orphan is missing: %q", tt.Texts())
	}
	if !tt.HasText("Adopt") || !tt.HasText("Delete") {
		t.Fatalf("the orphan operations are missing: %q", tt.Texts())
	}
	if err := tt.Click("Adopt"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(adopted) != 1 || adopted[0] != "dev.floter.orphan" {
		t.Errorf("adopted = %v", adopted)
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(deleted) != 1 || deleted[0] != "dev.floter.orphan" {
		t.Errorf("deleted = %v", deleted)
	}
	// The orphan has no enable switch: there is no record to enable.
	if tt.HasText("Search browser data") {
		t.Error("an unrelated control appeared")
	}
}

// The About page offers the update check, and shows what it found.
func TestAboutUpdateCheck(t *testing.T) {
	checked, installed := 0, 0
	a := New(newStore(t), Actions{
		CheckForUpdates: func() { checked++ },
		InstallUpdate:   func() { installed++ },
	})
	a.About = About{Name: "floter", Version: "0.3.14"}
	tt := render(t, a, 720, 620)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Check for updates"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if checked != 1 {
		t.Fatalf("checked %d times", checked)
	}
	// The button leaves a line behind while the check runs.
	if !tt.HasText("Checking\u2026") {
		t.Errorf("the status line is missing: %q", tt.Texts())
	}

	// An update that was found offers to install it.
	a.UpdateStatus = "floter 0.4.0 is available"
	a.UpdateReady = true
	tt.Frame()
	if !tt.HasText("Download and install") {
		t.Fatalf("the install button is missing: %q", tt.Texts())
	}
	if err := tt.Click("Download and install"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if installed != 1 {
		t.Errorf("installed %d times", installed)
	}
}
