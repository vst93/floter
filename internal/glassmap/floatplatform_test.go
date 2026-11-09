package glassmap

import "testing"

// P13-gb · 浮层那一档的 0.10，量的是「框的可见 alpha」还是「应用自己画的那一笔填充」？
//
// `FloatAlpha()` 旧算式是 `min(1, TintAlpha+floatStep)`。`TintAlpha` 在 macOS 上是
// 被 `MaterialVeil` 补偿过的（`compensate` 把窗口材质的 alpha 从填充里扣掉），于是
// §1 那条「浮 = 框 tint + 0.10」换算成**可见**抬升时要乘 `(1 − veil)`：
//
//	平台/档（47% 滑块）   veil   Tint  旧 Float  旧可见抬升  新 Float  新可见抬升
//	linux / windows       0      0.470 0.570     0.302       0.570     0.302
//	darwin frosted        0.12   0.398 0.498     0.264       0.570     0.302
//	darwin regular        0.22   0.321 0.421     0.223       0.570     0.302
//	darwin liquid         0.34   0.197 0.297     0.210       0.570     0.302
//
// 也就是「材质越厚，浮层离框越近」——阶梯方向是反的；10% 滑块 + `liquid` 只剩
// 0.066（linux 同滑块 0.180 的 37%）。而同一个 spec 里的 `ContentAlpha()` 早就用
// `overFrame` 把它的 0.09 表达成「可见 alpha 上的一步」：只有浮层在填充空间里做加法。
//
// 这张表是写死的字面量，不是「用被测者的公式算期望值」：`floatStep` 一改、`Composite`
// 一改，红的是具体的格子。
func TestTheFloatRungIsAStepOnTheFramesVisibleAlpha(t *testing.T) {
	type rung struct {
		goos     string
		step     string
		opacity  uint8
		compose  float32 // 框的可见 alpha，字面量
		float    float32 // 浮层填充，字面量
		whyLabel string
	}
	cases := []rung{
		// 无材质的平台：填充 == 可见 alpha，旧新同值（这条是「没有回归」的证据）。
		{"linux", "off", 10, 0.35, 0.45, "off 的读性地板 0.35 就是这一档的框"},
		{"linux", "regular", 10, 0.10, 0.20, "一个 10% 的薄框"},
		{"linux", "regular", 47, 0.47, 0.57, "出厂的默认档（settings.DefaultMainOpacity）"},
		{"linux", "regular", 70, 0.70, 0.80, "一个偏实的框"},
		{"linux", "regular", 90, 0.90, 1.00, "最后一条还有余量的滑块档"},
		{"linux", "regular", 100, 0.98, 1.00, "顶格：SolidTop 0.98 封底，min(1, …) 封浮"},
		{"windows", "regular", 47, 0.47, 0.57, "Windows 的背衬与无材质平台同值（MaterialVeil 只认 darwin）"},
		{"windows", "liquid", 47, 0.47, 0.57, "同一个可见 alpha，就该是同一个浮层"},
		// macOS：材质把填充补薄，可见 alpha 仍是滑块的值 —— 浮层必须跟着可见 alpha。
		{"darwin", "frosted", 10, 0.12, 0.22, "薄档：veil 12% 把 10% 的框抬到 0.12（slider 的低端被材质托住，§1）"},
		{"darwin", "frosted", 47, 0.47, 0.57, "薄档的默认滑块：填充比 linux 薄（0.398），浮层不薄"},
		{"darwin", "regular", 10, 0.10, 0.20, "透明玻璃保留滑块低端"},
		{"darwin", "regular", 47, 0.47, 0.57, "标准档的默认滑块：旧值 0.421，可见抬升只有 linux 的 74%"},
		{"darwin", "regular", 100, 0.98, 1.00, "标准档顶格"},
		{"darwin", "liquid", 10, 0.10, 0.20, "透明玻璃没有旧侧栏材质的 34% 下限"},
		{"darwin", "liquid", 25, 0.25, 0.35, "25% 随滑块变化"},
		{"darwin", "liquid", 47, 0.47, 0.57, "厚档的默认滑块：旧值 0.297，可见抬升 0.210（linux 的 70%）"},
		{"darwin", "liquid", 70, 0.70, 0.80, "厚档的实框"},
		{"darwin", "liquid", 90, 0.90, 1.00, "厚档的倒数第二档"},
	}
	for _, tc := range cases {
		spec := SpecForPreferencesOS(tc.goos, tc.step, tc.opacity, false)
		if got := spec.Composite(); diff(got, tc.compose) {
			t.Errorf("%s/%s/%d%%: the frame reads %v, want the literal %v", tc.goos, tc.step, tc.opacity, got, tc.compose)
		}
		got := spec.FloatAlpha()
		if diff(got, tc.float) {
			t.Errorf("%s/%s/%d%%: the float fill = %v, want %v — %s", tc.goos, tc.step, tc.opacity, got, tc.float, tc.whyLabel)
			continue
		}
		// 不变量 ①：浮层永远比它压着的那张面实。
		if got <= tc.compose {
			t.Errorf("%s/%s/%d%%: the float (%v) is not denser than the frame (%v)", tc.goos, tc.step, tc.opacity, got, tc.compose)
		}
		// 不变量 ②：可见抬升是 (1−C)·F —— 只要 C 相同，三个平台必须是同一档。
		lift := (1 - spec.Composite()) * spec.FloatAlpha()
		want := (1 - tc.compose) * tc.float
		if diff(lift, want) {
			t.Errorf("%s/%s/%d%%: the visible lift = %v, want %v", tc.goos, tc.step, tc.opacity, lift, want)
		}
	}
}

// 同一张面在三个平台上必须是同一档：C 相同 → F 相同。这条与上表互补——上表钉的是
// 具体的字面量，这条钉的是「没有任何平台借 veil 偷偷缩一档」。
func TestTheSameFrameYieldsTheSameFloatOnEveryPlatform(t *testing.T) {
	// 这三个滑块档上，veil 不会把框的可见 alpha 抬过滑块的值（10% 会被抬，走反面臂）。
	for _, opacity := range []uint8{25, 47, 70, 90} {
		base := SpecForPreferencesOS("linux", "regular", opacity, false)
		want := base.FloatAlpha()
		for _, platform := range []struct{ goos, step string }{
			{"windows", "regular"},
			{"windows", "liquid"},
			{"darwin", "frosted"},
			{"darwin", "regular"},
			{"darwin", "liquid"},
		} {
			spec := SpecForPreferencesOS(platform.goos, platform.step, opacity, false)
			if diff(spec.Composite(), base.Composite()) {
				continue // veil 把框抬过滑块值的那一档不在这条的射程里（见反面臂）
			}
			if diff(spec.FloatAlpha(), want) {
				t.Errorf("%s/%s/%d%%: the float = %v, want the material-free %v at the same frame %v",
					platform.goos, platform.step, opacity, spec.FloatAlpha(), want, base.Composite())
			}
		}
	}
	// 反面臂：材质把框抬起来时（10% + veil），浮层跟着抬起后的框走，不是跟着滑块。
	thinLinux := SpecForPreferencesOS("linux", "liquid", 10, false)
	thinDarwin := SpecForPreferencesOS("darwin", "frosted", 10, false)
	if !(thinDarwin.Composite() > thinLinux.Composite()) {
		t.Fatalf("the material did not lift the thin frame: %v vs %v", thinDarwin.Composite(), thinLinux.Composite())
	}
	if !(thinDarwin.FloatAlpha() > thinLinux.FloatAlpha()) {
		t.Errorf("the thin frame is denser under a material (%v > %v) but its float is not: %v vs %v",
			thinDarwin.Composite(), thinLinux.Composite(), thinDarwin.FloatAlpha(), thinLinux.FloatAlpha())
	}
	// 而且它仍是「框 + 0.10」：被抬起的框不会顺带把抬升压矮。
	if got, want := thinDarwin.FloatAlpha()-thinDarwin.Composite(), float32(0.10); diff(got, want) {
		t.Errorf("the lifted thin frame's lift = %v, want the constant %v", got, want)
	}
}
