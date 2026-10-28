package theme

import "github.com/egoist/mygo/ui"

// P11-an · 浮动层的投影只有一个读法：跟着框的透明度缩放。
//
// base.css 把每一档 elevation 的投影都乘上 `--elev-shadow-scale`
// （`main:src/styles/base.css:178`）：
//
//	--elev-shadow-scale: calc(0.45 + 0.9 * (1 - var(--main-opacity)));
//
// 也就是说同一条 `--elev-3` 扔在 100% 的框上是 0.135 的宽投影，扔在 10% 的
// 框上才是 0.378——框越薄，阴影越轻，这是「薄框配重影」那条观感的算术。
//
// 移植时把 **scale = 1 那一档的字面值**（0.3 / 0.2）抄进了四个浮动层，于是
// 透明度滑块只动填充、不动投影：用户把框拉到 10%（最轻的那一档）时，参考会
// 把投影放到 0.405，本仓仍是 0.3——一块几乎透明的面板顶着一团没变黑的黑边。
// 更糟的是 `internal/launcher/launcher.go:2970` 与 `internal/terminalui` 的
// 注释都写着「a function of the frame's own alpha, so its 0.2 is scaled here
// exactly as the terminal note's is」——那句话描述的耦合从来没实现过。
//
// 本轮把这条算术落成一处：`ElevScale` 是 `--elev-shadow-scale` 本身，
// `ElevThrow` / `ElevContact` 是 `--elev-3`（宽投影 + 紧贴边）与 `--elev-2`
// 的接触边。四个浮动层（启动器的警告状态行、设置页的卸载对话框、终端页的
// 退出便条与失败提示）全部改成调它们。

// ElevScale is base.css's `--elev-shadow-scale` for a frame at the given
// transparency: 0.45 at an opaque frame (`main_opacity` 100), 1.26 at the
// slider's thinnest stop (10) and 1.35 at zero. The slider's own range is what the setting clamps to
// (`settings.NormalizeOpacity`), and a value outside it is clamped here rather
// than extrapolated: the reference's CSS cannot produce a factor outside the
// two ends either.
func ElevScale(opacity uint8) float32 {
	percent := float32(opacity)
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return 0.45 + 0.9*(1-percent/100)
}

// ElevThrow paints a floating layer's throw: the reference's `--elev-3`, which
// is the broad `0 10px 26px` at 0.30 and the tight `0 1px 2px` contact edge at
// 0.20, both times `ElevScale(opacity)`.
//
// The geometry is the callers' own (`--u` steps through the theme's spacing),
// so a surface that already carried the two shadows keeps its distances: only
// the two alphas move, and they move with the slider.
func ElevThrow(e ui.Element, t *ui.Theme, opacity uint8) {
	scale := ElevScale(opacity)
	e.Shadow(0, t.Space(2.5), t.Space(6.5), 0, ui.RGBA(0, 0, 0, 0.30*scale))
	e.Shadow(0, t.Space(0.25), t.Space(0.5), 0, ui.RGBA(0, 0, 0, 0.20*scale))
}

// ElevContact paints only the `--elev-3` contact edge (the reference's
// `--elev-2` step is the same `0 1px 2px` line): a raised row that sits on the
// frame rather than floating over it carries this layer alone.
func ElevContact(e ui.Element, t *ui.Theme, opacity uint8) {
	e.Shadow(0, t.Space(0.25), t.Space(0.5), 0, ui.RGBA(0, 0, 0, 0.20*ElevScale(opacity)))
}
