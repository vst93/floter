package i18n

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// P11-bv · **两种语言同字的显示文案只有这四类，多一类就红。**
//
// `TestEveryLanguageIsComplete` 只保证 zh 不空，`TestNoDeadCopy` 只保证每个键有人在读。
// 两者都不回答第三个问题：一段**显示文案**在 zh 里原样留着英文，谁也发现不了 —
// 用户读的是 zh（P11-ba），这类漏译在界面上就是中文界面里冒出一句英文。
//
// 全表的实测：en 与 zh 逐字节相同的叶子**恰好 52 条**，且全部落在四类里 —
// 47 条是存进设置文件的 **ID**（`tiny` / `dark` / `plugin:clipboard`，翻译了就读不回来）、
// 2 条是语言自己的名字（`English` / `中文`，任何语言下都写自己）、
// 1 条是品牌（`floter`），
// 2 条是应用图标档位的显示词（`Dark` / `Light`）——这是唯一一处**真的**英文进了中文界面，
// 属 `.pending-bugs.md` 里的用词拍板项，本测试把它钉成"已知债务"，多一处就红。
//
// 判据是路径形状，不是字符串内容：新增的 ID 自动归类，新增的显示文案必须翻译。
// 钉住的方向只有一个——同字的集合只许**缩小**：多一处显示文案同字，它就不再能
// 落进这四类，测试红。
func TestOnlyIdentifiersAndNamesAreSharedByBothLanguages(t *testing.T) {
	shared := sharedLeaves("Copy", reflect.ValueOf(en), reflect.ValueOf(zh))
	if len(shared) == 0 {
		t.Fatal("两种语言没有任何同字叶子；遍历没跑起来")
	}
	// The walk is only meaningful if it really crossed the whole table: the
	// identifier families alone are 47 leaves.
	if len(shared) < 47 {
		t.Fatalf("同字叶子只有 %d 条，遍历漏了分支：%v", len(shared), shared)
	}
	for _, path := range shared {
		switch classifyShared(path) {
		case sharedIdentifier, sharedEndonym, sharedBrand:
		case sharedPinnedWord:
			t.Logf("%s 仍是同一个词（用词拍板项，见 .pending-bugs.md）", path)
		default:
			t.Errorf("%s 在 zh 里与 en 同字：这是一段显示文案，没有翻译", path)
		}
	}
}

// sharedKind says why two languages may spell one leaf the same way.
type sharedKind int

const (
	sharedUnknown sharedKind = iota
	// sharedIdentifier is a value the app stores and looks up (`tiny`, `dark`,
	// `plugin:clipboard`): translating it would break the lookup, so both
	// languages ship the id.
	sharedIdentifier
	// sharedEndonym is a language's own name; it reads the same in every
	// language (`English`, `中文`).
	sharedEndonym
	// sharedBrand is the app's name.
	sharedBrand
	// sharedPinnedWord is display copy that has not been translated — the one
	// known debt, pinned rather than hidden.
	sharedPinnedWord
)

// classifyShared decides which leaves a language may leave untranslated.
func classifyShared(path string) sharedKind {
	leaf := path[strings.LastIndex(path, ".")+1:]
	// The segment that owns the leaf: `Languages[0]` for
	// `Copy.Settings.Languages[0].Label`.
	owner := ""
	if i := strings.LastIndex(path, "."); i > 0 {
		owner = path[strings.LastIndex(path[:i], ".")+1 : i]
	}
	switch {
	case leaf == "ID":
		return sharedIdentifier
	case leaf == "Label" && strings.HasPrefix(owner, "Languages["):
		return sharedEndonym
	case path == "Copy.Notifications.Title":
		return sharedBrand
	case leaf == "Label" && strings.HasPrefix(owner, "AppIcons["):
		return sharedPinnedWord
	}
	return sharedUnknown
}

// sharedLeaves lists, sorted, the string leaves two copies spell identically.
func sharedLeaves(path string, a, b reflect.Value) []string {
	var out []string
	switch a.Kind() {
	case reflect.String:
		if a.String() == b.String() {
			out = append(out, path)
		}
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			out = append(out, sharedLeaves(fmt.Sprintf("%s.%s", path, a.Type().Field(i).Name), a.Field(i), b.Field(i))...)
		}
	case reflect.Slice:
		for i := 0; i < a.Len(); i++ {
			out = append(out, sharedLeaves(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i))...)
		}
	}
	sort.Strings(out)
	return out
}
