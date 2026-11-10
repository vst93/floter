package shell

import (
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/browser"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

func TestBrowserDiscoveryRefreshesAnEmptyScanAndOpenSheet(t *testing.T) {
	a := pluginConfigApp(t)
	a.homeDir = t.TempDir()
	base := t.TempDir()
	a.Store.Preview(func(s *settings.Settings) {
		p := settings.BrowserPluginOf(*s)
		p.CustomBaseDir = base
		s.SetBrowserPlugin(p)
	})
	a.browserProfileList(base)
	// A profile created after the first empty scan appears on the next open.
	if err := os.MkdirAll(filepath.Join(base, "Default"), 0755); err != nil {
		t.Fatal(err)
	}
	config := a.pluginConfig(settings.CustomPluginBrowser)
	profiles := a.browserProfileList(base)
	if len(profiles) != 1 || profiles[0].BrowserID != browser.BrowserCustom {
		t.Fatalf("cached empty scan: %+v", profiles)
	}
	tt := ui.NewTester(func(c *ui.Context) { a.drawPluginConfig(c, config) }, 720, 900)
	tt.Frame()
	field, _ := config.Schema.Field("target")
	found := false
	for _, option := range field.Options {
		found = found || option.Value == browser.BrowserCustom
	}
	if !found {
		t.Fatalf("open configuration did not refresh: %+v", field.Options)
	}
}

func TestBrowserConfigurationNeverWaitsForInFlightDiscovery(t *testing.T) {
	a := pluginConfigApp(t)
	done := make(chan struct{})
	a.browserScans = map[string]*browserScan{"": {
		profiles: []browser.Profile{{BrowserID: browser.BrowserEdge, Browser: "Microsoft Edge"}},
		done:     done,
	}}
	answered := make(chan bool, 1)
	go func() {
		options := a.browserTargets()
		answered <- len(options) == 1 && options[0].ID == browser.BrowserEdge
	}()
	select {
	case ok := <-answered:
		if !ok {
			t.Fatal("in-flight refresh erased existing Edge option")
		}
	case <-time.After(time.Second):
		t.Fatal("configuration blocked on disk discovery")
	}
}

func TestSettingsSurfaceSwitchDoesNotReplayWindowEntrance(t *testing.T) {
	a := newChromeTestApp(t)
	a.arrivalSeen = func() bool { return true }
	for _, surface := range []Surface{SurfaceSettings, SurfaceLauncher, SurfaceSettings, SurfaceLauncher} {
		a.startShellEnter()
		a.Open(surface)
		if a.enterLive {
			t.Fatal("page switch replayed the whole-window entrance")
		}
	}
}

func TestBrowserDeniedEdgeOptionAndRecoveryGuidance(t *testing.T) {
	a := pluginConfigApp(t)
	a.homeDir = t.TempDir()
	done := make(chan struct{})
	close(done)
	a.browserScans = map[string]*browserScan{"": {profiles: []browser.Profile{
		{BrowserID: browser.BrowserChrome, Browser: "Google Chrome"},
		{BrowserID: browser.BrowserEdge, Browser: "Microsoft Edge", ReadError: os.ErrPermission},
	}, finished: time.Now(), done: done}}
	copy := i18n.For("zh").Settings
	state := settings.BrowserPluginOf(a.settings())
	schema, _ := a.browserConfig(copy, state)
	field, _ := schema.Field("target")
	var edge bool
	for _, option := range field.Options {
		if option.Value == browser.BrowserEdge {
			edge = true
			if !strings.Contains(option.Label, copy.BrowserUnavailable) {
				t.Fatalf("missing access status: %q", option.Label)
			}
		}
	}
	if !edge || !strings.Contains(field.Note, "Microsoft Edge") {
		t.Fatalf("denied Edge disappeared: %+v", field)
	}
	hint := copy.BrowserAccessHint
	if runtime.GOOS == "darwin" {
		hint = copy.BrowserAccessMacHint
	}
	if !strings.Contains(field.Note, hint) {
		t.Fatalf("no recovery guidance: %s", field.Help)
	}
	state.Target = browser.BrowserChrome
	schema, _ = a.browserConfig(copy, state)
	field, _ = schema.Field("target")
	if field.Help != copy.BrowserTargetHint || field.Note != "" {
		t.Fatal("Edge failure leaked into Chrome-only guidance")
	}
	// Reopening schedules a fresh discovery, clears stale errors and keeps drafts.
	a.pluginConfig(settings.CustomPluginBrowser)
	if profiles := a.browserProfileList(""); len(profiles) != 0 {
		t.Fatalf("stale denied result after retry: %+v", profiles)
	}
}

func TestBrowserAccessGuidanceIsVisibleInConfiguration(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := pluginConfigApp(t)
		a.Store.Preview(func(s *settings.Settings) {
			s.Language = "zh"
			p := settings.BrowserPluginOf(*s)
			p.Target = browser.BrowserEdge
			s.SetBrowserPlugin(p)
		})
		done := make(chan struct{})
		close(done)
		a.browserScans = map[string]*browserScan{"": {profiles: []browser.Profile{{BrowserID: browser.BrowserEdge, Browser: "Microsoft Edge", ReadError: os.ErrPermission}}, finished: time.Now(), done: done}}
		schema, values := a.browserConfig(i18n.For("zh").Settings, settings.BrowserPluginOf(a.settings()))
		config := &launcher.Config{Schema: schema, Values: values}
		tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(theme.For(a.settings(), dark).Theme); a.drawPluginConfig(c, config) }, 720, 720)
		tt.SetDark(dark)
		tt.Frame()
		field, _ := config.Schema.Field("target")
		note, ok := tt.Find(field.Note)
		if !ok || note.H < 30 || note.W < 500 || note.Y+note.H > 720 {
			t.Fatalf("access guidance is truncated or hidden: %+v", note)
		}
		if dir := os.Getenv("FLOTER_P15_DUMP"); dir != "" {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			f, err := os.Create(filepath.Join(dir, "edge-config-"+appearance+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
