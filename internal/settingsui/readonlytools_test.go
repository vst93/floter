package settingsui

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// P13-dl · 工具编辑器里两块**只读面**：后端铸的集成 id（`keyValue`）与权限边界那句
// 散文（`PermissionBoundary`）。P13-dk 的「仍待」说这两块的**盒子**没人量过——
// 本轮先量。
//
// 量下来一块是对的、一块是坏的：
//
//	集成 id：名字 caption / 500 / `TextSecondary`（浅 122 / 深 115），值
//	         emphasis / 580 / `TextStrong`（浅 202 / 深 198），值落在行尾、可选中
//	         —— P13-ak 的「只读也算答案」在这一行已经成立，**故意不改**。
//	边界那句：它是一整句散文（英 107 字），却走 `a.row(c, "", note, …)` ——
//	         **空标签**占着 emphasis 那行，句子被塞进第二行槽，那个槽是
//	         `SingleLine()` 的：720 宽下文字列 448、墨到 375 就断，尾列只有
//	         2 像素高 —— 框架的省略号。句子的后半段
//	         「These controls do not create a sandbox.」从来没被画出来过，
//	         而它是这段披露里唯一诚实的那半句（这句话的原话：
//	         `settings.extensions.permissionBoundary`）。
//
// 所以这一族只有一处要改：散文**要换行**，而且它不该穿着「一行设置」的框
// （44px 地板 + 一条发丝线 + 空标签行 + 因为嵌在 `below` 列里而多缩进的那 14px）。
// 改法是本仓已有的那条散文档 —— `sectionHint`（body / 500 / `t.TextMuted` / 整宽），
// **墨一个字节都没换**（浅 101 / 深 94，改前改后同值），换的是排版：句子从 1 行
// 截断变成 2 行读完，盒高 17 → 34，而那 19px 的空标签行没有了。

// toolFormEdit renders the integration form in edit mode with the Advanced
// disclosure open — the one state where both read-only faces are drawn.
func toolFormEdit(t *testing.T, w, h int, dark bool) *ui.Tester {
	t.Helper()
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(CustomTool) {},
		EditCustomTool:   func(string, CustomTool) {},
		ToolForEdit: func(id string) (CustomTool, bool) {
			return CustomTool{Name: "Ripgrep", Program: "/usr/bin/rg", Command: "rg"}, id == "local.abc"
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "local.abc", Name: "Ripgrep", Enabled: true, Running: true, Generated: true}}
	}
	tt, _ := renderIn(t, a, w, h, dark)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Edit\u2026"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	tt.Frame()
	if err := tt.Click(en.LocalToolAdvanced); err != nil {
		t.Fatalf("open advanced: %v", err)
	}
	tt.Frame()
	return tt
}

// glyphHeight is the painted height of the line's last ink column: a real
// letter is a handful of pixels tall, the framework's ellipsis is two
// (`rowlinecut_test.go` measured exactly that, 2 against 191).
func glyphHeight(tt *ui.Tester, box ui.Rect) int {
	cols := lineInk(tt.Image(), box)
	if len(cols) == 0 {
		return 0
	}
	last := cols[len(cols)-1]
	return last.Bottom - last.Top + 1
}

// TestThePermissionBoundaryNoteIsReadEndToEnd is the round's pin. Before P13-dl
// the note was one line of a 17px-high box, its ink stopped at column 375 of
// 448, and its last column was the ellipsis (2px). A sentence about a sandbox
// that ends at "These controls do not" says the opposite of what it means.
func TestThePermissionBoundaryNoteIsReadEndToEnd(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := appearanceName(dark)
		tt := toolFormEdit(t, 720, 2200, dark)
		note := mustFind(t, tt, en.PermissionBoundary)
		// The disclosure's own name sits in the card's content column; prose
		// that explains the block belongs to the same column, not one inset
		// further in (the old nested row drew at x=215.8 against the name's
		// 201.8).
		heading := mustFind(t, tt, en.LocalToolAdvanced)
		if d := note.X - heading.X; d > 0.5 || d < -0.5 {
			t.Errorf("%s: the note starts at x=%v, the disclosure's name at x=%v (Δ%v)",
				appearance, note.X, heading.X, d)
		}
		// Two lines: the English sentence does not fit one 487px line at the
		// hint step, and it must not pretend it does.
		if note.H < 30 {
			t.Errorf("%s: the note box is %v tall — one line, so the sentence is cut", appearance, note.H)
		}
		if got := glyphHeight(tt, note); got < 4 {
			t.Errorf("%s: the note's last ink column is %dpx tall — that is the ellipsis, not a letter", appearance, got)
		}
		// Ink is unchanged by this round: the prose rung, both appearances.
		span, at := inkPixel(tt, note, dark)
		want, wantLuma := 101, [3]uint8{128, 128, 135}
		if dark {
			want, wantLuma = 94, [3]uint8{133, 133, 141}
		}
		if span != want || at != wantLuma {
			t.Errorf("%s: the note resolves ink %d %v, want the hint rung %d %v",
				appearance, span, at, want, wantLuma)
		}
		t.Logf("%s: note box %v ink %d %v last glyph %dpx", appearance, note, span, at, glyphHeight(tt, note))
	}
}

// TestThePermissionBoundaryNoteIsNotARow pins what the fix removed: the note is
// no longer a 44px card row with a hairline above it and an empty emphasis line
// inside it. Measured from the disclosure's own name to the first switch's
// label: 74px before (913.3 → 987.3) and 71px after (913.3 → 984.3) — the block
// is 3px shorter while carrying 17px more text. What must not come back is the
// 44px floor under a single line, so the gap is pinned at its measured value.
func TestThePermissionBoundaryNoteIsNotARow(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := appearanceName(dark)
		tt := toolFormEdit(t, 720, 2200, dark)
		heading := mustFind(t, tt, en.LocalToolAdvancedPermissions)
		first := mustFind(t, tt, i18n.PermissionLabel("en", extensions.AllPermissions[0]))
		gap := first.Y - (heading.Y + heading.H)
		if gap > 71.5 || gap < 70.5 {
			t.Errorf("%s: the disclosure's name to the first switch is %v, want 71 (P13-dl 重定基线：旧值 74)",
				appearance, gap)
		}
		// The first switch keeps its own leading hairline: the note stopped
		// being a row, the switches are still a ruled list.
		t.Logf("%s: heading %v, first switch %v, gap %v", appearance, heading, first, gap)
	}
}

// TestTheIntegrationIdRowIsAValueNotASetting measures the other read-only face
// and pins its ink.
//
// P13-do 重定基线（`docs/liquid-glass.md` §6.84）：旧值 → 新值。这条 pin 此前把
// 只读 ID 行**单独**量了一次（`keyValue` 的层级：名字 caption 122 / 值 emphasis
// 202，深 115 / 198）就结了案，没有把它和它下面那张表里的行放在一起看。同一张
// 卡里可编辑行的标题是 `rowTitle`（emphasis / 580 / `TextStrong`，202 / 198），
// 于是同一列同一行距上，标题差 80 级、值却比每个字段里的字都响 —— 那是 About
// 事实表的层级（§6.11：那张卡没有一行按得动），不是设置表单的层级。
//
// 现在这一行按表单自己的语法画：名字是这张表每一行的标题（emphasis / 580 /
// `TextStrong`，202 / 198），值是程序自己起的名那一档（mono caption / 500 /
// `TextMuted`，101 / 94，与 `rowTitle` 把值放进控制槽时同一档墨，也同 P13-bt
// §6.38 的 `dataRow`）。只读的记号只剩结构：这行没有控件面。
//
// 旧断言 → 新断言：name 122 [107 107 112] → 202 [27 27 29]（深 115 [154 154 160]
// → 198 [237 237 239]）；value 202 [27 27 29] → 101 [128 128 135]（深 198
// [237 237 239] → 94 [133 133 141]）。「值在行的末端」那条一字未动。
func TestTheIntegrationIdRowIsAValueNotASetting(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := appearanceName(dark)
		tt := toolFormEdit(t, 720, 2200, dark)
		label := mustFind(t, tt, en.LocalToolId)
		value := mustFind(t, tt, "local.abc")
		nameSpan, nameAt := inkPixel(tt, label, dark)
		valueSpan, valueAt := inkPixel(tt, value, dark)
		wantName, wantNameAt := 202, [3]uint8{27, 27, 29}
		wantValue, wantValueAt := 101, [3]uint8{128, 128, 135}
		if dark {
			wantName, wantNameAt = 198, [3]uint8{237, 237, 239}
			wantValue, wantValueAt = 94, [3]uint8{133, 133, 141}
		}
		if nameSpan != wantName || nameAt != wantNameAt {
			t.Errorf("%s: the id's name resolves %d %v, want the form row's title rung %d %v (§6.84)",
				appearance, nameSpan, nameAt, wantName, wantNameAt)
		}
		if valueSpan != wantValue || valueAt != wantValueAt {
			t.Errorf("%s: the id's value resolves %d %v, want the data rung %d %v (§6.84)",
				appearance, valueSpan, valueAt, wantValue, wantValueAt)
		}
		// The value is the row's *answer*, so it ends the row: it starts to the
		// right of the name's half of the row rather than beside it.
		if value.X < label.X+label.W+200 {
			t.Errorf("%s: the id value starts at x=%v, too close to its name (x=%v w=%v)",
				appearance, value.X, label.X, label.W)
		}
		t.Logf("%s: id name %v ink %d %v | id value %v ink %d %v",
			appearance, label, nameSpan, nameAt, value, valueSpan, valueAt)
	}
}

// TestDumpP13dlPermissionNote writes the block's before/after frames. The
// `FLOTER_P13DL_TAG` suffix lets the same dump run against the reverted call so
// the two passes differ only in the code under test:
//
//	FLOTER_P13DL_DUMP=1 go test -count=1 -run TestDumpP13dlPermissionNote ./internal/settingsui/
func TestDumpP13dlPermissionNote(t *testing.T) {
	if os.Getenv("FLOTER_P13DL_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13DL_DUMP=1 to write /tmp/floter-P13-dl-boundary-*.png")
	}
	tag := os.Getenv("FLOTER_P13DL_TAG")
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		tt := toolFormEdit(t, 720, 2200, dark)
		note := mustFind(t, tt, en.PermissionBoundary)
		name := fmt.Sprintf("/tmp/floter-P13-dl-boundary-%s%s.png", appearanceName(dark), tag)
		f, err := os.Create(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		img := tt.Image()
		top := int(note.Y) - 60
		if top < 0 {
			top = 0
		}
		sub := img.SubImage(image.Rect(160, top, img.Bounds().Dx()-160, int(note.Y+note.H)+70))
		if err := png.Encode(f, sub); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
}
