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
