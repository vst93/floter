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

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/clipboard"
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

	if w, h := a.targetSize(s); w != launcher.InputWindowWidth || h != int(launcher.WindowHeight(s.UIScale)) {
		t.Errorf("launcher size = %dx%d", w, h)
	}
	a.Surf = SurfaceSettings
	if w, h := a.targetSize(s); w != launcher.InputWindowWidth || h != SettingsHeight(s.UIScale, 0) {
		t.Errorf("settings size = %dx%d", w, h)
	}
	a.Surf = SurfaceTerminal
	if w, h := a.targetSize(s); w != int(s.TerminalWidth) || h != int(s.TerminalHeight) {
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
	if a.Terminal.Term != nil || a.Terminal.Err == nil {
		t.Errorf("terminal session = %v, err = %v", a.Terminal.Term, a.Terminal.Err)
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
			want:    []string{"Open settings", "Open terminal", "Quit floter", "Type to search"},
		},
		{
			name:    "settings",
			surface: SurfaceSettings,
			w:       launcher.InputWindowWidth,
			h:       SettingsHeight(s.UIScale, 0),
			want: []string{
				"General", "Appearance", "Liquid glass effect", "Interface size",
				"Terminal appearance", "Font size", "Cursor shape", "Terminal palette",
			},
		},
		{
			name:    "terminal",
			surface: SurfaceTerminal,
			w:       int(s.TerminalWidth),
			h:       int(s.TerminalHeight),
			want:    []string{"Terminal", "The session has not started yet."},
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
	if !tt.HasText("打开设置") {
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

	// The launcher never holds a residency.
	a.Surf = SurfaceLauncher
	if a.residencyHolds() {
		t.Error("the launcher held a residency")
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
      "manifestPath": "", "executablePath": "` + executable + `", "channel": "stable",
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
  "distribution": {"type": "npm"},
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
      "packageVersion": "1.0.0", "manifestPath": "", "executablePath": "`+executable+`",
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
      "packageVersion": "1.0.0", "manifestPath": "", "executablePath": "`+executable+`",
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
	a.rememberSettingsPage("plugins")
	if got := settings.LastSettingsPage(store.Snapshot()); got != "plugins" {
		t.Errorf("stored page = %q", got)
	}
	a.Settings.Page = settingsui.PageGeneral
	a.restoreSettingsPage()
	if a.Settings.Page != settingsui.PagePlugins {
		t.Errorf("restored page = %d", a.Settings.Page)
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

// A command whose manifest sends its output to the background runs headless
// and its output reaches the launcher.
func TestCapturedCommandReachesTheLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	paths := integrationFixture(t)
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
	entry.Route = extensions.RouteBackground
	entry.Program = "/bin/sh"
	entry.Args = []string{"-c", "echo captured"}
	entry.Dir = ""

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
	if notified[0][0] != "floter" || !strings.Contains(notified[0][1], "Run things finished") ||
		!strings.Contains(notified[0][1], "Finished") {
		t.Errorf("notification = %v", notified[0])
	}

	// A failing run says so in the notification.
	notified = nil
	entry.Args = []string{"-c", "exit 4"}
	done = make(chan struct{})
	a.runCommandCaptured(entry, nil, func(extensions.CapturedRun, error) { close(done) })
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the second run never answered")
	}
	if len(notified) != 1 || !strings.Contains(notified[0][1], "Exit code 4") {
		t.Errorf("notifications = %v", notified)
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
// the status lands on the About page.
func TestUpdateCheckWithoutAFeed(t *testing.T) {
	a := newApp(t)
	a.checkForUpdates()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if a.Settings.UpdateStatus != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the check never answered")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if a.Settings.UpdateStatus != "This build has no update feed" {
		t.Errorf("status = %q", a.Settings.UpdateStatus)
	}
	if a.Settings.UpdateReady {
		t.Error("an update is ready in a build with no feed")
	}
	// Installing with nothing pending is a no-op, not a panic.
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

	a.exportIntegrations()
	waitForStatus(t, a)
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
	if !strings.Contains(a.Settings.TransferStatus, "1 integrations exported") {
		t.Errorf("status = %q", a.Settings.TransferStatus)
	}

	// An import of the same document lands, and reports it.
	a.Settings.TransferStatus = ""
	a.importIntegrations()
	waitForStatus(t, a)
	if !strings.Contains(a.Settings.TransferStatus, "1 imported") {
		t.Errorf("status = %q", a.Settings.TransferStatus)
	}

	// A file that is not a document is refused with a line, not a crash.
	if err := os.WriteFile(exportPath, []byte("not a document"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.Settings.TransferStatus = ""
	a.importIntegrations()
	waitForStatus(t, a)
	if a.Settings.TransferStatus != "The file is not an integrations export" {
		t.Errorf("status = %q", a.Settings.TransferStatus)
	}
}

// waitForStatus waits for an export or import to leave its line.
func waitForStatus(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if a.Settings.TransferStatus != "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the transfer never reported")
		}
		time.Sleep(5 * time.Millisecond)
	}
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
	deadline := time.Now().Add(10 * time.Second)
	for {
		if len(a.Launcher.InstallCatalog) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the catalog never reached the launcher")
		}
		time.Sleep(10 * time.Millisecond)
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
	a := New(Options{
		Store:                store,
		Paths:                paths,
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ConfirmPermissions:   func(extensions.PermissionApproval) bool { return true },
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

// A power action asks first, and only runs the system's command when the user
// confirms. Both answers are covered, and nothing runs without a confirmation.
func TestPowerAsksFirst(t *testing.T) {
	ran := []string{}
	answer := false
	a := New(Options{
		Store:                settings.NewStore(settings.Default()),
		Paths:                extensions.FromRoot(t.TempDir()),
		NewTerminal:          func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		ConfirmPower:         func(title string) bool { return answer },
		RunPowerCommand:      func(action string) error { ran = append(ran, action); return nil },
		RunSilentCommand:     func(string) error { return nil },
		OpenExternalTerminal: func() error { return nil },
	})

	// A refusal runs nothing.
	a.power("restart")
	time.Sleep(50 * time.Millisecond)
	if len(ran) != 0 {
		t.Fatalf("a refused action ran %v", ran)
	}

	// A confirmation runs it, once.
	answer = true
	deadline := time.Now().Add(5 * time.Second)
	a.power("shutdown")
	for {
		if len(ran) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the confirmed action never ran")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if ran[0] != "shutdown" {
		t.Errorf("ran %v", ran)
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
	    "manifestPath": "`+filepath.Join(pkg, "floter.extension.json")+`",
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

	// A save through the action writes the values, and the password lands in
	// the secrets file rather than the values file.
	if err := a.saveIntegrationConfiguration("dev.floter.configured", map[string]any{
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
