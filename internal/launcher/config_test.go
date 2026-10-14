package launcher

import (
	"testing"

	"floter/internal/plugincfg"
)

// configApp is a launcher with one toggle field, whose write is the caller's
// to accept or refuse. `accept` is a closure so a test can flip the backend's
// answer between two changes.
func configApp(accept func() bool) *App {
	a := testApp()
	a.Actions.ConfigFor = func(plugin string) *Config {
		return &Config{
			Schema: &plugincfg.Schema{Plugin: plugin, Fields: []plugincfg.Field{
				{Key: "enabled", Kind: plugincfg.Toggle, Label: "Keep a history"},
			}},
			Values: plugincfg.Values{"enabled": true},
		}
	}
	a.Actions.ConfigChange = func(plugin, key string, value any) plugincfg.Values {
		if !accept() {
			return nil
		}
		// The plugin's own normalization has the last word: the stored map is
		// not always the painted one.
		return plugincfg.Values{"enabled": value, "normalized": true}
	}
	return a
}

// A refused write rolls the control back to the last value the backend
// confirmed and reports the failure line, instead of keeping a value that was
// never stored (the old build's R113, `configChangeOutcome`). Before this the
// sheet kept the painted `false` on screen while the settings still held
// `true`.
func TestRefusedConfigChangeRollsBack(t *testing.T) {
	a := configApp(func() bool { return false })
	a.OpenConfig("clipboard")
	a.ChangeConfig("enabled", false)

	config := a.Config()
	if config.Values["enabled"] != true {
		t.Errorf("a refused write kept enabled=%v, want the committed true", config.Values["enabled"])
	}
	if !config.Failed {
		t.Error("a refused write did not report the failure line")
	}
}

// An accepted write commits what the plugin actually stored (its own
// normalization), not the value that was painted, and clears the failure line.
func TestAcceptedConfigChangeCommitsTheStoredValues(t *testing.T) {
	a := configApp(func() bool { return true })
	a.OpenConfig("clipboard")
	a.ChangeConfig("enabled", false)

	config := a.Config()
	if config.Values["enabled"] != false {
		t.Errorf("an accepted write left enabled=%v, want false", config.Values["enabled"])
	}
	if config.Values["normalized"] != true {
		t.Errorf("the sheet committed the painted values, not the stored ones: %v", config.Values)
	}
	if config.Failed {
		t.Error("an accepted write reported a failure")
	}
}

// A refusal after a confirmed change rolls back to *that* change, not to the
// values the sheet opened with: the committed snapshot follows every accepted
// write.
func TestRefusedChangeRollsBackToTheLastAccepted(t *testing.T) {
	accept := true
	a := configApp(func() bool { return accept })
	a.OpenConfig("clipboard")
	a.ChangeConfig("enabled", false)
	if a.Config().Values["enabled"] != false {
		t.Fatalf("the first change did not land: %v", a.Config().Values)
	}
	accept = false
	a.ChangeConfig("enabled", true)

	config := a.Config()
	if config.Values["enabled"] != false {
		t.Errorf("the refusal rolled back to %v, want the last accepted false", config.Values["enabled"])
	}
	if !config.Failed {
		t.Error("the refusal did not report the failure line")
	}
}

// actionApp is a launcher with one action field, whose command is the caller's
// to accept or refuse.
func actionApp(run func() bool) *App {
	a := testApp()
	a.Actions.ConfigFor = func(plugin string) *Config {
		return &Config{
			Schema: &plugincfg.Schema{Plugin: plugin, Fields: []plugincfg.Field{
				{Key: "clear_history", Kind: plugincfg.Action, Label: "Clear history",
					Confirm: "Clear", Cancel: "Cancel", Failed: "Could not clear it"},
			}},
			Values: plugincfg.Values{},
		}
	}
	a.Actions.ConfigAction = func(plugin, key string) bool { return run() }
	return a
}

// A refused action reports on its own row's line, never as a write failure:
// the old build's `ActionControl` kept its `failed` state beside the button,
// apart from the overlay's `settings.saveFailed` line (R113). Reporting it as
// a write failure made the sheet claim the *settings* could not be saved when
// only the command failed.
func TestRefusedConfigActionReportsOnItsOwnLine(t *testing.T) {
	a := actionApp(func() bool { return false })
	a.OpenConfig("clipboard")
	a.ArmConfig("clear_history")
	a.RunConfig("clear_history")

	config := a.Config()
	if config.Failed {
		t.Error("a refused action was reported as a write failure")
	}
	if config.ActionFailed != "clear_history" {
		t.Errorf("ActionFailed = %q, want the refused key", config.ActionFailed)
	}
	if config.Armed != "" {
		t.Errorf("a refused run left the button armed: %q", config.Armed)
	}

	// Asking again clears the line: the old build's `ActionControl` cleared
	// `failed` as it re-armed.
	a.ArmConfig("clear_history")
	if a.Config().ActionFailed != "" {
		t.Errorf("re-arming kept the failure line: %q", a.Config().ActionFailed)
	}
}

// An accepted action clears the line and is not a write failure either.
func TestAcceptedConfigActionClearsTheLine(t *testing.T) {
	a := actionApp(func() bool { return true })
	a.OpenConfig("clipboard")
	a.ArmConfig("clear_history")
	a.RunConfig("clear_history")

	config := a.Config()
	if config.Failed || config.ActionFailed != "" {
		t.Errorf("an accepted action reported a failure: %+v", config)
	}
}

// The two failure lines are separate state: a refused write leaves the action
// row alone, and a change does not clear an action's line.
func TestWriteAndActionFailuresAreSeparateLines(t *testing.T) {
	a := configApp(func() bool { return false })
	a.OpenConfig("clipboard")
	a.Config().ActionFailed = "clear_history"
	a.ChangeConfig("enabled", false)

	config := a.Config()
	if !config.Failed {
		t.Error("the refused write did not report its own line")
	}
	if config.ActionFailed != "clear_history" {
		t.Errorf("a write cleared the action's line: %q", config.ActionFailed)
	}
}
