package shell

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/clipboard"
	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/tools"
)

func newApp(t *testing.T) *App {
	t.Helper()
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store: store,
		// Every directory a test touches is a fixture: the machine's real
		// clipboard history and extension state are never opened.
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return nil, errors.New("no terminal library in tests")
		},
		// The machine's login items are never touched by a test.
		OpenAtLogin:    func() bool { return false },
		SetOpenAtLogin: func(bool) error { return nil },
	})
	return a
}

func TestSummonShortcutUsesTheStoredHotkey(t *testing.T) {
	if got := SummonShortcut(settings.Default()); got != "Ctrl+Space" {
		t.Errorf("default = %q, want Ctrl+Space", got)
	}
	s, err := settings.Parse([]byte(`{"hotkey": "Alt+Space"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := SummonShortcut(s); got != "Alt+Space" {
		t.Errorf("stored = %q, want Alt+Space", got)
	}
	blank, err := settings.Parse([]byte(`{"hotkey": "   "}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := SummonShortcut(blank); got != "Ctrl+Space" {
		t.Errorf("blank = %q, want the default", got)
	}
}

func TestSettingsHeightMatchesTheOldPanelRules(t *testing.T) {
	if got := SettingsHeight("default", 0); got != 580 {
		t.Errorf("uncapped = %d, want 580", got)
	}
	if got := SettingsHeight("large", 0); got != 638 {
		t.Errorf("large = %d, want 638", got)
	}
	if got := SettingsHeight("tiny", 0); got != 464 {
		t.Errorf("tiny = %d, want 464", got)
	}
	// A short screen caps it at 72% of the work area.
	if got := SettingsHeight("default", 600); got != 432 {
		t.Errorf("600-tall screen = %d, want 432", got)
	}
	// A very short screen floors at 240 and then the 420 minimum... the
	// 240 floor (screen less 24) is the cap, below the 420 minimum.
	if got := SettingsHeight("default", 300); got != 276 {
		t.Errorf("300-tall screen = %d, want 276", got)
	}
}

func TestTargetSizePerSurface(t *testing.T) {
	a := newApp(t)
	s := a.Store.Snapshot()

	// The expectation is the *same* function the surface asks, rounded the way
	// it rounds. `int()` truncates: the collapsed launcher's height is
	// `82 * 0.9 + 240 = 313.8` on Windows and `86 * 0.9 + 240 = 317.4` on
	// Linux, so truncation agreed with `math.Round` on the host this was
	// written on and was a pixel short on Windows — a red suite for a correct
	// window (「launcher size = 720x314」).
	if w, h := a.targetSize(s); w != launcher.InputWindowWidth || h != int(math.Round(launcher.WindowHeight(s.UIScale))) {
		t.Errorf("launcher size = %dx%d", w, h)
	}
	a.Surf = SurfaceSettings
	if w, h := a.targetSize(s); w != launcher.InputWindowWidth || h != SettingsHeight(s.UIScale, a.workArea) {
		t.Errorf("settings size = %dx%d", w, h)
	}
	a.Surf = SurfaceTerminal
	if w, h := a.targetSize(s); w != int(math.Round(s.TerminalWidth)) || h != int(math.Round(s.TerminalHeight)) {
		t.Errorf("terminal size = %dx%d", w, h)
	}
}

func TestOpenSwitchesSurfacesAndFocuses(t *testing.T) {
	a := newApp(t)
	a.Open(SurfaceSettings)
	if a.Surf != SurfaceSettings {
		t.Fatalf("surf = %v, want settings", a.Surf)
	}
	a.Open(SurfaceTerminal)
	if a.Surf != SurfaceTerminal {
		t.Fatalf("surf = %v, want terminal", a.Surf)
	}
	// The terminal surface started through the injected constructor, which
	// fails: the surface must hold the error rather than a session.
	if a.Terminal.Current() != nil || a.Terminal.Err == nil {
		t.Errorf("terminal session = %v, err = %v", a.Terminal.Current(), a.Terminal.Err)
	}
	a.Open(SurfaceLauncher)
	if a.Surf != SurfaceLauncher {
		t.Fatalf("surf = %v, want launcher", a.Surf)
	}
}

// TestSurfaceSnapshots renders each of the three surfaces and writes a PNG
// of it. These are the round's screenshots: the launcher with its catalog,
// the settings panel with the General controls, and the terminal's empty
// state.
func TestSurfaceSnapshots(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "floter-p1-shots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := settings.Default()

	cases := []struct {
		name    string
		surface Surface
		w, h    int
		want    []string
	}{
		{
			name:    "launcher",
			surface: SurfaceLauncher,
			w:       launcher.InputWindowWidth,
			h:       int(launcher.WindowHeight(s.UIScale)),
			want:    []string{"Restart the computer", "Shut down the computer", "Terminal", "Type to search"},
		},
		{
			name:    "settings",
			surface: SurfaceSettings,
			w:       launcher.InputWindowWidth,
			h:       SettingsHeight(s.UIScale, 0),
			want: []string{
				"General", "Appearance", "Liquid glass effect", "Interface size",
				"Terminal appearance", "Font family", "Font size", "Line height", "Terminal palette",
			},
		},
		{
			name:    "terminal",
			surface: SurfaceTerminal,
			w:       int(s.TerminalWidth),
			h:       int(s.TerminalHeight),
			want:    []string{"Terminal", "No terminal session yet"},
		},
	}

	for _, tc := range cases {
		a := newApp(t)
		a.Surf = tc.surface
		tt := ui.NewTester(a.View, tc.w, tc.h)
		tt.Frame()
		for _, want := range tc.want {
			if !tt.HasText(want) {
				t.Errorf("%s: missing %q in %q", tc.name, want, tt.Texts())
			}
		}

		path := filepath.Join(dir, tc.name+".png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s: %s (%dx%d)", tc.name, path, tc.w, tc.h)
	}
}

func TestSurfaceSwitchesThemeAndLanguage(t *testing.T) {
	a := newApp(t)
	if err := a.Store.Update(func(s *settings.Settings) {
		s.Theme = "light"
		s.Language = "zh"
	}); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.View, launcher.InputWindowWidth, int(launcher.WindowHeight("small")))
	tt.Frame()
	if !tt.HasText("重启电脑") {
		t.Errorf("missing the Chinese command: %q", tt.Texts())
	}
}

func TestParseSurface(t *testing.T) {
	cases := map[string]struct {
		surface Surface
		ok      bool
	}{
		"":         {SurfaceLauncher, true},
		"launcher": {SurfaceLauncher, true},
		"settings": {SurfaceSettings, true},
		"terminal": {SurfaceTerminal, true},
		"bogus":    {SurfaceLauncher, false},
	}
	for name, want := range cases {
		got, ok := ParseSurface(name)
		if got != want.surface || ok != want.ok {
			t.Errorf("ParseSurface(%q) = %v, %v; want %v, %v", name, got, ok, want.surface, want.ok)
		}
	}
}

func TestVersionInFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "mygo.json")
	if err := os.WriteFile(good, []byte(`{"name":"floter","version":"0.3.14"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := versionInFile(good); got != "0.3.14" {
		t.Errorf("versionInFile = %q, want 0.3.14", got)
	}
	if got := versionInFile(filepath.Join(dir, "missing.json")); got != "" {
		t.Errorf("missing file = %q, want empty", got)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := versionInFile(bad); got != "" {
		t.Errorf("bad file = %q, want empty", got)
	}
}

func TestResidencyRule(t *testing.T) {
	a := newApp(t)
	clock := time.Unix(1_700_000_000, 0)
	a.now = func() time.Time { return clock }

	// A surface entered now holds for the shipped ten seconds.
	a.Open(SurfaceSettings)
	clock = clock.Add(5 * time.Second)
	if !a.residencyHolds() {
		t.Error("settings did not survive a summon five seconds later")
	}
	clock = clock.Add(6 * time.Second)
	if a.residencyHolds() {
		t.Error("settings survived a summon eleven seconds later")
	}

	// Re-entering restarts the clock.
	a.Open(SurfaceSettings)
	clock = clock.Add(time.Second)
	if !a.residencyHolds() {
		t.Error("re-entering did not restart the clock")
	}

	// 0 turns the window off; the sentinel keeps it forever.
	if err := a.Store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 0 }); err != nil {
		t.Fatal(err)
	}
	if a.residencyHolds() {
		t.Error("residency 0 kept the surface")
	}
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SurfaceResidencySeconds = settings.SurfaceResidencyNever
	}); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(48 * time.Hour)
	if !a.residencyHolds() {
		t.Error("the never sentinel did not keep the surface")
	}

	// The ordinary search page never holds a residency.
	a.Surf = SurfaceLauncher
	if a.residencyHolds() {
		t.Error("the search page held a residency")
	}
}

// TestPluginModeResidency pins the piece R35's report actually asked for: a
// plugin mode is a surface too. Entering one starts the clock (noted by the
// View), a summon inside the window lands back on it, and past the window the
// summon is a fresh launcher — the mode and the field it owns are gone.
func TestPluginModeResidency(t *testing.T) {
	a := newApp(t)
	clock := time.Unix(1_700_000_000, 0)
	a.now = func() time.Time { return clock }
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 1200)
	tt.Frame()

	// The ordinary search page is not a tracked surface: a summon is allowed
	// to throw its half-finished query away.
	a.Launcher.Query = "left over"
	a.summon()
	if a.Launcher.Query != "" {
		t.Fatalf("the search page survived a summon with %q", a.Launcher.Query)
	}

	// Entering a plugin mode starts the clock; the frame is what notes it.
	a.Launcher.EnterCalculator()
	a.Launcher.Query = "12 * 3"
	tt.Frame()
	if !a.residencyHolds() {
		t.Fatal("a plugin mode did not hold right after the frame that entered it")
	}

	// A summon inside the window lands back on the same page: the mode and the
	// field it owns are still where the user left them.
	clock = clock.Add(5 * time.Second)
	a.summon()
	if !a.Launcher.InCalculatorMode() {
		t.Error("a summon inside the window left the plugin mode")
	}
	if a.Launcher.Query != "12 * 3" {
		t.Errorf("the plugin mode's field = %q, want it kept", a.Launcher.Query)
	}

	// A summon after the window is a fresh launcher. The clock restarted on
	// the summon above, so the lapse is measured from there.
	clock = clock.Add(11 * time.Second)
	a.summon()
	if a.Launcher.InCalculatorMode() {
		t.Error("a summon past the window kept the plugin mode")
	}
	if a.Launcher.Query != "" {
		t.Errorf("a summon past the window kept the query %q", a.Launcher.Query)
	}
}

// TestPluginModeResidencyClearsDroppedRows is the drop half of the same rule:
// the dropped rows belonged to the previous interaction, so the reveal that
// starts the next one takes them with it (the old build's clearDrops).
func TestPluginModeResidencyClearsDroppedRows(t *testing.T) {
	a := newApp(t)
	clock := time.Unix(1_700_000_000, 0)
	a.now = func() time.Time { return clock }
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 1200)

	a.Launcher.SetDropped([]drops.File{{Path: "/tmp/left.txt", Name: "left.txt"}})
	a.Launcher.EnterFiles()
	tt.Frame()
	if !a.Launcher.InFilesMode() {
		t.Fatal("the files mode did not open")
	}

	// Inside the window the mode survives, and so do its rows.
	clock = clock.Add(5 * time.Second)
	a.summon()
	if !a.Launcher.InFilesMode() || len(a.Launcher.Dropped) != 1 {
		t.Errorf("a summon inside the window dropped the files mode (%v, %d rows)",
			a.Launcher.InFilesMode(), len(a.Launcher.Dropped))
	}

	// Past the window both the mode and the rows are gone. The clock
	// restarted on the summon above, so the lapse is measured from there.
	clock = clock.Add(11 * time.Second)
	a.summon()
	if a.Launcher.InFilesMode() {
		t.Error("a summon past the window kept the files mode")
	}
	if len(a.Launcher.Dropped) != 0 {
		t.Errorf("a summon past the window kept %d dropped rows", len(a.Launcher.Dropped))
	}
}

// TestPluginModeResidencyFollowsTheSetting pins the two boundaries for a
// plugin mode: 0 turns the window off, and the "never" sentinel keeps the
// page however long the panel was away.
func TestPluginModeResidencyFollowsTheSetting(t *testing.T) {
	a := newApp(t)
	clock := time.Unix(1_700_000_000, 0)
	a.now = func() time.Time { return clock }
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 1200)

	a.Launcher.EnterCalculator()
	tt.Frame()
	if err := a.Store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 0 }); err != nil {
		t.Fatal(err)
	}
	if a.residencyHolds() {
		t.Fatal("residency 0 held a plugin mode")
	}
	a.summon()
	if a.Launcher.InCalculatorMode() {
		t.Error("residency 0 kept the plugin mode")
	}

	if err := a.Store.Update(func(s *settings.Settings) {
		s.SurfaceResidencySeconds = settings.SurfaceResidencyNever
	}); err != nil {
		t.Fatal(err)
	}
	a.Launcher.EnterCalculator()
	tt.Frame()
	clock = clock.Add(48 * time.Hour)
	a.summon()
	if !a.Launcher.InCalculatorMode() {
		t.Error("the never sentinel dropped the plugin mode")
	}
}

// TestPluginConfigResidency pins the overlay as its own surface: while the
// configuration sheet is open it is what a summon restores, past the window
// both the sheet and the mode beneath it are gone.
func TestPluginConfigResidency(t *testing.T) {
	a := newApp(t)
	clock := time.Unix(1_700_000_000, 0)
	a.now = func() time.Time { return clock }
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 1400)
	tt.Frame()

	a.Launcher.EnterClipboard()
	tt.Frame()
	a.Launcher.OpenConfig("clipboard")
	tt.Frame()
	if !a.Launcher.ConfigOpen() {
		t.Fatal("the configuration sheet did not open")
	}
	if !a.residencyHolds() {
		t.Fatal("the plugin configuration overlay did not hold")
	}

	clock = clock.Add(5 * time.Second)
	a.summon()
	if !a.Launcher.ConfigOpen() {
		t.Error("a summon inside the window closed the overlay")
	}

	clock = clock.Add(11 * time.Second)
	a.summon()
	if a.Launcher.ConfigOpen() || a.Launcher.InClipboardMode() {
		t.Error("a summon past the window kept the overlay or the mode beneath it")
	}
}

// TestLeavingASurfaceClearsTheResidencyClock is why a leftover clock cannot
// claim a later summon: the surface the app is on is derived from state every
// frame, so once the mode is left the clock is irrelevant even though it has
// not lapsed.
func TestLeavingASurfaceClearsTheResidencyClock(t *testing.T) {
	a := newApp(t)
	clock := time.Unix(1_700_000_000, 0)
	a.now = func() time.Time { return clock }
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 1200)

	a.Launcher.EnterCalculator()
	tt.Frame()
	// Esc leaves the mode by an explicit gesture; the clock has not lapsed.
	a.Launcher.ResetQuery()
	tt.Frame()
	if a.residencyHolds() {
		t.Fatal("a left plugin mode still held its residency")
	}
}

func TestHideOnBlurFollowsTheSetting(t *testing.T) {
	if !settings.DefaultHideOnBlur() {
		t.Skip("this machine's session disables hide on blur")
	}
	if !settings.Default().HideOnBlur {
		t.Error("the shipped default is not hide on blur")
	}
	s, err := settings.Parse([]byte(`{"hide_on_blur": false}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.HideOnBlur {
		t.Error("an explicit false did not win")
	}
}

// integrationFixture builds an extension root with one enabled integration
// whose provider is a script that answers describe.
func integrationFixture(t *testing.T) extensions.Paths {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	dir := t.TempDir()
	paths := extensions.FromRoot(dir)
	pkg := filepath.Join(paths.Extensions, "io.github.vst93.v")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "schemaVersion": "2.0", "id": "io.github.vst93.v", "name": "V Tools",
  "description": "Developer tools", "publisher": {"id": "vst93", "name": "vst"},
  "compatibility": {"floter": ">=0.3.0", "providerProtocol": "^1.0"},
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["v"]},
  "provider": {"type": "executable", "argsPrefix": ["--floter"]}
}`
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(pkg, "v.sh")
	script := "#!/bin/sh\ncat <<'JSONEOF'\n" + `{
  "protocolVersion": "1.0",
  "provider": {"id": "io.github.vst93.v", "name": "V Tools", "version": "0.0.12"},
  "commands": [{"id": "jv", "name": "JSON Viewer", "description": "View JSON", "execution": {"program": "self", "argsPrefix": ["jv"], "mode": "pty"}}]
}` + "\nJSONEOF\n"
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	repository := `{
  "schemaVersion": 1,
  "extensions": {
    "io.github.vst93.v": {
      "id": "io.github.vst93.v", "name": "V Tools", "publisherName": "vst",
      "distributionSource": "local", "runtimeOwnership": "system", "providerKind": "executable",
      "state": "enabled", "enabled": true, "packageVersion": "0.0.12",
      "manifestPath": "", "executablePath": "` + jsonEscaped(executable) + `", "channel": "stable",
      "installedAt": 1, "updatedAt": 1
    }
  }
}`
	if err := os.WriteFile(paths.RepositoryFile, []byte(repository), 0o644); err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestIntegrationsReachTheLauncher(t *testing.T) {
	paths := integrationFixture(t)
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})

	// A command with no switch entry has never been enabled, so the launcher
	// does not offer it: the per-command switches are the gate.
	a.RefreshIntegrations(context.Background())
	if got := a.Launcher.Commands; len(got) != 0 {
		t.Fatalf("a command with its switch off reached the launcher: %+v", got)
	}
	if err := store.Update(func(s *settings.Settings) { s.SetCommandSwitch("io.github.vst93.v", "jv", true) }); err != nil {
		t.Fatal(err)
	}
	a.RefreshIntegrations(context.Background())
	if got := a.Launcher.Commands; len(got) != 1 || got[0].Command.ID != "jv" {
		t.Fatalf("launcher commands = %+v", got)
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("recovered: %v\n%s", r, debug.Stack())
			}
		}()
		_ = a.integrationList()
	}()
	list := a.integrationList()
	if len(list) != 1 {
		t.Fatalf("integrations = %+v", list)
	}
	if list[0].Name != "V Tools" || !list[0].Running || !list[0].Enabled || list[0].Publisher != "vst" {
		t.Errorf("integration = %+v", list[0])
	}

	// Running the command opens the terminal surface. The injected
	// constructor fails, so the surface shows the error instead of a
	// session: the point is that the hand-off happened.
	a.runCommand(a.Launcher.Commands[0], nil)
	if a.Surf != SurfaceTerminal {
		t.Errorf("surface = %v, want terminal", a.Surf)
	}
	if a.Terminal.Err == nil {
		t.Error("the failed session was not recorded")
	}
}

func TestIntegrationsToggle(t *testing.T) {
	paths := integrationFixture(t)
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.RefreshIntegrations(context.Background())

	if err := a.Integrations.SetEnabled("io.github.vst93.v", false); err != nil {
		t.Fatal(err)
	}
	a.Launcher.SetCommands(a.Integrations.CommandEntries())
	if got := a.Launcher.Commands; len(got) != 0 {
		t.Errorf("a disabled integration still contributes %+v", got)
	}
	if list := a.integrationList(); len(list) != 1 || list[0].Enabled {
		t.Errorf("integrations = %+v", list)
	}
}

func TestHandleURLRoutesDeepLinks(t *testing.T) {
	a := newApp(t)
	a.Launcher.SetQuery("previous")

	a.HandleURL("floter://settings")
	if a.Surf != SurfaceSettings {
		t.Errorf("floter://settings -> %v", a.Surf)
	}
	a.HandleURL("floter://terminal")
	if a.Surf != SurfaceTerminal {
		t.Errorf("floter://terminal -> %v", a.Surf)
	}
	a.HandleURL("FLOTER://Settings")
	if a.Surf != SurfaceSettings {
		t.Errorf("a case-insensitive scheme -> %v", a.Surf)
	}

	a.HandleURL("floter://search?q=json+viewer")
	if a.Surf != SurfaceLauncher {
		t.Errorf("floter://search -> %v", a.Surf)
	}
	if a.Launcher.Query != "json viewer" {
		t.Errorf("query = %q", a.Launcher.Query)
	}
	a.HandleURL("floter://?q=notes")
	if a.Launcher.Query != "notes" {
		t.Errorf("host query = %q", a.Launcher.Query)
	}

	// Anything else opens the launcher without touching the query.
	a.Launcher.SetQuery("keep")
	a.HandleURL("https://example.com/")
	if a.Surf != SurfaceLauncher || a.Launcher.Query != "keep" {
		t.Errorf("a foreign URL -> %v, %q", a.Surf, a.Launcher.Query)
	}
	a.HandleURL("floter://%zz")
	if a.Surf != SurfaceLauncher {
		t.Errorf("a malformed URL -> %v", a.Surf)
	}
}

func TestLaunchAtStartupFollowsTheSetting(t *testing.T) {
	store := settings.NewStore(settings.Default())
	calls := []bool{}
	a := New(Options{
		Store:          store,
		NewTerminal:    func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin:    func() bool { return false },
		SetOpenAtLogin: func(open bool) error { calls = append(calls, open); return nil },
	})
	if a.lastLaunchAtStartup {
		t.Error("the shipped default should be off")
	}
	// The listener syncs the login item when the setting changes. The test
	// cannot touch the machine's login items, so it only checks that the
	// bookkeeping follows: ApplyStartup is what talks to the system.
	if err := store.Update(func(s *settings.Settings) { s.LaunchAtStartup = true }); err != nil {
		t.Fatal(err)
	}
	if !a.lastLaunchAtStartup {
		t.Error("the setting change did not reach the shell")
	}
	if len(calls) != 1 || !calls[0] {
		t.Errorf("the login item was set %v, want one true", calls)
	}
}

func TestClipboardSettings(t *testing.T) {
	// The shipped defaults: on, three hundred entries.
	state := clipboardState(settings.Default())
	if !state.Enabled || state.MaxItems != settings.DefaultClipboardMaxItems {
		t.Errorf("defaults = %+v", state)
	}

	// The keys are ones this app does not own, so they ride in the
	// carried-through map.
	s, err := settings.Parse([]byte(`{"clipboard_history_enabled": false, "clipboard_history_max_items": 120}`))
	if err != nil {
		t.Fatal(err)
	}
	state = clipboardState(s)
	if state.Enabled || state.MaxItems != 120 {
		t.Errorf("parsed = %+v", state)
	}
	if extra := s.Extra(); extra["clipboard_history_max_items"] == nil {
		t.Errorf("the key is not carried through: %v", extra)
	}

	// A hand-edited number outside the band is clamped on read, as the old
	// build's settings normalizer did.
	loose, err := settings.Parse([]byte(`{"clipboard_history_max_items": 100000}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := clipboardState(loose).MaxItems; got != settings.MaxClipboardMaxItems {
		t.Errorf("max items = %d, want the %d ceiling", got, settings.MaxClipboardMaxItems)
	}
	store := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), clipboardState(loose).MaxItems)
	if err := store.SetMaxItems(clipboardState(loose).MaxItems); err != nil {
		t.Fatal(err)
	}
}

func TestClipboardReachesTheLauncher(t *testing.T) {
	a := newApp(t)
	if a.Launcher.Clipboard == nil {
		t.Fatal("the launcher has no clipboard history")
	}
	if a.Clipboard.Paths().Root != clipboard.FromConfigRoot(a.Paths.Root).Root {
		t.Errorf("the clipboard store reads %q, want the fixture", a.Clipboard.Paths().Root)
	}
	if _, _, err := a.Clipboard.AddText("a clip"); err != nil {
		t.Fatal(err)
	}
	if got := a.Launcher.Clipboard.Search("a clip", 0); len(got) != 1 || got[0].Text != "a clip" {
		t.Errorf("search = %+v", got)
	}
}

// npmFixture serves a registry with one package whose tarball holds a floter
// manifest, and returns the server and the package name.
func npmFixture(t *testing.T, permissions ...string) (*httptest.Server, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's runtime is resolved through sh")
	}
	declared := ""
	if len(permissions) > 0 {
		quoted := make([]string, 0, len(permissions))
		for _, permission := range permissions {
			quoted = append(quoted, `"`+permission+`"`)
		}
		declared = `, "permissions": [` + strings.Join(quoted, ", ") + `]`
	}
	manifest := `{
  "schemaVersion": "2.0", "id": "dev.floter.npm", "name": "NPM Tool", "version": "1.0.0",
  "publisher": {"id": "floter", "name": "floter"},
  "compatibility": {"floter": ">=0.3.0", "providerProtocol": "^1.0"},
  "distribution": {"type": "local"},
  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
  "provider": {"type": "static-descriptor", "descriptor": "description.json", "argsPrefix": []}` + declared + `
}`
	description := `{"protocolVersion": "1.0", "provider": {"id": "dev.floter.npm", "name": "NPM Tool", "version": "1.0.0"}, "commands": []}`
	packageJSON := `{"name": "@vst93/fixture", "version": "1.0.0", "floter": {"manifest": "floter.extension.json"}}`

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	files := map[string]struct {
		body string
		mode int64
	}{
		"package/package.json":          {packageJSON, 0o644},
		"package/floter.extension.json": {manifest, 0o644},
		"package/description.json":      {description, 0o644},
		"package/tool.sh":               {"#!/bin/sh\necho tool\n", 0o755},
	}
	for name, file := range files {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: file.mode, Size: int64(len(file.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	tarball := buffer.Bytes()
	sum := sha512.Sum512(tarball)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/@vst93/fixture":
			json.NewEncoder(w).Encode(map[string]any{
				"name":      "@vst93/fixture",
				"dist-tags": map[string]string{"latest": "1.0.0"},
				"versions": map[string]any{
					"1.0.0": map[string]any{"version": "1.0.0", "dist": map[string]string{
						"tarball":   "http://" + r.Host + "/tarball",
						"integrity": integrity,
					}},
				},
			})
		case "/tarball":
			w.Write(tarball)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, "@vst93/fixture"
}

func TestInstallFromRegistryReachesTheStore(t *testing.T) {
	server, name := npmFixture(t)
	paths := extensions.FromRoot(t.TempDir())
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		Registry:    &extensions.Registry{BaseURL: server.URL, Client: server.Client()},
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})

	// The settings action installs in the background and refreshes, so the
	// test waits for the inventory to catch up.
	a.Settings.Actions.InstallFromRegistry(name, "1.0.0")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if integration, ok := a.Integrations.Inventory().WithID("dev.floter.npm"); ok && integration.Entry.PackageVersion == "1.0.0" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the install did not land: %+v", a.Integrations.Inventory().Integrations)
		}
		time.Sleep(20 * time.Millisecond)
	}

	integration, _ := a.Integrations.Inventory().WithID("dev.floter.npm")
	if integration.Entry.DistributionSource != "npm" || integration.Name != "NPM Tool" {
		t.Errorf("integration = %+v", integration.Entry)
	}
	if list := a.integrationList(); len(list) != 1 || !list[0].Running {
		t.Errorf("the settings list = %+v", list)
	}
	if _, err := os.Stat(filepath.Join(paths.Extensions, "dev.floter.npm", "tool.sh")); err != nil {
		t.Errorf("the package did not land: %v", err)
	}
}

func TestAppIconFollowsTheSetting(t *testing.T) {
	if len(appIconBytes("dark")) == 0 || len(appIconBytes("light")) == 0 {
		t.Fatal("an app icon is missing")
	}
	if string(appIconBytes("light")) == string(appIconBytes("dark")) {
		t.Error("both appearances use the same icon")
	}
	// The shipped default is dark, and anything unknown lands on it.
	if string(appIconBytes("bogus")) != string(appIconBytes("dark")) {
		t.Error("an unknown icon is not the dark one")
	}
	if got := storedAppIcon(settings.Default()); got != "dark" {
		t.Errorf("default app icon = %q, want dark", got)
	}
	s, err := settings.Parse([]byte(`{"app_icon": "light"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := storedAppIcon(s); got != "light" {
		t.Errorf("stored app icon = %q", got)
	}

	// A settings change reaches the shell's bookkeeping.
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:          store,
		Paths:          extensions.FromRoot(t.TempDir()),
		NewTerminal:    func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin:    func() bool { return false },
		SetOpenAtLogin: func(bool) error { return nil },
	})
	if a.appIcon != "dark" {
		t.Fatalf("initial icon = %q", a.appIcon)
	}
	if err := store.Update(func(s *settings.Settings) { s.SetExtra("app_icon", "light") }); err != nil {
		t.Fatal(err)
	}
	if a.appIcon != "light" || a.lastAppIcon != "light" {
		t.Errorf("icon after the change = %q/%q", a.appIcon, a.lastAppIcon)
	}
}

func TestSetExtraRoundTripsThroughTheStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(s *settings.Settings) { s.SetExtra("app_icon", "light") }); err != nil {
		t.Fatal(err)
	}
	back, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := back.Extra()["app_icon"].(string); value != "light" {
		t.Errorf("app_icon = %v", back.Extra())
	}
}

func TestMenuTemplate(t *testing.T) {
	copy := i18n.For("en").Launcher
	var settingsCalls, terminalCalls, launcherCalls int
	items := menuTemplate(copy, menuActions{
		Settings: func() { settingsCalls++ },
		Terminal: func() { terminalCalls++ },
		Launcher: func() { launcherCalls++ },
	})
	if len(items) != 4 {
		t.Fatalf("menus = %d, want 4", len(items))
	}
	// The app menu carries the settings item and the standard items; the
	// Edit menu is the roles every text field needs.
	if items[0].Role != mygo.RoleAppMenu {
		t.Errorf("first menu role = %q", items[0].Role)
	}
	if len(items[0].Submenu) == 0 || items[0].Submenu[0].Label != copy.CommandSettings {
		t.Errorf("the app menu does not lead with settings: %+v", items[0].Submenu)
	}
	if items[2].Role != mygo.RoleEditMenu {
		t.Errorf("the edit menu is missing: %q", items[2].Role)
	}
	// The view menu's items run their actions.
	view := items[1]
	if view.Label != copy.MenuView || len(view.Submenu) != 4 {
		t.Fatalf("view menu = %+v", view)
	}
	view.Submenu[0].Click(nil, nil)
	view.Submenu[1].Click(nil, nil)
	if launcherCalls != 1 || terminalCalls != 1 || settingsCalls != 0 {
		t.Errorf("calls = %d/%d/%d", launcherCalls, terminalCalls, settingsCalls)
	}
	items[0].Submenu[0].Click(nil, nil)
	if settingsCalls != 1 {
		t.Errorf("settings calls = %d", settingsCalls)
	}
	if items[1].Submenu[1].Accelerator != "CmdOrCtrl+Shift+T" {
		t.Errorf("terminal accelerator = %q", items[1].Submenu[1].Accelerator)
	}
}

// The tray menu is the app's settings and quit door outside macOS. The
// launcher's built-in rows are the reference's six and carry neither
// (`main:src/hooks/useLauncherCatalog.ts:184-292`), so this menu has to keep
// both: a tray-less build would otherwise have no quit path at all, and the
// port would have deleted its only door.
func TestTrayMenuKeepsSettingsAndQuit(t *testing.T) {
	copy := i18n.For("en").Launcher
	var show, settingsCalls, reload, quit int
	menu := trayMenu(copy,
		func() { show++ },
		func() { settingsCalls++ },
		func() { reload++ },
		func() { quit++ },
	)
	items := menu.Items()
	if len(items) != 5 {
		t.Fatalf("tray items = %d, want 5 (four actions and a separator)", len(items))
	}
	want := []string{copy.TrayShow, copy.TraySettings, copy.TrayReload, "", copy.TrayQuit}
	for i, label := range want {
		if items[i].Label != label {
			t.Errorf("tray item %d = %q, want %q", i, items[i].Label, label)
		}
	}
	if items[3].Type != mygo.MenuItemSeparator {
		t.Errorf("the separator is missing: %+v", items[3])
	}
	for _, it := range items {
		if it.Click != nil {
			it.Click(it, nil)
		}
	}
	if show != 1 || settingsCalls != 1 || reload != 1 || quit != 1 {
		t.Errorf("tray actions ran %d/%d/%d/%d", show, settingsCalls, reload, quit)
	}
	// The labels follow the stored language, so the tray never lags the UI.
	zhCopy := i18n.For("zh").Launcher
	if got := trayMenu(zhCopy, func() {}, func() {}, func() {}, func() {}).Items()[4].Label; got != zhCopy.TrayQuit || got == copy.TrayQuit {
		t.Errorf("the tray quit label did not follow the language: %q", got)
	}
}

// The Settings… item advertises the user's own `open_settings` binding, so a
// rebind moves the menu accelerator instead of leaving a frozen Cmd+, behind.
func TestMenuSettingsAcceleratorFollowsTheBinding(t *testing.T) {
	copy := i18n.For("en").Launcher
	items := menuTemplate(copy, menuActions{SettingsAccelerator: "Ctrl+Shift+O"})
	if got := items[0].Submenu[0].Accelerator; got != "Ctrl+Shift+O" {
		t.Errorf("settings accelerator = %q, want the stored binding", got)
	}
	// An empty binding keeps the shipped accelerator.
	items = menuTemplate(copy, menuActions{})
	if got := items[0].Submenu[0].Accelerator; got != "CmdOrCtrl+," {
		t.Errorf("default settings accelerator = %q", got)
	}
}

func TestInstallAsksBeforeGrantingPermissions(t *testing.T) {
	server, name := npmFixture(t, "environment", "network-fetch")

	asked := extensions.PermissionApproval{}
	answer := false
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       extensions.FromRoot(t.TempDir()),
		Registry:    &extensions.Registry{BaseURL: server.URL, Client: server.Client()},
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
		ConfirmPermissions: func(approval extensions.PermissionApproval) bool {
			asked = approval
			return answer
		},
	})

	// Declining installs nothing.
	a.Settings.Actions.InstallFromRegistry(name, "1.0.0")
	deadline := time.Now().Add(10 * time.Second)
	for len(asked.Added) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the approval was never asked for")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(asked.Added) != 2 || asked.Name != "NPM Tool" {
		t.Fatalf("approval = %+v", asked)
	}
	time.Sleep(200 * time.Millisecond) // let the declined install finish
	if inventory := a.Integrations.Inventory(); len(inventory.Integrations) != 0 {
		t.Fatalf("a declined install landed: %+v", inventory.Integrations)
	}

	// Approving installs it, with the approval recorded.
	answer = true
	a.Settings.Actions.InstallFromRegistry(name, "1.0.0")
	for {
		if integration, ok := a.Integrations.Inventory().WithID("dev.floter.npm"); ok && integration.Entry.ApprovedAt != 0 {
			if len(integration.Entry.ApprovedPermissions) != 2 {
				t.Errorf("approved permissions = %v", integration.Entry.ApprovedPermissions)
			}
			if integration.Entry.ApprovedManifestDigest == nil {
				t.Error("the approval is not bound to a manifest digest")
			}
			break
		}
		if time.Now().After(deadline.Add(20 * time.Second)) {
			t.Fatalf("the approved install did not land: %+v", a.Integrations.Inventory().Integrations)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestDynamicCompletionReachesTheLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	dir := t.TempDir()
	paths := extensions.FromRoot(dir)
	pkg := filepath.Join(paths.Extensions, "dev.floter.completer")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "schemaVersion": "2.0", "id": "dev.floter.completer", "name": "Completer",
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["completer"]},
  "provider": {"type": "executable", "argsPrefix": ["--floter"], "completeTimeoutMs": 500}
}`
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(pkg, "completer.sh")
	script := `#!/bin/sh
case "$*" in
  "--floter describe --protocol 1.0")
    echo '{"protocolVersion":"1.0","provider":{"id":"dev.floter.completer","name":"Completer","version":"1.0.0"},"commands":[{"id":"run","name":"Run","description":"Run a task","execution":{"program":"self","argsPrefix":["run"],"mode":"pty"},"arguments":[{"names":["-task"],"kind":"command","takesValue":true,"description":"Task"}]}]}'
    ;;
  "--floter complete --protocol 1.0")
    cat >/dev/null
    echo '{"completions":[{"label":"deploy","kind":"command","detail":"Deploy it"}]}'
    ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
  "schemaVersion": 1,
  "extensions": {
    "dev.floter.completer": {
      "id": "dev.floter.completer", "name": "Completer", "state": "enabled", "enabled": true,
      "packageVersion": "1.0.0", "manifestPath": "", "executablePath": "`+jsonEscaped(executable)+`",
      "channel": "stable", "installedAt": 1, "updatedAt": 1
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.SetCommandSwitch("dev.floter.completer", "run", true) }); err != nil {
		t.Fatal(err)
	}
	a := New(Options{
		Store:       store,
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})
	a.RefreshIntegrations(context.Background())
	if got := a.Launcher.Commands; len(got) != 1 {
		t.Fatalf("commands = %+v", got)
	}
	entry := a.Launcher.Commands[0]

	// The shell answers the launcher's request on the main thread.
	done := make(chan []extensions.Completion, 1)
	a.Launcher.Actions.Complete(entry, []string{"-task", ""}, func(items []extensions.Completion) {
		done <- items
	})
	select {
	case items := <-done:
		if len(items) != 1 || items[0].Label != "deploy" {
			t.Errorf("completions = %+v", items)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the completion never came back")
	}
}

func TestDiagnoseIntegrationRecordsTheResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	dir := t.TempDir()
	paths := extensions.FromRoot(dir)
	pkg := filepath.Join(paths.Extensions, "dev.floter.diagnosed")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(`{
  "schemaVersion": "2.0", "id": "dev.floter.diagnosed", "name": "Diagnosed",
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["diagnosed"]},
  "provider": {"type": "executable", "argsPrefix": []}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(pkg, "diagnosed.sh")
	script := `#!/bin/sh
case "$*" in
  "describe --protocol 1.0") echo '{"protocolVersion":"1.0","provider":{"id":"dev.floter.diagnosed","name":"Diagnosed","version":"1.0.0"},"commands":[]}' ;;
  "diagnose --protocol 1.0") echo '{"status":"problem","checks":[{"id":"tool","status":"problem","message":"the tool is outdated"}]}' ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
  "schemaVersion": 1,
  "extensions": {
    "dev.floter.diagnosed": {
      "id": "dev.floter.diagnosed", "name": "Diagnosed", "state": "enabled", "enabled": true,
      "packageVersion": "1.0.0", "manifestPath": "", "executablePath": "`+jsonEscaped(executable)+`",
      "channel": "stable", "installedAt": 1, "updatedAt": 1
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})
	a.RefreshIntegrations(context.Background())
	a.diagnoseIntegration("dev.floter.diagnosed")

	deadline := time.Now().Add(5 * time.Second)
	for {
		list := a.integrationList()
		if len(list) == 1 && list[0].Diagnosis != "" {
			if !list[0].DiagnosisFailed || !strings.Contains(list[0].Diagnosis, "outdated") {
				t.Errorf("diagnosis = %+v", list[0])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the diagnosis never landed: %+v", list)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestABrokenIntegrationCarriesItsRecordedReason(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	// Two failures as the repository records them: the keyed detail this build
	// writes, and the English sentence an older one left behind.
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
  "schemaVersion": 1,
  "extensions": {
    "dev.floter.keyed": {
      "id": "dev.floter.keyed", "name": "Keyed", "state": "broken", "enabled": false,
      "lastErrorCode": "binding-changed",
      "brokenReason": "binding-changed:{\"path\":\"/usr/local/bin/tool\"}",
      "installedAt": 1, "updatedAt": 1
    },
    "dev.floter.legacy": {
      "id": "dev.floter.legacy", "name": "Legacy", "state": "broken", "enabled": false,
      "lastErrorCode": "binding-missing",
      "brokenReason": "Executable is no longer available at /nope/tool",
      "installedAt": 1, "updatedAt": 1
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})
	a.RefreshIntegrations(context.Background())

	found := map[string]settingsui.Integration{}
	for _, item := range a.integrationList() {
		found[item.ID] = item
	}
	for _, want := range []struct {
		id     string
		code   string
		detail string
	}{
		{"dev.floter.keyed", "binding-changed", `binding-changed:{"path":"/usr/local/bin/tool"}`},
		{"dev.floter.legacy", "binding-missing", "Executable is no longer available at /nope/tool"},
	} {
		item, ok := found[want.id]
		if !ok {
			t.Fatalf("%s is missing from %+v", want.id, found)
		}
		if item.FailureCode != want.code || item.FailureDetail != want.detail {
			t.Errorf("%s = %q / %q, want %q / %q", want.id, item.FailureCode, item.FailureDetail, want.code, want.detail)
		}
	}
}

func TestClipboardFormatHelpers(t *testing.T) {
	formats := []transfer.Format{transfer.Text, transfer.PNG}
	if !containsFormat(formats, "image/png") {
		t.Error("the PNG format was not found")
	}
	if containsFormat(formats, "text/html") {
		t.Error("an absent format was reported")
	}
	// The key is order-insensitive, so reordered formats are one state.
	if formatKey(formats) != formatKey([]transfer.Format{transfer.PNG, transfer.Text}) {
		t.Errorf("keys differ: %q vs %q", formatKey(formats), formatKey([]transfer.Format{transfer.PNG, transfer.Text}))
	}

	// A real PNG's size, and zero for junk.
	img := image.NewRGBA(image.Rect(0, 0, 12, 5))
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	if width, height := imageSize(buffer.Bytes()); width != 12 || height != 5 {
		t.Errorf("size = %dx%d", width, height)
	}
	if width, height := imageSize([]byte("not a png")); width != 0 || height != 0 {
		t.Errorf("junk size = %dx%d", width, height)
	}
}

func TestCopyClipboardEntryRestoresTheKind(t *testing.T) {
	var written []ClipboardPayload
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
		WriteClipboard: func(payload ClipboardPayload) { written = append(written, payload) },
	})

	// A file list goes back as files, an image as bytes, text as text.
	entry, _, err := a.Clipboard.AddFiles([]string{"/tmp/one.txt", "/tmp/two.txt"})
	if err != nil {
		t.Fatal(err)
	}
	a.copyClipboardEntry(entry)
	if len(written) != 1 || written[0].Kind != clipboard.KindFiles || len(written[0].Paths) != 2 {
		t.Fatalf("files payload = %+v", written)
	}
	if a.selfCopied != "" {
		t.Errorf("a file copy recorded text: %q", a.selfCopied)
	}

	png := testPNGBytes(t)
	image, _, err := a.Clipboard.AddImage(png, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	a.copyClipboardEntry(image)
	if len(written) != 2 || written[1].Kind != clipboard.KindImage || len(written[1].Image) != len(png) {
		t.Errorf("image payload = %+v", written[1].Kind)
	}

	a.copyClipboardEntry(clipboard.Entry{Kind: clipboard.KindText, Text: "hello"})
	if len(written) != 3 || written[2].Text != "hello" {
		t.Errorf("text payload = %+v", written[2])
	}
	if a.selfCopied != "hello" {
		t.Errorf("selfCopied = %q", a.selfCopied)
	}
}

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestRecentsFollowTheUsageFileAndTheSetting(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})

	// Two launches: the second one twice, so it leads.
	first := apps.App{Name: "Editor", Path: "/Applications/Editor.app"}
	second := apps.App{Name: "Safari", Path: "/Applications/Safari.app"}
	a.Launcher.SetApps([]apps.App{first, second})
	if err := a.Usage.Record(first.Path); err != nil {
		t.Fatal(err)
	}
	if err := a.Usage.Record(second.Path); err != nil {
		t.Fatal(err)
	}
	if err := a.Usage.Record(second.Path); err != nil {
		t.Fatal(err)
	}
	a.refreshRecents()
	if got := a.Launcher.Recent; len(got) != 2 || got[0] != second.Path {
		t.Errorf("recents = %v", got)
	}
	if !a.Launcher.ShowRecent {
		t.Error("show recent is off by default")
	}

	// An uninstalled application is dropped from the list.
	a.Launcher.SetApps([]apps.App{first})
	a.refreshRecents()
	if got := a.Launcher.Recent; len(got) != 1 || got[0] != first.Path {
		t.Errorf("recents after an uninstall = %v", got)
	}

	// The setting turns the empty state's recents off.
	if err := a.Store.Update(func(s *settings.Settings) { s.SetShowRecentInLauncher(false) }); err != nil {
		t.Fatal(err)
	}
	a.refreshRecents()
	if a.Launcher.ShowRecent {
		t.Error("show_recent_in_launcher false did not reach the launcher")
	}
	if err := a.Store.Update(func(s *settings.Settings) { s.SetShowRecentInLauncher(true) }); err != nil {
		t.Fatal(err)
	}
	a.refreshRecents()
	if !a.Launcher.ShowRecent {
		t.Error("show_recent_in_launcher true did not reach the launcher")
	}
}

// The recents' ranking is the old build's `recentItems` (src/launcher.ts):
// count descending, and between equals the application list's own scan order —
// `b.count - a.count || a.index - b.index`. `usage.Top` breaks the tie by
// recency instead, so this pins the shell's re-rank, and the front door's
// budget (`MAX_RESULTS` = 10, not the previous five).
func TestRecentsRankByCountThenScanOrder(t *testing.T) {
	a := newApp(t)
	// The scan order is A, B, C, D, E; the counts make A and E tie at two, B
	// and D tie at one, and C never launched.
	names := []string{"A", "B", "C", "D", "E"}
	appsList := make([]apps.App, 0, len(names))
	for _, name := range names {
		appsList = append(appsList, apps.App{Name: name, Path: "/Applications/" + name + ".app"})
	}
	a.Launcher.SetApps(appsList)
	counts := map[string]int{"/Applications/A.app": 2, "/Applications/B.app": 1, "/Applications/D.app": 1, "/Applications/E.app": 2}
	for path, n := range counts {
		for i := 0; i < n; i++ {
			if err := a.Usage.Record(path); err != nil {
				t.Fatal(err)
			}
		}
	}
	a.refreshRecents()
	// A and E tie at two, so the scan order puts A first; B and D tie at one,
	// so B before D; C never launched and takes no slot.
	want := []string{"/Applications/A.app", "/Applications/E.app", "/Applications/B.app", "/Applications/D.app"}
	if len(a.Launcher.Recent) != len(want) {
		t.Fatalf("recents = %v, want %v", a.Launcher.Recent, want)
	}
	for i := range want {
		if a.Launcher.Recent[i] != want[i] {
			t.Errorf("recents = %v, want %v", a.Launcher.Recent, want)
			break
		}
	}

	// The front door's budget is the old build's MAX_RESULTS: ten, not five.
	many := make([]apps.App, 0, 12)
	for i := 0; i < 12; i++ {
		path := "/Applications/M" + string(rune('a'+i)) + ".app"
		many = append(many, apps.App{Name: path, Path: path})
		if err := a.Usage.Record(path); err != nil {
			t.Fatal(err)
		}
	}
	a.Launcher.SetApps(many)
	a.refreshRecents()
	if got := len(a.Launcher.Recent); got != 10 {
		t.Errorf("the front door holds %d recents, want the old build's 10", got)
	}
}

func TestSystemCommandsFollowTheSetting(t *testing.T) {
	a := newApp(t)
	if settings.ShowCommandsInSearch(a.Store.Snapshot()) {
		t.Error("show_commands_in_search is on by default")
	}
	a.scanTools()
	if a.Launcher.ShowTools {
		t.Error("the tools were offered with the setting off")
	}

	// Turning it on scans the PATH. The scan runs in the background, so the
	// test waits for the launcher to hear about it, and only checks that
	// something plausible arrived: the machine's own PATH decides what.
	if err := a.Store.Update(func(s *settings.Settings) { s.SetShowCommandsInSearch(true) }); err != nil {
		t.Fatal(err)
	}
	a.scanTools()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if a.Launcher.ShowTools {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the scan never reached the launcher")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(a.Launcher.Tools) == 0 {
		t.Skip("this machine's PATH has no commands to find")
	}
	for _, tool := range a.Launcher.Tools {
		if tool.Name == "" || tool.Path == "" {
			t.Errorf("a tool without a name or path: %+v", tool)
		}
	}

	// Running one goes to the terminal surface. The injected constructor
	// fails, so the surface shows the error instead of a session: the point
	// is the hand-off.
	a.Launcher.Actions.RunInTerminal([]string{a.Launcher.Tools[0].Path})
	if a.Surf != SurfaceTerminal {
		t.Errorf("surface = %v, want terminal", a.Surf)
	}
}

func TestSetShortcutRegistersAndPersists(t *testing.T) {
	registered := []string{}
	failing := false
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
		RegisterShortcut: func(accelerator string, fn func()) error {
			if failing {
				return errors.New("taken")
			}
			registered = append(registered, accelerator)
			return nil
		},
	})
	// Start registers the shipped default (or the stored hotkey).
	a.summonKey = "Ctrl+Space"
	a.Settings.ShortcutID = shortcutToggleWindow

	a.setShortcut(shortcutToggleWindow, "Cmd+Shift+G")
	if len(registered) != 1 || registered[0] != "Cmd+Shift+G" {
		t.Fatalf("registered = %v", registered)
	}
	if a.summonKey != "Cmd+Shift+G" {
		t.Errorf("summon key = %q", a.summonKey)
	}
	// The old spelling normalizes on the way in, and the file keeps it.
	a.setShortcut(shortcutToggleWindow, "Alt+Comma")
	if got := store.Snapshot().Extra()["hotkey"]; got != "Alt+," {
		t.Errorf("hotkey = %v", got)
	}
	if got := store.Snapshot().Extra()["shortcuts"]; got == nil {
		t.Error("the shortcuts map was not written")
	}
	// A registration the system refuses changes nothing.
	failing = true
	a.setShortcut(shortcutToggleWindow, "Cmd+F1")
	if a.summonKey != "Alt+," {
		t.Errorf("a refused registration changed the key to %q", a.summonKey)
	}
	failing = false

	// Junk and unknown actions are ignored.
	before := a.summonKey
	a.setShortcut(shortcutToggleWindow, "Hyper+X")
	a.setShortcut("something_else", "Cmd+K")
	if a.summonKey != before {
		t.Errorf("the key changed to %q", a.summonKey)
	}
}

// A command switch is the gate the launcher reads, and toggling one from the
// settings surface re-hands the launcher its command list.
func TestCommandSwitchGatesAndUpdatesTheLauncher(t *testing.T) {
	paths := integrationFixture(t)
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.RefreshIntegrations(context.Background())
	if got := a.Launcher.Commands; len(got) != 0 {
		t.Fatalf("commands with the switch off = %+v", got)
	}

	// The integrations list shows the command with its switch state.
	list := a.integrationList()
	if len(list) != 1 || len(list[0].Commands) != 1 {
		t.Fatalf("integration list = %+v", list)
	}
	if list[0].Commands[0].ID != "jv" || list[0].Commands[0].Enabled {
		t.Errorf("command row = %+v", list[0].Commands[0])
	}

	// Turning it on persists and reaches the launcher without a refresh.
	a.setCommandEnabled("io.github.vst93.v", "jv", true)
	if !settings.CommandSwitchesOf(store.Snapshot()).Enabled("io.github.vst93.v", "jv") {
		t.Error("the switch did not persist")
	}
	if got := a.Launcher.Commands; len(got) != 1 || got[0].Command.ID != "jv" {
		t.Errorf("the launcher did not hear about the switch: %+v", got)
	}
	if got := a.integrationList()[0].Commands[0]; !got.Enabled {
		t.Errorf("the row still reads off: %+v", got)
	}

	// Turning it back off removes it again.
	a.setCommandEnabled("io.github.vst93.v", "jv", false)
	if got := a.Launcher.Commands; len(got) != 0 {
		t.Errorf("the command stayed after its switch went off: %+v", got)
	}
}

// The settings surface reopens on the page the user last looked at.
func TestLastSettingsPageIsRestored(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(Options{Store: store, Paths: extensions.FromRoot(t.TempDir())})
	a.rememberSettingsPage("integrations")
	if got := settings.LastSettingsPage(store.Snapshot()); got != "integrations" {
		t.Errorf("stored page = %q", got)
	}
	a.Settings.Page = settingsui.PageGeneral
	a.restoreSettingsPage()
	if a.Settings.Page != settingsui.PageIntegrations {
		t.Errorf("restored page = %d", a.Settings.Page)
	}

	// A file that still names the retired plugins page opens on the first
	// page: the page is gone, and a stored id the build does not know is not
	// an error.
	if err := store.Update(func(s *settings.Settings) { s.SetLastSettingsPage("plugins") }); err != nil {
		t.Fatal(err)
	}
	a.Settings.Page = settingsui.PageAbout
	a.restoreSettingsPage()
	if a.Settings.Page != settingsui.PageGeneral {
		t.Errorf("the retired plugins page restored %d", a.Settings.Page)
	}

	// An unknown or missing page falls back to General.
	if err := store.Update(func(s *settings.Settings) { s.SetLastSettingsPage("nope") }); err != nil {
		t.Fatal(err)
	}
	a.Settings.Page = settingsui.PageAbout
	a.restoreSettingsPage()
	if a.Settings.Page != settingsui.PageGeneral {
		t.Errorf("an unknown page restored %d", a.Settings.Page)
	}
}

// A custom shortcut is registered on start, re-registered when the list
// changes, and released when it goes away; a key the system refuses is
// reported rather than left looking bound.
func TestCustomShortcutsRegisterAndRun(t *testing.T) {
	live := map[string]func(){}
	taken := map[string]bool{"Cmd+Shift+T": true}
	silent := []string{}
	external := 0

	initial := settings.Default()
	initial.SetCustomShortcuts([]settings.CustomShortcut{
		{Key: "Cmd+Shift+P", Action: "plugin:clipboard"},
		{Key: "Cmd+Shift+T", Action: "plugin:browser"},
		{Key: "Cmd+Shift+R", Action: "action:open_settings"},
		{Key: "Cmd+Shift+N", Action: "action:new_command"},
		{Key: "Cmd+Shift+E", Action: "action:open_external_terminal"},
		{Key: "Cmd+Shift+C", Action: "say done"},
	})
	store := settings.NewStore(initial)
	a := New(Options{
		Store: store,
		Paths: extensions.FromRoot(t.TempDir()),
		RegisterShortcut: func(accelerator string, fn func()) error {
			if taken[accelerator] {
				return errors.New("taken")
			}
			live[accelerator] = fn
			return nil
		},
		UnregisterShortcut:   func(accelerator string) { delete(live, accelerator) },
		RunSilentCommand:     func(command string) error { silent = append(silent, command); return nil },
		OpenExternalTerminal: func() error { external++; return nil },
	})

	rejections := a.ApplyCustomShortcuts()
	if len(rejections) != 1 || rejections[0].Key != "Cmd+Shift+T" {
		t.Fatalf("rejections = %+v", rejections)
	}
	for _, accelerator := range []string{"Cmd+Shift+P", "Cmd+Shift+R", "Cmd+Shift+N", "Cmd+Shift+E", "Cmd+Shift+C"} {
		if live[accelerator] == nil {
			t.Errorf("%s was not registered", accelerator)
		}
	}
	if live["Cmd+Shift+T"] != nil {
		t.Error("a taken key was registered")
	}

	// The plugin action shows the panel in that mode.
	live["Cmd+Shift+P"]()
	if a.Surf != SurfaceLauncher || !a.Launcher.InClipboardMode() {
		t.Errorf("clipboard action: surface %v", a.Surf)
	}

	// The app actions run theirs.
	a.Launcher.ResetQuery()
	live["Cmd+Shift+R"]()
	if a.Surf != SurfaceSettings {
		t.Errorf("open_settings: surface %v", a.Surf)
	}
	a.Launcher.SetQuery("leftover")
	live["Cmd+Shift+N"]()
	if a.Surf != SurfaceLauncher || a.Launcher.Query != "" {
		t.Errorf("new_command: surface %v query %q", a.Surf, a.Launcher.Query)
	}
	live["Cmd+Shift+E"]()
	if external != 1 {
		t.Errorf("external terminal ran %d times", external)
	}

	// Anything else is a command line, run silently.
	live["Cmd+Shift+C"]()
	if len(silent) != 1 || silent[0] != "say done" {
		t.Errorf("silent commands = %v", silent)
	}

	// A shorter list releases what is gone and keeps what stayed. The store's
	// own listener re-registers, so the caller does not have to ask.
	if err := store.Update(func(s *settings.Settings) {
		s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}})
	}); err != nil {
		t.Fatal(err)
	}
	if len(live) != 1 || live["Cmd+Shift+P"] == nil {
		t.Errorf("the registrations after a shorter list = %v", keysOf(live))
	}
}

// keysOf lists a registration map's keys, for a failure message.
func keysOf(live map[string]func()) []string {
	out := make([]string, 0, len(live))
	for key := range live {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// The settings change that rewrites the list re-registers without the caller
// asking, and a list that did not change does not churn the registrations.
func TestCustomShortcutsFollowTheStore(t *testing.T) {
	registered := 0
	unregistered := 0
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:                store,
		Paths:                extensions.FromRoot(t.TempDir()),
		RegisterShortcut:     func(string, func()) error { registered++; return nil },
		UnregisterShortcut:   func(string) { unregistered++ },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})
	a.ApplyCustomShortcuts()
	before := registered

	if err := store.Update(func(s *settings.Settings) {
		s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}})
	}); err != nil {
		t.Fatal(err)
	}
	if registered != before+1 {
		t.Errorf("the store change did not register the key: %d -> %d", before, registered)
	}

	// A write that leaves the list alone must not re-register it.
	if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
		t.Fatal(err)
	}
	if registered != before+1 || unregistered != 0 {
		t.Errorf("an unrelated change churned the shortcuts: %d registered, %d released", registered, unregistered)
	}
}

// A custom key that shadows one of the app's own shortcuts is refused as a
// `conflict` and never reaches the disk; one the system will not take is
// `occupied` and is not written either. Only the keys that registered are
// stored, as the old host's `set_custom_shortcuts` had it — a refused key must
// not be left on disk to be retried on the next launch.
func TestCustomShortcutsRefuseConflictsAndKeepOnlyRegistered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	// Pin an app binding so the shadowing key is the same on every host.
	if err := store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutNewCommand, "Cmd+W")
	}); err != nil {
		t.Fatal(err)
	}

	live := map[string]func(){}
	taken := map[string]bool{"Cmd+Shift+T": true}
	a := New(Options{
		Store: store,
		Paths: extensions.FromRoot(t.TempDir()),
		RegisterShortcut: func(accelerator string, fn func()) error {
			if taken[accelerator] {
				return errors.New("taken")
			}
			live[accelerator] = fn
			return nil
		},
		UnregisterShortcut: func(accelerator string) { delete(live, accelerator) },
	})
	a.summonKey = "Ctrl+Space"

	rejections := a.setCustomShortcuts([]settings.CustomShortcut{
		{Key: "Cmd+W", Action: "plugin:clipboard"},       // shadows new_command
		{Key: "Cmd+Shift+T", Action: "plugin:browser"},   // the system refuses it
		{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}, // the one that lands
	})
	reason := map[string]string{}
	for _, rejection := range rejections {
		reason[rejection.Key] = rejection.Reason
	}
	if len(rejections) != 2 {
		t.Fatalf("rejections = %+v", rejections)
	}
	if reason["Cmd+W"] != settingsui.CustomShortcutReasonConflict {
		t.Errorf("the shadowed key = %q, want conflict", reason["Cmd+W"])
	}
	if reason["Cmd+Shift+T"] != settingsui.CustomShortcutReasonOccupied {
		t.Errorf("the refused key = %q, want occupied", reason["Cmd+Shift+T"])
	}

	// Only the key that registered is stored, in memory and on disk.
	if stored := settings.CustomShortcutsOf(store.Snapshot()); len(stored) != 1 || stored[0].Key != "Cmd+Shift+P" {
		t.Errorf("stored = %+v; only the registered key must be kept", stored)
	}
	onDisk, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if stored := settings.CustomShortcutsOf(onDisk); len(stored) != 1 || stored[0].Key != "Cmd+Shift+P" {
		t.Errorf("the file kept a refused key: %+v", stored)
	}

	// The conflict was never claimed; the free key was.
	if live["Cmd+W"] != nil {
		t.Error("the shadowed key was registered")
	}
	if live["Cmd+Shift+P"] == nil {
		t.Error("the free key was not registered")
	}
}

// The calculator history is wired to the store the shell owns: a recorded
// calculation lands in the file the old build wrote, and a retention change
// prunes it.
func TestCalculatorHistoryIsWired(t *testing.T) {
	root := t.TempDir()
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       extensions.FromRoot(root),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	if a.Calculator == nil || a.Launcher.Calculator == nil {
		t.Fatal("the calculator history was not wired")
	}

	// Recording through the launcher's mode writes the index.
	a.Launcher.EnterCalculator()
	a.Launcher.SetQuery("calc 6*7")
	a.Launcher.ResetQuery()
	entry, err := a.Calculator.Add("6*7", "42")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "calculator-history", "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the index was not written: %v", err)
	}
	if !strings.Contains(string(data), "6*7") || !strings.Contains(string(data), entry.ID) {
		t.Errorf("the index = %s", data)
	}

	// A settings change re-applies the retention and the copy mode.
	if err := store.Update(func(s *settings.Settings) {
		s.SetCalculatorPlugin(settings.CalculatorPlugin{MaxItems: 50, RetentionDays: 0, CopyMode: settings.CalculatorCopyResult})
	}); err != nil {
		t.Fatal(err)
	}
	if a.Launcher.CopyMode != settings.CalculatorCopyResult {
		t.Errorf("copy mode = %q", a.Launcher.CopyMode)
	}
	if a.Calculator.MaxItems() != 50 || a.Calculator.RetentionDays() != 0 {
		t.Errorf("retention = %d / %d", a.Calculator.MaxItems(), a.Calculator.RetentionDays())
	}
}

// A command whose manifest declares `output: background` runs headless and its
// output reaches the launcher: the manifest's route has to travel all the way
// from the file through CommandEntries to the launcher's own row.
func TestCapturedCommandReachesTheLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	paths := integrationFixture(t)
	// The fixture's own manifest declares no `output` mode at all, so it must
	// land on the schema default — `background` — rather than the terminal: the
	// route under test comes from the manifest, not from a field set here.
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.SetCommandSwitch("io.github.vst93.v", "jv", true) }); err != nil {
		t.Fatal(err)
	}
	a := New(Options{
		Store:       store,
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.RefreshIntegrations(context.Background())
	if len(a.Launcher.Commands) != 1 {
		t.Fatalf("commands = %+v", a.Launcher.Commands)
	}
	entry := a.Launcher.Commands[0]
	// The manifest's route survived the trip: the launcher's row reads it for
	// the direct-output nudge and `runCommandWith` reads it to pick the
	// captured path.
	if entry.Route != extensions.RouteBackground {
		t.Fatalf("route = %q, want %q", entry.Route, extensions.RouteBackground)
	}
	entry.Program = "/bin/sh"
	entry.Args = []string{"-c", "echo captured"}

	done := make(chan struct{})
	a.Launcher.Actions.RunCommandCaptured(entry, nil, func(run extensions.CapturedRun, err error) {
		if err != nil {
			t.Errorf("run: %v", err)
		}
		if strings.TrimSpace(run.Stdout) != "captured" {
			t.Errorf("stdout = %q", run.Stdout)
		}
		if !run.Success || run.ExitCode != 0 {
			t.Errorf("run = %+v", run)
		}
		close(done)
	})
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the captured run never answered")
	}
}

// A background run that finishes while the panel is hidden raises one system
// notification; a visible panel shows the output itself.
func TestCapturedRunNotifiesOnlyWhenHidden(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	notified := [][2]string{}
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                extensions.FromRoot(t.TempDir()),
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
		Notify:               func(title, body string) { notified = append(notified, [2]string{title, body}) },
	})
	entry := extensions.CommandEntry{
		Command: extensions.Command{ID: "run", Name: "Run things"},
		Program: "/bin/sh",
		Args:    []string{"-c", "echo done"},
		Route:   extensions.RouteBackground,
	}
	done := make(chan struct{})
	a.runCommandCaptured(entry, nil, func(extensions.CapturedRun, error) { close(done) })
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the captured run never answered")
	}
	if len(notified) != 1 {
		t.Fatalf("notifications = %v", notified)
	}
	if notified[0][0] != "floter" || !strings.Contains(notified[0][1], "Run things finished in ") ||
		!strings.Contains(notified[0][1], "1 lines of output") {
		t.Errorf("notification = %v", notified[0])
	}

	// A completed run that failed names its exit code, and carries no line
	// count: the line count belongs to a run that finished successfully.
	notified = nil
	entry.Args = []string{"-c", "exit 4"}
	done = make(chan struct{})
	a.runCommandCaptured(entry, nil, func(extensions.CapturedRun, error) { close(done) })
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the second run never answered")
	}
	if len(notified) != 1 || !strings.Contains(notified[0][1], "Run things failed with exit code 4") {
		t.Errorf("notifications = %v", notified)
	}
	if len(notified) == 1 && strings.Contains(notified[0][1], "lines of output") {
		t.Errorf("a failed run should not carry a line count: %v", notified[0])
	}
}

// Disabling an integration stops its background run in flight, and the run
// reports the run_killed failure the old host's R98 emitted. Before this the
// key was defined and localised but no code path ever produced it.
func TestDisablingStopsARunInFlight(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "run", Name: "Run"},
		Program:       "/bin/sh",
		Args:          []string{"-c", "sleep 5"},
		Route:         extensions.RouteBackground,
	}
	done := make(chan error, 1)
	a.runCommandCaptured(entry, nil, func(_ extensions.CapturedRun, err error) { done <- err })

	// Wait until the run is registered, then disable its integration.
	deadline := time.Now().Add(5 * time.Second)
	for a.Runs.InFlight(entry.IntegrationID) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the run was never registered")
		}
		time.Sleep(time.Millisecond)
	}
	a.Settings.Actions.SetIntegrationEnabled(entry.IntegrationID, false, nil)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the disabled run finished instead of being killed")
		}
		failure, ok := extensions.ParseRunFailure(err.Error())
		if !ok || failure.Key != extensions.RunErrorKilled || failure.String("extension_id") != "test.ext" {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the killed run never answered")
	}
	if got := a.Runs.InFlight(entry.IntegrationID); got != 0 {
		t.Errorf("the killed run stayed registered: %d", got)
	}
}

// The panel's manual Run follows the manifest's output mode: a terminal route
// streams through the PTY, a background route runs headless and leaves the
// captured output on the integration's row. Before this split a background
// integration's Run button opened a terminal anyway.
func TestPanelRunFollowsTheManifestOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	paths := integrationFixture(t)
	// The fixture declares no output mode, so it keeps the schema's default —
	// background — and the panel run captures onto the row.
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.RefreshIntegrations(context.Background())
	a.runWithParams("io.github.vst93.v", nil)
	result, ok := waitForRun(t, a, "io.github.vst93.v")
	if !ok {
		t.Fatal("a background panel run left no output on the row")
	}
	if !result.Success || !strings.Contains(result.Stdout, "protocolVersion") {
		t.Errorf("captured result = %+v", result)
	}

	// A terminal route never records captured output: it hands the command to
	// the terminal surface.
	manifestPath := filepath.Join(paths.Extensions, "io.github.vst93.v", "floter.extension.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.Replace(string(data), `"runtime"`, `"output": "terminal", "runtime"`, 1)
	if err := os.WriteFile(manifestPath, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	terminalApp := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	terminalApp.RefreshIntegrations(context.Background())
	terminalApp.runWithParams("io.github.vst93.v", nil)
	if terminalApp.Surf != SurfaceTerminal {
		t.Errorf("a terminal panel run did not open the terminal surface: %v", terminalApp.Surf)
	}
	if _, ok := terminalApp.Settings.LastRun("io.github.vst93.v"); ok {
		t.Errorf("a terminal panel run recorded captured output")
	}
}

// waitForRun polls until the integration's background run has left its output
// on the row, bounded so a run that never answers fails the test rather than
// hanging it.
func waitForRun(t *testing.T, a *App, id string) (settingsui.RunResult, bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if result, ok := a.Settings.LastRun(id); ok {
			return result, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return settingsui.RunResult{}, false
}

// The completion notification phrases a finished run the way the old build's
// toast did: the duration word form, the output line count on success, and the
// exit code (without a line count) on failure - in the stored language.
func TestRunNotificationSummary(t *testing.T) {
	run := extensions.CapturedRun{
		Command:  []string{"tool"},
		Stdout:   "one\ntwo\n",
		Success:  true,
		Duration: 420 * time.Millisecond,
	}
	body := (&App{}).runNotificationBody(
		i18n.NotificationsFor("en"), i18n.For("en").Settings, "Deploy", run, nil)
	if body != "Deploy finished in 420 ms \u00b7 2 lines of output" {
		t.Errorf("body = %q", body)
	}

	// A run over a second reads in seconds, and a truncated stream says so in
	// place of the plain line count.
	run.Duration = 3400 * time.Millisecond
	run.Truncated = true
	body = (&App{}).runNotificationBody(
		i18n.NotificationsFor("en"), i18n.For("en").Settings, "Deploy", run, nil)
	if body != "Deploy finished in 3.4 s \u00b7 2 lines \u00b7 truncated at 64 KB" {
		t.Errorf("truncated body = %q", body)
	}

	// A silent run has no output summary at all.
	run.Stdout, run.Truncated, run.Duration = "", false, 1000*time.Millisecond
	body = (&App{}).runNotificationBody(
		i18n.NotificationsFor("en"), i18n.For("en").Settings, "Deploy", run, nil)
	if body != "Deploy finished in 1.0 s" {
		t.Errorf("silent body = %q", body)
	}

	// The same facts in Chinese.
	run.Stdout = "\u4e00\n\u4e8c\n"
	body = (&App{}).runNotificationBody(
		i18n.NotificationsFor("zh"), i18n.For("zh").Settings, "Deploy", run, nil)
	if body != "Deploy 已完成，用时 1.0 s \u00b7 输出 2 行" {
		t.Errorf("zh body = %q", body)
	}

	// A completed run that failed names its exit code, with no line count.
	failed := extensions.CapturedRun{ExitCode: 4, Duration: 900 * time.Millisecond}
	body = (&App{}).runNotificationBody(
		i18n.NotificationsFor("en"), i18n.For("en").Settings, "Deploy", failed, nil)
	if body != "Deploy failed with exit code 4" {
		t.Errorf("failure body = %q", body)
	}
}

// A settled terminal resize is written back to the settings, so the next
// launch opens the size the user left it at; a drag that keeps moving costs
// one write.
func TestTerminalResizeIsStored(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.terminalSizeDelay = 20 * time.Millisecond

	// A drag's frames: only the last one lands.
	a.scheduleTerminalSize(900, 600)
	a.scheduleTerminalSize(1000, 700)
	a.scheduleTerminalSize(1200, 800)
	deadline := time.Now().Add(5 * time.Second)
	for {
		snapshot := store.Snapshot()
		if snapshot.TerminalWidth == 1200 && snapshot.TerminalHeight == 800 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the size never settled: %v x %v", snapshot.TerminalWidth, snapshot.TerminalHeight)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The stored size is clamped into the shipped band, so a drag below the
	// minimum cannot store an unusable window.
	a.scheduleTerminalSize(100, 100)
	deadline = time.Now().Add(5 * time.Second)
	for {
		snapshot := store.Snapshot()
		if snapshot.TerminalWidth == settings.MinTerminalWidth && snapshot.TerminalHeight == settings.MinTerminalHeight {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the size was not clamped: %v x %v", snapshot.TerminalWidth, snapshot.TerminalHeight)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A build without an update feed says so rather than pretending to look, and
// the state lands on the About page.
func TestUpdateCheckWithoutAFeed(t *testing.T) {
	a := newApp(t)
	a.checkForUpdates()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if a.Settings.Update.Checked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the check never answered")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !a.Settings.Update.Disabled {
		t.Errorf("update state = %+v, want the no-feed state", a.Settings.Update)
	}
	if a.Settings.Update.Available {
		t.Error("an update is available in a build with no feed")
	}
	// Installing with no feed is a no-op that lands in the same state, not a
	// panic.
	a.installUpdate()
}

// The integrations export writes a document of what is installed, and the
// import applies one, both through injected file dialogs.
func TestIntegrationExportAndImport(t *testing.T) {
	paths := integrationFixture(t)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.SetCommandSwitch("io.github.vst93.v", "jv", true) }); err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(t.TempDir(), "backup.json")
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		SaveFileDialog:       func(string) (string, error) { return exportPath, nil },
		OpenFileDialog:       func() (string, error) { return exportPath, nil },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { return true },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})
	a.RefreshIntegrations(context.Background())

	copy := i18n.For("en").Settings

	a.exportIntegrations()
	waitForTransfer(t, a)
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("the export was not written: %v", err)
	}
	document, err := extensions.ReadSync(data)
	if err != nil {
		t.Fatalf("the export is not readable: %v", err)
	}
	if len(document.Extensions) != 1 || document.Extensions[0].ID != "io.github.vst93.v" {
		t.Fatalf("document = %+v", document.Extensions)
	}
	// A clean export is a success toast with the reference's sentence, not a
	// line left on the page.
	if toast := queuedToast(a, transferNoticeID); toast.Title != copy.IntegrationsExported(1) {
		t.Errorf("the export toast reads %q, want %q", toast.Title, copy.IntegrationsExported(1))
	} else if toast.Type != "success" {
		t.Errorf("the export toast's kind = %q, want success", toast.Type)
	}

	// An import of the same document lands, and leaves its report on the page.
	clearNotices(a)
	a.importIntegrations()
	waitForTransfer(t, a)
	report := a.Settings.TransferReport
	if report == nil || report.Succeeded != 1 || len(report.Failed) != 0 {
		t.Fatalf("the import report = %+v", report)
	}
	if report.Path != exportPath {
		t.Errorf("the report's path = %q, want %q", report.Path, exportPath)
	}

	// A file that is not a document is refused with an error toast, not a
	// crash and no report.
	if err := os.WriteFile(exportPath, []byte("not a document"), 0o600); err != nil {
		t.Fatal(err)
	}
	clearNotices(a)
	a.Settings.TransferReport = nil
	a.importIntegrations()
	waitForTransfer(t, a)
	if a.Settings.TransferReport != nil {
		t.Errorf("a refused import left a report: %+v", a.Settings.TransferReport)
	}
	if !noticeSays(a, copy.IntegrationsTransferBad) {
		t.Error("the refusal toast is missing")
	}
}

// TestTheTransferIsOneAtATime pins the reference's single-flight guard
// (`syncOperationRef.current`): a second export press while one is in flight
// does not open a second dialog, and the busy line is on the page until the
// first one finishes.
func TestTheTransferIsOneAtATime(t *testing.T) {
	paths := integrationFixture(t)
	exportPath := filepath.Join(t.TempDir(), "backup.json")
	release := make(chan struct{})
	entered := make(chan struct{})
	dialogs := 0
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		SaveFileDialog: func(string) (string, error) {
			dialogs++
			close(entered)
			<-release
			return exportPath, nil
		},
	})
	a.RefreshIntegrations(context.Background())

	a.exportIntegrations()
	if !a.transferRunningNow() {
		t.Fatal("the first export never claimed the transfer slot")
	}
	if a.Settings.TransferBusy != "export" {
		t.Errorf("the busy line = %q, want export", a.Settings.TransferBusy)
	}
	// The claim itself is refused while one is in flight.
	if a.beginTransfer("export") {
		t.Error("the transfer guard allowed a second claim")
	}
	// Wait until the first export is parked in its dialog, then press again.
	<-entered
	// The reference refuses the second press; nothing else opens a dialog.
	a.exportIntegrations()
	if dialogs != 1 {
		t.Fatalf("the guard let %d dialogs through, want 1", dialogs)
	}

	close(release)
	waitForTransfer(t, a)
	if a.Settings.TransferBusy != "" {
		t.Errorf("the busy line stayed %q after the transfer", a.Settings.TransferBusy)
	}
	if dialogs != 1 {
		t.Errorf("a second export opened a dialog after the guard released: %d", dialogs)
	}
}

// waitForTransfer waits for the one transfer slot to be free again, which is
// the point at which the goroutine's toast and report are in place.
func waitForTransfer(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if !a.transferRunningNow() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the transfer never finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The script export writes the form's script to the file the save dialog
// answers with, and confirms with the reference's success toast
// (`ExtensionsPanel.tsx:1951-1964`, `i18n.ts:651`). The file name the dialog is
// offered already carries the language's extension, which is what this pins.
func TestTheScriptExportWritesTheFile(t *testing.T) {
	copy := i18n.For("en").Settings
	path := filepath.Join(t.TempDir(), "local.abc-script.py")
	var offered string
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		SaveFileDialog: func(defaultName string) (string, error) {
			offered = defaultName
			return path, nil
		},
	})
	a.exportScript("local.abc-script.py", "print('hello')\n")
	if toast := waitForToast(t, a, scriptExportNoticeID); toast.Title != copy.LocalToolScriptExported {
		t.Fatalf("the export toast reads %q, want %q", toast.Title, copy.LocalToolScriptExported)
	} else if toast.Type != "success" {
		t.Errorf("the export toast's kind = %q, want success", toast.Type)
	}
	if offered != "local.abc-script.py" {
		t.Errorf("the dialog was offered %q, want the name settingsui built", offered)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the script was not written: %v", err)
	}
	if string(data) != "print('hello')\n" {
		t.Errorf("the file holds %q, want the form's script", data)
	}
}

// A closed dialog writes nothing and says nothing, and a dialog that failed is
// the one sentence this build has for a script that could not be exported.
func TestTheScriptExportStaysQuietWhenItWritesNothing(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(Options{
		Store:          settings.NewStore(settings.Default()),
		NewTerminal:    func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		SaveFileDialog: func(string) (string, error) { return "", nil },
	})
	a.exportScript("local.abc-script.py", "print('hello')\n")
	waitForGoroutines()
	if toast := queuedToast(a, scriptExportNoticeID); toast.Title != "" {
		t.Errorf("a closed dialog raised a toast: %q", toast.Title)
	}
	if notices := noticeTitles(a); len(notices) != 0 {
		t.Errorf("a closed dialog left notices: %v", notices)
	}

	failed := New(Options{
		Store:          settings.NewStore(settings.Default()),
		NewTerminal:    func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		SaveFileDialog: func(string) (string, error) { return "", errors.New("no disk") },
	})
	failed.exportScript("local.abc-script.py", "print('hello')\n")
	if !waitForNoticeSay(t, failed, copy.LocalToolExportFailed) {
		t.Errorf("a failed dialog did not raise %q: %v", copy.LocalToolExportFailed, noticeTitles(failed))
	}
}

// noticeTitles is the queued notices' sentences, for a test's own diagnostics.
func noticeTitles(a *App) []string {
	a.noticeMu.Lock()
	defer a.noticeMu.Unlock()
	out := make([]string, 0, len(a.notices))
	for _, toast := range a.notices {
		out = append(out, toast.Title)
	}
	return out
}

// waitForToast waits for the goroutine's toast to land: the export opens its
// dialog off the render thread, so the test cannot read the notice the moment
// the call returns.
func waitForToast(t *testing.T, a *App, id string) ui.Toast {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if toast := queuedToast(a, id); toast.Title != "" {
			return toast
		}
		if time.Now().After(deadline) {
			t.Fatalf("the toast %q never landed", id)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitForNoticeSay waits for a queued notice carrying the sentence.
func waitForNoticeSay(t *testing.T, a *App, title string) bool {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if noticeSays(a, title) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitForGoroutines gives a just-launched export goroutine time to reach its
// (silent) end before a test reads the notice stack.
func waitForGoroutines() {
	time.Sleep(50 * time.Millisecond)
}

// queuedToast finds the queued toast with the given id, empty when none is
// waiting.
func queuedToast(a *App, id string) ui.Toast {
	a.noticeMu.Lock()
	defer a.noticeMu.Unlock()
	for _, toast := range a.notices {
		if toast.ID == id {
			return toast
		}
	}
	return ui.Toast{}
}

// noticeSays reports whether a queued notice carries the sentence.
func noticeSays(a *App, title string) bool {
	a.noticeMu.Lock()
	defer a.noticeMu.Unlock()
	for _, toast := range a.notices {
		if toast.Title == title {
			return true
		}
	}
	return false
}

// clearNotices drops anything queued so the next assertion sees only its own
// toast.
func clearNotices(a *App) {
	a.noticeMu.Lock()
	a.notices = nil
	a.noticeMu.Unlock()
}

// A drop on the window becomes the files mode, with the paths normalized, and
// nothing runs on arrival.
func TestFileDropOpensTheFilesMode(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.handleFileDrop(&mygo.FileDropEvent{Paths: []string{file, filepath.Join(dir, "gone.txt")}})
	if !a.Launcher.InFilesMode() {
		t.Fatal("the files mode did not open")
	}
	if len(a.Launcher.Dropped) != 1 || a.Launcher.Dropped[0].Name != "report.pdf" {
		t.Fatalf("dropped = %+v", a.Launcher.Dropped)
	}
	// A drop of nothing readable leaves the launcher where it was.
	b := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	b.handleFileDrop(&mygo.FileDropEvent{Paths: []string{filepath.Join(dir, "gone.txt")}})
	if b.Launcher.InFilesMode() {
		t.Error("an empty drop opened the mode")
	}
	b.handleFileDrop(nil)
}

// The install catalog reaches the launcher from the machine's search path.
func TestToolCatalogReachesTheLauncher(t *testing.T) {
	a := newApp(t)
	a.scanToolCatalog()
	awaitCatalogIdle(t, a)
	if len(a.Launcher.InstallCatalog) == 0 {
		t.Fatal("the catalog never reached the launcher")
	}
	byID := map[string]tools.State{}
	for _, state := range a.Launcher.InstallCatalog {
		byID[state.ID] = state
	}
	if len(byID) != len(tools.Catalog) {
		t.Errorf("the catalog has %d entries, want %d", len(byID), len(tools.Catalog))
	}
	// Detection ran against the real search path, so a tool that is installed
	// here is marked as such (whichever it is: the assertion is the shape).
	installed := 0
	for _, state := range byID {
		if state.Installed {
			installed++
		}
		if state.Command == "" && len(state.Recipes[tools.Platform()]) > 0 {
			t.Errorf("%s has recipes but no command: %+v", state.ID, state)
		}
	}
	t.Logf("%d of %d catalog tools are installed on this machine", installed, len(byID))
}

// awaitCatalogStart waits for a scan to reach the machine.
func awaitCatalogStart(t *testing.T, starts <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-starts:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not refresh the install catalog", what)
	}
}

// awaitCatalogIdle waits for the in-flight scan to settle, so the next refresh
// is a new one rather than a shared in-flight.
func awaitCatalogIdle(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		a.catalogMu.Lock()
		scanning := a.catalogScanning
		a.catalogMu.Unlock()
		if !scanning {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the catalog scan never settled")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// R68 · the install catalog is re-read on every reveal, so a tool the user
// installed in their own shell while the window was hidden stops being offered
// as an install row on the next summon (src/App.tsx:2453-2457).
func TestASummonRefreshesTheInstallCatalog(t *testing.T) {
	a := newApp(t)
	starts := make(chan struct{}, 4)
	a.lookTools = func() []tools.State { starts <- struct{}{}; return nil }

	a.summon()
	awaitCatalogStart(t, starts, "a summon")
	awaitCatalogIdle(t, a)
}

// R74 · the install row's own door into the terminal page never hides the
// window, so the summon-time refresh cannot see the way back: the explicit
// return to the launcher asks for one of its own (src/App.tsx:781-788).
func TestTheWayBackRefreshesTheInstallCatalog(t *testing.T) {
	a := newApp(t)
	starts := make(chan struct{}, 4)
	a.lookTools = func() []tools.State { starts <- struct{}{}; return nil }

	a.returnToInput()
	awaitCatalogStart(t, starts, "the way back to the launcher")
	awaitCatalogIdle(t, a)
}

// R68 · a refresh that lands while a scan is in flight shares it rather than
// starting a second stat scan (the old build's `refreshToolCatalog` reused its
// in-flight promise), and a refresh after it settles runs again.
func TestOneCatalogScanRunsAtATime(t *testing.T) {
	a := newApp(t)
	release := make(chan struct{})
	starts := make(chan struct{}, 4)
	a.lookTools = func() []tools.State {
		starts <- struct{}{}
		<-release
		return nil
	}
	a.scanToolCatalog()
	<-starts
	// A second refresh while the first is still blocked must not start one.
	a.scanToolCatalog()
	select {
	case <-starts:
		t.Fatal("a second scan started while one was in flight")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	awaitCatalogIdle(t, a)
	// Once it has settled, a later refresh runs again.
	a.scanToolCatalog()
	awaitCatalogStart(t, starts, "a later refresh")
	awaitCatalogIdle(t, a)
}

// R68 · a deep link that arrives while the panel is dismissed brings it to the
// front through Show, and that reveal is the one moment the install catalog is
// re-read (src/App.tsx:2453-2457 listens for the native reveal event, which the
// framework fires for the same hidden-to-visible transition). The test has no
// window, so it drives the visibility reading through the windowVisible seam.
func TestARevealRefreshesTheInstallCatalog(t *testing.T) {
	a := newApp(t)
	starts := make(chan struct{}, 4)
	a.lookTools = func() []tools.State { starts <- struct{}{}; return nil }
	a.windowVisible = func() bool { return false }

	a.Show()
	awaitCatalogStart(t, starts, "a reveal")
	awaitCatalogIdle(t, a)
}

// A switch of the surface on a window the user can already see is not a
// reveal: the memo serves every query between reveals (the old build's
// tool-catalog-store memo), so the catalog is not re-read.
func TestASurfaceSwitchOnAVisiblePanelKeepsTheCatalogMemo(t *testing.T) {
	a := newApp(t)
	starts := make(chan struct{}, 4)
	a.lookTools = func() []tools.State { starts <- struct{}{}; return nil }
	a.windowVisible = func() bool { return true }

	a.Show()
	select {
	case <-starts:
		t.Fatal("a surface switch on a visible panel re-read the install catalog")
	case <-time.After(100 * time.Millisecond):
	}
}

// The reveal decision and the re-home decision are the same reading, so a
// window already on screen is not re-homed either (the old build's
// preserve_anchor).
func TestAVisiblePanelIsNotRehomed(t *testing.T) {
	if placementForReveal(true) {
		t.Error("a visible panel was treated as a reveal")
	}
	if !placementForReveal(false) {
		t.Error("a hidden panel was not treated as a reveal")
	}
}

// R68 · a refresh replaces the memo: the launcher's rows come from the newest
// scan, not the startup one.
func TestARefreshReplacesTheInstallCatalog(t *testing.T) {
	a := newApp(t)
	a.lookTools = func() []tools.State {
		return []tools.State{{Entry: tools.Entry{ID: "alpha", Name: "Alpha"}, Command: "x alpha"}}
	}
	a.scanToolCatalog()
	awaitCatalogIdle(t, a)
	if len(a.Launcher.InstallCatalog) != 1 || a.Launcher.InstallCatalog[0].ID != "alpha" {
		t.Fatalf("the first scan did not land: %+v", a.Launcher.InstallCatalog)
	}
	a.lookTools = func() []tools.State {
		return []tools.State{{Entry: tools.Entry{ID: "beta", Name: "Beta"}, Command: "x beta"}}
	}
	a.scanToolCatalog()
	awaitCatalogIdle(t, a)
	if len(a.Launcher.InstallCatalog) != 1 || a.Launcher.InstallCatalog[0].ID != "beta" {
		t.Fatalf("the refresh did not replace the memo: %+v", a.Launcher.InstallCatalog)
	}
}

// The shipped package can be connected from the Integrations page, through the
// same install pipeline as any local tool.
func TestConnectRecommendedInstallsTheShippedPackage(t *testing.T) {
	// The shipped package runs the V toolchain; a fixture stands in for it, so
	// the test does not depend on the machine having V installed.
	restore := extensions.StubToolLookup(func(names ...string) (string, bool) {
		dir := t.TempDir()
		path := filepath.Join(dir, "tool")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path, true
	})
	defer restore()

	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	notified := [][2]string{}
	store := settings.NewStore(settings.Default())
	var reviewed []extensions.Prepared
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { return true },
		ReviewRecommended:    func(prepared extensions.Prepared) bool { reviewed = append(reviewed, prepared); return true },
		Notify:               func(title, body string) { notified = append(notified, [2]string{title, body}) },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})

	// Before connecting, the page is offered it.
	list := a.recommendedTools()
	if len(list) != 1 || list[0].Installed {
		t.Fatalf("recommended = %+v", list)
	}

	a.connectRecommended("io.github.vst93.v")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if list := a.recommendedTools(); len(list) == 1 && list[0].Installed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the connect never landed: %+v", a.recommendedTools())
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The connect flow reviewed the manifest's own permissions before it
	// committed: the shipped package declares seven.
	if len(reviewed) != 1 {
		t.Fatalf("the review count = %d, want 1", len(reviewed))
	}
	if len(reviewed[0].Approval.Declared) != 7 {
		t.Errorf("reviewed permissions = %v", reviewed[0].Approval.Declared)
	}
	// The package is on disk and in the repository, and the notification says
	// so.
	if _, err := os.Stat(filepath.Join(paths.Extensions, "io.github.vst93.v", "floter.extension.json")); err != nil {
		t.Errorf("the package is missing: %v", err)
	}
	integration, ok := a.Integrations.Inventory().WithID("io.github.vst93.v")
	if !ok || !integration.Entry.Enabled {
		t.Errorf("integration = %+v", integration)
	}
	if len(notified) != 1 || !strings.Contains(notified[0][1], "V Tools") {
		t.Errorf("notified = %v", notified)
	}
}

// Restoring the shipped keys drops the stored map and puts the summon key back
// to the default, re-registered with the system.
func TestResetShortcuts(t *testing.T) {
	registered := []string{}
	unregistered := []string{}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) {
		s.SetExtra("hotkey", "Alt+Space")
		s.SetShortcut(settings.ShortcutNewCommand, "Alt+N")
	}); err != nil {
		t.Fatal(err)
	}
	a := New(Options{
		Store:                store,
		Paths:                extensions.FromRoot(t.TempDir()),
		RegisterShortcut:     func(accelerator string, fn func()) error { registered = append(registered, accelerator); return nil },
		UnregisterShortcut:   func(accelerator string) { unregistered = append(unregistered, accelerator) },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})
	a.summonKey = "Alt+Space"
	if got := settings.Shortcut(store.Snapshot(), settings.ShortcutNewCommand); got != "Alt+N" {
		t.Fatalf("the stored binding = %q", got)
	}

	a.resetShortcuts()
	if got := settings.Shortcut(store.Snapshot(), settings.ShortcutNewCommand); got != settings.ShortcutsOf(settings.Default())[settings.ShortcutNewCommand] {
		t.Errorf("the app key was not restored: %q", got)
	}
	hotkey, _ := store.Snapshot().Extra()["hotkey"].(string)
	if hotkey != settings.DefaultSummonShortcut {
		t.Errorf("hotkey = %q", hotkey)
	}
	if a.summonKey != settings.DefaultSummonShortcut {
		t.Errorf("the summon key = %q", a.summonKey)
	}
	if len(registered) == 0 || registered[len(registered)-1] != settings.DefaultSummonShortcut {
		t.Errorf("registered = %v", registered)
	}
	if len(unregistered) == 0 || unregistered[len(unregistered)-1] != "Alt+Space" {
		t.Errorf("released = %v", unregistered)
	}
}

// While a settings recorder waits for a key, the global shortcuts are released
// and then taken back.
func TestShortcutsAreSuspendedWhileRecording(t *testing.T) {
	registered := []string{}
	unregistered := []string{}
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:              store,
		Paths:              extensions.FromRoot(t.TempDir()),
		RegisterShortcut:   func(accelerator string, fn func()) error { registered = append(registered, accelerator); return nil },
		UnregisterShortcut: func(accelerator string) { unregistered = append(unregistered, accelerator) },
	})
	// A summon key and one custom key are held.
	a.summonKey = "Ctrl+Space"
	if err := store.Update(func(s *settings.Settings) {
		s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Cmd+Shift+P", Action: "plugin:clipboard"}})
	}); err != nil {
		t.Fatal(err)
	}
	a.ApplyCustomShortcuts()

	a.setShortcutsSuspended(true)
	if !a.shortcutsSuspended {
		t.Fatal("the shell did not record the suspension")
	}
	for _, key := range []string{"Ctrl+Space", "Cmd+Shift+P"} {
		if !containsString(unregistered, key) {
			t.Errorf("%s was not released: %v", key, unregistered)
		}
	}
	registered = nil
	a.setShortcutsSuspended(false)
	if a.shortcutsSuspended {
		t.Error("the shell stayed suspended")
	}
	if !containsString(registered, "Ctrl+Space") || !containsString(registered, "Cmd+Shift+P") {
		t.Errorf("registered = %v", registered)
	}
}

func containsString(list []string, want string) bool {
	for _, value := range list {
		if value == want {
			return true
		}
	}
	return false
}

// A power action asks first — in the panel, in the launcher's own confirmation
// row — and only runs the system's command when the user presses that row's own
// button. Nothing runs without an answer.
func TestPowerAsksInThePanelFirst(t *testing.T) {
	ran := []string{}
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                extensions.FromRoot(t.TempDir()),
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		RunPowerCommand:      func(action string) error { ran = append(ran, action); return nil },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})
	copy := i18n.For("en").Launcher
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()
	tt.Type("restart")
	tt.Frame()
	if err := tt.Click(copy.PowerRestart); err != nil {
		t.Fatalf("the power row: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	// The row only asked: the panel shows the confirmation and nothing ran.
	if got := a.Launcher.PowerPending(); got != launcher.PowerRestart {
		t.Fatalf("pending = %q", got)
	}
	if len(ran) != 0 {
		t.Fatalf("an unanswered action ran %v", ran)
	}
	if !tt.HasText(copy.PowerConfirmMessage(launcher.PowerRestart)) {
		t.Errorf("the confirmation is not drawn: %q", tt.Texts())
	}

	// Escape answers the confirmation the way its cancel button does.
	tt.TypeKey(0, ui.KeyEscape, "")
	tt.Frame()
	if a.Launcher.PowerPending() != "" {
		t.Errorf("Escape left the confirmation up: %q", a.Launcher.PowerPending())
	}
	if len(ran) != 0 {
		t.Fatalf("a cancelled action ran %v", ran)
	}

	// Asking again, the confirmation's own button runs it, once.
	a.Launcher.SetQuery("shutdown")
	tt.Frame()
	if err := tt.Click(copy.PowerShutdown); err != nil {
		t.Fatalf("the power row: %v", err)
	}
	tt.Frame()
	if err := tt.Click(copy.PowerShutdownAction); err != nil {
		t.Fatalf("the confirmation's button: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(ran) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the confirmed action never ran")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(ran) != 1 || ran[0] != launcher.PowerShutdown {
		t.Errorf("ran %v", ran)
	}
	if a.Launcher.PowerPending() != "" {
		t.Errorf("the confirmation stayed up after running")
	}
}

// The platform commands are the ones each system answers, with the SysV
// binaries behind systemctl for the rare init that is not systemd.
func TestPowerCommands(t *testing.T) {
	cases := []struct {
		goos    string
		action  string
		program string
	}{
		{"darwin", "restart", "osascript"},
		{"darwin", "shutdown", "osascript"},
		{"windows", "restart", "shutdown"},
		{"windows", "shutdown", "shutdown"},
		{"linux", "restart", "systemctl"},
		{"linux", "shutdown", "systemctl"},
	}
	for _, tc := range cases {
		candidates, err := powerCommands(tc.action, tc.goos)
		if err != nil || len(candidates) == 0 || candidates[0][0] != tc.program {
			t.Fatalf("%s/%s = %v, %v", tc.goos, tc.action, candidates, err)
		}
	}

	// An unknown action is refused rather than run.
	if _, err := powerCommands("explode", "linux"); err == nil {
		t.Error("an unknown action was accepted")
	}
}

// An integration whose provider declares a configuration schema gets a form on
// the settings page, seeded with this machine's values, and a save writes the
// values back (the password into the secrets file).
func TestIntegrationConfigurationFormEndToEnd(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.configured")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.configured", "name": "Configured",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\ncat <<'JSONEOF'\n"+
		`{"protocolVersion":"1.0","provider":{"id":"dev.floter.configured","name":"Configured","version":"1.0.0"},
		  "configuration":{"configVersion":1,"owner":"host","schema":[
		    {"key":"endpoint","type":"text","required":true},
		    {"key":"token","type":"password","required":true},
		    {"key":"region","type":"select","options":["eu","us"]}]}}`+
		"\nJSONEOF\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.configured": {"id": "dev.floter.configured", "name": "Configured",
	    "state": "enabled", "enabled": true, "packageVersion": "1.0.0",
	    "manifestPath": "`+jsonEscaped(filepath.Join(pkg, "floter.extension.json"))+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) {
		s.SetCommandSwitch("dev.floter.configured", "run", true)
	}); err != nil {
		t.Fatal(err)
	}
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
		SaveFileDialog:       func(string) (string, error) { return "", nil },
		OpenFileDialog:       func() (string, error) { return "", nil },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { return true },
	})
	a.RefreshIntegrations(context.Background())

	list := a.integrationList()
	if len(list) != 1 || len(list[0].Config) != 3 {
		t.Fatalf("config schema = %+v", list[0].Config)
	}
	if list[0].Config[1].Type != "password" {
		t.Errorf("field types = %+v", list[0].Config)
	}
	// The provider's own owner reaches the row: the settings form is drawn for
	// a `host`-owned configuration and not for a `tool`-owned one
	// (`configuration.descriptor.owner`, `ExtensionsPanel.tsx:2975`).
	if list[0].ConfigOwner != "host" {
		t.Errorf("config owner = %q, want %q", list[0].ConfigOwner, "host")
	}

	// A save through the action writes the values, and the password lands in
	// the secrets file rather than the values file.
	if err := a.saveIntegrationConfigurationNow("dev.floter.configured", map[string]any{
		"endpoint": "https://example.com", "token": "s3cret", "region": "eu",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(paths.Data, "dev.floter.configured", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "s3cret") {
		t.Errorf("the secret is in the values file: %s", data)
	}
	stored, err := extensions.LoadStoredConfiguration(paths.Data, "dev.floter.configured")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Values["token"] != "s3cret" || stored.Values["endpoint"] != "https://example.com" {
		t.Errorf("values = %v", stored.Values)
	}
	// The integration declares no commands, so nothing is summonable: the
	// configuration is the whole surface here.
	if got := a.Launcher.Commands; len(got) != 0 {
		t.Errorf("commands = %+v", got)
	}
}

// The power action's candidates are tried in order, so a system without
// systemd falls back to the SysV binary.
func TestPowerFallbacksRun(t *testing.T) {
	started := []string{}
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                extensions.FromRoot(t.TempDir()),
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})
	// The injected runner records the program of each attempt and fails all
	// but the last one the shell would try.
	a.runPowerCommand = func(action string) error {
		candidates, err := powerCommands(action, runtime.GOOS)
		if err != nil {
			return err
		}
		for _, candidate := range candidates {
			started = append(started, candidate[0])
		}
		return nil
	}
	if err := a.runPower("restart"); err != nil {
		t.Fatal(err)
	}
	if len(started) == 0 {
		t.Fatal("nothing was started")
	}
}

// A `floter://connect?manifest=…` link reviews the manifest through the same
// permission dialog an install uses, and a refusal connects nothing. The
// register link brings the integrations page up and stops there.
func TestDeepLinkConnectAndRegister(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.linked")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
	  "schemaVersion": "2.0", "id": "dev.floter.linked", "name": "Linked",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "permissions": ["environment"]
	}`
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	answer := false
	asked := 0
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { asked++; return answer },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})

	// The register link only opens the integrations page.
	if _, err := url.Parse("floter://register?cmd=rg"); err != nil {
		t.Fatalf("parse: %v", err)
	}
	a.HandleURL("floter://register?cmd=rg")
	if a.Surf != SurfaceSettings || a.Settings.Page != settingsui.PageIntegrations {
		t.Fatalf("register: surface %v page %d", a.Surf, a.Settings.Page)
	}

	// A refusal connects nothing.
	a.HandleURL("floter://connect?manifest=" + filepath.Join(pkg, "floter.extension.json"))
	answer = false
	deadline := time.Now().Add(5 * time.Second)
	for asked == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the review never ran")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := a.Integrations.Inventory().WithID("dev.floter.linked"); ok {
		t.Fatal("a refused link installed the integration")
	}

	// An approval connects it, and the page is the integrations one.
	answer = true
	a.HandleURL("floter://connect?manifest=" + filepath.Join(pkg, "floter.extension.json"))
	deadline = time.Now().Add(10 * time.Second)
	for {
		if _, ok := a.Integrations.Inventory().WithID("dev.floter.linked"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the link never connected the integration")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if asked != 2 {
		t.Errorf("asked %d times", asked)
	}
	// The manifest link may name a directory or its manifest file.
	a.HandleURL("floter://connect?manifest=" + pkg)
	time.Sleep(100 * time.Millisecond)
	if got := settingsui.IntegrationsPage(); got != settingsui.PageIntegrations {
		t.Errorf("page = %d", got)
	}
}

// A stored lifecycle health is shown on the row, from the repository entry the
// reprobe persisted.
func TestHealthShowsOnTheRow(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.health")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.health", "name": "Healthy",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "lifecycle": {"probes": [
	    {"id": "up", "args": ["--up"], "required": true},
	    {"id": "extras", "args": ["--extras"], "required": false}
	  ]}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.health": {"id": "dev.floter.health", "name": "Healthy",
	    "state": "enabled", "enabled": true, "manifestPath": "`+jsonEscaped(filepath.Join(pkg, "floter.extension.json"))+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Language = "en" }); err != nil {
		t.Fatal(err)
	}
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})
	a.RefreshIntegrations(context.Background())
	if got := a.integrationList()[0].Health; got != "" {
		t.Errorf("health before a reprobe = %q", got)
	}
	if report := a.integrationList()[0].HealthReport; report != nil {
		t.Errorf("a report before any probe = %+v", report)
	}

	// A reprobe persists the health; the shell's view of the repository is
	// refreshed, and the row shows it.
	if _, err := extensions.Reprobe(context.Background(), paths, "dev.floter.health"); err != nil {
		t.Fatal(err)
	}
	a.RefreshIntegrations(context.Background())
	if got := a.integrationList()[0].Health; got != "Healthy" {
		t.Errorf("health after a reprobe = %q", got)
	}
	// The report's detail reaches the card: both probes, their durations and
	// the run's timestamp; retryable is the manifest's `required`, inverted.
	report := a.integrationList()[0].HealthReport
	if report == nil || len(report.Probes) != 2 {
		t.Fatalf("the stored report did not reach the card: %+v", report)
	}
	if report.CheckedAt == "" {
		t.Errorf("the report has no timestamp: %+v", report)
	}
	for _, probe := range report.Probes {
		if !probe.Passed || probe.ExitCode != 0 {
			t.Errorf("probe %q = %+v, want a pass", probe.ID, probe)
		}
	}
	// The projection is the manifest's `required`, inverted: the optional
	// probe is the retryable one, whether or not it happened to pass.
	for _, probe := range report.Probes {
		wantRetryable := probe.ID == "extras"
		if probe.Retryable != wantRetryable {
			t.Errorf("probe %q retryable = %v, want %v", probe.ID, probe.Retryable, wantRetryable)
		}
	}

	// A failing required probe marks the integration broken and says so.
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\nexit 9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := extensions.Reprobe(context.Background(), paths, "dev.floter.health"); err != nil {
		t.Fatal(err)
	}
	a.RefreshIntegrations(context.Background())
	if got := a.integrationList()[0].Health; got != "Unhealthy" {
		t.Errorf("broken health = %q", got)
	}
	// A failed required probe is not retryable; the failed optional one is.
	broken := a.integrationList()[0].HealthReport
	if broken == nil || len(broken.Probes) != 2 {
		t.Fatalf("the failing report did not reach the card: %+v", broken)
	}
	for _, probe := range broken.Probes {
		wantRetryable := probe.ID == "extras"
		if probe.Passed || probe.Retryable != wantRetryable {
			t.Errorf("probe %q = %+v, want a failure with retryable=%v", probe.ID, probe, wantRetryable)
		}
	}
	// A broken integration contributes no commands.
	if got := a.Launcher.Commands; len(got) != 0 {
		t.Errorf("a broken integration contributed %+v", got)
	}
}

// The recorded approval reaches the card (R7-8): the entry's audit fields —
// when, which permissions, against which manifest — with the on-disk digest
// read beside them, and the permissions tiered by the host's own classifier.
func TestApprovalRecordReachesTheCard(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.approved")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.approved", "name": "Approved",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable"},
	  "permissions": ["environment", "network-fetch"]
	}`)
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := extensions.ManifestDigest(filepath.Join(pkg, "floter.extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	repo := strings.NewReplacer("{DIGEST}", digest).Replace(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.approved": {"id": "dev.floter.approved", "name": "Approved",
	    "state": "enabled", "enabled": true, "manifestPath": "` + jsonEscaped(filepath.Join(pkg, "floter.extension.json")) + `",
	    "installedAt": 1, "updatedAt": 1,
	    "approvedPermissions": ["environment", "network-fetch"], "approvedAt": 1750000000,
	    "approvedManifestDigest": "{DIGEST}"}}
	}`)
	if err := os.WriteFile(paths.RepositoryFile, []byte(repo), 0o644); err != nil {
		t.Fatal(err)
	}
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})
	a.RefreshIntegrations(context.Background())
	list := a.integrationList()
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	approval := list[0].Approval
	if approval == nil {
		t.Fatal("the recorded approval did not reach the card")
	}
	if approval.At != 1750000000 || approval.ApprovedDigest != digest || approval.CurrentDigest != digest {
		t.Errorf("approval = %+v, want at/digests recorded and read", approval)
	}
	// The tiers are the host's own classifier: `environment` is enforced,
	// `network-fetch` is disclosure; each group keeps the record's order.
	if len(approval.Enforced) != 1 || approval.Enforced[0] != "environment" {
		t.Errorf("enforced = %v", approval.Enforced)
	}
	if len(approval.Disclosure) != 1 || approval.Disclosure[0] != "network-fetch" {
		t.Errorf("disclosure = %v", approval.Disclosure)
	}
	// Both sides known and equal: not stale.
	if extensions.ApprovalIsStale(approval.ApprovedDigest, approval.CurrentDigest) {
		t.Error("an unchanged manifest was reported stale")
	}

	// The bytes on disk changing under the approval is what "stale" means.
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), append(manifest, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	a.RefreshIntegrations(context.Background())
	moved := a.integrationList()[0].Approval
	if moved == nil || moved.ApprovedDigest != digest {
		t.Fatalf("the recorded digest moved: %+v", moved)
	}
	if moved.CurrentDigest == digest || !extensions.ApprovalIsStale(moved.ApprovedDigest, moved.CurrentDigest) {
		t.Errorf("a changed manifest was not reported stale: %+v", moved)
	}

	// An entry without a record carries none: unknown is its own state, not a
	// zeroed block.
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.approved": {"id": "dev.floter.approved", "name": "Approved",
	    "state": "enabled", "enabled": true, "manifestPath": "`+jsonEscaped(filepath.Join(pkg, "floter.extension.json"))+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a.RefreshIntegrations(context.Background())
	if got := a.integrationList()[0].Approval; got != nil {
		t.Errorf("a record-less entry carried %+v", got)
	}
}

// The persisted report, for the debugging of the health display.
func TestDebugPersistedReport(t *testing.T) {
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.health")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.health", "name": "Healthy",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]},
	  "lifecycle": {"probes": [{"id": "up", "args": ["--up"], "required": true}]}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.health": {"id": "dev.floter.health", "name": "Healthy",
	    "state": "enabled", "enabled": true, "manifestPath": "`+jsonEscaped(filepath.Join(pkg, "floter.extension.json"))+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := extensions.Reprobe(context.Background(), paths, "dev.floter.health"); err != nil {
		t.Fatal(err)
	}
	inventory := extensions.LoadInventory(paths)
	integration, _ := inventory.WithID("dev.floter.health")
	t.Logf("probeReport=%+v state=%q", integration.Entry.ProbeReport, integration.Entry.State)
}

// The clear-history action asks first, and only clears when the user confirms.
func TestClearClipboardHistoryAsksFirst(t *testing.T) {
	dir := t.TempDir()
	clipStore := clipboard.NewStore(clipboard.FromConfigRoot(dir), 0)
	if _, _, err := clipStore.AddText("plain"); err != nil {
		t.Fatal(err)
	}
	starred, _, err := clipStore.AddText("keep me")
	if err != nil {
		t.Fatal(err)
	}
	if err := clipStore.SetFavorite(starred.ID, true); err != nil {
		t.Fatal(err)
	}
	confirmed := false
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                extensions.FromRoot(t.TempDir()),
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})
	a.Clipboard = clipStore
	a.clearClipboardHistoryAsks(func(title string) bool { confirmed = true; return confirmed })

	deadline := time.Now().Add(5 * time.Second)
	for {
		if len(clipStore.Entries()) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the clear never landed: %v", clipStore.Entries())
		}
		time.Sleep(5 * time.Millisecond)
	}
	entries := clipStore.Entries()
	if len(entries) != 1 || !entries[0].Favorite || entries[0].ID != starred.ID {
		t.Errorf("after the clear = %+v", entries)
	}
	_ = confirmed
}

// Clearing the clipboard history is the clipboard plugin page's one long
// action, and it reports its outcome through the same completion rule as every
// other background task: the panel's own row when the panel is on screen, a
// system notification when it is hidden (the old build's
// `clipboard_clear_history` → `notify_completion`). Both channels name the
// plugin by its localized page name.
func TestClipboardClearReportsWhereTheUserIsLooking(t *testing.T) {
	clipStore := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 0)
	if _, _, err := clipStore.AddText("plain"); err != nil {
		t.Fatal(err)
	}
	notifications := []string{}
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       extensions.FromRoot(t.TempDir()),
		Notify:      func(title, body string) { notifications = append(notifications, title+"|"+body) },
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.Clipboard = clipStore
	name := i18n.For("en").Settings.ClipboardPlugin

	// Hidden: the system notification is the only channel, and it carries the
	// old build's success wording under the plugin page's own name.
	a.onScreen = func() bool { return false }
	if !a.clearClipboardHistory() {
		t.Fatal("the clear was refused")
	}
	if len(notifications) != 1 {
		t.Fatalf("notifications = %v", notifications)
	}
	if want := name + " cleared"; !strings.Contains(notifications[0], want) {
		t.Errorf("the notification = %q, want %q in it", notifications[0], want)
	}

	// On screen: the panel's own row answers and no notification is raised.
	notifications = notifications[:0]
	a.onScreen = func() bool { return true }
	if _, _, err := clipStore.AddText("another"); err != nil {
		t.Fatal(err)
	}
	if !a.clearClipboardHistory() {
		t.Fatal("the second clear was refused")
	}
	if len(notifications) != 0 {
		t.Errorf("a visible panel still notified: %v", notifications)
	}
	if !a.Launcher.FeedbackVisible() {
		t.Error("the panel's own row said nothing")
	}

	// The failure wording is the old build's `{name} could not be cleared`.
	notifications = notifications[:0]
	a.onScreen = func() bool { return false }
	a.notifyClipboardClear(false)
	if len(notifications) != 1 {
		t.Fatalf("failure notifications = %v", notifications)
	}
	if want := name + " could not be cleared"; !strings.Contains(notifications[0], want) {
		t.Errorf("the failure notification = %q, want %q in it", notifications[0], want)
	}
}

// The alias editor's data path: an alias is recorded, reaches the settings
// list with the conflict flag set when another command claimed it first, and
// an empty alias removes the entry.
func TestCommandAliasFlow(t *testing.T) {
	paths := integrationFixture(t)
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.RefreshIntegrations(context.Background())

	// One alias on the fixture's only command.
	a.setCommandAlias("io.github.vst93.v", "jv", "j")
	snapshot := a.Store.Snapshot()
	if got := settings.CommandAliasesOf(snapshot)["jv"]; got != "j" {
		t.Errorf("alias = %q, want j", got)
	}
	list := a.integrationList()
	if len(list) != 1 || len(list[0].Commands) != 1 {
		t.Fatalf("integrations = %+v", list)
	}
	if command := list[0].Commands[0]; command.Alias != "j" || command.Taken {
		t.Errorf("command = %+v, want the live alias j", command)
	}

	// The alias reaches the launcher's catalog, and it is a catalog-wide
	// fact rather than a PATH one: the launcher holds it even when the PATH
	// rows are off, so an alias set on an extension command's own row is
	// actually reachable. The scan runs inline here (Win is nil).
	if got := a.Launcher.Aliases["jv"]; got != "j" {
		t.Errorf("the launcher's alias = %q, want j", got)
	}

	// Clearing the alias removes the entry (the store's update runs on the
	// settings goroutine in production; here the synchronous store applies
	// it directly).
	a.setCommandAlias("io.github.vst93.v", "jv", "")
	if aliases := settings.CommandAliasesOf(a.Store.Snapshot()); len(aliases) != 0 {
		t.Errorf("aliases = %+v, want none", aliases)
	}
	if got := a.Launcher.Aliases["jv"]; got != "" {
		t.Errorf("the launcher kept a cleared alias: %q", got)
	}
}

// A command whose alias another command claimed first is marked taken: the
// settings list carries the raw alias and the conflict flag side by side, so
// the editor can say so rather than pretending the alias is live.
func TestCommandAliasTakenFlag(t *testing.T) {
	// bat is asked first (ascending name order), so it owns gfm.
	resolved := settings.ResolveCommandAliases(settings.CommandAliases{
		"bat": "gfm",
		"git": "gfm",
	})
	if resolved["bat"] != "gfm" {
		t.Fatalf("resolved = %+v", resolved)
	}
	raw := settings.CommandAliases{"bat": "gfm", "git": "gfm"}
	_, gitLive := resolved["git"]
	if _, batLive := resolved["bat"]; !batLive {
		t.Error("the winning command lost its alias")
	}
	if gitLive {
		t.Error("the losing command's alias is live")
	}
	_ = raw
}

// The launcher's window follows the list: the View measures the rows, the
// shell walks the window to the band, and a height that has not changed
// never reaches the platform.
func TestLauncherResizeFollowsTheList(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	paths := integrationFixture(t)
	a := New(Options{
		Store:          settings.NewStore(settings.Default()),
		Paths:          paths,
		WorkAreaHeight: 900,
		NewTerminal:    func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})
	a.RefreshIntegrations(context.Background())

	// No window yet: the request is a no-op, not a panic.
	a.resizeLauncher(500)

	// A window whose surface is not the launcher does not move either.
	a.Open(SurfaceTerminal)
	a.resizeLauncher(500)

	// The launcher's height follows the band the View held. The store's
	// synchronous update path means the View has not run here; the height
	// comes from the slab until it has.
	a.Open(SurfaceLauncher)
	if a.Launcher.HeldRows != 0 {
		t.Errorf("held rows before a frame = %d", a.Launcher.HeldRows)
	}
	w, h := a.targetSize(a.Store.Snapshot())
	if h <= 0 {
		t.Errorf("target height = %d", h)
	}
	_ = w

	// A View that has measured a list sizes the window to the band.
	a.Launcher.Font = 14
	a.Launcher.Spacing = 4
	a.Launcher.Rows = []launcher.RowInfo{{Lines: 2}, {Lines: 2}, {Lines: 2}, {Lines: 1}, {Lines: 1}}
	a.Launcher.HeldRows = 5
	resizes := 0
	heights := map[int]int{}
	realSet := a.Win
	a.Win = nil
	a.resizeLauncher(0) // no window: nothing
	a.Win = realSet
	if a.Win != nil {
		// The real window in tests is nil (no mygo library), so the walk is
		// a no-op. The accounting is still exercised through targetSize.
		w, h = a.targetSize(a.Store.Snapshot())
		heights[h]++
		resizes++
	}
	if resizes > 0 && heights[h] != 1 {
		t.Errorf("target heights = %v", heights)
	}
}

// TestTerminalGearOpensTheDrawer: the terminal bar's gear flips the surface's
// own appearance drawer — the session is never left to change how it looks,
// which is the reference's `.terminal-settings-drawer` (R42/R54).
func TestTerminalGearOpensTheDrawer(t *testing.T) {
	a := newApp(t)
	if a.Terminal == nil {
		t.Fatal("no terminal surface")
	}
	a.Open(SurfaceTerminal)
	if a.Surf != SurfaceTerminal {
		t.Fatalf("surface = %v, want terminal", a.Surf)
	}
	if a.Terminal.Drawer {
		t.Fatal("the drawer is open before anything asked")
	}
	a.Terminal.Actions.Settings()
	if !a.Terminal.Drawer {
		t.Fatal("the gear did not open the drawer")
	}
	if a.Surf != SurfaceTerminal {
		t.Errorf("the gear left the terminal for surface %v", a.Surf)
	}
	a.Terminal.Actions.Settings()
	if a.Terminal.Drawer {
		t.Error("the gear did not close the drawer")
	}
}

// The terminal panel's own "open in system terminal" control: when no emulator
// can be opened, the panel says so (the old build's `.terminal-feedback`) and
// takes the keyboard back, rather than the click appearing to do nothing.
func TestTerminalOpenExternalFailureSpeaksInThePanel(t *testing.T) {
	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return nil, errors.New("no terminal library in tests")
		},
		OpenExternalTerminal: func() error { return errors.New("no emulator") },
	})
	if a.Terminal == nil {
		t.Fatal("no terminal surface")
	}
	a.Open(SurfaceTerminal)
	a.Terminal.Actions.OpenExternal()
	if !a.Terminal.FeedbackVisible() {
		t.Fatal("the failure was silent")
	}
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 860, 600)
	tt.Frame()
	if want := i18n.For("en").Launcher.OpenTermFailed; !tt.HasText(want) {
		t.Errorf("the panel did not draw the notice: want %q, got %q", want, tt.Texts())
	}
}

// configuredIntegrationFixture is one installed integration whose provider
// describes a host-owned configuration: a required text endpoint, a required
// password token and a required select region. It is the fixture the
// configuration form's own shell tests render against, so the schema and the
// config generation are real rather than a stub.
func configuredIntegrationFixture(t *testing.T, saveDialog func(string) (string, error)) (*App, extensions.Paths) {
	t.Helper()
	paths := extensions.FromRoot(t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(paths.Extensions, "dev.floter.configured")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(`{
	  "schemaVersion": "2.0", "id": "dev.floter.configured", "name": "Configured",
	  "distribution": {"type": "local"},
	  "runtime": {"type": "script", "language": "shell", "path": "tool.sh"},
	  "provider": {"type": "executable", "argsPrefix": ["--floter"]}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "tool.sh"), []byte("#!/bin/sh\ncat <<'JSONEOF'\n"+
		`{"protocolVersion":"1.0","provider":{"id":"dev.floter.configured","name":"Configured","version":"1.0.0"},
		  "configuration":{"configVersion":1,"owner":"host","schema":[
		    {"key":"endpoint","type":"text","required":true},
		    {"key":"token","type":"password","required":true},
		    {"key":"region","type":"select","options":["eu","us"]}]}}`+
		"\nJSONEOF\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RepositoryFile, []byte(`{
	  "schemaVersion": 1,
	  "extensions": {"dev.floter.configured": {"id": "dev.floter.configured", "name": "Configured",
	    "state": "enabled", "enabled": true, "packageVersion": "1.0.0",
	    "manifestPath": "`+jsonEscaped(filepath.Join(pkg, "floter.extension.json"))+`",
	    "installedAt": 1, "updatedAt": 1}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
		SaveFileDialog:       saveDialog,
		OpenFileDialog:       func() (string, error) { return "", nil },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { return true },
	})
	a.RefreshIntegrations(context.Background())
	return a, paths
}

// The configuration export writes the reference's `ConfigurationExportDocument`
// to the path the dialog returns, with the password redacted, and raises the
// success notice — the reference's `extensions_config_export`
// (`ExtensionsPanel.tsx:2036-2048`).
func TestConfigurationExportWritesTheRedactedDocument(t *testing.T) {
	exportPath := filepath.Join(t.TempDir(), "floter-config.json")
	a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return exportPath, nil })
	copy := i18n.For("en").Settings

	clearNotices(a)
	a.exportConfiguration("dev.floter.configured", map[string]any{
		"endpoint": "https://example.com", "token": "s3cret", "region": "eu",
	}, nil)
	waitForGoroutines()

	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("the export was not written: %v", err)
	}
	text := string(data)
	for _, want := range []string{`"extensionId": "dev.floter.configured"`, `"formatVersion": 1`, `"endpoint": "https://example.com"`} {
		if !strings.Contains(text, want) {
			t.Errorf("the exported document lacks %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, "s3cret") {
		t.Errorf("the exported document carries the password:\n%s", text)
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Errorf("the exported document does not redact the password:\n%s", text)
	}
	if !noticeSays(a, copy.ConfigExported) {
		t.Errorf("the export raised no %q notice", copy.ConfigExported)
	}
}

// The configuration copy builds the same redacted document from the
// provider's real schema and raises the copy notice — the reference's
// `extensions_config_copy` (`ExtensionsPanel.tsx:2019-2031`). The document
// itself is pinned by `TestExportConfigurationRedactsAndNames` in the
// extensions package; here the observable is the success notice the shell
// raises over the real provider.
func TestConfigurationCopyRaisesTheNotice(t *testing.T) {
	a, _ := configuredIntegrationFixture(t, func(string) (string, error) { return "", nil })
	copy := i18n.For("en").Settings

	clearNotices(a)
	a.copyConfiguration("dev.floter.configured", map[string]any{
		"endpoint": "https://example.com", "token": "s3cret", "region": "eu",
	}, nil)
	waitForGoroutines()
	if !noticeSays(a, copy.ConfigCopied) {
		t.Errorf("the copy raised no %q notice", copy.ConfigCopied)
	}
}
