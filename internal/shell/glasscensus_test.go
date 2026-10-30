package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reference ships its own census of the controls the liquid glass must
// reach, and this is that census mapped onto this build's draw points —
// `main:tests/glass-reaxis.test.ts:47-90`, `every named control draws a lens
// rung, not a bare ring`. The user's third hard requirement was
// 「当前的液态玻璃效果还是不够，特别是各种控件按钮」, and the failure mode the
// census guards is silent: a control that regressed to a flat pane (or lost its
// lens in a rewrite) keeps drawing, so only a list like this makes it loud.
//
// Each entry either points at a landing (a file and the lens call that must
// still be in it, plus the pixel test that grades it) or records, honestly,
// why this build has no distinct landing (`pending`). The file check is the
// same shape as the reference's own CSS check: the named call must be present,
// or the census fails.
type lensLanding struct {
	selector string // the reference's selector
	file     string // this build's draw point, relative to internal/shell
	marker   string // the lens call that must still be in file
	test     string // the pixel test that grades it, when one exists
	pending  string // why there is no landing yet ("" when landed)
	// dropped names the pin that proves the lens was removed *on purpose*: the
	// Raycast contract replaced the liquid-glass lens on that control with a
	// neutral face and a 1px hairline (`docs/raycast-design.md` §1/§6.1), so
	// the census's job here is the opposite of a landing — it must fail if the
	// lens comes *back* without the contract changing.
	dropped string
}

func TestEveryNamedControlDrawsALensRung(t *testing.T) {
	landings := []lensLanding{
		// Buttons (raised lens).
		{
			selector: ".extensions-action-button",
			file:     "../settingsui/rows.go",
			// P13-i 重定基线（V）：这条 census 条目原本钉的是四层控件透镜
			// （`tokens.Lens(button)`），它的第四层是四边 `ControlEdge` 的环，
			// 而环画在第一条 `LensRim` 之后、落在同一个像素行上——P12-a 抬到
			// 0.09 的受光边从来没亮过。§2 的 E1 是「面 + 一条受光边 + 对侧暗线」，
			// 不是「画框分层」，所以这条从「必须还有那个调用」翻成「那个调用不许
			// 回来」，新面由 `settingsui.TestTheActionPillIsAFaceNotARing` 逐列量着。
			marker:  "tokens.Lens(button)",
			dropped: "docs/liquid-glass.md §2/§6.2 第 9-1 行: the action pill is an E1 face (LitFace), not the four-layer control lens (P13-i)",
			test:    "settingsui.TestTheActionPillIsAFaceNotARing",
		},
		{
			selector: ".app-toast__action",
			file:     "notices.go",
			// P13-j：这枚 chip 坐在 toast 的 E2 面里，所以它是那一层的一个 E1 面
			// （`LitFace` 的一对面），不再是四层透镜。这条条目此前写着「toast 没有
			// 孤立渲染，所以只钉调用」——那个理由早已过期（`notices_test.go` 从 P8
			// 起就能渲染带动作的 toast）。现在它翻成 dropped：那个调用不许回来，新面
			// 由 `TestTheToastActionChipIsAFaceNotARing` 在真 toast 上逐像素量着。
			marker:  "tokens.Lens(action)",
			dropped: "docs/liquid-glass.md §6.6 第 6-3 行: the toast's action chip is an E1 face (LitFace), not the four-layer control lens (P13-j)",
			test:    "TestTheToastActionChipIsAFaceNotARing",
		},
		{
			selector: ".settings-save-alert button",
			file:     "../settingsui/savealert.go",
			// P13-i：这枚 ✕ 坐在 E2 的告警横幅里，所以它是那一层里的一个 E1
			// 面（hover 时抬 `ControlHover` + `LitFace`），不再是四层透镜。
			// 它没有自己的孤立渲染（要一次保存失败才有横幅），所以像素那一半
			// 由同一支 hover 臂的 pin 量着；这一条钉的是「这个文件里那个调用
			// 不许回来」，以及整个设置页表面都不许再叫它。
			marker:  "tokens.Lens(button)",
			dropped: "docs/liquid-glass.md §6.2 第 9-10 行: the save alert's ✕ is an E1 face inside the E2 banner (P13-i)",
			test:    "settingsui.TestTheHoveredIconButtonIsAFaceNotARing",
		},
		{
			selector: ".settings-option--active",
			file:     "../control/control.go",
			marker:   "tokens.Lens(segment)",
			test:     "control.TestTheSegmentedControlCarriesTheStopLens",
		},
		{
			selector: ".settings-sidebar__item:hover",
			file:     "../settingsui/settingsui.go",
			// The sidebar row's lens is gone by contract, not by accident.
			marker:  "tokens.Lens(row)",
			dropped: "docs/raycast-design.md §6.1 第 1 条: the chosen/hovered page is the neutral RowSelected/RowHover rung with no lens (P8-b)",
			test:    "settingsui.TestTheSidebarSelectionIsTheNeutralRowSelected",
		},
		{
			selector: ".settings-section__card",
			file:     "../settingsui/rows.go",
			marker:   "tokens.Lens(card)",
			dropped:  "docs/raycast-design.md §6.1 第 3 条: the grouped card wears Control + HairlineSoft at radius 10, no lens (P8-b)",
			test:     "settingsui.TestTheGroupedCardWearsTheContractsFaceEdgeAndCorner",
		},
		{
			selector: ".launcher-result__icon",
			file:     "../launcher/glyphs.go",
			// P13-h 重定基线（V）：这条 census 条目原本钉的是旧一代的**四层控件
			// 透镜**（`tokens.Lens(plate)`），它的第四层是四边 `ControlEdge` 的环。
			// `docs/liquid-glass.md` §2 把「画框分层」换成「面差 + 一条受光边」，
			// 而这块 28u 的板在每一行都出现 —— 环是「macOS 旧版工具条按钮」那个
			// 读法的来源。现在它是 E1 的一个面（`LitFace`：顶受光边 + 底对侧
			// 暗线，无环），所以这条从「必须还有那个调用」翻成「那个调用不许
			// 回来」，新面由 `launcher.TestTheIconPlateIsAFaceNotARing` 逐列量着。
			marker:  "tokens.Lens(plate)",
			dropped: "docs/liquid-glass.md §2 判据 + §6.1 第 1-4 行: the 28u icon plate is an E1 face (LitFace), not the four-layer control lens (P13-h)",
			test:    "launcher.TestTheIconPlateIsAFaceNotARing",
		},
		{
			// The action bar's rows are result rows in this build, so they lead
			// with the same plate: one draw point serves both names.
			selector: ".launcher-action-bar__icon",
			file:     "../launcher/glyphs.go",
			// 同上一条：一个 draw point 服务两个名字，环退掉时两个名字一起退。
			marker:  "tokens.Lens(plate)",
			dropped: "docs/liquid-glass.md §2 判据 + §6.1 第 1-3/1-4 行: the action bar's plate is the same E1 face (P13-h)",
			test:    "launcher.TestTheIconPlateIsAFaceNotARing",
		},
		// Segmented controls (recessed track + raised chosen slot).
		{
			selector: ".settings-options--inline",
			file:     "../control/control.go",
			marker:   "tokens.LensInset(track)",
			test:     "control.TestTheSegmentedControlCarriesTheStopLens",
		},
		{
			selector: ".extension-custom-mode",
			file:     "../control/control.go",
			// The panel's picker is the same shared body the settings pages' picker
			// draws (`control.SegmentedIndex`), so its recessed track lands the
			// lens in one place for both surfaces.
			marker: "tokens.LensInset(track)",
			test:   "control.TestTheSegmentedControlCarriesTheStopLens",
		},
		// Switch track (recessed lens).
		{
			selector: ".settings-switch",
			file:     "../settingsui/rows.go",
			marker:   "a.tokens(c).LensInset(ui.Switch(c, on))",
			test:     "settingsui.TestTheSettingsSwitchCarriesTheRecessedStopLens",
		},
		// Two controls are deliberately transparent at rest and light their lens
		// only under the pointer.
		{
			selector: ".extensions-icon-button:hover",
			file:     "../settingsui/rows.go",
			// P13-i：这条条目此前是 `pending`，理由是「本仓没有静息透明的扩展
			// 图标钮」——那个理由早就过期了（`rows.go` 的 `extensionIconButton`
			// 从 P8 起就在配置工具栏里）。它 hover 时抬 `ControlHover` 面 +
			// `LitFace` 的一对线，不再是四层透镜，所以这条翻成 dropped。
			marker:  "tokens.Lens(button)",
			dropped: "docs/liquid-glass.md §6.2 第 9-2 行: the 30u icon button's hover rung is an E1 face (P13-i)",
			test:    "settingsui.TestTheHoveredIconButtonIsAFaceNotARing",
		},
		{
			selector: ".toolbar-button:hover",
			file:     "../terminalui/terminalui.go",
			// P13-d 重定基线（V）：这条 census 条目原本钉的是旧一代的
			// **四层控件透镜**（`tokensOf(a, c).Lens(button)`），它的第四层是
			// 四边 `ControlEdge` 的环。`docs/liquid-glass.md` §2 把「画框分层」
			// 换成「面差 + 一条受光边」，§6.3 第 3-7 行就是这一处：悬停的 28u
			// 图标按钮现在是 E1 的一个小面（`LitFace`：顶受光边 + 底对侧暗线，
			// 无环）。所以这条从「必须还有那个调用」翻成「那个调用不许回来」，
			// 新面由 `terminalui.TestTheHoveredBarButtonIsAFaceNotARing` 用像素
			// 逐列量着（左右两侧的列必须与按钮自己的面同色）。
			marker:  "tokensOf(a, c).Lens(button)",
			dropped: "docs/liquid-glass.md §6.3 第 3-7 行: the hovered icon button is an E1 face (LitFace), not the four-layer control lens (P13-d)",
			test:    "terminalui.TestTheHoveredBarButtonIsAFaceNotARing",
		},
	}

	// The reference's own list, in its order: the census cannot silently lose a
	// selector, and a newly named control must be added here with its landing.
	want := []string{
		".extensions-action-button", ".app-toast__action", ".settings-save-alert button",
		".settings-option--active", ".settings-sidebar__item:hover", ".settings-section__card",
		".launcher-result__icon", ".launcher-action-bar__icon",
		".settings-options--inline", ".extension-custom-mode", ".settings-switch",
		".extensions-icon-button:hover", ".toolbar-button:hover",
	}

	if len(landings) != len(want) {
		t.Fatalf("the census lists %d controls, the reference names %d", len(landings), len(want))
	}
	for i, selector := range want {
		if landings[i].selector != selector {
			t.Errorf("census %d = %q, want the reference's %q", i, landings[i].selector, selector)
		}
	}

	root := repoRoot(t)
	for _, landing := range landings {
		landing := landing
		t.Run(landing.selector, func(t *testing.T) {
			if landing.pending != "" {
				if landing.file != "" || landing.marker != "" || landing.dropped != "" {
					t.Errorf("a pending control must not claim a landing: %+v", landing)
				}
				return
			}
			if landing.dropped != "" {
				// The contract removed this lens: the draw point must *not* call it
				// any more, and the pin named in `test` is what proves the new face.
				if landing.file == "" || landing.test == "" {
					t.Fatalf("a dropped control needs a file and the pin that replaces it: %+v", landing)
				}
				source, err := os.ReadFile(filepath.Join(root, "internal/shell", landing.file))
				if err != nil {
					t.Fatalf("read %s: %v", landing.file, err)
				}
				if landing.marker != "" && strings.Contains(string(source), landing.marker) {
					t.Errorf("%s draws %s again; the Raycast contract dropped that lens (%s)",
						landing.file, landing.marker, landing.dropped)
				}
				return
			}
			if landing.file == "" || landing.marker == "" {
				t.Fatalf("a landed control needs a file and a marker: %+v", landing)
			}
			// The named draw point must still call the lens. This is the
			// source-level half of the census; the pixel tests are the other.
			source, err := os.ReadFile(filepath.Join(root, "internal/shell", landing.file))
			if err != nil {
				t.Fatalf("read %s: %v", landing.file, err)
			}
			if !strings.Contains(string(source), landing.marker) {
				t.Errorf("%s no longer draws its lens rung: %s is gone (graded by %q)",
					landing.file, landing.marker, landing.test)
			}
		})
	}
}

// repoRoot walks up from the test's own directory to the module root (the one
// holding go.mod), so the census can read the source files it names.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}
