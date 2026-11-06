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

func configEpochFrame(t *testing.T, dark bool, owner *launcher.App, a *App) *ui.Tester {
	t.Helper()
	owner.Actions.DrawConfig = func(c *ui.Context, config *launcher.Config) {
		a.PluginSheet(c, PluginSheet{Schema: config.Schema, Values: config.Values, Failed: config.Failed,
			Armed: config.Armed, ActionFailed: config.ActionFailed, Change: owner.ChangeConfig,
			Arm: owner.ArmConfig, Run: owner.RunConfig, Unarm: owner.UnarmConfig})
	}
	owner.EnterClipboard()
	owner.OpenConfig(settings.CustomPluginClipboard)
	tt := ui.NewTester(owner.View, 720, 420)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func configEpochApp(dark bool) (*App, *launcher.App, *launcher.Config) {
	s := settings.Default()
	s.Theme, s.Language = appearanceName(dark), "zh"
	store := settings.NewStore(s)
	a, owner := New(store, Actions{}), launcher.New(store, launcher.Actions{})
	sheet := actionLifetimeSheet()
	sheet.Schema.Plugin = settings.CustomPluginClipboard
	sheet.Schema.Fields = append([]plugincfg.Field{
		{Key: "enabled", Kind: plugincfg.Toggle, Label: "记录历史"},
		{Key: "limit", Kind: plugincfg.Number, Label: "保留上限", Min: 0, Max: 100, Step: 1},
	}, sheet.Schema.Fields...)
	config := &launcher.Config{Schema: sheet.Schema, Values: plugincfg.Values{"enabled": true, "limit": float64(10)}}
	owner.Actions.ConfigFor = func(string) *launcher.Config { return config }
	return a, owner, config
}

func TestConfigEpochReopenedWrite(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, accept := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/accepted=%v", appearanceName(dark), accept), func(t *testing.T) {
				a, owner, config := configEpochApp(dark)
				calls := 0
				owner.Actions.ConfigChange = func(plugin, key string, value any) plugincfg.Values {
					calls++
					if key != "enabled" || value != false {
						t.Errorf("wrong write: %s=%v", key, value)
					}
					config.Values = plugincfg.Values{"enabled": false, "limit": float64(25)}
					owner.OpenConfig(plugin) // ConfigFor may reuse a cached object.
					if accept {
						return plugincfg.Values{"enabled": true, "limit": float64(80)}
					}
					return nil
				}
				tt := configEpochFrame(t, dark, owner, a)
				if err := tt.Click("记录历史"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				epochDump(t, tt, fmt.Sprintf("reopen-write-%v", accept), dark)
				if calls != 1 || owner.Config() != config {
					t.Fatalf("write changed sheet/calls=%d", calls)
				}
				if config.Values["enabled"] != false || config.Values["limit"] != float64(25) || config.Failed {
					t.Errorf("reopened sheet inherited old write: values=%v failed=%v", config.Values, config.Failed)
				}
				if tt.HasText(i18n.For("zh").Settings.SaveFailed) {
					t.Error("reopened sheet shows old write refusal")
				}
			})
		}
	}
}

func TestConfigEpochReopenedAction(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, owner, config := configEpochApp(dark)
			calls := 0
			owner.Actions.ConfigAction = func(plugin, key string) bool {
				calls++
				if key != "clear" {
					t.Errorf("wrong action %q", key)
				}
				owner.OpenConfig(plugin)
				return false
			}
			tt := configEpochFrame(t, dark, owner, a)
			if err := tt.Click("清理历史"); err != nil {
				t.Fatal(err)
			}
			if calls != 0 {
				t.Fatal("first press ran action")
			}
			if err := tt.Click("确认清理"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			epochDump(t, tt, "reopen-action", dark)
			if calls != 1 || config.ActionFailed != "" || tt.HasText("清理失败，请重试") {
				t.Errorf("reopened sheet inherited old action refusal: calls=%d failed=%q", calls, config.ActionFailed)
			}
			if err := tt.Click("清理历史"); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText("确认清理") || calls != 1 {
				t.Error("fresh action could not ask without running")
			}
			tt.Key(0, ui.KeyEscape)
			if owner.ConfigOpen() || calls != 1 {
				t.Error("Escape failed to close the sheet without running")
			}
		})
	}
}
