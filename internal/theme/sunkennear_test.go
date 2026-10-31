package theme

import (
	"testing"

	"floter/internal/settings"
)

// P13-au · 设置页滚动体的两道边穿的是 `t.Background`，而它们脚下是面板的
// `Sunken` 面（`settingsui.go` 的 `body`）。§6.16 把这件事记成「静止帧顶部那
// 16px 理论上有一道 ~15 级的淡痕」，前提是「面板 229 / 39」——那是 `Sunken`
// 压在**扩展卡**上的合成值，不是压在**窗口地**上的值。
//
// 实测（720×580，General 页，上一轮 P13-at 造的探针）：面板的合成面浅色 **246**、
// 深色 **15**，而窗口地是 247 / 14——同一张面上只差 **1 级**。所以这两道边淡向
// `t.Background` 是一个**量过、不改**的结论，不是一笔没查的账。
//
// 这条 pin 守的是那个前提本身：`Sunken` 一直是「窗口地 + 一层近白/近黑的薄罩」，
// 它压在窗口地上只挪 1~2 级。哪天 `Sunken` 变厚（超过 3 级），这两道边就必须换成
// `theme.FaceOver(t.Background, tokens.Sunken)`——那时这条 pin 会先红，红在
// 「边底下那张面已经不是边穿的那张」这句话上。
func TestTheSunkenPaneIsNearlyTheGroundItCovers(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tokens := For(settings.Settings{Theme: "auto"}, dark)
			bg := tokens.Theme.Background
			pane := FaceOver(bg, tokens.Sunken)
			for _, d := range []struct {
				name string
				got  uint8
				want uint8
			}{
				{"R", pane.R, bg.R},
				{"G", pane.G, bg.G},
				{"B", pane.B, bg.B},
			} {
				diff := int(d.got) - int(d.want)
				if diff < 0 {
					diff = -diff
				}
				if diff > 2 {
					t.Errorf("the pane over the ground reads %s %d against the ground's %d (%d levels): "+
						"the settings body's scroll edges (Background: t.Background) now fade into a "+
						"different surface — they must take theme.FaceOver(t.Background, Sunken)",
						d.name, d.got, d.want, diff)
				}
			}
		})
	}
}
