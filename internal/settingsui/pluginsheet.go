package settingsui

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
)

// The generic plugin configuration sheet: the old build's R29 overlay, in the
// settings pages' own card language (R59).
//
// 「配置页面通用化：点击设置之后，弹出一个基于通用规则的配置页面，而不是一个新的
// 完全独立的页面。」 A plugin ships a `plugincfg.Schema` — an ordered list of
// fields — and this renders it: the same `SettingsCard` and `SettingsRow`
// language the pages use, the same controls, the same tokens. Nothing here
// knows a plugin's name: every label, bound and option comes from the schema,
// and every change goes back through the callbacks the launcher owns.
//
// The one field kind that is not a value is the action: a button that arms on
// the first press and runs on the second, so a plugin can offer "clear
// history" without this file growing a plugin-specific branch.

// PluginSheet is one frame of the overlay: what to draw, and what to report.
type PluginSheet struct {
	// Schema is the plugin's configuration, nil when there is none.
	Schema *plugincfg.Schema
	// Values are the values as the plugin's settings hold them.
	Values plugincfg.Values
	// Armed is the action key whose button has been pressed once, empty for
	// none.
	Armed string
	// Failed shows the one line under the fields: the last write was refused.
	Failed bool
	// ActionFailed is the action key whose command was refused, empty for
	// none: that row prints the field's own failure line, the way the old
	// build's `ActionControl` kept its own `failed` state beside the button.
	ActionFailed string
	// Change writes one field; the sheet paints the value either way and the
	// caller reports whether the plugin took it.
	Change func(key string, value any)
	// Arm and Run are the action's two presses.
	Arm func(key string)
	Run func(key string)
	// Unarm is the way back from an armed button.
	Unarm func(key string)
}

// PluginSheet draws a plugin's configuration: the title, the failure line,
// and each section's heading and card of rows. It is drawn where the result
// list would be — the launcher gives it the band and scrolls it.
func (a *App) PluginSheet(c *ui.Context, sheet PluginSheet) {
	if sheet.Schema == nil {
		return
	}
	t := c.Theme()

	// The sheet's own scroller: a schema longer than the band scrolls inside
	// it rather than growing the window past the band it was given.
	ui.Scroll(c.Key("plugin.config")).TrackScroll(&a.PluginScroll).FillWidth().
		Padding(t.Space(1), t.Space(4), t.Space(3), t.Space(4)).Children(func() {
		ui.Column(c).FillWidth().Gap(t.Space(3)).Children(func() {
			// The header: the plugin's own name at the meta register — one
			// step *below* a section heading, so the hierarchy a reader sees
			// is name (meta) > section (title) > field (label) > help, as the
			// old build's `.plugin-config__title` had it (body 600, muted).
			ui.Row(c).FillWidth().Height(t.Space(7)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, sheet.Schema.Title).FontSize(t.FontSize).FontWeight(600).
					TextColor(t.TextMuted).SingleLine()
			})
			if sheet.Failed {
				// The status line is the caption step in the warning colour
				// (`.plugin-config-action__status`): a muted line, never a
				// red banner.
				ui.Text(c, i18n.For(a.language()).Settings.SaveFailed).
					FontSize(t.FontSize - 1).TextColor(t.Warning)
			}
			for _, section := range plugincfg.Sections(sheet.Schema) {
				section := section
				a.sheetSection(c, sheet, section)
			}
		})
	})
}

// sheetSection draws one group: its heading (when it has one) and the card
// holding its fields.
func (a *App) sheetSection(c *ui.Context, sheet PluginSheet, section plugincfg.Section) {
	t := c.Theme()
	// The section is the settings pages' own: an 8u gap between the title and
	// its card, and the title at the title step (620, strong) — the old build's
	// `.plugin-config__section` / `__section-title`.
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		if section.Title != "" {
			// The group title starts on the column the row labels under it
			// start on: the card's own inset, added to the scroller's.
			ui.Text(c, section.Title).FontSize(t.FontSize+2).FontWeight(620).
				TextColor(a.tokens(c).TextStrong).Padding(0, t.Space(3.5))
		}
		a.card(c, func() {
			for _, field := range section.Fields {
				field := field
				a.sheetField(c, sheet, field)
			}
		})
	})
}

// sheetField draws one field with the control its kind names.
func (a *App) sheetField(c *ui.Context, sheet PluginSheet, field plugincfg.Field) {
	value := sheet.Values[field.Key]
	switch field.Kind {
	case plugincfg.Toggle:
		on, _ := value.(bool)
		a.checkbox(c, field.Label, field.Help, on, func(next bool) { sheet.Change(field.Key, next) })
	case plugincfg.Select:
		current, _ := value.(string)
		a.choose(c, field.Label, field.Help, optionsOf(field.Options), current,
			func(next string) { sheet.Change(field.Key, next) })
	case plugincfg.Radio:
		current, _ := value.(string)
		a.sheetRadio(c, sheet, field, current)
	case plugincfg.Checkboxes:
		chosen := stringsOf(value)
		a.sheetCheckboxes(c, sheet, field, chosen)
	case plugincfg.Slider:
		number, _ := value.(float64)
		a.sheetSlider(c, sheet, field, number)
	case plugincfg.Number:
		number, _ := value.(float64)
		a.sheetNumber(c, sheet, field, number)
	case plugincfg.Text:
		text, _ := value.(string)
		edit := text
		// The reference stacks a free-text field: it takes the row's full
		// width, not a trailing slot (`.plugin-config-text { width: 100% }`,
		// `plugin-config.css:110-113`; `STACKED_KINDS` in
		// `plugins/controls.tsx:291-296`).
		a.stackedRow(c, field.Label, field.Help, func() {
			input := a.sheetTextField(c, field.Placeholder, &edit).Label(field.Label)
			if input.Submitted() {
				sheet.Change(field.Key, edit)
			}
		})
	case plugincfg.Action:
		a.sheetAction(c, sheet, field)
	}
}

// sheetRadio is a stacked row: the label and help above, the choices under
// them, as the old build's radio group drew.
func (a *App) sheetRadio(c *ui.Context, sheet PluginSheet, field plugincfg.Field, current string) {
	labels := make([]string, len(field.Options))
	index := 0
	for i, option := range field.Options {
		labels[i] = option.Label
		if option.Value == current {
			index = i
		}
	}
	chosen := index
	changed := false
	a.stackedRow(c, field.Label, field.Help, func() {
		a.segmented(c, labels, chosen, func(i int) {
			if i >= 0 && i < len(field.Options) {
				chosen = i
				changed = true
			}
		})
	})
	if changed {
		sheet.Change(field.Key, field.Options[chosen].Value)
	}
}

// sheetCheckboxes is the same stacked shape, with a check box per option.
func (a *App) sheetCheckboxes(c *ui.Context, sheet PluginSheet, field plugincfg.Field, current []string) {
	t := c.Theme()
	on := make([]bool, len(field.Options))
	for i, option := range field.Options {
		for _, value := range current {
			if value == option.Value {
				on[i] = true
			}
		}
	}
	pressed := -1
	a.stackedRow(c, field.Label, field.Help, func() {
		ui.Row(c).FillWidth().Gap(t.Space(3)).Wrap().Children(func() {
			for i, option := range field.Options {
				i, option := i, option
				checked := on[i]
				if ui.Checkbox(c, &checked, option.Label).Changed() {
					on[i] = checked
					pressed = i
				}
			}
		})
	})
	if pressed >= 0 {
		next := make([]string, 0, len(field.Options))
		for i, option := range field.Options {
			if on[i] {
				next = append(next, option.Value)
			}
		}
		sheet.Change(field.Key, next)
	}
}

// sheetSlider is the old build's capacity row: the label and help above, the
// track and its value under them, the value quantized to the schema's step
// and trailed by the schema's unit word.
func (a *App) sheetSlider(c *ui.Context, sheet PluginSheet, field plugincfg.Field, value float64) {
	t := c.Theme()
	v := value
	changed := false
	a.stackedRow(c, field.Label, field.Help, func() {
		ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			e := ui.Slider(c, &v, field.Min, field.Max).Grow(1)
			if e.Changed() {
				changed = true
			}
			ui.Text(c, boundedText(field, v)).FontSize(t.FontSize).TextColor(t.TextMuted)
		})
	})
	if changed {
		sheet.Change(field.Key, quantize(v, field.Step))
	}
}

// sheetNumber is a trailing row: the label and help at the start, a stepper
// at the end.
func (a *App) sheetNumber(c *ui.Context, sheet PluginSheet, field plugincfg.Field, value float64) {
	t := c.Theme()
	v := value
	changed := false
	a.row(c, field.Label, field.Help, func() {
		ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
			e := ui.NumberInput(c, &v, field.Min, field.Max, field.Step).Label(field.Label).Width(90)
			if e.Changed() {
				changed = true
			}
			if field.Unit != "" {
				ui.Text(c, field.Unit).FontSize(t.FontSize).TextColor(t.TextMuted)
			}
		})
	})
	if changed {
		sheet.Change(field.Key, v)
	}
}

// sheetTextField draws the overlay's own text control (R59,
// `plugin-config.css:97-118`). It is deliberately not the framework's
// `ui.TextInput`: the reference's field is a 28u slot cut into the glass — a
// `--glass-field` fill, a `--glass-control-edge` hairline, the `--radius-sm`
// corner, the body step and 8u of side padding, with the field's hairline and
// inset shadow telling the user the value lives inside it. The input itself is
// the framework's look-free base, so its own padding, surface and border do
// not fight the slot.
func (a *App) sheetTextField(c *ui.Context, placeholder string, value *string) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	field := ui.Row(c).FillWidth().MinHeight(t.Space(7)).AlignItems(ui.Center).
		Padding(0, t.Space(2)).Radius(tokens.RadiusSM).
		Border(1, tokens.ControlEdge).Background(tokens.Field)
	field.Children(func() {
		ui.TextInputBase(c, value).Placeholder(placeholder).Grow(1)
	})
	return field
}

// sheetActionButton draws one of the overlay's action buttons (R38,
// `plugin-config.css:223-260`). It is deliberately **not** the framework's
// filled `Button`: the reference's control is a quiet hairline pill that
// spends no accent fill — a 28u height, 10u of side padding, a
// `--glass-control-edge` hairline, a `999px` corner (half the 28u height, so
// the shape is a pill), the `--glass-control` plane and the `--text-caption`
// step in `--text-primary`. The danger variant is the one the destructive
// command wears: `color-mix(in srgb, var(--text-warning) 45%, transparent)` on
// its hairline, `--text-warning` on its ink, and a transparent plane, so it
// reads as the destructive action without becoming the overlay's primary one.
// Hover lifts either face to `--glass-control-hover`.
func (a *App) sheetActionButton(c *ui.Context, label string, danger bool, run func()) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	button := ui.ButtonBase(c).
		MinHeight(t.Space(7)).
		Padding(0, t.Space(2.5)).
		Radius(t.Space(3.5))
	ink := t.Text
	edge := tokens.ControlEdge
	plane := tokens.Control
	if danger {
		ink = t.Warning
		edge = t.Warning.Alpha(0.45)
		plane = ui.Transparent
	}
	if button.Hovered() {
		plane = tokens.ControlHover
	}
	button.Border(1, edge).Background(plane)
	button.Children(func() {
		ui.Text(c, label).FontSize(tokens.Caption).TextColor(ink)
	})
	if button.Clicked() {
		run()
	}
	return button
}

// sheetAction is the two-step button: the first press arms it (the button
// becomes the confirm question, with the way back beside it), the second
// runs the command. The status line is the sheet's own failure caption.
func (a *App) sheetAction(c *ui.Context, sheet PluginSheet, field plugincfg.Field) {
	t := c.Theme()
	armed := sheet.Armed == field.Key
	// The row's own label is a hit target, as it is for every other kind: it
	// arms the button, so the whole row answers the first press.
	clicked := a.row(c, field.Label, field.Help, func() {
		// The reference's cluster: inline-flex, flex-end, a 6u gap
		// (`.plugin-config-action`, `plugin-config.css:218-224`).
		ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Wrap().Children(func() {
			if armed {
				// The armed button is the old build's danger button: the
				// same quiet hairline pill, its own warning face.
				a.sheetActionButton(c, field.Confirm, true, func() { sheet.Run(field.Key) })
				a.sheetActionButton(c, field.Cancel, false, func() { sheet.Unarm(field.Key) })
				return
			}
			if sheet.ActionFailed == field.Key && field.Failed != "" {
				// The refused run's line sits beside the button, not in the
				// sheet's write-failure slot: the old build's
				// `.plugin-config-action__status`, the field's own wording.
				ui.Text(c, field.Failed).FontSize(a.tokens(c).Caption).TextColor(t.Warning)
			}
			a.sheetActionButton(c, field.Label, true, func() { sheet.Arm(field.Key) })
		})
	})
	if clicked && !armed {
		sheet.Arm(field.Key)
	}
}

// optionsOf converts a schema's choices to the settings pages' option shape,
// so one dropdown helper serves both.
func optionsOf(options []plugincfg.Option) []i18n.Option {
	out := make([]i18n.Option, 0, len(options))
	for _, option := range options {
		out = append(out, i18n.Option{ID: option.Value, Label: option.Label})
	}
	return out
}

// boundedText is a bounded value with its unit: "300 items", "30 days".
func boundedText(field plugincfg.Field, value float64) string {
	text := strconv.Itoa(int(math.Round(value)))
	if field.Unit != "" {
		text += " " + field.Unit
	}
	return text
}

// quantize rounds a dragged value to the schema's step, so a slider that
// steps by ten lands on round numbers.
func quantize(value, step float64) float64 {
	if step <= 0 {
		return value
	}
	return math.Round(value/step) * step
}

func stringsOf(value any) []string {
	list, _ := value.([]string)
	return list
}

// language is the stored interface language, for the copy the sheet owns.
func (a *App) language() string { return a.Store.Snapshot().Language }
