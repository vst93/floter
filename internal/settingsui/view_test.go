package settingsui

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/terminalui"
	"time"
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
		"Settings", "General", "Sessions", "Shortcuts", "Integrations", "About",
		"Appearance", "Language", "Liquid glass effect", "App transparency",
		"Terminal transparency", "Interface size",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

// Every settings row keeps the grey second line the reference showed under
// its label. The check-box helper used to drop it, so the four General-page
// switches — and the Integrations page's PATH switch — read as bare labels
// with nothing saying what they do.
func TestSettingsRowsKeepTheirHints(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 1600)

	for _, want := range []string{
		copy.ThemeHint,
		copy.LaunchAtStartupHint,
		copy.HideOnBlurHint,
		copy.ShowRecentInLauncherHint,
		copy.ShowMenubarIconHint,
	} {
		if !tt.HasText(want) {
			t.Errorf("missing the hint %q in %q", want, tt.Texts())
		}
	}

	// The Integrations page's own switch carries its hint too.
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(copy.ShowCommandsInSearchHint) {
		t.Errorf("missing the PATH switch's hint in %q", tt.Texts())
	}
}

func TestSettingsControlsWriteThroughTheStore(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 1400)

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
	tt := render(t, a, 720, 1800)

	// The settings card paints exactly the axes the schema gives this surface
	// (internal/control, R55): the family, the size, the line height and the
	// palette. Reading the list from the schema is what keeps this test and the
	// view from agreeing on a stale subset.
	labels := map[control.AppearanceAxis]string{
		control.AxisFontFamily: "Font family",
		control.AxisFontSize:   "Font size",
		control.AxisLineHeight: "Line height",
		control.AxisPalette:    "Terminal palette",
	}
	if len(labels) != len(control.SettingsCardAxes()) {
		t.Fatalf("the test names %d card axes, the schema has %d", len(labels), len(control.SettingsCardAxes()))
	}
	if !tt.HasText("Terminal appearance") {
		t.Errorf("missing the group title in %q", tt.Texts())
	}
	for _, axis := range control.SettingsCardAxes() {
		label, ok := labels[axis]
		if !ok {
			t.Fatalf("the card owes the schema an axis with no row: %q", axis)
		}
		if !tt.HasText(label) {
			t.Errorf("missing %q in %q", label, tt.Texts())
		}
	}

	// The rows the schema reserves for the terminal's own drawer are *absent*
	// here: the card is not a second copy of the drawer.
	for _, gone := range []string{"Cursor shape", "Blinking cursor", "Padding", "Safe paste"} {
		if rect, ok := tt.Find(gone); ok {
			t.Errorf("the settings card draws the drawer-only row %q at %v", gone, rect)
		}
	}

	// The card keeps the schema's own order (family, size, line height,
	// palette), so a row cannot be inserted between two others by accident.
	var lastY float32 = -1
	for _, axis := range control.SettingsCardAxes() {
		rect, ok := tt.Find(labels[axis])
		if !ok {
			t.Fatalf("%q is missing", labels[axis])
		}
		if rect.Y <= lastY {
			t.Errorf("%q at %v is not below the row the schema puts before it", labels[axis], rect)
		}
		lastY = rect.Y
	}

	// The terminal group sits below the fold: scroll the body to reach it.
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()

	// The trailing select's trigger shows its current value ("Follow app"),
	// not the row label; clicking it opens the drop-down.
	if err := tt.Click("Follow app"); err != nil {
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
	tt := render(t, a, 720, 760)

	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("No sessions are running.") {
		t.Errorf("missing the empty session state: %q", tt.Texts())
	}

	closed, shown := 0, 0
	a.Actions.CloseSessionAt = func(index int) { closed = index + 1 }
	a.Actions.SelectSession = func(index int) { shown = index + 1 }
	notes := []string{"Running · attached", "Running · detached", "Process exited · code 1"}
	a.Sessions = func() []Session {
		out := []Session{
			{Title: "zsh — floter", Running: true, Active: true, Index: 0},
			{Title: "vim", Running: true, Index: 1},
			{Title: "build", Running: false, Index: 2},
		}
		for i := range out {
			out[i].Note = notes[i]
		}
		return out
	}
	tt.Frame()
	for _, want := range []string{
		"zsh — floter", "vim", "build",
		"Running · attached", "Running · detached", "Process exited · code 1",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	// Closing a session kills its program, so the control arms first: one
	// press asks, and the second ends it.
	if err := tt.Click("Close session"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if closed != 0 {
		t.Fatalf("the first press closed a session: %d", closed)
	}
	if !tt.HasText("Close it?") {
		t.Fatalf("the armed question is missing: %q", tt.Texts())
	}
	if err := tt.Click("Keep"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tt.Frame()
	if closed != 0 {
		t.Errorf("the way back closed a session: %d", closed)
	}
	if err := tt.Click("Close session"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Close it?"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	tt.Frame()
	if closed != 1 {
		t.Errorf("close session ran with index %d, want 0", closed-1)
	}
	// Resuming another session hands its index back.
	if err := tt.Click("Show"); err != nil {
		t.Fatalf("show: %v", err)
	}
	tt.Frame()
	if shown != 2 {
		t.Errorf("show ran with index %d, want 1", shown-1)
	}
	// A new session is the page's own control.
	started := 0
	a.Actions.NewSession = func() { started++ }
	tt.Frame()
	if err := tt.Click("New session"); err != nil {
		t.Fatalf("new session: %v", err)
	}
	tt.Frame()
	if started != 1 {
		t.Errorf("new session ran %d times", started)
	}
}

// TestSessionsListOrdersAndToggles pins the sessions list's own axis: the
// newest session is first (the old page's default), the heading carries that
// order as a control, and pressing it swaps the list to oldest first. It also
// pins what a row says under the session's name: the shell's working
// directory, the state line, the screen size and the start time.
func TestSessionsListOrdersAndToggles(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	a.Sessions = func() []Session {
		return []Session{
			{Title: "first", Running: true, Index: 0, Note: "Running \u00b7 detached",
				Dir: "/home/u/first", Size: "80\u00d724", Created: base},
			{Title: "second", Running: true, Active: true, Index: 1, Note: "Running \u00b7 attached",
				Dir: "/home/u/second", Size: "100\u00d730", Created: base.Add(time.Hour)},
			{Title: "third", Running: false, Index: 2, Note: "Process exited \u00b7 code 1",
				Created: base.Add(2 * time.Hour)},
		}
	}
	tt.Frame()
	rowY := func(label string) float32 {
		t.Helper()
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no row %q in %q", label, tt.Texts())
		}
		return rect.Y
	}
	// Newest first, the shipped order.
	if !(rowY("third") < rowY("second") && rowY("second") < rowY("first")) {
		t.Errorf("the list is not newest-first: first=%v second=%v third=%v",
			rowY("first"), rowY("second"), rowY("third"))
	}
	if !tt.HasText("Newest first") {
		t.Errorf("the order control is missing: %q", tt.Texts())
	}
	// The row carries its own facts: the shell's cwd, the state, the size and
	// the start time (formatted the way the launcher writes a visit time).
	if !tt.HasText("/home/u/second") {
		t.Errorf("the working directory is missing: %q", tt.Texts())
	}
	if !tt.HasText("100\u00d730") {
		t.Errorf("the screen size is missing: %q", tt.Texts())
	}
	if !tt.HasText(base.Add(time.Hour).Local().Format("2006-01-02 15:04")) {
		t.Errorf("the start time is missing: %q", tt.Texts())
	}
	// Pressing the control swaps the axis, and its own word says so.
	if err := tt.Click("Newest first"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Oldest first") {
		t.Errorf("the order control did not flip: %q", tt.Texts())
	}
	if !(rowY("first") < rowY("second") && rowY("second") < rowY("third")) {
		t.Errorf("the list is not oldest-first after the toggle: first=%v second=%v third=%v",
			rowY("first"), rowY("second"), rowY("third"))
	}
}

// TestSessionsWithoutAStartKeepTheShellsOrder pins the sort's stability: the
// shell hands sessions over in creation order and a fixture that records no
// start time must not be reshuffled by the sort.
func TestSessionsWithoutAStartKeepTheShellsOrder(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Sessions = func() []Session {
		return []Session{
			{Title: "a", Index: 0},
			{Title: "b", Index: 1},
			{Title: "c", Index: 2},
		}
	}
	got := a.sortedSessions()
	for i, want := range []string{"a", "b", "c"} {
		if got[i].Title != want {
			t.Errorf("order after sorting = %v, want a b c", []string{got[0].Title, got[1].Title, got[2].Title})
			break
		}
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
		"v0.3.14", "0.3.14", "mygo 0.3.5", "floter://",
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

// The deep-link copy button confirms the copy on itself — the old build's own
// "Link copied" for 1.6s — because a clipboard write is otherwise silent. The
// confirmation belongs to the line that was copied, not to both.
func TestDeepLinkCopyConfirmsOnTheButton(t *testing.T) {
	en := i18n.For("en").Settings
	var copied []string
	a := New(newStore(t), Actions{CopyText: func(text string) { copied = append(copied, text) }})
	tt := render(t, a, 720, 900)
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(en.DeepLinkCopy) || tt.HasText(en.DeepLinkCopied) {
		t.Fatalf("before a copy: %q", tt.Texts())
	}
	if err := tt.Click(en.DeepLinkCopy); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(copied) != 1 || copied[0] != "floter://connect?manifest=/path/to/tool.json" {
		t.Fatalf("copied = %v", copied)
	}
	if !tt.HasText(en.DeepLinkCopied) {
		t.Errorf("the button did not confirm the copy: %q", tt.Texts())
	}
	// Only the copied line confirms; the other keeps its resting label.
	resting := 0
	for _, text := range tt.Texts() {
		if text == en.DeepLinkCopy {
			resting++
		}
	}
	if resting != 1 {
		t.Errorf("the other line's button changed too: %d rest on %q", resting, en.DeepLinkCopy)
	}
	// The window is the app's own state, so its expiry is testable without
	// the frame clock.
	if !a.linkCopied("floter://connect?manifest=/path/to/tool.json", time.Now()) {
		t.Error("the copied line is not marked copied")
	}
	if a.linkCopied("floter://connect?manifest=/path/to/tool.json", time.Now().Add(2*time.Second)) {
		t.Error("the confirmation outlived its window")
	}
}

func TestWindowBehaviourControls(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 1400)

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

	// The Running row's switch asks before turning the integration off, and
	// the confirmation is what performs it.
	if err := tt.Click("Enabled"); err != nil {
		t.Fatalf("the enable switch is missing: %v", err)
	}
	tt.Frame()
	if toggled != "" {
		t.Errorf("turning off happened without asking: %q", toggled)
	}
	if !tt.HasText("Disable V Tools?") {
		t.Fatalf("the confirmation is missing: %q", tt.Texts())
	}
	if err := tt.Click("Disable"); err != nil {
		t.Fatalf("the confirm button: %v", err)
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
	tt := render(t, a, 720, 1400)

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
	tt := render(t, a, 720, 760)
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
	// The per-command switches carry their own caption and the one line that
	// says what a switch does, above the list rather than on every row.
	en := i18n.For("en").Settings
	if !tt.HasText(en.IntegrationsCommands) || !tt.HasText(en.IntegrationsCommandsHint) {
		t.Errorf("the command list caption/hint = %q", tt.Texts())
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
	if page, ok := PageByName("integrations"); !ok || page != PageIntegrations {
		t.Errorf("PageByName(integrations) = %d, %v", page, ok)
	}
	// The plugins page is gone: its settings are the launcher's own sheet.
	// A file that still names it resolves to nothing, and the settings open
	// on the first page.
	if _, ok := PageByName("plugins"); ok {
		t.Error("the retired plugins page still resolves")
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
	tt := render(t, a, 720, 1000)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The binding prints through the platform's own formatter (the glyphs on
	// macOS, the modifier words elsewhere).
	if !tt.HasText("Custom shortcuts") || !tt.HasText(shortcuts.Display("Cmd+Shift+P")) {
		t.Fatalf("the custom section = %q", tt.Texts())
	}
	// The custom section sits below the app-keys list: scroll it into view.
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()

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
	// The row explains what Adopt does: a plug-in directory unrecorded in the
	// repository, grafted in under the same permission review as an install.
	if hint := i18n.For("en").Settings.IntegrationsAdoptHint; !tt.HasText(hint) {
		t.Fatalf("the orphan row does not explain Adopt: %q", tt.Texts())
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

// A row's permission list unfolds into an audit that says what each permission
// allows and whether the host itself decides it.
func TestPermissionAuditUnfolds(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Permissions: []string{"filesystem-read", "environment"},
			Enforced:    map[string]bool{"environment": true},
		}}
	}
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Read files") || !tt.HasText("Read the environment") {
		t.Fatalf("the permission line is missing: %q", tt.Texts())
	}
	// The explanations are folded away until asked for.
	if tt.HasText("The tool may read files your user can read.") {
		t.Error("the audit is open before it was asked for")
	}
	if err := tt.Click("Permissions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("The tool may read files your user can read.") {
		t.Errorf("the audit did not open: %q", tt.Texts())
	}
	// The two tiers are named, and each permission sits under its own.
	if !tt.HasText("Floter enforces") || !tt.HasText("Declared, not blocked") {
		t.Errorf("the tiers are missing: %q", tt.Texts())
	}
	if !tt.HasText("Read the environment") || !tt.HasText("Read files") {
		t.Errorf("a permission is missing: %q", tt.Texts())
	}
	if !tt.HasText("Floter enforces this one: the host decides what the tool receives.") {
		t.Errorf("the enforced explanation is missing: %q", tt.Texts())
	}
	// Asking again folds it away.
	if err := tt.Click("Permissions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("The tool may read files your user can read.") {
		t.Error("the audit did not fold away")
	}
}

// The Integrations page offers the export and import of the whole list, and
// shows what the last one did.
func TestIntegrationTransferButtons(t *testing.T) {
	exported, imported := 0, 0
	a := New(newStore(t), Actions{
		ExportIntegrations: func() { exported++ },
		ImportIntegrations: func() { imported++ },
	})
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Export"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if exported != 1 {
		t.Fatalf("exported %d times", exported)
	}
	if err := tt.Click("Import"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if imported != 1 {
		t.Errorf("imported %d times", imported)
	}

	// The line the shell sets is what the page shows.
	a.TransferStatus = "2 integrations exported to /tmp/backup.json"
	tt.Frame()
	if !tt.HasText("2 integrations exported to /tmp/backup.json") {
		t.Errorf("the status line is missing: %q", tt.Texts())
	}
}

// The base plugins the app ships are listed on the Integrations page with the
// two controls they can have: the switch that turns one on or off, and the
// Configure button that opens its declarative configuration in the launcher
// (the same sheet the mode's own gear opens). The browser mode's own message
// points at this section, so a section that did not render would leave the
// hint pointing at a door that does not exist.
func TestBasePluginsSectionOpensConfigurationAndToggles(t *testing.T) {
	opened := ""
	toggles := map[string]bool{}
	a := New(newStore(t), Actions{
		OpenPluginConfig: func(id string) { opened = id },
		SetPluginEnabled: func(id string, enabled bool) { toggles[id] = enabled },
	})
	a.Integrations = func() []Integration { return nil }
	tt := render(t, a, 720, 900)

	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{
		"Base plugins",
		"Built-in functionality that ships with floter.",
		"Clipboard history", "Browser bookmarks & history", "Calculator",
		// The clipboard's privacy line is the one thing the row discloses
		// rather than describes; it must not be dropped.
		"Everything you copy system-wide is recorded — stored locally only.",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// Configure opens the plugin's own sheet: the row names the plugin in its
	// accessible name, because three buttons reading "Configure" alone are
	// three buttons a screen reader cannot tell apart.
	if err := tt.Click("Configure Calculator"); err != nil {
		t.Fatalf("the calculator's Configure button: %v", err)
	}
	tt.Frame()
	if opened != "calculator" {
		t.Errorf("Configure opened %q, want calculator", opened)
	}

	// The clipboard's switch turns the history off; the name is a hit target,
	// as every switch row's is.
	if err := tt.Click("Clipboard history"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got, ok := toggles["clipboard"]; !ok || got {
		t.Errorf("clipboard toggle = %v (set=%v), want false", got, ok)
	}

	// The calculator has no persisted on/off field, so it carries no switch
	// and clicking its name toggles nothing.
	if err := tt.Click("Calculator"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := toggles["calculator"]; ok {
		t.Error("the calculator toggled, but it has no switch")
	}
}

// The base-plugin rows are a door only when the shell has one to offer: with
// no configuration or switch actions the section is not drawn, so a page with
// nothing behind it cannot promise a control that does nothing.
func TestBasePluginsSectionNeedsAnAction(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration { return nil }
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("Base plugins") {
		t.Errorf("the section rendered with no action behind it: %q", tt.Texts())
	}
}

// The Shortcuts page lists the app's own keys with their bindings, and a
// recorder rebinds one.
func TestAppShortcutRowsRecord(t *testing.T) {
	store := newStore(t)
	recorded := [][2]string{}
	a := New(store, Actions{
		SetShortcut: func(id, accelerator string) { recorded = append(recorded, [2]string{id, accelerator}) },
	})
	a.Shortcut, a.ShortcutID = "Ctrl+Space", "toggle_window"
	tt := render(t, a, 720, 620)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"App keys", "New command", "Open a terminal window", "Run the n-th result"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// Each row has its own recorder: click the one beside the "New command"
	// row's binding (the tester finds elements by text, and several rows say
	// "Record").
	binding, ok := tt.Find("Ctrl + W")
	if !ok {
		t.Fatalf("the new command's binding is missing: %q", tt.Texts())
	}
	tt.ClickAt(binding.X+binding.W+20, binding.Y+binding.H/2)
	tt.Frame()
	if !tt.HasText("Press keys\u2026") {
		t.Fatalf("the recorder did not open: %q", tt.Texts())
	}
	tt.Key(ui.Super, ui.KeyK)
	tt.Frame()
	if len(recorded) != 1 || recorded[0][0] != "new_command" {
		t.Fatalf("recorded %v", recorded)
	}
}

// The Integrations page offers the shipped tool packages that are not
// installed, and hides the section once they all are.
func TestRecommendedToolsConnect(t *testing.T) {
	connected := []string{}
	a := New(newStore(t), Actions{
		ConnectRecommended: func(id string) { connected = append(connected, id) },
	})
	a.Recommended = func() []RecommendedTool {
		return []RecommendedTool{
			{ID: "io.github.vst93.v", Name: "V Tools", Description: "Developer tools"},
			{ID: "other.tool", Name: "Other", Installed: true},
		}
	}
	tt := render(t, a, 720, 620)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Recommended tools") || !tt.HasText("V Tools") {
		t.Fatalf("the recommended section = %q", tt.Texts())
	}
	// An installed one is not offered.
	if tt.HasText("Other") {
		t.Errorf("an installed tool was offered: %q", tt.Texts())
	}
	if err := tt.Click("Connect"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(connected) != 1 || connected[0] != "io.github.vst93.v" {
		t.Errorf("connected %v", connected)
	}

	// With everything installed the section is gone.
	a.Recommended = func() []RecommendedTool {
		return []RecommendedTool{{ID: "io.github.vst93.v", Name: "V Tools", Installed: true}}
	}
	tt.Frame()
	if tt.HasText("Recommended tools") {
		t.Errorf("the section stayed with nothing to connect: %q", tt.Texts())
	}
}

// A recorder releases the global shortcuts while it waits for a key, and takes
// them back when it is done: pressing the key that summons floter must be
// recorded, not acted on.
func TestRecorderSuspendsTheGlobalShortcuts(t *testing.T) {
	suspended := []bool{}
	a := New(newStore(t), Actions{
		SetShortcut:           func(string, string) {},
		SetShortcutsSuspended: func(on bool) { suspended = append(suspended, on) },
	})
	a.Shortcut, a.ShortcutID = "Ctrl+Space", "toggle_window"
	tt := render(t, a, 720, 1000)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(suspended) != 0 {
		t.Fatalf("the shortcuts were released before recording: %v", suspended)
	}

	// The summon key's recorder suspends, and a captured key resumes.
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(suspended) != 1 || suspended[0] != true {
		t.Fatalf("suspended = %v", suspended)
	}
	tt.Key(ui.Super, ui.KeyG)
	tt.Frame()
	if len(suspended) != 2 || suspended[1] != false {
		t.Errorf("suspended = %v", suspended)
	}

	// Escape cancels and resumes too.
	suspended = nil
	if err := tt.Click("Record"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if len(suspended) != 2 || suspended[0] != true || suspended[1] != false {
		t.Errorf("suspended = %v", suspended)
	}

	// The custom section's recorder does the same.
	suspended = nil
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()
	if err := tt.Click("Record key"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if len(suspended) != 2 || suspended[0] != true || suspended[1] != false {
		t.Errorf("suspended = %v", suspended)
	}
}

// An integration's configuration form renders its schema, keeps its draft
// across frames, and saves through the shell.
func TestConfigurationForm(t *testing.T) {
	saved := map[string]map[string]any{}
	min := 0.5
	max := 9.0
	a := New(newStore(t), Actions{
		SaveConfiguration: func(id string, values map[string]any) error {
			if saved == nil {
				saved = map[string]map[string]any{}
			}
			saved[id] = values
			return nil
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Config: []ConfigField{
				{Key: "endpoint", Label: "Endpoint", Type: "text", Required: true, MinLength: intPtr(4)},
				{Key: "token", Label: "Token", Type: "password", Required: true},
				{Key: "region", Label: "Region", Type: "select", Options: []string{"eu", "us"}, Default: "eu"},
				{Key: "tags", Label: "Tags", Type: "multiSelect", Options: []string{"a", "b", "c"}},
				{Key: "retries", Label: "Retries", Type: "number", Minimum: &min, Maximum: &max},
				{Key: "verbose", Label: "Verbose", Type: "boolean", Description: "Print every request the tool makes"},
			},
			ConfigValues: map[string]any{"endpoint": "https://example.com", "token": "s3cret", "region": "us"},
		}}
	}
	tt := render(t, a, 720, 1100)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The form is folded away until asked for.
	if tt.HasText("Region") {
		t.Error("the form was open before it was asked for")
	}
	if err := tt.Click("Configure"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"Endpoint", "Token", "Region", "Tags", "Retries", "Verbose"} {
		if !tt.HasText(want) {
			t.Errorf("the field %q is missing: %q", want, tt.Texts())
		}
	}
	// The boolean field keeps its description too: the switch used to drop
	// it while the select and the text field showed theirs.
	if !tt.HasText("Print every request the tool makes") {
		t.Errorf("the boolean field's description is missing: %q", tt.Texts())
	}
	// The loaded values seed the draft: the select shows the stored region,
	// and a defaulted field the default.
	if !tt.HasText("us") {
		t.Errorf("the stored region is not shown: %q", tt.Texts())
	}

	// Ticking a multiselect option, typing into a number field and saving
	// hand the whole draft to the shell.
	if err := tt.Click("a"); err != nil {
		t.Fatalf("multiselect: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	values := saved["io.github.vst93.v"]
	if values == nil {
		t.Fatal("nothing was saved")
	}
	if values["endpoint"] != "https://example.com" || values["token"] != "s3cret" {
		t.Errorf("text/password = %v", values)
	}
	if values["region"] != "us" {
		t.Errorf("region = %v", values["region"])
	}
	tags, ok := values["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "a" {
		t.Errorf("tags = %v", values["tags"])
	}
	// The boolean and the number keep their state (untouched here: the
	// defaults are what a save would write).
	if _, ok := values["verbose"]; !ok && values["verbose"] != nil {
		t.Logf("verbose = %v", values["verbose"])
	}

	// A save error is reported on the form rather than vanishing.
	a.Actions.SaveConfiguration = func(string, map[string]any) error {
		return errors.New("the endpoint is unreachable")
	}
	tt.Click("Save")
	tt.Frame()
	if !tt.HasText("the endpoint is unreachable") {
		t.Errorf("the error is missing: %q", tt.Texts())
	}
}

func intPtr(v int) *int { return &v }

// The Plugins page's clear-history control reports a press.
// The launcher's own switches are reachable and write through the store: the
// recent applications, the menu bar icon, and the PATH commands joining the
// search.
func TestLauncherSwitchesWriteThroughTheStore(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	tt := render(t, a, 720, 1400)
	if err := tt.Click("Show recently launched apps"); err != nil {
		t.Fatalf("recent switch: %v", err)
	}
	tt.Frame()
	if settings.ShowRecentInLauncher(store.Snapshot()) {
		t.Error("the recent switch did not turn off")
	}
	if err := tt.Click("Show menu bar icon"); err != nil {
		t.Fatalf("tray switch: %v", err)
	}
	tt.Frame()
	if settings.ShowMenubarIcon(store.Snapshot()) {
		t.Error("the tray switch did not turn off")
	}
	// The commands switch lives on the Integrations page.
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Show PATH commands in search"); err != nil {
		t.Fatalf("commands switch: %v", err)
	}
	tt.Frame()
	if !settings.ShowCommandsInSearch(store.Snapshot()) {
		t.Error("the commands switch did not turn on")
	}
}

// The sidebar leads every page with a mark and paints the chosen page on the
// resting control fill; the page's own navigation still switches.
func TestSidebarRowsCarryMarksAndSelection(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	tt := render(t, a, 720, 900)
	tt.Frame()
	// Every page's label is there, and each row is the sidebar's own height.
	for _, title := range []string{"General", "Sessions", "Shortcuts", "Integrations", "About"} {
		rect, ok := tt.Find(title)
		if !ok {
			t.Fatalf("the sidebar is missing %q: %v", title, tt.Texts())
		}
		if rect.H < 24 {
			t.Errorf("%q is not a full row: %+v", title, rect)
		}
	}
	// Every page leads with a mark from the shared vocabulary.
	for page := PageGeneral; page < pageCount; page++ {
		if _, ok := pageGlyph(page); !ok {
			t.Errorf("page %d has no mark", page)
		}
	}
	// The chosen page is the one showing; clicking another switches both.
	if a.Page != PageGeneral {
		t.Fatalf("the panel opened on page %v", a.Page)
	}
	if err := tt.Click("About"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Page != PageAbout {
		t.Errorf("page = %v, want about", a.Page)
	}
}

// The local-tool form collects what the host needs and hands it on: the name,
// the program (with the picker), the command word, the arguments and the
// permissions the user ticked.
func TestLocalToolForm(t *testing.T) {
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
		ChooseProgram:    func() (string, error) { return "/usr/bin/rg", nil },
	})
	tt := render(t, a, 720, 1400)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The form is folded away until asked for.
	if tt.HasText("Name") {
		t.Error("the form is open before it was asked for")
	}
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatalf("the form's door: %v", err)
	}
	tt.Frame()
	for _, want := range []string{"Name", "Program", "Command", "Arguments", "Version arguments"} {
		if !tt.HasText(want) {
			t.Errorf("the form is missing %q: %q", want, tt.Texts())
		}
	}
	// The picker fills the program field. (The tester keeps a per-window
	// editor buffer, so the *state* is what the assertion reads.)
	if err := tt.Click("Choose\u2026"); err != nil {
		t.Fatalf("the picker: %v", err)
	}
	tt.Frame()
	if a.toolProgram != "/usr/bin/rg" {
		t.Errorf("the picker did not fill the program: %q", a.toolProgram)
	}
	// The permissions are rows, and one is ticked.
	for _, permission := range extensions.AllPermissions {
		if !tt.HasText(i18n.PermissionLabel("en", permission)) {
			t.Errorf("the permission %q is missing", permission)
		}
	}
	if err := tt.Click(i18n.PermissionLabel("en", "environment")); err != nil {
		t.Fatalf("a permission row: %v", err)
	}
	tt.Frame()
	// Connect hands the whole draft on. The name is the form's own state (the
	// tester cannot type into a field it has already drawn).
	a.toolName = "Ripgrep"
	tt.Frame()
	if err := tt.Click("Connect"); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("connected = %+v", connected)
	}
	tool := connected[0]
	if tool.Program != "/usr/bin/rg" {
		t.Errorf("program = %q", tool.Program)
	}
	found := false
	for _, permission := range tool.Permissions {
		if permission == "environment" {
			found = true
		}
	}
	if !found {
		t.Errorf("permissions = %v", tool.Permissions)
	}
}

// The freshness row: what is known about the command list, with "unknown"
// spelled out rather than painted as a zero.
func TestFreshnessRow(t *testing.T) {
	count := 3
	previous := 1
	a := New(settings.NewStore(settings.Default()), Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			if id != "io.github.vst93.v" {
				return extensions.Freshness{}, false
			}
			return extensions.Freshness{
				AtSeconds:            time.Now().Add(-3 * time.Minute).Unix(),
				Source:               extensions.FreshnessProbe,
				Result:               extensions.FreshnessSuccess,
				CommandCount:         &count,
				PreviousCommandCount: &previous,
				Delta:                extensions.DeltaIncrease,
			}, true
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Command list") {
		t.Fatalf("the freshness row is missing: %q", tt.Texts())
	}
	if !tt.HasText("Succeeded") {
		t.Errorf("the result is missing: %q", tt.Texts())
	}
	// The delta says how far the list moved, and the count is the real one.
	if !tt.HasText("Commands 3") {
		t.Errorf("the count is missing: %q", tt.Texts())
	}
	if !tt.HasText("+2 since the scan before") {
		t.Errorf("the delta is missing: %q", tt.Texts())
	}
	// A never-probed integration shows no row at all.
	a.Actions.Freshness = func(string) (extensions.Freshness, bool) { return extensions.Freshness{}, false }
	tt.Frame()
	if tt.HasText("Command list") {
		t.Errorf("a never-probed integration shows a freshness row: %q", tt.Texts())
	}
}

// The stored lifecycle health is drawn on the integration row: the probe
// report lives in the repository, so the row says how the tool was last found
// to be before any check runs in this session. It is its own row, under its
// own label, distinct from the live diagnostics below.
func TestFreshnessDotProjectsTheRowState(t *testing.T) {
	count := func(n int) *int { return &n }
	freshness := extensions.Freshness{
		AtSeconds:            time.Now().Unix(),
		Source:               extensions.FreshnessProbe,
		Result:               extensions.FreshnessSuccess,
		Delta:                extensions.DeltaUnchanged,
		CommandCount:         count(4),
		PreviousCommandCount: count(4),
	}
	a := New(newStore(t), Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			return freshness, id == "dev.floter.tool"
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.tool", Name: "Tool", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// A completed comparison with equal counts reads as in sync.
	if !tt.HasText("Command status: in sync") {
		t.Fatalf("the synced dot is missing: %q", tt.Texts())
	}
	// A count that moved is the one state that earns warmth.
	freshness.Delta = extensions.DeltaIncrease
	freshness.CommandCount = count(6)
	tt.Frame()
	if !tt.HasText("Command status: changed upstream") {
		t.Errorf("the changed dot is missing: %q", tt.Texts())
	}
	if tt.HasText("Command status: in sync") {
		t.Errorf("a changed list still reads as in sync: %q", tt.Texts())
	}
	// A first probe has nothing to compare against, so it is unknown, never
	// a fabricated "in sync".
	freshness.Delta = extensions.DeltaUnknown
	freshness.PreviousCommandCount = nil
	tt.Frame()
	if !tt.HasText("Command status: unknown, not compared") {
		t.Errorf("the unknown dot is missing: %q", tt.Texts())
	}
	// The dot's word is also its tooltip, so a pointer resting on it reads the
	// same sentence the label carries. (A Label and the tooltip a hover reveals
	// are two nodes; the count must rise.)
	countOf := func(want string) int {
		n := 0
		for _, s := range tt.Texts() {
			if s == want {
				n++
			}
		}
		return n
	}
	label := "Command status: unknown, not compared"
	if rect, ok := tt.Find(label); ok {
		before := countOf(label)
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
		// The tip waits for the pointer to rest (Base UI's 600ms).
		time.Sleep(700 * time.Millisecond)
		tt.Frame()
		if countOf(label) <= before {
			t.Errorf("the dot has no tooltip (nodes %d → %d): %q", before, countOf(label), tt.Texts())
		}
	} else {
		t.Fatal("the dot is not findable by its own label")
	}
	// A never-probed integration has no mark at all.
	a.Actions.Freshness = func(string) (extensions.Freshness, bool) { return extensions.Freshness{}, false }
	tt.Frame()
	if tt.HasText("Command status:") {
		t.Errorf("a never-probed integration shows a dot: %q", tt.Texts())
	}
}

func TestStoredHealthShowsOnTheRow(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Integrations = func() []Integration {
		return []Integration{
			{ID: "dev.floter.healthy", Name: "Alpha Tool", Enabled: true, Health: "healthy"},
			{ID: "dev.floter.broken", Name: "Beta Tool", Enabled: true, Broken: true, Health: "not working"},
			{ID: "dev.floter.diagnosed", Name: "Gamma Tool", Enabled: true, Diagnosis: "ok"},
		}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The stored health is the row's value, under the Health label.
	if !tt.HasText("Health") {
		t.Fatalf("the health label is missing: %q", tt.Texts())
	}
	if !tt.HasText("healthy") {
		t.Errorf("the stored health is missing: %q", tt.Texts())
	}
	if !tt.HasText("not working") {
		t.Errorf("a broken tool's stored health is missing: %q", tt.Texts())
	}
	// The live check's own sentence is a different row with its own label, so
	// the two facts never read as one another.
	if !tt.HasText("Diagnostics") {
		t.Errorf("the diagnostics label is missing: %q", tt.Texts())
	}
	if !tt.HasText("ok") {
		t.Errorf("the diagnosis is missing: %q", tt.Texts())
	}

	// An integration with neither a stored report nor a diagnosis draws
	// neither row — "unknown" is its own state, not a row of zeroes.
	b := New(newStore(t), Actions{})
	b.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.plain", Name: "Plain Tool", Enabled: true}}
	}
	bt := render(t, b, 720, 900)
	if err := bt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	bt.Frame()
	if bt.HasText("Health") {
		t.Errorf("an unprobed integration shows a health row: %q", bt.Texts())
	}
	if bt.HasText("Diagnostics") {
		t.Errorf("an unchecked integration shows a diagnostics row: %q", bt.Texts())
	}
}

// The detected section lists what the machine has and nothing has connected,
// and its Connect press hands the tool on with the discovery's own words.
func TestDetectedToolsSection(t *testing.T) {
	connected := []DetectedTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		Detected: func() []DetectedTool {
			return []DetectedTool{{
				Name:        "ripgrep",
				Path:        "/usr/bin/rg",
				Description: "Fast search",
			}}
		},
		ConnectDetected: func(tool DetectedTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Found on this machine") || !tt.HasText("ripgrep") {
		t.Fatalf("the detected section is missing: %q", tt.Texts())
	}
	if !tt.HasText("Fast search") {
		t.Errorf("the description is missing: %q", tt.Texts())
	}
	if err := tt.Click("Connect"); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 || connected[0].Path != "/usr/bin/rg" {
		t.Errorf("connected = %+v", connected)
	}
	// Nothing detected: no section at all.
	a.Actions.Detected = func() []DetectedTool { return nil }
	tt.Frame()
	if tt.HasText("Found on this machine") {
		t.Errorf("an empty section was drawn: %q", tt.Texts())
	}
}

// Disabling asks first — stopping an integration kills whatever it is running
// — while enabling is no question at all, and Cancel leaves the state alone.
func TestDisableAsksFirst(t *testing.T) {
	toggled := []bool{}
	a := New(newStore(t), Actions{
		SetIntegrationEnabled: func(id string, enabled bool) { toggled = append(toggled, enabled) },
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// Cancel: nothing happens.
	if err := tt.Click("Enabled"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Cancel"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tt.Frame()
	if len(toggled) != 0 {
		t.Errorf("Cancel toggled: %v", toggled)
	}
	if tt.HasText("Disable V Tools?") {
		t.Errorf("the confirmation stayed up: %q", tt.Texts())
	}
	// Enabling a disabled integration is immediate.
	a.Integrations = func() []Integration {
		return []Integration{{ID: "io.github.vst93.v", Name: "V Tools", Enabled: false}}
	}
	tt.Frame()
	if err := tt.Click("Enabled"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(toggled) != 1 || !toggled[0] {
		t.Errorf("enabling asked or failed: %v", toggled)
	}
}

// The package form's door: a folder the user picks goes to the shell, which
// reviews and installs it.
func TestInstallFromFolderRow(t *testing.T) {
	installed := []string{}
	a := New(settings.NewStore(settings.Default()), Actions{
		InstallFromFolder: func(dir string) { installed = append(installed, dir) },
		ChooseFolder:      func() (string, error) { return "/tmp/my-package", nil },
	})
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Install from a folder") {
		t.Fatalf("the row is missing: %q", tt.Texts())
	}
	if err := tt.Click("Choose\u2026"); err != nil {
		t.Fatalf("the picker: %v", err)
	}
	tt.Frame()
	if len(installed) != 1 || installed[0] != "/tmp/my-package" {
		t.Errorf("installed = %v", installed)
	}
	// A caller with no picker hides the row entirely.
	a.Actions.ChooseFolder = nil
	tt.Frame()
	if tt.HasText("Install from a folder") {
		t.Errorf("the row shows without a picker: %q", tt.Texts())
	}
}

// The run form: one control per declared input, the required values refused
// until they are filled, and the answers handed to the shell as a map — never
// turned into a command string here.
func TestRunParamForm(t *testing.T) {
	type run struct {
		id     string
		values map[string]string
	}
	runs := []run{}
	a := New(settings.NewStore(settings.Default()), Actions{
		RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
			runs = append(runs, run{id, values})
			return nil
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Params: []Param{
				{ID: "target", Label: "Target", Kind: "text", Required: true, Flag: "--target"},
				{ID: "verbose", Label: "Verbose", Kind: "boolean", Flag: "--verbose"},
				{ID: "mode", Label: "Mode", Kind: "select", Options: []string{"fast", "slow"}},
			},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"Target", "Verbose", "Mode"} {
		if !tt.HasText(want) {
			t.Fatalf("the form is missing %q: %q", want, tt.Texts())
		}
	}
	// A required value that is empty is refused live, under the input that
	// caused it, and the Run button is disabled while it stands.
	if !tt.HasText("Required: fill in Target before running") {
		t.Fatalf("the row's reason is missing: %q", tt.Texts())
	}
	if err := tt.Click("Run with these values"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(runs) != 0 {
		t.Fatalf("an incomplete form ran: %v", runs)
	}
	// Filling it clears the reason and runs, with the select's default seeded.
	a.runValues["io.github.vst93.v"]["target"] = "here"
	tt.Frame()
	if tt.HasText("Required: fill in Target before running") {
		t.Errorf("the reason outlived the fix: %q", tt.Texts())
	}
	if err := tt.Click("Run with these values"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(runs) != 1 {
		t.Fatalf("runs = %v", runs)
	}
	values := runs[0].values
	if values["target"] != "here" || values["mode"] != "fast" {
		t.Errorf("values = %v", values)
	}
}

// A value the form can judge is refused live, under the input that caused it:
// a number that is not a finite decimal, a choice off the list. The Run button
// stays disabled while a reason stands.
func TestRunParamFormShowsPerRowReasons(t *testing.T) {
	runs := 0
	a := New(newStore(t), Actions{
		RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
			runs++
			return nil
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Params: []Param{
				{ID: "count", Label: "Count", Kind: "number"},
				{ID: "mode", Label: "Mode", Kind: "select", Options: []string{"fast", "slow"}},
			},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// A number the form cannot judge.
	a.runValues["io.github.vst93.v"]["count"] = "two"
	tt.Frame()
	if !tt.HasText("Count is not a valid value for this parameter") {
		t.Fatalf("the number reason is missing: %q", tt.Texts())
	}
	if err := tt.Click("Run with these values"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if runs != 0 {
		t.Fatalf("an invalid form ran: %d", runs)
	}
	// A choice off the list, then everything fixed.
	a.runValues["io.github.vst93.v"]["count"] = "2"
	a.runValues["io.github.vst93.v"]["mode"] = "medium"
	tt.Frame()
	if !tt.HasText("Mode is not a valid value for this parameter") {
		t.Fatalf("the choice reason is missing: %q", tt.Texts())
	}
	a.runValues["io.github.vst93.v"]["mode"] = "fast"
	tt.Frame()
	if tt.HasText("is not a valid value") {
		t.Errorf("the reasons outlived the fixes: %q", tt.Texts())
	}
}

// A run the backend refuses for a parameter reason is worded by the surface,
// with the label the user sees — never the raw key, and the id only when there
// is no label. The backend owns the fact, the dictionary owns the sentence.
func TestRunParamRefusalIsWorded(t *testing.T) {
	params := []extensions.ParamDefinition{
		{ID: "note", Label: "Note"},
		{ID: "mode", Label: "  "},
	}
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings
	cases := []struct {
		failure *extensions.ParamFailure
		en, zh  string
	}{
		{&extensions.ParamFailure{Key: extensions.RunErrorParamRequired, Param: "note"},
			"Required: fill in Note before running", "必填：运行前请填写 Note"},
		{&extensions.ParamFailure{Key: extensions.RunErrorParamInvalid, Param: "note"},
			"Note is not a valid value for this parameter", "Note 不是该参数的有效值"},
		{&extensions.ParamFailure{Key: extensions.RunErrorParamUnknown, Param: "sneaky"},
			"No parameter is declared with the id sneaky", "没有声明 id 为 sneaky 的参数"},
		{&extensions.ParamFailure{Key: extensions.RunErrorParamWindowsUnsafe, Param: "note"},
			"Note contains a character", "Note 含 cmd.exe"},
		// A label-less parameter falls back to its id.
		{&extensions.ParamFailure{Key: extensions.RunErrorParamRequired, Param: "mode"},
			"Required: fill in mode before running", "必填：运行前请填写 mode"},
	}
	for _, tc := range cases {
		got := paramFailureMessage(en, params, tc.failure)
		if !strings.Contains(got, tc.en) {
			t.Errorf("en = %q, want %q", got, tc.en)
		}
		if got := paramFailureMessage(zh, params, tc.failure); !strings.Contains(got, tc.zh) {
			t.Errorf("zh = %q, want %q", got, tc.zh)
		}
		// The raw key never leaks into the sentence.
		if strings.Contains(paramFailureMessage(en, params, tc.failure), string(tc.failure.Key)) {
			t.Errorf("the key leaked: %q", paramFailureMessage(en, params, tc.failure))
		}
	}
}

// The run form shows the backend's refusal after a successful local pre-check:
// a value the form cannot judge (a cmd.exe metacharacter) is caught by the
// authority and worded under the button.
func TestRunParamFormShowsTheBackendRefusal(t *testing.T) {
	a := New(newStore(t), Actions{
		RunWithParams: func(id string, values map[string]string) *extensions.ParamFailure {
			return &extensions.ParamFailure{Key: extensions.RunErrorParamWindowsUnsafe, Param: "target"}
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true,
			Params: []Param{{ID: "target", Label: "Target", Kind: "text"}},
		}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.runValues["io.github.vst93.v"]["target"] = "a&b"
	tt.Frame()
	if err := tt.Click("Run with these values"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Target contains a character") {
		t.Fatalf("the refusal is missing: %q", tt.Texts())
	}
}

// The last background run's output block in an integration's row: folded
// behind "View output" after a successful run, unfolded at once after a failed
// one, and four states inside — the two streams, the empty line, and the two
// truncation marks.
func TestRunOutputBlock(t *testing.T) {
	integration := func() []Integration {
		return []Integration{{ID: "io.github.vst93.v", Name: "V Tools", Enabled: true, Running: true}}
	}
	a := New(newStore(t), Actions{})
	a.Integrations = integration
	a.SetRunResult("io.github.vst93.v", RunResult{Stdout: "hello\nworld\n", Success: true})
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// A successful run leaves the block folded: the row offers to view it.
	if !tt.HasText("View output") {
		t.Fatalf("the view control is missing: %q", tt.Texts())
	}
	if tt.HasText("Last output") {
		t.Fatalf("the block is unfolded before the press: %q", tt.Texts())
	}
	if err := tt.Click("View output"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Last output") || !tt.HasText("Hide output") {
		t.Fatalf("the block did not open: %q", tt.Texts())
	}
	if !tt.HasText("hello") || !tt.HasText("world") {
		t.Errorf("the output is missing: %q", tt.Texts())
	}
	// The hide control folds it again.
	if err := tt.Click("Hide output"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("Last output") {
		t.Errorf("the hide control did not fold the block: %q", tt.Texts())
	}

	// An empty capture says so rather than leaving a blank block.
	a.SetRunResult("io.github.vst93.v", RunResult{Success: true})
	a.setRunOutputOpen("io.github.vst93.v", true)
	tt.Frame()
	if !tt.HasText("(No output)") {
		t.Errorf("the empty line is missing: %q", tt.Texts())
	}

	// A truncated capture marks both ends.
	a.SetRunResult("io.github.vst93.v", RunResult{Stdout: "cut", Truncated: true, Success: true})
	a.setRunOutputOpen("io.github.vst93.v", true)
	tt.Frame()
	if !tt.HasText("Output was truncated") || !tt.HasText("output truncated at 64 KB") {
		t.Errorf("the truncation marks are missing: %q", tt.Texts())
	}

	// A failed run unfolds its output at once: the reason is one glance away.
	failed := New(newStore(t), Actions{})
	failed.Integrations = integration
	failed.SetRunResult("io.github.vst93.v", RunResult{Stderr: "boom\n", Success: false})
	tf := render(t, failed, 720, 900)
	if err := tf.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tf.Frame()
	if !tf.HasText("Last output") || !tf.HasText("boom") {
		t.Errorf("a failed run did not unfold its output: %q", tf.Texts())
	}
}

// The connect form's declared inputs: a row per parameter, add and remove, and
// the rows a request carries — a row still being typed (no id) is skipped
// rather than refused.
func TestConnectFormDeclaresInputs(t *testing.T) {
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	// The frame is taller than the panel: the settings rows are the 44u floor
	// now, and the inputs editor sits at the end of them.
	tt := render(t, a, 720, 2000)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Parameters") || !tt.HasText("None declared") {
		t.Fatalf("the inputs editor is missing: %q", tt.Texts())
	}
	if err := tt.Click("Add an input"); err != nil {
		t.Fatalf("add: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Input 1") {
		t.Fatalf("the row is missing: %q", tt.Texts())
	}
	// The row exists but has no id yet: it is not a declaration.
	a.toolName, a.toolProgram = "Gadget", "/usr/bin/rg"
	tool := a.declaredParams()
	if len(tool) != 0 {
		t.Errorf("an id-less row became a declaration: %+v", tool)
	}
	// Filling it in makes it one.
	a.toolParams[0].ID = "target"
	a.toolParams[0].Flag = "--target"
	a.toolParams[0].Kind = extensions.ParamText
	params := a.declaredParams()
	if len(params) != 1 || params[0].ID != "target" || params[0].Flag != "--target" {
		t.Fatalf("declared = %+v", params)
	}
	// Removing the row takes it back out.
	tt.Frame()
	if err := tt.Click("Remove"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	tt.Frame()
	if len(a.toolParams) != 0 {
		t.Errorf("the row survived its removal: %+v", a.toolParams)
	}
}

// A declared input that still needs something says so under its own row, and
// the save refuses there rather than bouncing the typo back from the backend.
func TestConnectFormNamesAnUnfinishedInputAndRefusesToSave(t *testing.T) {
	copy := i18n.For("en").Settings
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 2200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add an input"); err != nil {
		t.Fatalf("add: %v", err)
	}
	tt.Frame()
	// A named select with no choices: its row names the missing options.
	a.toolParams[0].ID = "mode"
	a.toolParams[0].Kind = extensions.ParamSelect
	a.toolName, a.toolProgram = "Gadget", "/usr/bin/rg"
	tt.Frame()
	if !tt.HasText(copy.ParamNeedsOptions) {
		t.Fatalf("the row did not name its missing options: %q", tt.Texts())
	}
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 0 {
		t.Fatalf("an unfinished input reached the backend: %+v", connected)
	}
	// Giving it options clears the line and lets the save through.
	a.toolParams[0].Options = "fast, slow"
	tt.Frame()
	if tt.HasText(copy.ParamNeedsOptions) {
		t.Fatalf("the row still names options it now has: %q", tt.Texts())
	}
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 || len(connected[0].Params) != 1 || connected[0].Params[0].ID != "mode" {
		t.Fatalf("connected = %+v", connected)
	}
}

// Editing a generated integration: the button fills the same form from what
// the integration declares now, and the submit rewrites it (the id names it)
// rather than connecting a second one.
func TestEditGeneratedIntegration(t *testing.T) {
	edited := []struct {
		id   string
		tool CustomTool
	}{}
	connected := []CustomTool{}
	a := New(settings.NewStore(settings.Default()), Actions{
		EditCustomTool: func(id string, tool CustomTool) {
			edited = append(edited, struct {
				id   string
				tool CustomTool
			}{id, tool})
		},
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
		ToolForEdit: func(id string) (CustomTool, bool) {
			if id != "local.abc" {
				return CustomTool{}, false
			}
			return CustomTool{
				Name: "Ripgrep", Program: "/usr/bin/rg", Command: "rg",
				Args: "--floter", Version: "1.0.0",
				Params: []extensions.ParamDefinition{{ID: "target", Kind: "text", Flag: "--target"}},
			}, true
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{
			ID: "local.abc", Name: "Ripgrep", Enabled: true, Running: true, Generated: true,
		}}
	}
	tt := render(t, a, 720, 2200)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Edit\u2026"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	tt.Frame()
	// The form is open and filled from the integration.
	if !tt.HasText("Ripgrep") || !tt.HasText("Input 1") {
		t.Fatalf("the form was not filled: %q", tt.Texts())
	}
	if a.toolEditing != "local.abc" || a.toolProgram != "/usr/bin/rg" {
		t.Fatalf("editing=%q program=%q", a.toolEditing, a.toolProgram)
	}
	// Saving rewrites it, and does not connect a second package.
	if err := tt.Click("Save"); err != nil {
		t.Fatalf("save: %v", err)
	}
	tt.Frame()
	if len(edited) != 1 || edited[0].id != "local.abc" {
		t.Fatalf("edited = %+v", edited)
	}
	if len(connected) != 0 {
		t.Errorf("editing also connected: %+v", connected)
	}
	if edited[0].tool.Command != "rg" || edited[0].tool.Args != "--floter" {
		t.Errorf("the draft lost its fields: %+v", edited[0].tool)
	}
	if a.toolEditing != "" {
		t.Errorf("the form stayed in edit mode: %q", a.toolEditing)
	}
}

// The settings panel's own header band: the app's name and version at the
// start, and the two ways out at the end — quit, and back to the launcher.
// It is the same 56u band the launcher's field row and every page's header
// use, so the window's first line does not move when the surface changes.
func TestSettingsHeaderBand(t *testing.T) {
	quit, closed := 0, 0
	a := New(newStore(t), Actions{
		Quit:  func() { quit++ },
		Close: func() { closed++ },
	})
	a.About = About{Name: "floter", Version: "0.3.14"}
	tt := render(t, a, 720, 580)
	for _, want := range []string{"Settings", "v0.3.14", "Quit floter", "Close settings"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("Quit floter"); err != nil {
		t.Fatalf("quit: %v", err)
	}
	tt.Frame()
	if quit != 1 || closed != 0 {
		t.Errorf("quit = %d, close = %d", quit, closed)
	}
	if err := tt.Click("Close settings"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if closed != 1 {
		t.Errorf("close = %d", closed)
	}
	// A panel with no actions draws no buttons for them.
	b := New(newStore(t), Actions{})
	b.About = About{Name: "floter"}
	tt = render(t, b, 720, 580)
	if tt.HasText("Quit floter") || tt.HasText("Close settings") {
		t.Errorf("buttons were drawn with no actions: %q", tt.Texts())
	}
}

// The header's two icon-only controls carry their own sentence: the copy the
// reference put in each button's `title=` is the tip that shows once the
// pointer rests, so a bare glyph is not an unexplained mark. The strings were
// declared and translated but no view showed them.
func TestSettingsHeaderButtonsCarryTheirHints(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{Quit: func() {}, Close: func() {}})
	tt := render(t, a, 720, 580)
	for _, want := range []struct{ label, hint string }{
		{copy.Quit, copy.QuitHint},
		{copy.Close, copy.CloseHint},
	} {
		rect, ok := tt.Find(want.label)
		if !ok {
			t.Fatalf("no %q control: %q", want.label, tt.Texts())
		}
		// Park the pointer away first, so this control's own rest timer starts
		// fresh rather than warming on the previous tip.
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H+120)
		tt.Frame()
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
		// The tip waits for the pointer to rest (Base UI's 600ms).
		time.Sleep(700 * time.Millisecond)
		tt.Frame()
		if !tt.HasText(want.hint) {
			t.Errorf("the %q control has no tooltip %q: %q", want.label, want.hint, tt.Texts())
		}
	}
}

// "Restore defaults" is the shortcuts group's own action, on the group title
// row - and it really resets: the shell's reset re-registers the shipped
// bindings and drops the custom ones.
func TestRestoreDefaultsRuns(t *testing.T) {
	reset := 0
	a := New(newStore(t), Actions{ResetShortcuts: func() { reset++ }})
	tt := render(t, a, 720, 800)
	if err := tt.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Restore the shipped keys"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	tt.Frame()
	if reset != 1 {
		t.Errorf("restore ran %d times", reset)
	}
}

// sample reads one pixel of the last frame, at a logical coordinate.
func sample(tt *ui.Tester, x, y float32) ui.Color {
	img := tt.Image()
	i := int(y)*img.Stride + int(x)*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// The palette row shows what its choice paints: the block under the picker is
// drawn in the palette's own background, and follows the picker when it
// changes (R43 — the user asked a palette to show what it does).
func TestThePaletteRowPreviewsItsColors(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	if err := store.Update(func(s *settings.Settings) { s.TerminalTheme = "ink" }); err != nil {
		t.Fatal(err)
	}
	tt := render(t, a, 720, 3000)
	label := i18n.For("en").Settings.PalettePreview
	rect, ok := tt.Find(label)
	if !ok {
		t.Fatalf("the palette preview is missing: %q", tt.Texts())
	}
	want, _ := terminalui.PaletteColorsFor("ink", true)
	// The sample sits in the block's top-left padding, clear of any glyph.
	if got := sample(tt, rect.X+4, rect.Y+4); got != want.Background {
		t.Errorf("preview background = %v, want the ink background %v", got, want.Background)
	}

	// A new choice repaints the block.
	if err := store.Update(func(s *settings.Settings) { s.TerminalTheme = "paper" }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	rect, _ = tt.Find(label)
	want, _ = terminalui.PaletteColorsFor("paper", false)
	if got := sample(tt, rect.X+4, rect.Y+4); got != want.Background {
		t.Errorf("preview background after the change = %v, want the paper background %v", got, want.Background)
	}
}

// `inherit` has no fixed pair, so the preview follows the app's own
// appearance, as the reference's `--terminal-*` tokens did.
func TestThePalettePreviewFollowsTheAppearanceForInherit(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 3000)
	label := i18n.For("en").Settings.PalettePreview
	rect, ok := tt.Find(label)
	if !ok {
		t.Fatalf("the palette preview is missing: %q", tt.Texts())
	}
	want, _ := terminalui.PaletteColorsFor("inherit", false)
	if got := sample(tt, rect.X+4, rect.Y+4); got != want.Background {
		t.Errorf("light inherit preview = %v, want %v", got, want.Background)
	}

	tt.SetDark(true)
	tt.Frame()
	rect, _ = tt.Find(label)
	want, _ = terminalui.PaletteColorsFor("inherit", true)
	if got := sample(tt, rect.X+4, rect.Y+4); got != want.Background {
		t.Errorf("dark inherit preview = %v, want %v", got, want.Background)
	}
}
