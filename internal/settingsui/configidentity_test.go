package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

func configIdentityFrame(t *testing.T, dark bool, integration *Integration) (*App, *ui.Tester) {
	t.Helper()
	s := settings.Default()
	s.Theme, s.Language = appearanceName(dark), "en"
	a := New(settings.NewStore(s), Actions{})
	a.configOpen = map[string]bool{integration.ID: true}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		ui.Column(c).FillWidth().Padding(20).Children(func() {
			a.extensionMetrics(func() { a.configFields(c, a.copy(), *integration) })
		})
	}, 720, 320)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return a, tt
}

func configIdentityFocus(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r := mustFind(t, tt, label)
	tt.ClickAt(240, r.Y+r.H/2)
	tt.Frame()
	if !tt.Focused(label) {
		t.Fatalf("%s did not take focus", label)
	}
	tt.Key(ui.Cmd, ui.KeyA)
}

func configIdentityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HD_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hd-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfigIdentityReorderingKeepsTheActiveField(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{
				{Key: "endpoint", Label: "Endpoint", Type: "text"},
				{Key: "directory", Label: "Directory", Type: "path"},
			}, ConfigValues: map[string]any{"endpoint": "https://localhost", "directory": "/workspace"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "Directory")
			integration.Config[0], integration.Config[1] = integration.Config[1], integration.Config[0]
			tt.Frame()
			tt.Type("/workspace/search")
			tt.Frame()
			configIdentityDump(t, tt, "config-reorder", dark)
			values := a.draft(integration).values
			if values["directory"] != "/workspace/search" || values["endpoint"] != "https://localhost" {
				t.Errorf("reorder wrote into another field: %v", values)
			}
		})
	}
}

func TestConfigIdentityErrorUsesCurrentLanguage(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{
				{Key: "limit", Label: "结果上限", Type: "number"},
			}, ConfigValues: map[string]any{"limit": float64(5)}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "结果上限")
			tt.Type("1e")
			tt.Frame()
			old := a.copy().ConfigNumber
			if !tt.HasText(old) {
				t.Fatal("invalid number did not display its error")
			}
			if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			configIdentityDump(t, tt, "config-error-language", dark)
			if !tt.HasText(a.copy().ConfigNumber) || tt.HasText(old) {
				t.Errorf("field error kept previous language: %q", tt.Texts())
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("12")
			tt.Frame()
			if a.configFieldProblem(integration, integration.Config[0]) != "" || a.draft(integration).values["limit"] != float64(12) {
				t.Error("translated error did not recover on valid input")
			}
		})
	}
}
