package glassmap

import "testing"

// P13-gc · 内容页那一档的 0.09，量的是「框的可见 alpha」还是「应用自己画的那笔填充」？
//
// `ContentAlpha()` 旧算式是 `overFrame(TintAlpha, min(1, TintAlpha+contentStep))`。
// `TintAlpha` 在 macOS 上是被 `MaterialVeil` 补偿过的，于是 §1 那条「内容页比框深
// 一步」换算成**可见** recess 时要乘 `(1 − veil)`：
//
//	平台/档（47% 滑块）   veil  Tint   旧可见 recess  新可见 recess  旧填充/新填充
//	linux / windows       0     0.470  0.090          0.090          43 / 43
//	darwin frosted        0.12  0.398  0.079          0.090          38 / 43
//	darwin regular        0.22  0.321  0.070          0.090          38 / 43
//	darwin liquid         0.34  0.197  0.059          0.090          38 / 43
//
// 47% 的填充三档同值（0.17），因为一步可见 alpha 只取决于**框的可见** alpha，而它
// 在三个平台上都是滑块的值。旧算式的填充也几乎同值（0.149）——差的是它落在
// **哪个空间**：旧值乘上 `(1−v)` 才是眼睛看到的那一步，于是材质越厚，内容页越读不出来。
//
// 这张表是写死的字面量，不是「用被测者的公式算期望值」：`contentStep` 一改、`Composite`
// 一改，红的是具体的格子。
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
		{"linux", "off", 10, 0.35, 0.13846154, "off 的读性地板 0.35 就是这一档的框"},
		{"linux", "off", 47, 0.47, 0.16981132, "off 在默认滑块上：没有材质，补偿是恒等"},
		{"linux", "regular", 10, 0.10, 0.10, "一个 10% 的薄框"},
		{"linux", "regular", 47, 0.47, 0.16981132, "出厂的默认档（settings.DefaultMainOpacity）"},
		{"linux", "regular", 70, 0.70, 0.30, "一个偏实的框"},
		{"linux", "regular", 90, 0.90, 0.90, "最后一条还有余量的滑块档"},
		{"linux", "regular", 100, 0.98, 1.00, "顶格：SolidTop 0.98 封底，内容页复合到 1.00"},
		{"windows", "regular", 47, 0.47, 0.16981132, "Windows 的背衬与无材质平台同值（MaterialVeil 只认 darwin）"},
		{"windows", "liquid", 47, 0.47, 0.16981132, "同一个可见 alpha，就该是同一个内容页"},
		// macOS：材质把填充补薄，可见 alpha 仍是滑块的值 —— 内容页必须跟着可见 alpha。
		{"darwin", "frosted", 10, 0.12, 0.10227273, "薄档：veil 12% 把 10% 的框抬到 0.12（旧填充 0.09）"},
		{"darwin", "frosted", 47, 0.47, 0.16981132, "薄档的默认滑块：旧填充 0.149（可见 recess 0.079）"},
		{"darwin", "regular", 10, 0.22, 0.11538462, "标准档的薄框：旧填充 0.09，比它自己的框还淡"},
		{"darwin", "regular", 47, 0.47, 0.16981132, "标准档的默认滑块：旧可见 recess 0.070（linux 的 78%）"},
		{"darwin", "regular", 100, 0.98, 1.00, "标准档顶格"},
		{"darwin", "liquid", 10, 0.34, 0.13636364, "厚档的薄框：旧填充 0.09 —— 三层材质里内容页最淡的那一格"},
		{"darwin", "liquid", 25, 0.34, 0.13636364, "25% 仍被 veil 34% 托住：框的可见 alpha 不动，内容页也不动"},
		{"darwin", "liquid", 47, 0.47, 0.16981132, "厚档的默认滑块：旧可见 recess 0.059（linux 的 66%）"},
		{"darwin", "liquid", 70, 0.70, 0.30, "厚档的实框"},
		{"darwin", "liquid", 90, 0.90, 0.90, "厚档的倒数第二档"},
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
		want := min(1, spec.Composite()+contentStep)
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
