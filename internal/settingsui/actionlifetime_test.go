package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/launcher"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func actionLifetimeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HS_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hs-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func actionLifetimeSheet() PluginSheet {
	return PluginSheet{Schema: &plugincfg.Schema{Plugin: "local.history", Title: "历史记录配置", Fields: []plugincfg.Field{
		{Key: "clear", Label: "清理历史", Kind: plugincfg.Action, Confirm: "确认清理", Cancel: "取消清理", Failed: "清理失败，请重试"},
	}}, Values: plugincfg.Values{}}
}

func checkActionLifetimeKeyboard(t *testing.T, outcome string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := actionLifetimeSheet()
			arms, runs, cancels := 0, 0, 0
			sheet.Arm = func(key string) { arms++; sheet.Armed = key; sheet.ActionFailed = "" }
			sheet.Unarm = func(string) { cancels++; sheet.Armed = "" }
			sheet.Run = func(key string) {
				runs++
				sheet.Armed = ""
				if outcome == "failure" {
					sheet.ActionFailed = key
				}
			}
			tt := pluginIdentityFrame(t, dark, &sheet)
			for i := 0; i < 4 && !tt.Focused("清理历史"); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused("清理历史") {
				t.Fatal("action did not take focus")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			if arms != 1 || sheet.Armed != "clear" {
				t.Fatalf("action did not arm: %d %q", arms, sheet.Armed)
			}
			if outcome == "arm" {
				actionLifetimeDump(t, tt, outcome, dark)
				if !tt.Focused("确认清理") {
					t.Error("arming lost confirmation keyboard focus")
				}
				return
			}
			if outcome == "cancel" {
				for i := 0; i < 4 && !tt.Focused("取消清理"); i++ {
					tt.Key(0, ui.KeyTab)
					tt.Frame()
				}
				if !tt.Focused("取消清理") {
					t.Fatal("cancel did not take focus")
				}
			} else if !tt.Focused("确认清理") {
				t.Fatal("confirm did not retain focus")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			actionLifetimeDump(t, tt, outcome, dark)
			if !tt.Focused("清理历史") {
				t.Errorf("%s lost action keyboard focus", outcome)
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			if arms != 2 || sheet.Armed != "clear" {
				t.Errorf("%s cannot rearm by keyboard: arms=%d armed=%q", outcome, arms, sheet.Armed)
			}
			if outcome == "cancel" && (runs != 0 || cancels != 1) {
				t.Errorf("cancel dispatched runs=%d cancels=%d", runs, cancels)
			}
			if outcome != "cancel" && runs != 1 {
				t.Errorf("confirmation dispatched %d runs", runs)
			}
		})
	}
}

func TestActionLifetimeArmKeyboard(t *testing.T)     { checkActionLifetimeKeyboard(t, "arm") }
func TestActionLifetimeCancelKeyboard(t *testing.T)  { checkActionLifetimeKeyboard(t, "cancel") }
func TestActionLifetimeFailureKeyboard(t *testing.T) { checkActionLifetimeKeyboard(t, "failure") }
func TestActionLifetimeSuccessKeyboard(t *testing.T) { checkActionLifetimeKeyboard(t, "success") }

func checkActionLifetimeSchema(t *testing.T, retype bool) {
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
			owner.Actions.ConfigAction = func(string, string) bool { return false }
			owner.Actions.DrawConfig = func(c *ui.Context, config *launcher.Config) {
				a.PluginSheet(c, PluginSheet{Schema: config.Schema, Values: config.Values, Armed: config.Armed, ActionFailed: config.ActionFailed, Arm: owner.ArmConfig, Run: owner.RunConfig, Unarm: owner.UnarmConfig})
			}
			owner.EnterClipboard()
			owner.OpenConfig(settings.CustomPluginClipboard)
			tt := ui.NewTester(owner.View, 720, 400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			// The row label is the first-press target. Use the actual UI event.
			if err := tt.Click("清理历史"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if !tt.HasText("确认清理") {
				t.Fatal("action did not ask for confirmation")
			}
			field := sheet.Schema.Fields[0]
			name := "removed"
			if retype {
				sheet.Schema.Fields[0].Kind = plugincfg.Toggle
				name = "retyped"
			} else {
				sheet.Schema.Fields = nil
			}
			tt.Frame()
			sheet.Schema.Fields = []plugincfg.Field{field}
			tt.Frame()
			tt.Frame()
			actionLifetimeDump(t, tt, name, dark)
			if tt.HasText("确认清理") || owner.Config().Armed != "" {
				t.Error("re-added action still asks to run the retired confirmation")
			}
		})
	}
}

func TestActionLifetimeRemovedSchema(t *testing.T) { checkActionLifetimeSchema(t, false) }
func TestActionLifetimeRetypedSchema(t *testing.T) { checkActionLifetimeSchema(t, true) }
