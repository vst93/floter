package terminalui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// ToggleDrawer opens or closes the terminal's own appearance drawer: the bar's
// gear flips it, exactly as the reference's `terminalSettingsOpen` did.
func (a *App) ToggleDrawer() { a.Drawer = !a.Drawer }

// CloseDrawer closes the drawer, if it is open. It is the drawer's own close
// control's action.
func (a *App) CloseDrawer() { a.Drawer = false }

// set writes one appearance change through the settings store. The shell's
// store listener hands it to every running session (Refresh) and repaints, so
// the change lands on the terminal the drawer is docked to without the session
// ever being left.
func (a *App) set(mutate func(*settings.Settings)) {
	_ = a.Store.Update(mutate)
}

// drawer is the terminal's appearance strip, docked at the panel's bottom edge
// — a sibling of the canvas, never a layer over it, so opening it shrinks the
// canvas (the PTY is resized) while the frame and the session's scrollback stay
// exactly where they were. The reference's user asked for 「简洁、不繁琐」, so
// the rows are a flat, wrapping grid: no tabs, no sub-headings, no second card.
//
// P8-c 复核（§6.2 第 8 条）：面与几何全部保留（`Control` 一档 + 16u/12u 的
// padding 对 + 10u 的 gap），只有上边那条线从 `ControlEdge` 降成 §4.1 的**弱
// 发丝** `HairlineSoft`：抽屉是面板里的一个 block，不是一面独立的控件；块与块
// 之间的分工在合约里由弱发丝承担（§1 的不变量一：卡的边与分区线是同一条发丝的
// 两种宽度）。
//
// Only the axes this build can actually apply are drawn. The reference's strip
// also carried the bold mode, the wheel step, select-to-copy and the scrollbar
// switch; the framework's terminal exposes no knob for those, and a control
// that does nothing is a lie, not a feature (see
// docs/mygo-rewrite-status.md). Paste-safety used to be on that list by
// mistake: the framework's `Options.OnPaste` hook was there all along, and the
// setting was dead because this build never set it (see the paste boundary in
// terminalui.go).
func (a *App) drawer(c *ui.Context, copy i18n.Terminal) {
	t := c.Theme()
	tokens := tokensOf(a, c)
	scopy := i18n.For(a.Store.Snapshot().Language).Settings

	strip := ui.Row(c).FillWidth().Shrink(0).
		Padding(t.Space(2.5), t.Space(3)).Gap(t.Space(2.5)).AlignItems(ui.Start).
		Background(tokens.Control).
		BorderWidth(0, 0, 1, 0).BorderColor(tokens.HairlineSoft).
		Label(scopy.GroupTerminal)
	strip.Children(func() {
		// P11-at：条带从不让开浮条那条带（画布的地板，见 View）：面板比
		// 网格矮时它按自然高度排在画布之下，多出来的那几行在面板底边被
		// 窗口裁掉，而不是爬到 y=0 去压标题与关闭钮。让它们可滚需要一个
		// 会报出自己范围的滚动容器（实测本布局里 `MaxY` 恒为 0，见报告
		// §5），那是下一条，不在本轮硬凑。
		a.drawerGrid(c, t.Space(drawerCellFloor))
		// The drawer's own dismiss, in the corner a card's close button lives
		// in: the same 28u icon button the bar's gear/X is, so the module can
		// be dismissed from where it opened or from itself.
		a.barButton(c, "close", copy.SettingsClose, copy.SettingsClose, a.CloseDrawer)
	})
}

// drawerCellFloor is the reference's `minmax(230px, 1fr)` floor for a row's
// grid cell (`.terminal-settings`, `terminal.css:654-658`): 230px in the old
// build's unit is 57.5 of this build's `--u`, not the 60 (240px) the old port
// rounded it to. It is a parameter of drawerGrid so a test can hold the shelf
// it moves.
const drawerCellFloor = 57.5

// paletteCellFloor is the palette row's own cell floor. The reference's grid
// is uniform (`minmax(230px, 1fr)` for every row), but its palette row carries
// the picker beside the miniature preview, so forcing the widest row onto a
// line of its own keeps the preview from being squeezed — the same intent the
// reference's growing content gave it.
const paletteCellFloor = 95

// drawerGrid is the strip's wrapping grid and its rows, in the schema's own
// order (internal/control): the drawer carries the full appearance set, so it
// paints every axis the schema names — never a second list kept here. The
// cellFloor is the floor below which a row moves to the next line rather than
// squeezing its control.
func (a *App) drawerGrid(c *ui.Context, cellFloor float32) {
	t := c.Theme()
	tokens := tokensOf(a, c)
	s := a.Store.Snapshot()
	scopy := i18n.For(s.Language).Settings

	// Shrink(0): the strip scrolls this grid when the panel is shorter than
	// it (see the call site), so the grid keeps its natural height inside the
	// viewport instead of being squeezed into it — a squeezed grid clips the
	// last row to a few units of height (measured at 640×360: `Safe paste`
	// came back 4u tall).
	grid := ui.Row(c).FillWidth().Shrink(0).Wrap().GapX(t.Space(4.5)).GapY(t.Space(1.5))
	grid.Children(func() {
		rows := map[control.AppearanceAxis]func(){
			control.AxisFontSize: func() {
				a.drawerRow(c, cellFloor, scopy.FontSize, func() {
					value := float64(s.FontSize)
					changed := false
					if drawerRange(c, &value, float64(settings.MinFontSize), float64(settings.MaxFontSize)).
						Changed() {
						changed = true
					}
					drawerReadout(c, tokens, fmt.Sprintf("%d px", int(math.Round(value))))
					if changed {
						a.set(func(s *settings.Settings) { s.FontSize = int(math.Round(value)) })
					}
				})
			},
			control.AxisFontFamily: func() {
				a.drawerRow(c, cellFloor, scopy.FontFamily, func() {
					families := FontFamilyOptions(s.FontFamily)
					selected := s.FontFamily
					chosen := ""
					if ui.Select(c, &selected, families).Grow(1).Changed() {
						chosen = selected
					}
					if chosen != "" {
						a.set(func(s *settings.Settings) { s.FontFamily = chosen })
					}
				})
			},
			control.AxisLineHeight: func() {
				a.drawerRow(c, cellFloor, scopy.LineHeight, func() {
					value := s.TerminalLineHeight
					changed := false
					if drawerRange(c, &value, settings.MinTerminalLineHeight, settings.MaxTerminalLineHeight).
						Changed() {
						changed = true
					}
					drawerReadout(c, tokens, fmt.Sprintf("%.2f\u00d7", value))
					if changed {
						a.set(func(s *settings.Settings) { s.TerminalLineHeight = value })
					}
				})
			},
			control.AxisPadding: func() {
				a.drawerRowStacked(c, cellFloor, scopy.Padding, func() {
					control.SegmentedFill(c, tokens, optionLabels(scopy.Paddings),
						optionIndex(scopy.Paddings, s.TerminalPadding), func(i int) {
							a.set(func(s *settings.Settings) { s.TerminalPadding = scopy.Paddings[i].ID })
						})
				})
			},
			control.AxisCursorShape: func() {
				a.drawerRowStacked(c, cellFloor, scopy.CursorShape, func() {
					control.SegmentedFill(c, tokens, optionLabels(scopy.CursorShapes),
						optionIndex(scopy.CursorShapes, s.CursorShape), func(i int) {
							a.set(func(s *settings.Settings) { s.CursorShape = scopy.CursorShapes[i].ID })
						})
				})
			},
			control.AxisCursorBlink: func() {
				on := s.CursorBlink
				switched := false
				clicked := a.drawerRow(c, cellFloor, scopy.CursorBlink, func() {
					if ui.Switch(c, &on).Label(scopy.CursorBlink).Changed() {
						switched = true
					}
				})
				// The row's own label is the switch's name, so clicking it toggles
				// too — unless the switch itself was the target, in which case its
				// own report already carried the new value.
				if clicked && !switched {
					on = !on
				}
				if switched || clicked {
					a.set(func(s *settings.Settings) { s.CursorBlink = on })
				}
			},
			// The copy-on-select switch: a finished selection lands on the
			// clipboard, which is what the terminal surface has read from
			// `terminal_select_copy` all along (`internal/terminalui` →
			// `Options.CopyOnSelect` at creation, `SetCopyOnSelect` on Refresh).
			// The row is here because the setting was wired and unreachable:
			// nothing drew it, so it shipped as a knob only settings.json could
			// turn. The reference keeps it on the strip
			// (`main:src/settings/TerminalAppearance.tsx:328-338`).
			control.AxisSelectCopy: func() {
				on := s.TerminalSelectCopy
				switched := false
				clicked := a.drawerRow(c, cellFloor, scopy.SelectCopy, func() {
					if ui.Switch(c, &on).Label(scopy.SelectCopy).Changed() {
						switched = true
					}
				})
				if clicked && !switched {
					on = !on
				}
				if switched || clicked {
					a.set(func(s *settings.Settings) { s.TerminalSelectCopy = on })
				}
			},
			control.AxisPasteSafe: func() {
				on := s.TerminalPasteSafe
				switched := false
				clicked := a.drawerRow(c, cellFloor, scopy.PasteSafe, func() {
					if ui.Switch(c, &on).Label(scopy.PasteSafe).Changed() {
						switched = true
					}
				})
				if clicked && !switched {
					on = !on
				}
				if switched || clicked {
					a.set(func(s *settings.Settings) { s.TerminalPasteSafe = on })
				}
			},
			// The palette row is the wide one: the picker beside the miniature
			// terminal that shows what the choice paints: the picker keeps the
			// row's label line and the preview takes the line under it, filling the
			// cell. It used to sit *beside* the picker inside one trailing slot,
			// and the slot's floor (95u) could not hold label + picker + a fixed
			// 37u preview: at 900×600 the preview's box ran 24u past its cell and
			// the next row's label started on the preview's own face (实测
			// `Palette preview` 压 `Copy on select` 24×22u). A preview that fills
			// the cell has no width to overflow — the same form the settings page
			// draws it in (`PalettePreview` at width zero).
			control.AxisPalette: func() {
				a.drawerPalette(c, tokens, t.Space(paletteCellFloor), scopy, s)
			},
		}
		for _, axis := range control.AppearanceAxes() {
			if row, ok := rows[axis]; ok {
				row()
			}
		}
	})
}

// terminalSettingValueFont is the monospace stack the reference gave
// `.terminal-setting-control__value` (`settings.css:854-865`).
const terminalSettingValueFont = "SF Mono, Menlo, Monaco, Consolas, monospace"

// drawerRangeWidth is the reference's `.terminal-setting-range` width
// (`terminal.css:677-692`): a fixed `width: 110px` track, which in this build's
// unit is 27.5 — not the 22 (88px) the old port rounded it to.
const drawerRangeWidth = 27.5

// drawerRange is the strip's slider: the reference's fixed-width range, which
// does not size to its cell (a wrapping cell's control would otherwise grow and
// shrink with the window). The caller reads `.Changed()` for the write.
func drawerRange(c *ui.Context, value *float64, lo, hi float64) ui.Element {
	return ui.Slider(c, value, lo, hi).Width(c.Theme().Space(drawerRangeWidth))
}

// drawerReadout is the strip's live value: the reference's
// `.terminal-setting-control__value` (`settings.css:854-865`) — the body step in
// the monospace stack with `tabular-nums` at 600 — in the **neutral strong**
// ink, not the accent.
//
// P8-c 重定基线（§6.2 第 7 条 / §5）：`Accent` → `TextStrong`。这个读数是「滑轴
// 现在的值是什么」，它不回答「哪个被选中/哪个是开的」，所以 accent 在这里
// 越界。register与权重（body 步、mono、`tnum`、600）零改动——读到它的人仍然
// 看得见数位对齐，只是不再靠色相吸引注意力。
func drawerReadout(c *ui.Context, tokens theme.Tokens, text string) ui.Element {
	return ui.Text(c, text).
		Font(terminalSettingValueFont).FontSize(tokens.Body).
		TextColor(tokens.TextStrong).FontWeight(600).FontFeatures("tnum")
}

// drawerRow is one appearance row: its label at the start and its control at
// the end, the pair a cell of the strip's grid. minWidth is the cell's floor —
// the width below which the grid moves the row to the next line rather than
// squeezing the control. It reports whether the row itself was clicked, so a
// row whose label is the control's own name (the switch) can act on it, as the
// settings panel's rows do.
func (a *App) drawerRow(c *ui.Context, minWidth float32, label string, build func()) bool {
	t := c.Theme()
	tokens := tokensOf(a, c)
	row := ui.Row(c).Grow(1).MinWidth(minWidth).MinHeight(t.Space(7.5)).
		Gap(t.Space(2.5)).AlignItems(ui.Center)
	row.Children(func() {
		ui.Text(c, label).FontSize(t.FontSize).FontWeight(560).
			TextColor(tokens.TextSecondary).SingleLine().Shrink(0)
		slot := ui.Row(c).Grow(1).Gap(t.Space(2)).AlignItems(ui.Center).Justify(ui.End)
		slot.Children(build)
	})
	return row.Clicked()
}

// drawerRowStacked is drawerRow for a control that is a *choice of several*
// and therefore wider than a cell's trailing slot can ever be by guesswork: the
// label takes its own line and the control fills the one below it. The trailing
// form measures its content, and a wrapping grid hands a cell no more than the
// row's floor — so a three-way picker (or the palette's picker-plus-preview)
// ran past the cell's right edge and painted over the next row's label (实测
// 900×600：`Underline` 与 `Blinking cursor` 压同一块像素 4.7×22u，`Palette
// preview` 压 `Copy on select` 24×22u)。The stacked form has no content-driven
// width to overflow: the control is given the cell and shares it among its
// segments (`SegmentedFill` / a select that grows, `PalettePreview` at width
// zero), which is the same form the settings page draws the same choices in.
func (a *App) drawerRowStacked(c *ui.Context, minWidth float32, label string, build func()) {
	t := c.Theme()
	tokens := tokensOf(a, c)
	column := ui.Column(c).Grow(1).MinWidth(minWidth).
		Gap(t.Space(1.5)).AlignItems(ui.Start)
	column.Children(func() {
		ui.Text(c, label).FontSize(t.FontSize).FontWeight(560).
			TextColor(tokens.TextSecondary).SingleLine().Shrink(0)
		line := ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center)
		line.Children(build)
	})
}

// drawerPalette is the strip's palette row: its label and picker on the row's
// own line, the miniature terminal filling the line under them. See the call
// site for why neither the picker nor the preview may sit in a trailing slot.
func (a *App) drawerPalette(c *ui.Context, tokens theme.Tokens, minWidth float32, scopy i18n.Settings, s settings.Settings) {
	t := c.Theme()
	column := ui.Column(c).Grow(1).MinWidth(minWidth).
		Gap(t.Space(1.5)).AlignItems(ui.Stretch)
	column.Children(func() {
		chosen, shown := "", s.TerminalTheme
		line := ui.Row(c).FillWidth().Gap(t.Space(2.5)).AlignItems(ui.Center)
		line.Children(func() {
			ui.Text(c, scopy.Palette).FontSize(t.FontSize).FontWeight(560).
				TextColor(tokens.TextSecondary).SingleLine().Shrink(0)
			selected := optionLabel(scopy.Palettes, s.TerminalTheme)
			if ui.Select(c, &selected, optionLabels(scopy.Palettes)).
				Grow(1).Changed() {
				for _, option := range scopy.Palettes {
					if option.Label == selected {
						chosen = option.ID
						break
					}
				}
			}
			if chosen != "" {
				shown = chosen
			}
		})
		// The preview follows the picker in the same frame the choice is made,
		// so a click on the select repaints the block immediately.
		PalettePreview(c, tokens, shown, scopy.PalettePreview, 0)
		if chosen != "" {
			a.set(func(s *settings.Settings) { s.TerminalTheme = chosen })
		}
	})
}

// optionLabels is a copy table's choices as the labels the segmented control
// draws.
func optionLabels(options []i18n.Option) []string {
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	return labels
}

// optionLabel is the label of the choice whose id is `id`, the id itself when
// the table does not name it (a hand-edited value the picker must still show).
func optionLabel(options []i18n.Option, id string) string {
	for _, option := range options {
		if option.ID == id {
			return option.Label
		}
	}
	return id
}

// optionIndex is the position of the choice with this id, 0 when the table does
// not name it.
func optionIndex(options []i18n.Option, id string) int {
	for i, option := range options {
		if option.ID == id {
			return i
		}
	}
	return 0
}
