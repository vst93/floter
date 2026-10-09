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

// configSection draws the form for one integration's configuration.
func (a *App) configSection(c *ui.Context, copy i18n.Settings, integration Integration) {
	t := c.Theme()
	if len(integration.Config) == 0 {
		return
	}
	if a.configOpen == nil {
		a.configOpen = map[string]bool{}
	}
	if a.configError == nil {
		a.configError = map[string]string{}
	}
	if ui.Button(c, copy.ConfigOpen).Clicked() {
		a.configOpen[integration.ID] = !a.configOpen[integration.ID]
		a.configError[integration.ID] = ""
	}
	if !a.configOpen[integration.ID] {
		return
	}
	draft := a.draft(integration)
	ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(0, t.Space(2), 0, 0).Children(func() {
		for _, field := range integration.Config {
			a.configField(c, copy, integration, *draft, field)
		}
		if a.configError[integration.ID] != "" {
			ui.Text(c, a.configError[integration.ID]).FontSize(t.FontSize - 1).TextColor(t.Danger)
		}
		if ui.Button(c, copy.ConfigSave).Clicked() && a.Actions.SaveConfiguration != nil {
			if err := a.Actions.SaveConfiguration(integration.ID, draft.values); err != nil {
				a.configError[integration.ID] = err.Error()
				return
			}
			a.configError[integration.ID] = copy.ConfigSaved
		}
	})
}

// configField draws one field, by its type.
func (a *App) configField(c *ui.Context, copy i18n.Settings, integration Integration, draft configDraft, field ConfigField) {
	t := c.Theme()
	label := field.Label
	if label == "" {
		label = field.Key
	}
	description := field.Description
	if field.Required {
		description = strings.TrimSpace(description + "  ·  " + copy.ConfigRequired)
	}
	kind := strings.ToLower(field.Type)

	switch kind {
	case "boolean":
		value, _ := draft.values[field.Key].(bool)
		a.checkbox(c, label, value, func(on bool) { draft.values[field.Key] = on })
	case "select":
		options := make([]i18n.Option, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, i18n.Option{ID: option, Label: option})
		}
		current, _ := draft.values[field.Key].(string)
		a.choose(c, label, description, options, current, func(id string) {
			draft.values[field.Key] = id
		})
	case "multiselect":
		ticked := map[string]bool{}
		if values, ok := draft.values[field.Key].([]any); ok {
			for _, value := range values {
				if text, ok := value.(string); ok {
					ticked[text] = true
				}
			}
		}
		ui.Column(c).FillWidth().Children(func() {
			ui.Text(c, label).FontSize(t.FontSize)
			for _, option := range field.Options {
				on := ticked[option]
				changed := false
				if ui.Checkbox(c, &on, option).Changed() {
					changed = true
				}
				if changed {
					ticked[option] = on
					draft.values[field.Key] = tickedToValues(field.Options, ticked)
				}
			}
		})
	case "number":
		value := ""
		if number, ok := draft.values[field.Key].(float64); ok {
			value = strconv.FormatFloat(number, 'f', -1, 64)
		}
		a.text(c, label, description, "", value, func(text string) {
			number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
			if err != nil {
				a.configError[integration.ID] = fmt.Sprintf("%s: %s", label, copy.ConfigNumber)
				return
			}
			draft.values[field.Key] = number
		})
	default: // text, password, path
		value, _ := draft.values[field.Key].(string)
		edit := value
		field_ := ui.Field(c, label, func() {
			input := ui.TextInput(c, &edit).Placeholder(field.Key).Label(label).Width(220)
			if kind == "password" {
				input = input.Password()
			}
			if input.Submitted() {
				draft.values[field.Key] = edit
			}
		})
		if description != "" {
			field_.Description(description)
		}
	}
	_ = t
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
