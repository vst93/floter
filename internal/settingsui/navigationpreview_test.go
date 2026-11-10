package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
	"floter/internal/theme"
	"github.com/egoist/mygo/ui"
)

func navigationFixture() *App {
	s := settings.Default()
	s.Language = "zh"
	a := New(settings.NewStore(s), Actions{SetCommandEnabled: func(string, string, bool) {}, OpenPluginConfig: func(string) {}, SetPluginEnabled: func(string, bool) {}})
	a.Page = PageIntegrations
	a.Integrations = func() []Integration {
		var out []Integration
		for i, name := range []string{"Ripgrep", "Git", "Docker", "Node.js", "Python", "V Tools"} {
			out = append(out, Integration{ID: fmt.Sprint(i), Name: name, Description: "本地命令行工具", Enabled: true, RuntimeAvailable: true, Commands: []Command{{ID: "search", Name: "Search", Enabled: true, Declared: true, Available: true}}})
		}
		return out
	}
	return a
}

func TestSettingsNavigationPreview(t *testing.T) {
	dir := os.Getenv("FLOTER_NAV_PREVIEW")
	if dir == "" {
		t.Skip("set FLOTER_NAV_PREVIEW for visual evidence")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, appearance := range []string{"light", "dark"} {
		a := navigationFixture()
		items := a.Integrations()
		items[1].Enabled = false
		items[2].RuntimeAvailable = false
		items[3].Broken = true
		items[4].Orphan = true
		a.Integrations = func() []Integration { return items }
		s := a.Store.Snapshot()
		s.Theme = appearance
		a.Store = settings.NewStore(s)
		tt := ui.NewTester(func(c *ui.Context) {
			look := theme.ForPreferences(a.Store.Snapshot(), c.Theme().Dark, c.Preferences())
			c.SetTheme(look.Theme)
			a.View(c)
		}, 720, 580)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		save := func(name string) {
			t.Helper()
			f, err := os.Create(filepath.Join(dir, name+"-"+appearance+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
		save("integrations")
		a.Body.Y = a.Body.MaxY
		tt.Frame()
		save("integrations-scrolled")
		a.Body.Y = 0
		tt.Frame()
		a.openIntegrationDetail(integrationKey{ID: "0"})
		tt.Frame()
		save("tool-detail")
		a.closeIntegrationDetail()
		a.Actions.Detected = func() []DetectedTool {
			return []DetectedTool{{Name: "ffmpeg", Description: "音视频转换工具", Path: "/opt/homebrew/bin/ffmpeg", RuntimeAvailable: true}}
		}
		a.Actions.ConnectDetected = func(DetectedTool) {}
		a.Actions.ConnectLocalTool = func(CustomTool) {}
		a.Actions.InstallFromRegistry = func(string, string) {}
		a.Actions.InstallFromFolder = func(string) {}
		a.Actions.ChooseFolder = func() (string, error) { return "", nil }
		a.Actions.ExportIntegrations, a.Actions.ImportIntegrations = func() {}, func() {}
		for method, name := range []string{"add-detected", "add-local", "add-package"} {
			a.integrationNav.tab, a.integrationNav.addMethod = 1, method
			a.Body.Y = 0
			tt.Frame()
			save(name)
		}
		a.integrationNav.tab, a.Body.Y = 2, 0
		tt.Frame()
		save("transfer")
		a.Page, a.Body.Y = PageGeneral, 0
		tt.Frame()
		save("general")

	}
}
