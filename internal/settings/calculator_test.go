package settings

import (
	"encoding/json"
	"testing"
)

func TestCalculatorPluginDefaultsAndRoundTrip(t *testing.T) {
	plugin := CalculatorPluginOf(Default())
	if plugin.MaxItems != DefaultCalculatorMaxItems || plugin.RetentionDays != DefaultCalculatorRetentionDays ||
		plugin.CopyMode != CalculatorCopyFull {
		t.Fatalf("defaults = %+v", plugin)
	}

	s, err := Parse([]byte(`{"calculator_plugin": {"max_items": 250, "retention_days": 7, "copy_mode": "result"}}`))
	if err != nil {
		t.Fatal(err)
	}
	plugin = CalculatorPluginOf(s)
	if plugin.MaxItems != 250 || plugin.RetentionDays != 7 || plugin.CopyMode != CalculatorCopyResult {
		t.Fatalf("parsed = %+v", plugin)
	}

	// A write keeps the keys the block carries that this build does not own.
	s, err = Parse([]byte(`{"calculator_plugin": {"max_items": 20, "futureKey": [1,2]}}`))
	if err != nil {
		t.Fatal(err)
	}
	plugin = CalculatorPluginOf(s)
	plugin.RetentionDays = 1
	s.SetCalculatorPlugin(plugin)
	encoded := mustEncode(t, s)
	var decoded struct {
		Block map[string]any `json:"calculator_plugin"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Block["futureKey"] == nil {
		t.Errorf("an unknown key inside the block was dropped: %v", decoded.Block)
	}
	if decoded.Block["retention_days"] != float64(1) || decoded.Block["max_items"] != float64(20) {
		t.Errorf("the write did not land: %v", decoded.Block)
	}

	// Unusable values fall back to the shipped ones.
	s, err = Parse([]byte(`{"calculator_plugin": {"max_items": 0, "retention_days": 5, "copy_mode": "nonsense"}}`))
	if err != nil {
		t.Fatal(err)
	}
	plugin = CalculatorPluginOf(s)
	if plugin.MaxItems != DefaultCalculatorMaxItems || plugin.RetentionDays != DefaultCalculatorRetentionDays ||
		plugin.CopyMode != CalculatorCopyFull {
		t.Fatalf("normalized = %+v", plugin)
	}

	// A hand-written block of the wrong shape is ignored, not fatal.
	broken, err := Parse([]byte(`{"calculator_plugin": 7}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := CalculatorPluginOf(broken); got != DefaultCalculatorPlugin() {
		t.Errorf("a broken block = %+v", got)
	}
}

func TestCalculatorNormalizers(t *testing.T) {
	if got := NormalizeCalculatorMaxItems(0); got != DefaultCalculatorMaxItems {
		t.Errorf("0 = %d", got)
	}
	if got := NormalizeCalculatorMaxItems(1); got != MinCalculatorMaxItems {
		t.Errorf("1 = %d", got)
	}
	if got := NormalizeCalculatorMaxItems(9000); got != MaxCalculatorMaxItems {
		t.Errorf("9000 = %d", got)
	}
	if got := NormalizeCalculatorRetentionDays(7); got != 7 {
		t.Errorf("7 = %d", got)
	}
	if got := NormalizeCalculatorRetentionDays(3); got != DefaultCalculatorRetentionDays {
		t.Errorf("3 = %d", got)
	}
	if got := NormalizeCalculatorCopyMode("result"); got != CalculatorCopyResult {
		t.Errorf("result = %q", got)
	}
	if got := NormalizeCalculatorCopyMode("x"); got != CalculatorCopyFull {
		t.Errorf("x = %q", got)
	}
}
