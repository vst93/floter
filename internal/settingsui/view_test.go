package settingsui

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

func newStore(t *testing.T) *settings.Store {
	t.Helper()
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
		"Settings", "General", "Sessions", "Shortcuts", "Integrations", "About",
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
	if !tt.HasText("Show / hide floter") || !tt.HasText("Alt+Space") {
		t.Errorf("the shortcut did not show: %q", tt.Texts())
	}
	if !tt.HasText("More shortcuts arrive with the system integration.") {
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
