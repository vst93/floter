package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/theme"
)

// textInput is the settings surface's one text field: the framework's field at
// the surface's own `--u` metric, wearing the surface's one field face (see
// `fieldFace`).
//
// The reference writes a text field's material as `--glass-field` under the
// `--glass-field-shadow` inset stack — `.settings-select`, `.residency-control
// __input` (`settings.css:919-940,964-977`), `.extension-config-field input,
// select` (`extensions.css:2402-2417`), `.extension-custom-form input`
// (`extensions.css:1770-1781`) all say it — and the old port carried that stack
// verbatim. P8-c 收掉了它（见 `fieldFace`）：`--glass-field-shadow` 在浅色下的
// 白边是 0.85，与合约 §1 的 0.04 光泽上限互斥，所以字段和弹层字段槽一样走
// `Control` + `ControlEdge` 一档。
//
// The `↑`/`↓` rule is the settings surface's own: ↑/↓ never walk the sidebar
// while the keyboard is in a text field, whether the field is single- or
// multi-line — the reference's `isArrowKeyEditableTarget`
// (src/settings-nav.ts:24-38) treats every INPUT and TEXTAREA alike.
//
// mygo's editor only claims ↑/↓ for a multiline field (`editor.wants`,
// ui/editor_navigation.go:46-47: `return ed.multiline || m != 0`), so a
// single-line TextInput lets the surface's window-level shortcut take them.
// The field therefore claims them itself: an element-scoped shortcut sits in
// the focus chain, and `routeKeys` (ui/input.go:730) gives a key to the
// innermost element handling it, so a focused field wins over the window.
func (a *App) textInput(c *ui.Context, value *string) ui.Element {
	return a.textInputLit(c, value, true)
}

// textInputLit is `textInput` with the field's lit edge under the caller's
// control: `lit` is false when the field is locked, and the material then goes
// out with the state (see `fieldFace`). The keyboard model is untouched — the
// two shortcuts and the user's chord below are the same in both arms, so a
// locked field and a resting field differ in exactly one line of ink.
func (a *App) textInputLit(c *ui.Context, value *string, lit bool) ui.Element {
	e := a.fieldFace(c, ui.TextInput(c, value), lit)
	e.Shortcut(0, ui.KeyUp)
	e.Shortcut(0, ui.KeyDown)
	// The user's own chords are the field's, before the editor's: the
	// framework asks the focused element's HandleInput first, and the editor
	// after it — so on macOS a binding recorded as `Alt+N` was typed as ñ and
	// the settings page closed nothing (the same root cause as the launcher's
	// field, see launcher/fieldkeys.go). Only the new-command binding is
	// claimed here, because it is the one the settings surface answers:
	// `DISMISS_TABLE.settings["new-command"]` closes the panel
	// (`main:src/surface-policy.ts:216`), which is `Actions.Close`.
	return a.editorDismissKeys(e)
}

// fieldFace lays the reference's field material over an element: one quiet
// pane plus one hairline. It is the one place a field's material is decided,
// so a config input, a run parameter, a residency field and a shortcut command
// cannot drift into two faces.
//
// P8-c 重定基线（docs/raycast-design.md §0(b)/§6.1）：旧的 `--glass-field` 面 +
// `--glass-field-shadow` 凹槽（`theme.Tokens.FieldSlot`）收成 `Control` +
// `ControlEdge` 一档，与弹层的 `sheetFieldSlot` 同面。理由是合约自己的上限：
// §1 把光泽（rim/sheen/lens）一律压到 alpha ≤ 0.04，而 `FieldSlot` 在浅色下
// 的白边是 **0.85**——上限的 21 倍；Raycast 的输入框是一档浅面 + 一条发丝边，
// 没有内侧白边。旧材质在图上就是「输入框像一块玻璃砖」，这正是本片要收掉的。
func (a *App) fieldFace(c *ui.Context, e ui.Element, lit bool) ui.Element {
	return a.fieldFaceEdge(c, e, lit, a.tokens(c).ControlEdge)
}

// fieldFaceEdge is `fieldFace` with the frame as a parameter: the field family's
// one material, and the one place its outer edge is chosen.
//
// P13-cc（`docs/liquid-glass.md` §6.46）：`fieldFace` 一直是「字段材质只有一处
// 决定」的说法，但三块字段面在自己拼材质——`aliasField`（外圈 `Hairline`，参考
// 的 `border: 1px solid var(--hairline)`）、`sheetFieldSlot` 与
// `configMultiselectBox`（外圈 `ControlEdge`，与这里逐字节相同的两行）。前两处
// 是纯复制，`aliasField` 只有外圈这一个合法差。所以边成了参数：面、状态臂
// （`lit`）、受光边那一对仍然只在这里决定，调用方只能选**画框**，不能选材质。
//
// 为什么不让调用方自己 `Border`：`DimFace`/`LitFace` 画的是**内**阴影，外圈是
// 另一件事（§1「受光边是内线，画框是外线」）——但两件事必须在同一处落地，否则
// 下一次改受光边的定值，绕开的那一块就留在旧材质上，同一个面板里出现两种字段。
func (a *App) fieldFaceEdge(c *ui.Context, e ui.Element, lit bool, edge ui.Color) ui.Element {
	tokens := a.tokens(c)
	// P12-c（§6.2 第 2-4 行）：字段是 **E1 面**。P8-c 把它收成「`Control` 面 +
	// 一条 `ControlEdge` 发丝边」是对的（它去掉了 0.85 的内侧白边），但它只
	// 停在了「一块被画框圈住的平涂」。`LitFace` 补上 E1 缺的那一对线：顶受光
	// 边 `LensRim` + 底对侧 `LensEdge`，无投影。外圈 `ControlEdge` 不动——它是
	// 字段与卡片之间的那道线，不是受光边。
	//
	// P13-cb（§6.45）：`lit` 是**状态**交给**材质**的那一条线。锁定（`Disabled`）
	// 的字段此前戴着一半的灯——冠线仍在（实测 +10 级深 / +2 级浅），底对侧的
	// 暗线却被 0.5 那道淡吃掉了，于是它读起来不是「一个灭了灯的凹槽」，是
	// 「一块平涂 + 一条半亮的顶线」。受光边是「这块面可以被碰」的记号（§3），
	// 锁定的面不该戴着它，所以这一档走 `DimFace`：摘冠线、留暗线。
	//
	// 为什么是参数而不是事后叠 `DimFace`：mygo 的内阴影只追加不撤回
	// （`ui/element.go:1162`），冠线是这里建元素时画上去的，之后再叠什么都
	// 摘不掉它（P13-ca 试过两条路，都失败，见 §6.44）。所以状态必须在**建面**
	// 的这一刻就到场；而 `lit` 仍然只由这一处决定材质——同一张表单上的输入框、
	// 下拉框、参数框不会各自漂到两种面。
	e = e.Background(tokens.Control).Border(1, edge)
	if !lit {
		return tokens.DimFace(e)
	}
	return tokens.LitFace(e)
}

// extensionFieldMetrics is the literal box one of the extension panel's text
// fields wears. The reference writes the panel's fields in px, one step apart:
// a config field is a 28px floor with 4px/8px padding and a 999px pill
// (`.extension-config-field input`, extensions.css:2402-2417), while a custom
// form field — the local-tool form and the parameter editor — is a 30px floor
// with 3px/7px padding and the same pill (`.extension-custom-form input`,
// extensions.css:1770-1781).
type extensionFieldMetrics struct{ minHeight, padY, padX float32 }

var (
	configFieldMetrics = extensionFieldMetrics{minHeight: 28, padY: 4, padX: 8}
	customFieldMetrics = extensionFieldMetrics{minHeight: 30, padY: 3, padX: 7}
)

// aliasFieldMetrics is the command-alias editor's own box: the reference's
// `.extension-command-alias__input` (`extensions.css:2281-2290`) — a
// `padding: 4px 8px` box with the *ladder's* `--radius-xs` (6px) corner and the
// `--text-caption` step. It is deliberately neither the config pill (28/4/8/
// 999) nor the custom pill (30/3/7/999): the alias is a caption-scale inline
// box, so it keeps the small corner and the panel's caption register, and its
// 4/8 padding is written in literal px like the rest of `extensions.css`.
type aliasFieldMetrics struct{ padY, padX, radius float32 }

var commandAliasMetrics = aliasFieldMetrics{padY: 4, padX: 8, radius: theme.RadiusXS}

// extensionField is the framework's text field at the extension panel's own
// metric. The reference's panel is written in literal px and does not follow
// the interface step the way the settings pages' `--u` metrics do, so inside
// `extensionMetrics` the field is pinned to `m` and keeps the same box at every
// step; on the settings pages, where `cardUnit` is zero, the field keeps the
// framework's metrics and scales with the step as the reference's `--u` fields
// do. Every extension-panel entry point for a text field goes through here, so
// a config field, a run parameter, a local-tool input and a parameter-editor
// input cannot drift into two boxes.
func (a *App) extensionField(c *ui.Context, value *string, m extensionFieldMetrics) ui.Element {
	return a.extensionFieldLit(c, value, m, true)
}

// extensionFieldLit is `extensionField` with the field's lit edge under the
// caller's control: the config form's fields are the one chain that can be
// locked (`configLocked`), so they are the one chain that passes `lit`.
func (a *App) extensionFieldLit(c *ui.Context, value *string, m extensionFieldMetrics, lit bool) ui.Element {
	e := a.textInputLit(c, value, lit)
	if a.cardUnit > 0 {
		e = e.MinHeight(m.minHeight).Padding(m.padY, m.padX).Radius(999)
	}
	return e
}

// aliasField draws one command's alias editor: the reference's
// `.extension-command-alias__input` (`extensions.css:2281-2290`) — a caption
// register, `--hairline`-edged, `--glass-control`-filled box with the ladder's
// 6px corner and a literal `4px 8px` padding. The box is pinned the same way
// `extensionField` pins its fields, so the alias keeps one box at every
// interface step and does not follow the settings pages' `--u`.
func (a *App) aliasField(c *ui.Context, value *string) ui.Element {
	return a.aliasFieldLit(c, value, true)
}

// aliasFieldLit chooses the material before the row lock dims the control.
func (a *App) aliasFieldLit(c *ui.Context, value *string, lit bool) ui.Element {
	e := ui.TextInput(c, value)
	e.Shortcut(0, ui.KeyUp)
	e.Shortcut(0, ui.KeyDown)
	e = a.editorDismissKeys(e)
	tokens := a.tokens(c)
	if a.cardUnit > 0 {
		m := commandAliasMetrics
		e = e.MinHeight(tokens.Caption+m.padY*2).
			Padding(m.padY, m.padX).
			Radius(m.radius)
	}
	e = e.FontSize(tokens.Caption)
	// The alias field is the panel's one control filled with the *control*
	// rung (`.extension-command-alias__input { background: var(--glass-control);
	// border: 1px solid var(--hairline) }`), not the field rung the other panel
	// fields wear: it is an inline command-chip, not a settings slot.
	//
	// P13-m（`docs/liquid-glass.md` §6.2b-2 第 9-17 行）：上面那段说的是**边框**
	// 的选择（`Hairline` 而不是 `ControlEdge`），不是「不要受光边」的理由——受光
	// 边是**内线**，与画框是两件事。它是面板字段族里唯一没有 E1 一对线的字段，
	// 于是同一个面板里「能打字的槽」有两种材质。外圈 `Hairline` 不动。
	//
	// P13-cc（§6.46）：材质不再在这里拼——它走 `fieldFaceEdge`，外圈那一个合法差
	// 作为参数传进去。像素零改动（同一个面、同一对线、同一条 `Hairline`）。
	return a.fieldFaceEdge(c, e, lit, tokens.Hairline)
}

// A window shortcut can skip HandleInput and still lose to the native
// editor's selection/deletion keys. OnTextKey covers those editing chords
// before they change the value; HandleInput covers the unclaimed keys.
func (a *App) editorDismissKeys(e ui.Element) ui.Element {
	take := func(mods ui.Modifiers, key ui.Key) bool {
		wantMods, wantKey, ok := shortcuts.Parse(settings.Shortcut(a.Store.Snapshot(), settings.ShortcutNewCommand))
		if !ok || mods != wantMods || key != wantKey {
			return false
		}
		if a.Actions.Close != nil {
			a.Actions.Close()
		}
		return true
	}
	e.HandleInput(func(ev ui.InputEvent) bool {
		return ev.Kind == ui.InputKeyDown && take(ev.Mods, ev.Key)
	})
	return e.OnTextKey(func(key ui.TextKey) bool { return take(key.Mods, key.Key) })
}
