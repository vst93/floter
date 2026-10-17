package settingsui

import (
	"github.com/egoist/mygo/ui"

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
// `--ext-radius` (= `--radius-sm`) corner and 620 on the caption step.
func (a *App) healthStatusChip(c *ui.Context, copy i18n.Settings, status string) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	plane, keyline, ink := healthStatusFace(tokens, t, copy, status)
	chip := ui.Row(c).Shrink(0).Padding(t.Space(0.5), t.Space(2)).
		Radius(tokens.RadiusSM)
	if plane.A > 0 {
		chip = chip.Background(plane)
	}
	if keyline.A > 0 {
		chip = chip.InsetShadow(0, 0, 0, 1, keyline)
	}
	// A healthy or warm chip keeps the reference's `--elev-0` lens on top of
	// its keyline (the reference composes `var(--elev-0), inset 0 0 0 1px …`);
	// the transparent `unknown` face carries neither.
	if statusWearsLens(copy, status) {
		tokens.Lens(chip)
	}
	chip.Children(func() {
		ui.Text(c, status).FontSize(tokens.Caption).FontWeight(620).TextColor(ink)
	})
	return chip
}

// healthStatusFace resolves the three colours one status word wears. The word
// is localized (the shell hands the row `copy.IntegrationsHealthy` &c.), so the
// comparison is against the copy's own spelling — the same rule the row already
// followed to tint the word.
func healthStatusFace(tokens theme.Tokens, t *ui.Theme, copy i18n.Settings, status string) (plane, keyline, ink ui.Color) {
	switch status {
	case copy.IntegrationsHealthy:
		return tokens.RaisedQuiet, tokens.AccentEdge, tokens.TextStrong
	case copy.IntegrationsDegraded, copy.IntegrationsUnhealthy:
		return tokens.RaisedWarm, t.Warning, t.Warning
	default:
		// `unknown` (and any word this build cannot place): transparent, muted.
		return ui.Transparent, ui.Transparent, t.TextMuted
	}
}

// statusWearsLens reports whether a status chip draws the reference's `--elev-0`
// lens, which it composes only where it also draws a keyline: the two chip
// faces and never the transparent `unknown` one.
func statusWearsLens(copy i18n.Settings, status string) bool {
	switch status {
	case copy.IntegrationsHealthy, copy.IntegrationsDegraded, copy.IntegrationsUnhealthy:
		return true
	}
	return false
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
// An *enforced* permission is the recognisable other half of the review list's
// shape difference — the raised-quiet pane and the accent keyline — so "what
// floter refused" keeps the same face in the record as in the audit.
func (a *App) approvalPermissionPills(c *ui.Context, language string, permissions []string, enforced bool) ui.Element {
	t := a.cardTheme(c)
	tokens := a.tokens(c)
	list := ui.Row(c).FillWidth().Wrap().GapX(t.Space(1.5)).GapY(t.Space(2))
	list.Children(func() {
		for _, permission := range permissions {
			permission := permission
			pill := ui.Row(c).Shrink(0).Padding(t.Space(1), t.Space(2)).
				Radius(tokens.RadiusXS)
			if enforced {
				pill = pill.Background(tokens.RaisedQuiet)
				pill.InsetShadow(0, 0, 0, 1, tokens.AccentEdge)
			} else {
				pill = pill.Border(1, tokens.ControlEdge).Background(tokens.Control)
			}
			ink := tokens.TextSecondary
			if enforced {
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
