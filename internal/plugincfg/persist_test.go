package plugincfg

import "testing"

// An accepted write commits the painted values and clears the failure line; a
// refused one restores the committed snapshot and reports it. This is the old
// build's R113 `configChangeOutcome`: a control may never keep showing a value
// the plugin did not store.
func TestConfigChangeOutcomeCommitsOrRollsBack(t *testing.T) {
	committed := Values{"enabled": true, "max_items": 100.0}
	next := Values{"enabled": false, "max_items": 100.0}

	values, failed := ConfigChangeOutcome(next, committed, true)
	if failed {
		t.Error("an accepted write reported a failure")
	}
	if values["enabled"] != false {
		t.Errorf("an accepted write painted %v, want the painted values", values["enabled"])
	}

	values, failed = ConfigChangeOutcome(next, committed, false)
	if !failed {
		t.Error("a refused write reported success")
	}
	if values["enabled"] != true {
		t.Errorf("a refused write kept %v, want the committed snapshot", values["enabled"])
	}
	if len(values) != len(committed) || values["max_items"] != committed["max_items"] {
		t.Errorf("rollback values = %v, want the whole committed snapshot %v", values, committed)
	}
}

// The rollback returns the committed snapshot itself, not a merge with the
// painted values: a field the refused change did not touch must not come back
// from the painted map.
func TestConfigChangeOutcomeRollsBackEveryField(t *testing.T) {
	committed := Values{"enabled": true, "max_items": 100.0, "copy_mode": "full"}
	next := Values{"enabled": false, "max_items": 250.0, "copy_mode": "result"}

	values, failed := ConfigChangeOutcome(next, committed, false)
	if !failed {
		t.Fatal("a refused write reported success")
	}
	for key, want := range committed {
		if values[key] != want {
			t.Errorf("rollback %s = %v, want %v", key, values[key], want)
		}
	}
}
