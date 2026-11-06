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

func TestConfigLifetimeRunRequiresConfirmation(t *testing.T) {
	runs := 0
	a := actionApp(func() bool { runs++; return true })
	a.OpenConfig("clipboard")
	a.RunConfig("clear_history")
	if runs != 0 {
		t.Errorf("unconfirmed action ran %d times", runs)
	}
	a.ArmConfig("clear_history")
	a.RunConfig("clear_history")
	a.RunConfig("clear_history")
	if runs != 1 {
		t.Errorf("one confirmation dispatched %d commands", runs)
	}
}

func TestConfigLifetimeStaleActionCannotConsumeAnother(t *testing.T) {
	runs := 0
	a := actionApp(func() bool { runs++; return true })
	a.OpenConfig("clipboard")
	a.ArmConfig("clear_history")
	a.ArmConfig("removed")
	if a.Config().Armed != "clear_history" {
		t.Errorf("stale arm replaced current confirmation: %q", a.Config().Armed)
	}
	a.RunConfig("removed")
	if runs != 0 || a.Config().Armed != "clear_history" {
		t.Errorf("stale run consumed current confirmation: runs=%d armed=%q", runs, a.Config().Armed)
	}
	// An existing value field is not an action either.
	a.Config().Schema.Fields = append(a.Config().Schema.Fields, plugincfg.Field{Key: "enabled", Kind: plugincfg.Toggle})
	a.ArmConfig("enabled")
	a.RunConfig("enabled")
	if runs != 0 || a.Config().Armed != "clear_history" {
		t.Error("value field consumed current confirmation")
	}
	a.RunConfig("clear_history")
	if runs != 1 {
		t.Errorf("valid confirmation dispatched %d commands", runs)
	}
	// A schema can retire an armed action before the next drawing frame.
	a.ArmConfig("clear_history")
	a.Config().Schema.Fields[0].Kind = plugincfg.Text
	a.RunConfig("clear_history")
	if runs != 1 {
		t.Error("retyped action ran before the next sync frame")
	}
}
