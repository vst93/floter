package glassmap

import (
	"testing"
)

// Floating panes stay denser until the opaque endpoint; at 100% color and
// shadow provide separation because every background is fully opaque.
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
		{100, 0, true, "the opaque endpoint uses color and shadow for separation"},
	}
	for _, tc := range steps {
		spec := SpecForPreferencesOS("linux", "regular", tc.opacity, false)
		ground, float := spec.TintAlpha, spec.FloatAlpha()
		if float < ground || (tc.opacity < 100 && float == ground) {
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
		if want := ground + min32(contentStep, (1-ground)*.5); abs32(seen-want) > 0.0005 {
			t.Errorf("%d%%: the content pane composites to %.3f, want the ground plus contentStep (%.3f)", tc.opacity, seen, want)
		}
		if seen < ground || (tc.opacity < 100 && seen == ground) {
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
