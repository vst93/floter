package launcher

import (
	"testing"

	"floter/internal/plugincfg"
	"floter/internal/settings"
)

func checkConfigActionRetirement(t *testing.T, retype bool) {
	t.Helper()
	for _, failed := range []bool{false, true} {
		name := "armed"
		if failed {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) {
			a := actionApp(func() bool { return false })
			a.clipboard = true
			a.OpenConfig(settings.CustomPluginClipboard)
			a.ArmConfig("clear_history")
			if failed {
				a.RunConfig("clear_history")
			}
			field := a.Config().Schema.Fields[0]
			if retype {
				a.Config().Schema.Fields[0].Kind = plugincfg.Toggle
			} else {
				a.Config().Schema.Fields = nil
			}
			a.syncConfig()
			config := a.Config()
			if config == nil {
				t.Fatal("same-plugin refresh closed the sheet")
			}
			if config.Armed != "" || config.ActionFailed != "" {
				t.Errorf("retired action kept state: armed=%q failed=%q", config.Armed, config.ActionFailed)
			}
			config.Schema.Fields = []plugincfg.Field{field}
			a.syncConfig()
			if config.Armed != "" || config.ActionFailed != "" {
				t.Error("re-added action resurrected confirmation or failure")
			}
			a.ArmConfig(field.Key)
			if config.Armed != field.Key {
				t.Error("re-added action cannot ask for fresh confirmation")
			}
		})
	}
}

func TestConfigLifetimeRemovedAction(t *testing.T) { checkConfigActionRetirement(t, false) }
func TestConfigLifetimeRetypedAction(t *testing.T) { checkConfigActionRetirement(t, true) }
