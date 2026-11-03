package launcher

// P13-eq · 过滤器 chips 行的两条契约（此前一条都没钉）。
//
// ① 点一枚 chip 之后，键盘**仍然落在字段上**：chips 是 `Focusable` 的（好让指针有
// 一个落点），框架点一下就会把焦点交给它，而本仓没有任何一处把焦点还给
// `a.Search`（`internal/launcher` 里只有 `FocusSearch` 一个 `Focus()` 调用点）。
// 于是「点一下 Text 过滤器，然后接着打字」是一条**没有任何测试守着的**路径：
// 它今天成立，是因为按键由外壳统一路由，而不是因为 chips 没抢焦点。
// ② chips 的词挂在**字段那一列**（`Space(4)` 的内缩）上，不挂在结果行标题那一列
// （列表 gutter + 行内 padding + 图标列 + gap = 54u）上。两条线差着整个图标列，
// 这是**有意的**：chips 是这张卡顶部 chrome 的一部分（与字段同一个内缩），
// 不是列表里的一行。下一轮若有人把它「对齐」到 54u，那是回归，不是修复。

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAChipClickKeepsTheFieldTyping(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	label := a.copy().ClipboardTypeText
	if err := tt.Click(label); err != nil {
		t.Fatalf("the chip %q: %v", label, err)
	}
	tt.Frame()
	if got, want := a.clipboardFilter, filterText; got != want {
		t.Fatalf("the chip chose %q, want %q", got, want)
	}
	tt.Type("zz")
	tt.Frame()
	if !strings.HasSuffix(a.Query, "zz") {
		t.Errorf("typing after a chip click did not reach the field: query %q", a.Query)
	}
}

func TestTheChipsHangOnTheFieldsColumnNotTheRows(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	chip, ok := tt.Find(a.copy().ClipboardFilterAll)
	if !ok {
		t.Fatalf("the chips row is missing: %q", tt.Texts())
	}
	rowTitle := ""
	for _, item := range a.Results() {
		if !item.heading && item.Title != "" {
			rowTitle = item.Title
			break
		}
	}
	if rowTitle == "" {
		t.Fatalf("no result row to compare against: %v", ids(a.Results()))
	}
	title, ok := tt.Find(rowTitle)
	if !ok {
		t.Fatalf("the first row's title %q is missing: %q", rowTitle, tt.Texts())
	}
	// 结果行标题那一列比 chips 右一整个图标列（`Space(7)` = 25.2px 起）：差不能是
	// 一两个像素的手抖，也不能缩成 0（那是「有人把 chips 对齐到了 54u」）。
	gap := title.X - chip.X
	if gap < 25 {
		t.Errorf("the row title starts at %v and the chips at %v (a %v gap): the chips must hang "+
			"left of the result column, on the field's own inset", title.X, chip.X, gap)
	}
	t.Logf("chip x %v, row title x %v (gap %v)", chip.X, title.X, gap)
}
