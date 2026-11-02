package launcher

import (
	"testing"

	"floter/internal/settings"
)

// P13-dq · **两把尺子各有名字，帧高才不是猜的。**
//
// `docs/liquid-glass.md` §6.76 把两套主题量成了两个比例（`Spacing` 0.90、
// `FontSize`/行盒 0.77）；§6.86 把启动器那批红的账摊开：夹具画的是框架库存主题，
// 生产画的是这一面自己的调色板。
//
// 本轮落地的是**尺子本身**，不是那批账里的任何一条：`frameHeight`（库存尺，4.0、
// 行盒没喂，由 `Geometry` 的 `font = 14` 兜底）与 `themedFrameHeight`（这一面真正
// 画的尺，3.6 + 实测行盒 19/17）分开命名，`designMetrics` 按 `App.View` 自己的
// 量法量（`c.MeasureText(0, Span{"Ag", …})`，`launcher.go:1297-1307`），不按字号
// 猜一个倍数。下一格起每收口一个表面，就把它的取景换到 `themedFrameHeight`。
//
// 这条 pin 钉两件事：
//  1. 设计尺的四个数是实测的那四个（间距 / 字号 / 两条行盒）；
//  2. 两条帧高的差就是这两把尺子的差 —— 谁漂了这里立刻红，不用等一批像素断言
//     在别处一起变红才知道。
func TestTheTwoRulersAreNamedAndMeasured(t *testing.T) {
	spacing, font, titleLine, subtitleLine := designMetrics()
	if s, f := float32(spacing), float32(font); s != 3.6 || f != 10.8 {
		t.Errorf("the shipped ruler is spacing %.2f / font %.2f, want 3.6 / 10.80 "+
			"(§6.76 measured `Spacing` 4.0 → 3.6, `FontSize` 14.00 → 10.80)", spacing, font)
	}
	// The line boxes are the text's own, not `font + 2` / `font`: the framework
	// lays a line at the font's ascent, descent and gap.
	if titleLine != 19 || subtitleLine != 17 {
		t.Errorf("the shipped line boxes are title %.2f / subtitle %.2f, want 19 / 17 "+
			"(measured `Ag` at 12.6 and 10.8 — §6.76's 22.0 → 17.0 at the body size)",
			titleLine, subtitleLine)
	}

	// The two frames at the numbered family's ten rows: the stock ruler charges
	// the 40u row floor and the chrome at 4.0; the shipped one at 3.6 with the
	// measured line boxes. 488 / 439 is the pair measured in P13-dq.
	if got, want := frameHeight(maxResultSlots), 488; got != want {
		t.Errorf("the stock ruler's ten-row frame is %d, want %d", got, want)
	}
	if got, want := themedFrameHeight(maxResultSlots), 439; got != want {
		t.Errorf("the shipped ruler's ten-row frame is %d, want %d", got, want)
	}

	// And the shipped frame is the same arithmetic with the measured boxes fed
	// in — the helper is not a second constant beside `Geometry.Height`.
	geometry := Geometry{
		Font: font, Spacing: spacing,
		TitleLine: titleLine, SubtitleLine: subtitleLine,
		Rows: make([]RowInfo, maxResultSlots), Held: maxResultSlots,
	}
	if got, want := themedFrameHeight(maxResultSlots), geometry.Height()+2; got != want {
		t.Errorf("themedFrameHeight = %d, Geometry.Height()+2 = %d: the fixture's frame "+
			"and the window's own arithmetic disagree", got, want)
	}
}

// `designMetrics` resolves the shipped step from `settings.Default()` — the
// store this package's fixtures use (the two that change `UIScale` build their
// frames from their own numbers). This pin states that boundary so a later
// sweep that makes the probe take a store has to come here and say so.
func TestTheProbeResolvesTheDefaultStore(t *testing.T) {
	if got, want := settings.Default().UIScale, settings.DefaultUIScale; got != want {
		t.Errorf("the probe's store stands at interface step %q, want the shipped %q", got, want)
	}
	if spacing, _, _, _ := designMetrics(); float32(spacing) != 3.6 {
		t.Errorf("the probe's spacing is %.2f at the default step, want 3.6", spacing)
	}
}
