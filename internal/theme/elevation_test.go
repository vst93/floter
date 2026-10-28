package theme

import (
	"math"
	"testing"
)

// P11-an · `--elev-shadow-scale` 是框的透明度的函数，不是常数。
//
// 参考 `main:src/styles/base.css:178`：
// `--elev-shadow-scale: calc(0.45 + 0.9 * (1 - var(--main-opacity)))`。
// 移植时四个浮动层抄的是 scale = 1 的字面值（0.3 / 0.2），于是透明度滑块只动
// 填充、不动投影。这条 pin 钉住算术本身：两端、出厂默认那一档、以及它随
// 透明度单调下降的方向。
func TestTheElevationScaleFollowsTheFrameAlpha(t *testing.T) {
	const epsilon = 1e-4
	cases := []struct {
		opacity uint8
		want    float32
	}{
		{100, 0.45}, // an opaque frame: the reference's lightest throw
		{47, 0.927}, // the shipped default (`settings.DefaultMainOpacity`)
		{10, 1.26},  // the slider's thinnest stop
		{0, 1.35},   // clamped, not extrapolated
		{200, 0.45}, // clamped from above too
	}
	for _, c := range cases {
		if got := ElevScale(c.opacity); math.Abs(float64(got-c.want)) > epsilon {
			t.Errorf("ElevScale(%d) = %v, want %v", c.opacity, got, c.want)
		}
	}
	// Monotonic: a thinner frame never casts a lighter throw.
	previous := ElevScale(0)
	for o := uint8(1); o <= 100; o++ {
		got := ElevScale(o)
		if got > previous+epsilon {
			t.Fatalf("ElevScale(%d) = %v is heavier than ElevScale(%d) = %v: "+
				"the slider has to lighten the throw, not darken it", o, got, o-1, previous)
		}
		previous = got
	}
	// And it is not the constant the port used to paint: the default frame is
	// 7% lighter than the unscaled throw, so a surface that reads the scale is
	// one pixel lighter than one that does not.
	if scale := ElevScale(47); !(scale < 1 && scale > 0.9) {
		t.Errorf("the default frame's scale is %v; the port painted 1 (unscaled)", scale)
	}
}
