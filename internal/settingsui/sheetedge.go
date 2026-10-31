package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// sheetEdgeFace is the colour the plugin configuration sheet's scroll edges
// lay down: the face the scroller sits on, composited opaque.
//
// 为什么不把 `Sunken` 直接交给 `glass.ScrollEdge`：`Sunken` 是一层 0.17 的薄罩，
// 而 `ui.Color.Alpha` 是**乘**——那道 0.85 会被它自己的 0.17 乘成 0.14，边淡了个
// 寂寞（§6.17 / §6.18 的同一条反直觉）。
//
// 为什么只压一层：这块浮层的面就是启动器那张**面板**（`Sunken` 压窗口地），而
// `Sunken` 的颜色是地面色的邻居（§6.16 实测深浅两套都只差 1 级），所以「地 + 薄罩」
// 的不透明合成就是它看上去的面。设置页那处要压四层，是因为它脚下真的垫着卡
// （§6.17 的 `runOutputEdgeFace`、§6.18 的 `configListEdgeFace`）。
func sheetEdgeFace(tokens theme.Tokens, t *ui.Theme) ui.Color {
	return theme.FaceOver(t.Background, tokens.Sunken)
}
