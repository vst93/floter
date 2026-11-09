package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/browser"
	"floter/internal/settings"
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
