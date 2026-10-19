//go:build !windows

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

func TestParseEmissionReadsThePageBlock(t *testing.T) {
	// The bare array is a complete list: no page block.
	emission, ok := ParseEmission(`[{"id":"a","title":"A"}]`)
	if !ok || emission.Page != nil || len(emission.Rows) != 1 {
		t.Fatalf("bare array = %+v, %v", emission, ok)
	}

	// The object form carries the plugin's own continuation block.
	emission, ok = ParseEmission(`{
	  "output": [{"id":"a","title":"A"}],
	  "page": {"cursor": "opaque-token", "hasMore": true}
	}`)
	if !ok {
		t.Fatal("the object form was read as text")
	}
	if len(emission.Rows) != 1 {
		t.Fatalf("rows = %+v", emission.Rows)
	}
	if emission.Page == nil || emission.Page.Cursor != "opaque-token" || !emission.Page.HasMore {
		t.Fatalf("page = %+v", emission.Page)
	}

	// A malformed block is dropped, not guessed: the cursor is the start and
	// hasMore is strictly boolean.
	emission, ok = ParseEmission(`{"output": [], "page": {"cursor": 7, "hasMore": "yes"}}`)
	if !ok || emission.Page == nil || emission.Page.Cursor != "" || emission.Page.HasMore {
		t.Fatalf("malformed page = %+v, %v", emission, ok)
	}

	// An object without an output array is not a list at all.
	for _, stdout := range []string{
		`{"not":"a list"}`,
		`{"output": "text"}`,
		`{"output": [{"id":"a"}]}`,
		`{"output": [{"id":"a","title":"A","icon":"rocket"}]}`,
	} {
		if emission, ok := ParseEmission(stdout); ok {
			t.Errorf("%s was accepted: %+v", stdout, emission)
		}
	}
}

func TestWindowRowsIsAGrowingPrefix(t *testing.T) {
	rows := make([]Row, 25)
	for i := range rows {
		rows[i] = Row{ID: string(rune('a' + i)), Title: "row"}
	}

	first, page := WindowRows(rows, InitialPages)
	if len(first) != InitialPages*PageSize {
		t.Fatalf("the first window is %d rows", len(first))
	}
	if !page.HasMore || page.Cursor != "20" {
		t.Fatalf("the first page block = %+v", page)
	}

	second, page := WindowRows(rows, InitialPages+1)
	if len(second) != len(rows) {
		t.Fatalf("the second window is %d rows", len(second))
	}
	if page.HasMore || page.Cursor != "" {
		t.Fatalf("the last page block = %+v", page)
	}
	// The prefix is stable: a row already shown never moves.
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("row %d moved: %q -> %q", i, first[i].ID, second[i].ID)
		}
	}

	// A list that fits the first window is complete, with no block.
	short := rows[:PageSize]
	visible, page := WindowRows(short, InitialPages)
	if len(visible) != len(short) || page.HasMore {
		t.Fatalf("a short list paged: %d, %+v", len(visible), page)
	}
}

func TestMergeRowsDedupesByID(t *testing.T) {
	merged := MergeRows(
		[]Row{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}},
		[]Row{{ID: "b", Title: "B again"}, {ID: "c", Title: "C"}},
	)
	if len(merged) != 3 {
		t.Fatalf("merged = %+v", merged)
	}
	// The first occurrence keeps its position and its content.
	if merged[1].ID != "b" || merged[1].Title != "B" {
		t.Errorf("the repeated row won: %+v", merged[1])
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
