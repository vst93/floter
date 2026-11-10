package glassmap

import "testing"

// Literal expectations for the readability-adjusted material, across platforms.
func TestTheContentRungIsAStepOnTheFramesVisibleAlpha(t *testing.T) {
	type rung struct {
		goos     string
		step     string
		opacity  uint8
		compose  float32 // 框的可见 alpha，字面量
		content  float32 // 内容页填充，字面量
		whyLabel string
	}
	cases := []rung{
		// 无材质的平台：填充 == 可见 alpha 的反解，旧新同值（「没有回归」的证据）。
		{"linux", "off", 74, 0.74000000, 0.34615385, "actual opacity"},
		{"linux", "off", 86, 0.86000000, 0.50000000, "actual opacity"},
		{"linux", "regular", 74, 0.74000000, 0.34615385, "actual opacity"},
		{"linux", "regular", 86, 0.86000000, 0.50000000, "actual opacity"},
		{"linux", "regular", 92, 0.92000000, 0.50000000, "actual opacity"},
		{"linux", "regular", 96, 0.96000000, 0.50000000, "actual opacity"},
		{"linux", "regular", 98, 0.98000000, 0.50000000, "actual opacity"},
		{"windows", "regular", 86, 0.86000000, 0.50000000, "actual opacity"},
		{"windows", "liquid", 86, 0.86000000, 0.50000000, "actual opacity"},
		// macOS：材质把填充补薄，可见 alpha 仍是滑块的值 —— 内容页必须跟着可见 alpha。
		{"darwin", "frosted", 74, 0.74000000, 0.34615385, "actual opacity"},
		{"darwin", "frosted", 86, 0.86000000, 0.50000000, "actual opacity"},
		{"darwin", "regular", 74, 0.74000000, 0.34615385, "actual opacity"},
		{"darwin", "regular", 86, 0.86000000, 0.50000000, "actual opacity"},
		{"darwin", "regular", 98, 0.98000000, 0.50000000, "actual opacity"},
		{"darwin", "liquid", 74, 0.74000000, 0.34615385, "actual opacity"},
		{"darwin", "liquid", 80, 0.80000000, 0.45000000, "actual opacity"},
		{"darwin", "liquid", 86, 0.86000000, 0.50000000, "actual opacity"},
		{"darwin", "liquid", 92, 0.92000000, 0.50000000, "actual opacity"},
		{"darwin", "liquid", 96, 0.96000000, 0.50000000, "actual opacity"},
	}
	for _, tc := range cases {
		spec := SpecForPreferencesOS(tc.goos, tc.step, tc.opacity, false)
		if got := spec.Composite(); diff(got, tc.compose) {
			t.Errorf("%s/%s/%d%%: the frame reads %v, want the literal %v", tc.goos, tc.step, tc.opacity, got, tc.compose)
		}
		got := spec.ContentAlpha()
		if diff(got, tc.content) {
			t.Errorf("%s/%s/%d%%: the content pane paints %v, want %v — %s", tc.goos, tc.step, tc.opacity, got, tc.content, tc.whyLabel)
		}
		// 眼睛量的是「面比框深多少」，也就是两条**可见** alpha 之差。它必须恒等于
		// contentStep，不论平台、档位、滑块——那正是本轮改的那件事。
		visible := 1 - (1-spec.Material)*(1-spec.TintAlpha)*(1-spec.ContentAlpha())
		want := spec.Composite() + min(contentStep, (1-spec.Composite())*.5)
		if diff(visible, want) {
			t.Errorf("%s/%s/%d%%: the pane composites to %v, want the frame + step %v (recess %v, want %v)",
				tc.goos, tc.step, tc.opacity, visible, want, visible-spec.Composite(), want-spec.Composite())
		}
		if visible <= spec.Composite() {
			t.Errorf("%s/%s/%d%%: the pane (%v) is not below the frame (%v)", tc.goos, tc.step, tc.opacity, visible, spec.Composite())
		}
		// 同 C 就必须同填充：47% 的三个平台上框的可见 alpha 是同一个数。
		if spec.Composite() == 0.47 && diff(spec.ContentAlpha(), 0.16981132) {
			t.Errorf("%s/%s/%d%%: the same frame (0.47) carries %v", tc.goos, tc.step, tc.opacity, spec.ContentAlpha())
		}
		// 面不高于浮（§2 判据 1：一个表面只有一个 E2，且浮必须站在面之上）。顶格那一档
		// 两者都被封在 1.00 —— 顶端不靠 alpha 分家，§2.1 第 2 条已判「故意不做」。
		if spec.FloatAlpha() < spec.ContentAlpha() {
			t.Errorf("%s/%s/%d%%: the float %v stands below the pane %v", tc.goos, tc.step, tc.opacity, spec.FloatAlpha(), spec.ContentAlpha())
		}
		if spec.ContentAlpha() < 1 && spec.FloatAlpha() <= spec.ContentAlpha() {
			t.Errorf("%s/%s/%d%%: the float %v does not stand above the pane %v", tc.goos, tc.step, tc.opacity, spec.FloatAlpha(), spec.ContentAlpha())
		}
	}
}

// 反面臂：内容页的填充**不是**框的填充；两者只有在填充空间里才相等，而填充空间正是
// 本轮判掉的那个读法。这条守住「有人把 ContentAlpha 写成 `TintAlpha + contentStep`
// 或直接返回 TintAlpha」这两种改法：前者在 linux 上差 0.09，后者让面 = 框。
func TestTheContentRungIsNotTheFramesOwnFill(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		spec := SpecForPreferencesOS(goos, "liquid", 47, false)
		if !diff(spec.ContentAlpha(), spec.TintAlpha) {
			t.Errorf("%s: the content pane = the frame's own fill %v, want the recess above it (%v)", goos, spec.TintAlpha, spec.ContentAlpha())
		}
	}
	// `off` 停板与材质无关：`MaterialVeil` 在每一档 `off` 上都是 0，旧新同值。
	linux := SpecForOS("linux", "off", 47)
	darwin := SpecForOS("darwin", "off", 47)
	if diff(linux.ContentAlpha(), darwin.ContentAlpha()) {
		t.Errorf("the off stop's pane = %v on darwin and %v on linux, want the same (no material on either)",
			darwin.ContentAlpha(), linux.ContentAlpha())
	}
	if darwin.Material != 0 || linux.Material != 0 {
		t.Errorf("the off stop carries material: darwin %v / linux %v, want 0", darwin.Material, linux.Material)
	}
}
