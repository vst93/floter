package settingsui

import (
	"testing"

	"floter/internal/i18n"
)

// P13-cr（`docs/liquid-glass.md` §6.61）：扩展页命令板上「Summon with: …」这一行，
// 此前是 `TextMuted` 再乘一把 0.85（参考 CSS `.extension-command-list__triggers`
// 的 `opacity: .85` 叠在 `span { color: var(--text-muted) }` 上）。乘法不是阶梯：
// 同一个 0.85 落到两套外观上就是两个位置——实测浅色 `{141,141,147}`，与
// `TextTertiary` 的 `{142,142,148}` 只差 **1 级**；深色 `{122,122,129}`，落在
// `TextMuted` `{133,133,141}` 与 `TextTertiary` `{110,110,118}` 之间的**无人区**。
//
// 这条 pin 量的是**梯级**，不是这两个数：这一行的墨必须是阶梯的第三档本身
// （`panelOver(TextTertiary, 板面)`），并且仍然比它上面那行 description
// （`TextMuted`）低**一整档**——把乘法删干净（回到 `TextMuted`）会让这行与
// description 逐像素相同，那就少说了一句话；跳两档（`TextSecondary`）会让
// 阶梯倒挂，名字比事实还轻。
func TestTheSummonLineStandsOnTheInkLadder(t *testing.T) {
	en := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			a := New(newStore(t), Actions{})
			a.Integrations = func() []Integration { return []Integration{commandListIntegration()} }
			tt, tokens := renderIn(t, a, 720, 900, appearance.dark)
			if err := tt.Click("Integrations"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()

			nameRect := mustFind(t, tt, "JSON Viewer")
			descRect := mustFind(t, tt, "View JSON")
			summon := mustFind(t, tt, en.IntegrationsCommandsTriggers("jv, JSON, jview, viewer"))
			namePlane, nameInk := actionRungProbe(tt, nameRect)
			descPlane, descInk := actionRungProbe(tt, descRect)
			summonPlane, summonInk := actionRungProbe(tt, summon)

			// ① The summon line's ink is the ladder's third rung over the plate it
			// stands on — not a multiplication of the second one.
			if want := panelOver(tokens.TextTertiary, summonPlane); !panelClose(summonInk, want, 6) {
				t.Errorf("the summon line's ink is %v, want the TextTertiary rung over its plate (%v): "+
					"the line is drawn by a multiply, not by the ladder", summonInk, want)
			}
			// ② …and it is still one whole rung below the description above it:
			// dropping the 0.85 without moving a rung would make the two lines
			// pixel-identical, which says "these are the same fact".
			if d := channelDelta(summonInk, descInk); d < 8 {
				t.Errorf("the summon line %v and the description %v stand %d levels apart: "+
					"the two facts collapsed onto one rung", summonInk, descInk, d)
			}
			// ③ The plate's ladder runs one way only: name (`TextSecondary`) is
			// the heaviest, then the description (`TextMuted`), then the summon
			// words (`TextTertiary`).
			nameOff := channelDelta(nameInk, namePlane)
			descOff := channelDelta(descInk, descPlane)
			summonOff := channelDelta(summonInk, summonPlane)
			if !(nameOff > descOff && descOff > summonOff) {
				t.Errorf("the plate's inks run %d (name) / %d (description) / %d (summon) off their plate: "+
					"the ladder is out of order", nameOff, descOff, summonOff)
			}
			t.Logf("%s: plate %v · name %v (%d) · description %v (%d) · summon %v (%d)",
				appearance.name, summonPlane, nameInk, nameOff, descInk, descOff, summonInk, summonOff)
		})
	}
}
