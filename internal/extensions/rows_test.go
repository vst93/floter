package extensions

import "testing"

func TestParseRowsAcceptsTheProtocol(t *testing.T) {
	rows, ok := ParseRows(`[
	  {"id":"a","title":"First","subtitle":"one line","icon":"star","group":"Favorites",
	   "action":{"type":"open","url":"https://example.com"}},
	  {"id":"b","title":"Plain information row"},
	  {"id":"c","title":"Copy me","action":{"type":"copy","text":"copied"}},
	  {"id":"d","title":"Insert me","action":{"type":"insert","text":"query"}},
	  {"id":"e","title":"No matches","kind":"status","disabled":true}
	]`)
	if !ok {
		t.Fatal("a valid list was read as text")
	}
	if len(rows) != 5 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Icon != "star" || rows[0].Group != "Favorites" || !rows[0].Runnable() {
		t.Errorf("first row = %+v", rows[0])
	}
	if rows[0].Action.URL != "https://example.com" || rows[0].Action.Type != "open" {
		t.Errorf("first action = %+v", rows[0].Action)
	}
	if rows[1].Runnable() {
		t.Error("a row with no action is runnable")
	}
	if rows[2].Action.Text != "copied" || rows[3].Action.Text != "query" {
		t.Errorf("copy/insert = %+v / %+v", rows[2].Action, rows[3].Action)
	}
	if !rows[4].Status || rows[4].Runnable() {
		t.Errorf("status row = %+v", rows[4])
	}

	// An empty array is a list with nothing to say, not text.
	rows, ok = ParseRows("[]")
	if !ok || len(rows) != 0 {
		t.Errorf("empty array = %+v, %v", rows, ok)
	}
	// Surrounding whitespace is fine.
	if _, ok := ParseRows("\n  [ {\"id\":\"a\",\"title\":\"A\"} ]  \n"); !ok {
		t.Error("whitespace broke the parse")
	}
	// Unknown extra fields are ignored, as the protocol promises.
	if _, ok := ParseRows(`[{"id":"a","title":"A","future":42}]`); !ok {
		t.Error("an unknown field invalidated the list")
	}
}

func TestParseRowsFallsBackToText(t *testing.T) {
	cases := map[string]string{
		"not json":                 "plain text",
		"an object":                `{"id":"a"}`,
		"a number":                 `42`,
		"a missing id":             `[{"title":"A"}]`,
		"an empty id":              `[{"id":"","title":"A"}]`,
		"a missing title":          `[{"id":"a"}]`,
		"an empty title":           `[{"id":"a","title":""}]`,
		"a non-string title":       `[{"id":"a","title":7}]`,
		"an unknown icon":          `[{"id":"a","title":"A","icon":"rocket"}]`,
		"an unknown kind":          `[{"id":"a","title":"A","kind":"error"}]`,
		"an unknown action":        `[{"id":"a","title":"A","action":{"type":"run"}}]`,
		"an open without a url":    `[{"id":"a","title":"A","action":{"type":"open"}}]`,
		"a copy without text":      `[{"id":"a","title":"A","action":{"type":"copy"}}]`,
		"an action that is a list": `[{"id":"a","title":"A","action":[]}]`,
		"one bad row":              `[{"id":"a","title":"A"},{"id":"b"}]`,
	}
	for name, stdout := range cases {
		if rows, ok := ParseRows(stdout); ok {
			t.Errorf("%s was accepted as a list: %+v", name, rows)
		}
	}
}
