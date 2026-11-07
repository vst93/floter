package settingsui

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/launcher"
	"floter/internal/motion"
	"floter/internal/theme"
)

// The grouped-card language the old settings panel used, and macOS System
// Settings with it: a section title with a one-line explanation under it,
// then a flat card of rows — a label (and a grey second line) at the start,
// the control at the end, a rule inset to the label's own column between
// one row and the next. The card is a *plane*, never glass: the material
// belongs to the window shell, and a card that blurred would be a second
// material inside the one sheet the app allows.
//
// Four pieces, no third-party vocabulary: a section (title + hint + cards),
// a card, a row, and the action — the right-aligned accent text the old
// build used for "Edit…" and "Restore defaults". Every page keeps owning
// its own state, handlers and stored keys; these are presentation only.

// tokens resolves the card language's colors for the stored appearance,
// scaled with the interface step exactly as the CSS variables were.
//
// It is the frame's memo, not a fresh build: every row of every card asks for
// it (`a.tokens(c)` has 97 call sites on this surface, all while drawing), and
// `theme.ForPreferences` builds a whole `ui.Theme` and palette each time —
// 29% of `BenchmarkIntegrationsFrame`'s CPU before P11-bs, plus one
// `Store.Snapshot` per row. The memo is keyed by the store's version, the
// appearance and the desktop's preferences, and loads the settings on a miss
// only, so a frame reads them once.
func (a *App) tokens(c *ui.Context) theme.Tokens {
	if a.tokensStore != a.Store {
		// Two stores both begin at version zero, so a swap is noticed by
		// identity rather than by the version (the panel's tests do swap the
		// store under a built App).
		a.tokensStore, a.tokensMemo = a.Store, theme.Memo{}
	}
	t := c.Theme()
	return a.tokensMemo.Tokens(a.Store.Version(), t.Dark, c.Preferences(), a.settings)
}

// pageColumn is the column every settings page's body is drawn in: full
// width, and the reference's page grid gap, `.settings-page { gap: 22u }`
// (`settings.css:379-387`) — the room the reference's grid left between its
// header and each section, and therefore between one section and the next.
// All five pages share it so the gap has one definition; before P7-gs the
// General page was the lone exception at 16u (Space(4)).
func (a *App) pageColumn(c *ui.Context, fn func()) {
	ui.Column(c).FillWidth().Gap(c.Theme().Space(5.5)).Children(fn)
}

// section draws a titled group: the title, its one-line explanation, and
// the cards fn builds under them. The explanation is a `.settings-section__hint`
// line at the *end* of the section, after the cards it explains — the reference
// draws a group's explanations as `.settings-section__hint` lines at the end of
// the section when they speak for the whole group rather than for one row, so
// the cards above them carry no repeated per-row hint.
func (a *App) section(c *ui.Context, title, hint string, fn func()) ui.Element {
	return a.sectionGroup(c, title, func() {
		a.sectionHeading(c, title, nil)
		ui.Column(c).FillWidth().Gap(a.cardTheme(c).Space(2.5)).Children(fn)
		a.sectionHint(c, hint)
	})
}

// sectionGroup is the frame every section shares: the full-width column at the
// reference's 8u section gap (`.settings-section { gap: 8u }`,
// `settings.css:411-419`), labelled with the group's name. The label is the
// group's accessible name — the reference drew each `.settings-section` as a
// labelled group — and it also gives a test the group's own box: the page gap
// between two sections is the room from one group's bottom edge to the next
// group's top edge (`pageColumn`), which no text box shows.
//
// The gap is the one tick every group's *internal* rhythm wears — heading →
// cards → hint. P13-br removed the last exception (the material group's 10u,
// which rendered 9px against everyone else's 7.2px: a 25% step on a 7px gap
// that no eye resolves, while the padding it kept is five times larger and is
// what actually separates that section). See `docs/liquid-glass.md` §6.36.
func (a *App) sectionGroup(c *ui.Context, title string, fn func()) ui.Element {
	return a.sectionGroupPadded(c, title, 0, fn)
}

// sectionGroupPadded is sectionGroup for a section that keeps its own breathing
// room inside its own box: the General page's material group is the reference's
// one `.settings-section--material`, `padding: 10u 0` (`settings.css:767-772`).
// The padding is *inside* the group box, so the page gap between the material
// section and its neighbour stays the shared 22u (`pageColumn`) while the group
// itself keeps the reference's own breathing room. A zero padding leaves the
// plain group.
//
// Padding is the only degree of freedom this frame takes: the reference also
// gave that one section `gap: 10u` against the shared 8u, and that half of the
// exception is gone (P13-br) — a gap argument here is how the second tick came
// back the first time.
func (a *App) sectionGroupPadded(c *ui.Context, title string, padding float32, fn func()) ui.Element {
	group := ui.Column(c).FillWidth().Gap(a.cardTheme(c).Space(2))
	if padding > 0 {
		group = group.Padding(padding, 0)
	}
	group.Label(sectionLabel(title))
	group.Children(fn)
	return group
}

// sectionLabel names a section's group. The `\u00a0section` suffix keeps the
// label from colliding with the title text's own node in `Tester.Find`.
func sectionLabel(title string) string { return title + "\u00a0section" }

// sectionMaterial is section for the material group, the reference's one
// section that is spaced differently: `.settings-section--material` keeps
// `padding: 10u 0` (`settings.css:767-772`) where every other
// `.settings-section` (`:411-419`) sits flush in its group box. Its two cards
// are 10u apart (`.settings-cards { gap: 10u }`, `:491-495`), the same column
// gap section draws. The group's two explanations are footnotes under both
// cards, as the reference draws them.
//
// The reference also gave this one section `gap: 10u` against the shared 8u;
// P13-br dropped that half — measured, the 1.8px it added (9px against 7.2px)
// is not a step any eye resolves, and it was the page's only second tick for a
// role that already has one. The padding is what makes this section read as its
// own: it lifts the heading 9px off the group box's top edge, so the room above
// it is 29px against the 20px every other section gets.
func (a *App) sectionMaterial(c *ui.Context, title string, footnotes []string, fn func()) ui.Element {
	t := c.Theme()
	return a.sectionGroupPadded(c, title, t.Space(2.5), func() {
		a.sectionHeading(c, title, nil)
		ui.Column(c).FillWidth().Gap(t.Space(2.5)).Children(fn)
		for _, note := range footnotes {
			a.sectionHint(c, note)
		}
	})
}

// sectionInline is section for the deep-link group: the reference puts that
// group's one-line explanation inside `heading-main`, directly under the title
// (`DeepLinkRow.tsx:61-64`), rather than in a `sectionHint` line after the
// cards. The card and the 2u section gap are the same as section's.
func (a *App) sectionInline(c *ui.Context, title, hint string, fn func()) {
	a.sectionGroup(c, title, func() {
		a.sectionHeadingInline(c, title, hint, nil)
		ui.Column(c).FillWidth().Gap(c.Theme().Space(2.5)).Children(fn)
	})
}

// sectionHeading is the reference's `.settings-section__heading`
// (`settings.css:421-438`): the group's label, inset 2u from the page's edge
// and bottom-aligned against the group's own action with the reference's 12u
// gutter between them. The label is the reference's `heading-main` block — the
// one place the pane ever put an explanation *under* a title, which is the
// deep-link row's inline hint, not a group hint (those are `sectionHint`
// lines after the cards). The old port drew the title flush with the card and
// centred the action on the block instead of resting it on the block's bottom
// edge.
func (a *App) sectionHeading(c *ui.Context, title string, action func()) ui.Element {
	return a.sectionHeadingInline(c, title, "", action)
}

// sectionHeadingInline is sectionHeading for the reference's one heading that
// carries its own explanation: the deep-link row's `heading-main` block
// (`DeepLinkRow.tsx:61-64`; `settings.css:430-434`), a 1px stack of label and
// `settings-section__hint--inline` hint (`:1209-1211`, padding 0). Every other
// group's explanation is a `sectionHint` line after its cards. An empty hint
// keeps the plain single-line heading the rest of the pane uses.
func (a *App) sectionHeadingInline(c *ui.Context, title, inlineHint string, action func()) ui.Element {
	t := a.cardTheme(c)
	// P13-ek（`docs/liquid-glass.md` §6.106）：分组标题离它自己第一张卡的那一格是
	// **这一对**的空气，不是整列的（§4.3）。此前它只有框架的 `Space(2)`（7.2px），
	// 比同一组里两张卡之间的 9px **还紧**——标题读成「又一张卡」，不是「新的一组」。
	// 所以抬的是一整档 8u，加在标题的**下边**：标题 → 卡 = 7.2 + 7.2 = `Space(4)`
	// 14.4px，与浮层里同一格（§6.105）逐值相同；卡 → 它下面那句组注仍是 7.2px
	// （那一对是「同一块」，§4.3 的相邻一档不许动）。
	row := ui.Row(c).FillWidth().AlignItems(ui.End).Justify(ui.SpaceBetween).
		Gap(t.Space(3)).Padding(0, t.Space(0.5), t.Space(2), t.Space(0.5))
	row.Children(func() {
		// `heading-main` is the reference's `display:grid; gap:1px`, so the
		// inline hint rides one pixel under the title in the same column.
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			// `.settings-section__label` is the ladder's title step at 620 in the
			// strong colour.
			ui.Text(c, title).FontSize(a.tokens(c).Title).FontWeight(620).TextColor(a.tokens(c).TextStrong)
			if inlineHint != "" {
				// `--inline` drops the 2u inset the group hint keeps, but is
				// otherwise the same body step at 500 in the muted colour.
				ui.Text(c, inlineHint).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
			}
		})
		if action != nil {
			action()
		}
	})
	return row
}

// sectionHint is the reference's `.settings-section__hint` line
// (`settings.css:1198-1206`): the group's one-line explanation, the body step
// at 500 in the muted colour, inset 2u like the heading above it. The reference
// drew it at the *end* of the section — after the cards it explains — not in
// the heading, which is why the title block above it stays a single line.
func (a *App) sectionHint(c *ui.Context, hint string) {
	if hint == "" {
		return
	}
	t := a.cardTheme(c)
	ui.Row(c).FillWidth().Padding(0, t.Space(0.5)).Children(func() {
		ui.Text(c, hint).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
	})
}

// cardProse is a card's *prose*: a sentence that explains the rows around it
// rather than naming one of them. The permission boundary note is the card's one
// such line — "these switches record intent, they sandbox nothing" — and it wore
// the row it used to be drawn as: `a.row(c, "", note, …)`, i.e. an **empty
// label** in the emphasis step above the sentence, inside a 44px-floor row with
// its own hairline. Measured at 720×2200 (English, both appearances): the
// sentence's text column was 448 wide, the ink ran out at 375 and the last
// glyph was two pixels tall — the framework's ellipsis, not a letter. The
// sentence's second half ("These controls do not create a sandbox") was never
// painted; the honest half of the disclosure was the one part of it nobody
// could read, and 19px of blank emphasis line sat above it saying nothing.
//
// P13-dl 重定基线（`docs/liquid-glass.md` §6.81）：prose wraps, and it is drawn at
// the rung this build already gives prose under a group label — `sectionHint`: the
// body step, weight 500, `t.TextMuted`, full width (`rows.go:200-208`). **Ink is
// unchanged** (101 light / 94 dark, measured before and after); what changed is
// that the line is no longer cut, and that a sentence stops being framed as a
// setting. The row inset is kept: this helper is called from a row's `below`
// column, which is already padded to `rowInset`.
func (a *App) cardProse(c *ui.Context, text string) {
	if text == "" {
		return
	}
	t := a.cardTheme(c)
	ui.Row(c).FillWidth().Padding(t.Space(2.25), 0).Children(func() {
		ui.Text(c, text).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
	})
}

// extensionSection is the Integrations panel's own group frame: the
// reference's `.extensions-section-title` (`extensions.css:419-446`) — a
// body-step, muted, 620 label with the section's count in the neutral
// `.extension-status` chip beside it (`:448-472`) — rather than the settings
// cards' `.settings-section__label` (the title step in `--text-strong`) that
// every other page's `section` draws. The chip is a fixed 18u box with a 25u
// floor, so a 1→2 digit count (or a refresh) never reflows the heading or the
// list under it. A negative count drops the chip, for a group with a title
// and nothing to count.
//
// The panel draws its explanations *under the title and above the list*
// (Base plugins: `ExtensionsPanel.tsx:2435-2436`; Detected: `:2542-2544`),
// not at the end of the section the way the settings cards do, so fn draws
// its own hint first (`extensionHint`) and its cards after.
func (a *App) extensionSection(c *ui.Context, title string, count int, fn func()) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	return a.sectionGroup(c, title, func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).
			Padding(0, t.Space(1)).MinHeight(t.Space(4.5)).Children(func() {
			ui.Text(c, title).FontSize(tokens.Body).FontWeight(620).TextColor(t.TextMuted)
			if count < 0 {
				return
			}
			chip := ui.Row(c).MinWidth(t.Space(6.25)).Height(t.Space(4.5)).
				Padding(0, t.Space(1.75)).Radius(tokens.RadiusSM).Center()
			chip.Children(func() {
				ui.Text(c, strconv.Itoa(count)).FontSize(tokens.Caption).
					FontWeight(600).TextColor(tokens.TextSecondary)
			})
		})
		fn()
	})
}

// extensionHint is the Integrations panel's own explanation line: the
// sentence under a group's title and above its list — the panel's own order,
// not the settings cards' footnotes.
//
// P13-ey 重定基线（`docs/liquid-glass.md` §6.120）：这一族此前有**两个**寄存器，
// 而它们在同一页、同一条滚动里：Base plugins 的 `settings-section__hint`
// （正文档、`--text-tertiary`、2u 缩进）与 Detected 的
// `.extensions-section-hint`（**caption 档**、4u 缩进）。实测（760×1400，深浅
// 逐值相同）：两句的字号差一档（行盒 17 vs **15**），而 Base plugins 那句的
// 首个字形停在 X=188.8、它自己的标题在 X=190.8 —— 说明句比它解释的标题**往左
// 探出 2px**。一句话的角色不该由它落在哪一组决定，所以两条支合成一条：正文档、
// `--text-tertiary`（比标题的 `--text-muted` 低一档，层级靠墨不靠字号）、
// 缩进取标题自己的 `Space(1)`（首个字形与标题同一条竖线）。
//
// 旧的 `caption` 参数随之消失：它只剩一个作用——把 Detected 那句画小一档。
func (a *App) extensionHint(c *ui.Context, hint string) {
	if hint == "" {
		return
	}
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	ui.Row(c).FillWidth().Padding(0, t.Space(1)).Children(func() {
		ui.Text(c, hint).FontSize(tokens.Body).FontWeight(500).TextColor(tokens.TextTertiary)
	})
}

// card draws one grouped card: the resting control plane, its hairline
// edge, the card radius, and the clip that keeps the first and last rows
// inside the rounded corners. It is the row counter's own frame: the rows
// fn builds draw the rule *above* themselves except the first, so a card
// never opens or ends on a rule and no caller has to say which row is
// last.
func (a *App) card(c *ui.Context, fn func()) {
	a.cardLabeled(c, "", fn)
}

// cardLabeled is card with the group's accessible name, the reference's
// `aria-label` on a card whose visible title is the section heading above it
// (the transparency card inside the material section).
func (a *App) cardLabeled(c *ui.Context, label string, fn func()) {
	a.cardGeometry(c, 0, 0, 0, label, fn)
}

// cardGeometry draws a card whose rows wear geometry other than the settings
// default. `inset` is every row's horizontal inset (the label column and the
// separator's own margin), `titleFloor` is the height floor of the card's
// *first* row alone — the settings card's 44u floor for every row under it —
// and `lineGap` is the gap between a row's label and its sub-lines. A zero
// value keeps the settings card's own numbers (a 14u inset, a 44u floor and a
// 1px line gap), so `card` and `cardLabeled` are just this with zeroes.
//
// The integration card is the one caller that asks for the reference
// extension row's own geometry instead: `.extension-row` is a 66px row with
// 8px of padding and a 4px line gap (`extensions.css:15-16,733-744,879-880`),
// so its title row is a 66u floor and every row under it — Permissions,
// Freshness, the command list — shares the same 8u inset and 4u line gap,
// which is what makes their labels and the card's separators line up on the
// extension row's own text column. The inset/floor/gap override is scoped to
// this card: a card nested inside it (the run form, the config fields) resets
// to the settings geometry through `card`. The *unit* is not: the integration
// card is drawn inside `extensionMetrics`, so every helper that sizes itself
// through cardTheme/cardSpace — this frame's rows and the nested cards alike —
// keeps the extension panel's literal-px metrics.
func (a *App) cardGeometry(c *ui.Context, inset, titleFloor, lineGap float32, label string, fn func()) {
	tokens := a.tokens(c)
	saved := a.cardRows
	savedInset, savedFloor, savedGap := a.cardRowInset, a.cardTitleFloor, a.cardLineGap
	a.cardRows = 0
	a.cardRowInset, a.cardTitleFloor, a.cardLineGap = inset, titleFloor, lineGap
	defer func() {
		a.cardRows = saved
		a.cardRowInset, a.cardTitleFloor, a.cardLineGap = savedInset, savedFloor, savedGap
	}()
	// The card wears the contract's plane (`docs/raycast-design.md` §6.1): the
	// resting control fill, the *weak* hairline (`HairlineSoft`) as its edge,
	// the panel-block radius step (10) and **no lens** — a low-gloss design
	// 靠「面 + 线 + 字阶」分层，不靠高光（§1）。
	//
	// P8-b 重定基线：旧 `ControlEdge` + `Control` + `--elev-0` 透镜 →
	// `HairlineSoft` + `Control`，镜片去掉；半径 10 保留。
	card := ui.Column(c).FillWidth().
		Border(1, tokens.HairlineSoft).
		Radius(tokens.RadiusMD).
		Background(tokens.Control).
		Clip()
	// P12-c（`docs/liquid-glass.md` §6.2 第 2-3 行）：分组卡是 **E1 面**，不是
	// 一块平涂。它此前只有 `Control` 的填充和一圈弱发丝边——边是「画框」，
	// 而这一支要的是「块自己站起来」。`LitFace` 只加两条 1px 内线（顶受光边
	// `LensRim`、底对侧 `LensEdge`），不画投影（§2 判据 2：E1 长在底上，压不出
	// 影子），也不加宽面光泽（`LensSheen` 仍 0.04，面不许反光）。外圈的
	// `HairlineSoft` 一个字节没动：它仍是这张卡与页面之间的那圈线。
	card = tokens.LitFace(card)
	if label != "" {
		card.Label(label)
	}
	card.Children(fn)
}

// runParamsBox draws the reference's `.extension-run-params` box
// (`extensions.css:1224-1236`): the run form's own surface, a `--surface-sunken`
// plane under a `--glass-control-edge` hairline with the `--ext-radius` corner.
// The reference's box carries no lens and no card plane — it is a recess the
// inputs sit *in*, not a raised card — so the run form does not wear the
// settings card's `--glass-control` fill, its `--radius-md` corner or its
// `--elev-0` stack. Its rows keep the row machinery and the extension row
// counter, but they carry the box's own 8px inset instead of the settings
// card's 14u and they draw **no separator at all**: the reference is a
// `display: grid` with a `gap: 6px` and no hairline
// (`extensions.css:1224-1233`), so a parameter's input sits 1px + 8px inside
// the frame and the rows read as a spaced grid, not as a column of ruled
// settings rows (`cardNoRules` drops the separator `cardRow` would otherwise
// draw above every row but the first, and the box takes the reference's own
// 6px `gap` between rows).
func (a *App) runParamsBox(c *ui.Context, fn func()) {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	saved := a.cardRows
	savedInset, savedFloor, savedGap := a.cardRowInset, a.cardTitleFloor, a.cardLineGap
	savedNoRules := a.cardNoRules
	a.cardRows = 0
	a.cardRowInset, a.cardTitleFloor, a.cardLineGap = a.cardSpace(t, 2), 0, 0
	a.cardNoRules = true
	defer func() {
		a.cardRows = saved
		a.cardRowInset, a.cardTitleFloor, a.cardLineGap = savedInset, savedFloor, savedGap
		a.cardNoRules = savedNoRules
	}()
	box := ui.Column(c).FillWidth().Gap(t.Space(1.5)).
		Border(1, tokens.ControlEdge).
		Radius(tokens.RadiusSM).
		Background(tokens.Sunken).
		Clip()
	box.Children(fn)
}

// rowInset is the horizontal inset a card row wears: the settings card's own
// 14u (`--u * 14`), or the override cardGeometry set for the card being built
// (the integration card's reference extension-row padding, 8u).
func (a *App) rowInset(t *ui.Theme) float32 {
	if a.cardRowInset > 0 {
		return a.cardRowInset
	}
	return a.cardSpace(t, 3.5)
}

// rowLineGap is the gap a row's label column puts between its label and its
// sub-lines: the settings row's own 1px (`.settings-row__main { gap: 1px }`,
// `settings.css:528-531`), or the override cardGeometry set for the card being
// built (the extension row's `.extension-row__main { gap: 4px }`,
// `extensions.css:879-880`, which the integration title row's name, version,
// meta and description all share).
func (a *App) rowLineGap() float32 {
	if a.cardLineGap > 0 {
		return a.cardLineGap
	}
	return 1
}

// rowFloorPx is a card row's height floor: the reference's `.settings-row` is a
// literal 44px box (`min-height: 44px`, `settings.css:511-518`), so the floor
// is that literal and not `--u * 11`. At the shipped step `--u * 11` is 39.6 —
// four and a half pixels short of the box the reference drew, which is half of
// why every settings row read pinched — and at `large` it is 48.4, i.e. a row
// taller than the design's own. It is the same rule as the radius ladder (a
// corner is a literal px, not a `--u` metric): the floor is a touch target, the
// one number in the row that does not grow with the text it holds.
//
// The taller of the two wins, so a step that draws its text bigger than the
// literal floor (the `large` step's 13.2pt body in a 44px row) still gets a row
// its content fits in.
const rowFloorPx = 44

// sessionRowFloorPx is the sessions page's row floor: the reference's
// `.session-manager__row { min-height: 56px }`
// (`main:src/styles/settings.css:1237-1244`), whose own comment says it is
// "a taller floor, because a session is two lines of text plus a footer of
// metadata". Like rowFloorPx it is the literal the reference wrote and not
// `--u * 14`: at the shipped step 14u is 50.4, which is what this row's
// content already measures, so a `Space(14)` floor is invisible (P13-dz).
// The 5.6px the floor adds land **under** the metadata line — the title keeps
// its 2.25u padding from the row's top — so the row reads as a block with air
// beneath it rather than a block pressed onto its own bottom edge.
const sessionRowFloorPx = 56

// extensionMetrics runs fn with the extension panel's own metric unit: the
// reference's `extensions.css` is written in literal px (a 66px row, a 30px
// chip, an 8px padding) and therefore does not follow the interface step the
// way the settings pages' `--u` metrics do. Inside fn, cardTheme pins
// `Space(n)` to the literal 4n, so a helper that sizes its own metrics draws
// the reference's own numbers at any interface size. The setting is scoped and
// restored, and the framework's own widgets (a switch, an input) still read the
// context theme, so the controls the reference *does* scale keep scaling.
func (a *App) extensionMetrics(fn func()) {
	saved := a.cardUnit
	a.cardUnit = 4
	defer func() { a.cardUnit = saved }()
	fn()
}

// cardSpace is one metric of the surface being drawn, in that surface's own
// unit: the theme's `Space(n)` (4n·scale) for the settings pages, or the
// literal 4n for the extension panel, whose reference CSS is written in
// literal px. A helper that computes a size for itself rather than through a
// theme copy reads its unit here.
func (a *App) cardSpace(t *ui.Theme, n float32) float32 {
	if a.cardUnit > 0 {
		return a.cardUnit * n
	}
	return t.Space(n)
}

// cardTheme is the theme a drawing function should size its own metrics
// against: the framework theme, or — inside the extension panel, whose
// reference CSS is written in literal px — a copy whose Spacing is pinned to
// the literal 4px unit, so every `t.Space(n)` in the function is 4n at any
// interface step. Colours and the font ladder come through unchanged.
func (a *App) cardTheme(c *ui.Context) *ui.Theme {
	t := *c.Theme()
	if a.cardUnit > 0 {
		t.Spacing = a.cardUnit
	}
	return &t
}

// pageButton is the settings pages' own quiet action pill: the one face every
// control a page draws for itself wears, so no view has to reach for the
// framework's stock button.
//
// P13-ai 重定基线（`docs/liquid-glass.md` §6.8）：这一族此前是**四枚**
// `ui.Button`——全仓最后四个还在用框架默认按钮面的控件（会话页页头与空态各
// 一枚、自定义快捷键草稿行的「绑定」、自绘驻留的「应用」）。框架的
// `styleButton`（`mygo/ui/widgets.go:152-186`）给的是「`Surface` 平涂 + 四边
// 一圈同值的 `stroke`」：实测（中灰 128 地面）浅色面 119 / 四条边**全 108**，
// 深色面 141 / 四条边**全 151**——四条边一个值，也就是说它是一个**画框**，
// 不是一块面：没有任何一条线说它比地面高。同一页上其它每一个动作胶囊都戴
// E1 的一对面，只有这四枚还是 §2 判据里被退掉的「靠画框分层」。
//
// 几何逐字段照抄框架（`MinHeight` 30u、`Padding` 6u/14u、`Radius` 9u），所以
// 盒子一个像素都不动；变的只有墨与面：面还是 `Control` 三档，边换成这一族自己的
// `ControlEdge`，再戴上 `LitFace` 的一对面（顶受光边 + 底对侧），墨抬到 600，
// 按下收 0.97（§5 的第三档，与 `extensionActionFace` 同一句）。
func (a *App) pageButton(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	button := ui.ButtonBase(c).
		MinHeight(t.Space(7.5)).
		Padding(t.Space(1.5), t.Space(3.5)).
		Radius(t.Radius)
	ink, plane := tokens.TextStrong, tokens.Control
	if button.Hovered() {
		ink, plane = t.Text, tokens.ControlHover
	}
	pressed := button.Pressed()
	if pressed {
		plane = tokens.ControlPress
	}
	button.Border(1, tokens.ControlEdge).Background(plane).
		Transition(ui.ElementTransition{Colors: true, Duration: theme.ColorStep})
	if pressed {
		button = button.Scale(0.97, 0.97)
	}
	tokens.LitFace(button)
	button.Children(func() {
		ui.Text(c, label).FontSize(tokens.Body).FontWeight(600).TextColor(ink)
	})
	return button
}

// actionButton draws one of the card's action buttons — the reference's
// `.extensions-action-button` (`extensions.css:1313-1338`): the extension
// panel's shared pill, a 30u control (`.extensions-control-height`), 10u of
// side padding, a 999px corner (half the box, so the shape is a pill), the
// `--glass-control` plane under a `--glass-control-edge` hairline, and the
// body step at 600 in `--text-strong`. Hover lifts the plane to
// `--glass-control-hover`, the edge to the accent keyline and the ink to
// `--text-primary`; a disabled button keeps its resting face. The lens is the
// same four inset layers the card wears, and `--glass-lens-scale` grades it
// with the glass stop.
func (a *App) actionButton(c *ui.Context, label string) ui.Element {
	t := a.cardTheme(c)
	return a.extensionActionButton(c, label, t.Space(7.5), t.Space(3.75), a.tokens(c).Body)
}

// disabledActionButton is `actionButton` for a pill that is on screen but
// cannot be pressed — the row's whole action cluster while one of its
// operations is in flight (`rowAction`), the Run button of a row that has
// nothing to run, and the tool row with nothing to offer.
//
// It exists because `.Disabled(...)` **after** the pill was built is too late:
// `extensionActionFace` picks the material while it draws, and `actionButton`
// pinned that argument to `false`. The result was the one state this design
// forbids — a surface wearing the mark of a pressable one: measured on the
// light sheet the crown sat at **240 over a 238 face** and on the dark one at
// **36 over a 26 face**, i.e. a lit edge on a pill the pointer refuses. The
// material now goes through the same disabled arm every other locked control
// in this package uses (`busyActionButton`, `configOpenToolRow`, P13-s):
// `DimFace` — the counter-side shade stays, the light goes out.
//
// The face is the caller's and stays `Control`; the framework's painter
// already halves a disabled subtree (`ui/paint.go:257-259`), so the step off
// the ground is half a rung on purpose (§6.2b-6: one fade, and the material
// says the rest by going unlit).
func (a *App) disabledActionButton(c *ui.Context, label string) ui.Element {
	t := a.cardTheme(c)
	button, ink := a.extensionActionFace(c, t.Space(7.5), t.Space(3.75), true)
	button.Children(func() {
		ui.Text(c, label).FontSize(a.tokens(c).Body).FontWeight(600).TextColor(ink)
	})
	return button.Disabled(true).Cursor(ui.CursorDefault)
}

// extensionActionButton builds the panel's action pill at a chosen height,
// corner radius and text step, so the base-plugin card's Configure control
// can wear the reference's one-step-down variant
// (`.extensions-base-plugin__configure`, `extensions.css:2610-2614`: a 24u
// height and the emphasis step) without a second copy of the pill's face.
// The corner is passed rather than derived so a caller can keep the reference's
// 999px shape (half the box) at its own height.
func (a *App) extensionActionButton(c *ui.Context, label string, height, radius, fontSize float32) ui.Element {
	button, ink := a.extensionActionFace(c, height, radius, false)
	button.Children(func() {
		ui.Text(c, label).FontSize(fontSize).FontWeight(600).TextColor(ink)
	})
	return button
}

// extensionActionFace is the action pill without its content: the shared
// `.extensions-action-button` frame (`extensions.css:1313-1338`) that both the
// resting button and the in-flight one wear, so the two faces cannot drift
// apart. It returns the button and the ink its content wears; a disabled face
// keeps the resting plane, as the reference's
// `:hover:not(:disabled)` / `:disabled` pair did (`extensions.css:91,124-129`).
//
// P8-d 重定基线（§5 禁止侧）：hover 边曾是 `tokens.AccentEdge`。accent 不能
// 回答「指针在这条上」；hover 只抬面（`ControlHover`）与墨（`t.Text`），边
// 保持在 `ControlEdge`。
func (a *App) extensionActionFace(c *ui.Context, height, radius float32, disabled bool) (ui.Element, ui.Color) {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	button := ui.ButtonBase(c).
		MinHeight(height).Padding(0, t.Space(2.5)).Radius(radius)
	ink, edge, plane := tokens.TextStrong, tokens.ControlEdge, tokens.Control
	if disabled {
		ink = tokens.Disabled
	}
	// P13-s（`docs/liquid-glass.md` §6.2b-6）：禁用面不靠「把整棵子树乘两次」说话。
	// 框架的画笔已经给 `Disabled` 的子树乘了 **0.5**（`ui/paint.go:257-259`），而这一族
	// 之前又在外面叠了一个 `.Opacity(0.4)`：两个淡乘起来 = 有效 0.2，面离地面的台阶从
	// 17 级塌到 4 级（浅色 247 → 243）、从 25 级塌到 5 级（深色 14 → 19），E1 那一对线
	// 双双掉进噪声。现在只留一道淡（框架那一遭）：面继续用 `Control`（在 0.5 之下正好
	// 是半个台阶：浅 238 / 深 27），材质换成 `DimFace`——底对侧还在，**受光边熄了**
	// （受光边是「这块面可以被碰」的记号）。字也回到那一道 0.5，不再被乘成 0.2。
	if !disabled && button.Hovered() {
		ink, edge, plane = t.Text, tokens.ControlEdge, tokens.ControlHover
	}
	// P13-q（`docs/liquid-glass.md` §5「按下」这一档）：`.extensions-action-button`
	// 的第三档此前只有 `barButton` 一处实现（且只有几何，没有面），同一个 CSS 类的
	// 这一处连 `:active` 都没有——指针下去的那一下与悬停一模一样。`extensions.css:138-140`
	// 的 `:active:not(:disabled) { transform: scale(0.97) }` 是这一族的几何，
	// `--glass-control-press` 是这一族的面（`ControlPress`，沉到静息面之下）。
	// 面与几何都在按下时给，与 `barButton` 同一句。
	pressed := !disabled && button.Pressed()
	if pressed {
		ink, plane = tokens.TextStrong, tokens.ControlPress
	}
	button.Border(1, edge).Background(plane).
		Transition(ui.ElementTransition{Colors: true, Duration: theme.ColorStep})
	if pressed {
		button = button.Scale(0.97, 0.97)
	}
	// E1：顶受光边 + 底对侧暗线，**没有环**（`theme.Tokens.LitFace`）。四层 `Lens`
	// 的第四层是四边 `ControlEdge` 的环，它画在第一条 `LensRim` 之后、且落在同一个
	// 像素行上（mygo 的 `insetShadow` 把 hole 从 padding edge 算起，`spread=1` 的环
	// 正好占 padding 第一行，`y=1,spread=0` 的 rim 也是那一行），于是 P12-a 抬到
	// 0.09 的受光边被压在这条边下面——这枚胶囊的顶边一直是一条暗线。环与它自己的
	// `Border(1, edge)` 本来就是同一条边的第二次上色，退掉它不改几何，只改墨。
	if disabled {
		// 禁用面：熄了灯的面（只有底对侧）。
		tokens.DimFace(button)
	} else {
		tokens.LitFace(button)
	}
	return button, ink
}

// extensionPrimaryButton is the reference's accent face of the same pill:
// `.extensions-action-button--primary` (`extensions.css:1347-1364`) — the
// `--accent` fill, the `--accent-edge-strong` edge, the raised top rim
// (`--glass-raised-rim`) and the `--accent-contrast` ink. It is the one
// affirmative action in its block (the run form's Run, the config form's Save),
// never a state, so it spends the accent fill the neutral pills' keyline
// deliberately keeps for a chosen slot. A disabled primary turns its crown off and
// keeps the accent plane (P13-aq: one fade, the framework's 0.5 — the reference's
// `.4` was drawn *on top of* it, `docs/liquid-glass.md` §6.2b-6).
func (a *App) extensionPrimaryButton(c *ui.Context, label string, disabled bool) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	button := ui.ButtonBase(c).
		MinHeight(t.Space(7.5)).Padding(0, t.Space(2.5)).Radius(t.Space(3.75))
	button.Border(1, tokens.AccentEdgeStrong)
	button = a.accentPrimaryFace(c, button, disabled)
	button.Children(func() {
		ui.Text(c, label).FontSize(tokens.Body).FontWeight(600).TextColor(t.AccentText)
	})
	return button
}

// accentPrimaryFace is the material every accent primary CTA wears, in one
// place: the three face rungs, the lit crown, the press geometry and the
// disabled arm. The **edge** stays with each caller, because the two members of
// this class genuinely differ there — the reference draws
// `.extensions-action-button--primary` with `--accent-edge-strong`
// (`extensions.css:1347-1364`) and `.update-banner__button` with none
// (`settings.css:1542-1568`); adding one to the About key would also grow its
// box by 2px, which is a geometry change, not ink.
//
// §5 第 2 条 names 「主 CTA 填充」 as one class, and this build has exactly two
// members — the About page's update key (`updateButton`) and the extension
// panel's Save / Run (`extensionPrimaryButton`). Before P13-bc they answered the
// pointer in two different ways, and neither was the ladder's answer:
//
//   - `extensionPrimaryButton` had **no hover rung at all**: its resting and
//     hovered frames were pixel-identical (measured face `6/74/166` on both, in
//     the light appearance).
//   - `updateButton` hovered with `.Opacity(0.9)`, which multiplies the **face,
//     the crown and the ink** together. Two consequences: the lit edge — §3's
//     mark that this face can be touched — goes out exactly when the pointer
//     says it can; and the same fade moves in **opposite** z directions per
//     appearance, because transparency resolves against whatever ground is
//     underneath (light `6/74/166` faded *towards* the 247 ground to `29/91/173`
//     = sunk into the paper; dark `143/183/255` faded towards 14 to
//     `130/166/232` = also sunk). That is the same arithmetic P13-r rejected on
//     the segmented control's thumb: **opacity is not a z statement**.
//
// `theme` had carried `AccentHover` (`#0857C4` / `#A3C4FF`) and `AccentPressed`
// (`#053C87` / `#7AA7FF`) since P8-a with **zero call sites** — the ladder was
// designed and never wired. They are wired here, so both CTAs lift one rung
// under the pointer and sink one rung under the press, exactly as the neutral
// family already does through `Control` / `ControlHover` / `ControlPress`.
//
// The disabled arm is P13-aq's: the crown is the mark of a pressable face, so it
// goes out, and the fade is only the framework's own 0.5 for a `Disabled`
// subtree (the reference's `.4` used to be drawn *on top of* it,
// `docs/liquid-glass.md` §6.2b-6).
func (a *App) accentPrimaryFace(c *ui.Context, button ui.Element, disabled bool) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	plane := t.Accent
	if !disabled {
		switch {
		case button.Pressed():
			// The ladder stepping *back down*, `--accent-pressed`: the same third
			// rung the neutral pills wear as `ControlPress`, and the same 0.97
			// geometry (`extensionActionFace`'s `:active`).
			plane = t.AccentPressed
		case button.Hovered():
			plane = t.AccentHover
		}
	}
	button.Background(plane).
		Transition(ui.ElementTransition{Colors: true, Duration: theme.ColorStep})
	if disabled {
		button = button.Disabled(true).Cursor(ui.CursorDefault)
	} else {
		// `box-shadow: inset 0 1px 0 var(--glass-raised-rim)`: the raised top rim
		// is what makes the accent pill read as a lit control rather than a flat
		// accent block (the neutral pill gets the same from `Lens`). It stays on
		// every usable rung — including hover, where the old `.Opacity(0.9)` used
		// to fade it away.
		button.InsetShadow(0, 1, 0, 0, tokens.LensRim)
		if button.Pressed() {
			button = button.Scale(0.97, 0.97)
		}
	}
	return button
}

// extensionIconButton is the reference's `.extensions-icon-button`
// (`extensions.css:60-132`): the square icon-only control the config toolbar
// carries, a `--extensions-icon-size` (30px) box with the `--ext-radius` (9px)
// corner, a transparent face and `--text-secondary` ink. Hover lights the
// `--glass-control-edge` keyline, the `--ext-bg-hover` fill, the `--elev-0`
// lens and `--text-primary`; a disabled button keeps the resting face and turns
// the light off — the reference's `.42` used to be drawn on top of the framework's
// 0.5 (effective 0.21, P13-aq / `docs/liquid-glass.md` §6.2b-6). It is the
// icon-shaped sibling of
// `extensionActionButton`, so the toolbar's two controls share one vocabulary.
func (a *App) extensionIconButton(c *ui.Context, glyphName, label string, disabled bool) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	button := ui.ButtonBase(c).Label(label).
		Size(t.Space(7.5), t.Space(7.5)).
		Border(1, ui.Transparent).
		Radius(tokens.RadiusSM).
		Background(ui.Transparent).
		Center().
		Transition(ui.ElementTransition{Colors: true, Duration: theme.ColorStep})
	ink := tokens.TextSecondary
	switch {
	case disabled:
		ink = tokens.Disabled
		// P13-aq：只淡一道。框架的画笔已经给 `Disabled` 的整棵子树乘了 0.5
		// （`mygo@v0.7.3/ui/paint.go:257-259`），这里再叠一个 `.Opacity(0.42)` 就是
		// 有效 0.21——实测这枚键的字/面对比从 138 掉到 **29** 级（深色 141 → 30），
		// 恰好是 P13-s 在胶囊上判掉的那笔「一道淡画两遍」。
		button = button.Cursor(ui.CursorDefault).Disabled(true)
	case button.Pressed():
		// P13-q：28u 小格的第三档。启动器的 `rowControl`（20u）与折叠卡的设置格
		// （28u）按下时都是 `ControlPress` 面 + `Scale(0.92)`，同一个应用里同尺寸
		// 的小格只有一种触感；这一族此前只有静息与悬停两档。
		ink = t.Text
		button = button.Border(1, tokens.ControlEdge).Background(tokens.ControlPress)
		button = button.Scale(0.92, 0.92)
		tokens.LitFace(button)
	case button.Hovered():
		// `.extensions-icon-button:hover:not(:disabled)`: one keyline, the
		// ladder's hover fill, the resting lens and the strong ink.
		ink = t.Text
		button = button.Border(1, tokens.ControlEdge).Background(tokens.ControlHover)
		// E1 的同一对面，理由见 `extensionActionFace`：hover 的小格是面，不是环。
		tokens.LitFace(button)
	}
	button.Children(func() {
		ui.Icon(c, mustGlyph(glyphName)).Size(t.Space(3.5), t.Space(3.5)).TextColor(ink)
	})
	return button
}

// extensionGlyphActionButton is `extensionActionButton` with the reference's
// optional leading mark: the `.extensions-action-button` whose content is a
// lucide glyph then the word, at the panel's own `--extensions-inline-gap`
// (5px, `extensions.css:14,1313-1324`). The config form's "Restore defaults"
// is its one caller today, but the shape is the shared pill so the mark and
// the word keep the same face as every other action.
func (a *App) extensionGlyphActionButton(c *ui.Context, glyphName, label string, height, radius, fontSize float32, disabled bool) ui.Element {
	t := a.cardTheme(c)
	button, ink := a.extensionActionFace(c, height, radius, disabled)
	button.Gap(t.Space(1.25))
	button.Children(func() {
		ui.Icon(c, mustGlyph(glyphName)).Size(t.Space(3.5), t.Space(3.5)).TextColor(ink)
		ui.Text(c, label).FontSize(fontSize).FontWeight(600).TextColor(ink)
	})
	return button
}

// The in-flight mark's cadence: one full circle every 0.9 s, the framework's
// own spinner cadence (`ui.Spinner`). The turn is read through motion, so a
// desktop that asked for less motion draws the ring at rest.
const (
	actionSpinnerTurn    = 360
	actionSpinnerPerTurn = 900 * time.Millisecond
)

// actionSpinner draws the panel's in-flight mark inside an action pill:
// lucide's LoaderCircle, turning while the work runs. The reference swapped
// the row action's own glyph (its Wrench / RefreshCw) for exactly this one
// (`src/extensions/ExtensionRow.tsx:305,325,364`); on this pane the mark sits
// beside the action's word instead of replacing it, because the word is what
// tells a native pane *which* action is running. `key` names the loop, so two
// busy rows on one page turn independently.
func (a *App) actionSpinner(c *ui.Context, ink ui.Color, key string) {
	t := a.cardTheme(c)
	// P13-cl（`docs/liquid-glass.md` §6.55）：环占的是**静息 glyph 的那个盒子**
	// （`extensionGlyphActionButton` 的 `Size(Space(3.5))`，14px），不是它自己的
	// 10px。在飞臂与静息臂是同一枚键的两张画，槽一样宽，词才不会搬家；环比槽小
	// 4px 时，即使补了 glyph 也还差 4px。`busySwitchControl` 的轨道是 9u×5u
	// （36×20px），14px 的环在里面仍居中，不越界。
	icon := ui.Icon(c, mustGlyph("loader-circle")).
		Size(t.Space(3.5), t.Space(3.5)).TextColor(ink).Shrink(0)
	icon.Rotate(actionSpinnerTurn * motion.Value(c, func() float32 {
		return icon.Loop(key, actionSpinnerPerTurn, ui.Linear)
	}))
}

// busyActionButton draws an action pill in its in-flight face: the word the
// user pressed, preceded by the turning LoaderCircle, faded and unclickable
// for exactly as long as the work runs. It is the reference's
// `aria-busy` / `disabled={busy}` icon button
// (`src/extensions/ExtensionRow.tsx:313-330,345-372`) on a button that carries
// words, so a press that has not landed is visible rather than assumed.
func (a *App) busyActionButton(c *ui.Context, label, key string) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	button, ink := a.extensionActionFace(c, t.Space(7.5), t.Space(3.75), true)
	button.Gap(t.Space(1.25))
	button.Children(func() {
		a.actionSpinner(c, ink, key)
		ui.Text(c, label).FontSize(tokens.Body).FontWeight(600).TextColor(ink)
	})
	// `.extensions-action-button:disabled { opacity: 0.4; cursor: default;
	// pointer-events: none }` (`extensions.css:1369-1373`): the framework's
	// flag carries the pointer half, the cursor is the other one. The 0.4 no
	// longer rides on the whole subtree — `extensionActionFace` splits it into
	// 「墨淡到 40%，面走 `ControlDim` 并熄灯」（P13-s，§6.2b-6）。
	return button.Disabled(true).Cursor(ui.CursorDefault)
}

// switchControl draws one switch: the framework's Switch inside the recessed
// lens (base.css's `.settings-switch` with `--elev-track`), the groove the
// user looks into. The inset stack is added to the track the framework draws,
// so the whole toggle is graded by the glass stop.
func (a *App) switchControl(c *ui.Context, on *bool) ui.Element {
	return a.tokens(c).LensInset(ui.Switch(c, on))
}

// disabledSwitch dulls a switch the way `.settings-switch:disabled` did
// (`opacity: 0.6; cursor: wait`, `settings.css:1185-1188`): the framework's
// flag carries the pointer half and the press refusal, the flatness and the
// cursor are the two it does not paint. `wait` is the one cursor the framework
// does not expose, so a disabled switch wears the default arrow.
//
// P13-u（`docs/liquid-glass.md` §6.2b-6b 的规矩）：那个 `.6` 是**第二道淡**——
// 框架已经给 `Disabled` 的子树乘了 0.5（`ui/paint.go:257-259`），两道乘起来是
// 有效 0.3，开关轨道的台阶被压成半个的一半（实测浅色 247 → 220 而不是 231）。
// 槽没有受光边可熄：禁用的槽靠「整块退一半、内线各留一半」说话，所以这一臂
// 只留框架那一道，与 `lockedControl` 同一条算术。
func disabledSwitch(sw ui.Element) ui.Element {
	return sw.Disabled(true).Cursor(ui.CursorDefault)
}

// busySwitchControl draws the row's switch in its in-flight face: the same
// recessed groove as switchControl, with its thumb swapped for the turning
// LoaderCircle the reference put in its place (`settings-switch--loading`
// replaces `<span className="settings-switch__thumb"/>` with
// `<LoaderCircle className="extensions-spinner"/>`,
// `src/extensions/ExtensionRow.tsx:443-455`). It is a different picture from
// the resting switch and it is disabled, so a write already in flight cannot be
// asked again; the track keeps the state's own fill, so which way the switch
// was moving is still readable. `key` names the loop so two busy rows on one
// page turn independently.
func (a *App) busySwitchControl(c *ui.Context, on *bool, key string) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	track, ink := t.Border.Mix(t.Text, 0.15), tokens.TextSecondary
	if *on {
		track, ink = t.Accent, t.AccentText
	}
	sw := ui.SwitchBase(c, on).Size(t.Space(9), t.Space(5)).Radius(t.Space(2.5)).
		Center().Background(track)
	sw.Children(func() { a.actionSpinner(c, ink, key) })
	return disabledSwitch(tokens.LensInset(sw))
}

// segmentedControl draws one of the custom-tool form's segmented pickers
// (the reference's `.extension-custom-mode`). It is the same material the
// settings pages' picker draws (`control.Segmented`): the reference's stadium
// track (`border-radius: 999px`, `extensions.css:1752`) holding pill segments
// whose chosen one is a raised pane with the `--accent-edge` keyline
// (`.extension-custom-mode__item--active`, `extensions.css:1759`). The old port
// drew the framework's `ui.Segmented` here — a rounded rectangle with a
// darkened chosen segment — so the panel's picker and the settings pages'
// picker were two shapes for the same choice.
//
// The reference's `.extension-custom-mode` is written in literal px — a 3px
// track padding, a 2px gap and the 999px stadium (`extensions.css:1752-1759`)
// — so inside the extension panel the control's metrics are pinned to the
// panel's literal unit for the duration of the build: the shared body sizes the
// track and every segment from the context theme's `Spacing`/`Radius`, which
// follow the interface step, and a picker that shrank at `small` would be the
// one control in the panel that did. The pin is scoped to this call, so the row
// switches beside it keep following the step the way the reference's `--u`
// rows do.
func (a *App) segmentedControl(c *ui.Context, index *int, labels ...string) ui.Element {
	return a.segmentedControlKeyed(c, index, labels)
}

// Dynamic parameter choices keep their identity when the manifest reorders them.
func (a *App) segmentedControlKeyed(c *ui.Context, index *int, labels []string, keys ...[]string) ui.Element {
	saved := c.Theme()
	if a.cardUnit > 0 {
		pinned := *saved
		pinned.Spacing = a.cardUnit
		c.SetTheme(&pinned)
	}
	// The reference's own numbers: a 3px track padding (0.75 unit) and a 2px
	// gap (0.5 unit), both taken through `cardSpace` so they are the panel's
	// literal px inside `extensionMetrics` and the settings `--u` outside it.
	t := a.cardTheme(c)
	track := control.SegmentedIndex(c, a.tokens(c), index, labels, false, false, a.cardSpace(t, 0.75), a.cardSpace(t, 0.5), keys...)
	c.SetTheme(saved)
	return track
}

// row draws one row of a card: the label and its optional grey second line
// at the start and the control at the end, with the rule between rows inset
// to the label's own column. It reports whether the label was clicked — the
// label is a hit target that activates the row's control, as the framework's
// own Field does, so a switch row can be toggled from either end.
func (a *App) row(c *ui.Context, label, sublabel string, control func(), below ...func()) bool {
	return a.rowNote(c, label, sublabel, "", control, below...)
}

// rowState is row for a row whose second line is a **state** rather than a
// subtitle: the About page's update row says what the last check found in it
// (`已是最新` / `最新版本: v…` / `更新失败，重试？` / `此构建没有更新源`), and those
// four are not one kind of sentence. Only the ink differs — the label column,
// the floor, the control slot and the 1px line gap are `row`'s own, so a row
// that takes this helper has the same box it had before (§6.80).
func (a *App) rowState(c *ui.Context, label, sublabel string, ink rowInk, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Center, nil, rowFace{}, label,
		[]rowLine{{Text: sublabel, Ink: ink}}, "", control, below...)
}

// rowLine is one grey line under a row's label. Plain text is painted in
// the row's muted grey; a line may instead be several segments, each in a
// role's own colour — the sessions meta line, whose state word takes the strong
// rung while its session runs and the quiet grey once it has exited (P8-d:
// neither is the accent; §5's judgment is that "running" is a state, not a
// choice).
//
// Mono marks a line that is a *value* rather than prose: the reference sets a
// session row's working directory in its monospace stack
// (`.session-manager__cwd { font-family: "SF Mono", Menlo, … }`,
// `main:src/styles/settings.css:1272-1276`) because a path reads as data, where
// a hint reads as a sentence.
type rowLine struct {
	Text     string
	Segments []rowSegment
	Mono     bool
	// Ink is the rung a plain line is painted at. The zero value is the
	// subtitle rung every row has always had (`t.TextMuted`); a line that is a
	// **state word** rather than a subtitle takes one of the other two. A line
	// with Segments paints each segment's own role and ignores this.
	Ink rowInk
}

// rowInk is the rung a plain (`rowLine.Text`) second line is painted at.
//
// P13-dk（`docs/liquid-glass.md` §6.80）：更新行的状态字此前和它的副标题共用
// 一档墨 —— 「已是最新」「有新版可下」「更新失败」「此构建没有更新源」四种
// **不同的意思**，像素上一模一样（浅 101 / 深 94 级）。§6.26 已经为会话的 meta
// 行立过同一条判据：运行中 / 已退出 / 坏了是三档，坏了那一档借 `t.Warning`
// 「因为一次运行的失败是一个意思，而意思允许有色相」。这一族现在也有三档。
type rowInk uint8

const (
	// rowInkQuiet is the subtitle rung: the second line explains its row, it
	// does not report anything that changed. Zero value — every existing
	// `rowLine{Text: …}` keeps its ink.
	rowInkQuiet rowInk = iota
	// rowInkFact is for a line whose content is a value the program chose (the
	// version an update would install): a fact, one rung above a subtitle and
	// one below the row's own emphasis line.
	rowInkFact
	// rowInkWarning is for a line that reports something broke. It is a
	// meaning, so it takes a hue (§6.26 / §1 的「意思允许有色相」).
	rowInkWarning
)

// rowSegment is one piece of a row's meta line.
type rowSegment struct {
	Text string
	Role rowSegmentRole
}

// rowSegmentRole names the colour a segment is painted in, so a line can be
// built without a theme and painted with one.
type rowSegmentRole uint8

const (
	// rowSegmentTertiary is the quiet grey the old `.session-manager__meta`
	// line was drawn in: the screen size and the start time, and the state
	// word of a session that has exited.
	rowSegmentTertiary rowSegmentRole = iota
	// rowSegmentStrong is the strong ink the old `.session-state` used while the
	// session is running. P8-d 重定基线（§5 判据）：它曾是 `rowSegmentAccent`
	// （`t.Accent`），但「运行中」是一个持续状态，不是「哪个被选中 / 哪个是
	// 开的」——accent 不能回答「这是活的」。改中性后，运行中的那一段与已退出
	// 的 `rowSegmentTertiary` 仍然差两档，靠明度分层（§5 新补的那句）。
	rowSegmentStrong
	// rowSegmentWarning is the rung a session's state word takes when the
	// program ended *badly* (P13-cm). The reference's `.session-manager__meta`
	// knew only two colours, so this one has no CSS to copy: it comes from
	// §6.26's ruling instead — the failure of a run is a meaning, and a meaning
	// is allowed a hue (`t.Warning`, the same one the terminal surface's
	// resident note and the captured-output page's status line wear).
	//
	// It sits after `rowSegmentStrong` on purpose: the value is a rank in the
	// line, and a failed ending reads *louder* than a quiet fact but never as
	// the mark of a live session.
	rowSegmentWarning
)

// iconRow is rowAligned with the extension rows' leading package chip: the
// reference's `.extension-row__open` grid is `[icon | main]`, and the Detected
// list's rows are the same `.extension-row` as the connected ones. `detected`
// draws the chip as the reference's dashed ghost — a tool the machine has that
// nothing has connected.
func (a *App) iconRow(c *ui.Context, detected bool, label, sublabel string, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Center, func() { a.extensionIcon(c, detected) }, rowFace{}, label,
		[]rowLine{{Text: sublabel}}, "", control, below...)
}

// sessionChip is the sessions row's leading mark: the reference's
// `.settings-row__icon` (`settings.css:511-518`) — a 26u square with the
// `--radius-xs` corner carrying the `--icon-surface` plane and lucide's
// SquareTerminal at 16px in `--text-secondary` (`SessionsPage.tsx:186-187`,
// `<SquareTerminal size={16} strokeWidth={1.8}/>`). It is the *same* slot the
// integrations row fills with its package chip: the sessions page draws the
// shared `SettingsRow`, so its leading cell is the settings row's own, not the
// extension panel's package chip.
func (a *App) sessionChip(c *ui.Context) ui.Element {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	// base.css's `--icon-surface` is an alias of `--glass-control`
	// (`base.css:620`), so the chip's plane is the resting control rung.
	chip := ui.Box(c).Width(t.Space(6.5)).Height(t.Space(6.5)).Shrink(0).
		Radius(tokens.RadiusXS).Background(tokens.Control).Center()
	chip.Children(func() {
		ui.Icon(c, mustGlyph("square-terminal")).Size(t.Space(4), t.Space(4)).
			TextColor(tokens.TextSecondary)
	})
	return chip
}

// sessionIconButton is the sessions row's own trailing control: the
// reference's `.session-manager__icon-button` (`settings.css:1299-1310`) — a
// 28u square with a transparent face, a `1px` transparent keyline that lights
// to `--glass-control-edge` under the pointer, the `--radius-sm` corner and
// `--text-muted` ink. Hover lifts the fill to `--glass-control-hover` and the
// ink to `--text-primary`; a `--danger` button keeps the fill and swaps its ink
// to `--text-warning` (`:1336-1340`); a disabled one dims to the reference's
// 0.4. The reference's Play button may also carry the accent resume dot in its
// top-right corner (`:1319-1327`, `:203`), which is the only accent on the row.
func (a *App) sessionIconButton(c *ui.Context, glyphName, label string, danger, resumable, disabled bool, run func()) {
	tokens := a.tokens(c)
	t := c.Theme()
	button := ui.ButtonBase(c).Label(label).
		Size(t.Space(7), t.Space(7)).Border(1, ui.Transparent).
		Radius(tokens.RadiusSM).Background(ui.Transparent).Center().
		// P13-gv（`docs/liquid-glass.md` §6.169）：这一族的**每一枚**小格（30u 的
		// `extensionIconButton`、28u 的 `paramIconButton`、页头的 `headerButton`、
		// 弹层里的 `sheetActionButton`）都在 120ms 里走到悬停面，只有这一枚与
		// `sessionKillConfirm` 是**一帧跳过去**的（实测：`f0 == f60 == f400`，
		// 而 `pageButton` 的 `f0 ≠ f60`）。同一个指针动作在一个应用里两种速度，
		// 正是 §5 第一行「颜色过渡 120ms」要收掉的东西。
		Transition(ui.ElementTransition{Colors: true, Duration: theme.ColorStep})
	// P13-az：提示走 `tokens.Tip`（§6.20 20-3），不再是框架那枚反色片子。
	button = tokens.Tip(c, button, label)
	ink := t.TextMuted
	switch {
	case disabled:
		// P13-u：这一枚是**透明面**（没有面、没有边），所以它既不需要
		// `DimFace`（没有受光边可熄），也不需要产品自己那道 `.4`——那会让
		// 框架的 0.5 变成有效 0.2，图标墨只剩 24 级对比（P-5 的下限是 60）。
		// 一道淡，档位挂在控件根上：`lockedControl`。
		ink = tokens.Disabled
		button = lockedControl(button)
	case button.Pressed():
		// P13-q：同 28u 小格，第三档（`ControlPress` + `Scale(0.92)`）。
		button.Border(1, tokens.ControlEdge).Background(tokens.ControlPress)
		button.Scale(0.92, 0.92)
		tokens.LitFace(button)
		if danger {
			ink = t.Warning
		} else {
			ink = tokens.TextStrong
		}
	case button.Hovered():
		button.Border(1, tokens.ControlEdge).Background(tokens.ControlHover)
		// P13-p（`docs/liquid-glass.md` §6.2b-4 第 9-18 行）：这枚 28u 方格是
		// 这一族里唯一没戴 E1 那一对线的——`extensionIconButton`（30u，同文件
		// `:549`）、`paramIconButton`（28u，`parameditor.go:274`）、页头的
		// `headerButton`（28u，`settingsui.go:1199`）三块都戴了。同一个应用里
		// 同一枚悬停小格只有一种材质：外圈 `ControlEdge` 是它与行之间的线，
		// `LitFace` 补的是顶受光边与底对侧暗线，不画环、不画影。
		tokens.LitFace(button)
		if danger {
			ink = t.Warning
		} else {
			ink = t.Text
		}
	}
	button.Children(func() {
		ui.Icon(c, mustGlyph(glyphName)).Size(t.Space(3.5), t.Space(3.5)).TextColor(ink)
		if resumable {
			// The micro-badge that marks a running-but-detached session as
			// resumable (`.session-manager__resume-dot`): 5u in the button's own
			// top-right corner.
			//
			// P8-d 重定基线（§5 判据）：它曾是 `t.Accent`。它说的是「这条可以
			// 恢复」——一个动作的**机会**，既不是「这条被选中」，也不是「这个控件
			// 是开的」，更不属于主 CTA（§5 四类一个都不沾）。改 `TextSecondary`；
			// 召唤力交给位置（按钮自己的右上角）与按钮本身的 hover 档，不用色相。
			// P13-gz: the marker belongs to this button, including its transform.
			ui.Box(c).Absolute().Top(t.Space(0.75)).Right(t.Space(0.75)).
				Size(t.Space(1.25), t.Space(1.25)).Radius(t.Space(1.25)).
				Background(tokens.TextSecondary).PassThrough()
		}
	})
	if button.Clicked() && !disabled {
		run()
	}
}

// sessionKillConfirm is the reference's `.session-manager__kill-confirm`
// (`settings.css:1330-1347`): the row's destructive question as a 28u pill —
// `--text-warning` keyline and ink, the `--glass-raised-warm` plane, the
// `--glass-raised-shadow` lens, 9u of side padding, the caption step at 620 and
// the Trash2 mark at 13px before the word (`SessionsPage.tsx:216-229`). Hover
// lifts the plane to `--glass-raised-warm-hover`; a disabled question turns its
// crown off and keeps one fade (P13-aq: the reference's `.4` multiplied the
// framework's 0.5 — see `docs/liquid-glass.md` §6.2b-6).
func (a *App) sessionKillConfirm(c *ui.Context, label string, disabled bool, run func()) {
	tokens := a.tokens(c)
	t := c.Theme()
	pill := ui.ButtonBase(c).Label(label).
		MinHeight(t.Space(7)).Padding(0, t.Space(2.25)).Radius(999).
		Border(1, t.Warning).Background(tokens.RaisedWarm).
		AlignItems(ui.Center).Center().Gap(t.Space(1.25)).
		// P13-gv（§6.169）：同族的暖色面（电源确认带的 execute 键）与它上面那枚
		// 28u 小格都走 120ms，这一枚此前一帧跳过去。
		Transition(ui.ElementTransition{Colors: true, Duration: theme.ColorStep})
	// P13-az：同 `sessionIconButton`，提示是浮面（§6.20 20-3）。
	pill = tokens.Tip(c, pill, label)
	// `box-shadow: var(--glass-raised-shadow)`: the `--elev-2` contact cast plus
	// the lit top rim, the same pair the launcher's raised panes wear.
	pill.Shadow(0, t.Space(0.25), t.Space(0.5), 0, tokens.ShadowContact)
	// P13-ab（`docs/liquid-glass.md` §2/§3）：这枚药丸此前只有**顶**受光边
	// （`InsetShadow(0, 1, 0, 0, LensRim)`），没有对侧——与 P13-aa 修掉的电源
	// execute 键是同一种「半对」：一块只能看见光、看不见影的暖色抬升面，读起来是
	// 「贴了一张纸」。`LitFace` 把对侧补上（顶 `LensRim` / 底 `LensEdge`），影
	// **保留**（`--glass-raised-shadow` 是这块暖面的身份，§2 判据 2 管的是中性的
	// E1，不是它）。外圈警告色 `Border` 不动：那是「不可逆」的身份线，不是 E1 的边。
	//
	// 悬停臂只换面（`--glass-raised-warm-hover`），一对线于是四个状态共用——这正是
	// `internal/ci/litarm_contract_test.go` 要求的形状（臂外的共享一对）。
	// P13-aq：这一对的点亮曾经写在禁用臂**之外**，于是「问了也没有答案」的药丸仍然点着
	// 受光边（实测浅 243/244、深 19/23——顶端比面亮 1~4 级）。§3 说受光边是「这块面
	// 可以被碰」的记号，禁用时就该熄。褪色同样只留框架那一道 0.5：`.Opacity(0.4)`
	// 再乘一次 = 有效 0.2，字的对比实测从 161 掉到 **34**（深色 135 → 29）。
	if disabled {
		pill = pill.Cursor(ui.CursorDefault).Disabled(true)
	} else {
		tokens.LitFace(pill)
		// P13-gt · 第三档。这枚药丸此前只有静息与悬停两档（悬停把面抬到
		// `IconSurfaceHover`），按住的那一帧与悬停帧**逐像素相同**——「确认删除
		// 这个会话」最需要回手的那一下，墨上没有回答。同族的暖色面（电源确认的
		// execute 键）走的是同一条阶梯：按下时面退回**暖色族的静息面**
		// （`RaisedWarm`），一对线与影保留（这是块「能被碰的面」沉下去一档，
		// 不是沉进凹槽），几何收 0.92（同 `sessionIconButton`）。
		//
		// 为什么是 `RaisedWarm` 而不是 `ControlPress`：`ControlPress` 是**中性**
		// 阶梯的第三格（浅 0.032 黑 / 深 0.05 白），压在这块暖色面上会把它洗成
		// 一块灰——按下时就丢了「不可逆」的身份色相。暖色族自己的三档只有
		// `RaisedWarm` / `IconSurfaceHover` 两格，按下就是回到底下那一格。
		if pill.Pressed() {
			pill.Background(tokens.RaisedWarm)
			pill.Scale(0.92, 0.92)
		} else if pill.Hovered() {
			pill.Background(tokens.IconSurfaceHover)
		}
	}
	pill.Children(func() {
		ui.Icon(c, mustGlyph("trash")).Size(13, 13).TextColor(t.Warning)
		ui.Text(c, label).FontSize(tokens.Body).FontWeight(620).TextColor(t.Warning)
	})
	if pill.Clicked() && !disabled {
		run()
	}
}

// rowNote is row with one more line: a warning-coloured note under the
// description, for the one thing a row has to disclose rather than describe
// (the clipboard's privacy line). An empty note draws nothing.
func (a *App) rowNote(c *ui.Context, label, sublabel, note string, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Center, nil, rowFace{}, label, []rowLine{{Text: sublabel}}, note, control, below...)
}

// rowAligned is the row both helpers share; align places the control across
// the row's axis. lead draws the row's leading mark (nil for none) and face
// carries its own state decoration, both handed straight to cardRow.
func (a *App) rowAligned(c *ui.Context, align ui.Align, lead func(), face rowFace, label string, sublabels []rowLine, note string, control func(), below ...func()) bool {
	return a.rowTitle(c, align, lead, face, rowName{Text: label}, sublabels, note, control, below...)
}

// rowValue is rowAligned for a row whose *name* is a value rather than prose:
// a string the program chose for itself (a session's title, which a shell sets
// to `user@host: ~/where/you/are`), not a sentence this build wrote. The
// reference draws the two differently and so does this row — `.settings-row__
// label` wraps like any other label, while `.session-manager__row .settings-
// row__label` is `overflow: hidden; text-overflow: ellipsis; white-space:
// nowrap` (`main:src/styles/settings.css:1249-1260`) with the comment "a
// daemon-chosen session name ... has to truncate rather than wrap, or one row
// would grow and break the list's rhythm".
//
// The full string stays on the element, so the cut is in the paint and not in
// the data: `Tooltip` hands it back on hover, which the reference does not do
// (it has no second place to read a session's name) but this build's sessions
// page is the only such place, and a name nobody can read is a name nobody
// chose.
func (a *App) rowValue(c *ui.Context, align ui.Align, lead func(), face rowFace, label string, sublabels []rowLine, note string, control func(), below ...func()) bool {
	return a.rowTitle(c, align, lead, face, rowName{Text: label, Nowrap: true}, sublabels, note, control, below...)
}

// rowName is a row's emphasis line and how it behaves when it does not fit.
// Nowrap is for names that are values (see rowValue); prose keeps wrapping so a
// long translated sentence is never cut in half.
type rowName struct {
	Text   string
	Nowrap bool
}

// rowTitle draws the row's text column — the emphasis line, its grey lines and
// the note — and hands the state decoration to cardRow with it.
func (a *App) rowTitle(c *ui.Context, align ui.Align, lead func(), face rowFace, name rowName, sublabels []rowLine, note string, control func(), below ...func()) bool {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	return a.cardRow(c, align, lead, face, control, func() {
		// The label is the emphasis step (580, strong) and the grey second line
		// the body step (500, muted).
		label := ui.Text(c, name.Text).FontSize(tokens.Emphasis).FontWeight(580).TextColor(tokens.TextStrong)
		if name.Nowrap {
			// P13-az：截断名的全名提示（§6.20 20-3）——文本锚点，不是按钮。
			tokens.Tip(c, label.SingleLine().Ellipsis("\u2026"), name.Text)
		}
		for _, sublabel := range sublabels {
			if len(sublabel.Segments) > 0 {
				// A meta line is the old page's `.session-manager__meta`: a
				// wrapping flex row of its own facts, each a span with its own
				// colour, so the state word can be the accent while the size
				// and start time stay quiet. It is the *caption* step with
				// tabular figures, not the body step its label wears
				// (`.session-manager__meta { font-size: var(--text-caption);
				// font-variant-numeric: tabular-nums }`,
				// `main:src/styles/settings.css:1280-1286`), and its two gaps
				// are the reference's own `4u` rows / `10u` columns (`gap:
				// calc(var(--u)*4) calc(var(--u)*10)`, `:1285`).
				meta := ui.Row(c).FillWidth().Wrap().GapX(t.Space(2.5)).GapY(t.Space(1))
				meta.Children(func() {
					for _, segment := range sublabel.Segments {
						color := tokens.TextTertiary
						switch segment.Role {
						case rowSegmentStrong:
							color = tokens.TextStrong
						case rowSegmentWarning:
							// P13-cm: `c.Theme().Warning` — the palette member
							// `residentNote` and the output page's failed status
							// already draw with, rather than a second warm ink.
							color = t.Warning
						}
						ui.Text(c, segment.Text).FontSize(tokens.Caption).FontWeight(500).
							TextColor(color)
					}
				})
				continue
			}
			if sublabel.Text == "" {
				continue
			}
			// The plain second line is the reference's `.settings-row__sublabel`:
			// the body step at 500 in `--text-muted`, single-line so a long value
			// ellipsizes in the text column instead of growing the row
			// (`.session-manager__row .settings-row__label` / `__cwd` both
			// `nowrap`, `settings.css:1240-1244`). A *mono* line is a value
			// rather than prose — the reference sets a session's working
			// directory in its own monospace stack at the caption step
			// (`.session-manager__cwd`, `settings.css:1272-1276`).
			//
			// P13-bm：这句「ellipsizes」**不是**这一行自己写的——`SingleLine()` 只把
			// `maxLines` 钉成 1（`mygo/ui/element.go:1322`），记号是框架给的：
			// `mygo/internal/text/layout.go:81` 的 `Params.Ellipsis` 文档写着
			// 「`…` when empty」，`:1194-1206` 在 `MaxLines` 用尽时无条件补上它。
			// 所以名字那一行的 `.Ellipsis("\u2026")`（`:1136`）是把默认值又说了一遍，
			// 而这一行什么都不写也照样有记号。实测见
			// `rowlinecut_test.go`：文字列 233u、自然宽 511u 时尾端是低处的三个点，
			// 533u 时尾端是真正的字形。**别**把它换成 `Clip()`——那是真的硬切，
			// 而且会让整行折成三行。
			size := t.FontSize
			if sublabel.Mono {
				size = tokens.Caption
			}
			color := t.TextMuted
			switch sublabel.Ink {
			case rowInkFact:
				color = tokens.TextSecondary
			case rowInkWarning:
				color = t.Warning
			}
			e := ui.Text(c, sublabel.Text).FontSize(size).FontWeight(500).
				TextColor(color).SingleLine()
			if sublabel.Mono {
				e.Font(deepLinkValueFont)
			}
		}
		if note != "" {
			ui.Text(c, note).FontSize(t.FontSize).FontWeight(500).TextColor(t.Warning)
		}
	}, below...)
}

// factRow is a card row that reports a value instead of offering a control: the
// integration card's approval record (`ApprovalRecord` / `批准时间` / `清单摘要` /
// `磁盘摘要`) is a spec sheet of what the host recorded, not a list of things the
// user can change. Nothing on those rows is pressable — but they wore the plain
// settings row, whose emphasis step belongs to the **setting's name**
// (`rowTitle`: emphasis, 580, `TextStrong`) and whose second line is a subtitle
// (`t.TextMuted`, 500). On a record that is backwards: the *name* of the fact
// took the loud ink and the fact itself — the timestamp, the digest, the one
// thing the user opened the block to read — was the quietest text on the row.
//
// P13-ak 重定基线（`docs/liquid-glass.md` §6.10）：两档墨对调 —— 名字退到
// caption / `TextSecondary` 一档（它是值的标题，不是这一行的标题），值升到
// `rowTitle` 的 emphasis 档（`t.FontSize+1` / 580 / `TextStrong`）。**只换墨，
// 不换盒子**：行框架、44px 地板、分隔线、1px 行距、文字列都还是 `cardRow` 的
// 那一套，所以这一族四行没有一个像素的位置改变（行为契约零改动）。
//
// 与「禁用」（P13-s/t/u）不是一件事：禁用的是**一个不能用的控件**，淡一道；
// 只读的是**一个已经确定的答案**，它不淡，只是把重量交给答案本身。
func (a *App) factRow(c *ui.Context, name, value string) {
	tokens := a.tokens(c)
	a.cardRow(c, ui.Center, nil, rowFace{Inert: true}, func() {}, func() {
		ui.Text(c, name).FontSize(tokens.Caption).FontWeight(500).
			TextColor(tokens.TextSecondary).SingleLine()
		ui.Text(c, value).FontSize(tokens.Emphasis).FontWeight(580).
			TextColor(tokens.TextStrong).SingleLine()
	})
}

// codeRow is a card row whose label is a value rather than prose: the
// reference's `.settings-deep-link__value`, a sunken code chip that makes a
// `floter://` line read as data instead of copy (`DeepLinkRow.tsx:67,79`;
// `settings.css:1524-1535`). It is a `cardRow` with a different label column,
// so it keeps the same rule, counters and control slot as every other row.
// codeRow is the deep-link line: the value sits in a **code chip**, and the chip
// is the pane's own recess showing through the card — not a thin veil over the
// card's face.
//
// P13-bz 重定基线：旧填充是 `Background(tokens.Sunken)`。`Sunken` 是 0.17 的
// 浅色薄罩，压在卡片的面上只差 **4 级**（dark 卡 39 → 片 35），浅色档只差 2 级
// ——「凹陷」在像素上不存在。它以前看上去能过 `TestTheDeepLinkValueIsTheReferenceCodeChip`
// 只是因为那个 pin 跑在一张**混色帧**里（app 的 token 是深色、tester 的帧主题是
// 浅色），chip 与卡在那张帧里相差 34 级。本轮把内容页的面算成了不透明色，
// 混色帧消失，这个假绿才露出来。现在 chip 填的是**内容页自己**那个不透明色
// （`FaceOver(地, Sunken)`，与内容列同一个表达式）：dark 39 → **15**（−24 级）、
// light 230 → **246**（+16 级），它真的是一块挖穿的洞。
func (a *App) codeRow(c *ui.Context, value string, control func(), below ...func()) {
	focus := a.linkFocus(c, value)
	tokens := a.tokens(c)
	t := c.Theme()
	a.cardRow(c, ui.Center, nil, rowFace{}, control, func() {
		chip := ui.Box(c).FillWidth().MinWidth(0).Padding(t.Space(1), t.Space(2)).
			Radius(tokens.RadiusXS).
			Background(theme.FaceOver(tokens.Theme.Background, tokens.Sunken)).
			Border(1, tokens.ControlEdge)
		chip.Children(func() {
			text := ui.Text(c, value).Font(deepLinkValueFont).FontSize(tokens.Caption).
				TextColor(tokens.TextSecondary).Selectable().Bind(&focus.source)
			// Without the host copy action, the value itself is the useful
			// keyboard destination. Keep the usual action's tab order intact.
			// A capability refresh must not remove the current selection from
			// Tab order. Once it leaves, the normal action-only order resumes.
			if a.Actions.CopyText == nil || focus.source.Focused(c) {
				text.Focusable()
			}
			if a.Actions.CopyText == nil {
				focus.restore(c)
			}
		})
	}, below...)
}

// dataRow is a card row whose whole content is a **value the program chose** —
// the Detected list's `/usr/bin/ghost` under the tool's own name. It wore the
// plain settings row, so the path took the emphasis step (`t.FontSize+1`, 580,
// `TextStrong`) and measured exactly as loud as the tool's name above it
// (ink span 208 vs 208), 87 levels louder than the description (121): on a list
// of programs the *path* was the loudest thing on the card.
//
// P13-bt 重定基线（`docs/liquid-glass.md` §6.38）：a path is data, and this
// build already has one rung for data — `rowLine{Mono: true}`, the sessions
// page's working directory, which the reference sets in its monospace stack at
// the caption step (`.session-manager__cwd`, `settings.css:1272-1276`). This
// row draws its single line in exactly that rung: mono `Caption` / 500 /
// `TextMuted`. **只换墨，不换盒子**：仍是 `cardRow`（44px 地板、分隔线、1px 行距、
// 左右两端），行高与卡片高度逐像素不变；单行在行盒里居中（同 `keyValue` 的做法），
// 否则一行 9.9px 的字会贴在 44px 行盒的顶边。
func (a *App) dataRow(c *ui.Context, value string) {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	a.cardRow(c, ui.Center, nil, rowFace{Inert: true}, func() {}, func() {
		ui.Row(c).FillHeight().AlignItems(ui.Center).Children(func() {
			ui.Text(c, value).Font(deepLinkValueFont).FontSize(tokens.Caption).
				FontWeight(500).TextColor(t.TextMuted).SingleLine()
		})
	})
}

// labelRow is a card row that **names the block under it** rather than
// reporting a value or offering a control: the approval record's own title, and
// the `Permissions` line above its pills. Both wore the settings row's emphasis
// step, so the block's title measured exactly as loud as the answers it
// introduces (ink span 202 vs 202 for the values, 80 levels above the values'
// own names at 122) — a spec sheet that reads as five equally loud lines.
//
// P13-bt 重定基线（`docs/liquid-glass.md` §6.38）：a block label takes the rung
// this block already gives its own group labels (`Enforced` / `Disclosure`,
// `settingsui.go:2477`): `tokens.Caption` / 620 / `TextSecondary`. The label
// stays a step above the names it heads by **weight** (620 vs 500), and the
// hierarchy above it is carried by the row frame (the 44px row and its rule),
// not by a louder ink — the same "structure, not volume" judgment §2 makes for
// elevation. A `sublabel` (the no-record state's `None`) rides under it at the
// same size in the quiet rung (500 / `TextMuted`), so the two lines are told
// apart by weight, not by size. **只换墨，不换盒子**。
func (a *App) labelRow(c *ui.Context, label, sublabel string, below ...func()) {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	a.cardRow(c, ui.Center, nil, rowFace{Inert: true}, func() {}, func() {
		ui.Text(c, label).FontSize(tokens.Caption).FontWeight(620).
			TextColor(tokens.TextSecondary).SingleLine()
		if sublabel != "" {
			ui.Text(c, sublabel).FontSize(tokens.Caption).FontWeight(500).
				TextColor(t.TextMuted).SingleLine()
		}
	}, below...)
}

// rowFace is the decoration a card row wears on top of the shared frame: the
// 1px leading rule the extension rows light on hover, selection and the
// `floter://register` link, the face override a selected or broken row takes,
// and the inner keyline that goes with it. The zero value is a plain row — the
// card's own plane, no rule, no keyline.
type rowFace struct {
	// Bar is the leading rule's colour (the reference's
	// `.extension-row::before`); a transparent colour draws no rule, though the
	// 1px slot is always reserved so a row never reflows when the rule lights.
	Bar ui.Color
	// Face overrides the row's resting fill — a selected row's raised pane or a
	// broken row's warm one. FaceSet marks it as present, so a fully opaque
	// black can be told apart from "keep the card's plane".
	Face    ui.Color
	FaceSet bool
	// Inset is the 1px inner keyline a selected (accent) or broken (warning)
	// row draws (the reference's `inset 0 0 0 1px`), transparent for none.
	Inset ui.Color
	// Muted marked the half-made custom-shortcut row, which the reference
	// dimmed whole (`.settings-row--muted`, `settings.css:564-569`).
	//
	// P13-di（`docs/liquid-glass.md` §6.78）：这一格**故意不存在**了。那道
	// 0.62 是一个**乘法**，而这一行有两档墨：标题（`text`）与提示（`textMuted`）。
	// 实测浅色标题 `27 → 107`（正好落在 `textSecondary` 107 上）、提示
	// `128 → 173`（**低过阶梯最安静的一档** `textTertiary` 142）；深色
	// `237 → 159`（`textSecondary` 154）、提示 `133 → 99`（低过 `textTertiary` 110）。
	// 换任何因子都救不了这两档同时在线：`0.75` 时浅色标题 79／深色 187 都落在两档
	// **之间**，`0.50` 时 133／133 又落到另一对档之间。一个乘法不可能同时是两个
	// 档——所以「还没写完」这件事不该由整行的乘法来说。
	// Inert marks a row that reports something and offers nothing to press: the
	// read-only facts (`factRow`), the values the program chose (`dataRow`,
	// `keyValue`, `fixedRow`) and a block's own label (`labelRow`).
	//
	// P13-eu（`docs/liquid-glass.md` §6.116）：这种行**不答指针**。`cardRow` 此前对
	// 每一行都问 `Hovered()`/`Pressed()`，于是工具表单里那行只读的集成 id 在指针
	// 底下把面抬了**浅 24 / 深 30 级**（实测 229 → 205 / 39 → 69），读数与它下面
	// 那行真的能点的字段**逐字节相同**——指针在说「按我」，而按下去什么都不发生。
	// §3 说受光边是「这块面可以被碰」的记号；hover 面是同一句话在行上的说法。
	//
	// 只熄面，不动别的：墨、盒子、分隔线、`Selectable` 的取值都照旧（只读是一种
	// 结构，不是一种墨，§6.10/§6.84）。
	Inert bool
	// Floor is the row's own height floor, taller than the card's (a row whose
	// content is two lines of text plus a footer of metadata, i.e. the
	// reference's session row, keeps a taller box than a settings row's 44).
	// It is the row's number, not the card's: the card-level floor in
	// `cardGeometry` belongs to the card's *first* row only, while this one
	// follows the row it is handed with.
	Floor float32
	// Lit marks the face as an **E1 block** rather than a wash of paint on the
	// card: the row adds the two lines `theme.Tokens.LitFace` draws (one 1px
	// `LensRim` along its top, one 1px `LensEdge` along its bottom) and no cast
	// (`docs/liquid-glass.md` §2 判据 2 — an E1 block lies on the ground, it does
	// not float over it).
	//
	// P13-e（§6.4 第 4-1 行）：被 `floter://register` 链接命名的那一行此前只有
	// `Face` 一层平涂，与它脚下的 E1 分组卡只差一档 alpha——读起来是「卡里又刷
	// 了一块漆」，不是「卡上放着一个选中的块」。这是**新增**字段，零值 = 旧行为。
	Lit bool
}

// cardRow is the frame every row helper shares: the separator inset above the
// row (except the card's first), the height floor (the settings card's 44px,
// see rowFloorPx, or the title floor cardGeometry named for the card's first
// row), the leading rule's 1px slot, the label column and the trailing control
// slot. lead draws
// the row's own leading mark — the extension rows' package chip — so a row
// that needs one reuses the frame instead of copying it. face carries the
// row's own state decoration (the leading rule, a face override, an inner
// keyline). draw paints the label column's own content — a label with its
// second lines, or a code chip — so a new kind of label reuses the frame
// instead of copying it (which is how the deep-link chip drifted from the rows
// it sits between).
func (a *App) cardRow(c *ui.Context, align ui.Align, lead func(), face rowFace, control func(), draw func(), below ...func()) bool {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	inset := a.rowInset(t)
	first := a.cardRows == 0
	a.cardRows++
	floor := max(t.Space(11), rowFloorPx)
	if face.Floor > floor {
		// P13-ea（`docs/liquid-glass.md` §6.96）：行自己的地板。参考的会话行是
		// `min-height: 56px`（`main:src/styles/settings.css:1237-1244`），比设置
		// 卡的 44 高一档，因为那一行是两行字加一条脚注。**字面 px，不是
		// `--u * n`**：`u = 3.6` 时 `Space(14)` 只有 50.4，而这一行的内容自身
		// 就是 50.4 —— 拿 `Space(14)` 打地板等于没打（§6.95 记的那次误判）。
		floor = face.Floor
	}
	if first && a.cardTitleFloor > 0 {
		// The card's title row can wear the reference extension row's 66u
		// floor (`.extension-row { min-height: var(--extensions-row-height) }`,
		// `extensions.css:15-16,733-744`) while every row under it keeps the
		// settings card's own 44u.
		floor = a.cardTitleFloor
	}
	clicked := false
	ui.Column(c).FillWidth().Children(func() {
		if !first && !a.cardNoRules {
			// The row separator, inset on both sides and never running to
			// the card's frame; drawn above so only the card's first row
			// has none. A surface whose reference has no ruled rows (the
			// run form's `.extension-run-params` grid) suppresses it.
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).
				MarginX(inset).Shrink(0)
		}
		// The row is the reference's own: a 12u gap and a 9u padding pair, over
		// a floor of 44 (see rowFloorPx) or the step's own 44u, whichever is
		// taller. It is split in two so the leading rule can run the row's whole
		// height (the reference's `::before` spans `inset-block: 0`, padding
		// included) while the content keeps its inset: the 1px slot plus the
		// content's own `inset-1` padding add back up to the old inset, so a
		// plain row's content does not move.
		// A row becoming first loses its separator, not its input identity.
		row := ui.Row(c.Key("card-row-body")).FillWidth().AlignItems(ui.Stretch).MinHeight(floor)
		if face.FaceSet {
			row = row.Background(face.Face)
		}
		if face.Inset.A > 0 {
			row = row.InsetShadow(0, 0, 0, 1, face.Inset)
		}
		bar := face.Bar
		switch {
		case face.Inert:
			// P13-eu（§6.116）：没有可按的东西，就没有指针反馈。这一支写在最前——
			// `Pressed()`/`Hovered()` 对任何有状态的元素都为真，那与「这行能不能点」
			// 是两件事。
		case row.Pressed():
			row = row.Background(t.SurfacePressed)
		case row.Hovered():
			// The reference's `:hover:not(.extension-row--detected)` lights the
			// ladder's hover fill. P8-d 重定基线（§5 禁止侧「行 hover/按下」）：
			// 前导规则条曾经也在此抬到 `t.Accent`——那正是 §5 明写的禁止项。
			// 行 hover 只抬**面**（`SurfaceHover`），前导条的颜色由行自己的状态
			// （`face.Bar`）决定，不因指针而变。
			row = row.Background(t.SurfaceHover)
		}
		if face.Lit {
			// P13-e（`docs/liquid-glass.md` §6.4 第 4-1 行）：E1 = 面 + 顶受光边
			// `LensRim` + 底对侧 `LensEdge`，**不画投影**。与启动器选中块（§6.1
			// 第 1-2 行）、侧栏选中页（§6.2 第 2-2 行）、终端空态键帽（§6.3 第 3-8
			// 行）是同一个 `theme.Tokens.LitFace`——同一个函数、同一对数值，
			// 所以这个应用里每一块「被选中的面」都是同一个材质。压在 hover/press
			// 之后：指针换的是**面**的色号，不是这块面的层高。
			row = tokens.LitFace(row)
		}
		row.Children(func() {
			rule := ui.Box(c).Width(1).Shrink(0)
			if bar.A > 0 {
				rule.Background(bar)
			}
			ui.Row(c).Grow(1).MinWidth(0).Gap(t.Space(3)).AlignItems(ui.Stretch).
				Padding(t.Space(2.25), inset-1).Children(func() {
				if lead != nil {
					// The leading mark keeps its own size and its own centre line: an
					// icon never stretches (the framework's `stretches` skips icons), so
					// without a wrapper that says where it sits it would fall to the
					// row's top edge instead of staying beside the text.
					ui.Row(c).Shrink(0).AlignItems(align).Children(lead)
				}
				// The text column starts at the row's content top in every row (P13-bs):
				// a row with a second line is as tall as that block, a row without one
				// is as tall as its control, and the *block* is what used to be centred —
				// so a hint-less row's single line started lower than the same line in the
				// row above it. Stretching the column puts every label on one origin; the
				// control slot below keeps its own alignment.
				names := ui.Column(c).Grow(1).MinWidth(0).Gap(a.rowLineGap())
				names.Children(draw)
				if names.Clicked() {
					clicked = true
				}
				ui.Row(c).Shrink(0).Gap(t.Space(2)).AlignItems(align).Children(control)
			})
		})
		// What belongs *under* the row but inside the card: an unfolding
		// audit, a configuration form. It keeps the card's own rhythm and its
		// row counter, so a following row still draws its rule.
		//
		// P11-bf: it also keeps the row's *column*. Drawn as siblings of the
		// row, these started at the card's own edge — one whole inset (8u on
		// the integration card) left of every label above them — so a block's
		// caption read as the caption of the row above it and a warm banner
		// ran out to a frame no row touches. The row's content box begins one
		// rule slot plus its own padding in, which is exactly `inset`; content
		// that belongs to the row starts there too.
		if anyBelow(below) {
			ui.Column(c).FillWidth().Padding(0, inset).Children(func() {
				for _, extra := range below {
					if extra != nil {
						extra()
					}
				}
			})
		}
	})
	return clicked
}

// anyBelow reports whether any under-the-row callback was given, so a row
// without one draws no extra column at all.
func anyBelow(below []func()) bool {
	for _, extra := range below {
		if extra != nil {
			return true
		}
	}
	return false
}

// deepLinkValueFont is the monospace stack the reference gave
// `.settings-deep-link__value` (`settings.css:1530`).
const deepLinkValueFont = "SF Mono, Menlo, Monaco, Consolas, monospace"

// extensionIcon draws the extension row's package mark: the reference's
// `.extension-row__icon` (`extensions.css:871-889`), a 30u square chip with a
// 1px `--icon-edge` hairline, the `--ext-radius` corner and the `--icon-surface`
// plane, carrying lucide's `Package` at 17u in `--text-secondary`. A *detected*
// row — a tool the machine has that nothing has connected — wears the same chip
// as a ghost: the reference's dashed edge, no plane and `--text-tertiary` ink
// (`extensions.css:833-837`).
func (a *App) extensionIcon(c *ui.Context, detected bool) ui.Element {
	return a.packageChip(c, detected, false)
}

// packageChip is `extensionIcon` with the chosen mark: `chosen` paints the
// glyph in the accent. It is the **one** place an integration row may carry the
// accent (docs/raycast-design.md §6.3 第 2 条: the row's plane went neutral, so
// "this is the row the register link named" is said by the leading mark alone,
// exactly as §5 第 1 条 says it for the settings sidebar).
func (a *App) packageChip(c *ui.Context, detected, chosen bool) ui.Element {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	chip := ui.Box(c).Width(t.Space(7.5)).Height(t.Space(7.5)).Shrink(0).
		Border(1, tokens.ControlEdge).Radius(tokens.RadiusSM).
		Background(tokens.Control).Center()
	ink := tokens.TextSecondary
	if chosen {
		ink = t.Accent
	}
	if detected {
		chip = chip.BorderStyle(ui.BorderDashed).Background(ui.Transparent)
		ink = tokens.TextTertiary
	}
	chip.Children(func() {
		ui.Icon(c, mustGlyph("package")).Size(t.Space(4.25), t.Space(4.25)).TextColor(ink)
	})
	return chip
}

// stackedRow is a card row whose control wants the row's full width: the
// label and its grey second line above, the control under them, as the
// reference's `.settings-row--stacked` did for a range or a segmented row
// (the two controls wider than a trailing slot can hold). It draws the same
// rule above itself as any other row, through the same card-row counter, so a
// stacked row and a trailing row sit in one card without saying which is
// last.
func (a *App) stackedRow(c *ui.Context, label, sublabel string, control func()) {
	tokens := a.tokens(c)
	t := c.Theme()
	inset := a.rowInset(t)
	first := a.cardRows == 0
	a.cardRows++
	ui.Column(c).FillWidth().Children(func() {
		if !first {
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).MarginX(inset).Shrink(0)
		}
		// P13-ev（`docs/liquid-glass.md` §6.117）：这一列此前对**三对**都只问一个数
		// ——`Gap(t.Space(1))`，于是「标题 → 它的解释」与「解释 → 控件」读同一条缝。
		// 间距不属于区域，属于它两侧那一对（§4.3）：标题和它的灰线是**一句**，
		// 控件是**一件可以按的面**。前者取这一行自己的行内缝（`rowLineGap()`，与
		// 每一条 trailing 行同值），后者抬一档（`Space(2)`）。
		// Becoming the first row removes only the separator, not this body.
		ui.Column(c.Key("stacked-row-body")).FillWidth().Gap(t.Space(2)).Padding(t.Space(2.25), inset).Children(func() {
			// The label and its grey line keep the same steps as any trailing
			// row: the reference's `.settings-row__label` is `--text-emphasis`
			// and `.settings-row__sublabel` is `--text-body` for both shapes
			// (`settings.css:520-546`), and `SettingsRow` never demotes a
			// stacked row (`SettingsRows.tsx:104-135`).
			ui.Column(c).FillWidth().Gap(a.rowLineGap()).Children(func() {
				ui.Text(c, label).FontSize(tokens.Emphasis).FontWeight(580).TextColor(tokens.TextStrong)
				if sublabel != "" {
					ui.Text(c, sublabel).FontSize(t.FontSize).TextColor(t.TextMuted)
				}
			})
			control()
		})
	})
}

// sectionWithAction is a section whose title row carries the group's own
// action at its end: the reference pane's "Restore defaults" idiom
// (`.settings-section__heading-actions`).
func (a *App) sectionWithAction(c *ui.Context, title, hint, actionLabel string, run func(), fn func()) {
	a.sectionWithControls(c, title, hint, func() {
		if run != nil {
			a.action(c, actionLabel, run)
		}
	}, fn)
}

// sectionWithControls is sectionWithAction for a group whose heading carries
// more than one control: the reference's Sessions heading put both the sort
// toggle and the refresh key on that line (`SessionsPage.tsx:116-135`). The
// controls draw into the heading's own action slot, in the order given, so
// they share its bottom alignment and its 12u gutter from the title.
func (a *App) sectionWithControls(c *ui.Context, title, hint string, controls func(), fn func()) {
	a.sectionGroup(c, title, func() {
		a.sectionHeading(c, title, controls)
		fn()
		a.sectionHint(c, hint)
	})
}

// actionRow is a card row with no label: a full-width band that draws the
// row's separating hairline and padding and lets control fill it. The local-
// tool form's content-action row (a cluster of buttons, no label of its own)
// is its one caller, so the card's row rhythm — the rule above every row but
// the first — stays identical to the labelled rows beside it.
func (a *App) actionRow(c *ui.Context, control func()) {
	tokens := a.tokens(c)
	t := c.Theme()
	inset := a.rowInset(t)
	first := a.cardRows == 0
	a.cardRows++
	ui.Column(c).FillWidth().Children(func() {
		if !first {
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).MarginX(inset).Shrink(0)
		}
		ui.Column(c).FillWidth().Padding(t.Space(2.25), inset).Children(control)
	})
}

// action draws the right-aligned accent text action the reference's rows use
// ("Edit…", "Restore defaults"): the accent spent on text, never on a fill. It
// returns the action's own element so a test can read its box.
func (a *App) action(c *ui.Context, label string, clicked func()) ui.Element {
	return a.actionWithGlyph(c, "", label, clicked)
}

// actionWithGlyph is action with the reference's optional leading mark: the
// `.settings-reset__icon` slot the deep-link copy control filled with lucide's
// Copy / Check at 13px (`DeepLinkRow.tsx:47-58`, `SettingsRows.tsx:200-216`).
// An empty name draws the text-only action every other caller wants.
//
// The box is the reference's `.settings-reset` itself, not an approximation of
// it (`settings.css:682-730`): a 24u floor, a 5u gap, 2u of side padding, a
// `--radius-xs` corner, a transparent fill, and a hover/press pair that moves
// the *ink* (and, on press, the element's own scale) rather than painting a band
// behind it. The old port padded the row
// by 4u/6u, rounded it to `--radius-sm` and filled it on hover, so the same
// words stood 4u taller and turned into a control the reference never drew.
//
// P8-d 重定基线（§5 判据）：图标与文字都曾是 `t.Accent`。一个可点的文字动作
// 不是「哪个被选中 / 哪个是开的」——accent 在那里回答的是「这条重要/能点」，
// 正是 §5 禁止的那一类。静息 `TextSecondary`、hover `t.Text`：只抬**墨的明度**，
// 不抬色相，与 P8-b 录制器静息 hover 的处理同构（`shortcutRecorder`）。
func (a *App) actionWithGlyph(c *ui.Context, glyphName, label string, clicked func(), unavailable ...bool) ui.Element {
	t := c.Theme()
	tokens := a.tokens(c)
	disabled := len(unavailable) > 0 && unavailable[0]
	row := ui.Row(c).Focusable().AlignItems(ui.Center).MinHeight(t.Space(6)).
		Padding(0, t.Space(0.5)).Radius(tokens.RadiusXS)
	if disabled {
		row.Disabled(true).Cursor(ui.CursorDefault)
	}
	if glyphName != "" {
		row.Gap(t.Space(1.25))
	}
	// P13-cq（`docs/liquid-glass.md` §6.60）：这条动作此前**一个状态两个旋钮**——P7-gk 照抄了
	// 参考 CSS 的两个不透明度（`settings.css:713-721` 的 `.78` / `.6`），P8-d 又把墨抬了一档
	// （`TextSecondary → Text`），于是 hover 的抬墨被 0.78 吃回一半，而 press 先把墨**退回**
	// `TextSecondary` 再乘 0.6——按下那一帧比静息还淡（实测离面 20673 对 55494 的平方和，
	// 浅色 `{164,164,167}` 对 `{109,109,114}`）。合约的说法是**一个意图一道**（§6.59）：
	// 抬墨就是 hover 的全部，不透明度两道都删掉。
	//
	// **第三档换的是几何，不是墨**（§5 + `sidebarRow` 的同一句，`settingsui.go:1419`）：
	// 这条动作没有面（P7-gk 专门把底带删了），所以按下取 §5 的 `Scale(0.986)`——
	// 与启动器结果行（`launcher.go:2511`）同一句话。布局盒不动，命中框不动。
	ink := tokens.TextSecondary
	switch {
	case disabled:
		ink = tokens.Disabled
	case row.Pressed():
		ink = t.Text
		row.Scale(0.986, 0.986)
	case row.Hovered():
		ink = t.Text
	}
	row.Children(func() {
		if glyphName != "" {
			if glyph, ok := launcher.Glyph(glyphName); ok {
				ui.Icon(c, glyph).Size(t.Space(3.25), t.Space(3.25)).TextColor(ink)
			}
		}
		ui.Text(c, label).FontSize(t.FontSize).TextColor(ink)
	})
	if !disabled && row.Clicked() {
		clicked()
	}
	return row
}

// hairlineRule draws the settings panel's two separators — the header's rule
// against the body and the sidebar's rule against the content pane — as **one
// solid weak hairline** (`docs/raycast-design.md` §1/§6.1).
//
// P8-b 重定基线：旧 base.css 的 `--hairline-fade` 是两段渐隐（lead 0.12 / solid
// 0.76，竖排 0.1 / 0.8，颜色取强发丝 `Hairline`）→ 合约下分隔就是一条
// `HairlineSoft`，两端与中段同色、不渐隐。渐隐属于旧的液态玻璃语言（§1 的
// 低光泽上限之外还有「线属于材质」这层暗示）；Raycast 的分隔是一条**结构线**，
// 它说的是「这里是两段」，所以它不溶解、也不借用卡片的强边。vertical draws it
// top-to-bottom for the sidebar's column separator.
func (a *App) hairlineRule(c *ui.Context, vertical bool) {
	tokens := a.tokens(c)
	if vertical {
		ui.Column(c).FillHeight().Width(1).Background(tokens.HairlineSoft)
		return
	}
	ui.Row(c).FillWidth().Height(1).Background(tokens.HairlineSoft)
}
