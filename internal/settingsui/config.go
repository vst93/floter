package settingsui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The configuration form: the settings an integration asks the host to hold,
// rendered from the schema its provider declares.
//
// The form is folded away until the user asks for it, keeps its draft while it
// is open (so a half-filled form survives a redraw), and writes through the
// shell on Save — which is where the values are validated against the schema
// and the password fields are moved into the secrets file.

// configDraft is one integration's form state.
type configDraft struct {
	values  map[string]any
	enabled []string // the multiSelect options that are ticked
}

// draft is the form state for an integration, seeded from its loaded values
// the first time it is opened.
func (a *App) draft(integration Integration) *configDraft {
	if a.configDrafts == nil {
		a.configDrafts = map[string]*configDraft{}
	}
	draft, ok := a.configDrafts[integration.ID]
	if !ok {
		values := map[string]any{}
		for _, field := range integration.Config {
			if value, ok := integration.ConfigValues[field.Key]; ok {
				values[field.Key] = value
				continue
			}
			// An unfilled field shows its default, which is what a save
			// would write for it.
			if field.Default != nil {
				values[field.Key] = field.Default
			}
		}
		draft = &configDraft{values: values}
		a.configDrafts[integration.ID] = draft
	}
	return draft
}

// configFields draws the form for one integration's configuration: the
// fields and the save button, shown while the form is open. The toggle that
// opens it is the integration row's own (see integrationRow).
func (a *App) configFields(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := a.cardTheme(c)
	if len(integration.Config) == 0 {
		return
	}
	if a.configError == nil {
		a.configError = map[string]string{}
	}
	if !a.configOpen[integration.ID] {
		return
	}
	draft := a.draft(integration)
	// The form is a `below` block of the ConfigOpen row, drawn on the card's
	// content column: it carries the row's own inset so the form's card starts
	// on the same column as the row labels above it instead of flush against
	// the card's edge (the reference's `.extension-config-form`,
	// `extensions.css:2369-2373`, is inset the same way).
	inset := a.rowInset(t)
	ui.Column(c).FillWidth().Gap(t.Space(2)).Padding(0, inset, 0, inset).Children(func() {
		// The configuration is one plane of fields: the reference's
		// `.extension-config-form` is a `display: grid` with a 12px gap and no
		// rules (`extensions.css:2369-2373`), so a field is a spaced grid row,
		// not a settings card's ruled row. Each field's own two-column grid is
		// what `configField` draws.
		a.configPlane(c, func() {
			for _, field := range integration.Config {
				a.configField(c, copy, integration, *draft, field)
			}
		})
		if a.configError[integration.ID] != "" {
			ui.Text(c, a.configError[integration.ID]).FontSize(t.FontSize - 1).TextColor(t.Danger)
		}
		// The reference's `.extension-config-form__actions` is a right-aligned
		// row (`justify-content: space-between` with the form's one affirmative
		// action at its end; `extensions.css:2375`). The Save is the *primary*
		// accent pill (`.extensions-action-button--primary`,
		// `ExtensionsPanel.tsx:3019-3021`), not the framework's flat button the
		// old port drew, and it ends the line.
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.End).Children(func() {
			save := a.extensionPrimaryButton(c, copy.ConfigSave)
			if a.Actions.SaveConfiguration == nil {
				save = save.Disabled(true)
			}
			if save.Clicked() && a.Actions.SaveConfiguration != nil {
				if err := a.Actions.SaveConfiguration(integration.ID, draft.values); err != nil {
					a.configError[integration.ID] = err.Error()
					return
				}
				a.configError[integration.ID] = copy.ConfigSaved
			}
		})
	})
}

// configPlane is the config form's own plane. The reference's
// `.extension-config-form` is a `display: grid` with a 12px gap and no rules
// (`extensions.css:2369-2373`), so its fields are spaced grid rows rather than
// a settings card's ruled rows. The plane keeps the grouped-card face — the
// resting control fill, the control edge, the card radius and `--elev-0` — and
// the extension row's own 8px inset, and its children are the two-column grids
// `configField` draws. It resets the row counter so a following nested row
// (the multiselect's tick list) does not open on a rule.
func (a *App) configPlane(c *ui.Context, fn func()) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	saved := a.cardRows
	a.cardRows = 0
	defer func() { a.cardRows = saved }()
	plane := ui.Column(c).FillWidth().Gap(a.cardSpace(t, 3)).
		Padding(a.cardSpace(t, 2)).
		Border(1, tokens.ControlEdge).
		Radius(tokens.RadiusMD).
		Background(tokens.Control).
		Clip()
	tokens.Lens(plane)
	plane.Children(fn)
}

// configFieldGrid lays one config field on the reference's
// `.extension-config-field` grid (`extensions.css:2379-2430`): a fixed 118px
// label column, the control filling the second column, and — when the field
// has one — the description under the control in that same second column
// (`small { grid-column: 2 }`). The label carries the warning `*` a required
// field wears, as the reference's `<b>` did. The 118px column and the 12px/
// 4px gaps are the panel's literal px, taken through `cardSpace`, so the grid
// keeps one geometry at every interface step.
func (a *App) configFieldGrid(c *ui.Context, field ConfigField, label string, control func() ui.Element) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	grid := ui.Grid(c).FillWidth().
		ColumnTracks(ui.Fixed(a.cardSpace(t, 29.5)), ui.Fr(1)).
		GapX(a.cardSpace(t, 3)).GapY(a.cardSpace(t, 1)).
		AlignItems(ui.Center)
	grid.Children(func() {
		name := ui.Row(c).Gap(a.cardSpace(t, 0.75)).AlignItems(ui.Center).ColumnStart(1)
		name.Children(func() {
			ui.Text(c, label).FontSize(t.FontSize).FontWeight(500).TextColor(tokens.TextSecondary).SingleLine()
			if field.Required {
				ui.Text(c, "*").FontSize(t.FontSize).FontWeight(600).TextColor(t.Warning).SingleLine()
			}
		})
		control().ColumnStart(2)
		if field.Description != "" {
			ui.Text(c, field.Description).FontSize(tokens.Caption).FontWeight(500).
				TextColor(tokens.TextTertiary).ColumnStart(2)
		}
	})
}

// configSelect is the config field's drop-down, on the grid's second column:
// `.extension-config-field select` shares the input's literal box
// (`extensions.css:2402-2417`) but fills its column rather than taking the
// settings rows' fixed width. It is `chooseSelect` without that 220px.
func (a *App) configSelect(c *ui.Context, selected *string, labels []string, label string) ui.Element {
	e := ui.Select(c, selected, labels).Label(label).FillWidth().Radius(999)
	if a.cardUnit > 0 {
		e = e.MinHeight(28).Padding(4, 8)
	}
	return a.fieldFace(c, e)
}

// configField draws one field, by its type. Every field wears the reference's
// `.extension-config-field` two-column grid (see `configFieldGrid`): a 118px
// label column and the control, with the description under the control. The
// old port drew each field as a settings row (label left, control right, the
// description under the *label*), so a form's controls did not share one
// column and the description sat on the wrong side of the field.
func (a *App) configField(c *ui.Context, copy i18n.Settings, integration Integration, draft configDraft, field ConfigField) {
	label := field.Label
	if label == "" {
		label = field.Key
	}
	kind := strings.ToLower(field.Type)

	switch kind {
	case "boolean":
		on := false
		if value, ok := draft.values[field.Key].(bool); ok {
			on = value
		}
		changed := false
		a.configFieldGrid(c, field, label, func() ui.Element {
			e := a.switchControl(c, &on).Label(label)
			if e.Changed() {
				changed = true
			}
			return e
		})
		if changed {
			draft.values[field.Key] = on
		}
	case "select":
		options := make([]i18n.Option, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, i18n.Option{ID: option, Label: option})
		}
		labels := make([]string, len(options))
		for i, option := range options {
			labels[i] = option.Label
		}
		current, _ := draft.values[field.Key].(string)
		selected := current
		chosen := ""
		a.configFieldGrid(c, field, label, func() ui.Element {
			e := a.configSelect(c, &selected, labels, label)
			if e.Changed() {
				for _, option := range options {
					if option.Label == selected {
						chosen = option.ID
						break
					}
				}
			}
			return e
		})
		if chosen != "" {
			draft.values[field.Key] = chosen
		}
	case "multiselect":
		ticked := map[string]bool{}
		if values, ok := draft.values[field.Key].([]any); ok {
			for _, value := range values {
				if text, ok := value.(string); ok {
					ticked[text] = true
				}
			}
		}
		// The reference draws the field as one native multiple `select` on the
		// grid's second column (`.extension-config-field select[multiple]`,
		// `extensions.css:2420-2423`). This toolkit has no multi-select, so the
		// tick list is its stand-in: the field's own label heads the list on the
		// label column, and one check-box row per option follows, the label at
		// the start and the box at the end. A list of tick boxes reads as the
		// list of rows it is, and it keeps the reference's two-column field grid
		// (the label column holds the caption, the options the second column).
		a.configFieldGrid(c, field, label, func() ui.Element {
			list := ui.Column(c).FillWidth().Gap(a.cardSpace(a.cardTheme(c), 0.5))
			list.Children(func() {
				for _, option := range field.Options {
					option := option
					on := ticked[option]
					boxChanged := false
					labelClicked := a.row(c, option, "", func() {
						if ui.Checkbox(c, &on, option).Label(option).Clicked() {
							boxChanged = true
						}
					})
					if labelClicked {
						// The row's own label is a hit target: clicking it ticks the
						// option, as the framework's Field does for its control.
						on = !on
						boxChanged = true
					}
					if boxChanged {
						ticked[option] = on
						draft.values[field.Key] = tickedToValues(field.Options, ticked)
					}
				}
			})
			return list
		})
	case "number":
		value := ""
		if number, ok := draft.values[field.Key].(float64); ok {
			value = strconv.FormatFloat(number, 'f', -1, 64)
		}
		edit := value
		a.configFieldGrid(c, field, label, func() ui.Element {
			e := a.extensionField(c, &edit, configFieldMetrics).Placeholder(field.Key).Label(label).FillWidth()
			if e.Submitted() {
				number, err := strconv.ParseFloat(strings.TrimSpace(edit), 64)
				if err != nil {
					a.configError[integration.ID] = fmt.Sprintf("%s: %s", label, copy.ConfigNumber)
				} else {
					draft.values[field.Key] = number
				}
			}
			return e
		})
	default: // text, password, path
		value, _ := draft.values[field.Key].(string)
		edit := value
		a.configFieldGrid(c, field, label, func() ui.Element {
			e := a.extensionField(c, &edit, configFieldMetrics).Placeholder(field.Key).Label(label).FillWidth()
			if kind == "password" {
				e = e.Password()
			}
			if e.Submitted() {
				draft.values[field.Key] = edit
			}
			return e
		})
	}
}

// tickedToValues is a multiselect's ticked options as the value a save writes,
// in the options' order.
func tickedToValues(options []string, ticked map[string]bool) []any {
	var out []any
	for _, option := range options {
		if ticked[option] {
			out = append(out, option)
		}
	}
	return out
}
