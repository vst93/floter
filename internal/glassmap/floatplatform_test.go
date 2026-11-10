package glassmap

import "testing"

// Literal expectations for the readability-adjusted material, across platforms.
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
		{"linux", "off", 74, 0.74000000, 0.84000000, "actual opacity"},
		{"linux", "regular", 74, 0.74000000, 0.84000000, "actual opacity"},
		{"linux", "regular", 86, 0.86000000, 0.96000000, "actual opacity"},
		{"linux", "regular", 92, 0.92000000, 1.00000000, "actual opacity"},
		{"linux", "regular", 96, 0.96000000, 1.00000000, "actual opacity"},
		{"linux", "regular", 98, 0.98000000, 1.00000000, "actual opacity"},
		{"windows", "regular", 86, 0.86000000, 0.96000000, "actual opacity"},
		{"windows", "liquid", 86, 0.86000000, 0.96000000, "actual opacity"},
		// macOS：材质把填充补薄，可见 alpha 仍是滑块的值 —— 浮层必须跟着可见 alpha。
		{"darwin", "frosted", 74, 0.74000000, 0.84000000, "actual opacity"},
		{"darwin", "frosted", 86, 0.86000000, 0.96000000, "actual opacity"},
		{"darwin", "regular", 74, 0.74000000, 0.84000000, "actual opacity"},
		{"darwin", "regular", 86, 0.86000000, 0.96000000, "actual opacity"},
		{"darwin", "regular", 98, 0.98000000, 1.00000000, "actual opacity"},
		{"darwin", "liquid", 74, 0.74000000, 0.84000000, "actual opacity"},
		{"darwin", "liquid", 80, 0.80000000, 0.90000000, "actual opacity"},
		{"darwin", "liquid", 86, 0.86000000, 0.96000000, "actual opacity"},
		{"darwin", "liquid", 92, 0.92000000, 1.00000000, "actual opacity"},
		{"darwin", "liquid", 96, 0.96000000, 1.00000000, "actual opacity"},
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
	for _, opacity := range []uint8{74, 80, 86, 98} {
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
	// Both platforms share the readable low end; native veil is compensated.
	thinLinux := SpecForPreferencesOS("linux", "liquid", 10, false)
	thinDarwin := SpecForPreferencesOS("darwin", "frosted", 10, false)
	if diff(thinLinux.Composite(), thinDarwin.Composite()) || diff(thinLinux.FloatAlpha(), thinDarwin.FloatAlpha()) {
		t.Fatal("native veil changed effective opacity")
	}
}
