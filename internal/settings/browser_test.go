package settings

import (
	"encoding/json"
	"testing"
)

func TestBrowserPluginDefaultsAndRoundTrip(t *testing.T) {
	// A file with no block gets the shipped behaviour.
	plugin := BrowserPluginOf(Default())
	if !plugin.Enabled || plugin.Target != DefaultBrowserTarget ||
		plugin.HistoryDays != DefaultBrowserHistoryDays || plugin.CDPPort != DefaultCDPPort ||
		plugin.SortOrder != DefaultBrowserSortOrder || plugin.SearchField != DefaultBrowserSearchField {
		t.Fatalf("defaults = %+v", plugin)
	}

	// The block the old build wrote is read field for field.
	s, err := Parse([]byte(`{
		"browser_plugin": {
			"enabled": false,
			"target": "brave",
			"custom_base_dir": "  /tmp/profiles  ",
			"history_days": 7,
			"cdp_enabled": true,
			"cdp_port": 9333,
			"sort_order": "visits",
			"search_fields": "url"
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	plugin = BrowserPluginOf(s)
	if plugin.Enabled || plugin.Target != "brave" || plugin.CustomBaseDir != "/tmp/profiles" ||
		plugin.HistoryDays != 7 || !plugin.CDPEnabled || plugin.CDPPort != 9333 ||
		plugin.SortOrder != "visits" || plugin.SearchField != "url" {
		t.Fatalf("parsed = %+v", plugin)
	}

	// A write keeps every key the block carries that this build does not own.
	s, err = Parse([]byte(`{"browser_plugin": {"enabled": true, "futureKey": {"a": 1}, "target": "chrome"}}`))
	if err != nil {
		t.Fatal(err)
	}
	plugin = BrowserPluginOf(s)
	plugin.Target = "edge"
	plugin.HistoryDays = 90
	s.SetBrowserPlugin(plugin)

	encoded, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		BrowserPlugin map[string]any `json:"browser_plugin"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.BrowserPlugin["futureKey"] == nil {
		t.Errorf("an unknown key inside the block was dropped: %v", decoded.BrowserPlugin)
	}
	if decoded.BrowserPlugin["target"] != "edge" || decoded.BrowserPlugin["history_days"] != float64(90) {
		t.Errorf("the write did not land: %v", decoded.BrowserPlugin)
	}
	if got := BrowserPluginOf(mustParse(t, encoded)).Target; got != "edge" {
		t.Errorf("round trip target = %q", got)
	}

	// An empty custom directory is written as null, as the old build's
	// Option<String> did, and reads back as none.
	plugin.CustomBaseDir = ""
	s.SetBrowserPlugin(plugin)
	encoded, err = Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if value, ok := decoded.BrowserPlugin["custom_base_dir"]; !ok || value != nil {
		t.Errorf("custom_base_dir = %v (present %v)", value, ok)
	}
}

func TestBrowserPluginNormalization(t *testing.T) {
	// Unusable values fall back to the shipped ones rather than failing.
	s, err := Parse([]byte(`{
		"browser_plugin": {
			"target": "   ",
			"history_days": -5,
			"cdp_port": 0,
			"sort_order": "NONSENSE",
			"search_fields": "Title"
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	plugin := BrowserPluginOf(s)
	if plugin.Target != "auto" || plugin.HistoryDays != DefaultBrowserHistoryDays ||
		plugin.CDPPort != DefaultCDPPort || plugin.SortOrder != "relevance" || plugin.SearchField != "title" {
		t.Fatalf("normalized = %+v", plugin)
	}

	// A history window past the ceiling is capped, and a negative one floors
	// at zero ("all of it").
	if got := NormalizeCDPPort(70000); got != DefaultCDPPort {
		t.Errorf("port = %d", got)
	}
	plugin.HistoryDays = 99999
	s.SetBrowserPlugin(plugin)
	if got := BrowserPluginOf(s).HistoryDays; got != MaxBrowserHistoryDays {
		t.Errorf("history days = %d, want the %d cap", got, MaxBrowserHistoryDays)
	}
	plugin.HistoryDays = -1
	s.SetBrowserPlugin(plugin)
	if got := BrowserPluginOf(s).HistoryDays; got != 0 {
		t.Errorf("history days = %d, want the floor", got)
	}

	// A hand-written block of the wrong shape is ignored, not fatal.
	broken, err := Parse([]byte(`{"browser_plugin": "nonsense"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := BrowserPluginOf(broken); got != DefaultBrowserPlugin() {
		t.Errorf("a broken block = %+v", got)
	}
}

func TestClipboardSettings(t *testing.T) {
	state := ClipboardOf(Default())
	if !state.Enabled || state.MaxItems != DefaultClipboardMaxItems {
		t.Fatalf("defaults = %+v", state)
	}

	s, err := Parse([]byte(`{"clipboard_history_enabled": false, "clipboard_history_max_items": 120}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := ClipboardOf(s); got.Enabled || got.MaxItems != 120 {
		t.Errorf("parsed = %+v", got)
	}

	// The capacity is clamped into the band on read and on write.
	s, err = Parse([]byte(`{"clipboard_history_max_items": 100000}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := ClipboardOf(s).MaxItems; got != MaxClipboardMaxItems {
		t.Errorf("a huge capacity = %d", got)
	}
	state = ClipboardSettings{Enabled: true, MaxItems: 1}
	s.SetClipboard(state)
	if got := ClipboardOf(s).MaxItems; got != MinClipboardMaxItems {
		t.Errorf("a tiny capacity = %d", got)
	}
}

func mustParse(t *testing.T, data []byte) Settings {
	t.Helper()
	s, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
