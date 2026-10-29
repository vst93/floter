package launcher

import (
	"testing"
)

// P11-cd · **一个什么都没问的空页不报「没有匹配项」。**
//
// `len(results) == 0` 这一条分支同时接住两种完全不同的页面：
//
//   - 有 needle、搜索真的没命中 —— 该说「没有匹配项」（`copy.NoResults`）；
//   - 没有 needle、而且一条最近启动都没有（全新的 floter，或最近记录被清空）——
//     什么都没问，页面印一句「没有匹配项」是在**否认一次没发生的搜索**。字段自己
//     的占位句（`copy.Placeholder`）已经在那儿说「输入命令或应用名称」。
//
// 参考的安静空页也是这么分的：没有 needle 就没有空态句。P11-bh 给「没有拖入文件」
// 另立一句话是同一条规则的另一半（没有 needle 的页面不借 NoResults）。
func TestAnEmptyQueryNeverClaimsNoResults(t *testing.T) {
	a := testApp() // no recents, no tools: the first launch's own page
	copy := StringsFor(a.settings().Language)
	if got := a.emptyMessage(copy); got != "" {
		t.Errorf("an empty query with no recents has no sentence to say, got %q", got)
	}
	tt := render(t, a)
	if tt.HasText(copy.NoResults) {
		t.Errorf("the page denies a search nobody made: %q", tt.Texts())
	}
	if !tt.HasText(copy.Hint) {
		t.Errorf("the field's own guidance is gone too, the page says nothing: %q", tt.Texts())
	}

	// The other half: a needle that matches nothing still says so.
	a2 := testApp()
	copy2 := StringsFor(a2.settings().Language)
	a2.Query = "zzzz"
	tt2 := render(t, a2)
	if got := a2.emptyMessage(copy2); got != copy2.NoResults {
		t.Errorf("a real miss says %q, want %q", got, copy2.NoResults)
	}
	if !tt2.HasText(copy2.NoResults) {
		t.Errorf("a real miss drew no sentence: %q", tt2.Texts())
	}
}
