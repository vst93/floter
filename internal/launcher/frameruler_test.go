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

// P13-dr · 字段行一族的两个数，都是 **0.9 乘出来的**，不是写死的像素。
//
// §6.86 的第 1 行（单位算术）说的就是这件事：参考把窗口的墨写成 `u`
// （`--u: 4px`；一个 `Space(n)` 是四个 `u`），而出货的步子 `Spacing` 是 3.6，
// 所以 **1u = 0.9px**。字段行的三笔账都乘这个数：
//
//	文字列  16u + 28u + 10u = 54u → 48.6px（`TestTheFieldAndTheRowsShareTheReferenceTextColumn`）
//	块高    56u                  → 50.4px（`TestTheFieldBandIsThePanelsOwnSurface`）
//	块底    56u - 1px            → 49     （`TestTheFieldBlockClosesOnOneWeakHairline`）
//
// 这条 pin 只钉**尺子**：三个乘积与一个取整。谁把 0.9 写成 1.0，或者把 48.6
// 写回 54，这里先红，不必等三个像素断言在三个文件里一起漂。
func TestTheReferenceUnitIsNineTenths(t *testing.T) {
	unit := designUnit()
	if got := tenths(unit); got != 9 {
		t.Errorf("the reference unit at the shipped step is %.3fpx, want 0.90 "+
			"(§6.86: `Spacing` 4.0 → 3.6, one unit is `Spacing/4`)", unit)
	}
	// The field row's text column: the reference's 54u.
	if got, want := tenths(54*unit), 486; got != want {
		t.Errorf("the field row's text column is %.1f, want 48.6 (54u × 0.9)", float64(got)/10)
	}
	// Its band, and the row it closes: `Geometry` charges `Space(14)` and the
	// hairline is drawn on the block's last pixel.
	spacing, _, _, _ := designMetrics()
	if got, want := tenths(fieldRowUnits*spacing), 504; got != want {
		t.Errorf("the field block's band is %.1f tall, want 50.4 (56u × 0.9)", float64(got)/10)
	}
	if got, want := int(float64(fieldRowUnits)*spacing)-1, 49; got != want {
		t.Errorf("the field block's floor is at y=%d, want 49 (the last pixel of a 50.4px band)", got)
	}
}
