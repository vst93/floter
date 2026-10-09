package shell

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
	"floter/internal/settings"
)

func newApp(t *testing.T) *App {
	t.Helper()
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store: store,
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return nil, errors.New("no terminal library in tests")
		},
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
