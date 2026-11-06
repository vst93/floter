package settingsui

import (
	"testing"

	"floter/internal/launcher"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func checkActionCondition(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			s := settings.Default()
			s.Theme, s.Language = appearanceName(dark), "zh"
			store := settings.NewStore(s)
			a := New(store, Actions{})
			owner := launcher.New(store, launcher.Actions{})
			sheet := actionLifetimeSheet()
			sheet.Schema.Plugin = settings.CustomPluginClipboard
			owner.Actions.ConfigFor = func(string) *launcher.Config { return &launcher.Config{Schema: sheet.Schema, Values: sheet.Values} }
			calls := 0
			owner.Actions.ConfigAction = func(string, string) bool {
				calls++
				if scenario == "replaced" {
					sheet.Schema.Fields[0].Help = "新的清理范围"
				}
				if scenario == "rearmed" {
					owner.ArmConfig("clear")
				}
				return false
			}
			owner.Actions.DrawConfig = func(c *ui.Context, config *launcher.Config) {
				a.PluginSheet(c, PluginSheet{Schema: config.Schema, Values: config.Values, Armed: config.Armed, ActionFailed: config.ActionFailed, Arm: owner.ArmConfig, Run: owner.RunConfig, Unarm: owner.UnarmConfig})
			}
			owner.EnterClipboard()
			owner.OpenConfig(settings.CustomPluginClipboard)
			tt := ui.NewTester(owner.View, 720, 400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			if err := tt.Click("清理历史"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if owner.Config().Armed != "clear" || !tt.HasText("确认清理") {
				t.Fatal("action did not arm from its row")
			}
			if scenario == "replaced" || scenario == "rearmed" {
				if err := tt.Click("确认清理"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				if scenario == "rearmed" {
					if owner.Config().Armed != "clear" {
						t.Error("old result consumed new confirmation")
					}
					// Closing this new question must not uncover the old failure.
					if err := tt.Click("取消清理"); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
					tt.Frame()
				}
				runConditionDump(t, tt, "action-"+scenario, dark)
				if owner.Config().ActionFailed != "" || tt.HasText("清理失败，请重试") {
					t.Errorf("%s action inherited callback's obsolete refusal", scenario)
				}
				if calls != 1 {
					t.Errorf("callback dispatched %d commands, want 1", calls)
				}
				return
			}
			if scenario == "failed" {
				if err := tt.Click("确认清理"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				if !tt.HasText("清理失败，请重试") {
					t.Fatal("action refusal missing")
				}
			}
			// The same key now asks a different question, with a different scope.
			sheet.Schema.Fields[0].Confirm = "确认清理全部历史"
			sheet.Schema.Fields[0].Help = "包括收藏的历史记录"
			tt.Frame()
			tt.Frame()
			runConditionDump(t, tt, "action-"+scenario, dark)
			if scenario == "armed" {
				if owner.Config().Armed != "" || tt.HasText("确认清理全部历史") {
					t.Error("changed question inherited old confirmation")
				}
				if calls != 0 {
					t.Errorf("schema refresh ran %d commands", calls)
				}
			} else {
				if owner.Config().ActionFailed != "" || tt.HasText("清理失败，请重试") {
					t.Error("changed action inherited old refusal")
				}
				if calls != 1 {
					t.Errorf("schema refresh ran %d commands, want 1", calls)
				}
			}
		})
	}
}

func TestActionConditionChangedQuestion(t *testing.T)     { checkActionCondition(t, "armed") }
func TestActionConditionChangedFailure(t *testing.T)      { checkActionCondition(t, "failed") }
func TestActionConditionCallbackReplacement(t *testing.T) { checkActionCondition(t, "replaced") }
func TestActionConditionCallbackRearm(t *testing.T)       { checkActionCondition(t, "rearmed") }
