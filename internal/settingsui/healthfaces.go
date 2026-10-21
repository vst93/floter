package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/theme"
)

// The integrations page's health / approval blocks, drawn at the reference's
// own faces. These are the pieces `main:src/styles/extensions.css` names and
// this port had been rendering as plain card rows:
//
//	.extension-health__status        the status chip (healthy / degraded /
//	                                 unhealthy / unknown), four faces
//	.extension-health__probe         a compact probe line: an 11px Check/X
//	                                 mark, a *monospace* id, a right-aligned
//	                                 duration — not a ruled card row
//	.extension-health__failure       the warm failure card, 2px warning edge
//	.extension-approval-record__permission
//	                                 the approved-permission pill family
//	.extension-approval-record__stale
//	                                 the changed-manifest banner
//
// The handoff between the two registers is what a reviewer missed: the port
// read every one of these as "a row with a trailing word", which put the probe
// ids in the body emphasis register and flattened four chip faces into one
// grey sentence.

// healthStatusChip draws the reference's `.extension-health__status`
// (`extensions.css:2184-2188`): a caption chip whose *plane* carries the
// verdict. A `healthy` run wears the raised-quiet pane with the accent keyline
// and the strong ink; `degraded` and `unhealthy` take the warm raised pane with
// the warning keyline and the warning ink; `unknown` is transparent with the
// muted ink, because "nothing has run yet" is not a state to shout about.
//
// The chip's geometry is the reference's: a `2px 8px` padding pair, the
// `--ext-radius` (= `--radius-sm`) corner and the caption step at 600 (§6.1
// 第 8 条 puts the weight cap there: 字重 ≤600).
//
// P8-b 重定基线：卡片的透镜已经从设置页的分组卡与侧栏行上去掉（§6.1 第 1/3
// 条），chip 是同一族控件，所以它也不再把 `--elev-0` 叠在发丝线上——一圈
// `ControlEdge` 已经说清了「这是一个 chip」。低光泽的合约下，一个 4px 高的
// 药丸不需要内侧高光（§1）。
func (a *App) healthStatusChip(c *ui.Context, copy i18n.Settings, status, raw string) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	plane, keyline, ink := healthStatusFace(tokens, t, copy, status, raw)
	chip := ui.Row(c).Shrink(0).Padding(t.Space(0.5), t.Space(2)).
		Radius(tokens.RadiusSM)
	if plane.A > 0 {
		chip = chip.Background(plane)
	}
	if keyline.A > 0 {
		chip = chip.InsetShadow(0, 0, 0, 1, keyline)
	}
	chip.Children(func() {
		ui.Text(c, status).FontSize(tokens.Caption).FontWeight(600).TextColor(ink)
	})
	return chip
}

// healthStatusFace resolves the three colours one status word wears. The
// **raw** status enum decides — the reference composes the chip's modifier
// class from the report itself (`.extension-health__status--${status}`,
// `ExtensionsPanel.tsx:2924`) — and the localized word is only the fallback for
// a caller that has nothing else. The word is a *display* string: comparing it
// meant a translation change, or a status outside the four-word ladder, dropped
// a chip to the transparent `unknown` face without a word of complaint.
//
// P8-b 重定基线（自加第 12 条）：`healthy` 曾是 `RaisedQuiet` + `AccentEdge`
// （一圈 accent 环），那是 §5 三处之外的 accent。合约下环只有两档，所以健康的
// 环降到 `ControlEdge`、面降到 `Control`，而 `degraded`/`unhealthy` 的暖面
// （系统语义色）保留、只把 `t.Warning` 的环换成 `ControlEdge`（§6.1 第 8 条
// 的同一处理）。状态靠**面与字色**说，不靠彩色描边。
func healthStatusFace(tokens theme.Tokens, t *ui.Theme, copy i18n.Settings, status, raw string) (plane, keyline, ink ui.Color) {
	switch healthKindOf(copy, status, raw) {
	case healthHealthy:
		return tokens.Control, tokens.ControlEdge, tokens.TextStrong
	case healthDegraded, healthUnhealthy:
		return tokens.RaisedWarm, tokens.ControlEdge, t.Warning
	default:
		// `unknown` (and any word this build cannot place): transparent, muted.
		return ui.Transparent, ui.Transparent, t.TextMuted
	}
}

// healthKind is which of the reference's four faces a chip wears.
type healthKind int

const (
	healthUnknown healthKind = iota
	healthHealthy
	healthDegraded
	healthUnhealthy
)

// healthKindOf reads the raw status first and the printed word second: a row
// the shell built carries `HealthStatus`, and a caller (or a stored row) that
// has only the word still gets the face it names.
func healthKindOf(copy i18n.Settings, status, raw string) healthKind {
	switch raw {
	case extensions.HealthHealthy:
		return healthHealthy
	case extensions.HealthDegraded:
		return healthDegraded
	case extensions.HealthUnhealthy:
		return healthUnhealthy
	case extensions.HealthUnknown:
		return healthUnknown
	}
	switch status {
	case copy.IntegrationsHealthy:
		return healthHealthy
	case copy.IntegrationsDegraded:
		return healthDegraded
	case copy.IntegrationsUnhealthy:
		return healthUnhealthy
	}
	return healthUnknown
}

// healthProbeRow draws one probe line the way `LauncherResults`' sibling
// extension block did: a 4px-vertical-padded flex row with an 11px Check or X
// mark, the probe id in the *reference's* monospace (`ui-monospace, monospace`,
// `extensions.css:2197`), and the duration pushed to the trailing edge in the
// muted caption step (`__probe-duration`, `:2198`).
//
// The mark's ink is the reference's own split (`:2195-2196`): `--text-strong`
// for a pass — not the accent the port reached for, which made a passing check
// look like a selected row — and `--text-warning` for a failure. The mark is a
// literal 11px glyph (`<Check size={11}/>` / `<X size={11}/>`,
// `ExtensionsPanel.tsx:2954-2958`), so it does not scale with the interface
// step the way the id and duration beside it do.
func (a *App) healthProbeRow(c *ui.Context, probe HealthProbe) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	mark, ink := "check", tokens.TextStrong
	if !probe.Passed {
		mark, ink = "close", t.Warning
	}
	row := ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).
		Padding(t.Space(1), 0)
	row.InsetShadow(0, -1, 0, 0, tokens.Hairline)
	row.Children(func() {
		if glyph, ok := launcher.Glyph(mark); ok {
			ui.Icon(c, glyph).Size(11, 11).TextColor(ink)
		}
		ui.Text(c, probe.ID).Font(probeRowFont).FontSize(tokens.Caption).
			FontWeight(500).TextColor(tokens.TextSecondary).SingleLine().Grow(1)
		ui.Text(c, healthDuration(probe.DurationMs)).FontSize(tokens.Caption).
			FontWeight(500).TextColor(t.TextMuted).Shrink(0).TextAlign(ui.End)
	})
	return row
}

// probeRowFont is the reference's probe-id stack: `ui-monospace, monospace`
// (`extensions.css:2197`) — a probe id is an API name, not prose.
const probeRowFont = "ui-monospace, SF Mono, Menlo, Monaco, Consolas, monospace"

// healthFailureCard draws the reference's `.extension-health__failure`
// (`extensions.css:2200-2203`): a warm raised pane under a 1px warning keyline
// with a heavier 2px warning rule on its leading edge and the `--ext-radius`
// corner. Inside, the probe's id is the line's strong word, its exit code the
// muted parenthetical, stderr the secondary pre-wrap block, and a retryable
// failure a warm `--retryable` chip.
func (a *App) healthFailureCard(c *ui.Context, copy i18n.Settings, probe HealthProbe) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	card := ui.Column(c).FillWidth().Gap(t.Space(1)).
		Padding(t.Space(1.5)).Radius(tokens.RadiusSM).
		Background(tokens.RaisedWarm)
	card.InsetShadow(0, 0, 0, 1, t.Warning)
	card.InsetShadow(-2, 0, 0, 0, t.Warning)
	card.Children(func() {
		head := ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).Wrap()
		head.Children(func() {
			ui.Text(c, probe.ID).FontSize(tokens.Caption).FontWeight(620).
				TextColor(tokens.TextStrong)
			ui.Text(c, copy.IntegrationsHealthExitCode(healthExitCode(probe.ExitCode))).
				FontSize(tokens.Caption).TextColor(t.TextMuted)
			if probe.Retryable {
				chip := ui.Row(c).Shrink(0).Padding(t.Space(0.5), t.Space(1)).
					Radius(tokens.RadiusSM).Background(tokens.RaisedWarm)
				chip.InsetShadow(0, 0, 0, 1, t.Warning)
				chip.Children(func() {
					ui.Text(c, copy.IntegrationsHealthRetryable).
						FontSize(tokens.Caption).TextColor(t.Warning)
				})
			}
		})
		if probe.Stderr != "" {
			ui.Text(c, probe.Stderr).FontSize(tokens.Caption).
				TextColor(tokens.TextSecondary).Wrap()
		}
	})
	return card
}

// approvalPermissionPills draws the recorded permissions the reference way
// (`extensions.css:2149-2168`): each id a `4px 8px` pill on the icon-surface
// plane with a `--glass-control-edge` hairline and the `--radius-xs` corner.
//
// P8-b 重定基线：两级都是**中性**面。旧 enforced = `AccentTintQuiet.Over(
// RaisedQuiet)` + `AccentEdge` inset，与「已声明」靠色相分开——那是旧 glass
// 语言把 accent 当「选中」用的写法，而合约 §5 只允许 accent 落在三处（侧栏前导
// 图标、主 CTA、焦点环），权限审计不属于其中任何一处（§6.1）。两级现在靠
// **线**分开：enforced 是 `Control` + 一圈 `ControlEdge` keyline（一个实心控件），
// 声明的只有同一圈发丝（一个空面），字色差一阶。
func (a *App) approvalPermissionPills(c *ui.Context, language string, permissions []string, enforced bool) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	list := ui.Row(c).FillWidth().Wrap().GapX(t.Space(1.5)).GapY(t.Space(2))
	list.Children(func() {
		for _, permission := range permissions {
			permission := permission
			pill := ui.Row(c).Shrink(0).Padding(t.Space(1), t.Space(2)).
				Radius(tokens.RadiusXS).
				Border(1, tokens.ControlEdge).Background(tokens.Control)
			ink := tokens.TextSecondary
			if enforced {
				// The only thing that tells the two tiers apart now is the ink
				// step and the tier heading above each group: both pills wear one
				// neutral control face (see the note above).
				ink = t.Text
			}
			pill.Children(func() {
				ui.Text(c, i18n.PermissionLabel(language, permission)).
					FontSize(tokens.Caption).TextColor(ink)
			})
		}
	})
	return list
}

// approvalStaleBanner draws the reference's `.extension-approval-record__stale`
// (`extensions.css:2171-2179`): the warm raised pane with a 2px warning leading
// rule and the `--ext-radius` corner, carrying the sentence at the secondary
// caption step. The port drew it as a card row with an empty label and a lone
// `!`, which read as an unnamed row rather than as the block's own warning.
func (a *App) approvalStaleBanner(c *ui.Context, copy i18n.Settings) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	banner := ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(3)).
		Radius(tokens.RadiusSM).Background(tokens.RaisedWarm)
	banner.InsetShadow(-2, 0, 0, 0, t.Warning)
	banner.Children(func() {
		ui.Text(c, copy.ApprovalRecordChanged).FontSize(tokens.Caption).
			TextColor(tokens.TextSecondary).Wrap()
	})
	return banner
}
