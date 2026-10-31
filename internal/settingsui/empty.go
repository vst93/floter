package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
)

// emptyState is the reference's one empty region, `SettingsEmpty`
// (`SettingsRows.tsx:218-244`; `settings.css:633-676,1393`): one quiet glyph,
// the state in one line, the grey why underneath with its 320u measure, and an
// optional inline entry. Nothing here is chrome — an empty region is a
// sentence, not a card — so it is presentation only: every page keeps owning
// its own words, its own mark and its own way out.
//
// P8-b 重定基线（合约 §4.7）：主句从 body + `TextSecondary` 抬到
// **emphasis + `TextStrong`**，图标从 `TextTertiary`@0.72 抬到 `TextSecondary`
// （「图标次文字色」）。旧值是「一条灰句」，读起来像错误提示；合约的空态
// 是「一句陈述」。无底板、无 accent——这两样本就没有，且仍然不能有。
//
// floor is the region's own floor. The reference's list placeholder is 92u
// (`.settings-empty`), and a page whose empty region *is* its whole body
// overrides it with the taller 160u (`.session-manager > .settings-empty`).
// Zero means the 92u list floor.
//
// iconSize is the glyph's own size, because the reference's primitive does not
// size it: the caller passes the node. Sessions and the custom-shortcuts group
// pass 22px, the integrations list passes 20px. Zero means 22u.
func (a *App) emptyState(c *ui.Context, iconName, title, hint string, iconSize, floor float32, action func()) ui.Element {
	tokens := a.tokens(c)
	t := c.Theme()
	if floor <= 0 {
		floor = t.Space(23) // 92u, the reference's list-placeholder floor
	}
	if iconSize <= 0 {
		iconSize = t.Space(5.5) // 22u, the sessions/custom-shortcuts mark
	}
	// `place-content: center; justify-items: center` is the column's own
	// centring on both axes; `text-align: center` is the copy's.
	region := ui.Column(c).FillWidth().MinHeight(floor).
		Gap(t.Space(2)).Padding(t.Space(6)).Center()
	region.Children(func() {
		// P13-aj（`docs/liquid-glass.md` §4.3）：标记与它引入的那句话是**一块**。
		// 均匀的 8u 让三个部件读成三个平级的邻居——实测墨上间隙 12px（标记→
		// 句子）对 13px（句子→理由），两者几乎相等，一枚 22u 的标记因此悬在
		// 句子上方「自己一格」。这一对内收一档到 4u：墨上 12 → **8px**。
		// 只动这一对；句子与理由之间仍是区域的 8u。
		head := ui.Column(c).Center().Gap(t.Space(1))
		head.Children(func() {
			if glyph, ok := launcher.Glyph(iconName); ok {
				// `.settings-empty__icon`: the second ink step, quiet beneath the
				// sentence it introduces (§4.7「图标次文字色」).
				ui.Icon(c, glyph).Size(iconSize, iconSize).
					TextColor(tokens.TextSecondary)
			}
			// `.settings-empty__title`: the emphasis step in the strong ink —
			// one statement, not a grey line that reads as a complaint (§4.7).
			ui.Text(c, title).FontSize(tokens.Emphasis).FontWeight(580).
				LineHeight(1.4).TextColor(tokens.TextStrong).TextAlign(ui.Center)
		})
		if hint != "" {
			// `.settings-empty__hint`: a 320u measure, `--text-caption` in
			// `--text-tertiary`.
			ui.Text(c, hint).FontSize(tokens.Caption).LineHeight(1.5).
				MaxWidth(t.Space(80)).TextColor(tokens.TextTertiary).TextAlign(ui.Center)
		}
		if action != nil {
			// `.settings-empty__action`: inline, 2u above it.
			ui.Column(c).Margin(t.Space(0.5), 0, 0, 0).Children(action)
		}
	})
	return region
}

// sessionsEmptyFace is the Sessions page's empty state as a pure choice, the
// reference's `error ? ... : loading ? ... : ...` three-way
// (`SessionsPage.tsx:142-167`). The shell's session list is in-process and
// synchronous, so this build can only reach the resting arm; the other two are
// kept here so the marks and the words cannot drift from the old dictionary,
// and are exercised by the mapping's own test.
type sessionsEmptyFace struct {
	Icon  string
	Title string
	Hint  string
}

func sessionsEmptyFaceFor(copy i18n.Settings, failed, loading bool) sessionsEmptyFace {
	switch {
	case failed:
		return sessionsEmptyFace{Icon: "alert", Title: copy.SessionsError, Hint: copy.SessionsRefreshHint}
	case loading:
		return sessionsEmptyFace{Icon: "refresh", Title: copy.SessionsLoading}
	default:
		return sessionsEmptyFace{Icon: "square-terminal", Title: copy.SessionsEmpty, Hint: copy.SessionsEmptyHint}
	}
}
