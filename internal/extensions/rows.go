package extensions

import (
	"encoding/json"
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

// ParseRows reads a command's standard output as the list protocol, and
// reports whether it is a list at all. Not JSON, not an array, an item that is
// not an object, a missing id or title, an unknown icon or action type: each
// of those makes the whole output text (ok false), which is what the protocol
// promises.
//
// An empty array is a list with no rows: "nothing to say", not text.
func ParseRows(stdout string) ([]Row, bool) {
	trimmed := strings.TrimSpace(stdout)
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
