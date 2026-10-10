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
	tokens := a.tokens(c)
	inset := t.Space(4)

	// The sheet's own scroller: a schema longer than the band scrolls inside
	// it rather than growing the window past the band it was given.
	ui.Scroll(c.Key("plugin.config")).TrackScroll(&a.PluginScroll).FillWidth().
		Padding(t.Space(1), inset, t.Space(2), inset).Children(func() {
		ui.Column(c).FillWidth().Gap(t.Space(3)).Children(func() {
			// The header: the plugin's own name, at the settings pages'
			// display register.
			ui.Row(c).FillWidth().Height(t.Space(7)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, sheet.Schema.Title).FontSize(t.FontSize + 3).FontWeight(650).
					TextColor(tokens.TextStrong)
			})
			if sheet.Failed {
				ui.Text(c, i18n.For(a.language()).Settings.SaveFailed).
					FontSize(t.FontSize - 1).TextColor(t.Danger)
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
	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		if section.Title != "" {
			ui.Text(c, section.Title).FontSize(t.FontSize + 1).FontWeight(650).
				TextColor(a.tokens(c).TextStrong)
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
		a.checkbox(c, field.Label, on, func(next bool) { sheet.Change(field.Key, next) })
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
		a.text(c, field.Label, field.Help, field.Placeholder, text,
			func(next string) { sheet.Change(field.Key, next) })
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
		e := ui.Segmented(c, &chosen, labels...).Grow(1)
		if e.Changed() && chosen >= 0 && chosen < len(field.Options) {
			changed = true
		}
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

// sheetAction is the two-step button: the first press arms it (the button
// becomes the confirm question, with the way back beside it), the second
// runs the command. The status line is the sheet's own failure caption.
func (a *App) sheetAction(c *ui.Context, sheet PluginSheet, field plugincfg.Field) {
	t := c.Theme()
	armed := sheet.Armed == field.Key
	// The row's own label is a hit target, as it is for every other kind: it
	// arms the button, so the whole row answers the first press.
	clicked := a.row(c, field.Label, field.Help, func() {
		ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			if armed {
				// The armed button is the old build's danger button: the
				// same control, its own colour.
				if ui.Button(c, field.Confirm).TextColor(t.Danger).Clicked() {
					sheet.Run(field.Key)
				}
				if ui.Button(c, field.Cancel).Clicked() {
					sheet.Unarm(field.Key)
				}
				return
			}
			if ui.Button(c, field.Label).Clicked() {
				sheet.Arm(field.Key)
			}
		})
	})
	if clicked && !armed {
		sheet.Arm(field.Key)
	}
}

// stackedRow is a card row whose control wants the row's full width: the
// label and its help above, the control under them. It is the settings
// pages' range row shape, shared by the kinds that need it.
func (a *App) stackedRow(c *ui.Context, label, help string, control func()) {
	tokens := a.tokens(c)
	t := c.Theme()
	inset := t.Space(3.5)
	first := a.cardRows == 0
	a.cardRows++
	ui.Column(c).FillWidth().Children(func() {
		if !first {
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).MarginX(inset).Shrink(0)
		}
		ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(2.25), inset).Children(func() {
			ui.Text(c, label).FontSize(t.FontSize).FontWeight(580).TextColor(tokens.TextStrong)
			if help != "" {
				ui.Text(c, help).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
			control()
		})
	})
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
