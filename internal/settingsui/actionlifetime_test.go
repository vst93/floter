package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/plugincfg"
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
