// Package plugincfg is the declarative configuration of a built-in plugin:
// an ordered list of fields, each naming a control kind, a label and the
// bounds or options that control needs.
//
// The old app's R29 drew the same thing: a plugin ships a *description*
// rather than a settings page of its own, and one generic overlay renders it
// inside the launcher window — 「配置页面通用化：点击设置之后，弹出一个基于通用
// 规则的配置页面，而不是一个新的完全独立的页面」. The schema *is* the contract:
// this package is pure (no ui, no shell), so the overlay, the settings pages
// and the tests all read one description of what a plugin may be configured
// with.
package plugincfg

import (
	"math"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// Kind is a field's control kind: the shapes `plugins/controls.tsx` rendered.
type Kind int

const (
	// Toggle is a switch.
	Toggle Kind = iota
	// Select is a dropdown of options.
	Select
	// Radio is a row of options, one chosen.
	Radio
	// Checkboxes is a row of options, any number chosen.
	Checkboxes
	// Slider is a bounded number, dragged.
	Slider
	// Number is a bounded number, typed or stepped.
	Number
	// Text is free text.
	Text
	// Action is not a value: a button that arms on the first press and runs
	// its command on the second.
	Action
)

// Option is one choice of a Select, Radio or Checkboxes field.
type Option struct {
	// Value is what the plugin stores.
	Value string
	// Label is what the control shows.
	Label string
}

// Field is one field of a plugin's configuration.
type Field struct {
	// Key is the field's identity in the value map and the name a change
	// carries.
	Key string
	// Kind is the control to draw.
	Kind Kind
	// Label names the field; Help is the line under it.
	Label string
	Help  string
	// Placeholder is a Text field's placeholder, empty for none.
	Placeholder string
	// Options are the choices of a Select, Radio or Checkboxes field.
	Options []Option
	// Min, Max and Step bound a Slider or a Number.
	Min, Max, Step float64
	// Unit is the word after a bounded value ("items", "days"), empty for
	// none.
	Unit string
	// Section is the heading the field sits under. Consecutive fields with
	// the same Section are one card; "" is the untitled first group.
	Section string
	// Confirm, Cancel and Failed are an Action field's wording: the question
	// the armed button asks, the way back, and what a failed run reports.
	Confirm string
	Cancel  string
	Failed  string
}

// Schema is one plugin's whole configuration.
type Schema struct {
	// Plugin is the plugin's id, which the overlay reports a change with.
	Plugin string
	// Title is the overlay's heading: the plugin's own name.
	Title string
	// Fields is the ordered field list.
	Fields []Field
}

// Section is a rendered group: a heading (empty for the untitled first group)
// and the fields under it.
type Section struct {
	Title  string
	Fields []Field
}

// Sections splits a schema's ordered fields into the groups the overlay
// draws: a new group starts at the first field and whenever Section changes.
// Order is schema order, so a group is always contiguous and grouping can
// never reorder a plugin's fields.
func Sections(s *Schema) []Section {
	if s == nil {
		return nil
	}
	var out []Section
	for _, f := range s.Fields {
		last := len(out) - 1
		if last >= 0 && out[last].Title == f.Section {
			out[last].Fields = append(out[last].Fields, f)
			continue
		}
		out = append(out, Section{Title: f.Section, Fields: []Field{f}})
	}
	return out
}

// Field returns the field with a key.
func (s *Schema) Field(key string) (Field, bool) {
	if s == nil {
		return Field{}, false
	}
	for _, f := range s.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// Values is one plugin's configuration as the overlay holds it: a field key
// to a bool (a toggle), a string (a select, a radio, text), a float64 (a
// slider, a number), a []string (checkboxes) or nil (an action, or a field
// with no value yet).
type Values map[string]any

// Clone copies the values, so an edit never writes through a snapshot the
// caller still holds.
func (v Values) Clone() Values {
	out := make(Values, len(v))
	for k, value := range v {
		if list, ok := value.([]string); ok {
			value = append([]string(nil), list...)
		}
		out[k] = value
	}
	return out
}

// Apply returns the values with one field changed. A key the schema does not
// declare, or a value of the wrong shape for its kind, leaves the values
// untouched: the schema is the only thing that may name a field, and a
// control cannot write a value its own kind could not have read.
func (s *Schema) Apply(v Values, key string, raw any) Values {
	f, ok := s.Field(key)
	if !ok || f.Kind == Action {
		return v.Clone()
	}
	value, ok := coerce(f, raw)
	if !ok {
		return v.Clone()
	}
	out := v.Clone()
	out[key] = value
	return out
}

// coerce fits a raw control value to its field's kind.
func coerce(f Field, raw any) (any, bool) {
	switch f.Kind {
	case Toggle:
		on, ok := raw.(bool)
		return on, ok
	case Select, Radio, Text:
		text, ok := raw.(string)
		if !ok {
			return nil, false
		}
		if f.Kind != Text && !hasOption(f.Options, text) {
			return nil, false
		}
		return text, true
	case Slider, Number:
		number, ok := asFloat(raw)
		if !ok {
			return nil, false
		}
		return clamp(number, f.Min, f.Max), true
	case Checkboxes:
		list, ok := raw.([]string)
		if !ok {
			return nil, false
		}
		kept := make([]string, 0, len(list))
		for _, item := range list {
			if hasOption(f.Options, item) {
				kept = append(kept, item)
			}
		}
		return kept, true
	}
	return nil, false
}

func hasOption(options []Option, value string) bool {
	for _, o := range options {
		if o.Value == value {
			return true
		}
	}
	return false
}

func asFloat(raw any) (float64, bool) {
	switch n := raw.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

func clamp(n, lo, hi float64) float64 {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// The overlay's own chrome, in the old app's units (`--u`), from
// `result-budget.ts`: the header line, the fields block's padding pair, a
// section heading, and the gap before the next section's card.
const (
	headerUnits         = 28
	fieldsPaddingUnits  = 16
	sectionHeadingUnits = 25
	sectionGapUnits     = 12
)

// rowUnits is the height one field's row draws, in units: the worst case for
// the kind, as the old build budgeted it. A trailing row (a switch, a select,
// a number, an action) is the label and help stack plus its padding pair; a
// stacked row (the kinds whose control wants the row's full width) adds the
// control's own box. The drawn rows are given the same floor, so the budget
// is a height the window can hold rather than one the row can be clipped by.
func rowUnits(kind Kind) float64 {
	switch kind {
	case Text:
		return 85
	case Radio:
		return 91
	case Slider, Checkboxes:
		return 73
	default:
		return 49
	}
}

// ContentUnits is the overlay's content height in the old app's units: its
// header, its padding pair, each section's heading and gap, and every field's
// row. It is the part *below* the launcher's field row, which the window's
// own chrome already charges.
func (s *Schema) ContentUnits() float64 {
	if s == nil || len(s.Fields) == 0 {
		return 0
	}
	sections := Sections(s)
	total := float64(headerUnits + fieldsPaddingUnits)
	for i, section := range sections {
		if i > 0 {
			total += sectionGapUnits
		}
		if section.Title != "" {
			total += sectionHeadingUnits
		}
		for _, f := range section.Fields {
			total += rowUnits(f.Kind)
		}
	}
	return total
}

// Height is the overlay's content height in pixels at an interface step, plus
// the one pixel each card's edges add, never above the display's cap.
func (s *Schema) Height(scale float64, cap float64) float64 {
	units := s.ContentUnits()
	if units <= 0 {
		return 0
	}
	// Each section's card has a 1px edge top and bottom.
	chrome := 2.0 * float64(len(Sections(s)))
	height := math.Ceil(units*scale) + chrome
	if cap > 0 && height > cap {
		height = cap
	}
	return height
}

// Clipboard is the clipboard plugin's configuration: its switch, the capacity
// slider the old build bounded to ten steps, and the one destructive action
// it offers.
func Clipboard(copy i18n.Settings, state settings.ClipboardSettings) (*Schema, Values) {
	schema := &Schema{
		Plugin: settings.CustomPluginClipboard,
		Title:  copy.ClipboardPlugin,
		Fields: []Field{
			{
				Key: "enabled", Kind: Toggle,
				Label: copy.ClipboardEnabled, Help: copy.ClipboardPluginHint,
			},
			{
				Key: "max_items", Kind: Slider,
				Label: copy.ClipboardMaxItems, Help: copy.ClipboardMaxItemsHint,
				Min: float64(settings.MinClipboardMaxItems), Max: float64(settings.MaxClipboardMaxItems),
				Step: 10, Unit: copy.UnitItems,
			},
			{
				Key: "clear_history", Kind: Action,
				Label: copy.ClipboardClear, Help: copy.ClipboardClearHint,
				Confirm: copy.ClipboardClearButton, Cancel: copy.ClipboardClearCancel,
				Failed: copy.ClipboardClearFailed,
			},
		},
	}
	return schema, Values{
		"enabled":   state.Enabled,
		"max_items": float64(state.MaxItems),
	}
}

// Calculator is the calculator plugin's configuration.
func Calculator(copy i18n.Settings, state settings.CalculatorPlugin) (*Schema, Values) {
	options := make([]Option, 0, len(copy.CalculatorCopyModes))
	for _, o := range copy.CalculatorCopyModes {
		options = append(options, Option{Value: o.ID, Label: o.Label})
	}
	schema := &Schema{
		Plugin: settings.CustomPluginCalculator,
		Title:  copy.CalculatorPlugin,
		Fields: []Field{
			{
				Key: "max_items", Kind: Slider,
				Label: copy.CalculatorMaxItems, Help: copy.CalculatorMaxItemsHint,
				Min: float64(settings.MinCalculatorMaxItems), Max: float64(settings.MaxCalculatorMaxItems),
				Step: 10, Unit: copy.UnitItems,
			},
			{
				Key: "retention_days", Kind: Select,
				Label: copy.CalculatorRetention, Help: copy.CalculatorRetentionHint,
				Options: retentionOptions(copy),
			},
			{
				Key: "copy_mode", Kind: Radio,
				Label: copy.CalculatorCopyMode, Help: copy.CalculatorCopyModeHint,
				Options: options,
			},
			{
				Key: "clear_history", Kind: Action,
				Label: copy.ClipboardClear, Help: copy.ClipboardClearHint,
				Confirm: copy.ClipboardClearButton, Cancel: copy.ClipboardClearCancel,
				Failed: copy.CalculatorClearFailed,
			},
		},
	}
	return schema, Values{
		"max_items":      float64(state.MaxItems),
		"retention_days": itoa(state.RetentionDays),
		"copy_mode":      state.CopyMode,
	}
}

// retentionOptions is the age window's vocabulary, in the order the old build
// listed it (0 first: "never" is the widest choice, not the narrowest).
func retentionOptions(copy i18n.Settings) []Option {
	out := make([]Option, 0, len(settings.CalculatorRetentionDays))
	for _, days := range settings.CalculatorRetentionDays {
		label := copy.CalculatorRetentionNever
		if days > 0 {
			label = copy.CalculatorRetentionDays(days)
		}
		out = append(out, Option{Value: itoa(days), Label: label})
	}
	return out
}

// Browser is the browser plugin's configuration, grouped as the old build
// grouped it: the switch, the data source, what the search matches, and the
// debug port that reads open tabs. `targets` are the discovered browsers, in
// discovery order; "automatic" is always first, since it stays valid after a
// browser is uninstalled.
func Browser(copy i18n.Settings, state settings.BrowserPlugin, targets []i18n.Option) (*Schema, Values) {
	targetOptions := []Option{{Value: "auto", Label: copy.BrowserAuto}}
	for _, target := range targets {
		targetOptions = append(targetOptions, Option{Value: target.ID, Label: target.Label})
	}
	sortOptions := optionsOf(copy.BrowserSortOrders)
	fieldOptions := optionsOf(copy.BrowserSearchFields)
	schema := &Schema{
		Plugin: settings.CustomPluginBrowser,
		Title:  copy.BrowserPlugin,
		Fields: []Field{
			{
				Key: "enabled", Kind: Toggle,
				Label: copy.BrowserEnabled, Help: copy.BrowserPluginHint,
				Section: copy.SectionGeneral,
			},
			{
				Key: "target", Kind: Select,
				Label: copy.BrowserTarget, Help: copy.BrowserTargetHint,
				Options: targetOptions, Section: copy.BrowserSectionData,
			},
			{
				Key: "custom_base_dir", Kind: Text,
				Label: copy.BrowserCustomDir, Help: copy.BrowserCustomDirHint,
				Placeholder: copy.BrowserCustomDirHint,
				Section:     copy.BrowserSectionData,
			},
			{
				Key: "history_days", Kind: Number,
				Label: copy.BrowserHistoryDays, Help: copy.BrowserHistoryDaysHint,
				Min: 0, Max: settings.MaxBrowserHistoryDays, Step: 1,
				Unit: copy.UnitDays, Section: copy.BrowserSectionSearch,
			},
			{
				Key: "sort_order", Kind: Radio,
				Label: copy.BrowserSort, Help: copy.BrowserSortHint,
				Options: sortOptions, Section: copy.BrowserSectionSearch,
			},
			{
				Key: "search_fields", Kind: Radio,
				Label: copy.BrowserSearchField, Help: copy.BrowserSearchFieldHint,
				Options: fieldOptions, Section: copy.BrowserSectionSearch,
			},
			{
				Key: "cdp_enabled", Kind: Toggle,
				Label: copy.BrowserCDPEnabled, Help: copy.BrowserCDPEnabledHint,
				Section: copy.BrowserSectionTabs,
			},
			{
				Key: "cdp_port", Kind: Number,
				Label: copy.BrowserCDPPort, Help: copy.BrowserCDPPortHint,
				Min: 1, Max: 65535, Step: 1,
				Section: copy.BrowserSectionTabs,
			},
		},
	}
	return schema, Values{
		"enabled":         state.Enabled,
		"target":          state.Target,
		"custom_base_dir": state.CustomBaseDir,
		"history_days":    float64(state.HistoryDays),
		"sort_order":      state.SortOrder,
		"search_fields":   state.SearchField,
		"cdp_enabled":     state.CDPEnabled,
		"cdp_port":        float64(state.CDPPort),
	}
}

func optionsOf(options []i18n.Option) []Option {
	out := make([]Option, 0, len(options))
	for _, o := range options {
		out = append(out, Option{Value: o.ID, Label: o.Label})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
