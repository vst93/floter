package settingsui

import (
	"fmt"
	"testing"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func checkWriteResult(t *testing.T, scenario string, accepted bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("%s/accepted=%v", appearanceName(dark), accepted), func(t *testing.T) {
			s := settings.Default()
			s.Theme, s.Language = appearanceName(dark), "zh"
			store := settings.NewStore(s)
			a := New(store, Actions{})
			owner := launcher.New(store, launcher.Actions{})
			config := &launcher.Config{Schema: &plugincfg.Schema{Plugin: settings.CustomPluginClipboard, Title: "剪贴板配置", Fields: []plugincfg.Field{
				{Key: "enabled", Kind: plugincfg.Toggle, Label: "记录历史"},
				{Key: "limit", Kind: plugincfg.Number, Label: "保留上限", Min: 0, Max: 100, Step: 1},
				{Key: "format", Kind: plugincfg.Select, Label: "保存格式", Options: []plugincfg.Option{{Value: "text", Label: "文字"}, {Value: "html", Label: "网页"}}},
			}}, Values: plugincfg.Values{"enabled": true, "limit": float64(10), "format": "text"}}
			owner.Actions.ConfigFor = func(string) *launcher.Config { return config }
			calls := 0
			var lastValue any
			owner.Actions.ConfigChange = func(plugin, key string, value any) plugincfg.Values {
				calls++
				lastValue = value
				if plugin != settings.CustomPluginClipboard {
					t.Errorf("wrong owner %q", plugin)
				}
				switch scenario {
				case "owner":
					config.Schema.Plugin = "replacement"
					config.Values = plugincfg.Values{"enabled": false, "limit": float64(25), "format": "text"}
				case "kind":
					config.Schema.Fields[0].Kind = plugincfg.Text
					config.Values = plugincfg.Values{"enabled": "当前配置", "limit": float64(25), "format": "text"}
				case "removed":
					config.Schema.Fields = config.Schema.Fields[1:]
					config.Values = plugincfg.Values{"limit": float64(25), "format": "text"}
				case "bounds":
					config.Schema.Fields[1].Max = 5
					config.Values = plugincfg.Values{"enabled": true, "limit": float64(5), "format": "text"}
				case "options":
					config.Schema.Fields[2].Options[1].Value = "markdown"
					config.Schema.Fields[2].Options[1].Label = "Markdown"
					config.Values = plugincfg.Values{"enabled": true, "limit": float64(25), "format": "markdown"}
				}
				if accepted {
					return plugincfg.Values{"enabled": true, "limit": float64(10), "format": "text"}
				}
				return nil
			}
			owner.OpenConfig(settings.CustomPluginClipboard)
			ground := realPageGround(t, dark)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(ground)
				a.PluginSheet(c, PluginSheet{Schema: config.Schema, Values: config.Values, Failed: config.Failed, Change: owner.ChangeConfig})
			}, 720, 420)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			switch scenario {
			case "bounds":
				r := mustFind(t, tt, "保留上限")
				tt.ClickAt(660, r.Y+r.H/2)
				if !tt.Focused("保留上限") {
					t.Fatal("number input did not receive focus")
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("12")
				tt.Key(0, ui.KeyEnter)
			case "options":
				if err := tt.Click("文字"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if err := tt.Click("网页"); err != nil {
					t.Fatal(err)
				}
			default:
				if err := tt.Click("记录历史"); err != nil {
					t.Fatal(err)
				}
			}
			tt.Frame()
			tt.Frame()
			resultDump(t, tt, fmt.Sprintf("write-%s-%v", scenario, accepted), dark)
			if calls != 1 {
				t.Fatalf("write dispatched %d times, want 1; texts=%v", calls, tt.Texts())
			}
			if config.Failed || tt.HasText(i18n.For("zh").Settings.SaveFailed) {
				t.Error("refreshed field inherited old write refusal")
			}
			switch scenario {
			case "owner":
				if config.Values["enabled"] != false || config.Values["limit"] != float64(25) {
					t.Errorf("new owner values overwritten: %v", config.Values)
				}
			case "kind":
				if config.Values["enabled"] != "当前配置" {
					t.Errorf("new field kind values overwritten: %v", config.Values)
				}
				// Read the actual editor through an edit/submit event: Texts()
				// indexes static labels, not the text input's private buffer.
				r := mustFind(t, tt, "记录历史")
				tt.ClickAt(100, r.Y+r.H+15)
				if !tt.Focused("记录历史") {
					t.Fatal("replacement text input not focused")
				}
				tt.Key(0, ui.KeyEnd)
				tt.Type("_checked")
				tt.Key(0, ui.KeyEnter)
				if calls != 2 || lastValue != "当前配置_checked" {
					t.Errorf("replacement editor submitted %v in %d calls", lastValue, calls)
				}
			case "removed":
				if _, exists := config.Values["enabled"]; exists || config.Values["limit"] != float64(25) {
					t.Errorf("removed field resurrected by result: %v", config.Values)
				}
			case "bounds":
				if config.Values["limit"] != float64(5) {
					t.Errorf("new bounds values overwritten: %v", config.Values)
				}
			case "options":
				if config.Values["format"] != "markdown" {
					t.Errorf("new options values overwritten: %v", config.Values)
				}
			}
		})
	}
}

func TestWriteResultOwnerChangedInsideCallback(t *testing.T) {
	for _, accept := range []bool{false, true} {
		checkWriteResult(t, "owner", accept)
	}
}

func TestWriteResultFieldChangedInsideCallback(t *testing.T) {
	for _, scenario := range []string{"kind", "removed", "bounds", "options"} {
		t.Run(scenario, func(t *testing.T) {
			for _, accept := range []bool{false, true} {
				checkWriteResult(t, scenario, accept)
			}
		})
	}
}
