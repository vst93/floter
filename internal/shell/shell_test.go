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
	"strings"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"floter/internal/clipboard"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
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
	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
		Paths:       paths,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
	})

	a.RefreshIntegrations(context.Background())
	if got := a.Launcher.Commands; len(got) != 1 || got[0].Command.ID != "jv" {
		t.Fatalf("launcher commands = %+v", got)
	}

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
	if !state.enabled || state.maxItems != clipboard.DefaultMaxItems {
		t.Errorf("defaults = %+v", state)
	}

	// The keys are ones this app does not own, so they ride in the
	// carried-through map.
	s, err := settings.Parse([]byte(`{"clipboard_history_enabled": false, "clipboard_history_max_items": 120}`))
	if err != nil {
		t.Fatal(err)
	}
	state = clipboardState(s)
	if state.enabled || state.maxItems != 120 {
		t.Errorf("parsed = %+v", state)
	}
	if extra := s.Extra(); extra["clipboard_history_max_items"] == nil {
		t.Errorf("the key is not carried through: %v", extra)
	}

	// A hand-edited number outside the band is clamped by the store.
	loose, err := settings.Parse([]byte(`{"clipboard_history_max_items": 100000}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := clipboardMaxItems(loose); got != 100000 {
		t.Errorf("max items = %d, want the setting passed through", got)
	}
	store := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), clipboardMaxItems(loose))
	if err := store.SetMaxItems(clipboardMaxItems(loose)); err != nil {
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

	a := New(Options{
		Store:       settings.NewStore(settings.Default()),
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
