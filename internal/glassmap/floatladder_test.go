package glassmap

import (
	"testing"
)

// P13-ac · 浮层那一档沿透明度滑块还站得住吗？
//
// 任务书「先量再改的两个数」第 1 条猜的是「低透明度端太弱，可能要按框的 alpha 缩放
// 面档」。量下来方向是**反的**，而且真正塌的地方在**顶端**。实测
// （`SpecForPreferencesOS("linux", "regular", o, false)`，Linux 无平台 veil）：
//
//	滑块   TintAlpha(底)  FloatAlpha(浮)  浮−底   相对
//	10%    0.10           0.20            0.10    2.00×
//	25%    0.25           0.35            0.10    1.40×
//	47%    0.47           0.57            0.10    1.21×   ← 默认档
//	70%    0.70           0.80            0.10    1.14×
//	90%    0.90           1.00            0.10    1.11×
//	100%   0.98           1.00            0.02    1.02×   ← SolidTop 封顶
//
// 两条由此定下来的规矩，就是这条 pin：
//
//	① alpha 差是**常数** 0.10（不是比例）：所以低端不需要缩放——10% 时浮层本来就
//	   是底的两倍，而浮层的颜色相对桌面是近黑/近白（`#18191D` / `#FCFCFE`），框越薄
//	   透出的桌面越多，**像素**差反而越大。按 alpha 缩放会把两档拉平，是反方向。
//	② 顶端 `SolidTop = 0.98` 把底封在 0.98、`min(1, …)` 把浮封在 1.00，差只剩 0.02。
//	   这一档不靠 alpha 分家：E2 由两层影（§2 判据 3）+ `EdgeHighlight` 顶边承担，
//	   而把底压到 0.88 去换 0.10 会改掉 `main_opacity` 的承诺（滑块就是框自己的
//	   alpha），那是行为契约，不是墨。所以这里钉的是「**永远严格高于**」，不是
//	   「永远差 0.10」。
//
// 反面臂：浮层也不许比面档还淡——它是压在别的面**上面**的那一层。
func TestTheFloatRungStaysAboveTheGroundAcrossTheOpacitySlider(t *testing.T) {
	// 差值是**写死的数**，不是 `floatStep` 本身：拿常量当期望值等于让被测者给自己
	// 判卷，`floatStep` 一改这条 pin 就跟着改（P13-ac 的第一次变异 M1 就是这么没红
	// 的）。第一档的 `wantConst` 再把常量本身也钉一遍，于是「改常量」是一次显式的
	// 决定，不是一次静默的漂移。
	if floatStep != 0.1 {
		t.Errorf("floatStep = %v, want the shipped 0.1 (liquid-glass.md §1: 浮 = 框 tint + 0.10)", floatStep)
	}
	steps := []struct {
		opacity  uint8
		want     float32 // 期望的 浮−底，绝对值
		exact    bool    // 低端那五档必须是这个差，不是「至少」
		whyLabel string
	}{
		{10, 0.10, true, "a thin frame: the lift is the whole 0.10, and it is the biggest relative lift there is"},
		{25, 0.10, true, "a quarter-veiled frame"},
		{47, 0.10, true, "the shipped default (settings.DefaultMainOpacity)"},
		{70, 0.10, true, "a mostly solid frame"},
		{90, 0.10, true, "the last stop with room left"},
		{100, 0.02, true, "the maxed slider: SolidTop 0.98 caps the ground, min(1, …) caps the float — the rung is carried by the two shadows and the lit edge, not by alpha"},
	}
	for _, tc := range steps {
		spec := SpecForPreferencesOS("linux", "regular", tc.opacity, false)
		ground, float := spec.TintAlpha, spec.FloatAlpha()
		if float <= ground {
			t.Errorf("%d%%: the float (%.3f) is not above the ground (%.3f) — %s", tc.opacity, float, ground, tc.whyLabel)
			continue
		}
		if got := float - ground; tc.exact && abs32(got-tc.want) > 0.0005 {
			t.Errorf("%d%%: the float lifts %.3f, want %.3f (%s)", tc.opacity, got, tc.want, tc.whyLabel)
		}
		// 面档（content pane）也不许低于底：它同样长在框架上。`ContentAlpha` 是**画在
		// 框架之上**的那一层 fill（`overFrame` 的反解），所以它本身比 `TintAlpha` 小是
		// 正常的——要比的是**复合之后**的结果，那才是眼睛看到的那个面。
		seen := spec.ContentAlpha() + ground*(1-spec.ContentAlpha())
		if want := min32(1, ground+contentStep); abs32(seen-want) > 0.0005 {
			t.Errorf("%d%%: the content pane composites to %.3f, want the ground plus contentStep (%.3f)", tc.opacity, seen, want)
		}
		if seen <= ground {
			t.Errorf("%d%%: the content pane (%.3f) is not above the ground (%.3f) once composited", tc.opacity, seen, ground)
		}
	}
}

// abs32 is |v|, for the comparisons above.
func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// min32 is min for two float32s (the package's `min` is generic over ordered
// types already, but this keeps the pin's arithmetic readable).
func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
