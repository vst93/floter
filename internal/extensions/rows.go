package extensions

import (
	"encoding/json"
	"strconv"
	"strings"
)

// The list protocol: a command whose standard output is a JSON array of rows
// is drawn as a list instead of as text. The rule is all-or-nothing — one
// item that does not fit and the whole output is text — so a plugin can never
// half-break its own view.

// The glyphs a row may name. The vocabulary is closed: the launcher draws its
// own icons, and a plugin must not be able to name something that does not
// exist.
var RowIcons = map[string]bool{
	"link": true, "file": true, "folder": true, "globe": true,
	"star": true, "clock": true, "text": true, "image": true, "command": true,
}

// Row is one row of a plugin's list output.
type Row struct {
	// ID is the row's identity; a page merge dedupes by it.
	ID string
	// Title is the row's main text, Subtitle its second line (empty folds the
	// row to one line).
	Title    string
	Subtitle string
	// Icon is one of RowIcons, empty for the generic one.
	Icon string
	// Group names a section; adjacent rows of one group are drawn together.
	Group string
	// Status marks a row as a note rather than a door: it is not selectable
	// and Enter does nothing on it.
	Status bool
	// Disabled marks a row that cannot be run.
	Disabled bool
	// Action is what Enter does, nil for an informational row.
	Action *RowAction
}

// RowAction is what Enter does on a row.
type RowAction struct {
	// Type is "open", "copy" or "insert".
	Type string
	// URL is the page an "open" row opens.
	URL string
	// Text is what a "copy" row copies or an "insert" row puts back in the
	// field.
	Text string
}

// Runnable reports whether Enter does something on the row.
func (r Row) Runnable() bool { return r.Action != nil && !r.Status && !r.Disabled }

// Page is a list emission's pagination state.
//
// Cursor is the plugin's opaque continuation token for the next page. The
// launcher never interprets it: it is echoed back to a self-paging plugin
// verbatim (in FLOTER_PAGE_CURSOR) so offset-based (a numeric cursor) and
// key-based (an opaque id) plugins both fit. HasMore is the one bit the
// launcher reads — it draws the footer and decides whether reaching the end of
// the list should ask for another page.
type Page struct {
	Cursor  string
	HasMore bool
}

// Emission is a parsed list emission: its rows and, when the plugin pages
// itself, the continuation block it declared.
type Emission struct {
	Rows []Row
	// Page is the plugin's own pagination block, nil for a plugin that hands
	// its whole list over in one emission (the pre-pagination protocol).
	Page *Page
}

// ParseRows reads a command's standard output as the list protocol, and
// reports whether it is a list at all. Not JSON, not an array, an item that is
// not an object, a missing id or title, an unknown icon or action type: each
// of those makes the whole output text (ok false), which is what the protocol
// promises.
//
// An empty array is a list with no rows: "nothing to say", not text.
func ParseRows(stdout string) ([]Row, bool) {
	return parseRowArray(stdout)
}

// ParseEmission reads a command's standard output as a list emission. Two
// shapes are accepted: the bare array of rows (a complete list), and the
// object form `{"output": [...], "page": {"cursor": ..., "hasMore": ...}}` a
// plugin that pages itself emits. Anything else — including an object without
// a valid `output` array — is not a list, and the whole output stays text.
func ParseEmission(stdout string) (Emission, bool) {
	trimmed := strings.TrimSpace(stdout)
	if strings.HasPrefix(trimmed, "[") {
		rows, ok := parseRowArray(trimmed)
		if !ok {
			return Emission{}, false
		}
		return Emission{Rows: rows}, true
	}
	if !strings.HasPrefix(trimmed, "{") {
		return Emission{}, false
	}
	var raw struct {
		Output json.RawMessage `json:"output"`
		Page   json.RawMessage `json:"page"`
	}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return Emission{}, false
	}
	if len(raw.Output) == 0 {
		return Emission{}, false
	}
	rows, ok := parseRowArray(string(raw.Output))
	if !ok {
		return Emission{}, false
	}
	return Emission{Rows: rows, Page: normalizePage(raw.Page)}, true
}

// parseRowArray reads an array of rows, reporting whether it is a valid list.
func parseRowArray(text string) ([]Row, bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "[") {
		return nil, false
	}
	var raw []map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, false
	}
	rows := make([]Row, 0, len(raw))
	for _, item := range raw {
		row, ok := parseRow(item)
		if !ok {
			return nil, false
		}
		rows = append(rows, row)
	}
	return rows, true
}

// normalizePage accepts only a well-formed pagination block, and drops a
// malformed one rather than guessing: a cursor that is not a string is the
// start, and `hasMore` is strictly boolean. Guessing a continuation from
// malformed data is how a list silently loops.
func normalizePage(raw json.RawMessage) *Page {
	if len(raw) == 0 {
		return nil
	}
	var wire struct {
		Cursor  json.RawMessage `json:"cursor"`
		HasMore json.RawMessage `json:"hasMore"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil
	}
	page := &Page{}
	var cursor string
	if len(wire.Cursor) > 0 && json.Unmarshal(wire.Cursor, &cursor) == nil {
		page.Cursor = cursor
	}
	var hasMore bool
	if len(wire.HasMore) > 0 && json.Unmarshal(wire.HasMore, &hasMore) == nil {
		page.HasMore = hasMore
	}
	return page
}

// The pagination window, in rows: one page is the launcher's own viewport (ten
// rows, the numbered family's size), and the first emission carries two pages
// so the content is taller than the box and there is something to scroll
// before more is asked for.
const (
	// PageSize is one page of rows: the launcher's ten-row viewport.
	PageSize = 10
	// InitialPages is how many pages the first emission holds.
	InitialPages = 2
)

// WindowRows returns the prefix of rows a window of pagesLoaded pages holds,
// and the pagination block that goes with it. The window is a prefix, not a
// slice: page boundaries only ever grow the list, so a row the user already
// sees never moves. HasMore is false once the held rows are exhausted, and
// Cursor is the row count the next page would start at.
func WindowRows(rows []Row, pagesLoaded int) ([]Row, Page) {
	if pagesLoaded < 1 {
		pagesLoaded = 1
	}
	end := pagesLoaded * PageSize
	if end > len(rows) {
		end = len(rows)
	}
	visible := rows[:end]
	hasMore := len(rows) > len(visible)
	page := Page{HasMore: hasMore}
	if hasMore {
		page.Cursor = strconv.Itoa(len(visible))
	}
	return visible, page
}

// MergeRows appends one page of rows to the rows already held, dropping a row
// whose id already appeared. Pages can legitimately overlap — offset
// pagination over a store that shifted between requests repeats a row — and a
// list that showed the same row twice would be a bug the plugin cannot see.
// The first occurrence wins, so an already-selected row keeps its position.
func MergeRows(previous, next []Row) []Row {
	seen := make(map[string]bool, len(previous))
	for _, row := range previous {
		seen[row.ID] = true
	}
	merged := append([]Row{}, previous...)
	for _, row := range next {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		merged = append(merged, row)
	}
	return merged
}

// parseRow reads one item, reporting whether it is a valid row.
func parseRow(item map[string]any) (Row, bool) {
	row := Row{}
	id, ok := stringField(item, "id")
	if !ok || id == "" {
		return Row{}, false
	}
	row.ID = id
	title, ok := stringField(item, "title")
	if !ok || title == "" {
		return Row{}, false
	}
	row.Title = title
	if subtitle, ok := stringField(item, "subtitle"); ok {
		row.Subtitle = subtitle
	}
	if icon, ok := stringField(item, "icon"); ok {
		if icon != "" && !RowIcons[icon] {
			return Row{}, false
		}
		row.Icon = icon
	}
	if group, ok := stringField(item, "group"); ok {
		row.Group = group
	}
	if kind, ok := stringField(item, "kind"); ok {
		if kind != "" && kind != "status" {
			return Row{}, false
		}
		row.Status = kind == "status"
	}
	if disabled, ok := item["disabled"].(bool); ok {
		row.Disabled = disabled
	}
	if value, ok := item["action"]; ok && value != nil {
		action, ok := parseRowAction(value)
		if !ok {
			return Row{}, false
		}
		row.Action = action
	}
	if row.Status {
		row.Disabled = true
	}
	return row, true
}

// parseRowAction reads a row's action, reporting whether it is one the
// launcher can run.
func parseRowAction(value any) (*RowAction, bool) {
	record, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	kind, ok := stringField(record, "type")
	if !ok {
		return nil, false
	}
	switch kind {
	case "open":
		url, ok := requiredString(record, "url")
		if !ok || url == "" {
			return nil, false
		}
		return &RowAction{Type: kind, URL: url}, true
	case "copy", "insert":
		text, ok := requiredString(record, "text")
		if !ok {
			return nil, false
		}
		return &RowAction{Type: kind, Text: text}, true
	default:
		return nil, false
	}
}

// requiredString reads a field that must be there and must be a string.
func requiredString(record map[string]any, key string) (string, bool) {
	value, present := record[key]
	if !present || value == nil {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return text, true
}

// stringField reads a string field; a field that is present but not a string
// is not one.
func stringField(record map[string]any, key string) (string, bool) {
	value, present := record[key]
	if !present || value == nil {
		return "", true
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return text, true
}
