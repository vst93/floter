package settingsui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P13-by · 标题、正文与存进设置的那个 id，三处读同一个夹取。
//
// `body` 的 `switch` 此前有一个 `default` 臂，画的是 `copy.PagePlaceholder`
// （「此页面将在后续迭代中到来。」）——而同一帧里页头用的是 `a.page(a.Page)`，
// 它的 `default` 臂返回**第一页**。也就是说一个越界页号会让页头写「常规 / 外观、
// 启动方式与窗口行为。」，正文却写「此页面将在后续迭代中到来。」：一条走不到的
// 分支，而它说的是另一页。
//
// 这个 pin 不读注释、也不渲染：它只要求三处夹取在**每一个**整数上都给出同一页，
// 于是「正文画哪一页」与「标题写哪一页」再也不可能分家。
func TestAnUnknownPageDrawsTheSamePageItsTitleNames(t *testing.T) {
	// The whole valid range: every real page is itself in all three readers.
	for i := 0; i < pageCount; i++ {
		if got := bodyPage(i); got != i {
			t.Fatalf("bodyPage(%d) = %d; a page this build knows must be drawn as itself", i, got)
		}
		if got, want := pageNames[bodyPage(i)], PageName(i); got != want {
			t.Fatalf("page %d: body draws %q, the stored id says %q", i, got, want)
		}
	}
	// And the edges: an index this build does not know resolves to the first
	// page everywhere, so the header and the body can never disagree.
	for _, i := range []int{-1, -7, pageCount, pageCount + 1, 99} {
		if got, want := pageNames[bodyPage(i)], PageName(i); got != want {
			t.Errorf("page %d: body draws %q, the header/stored id says %q", i, got, want)
		}
	}
}

// TestNoPageSaysItIsNotBuiltYet is the copy half of the same账: this app has no
// page that tells the user it is not finished. The sentence lived in
// `i18n.Copy.PagePlaceholder`, read by that one unreachable arm; with the arm
// gone the field went with it, and this pin keeps a future page from bringing
// the sentence back. It greps the shipped tables rather than the struct: a
// field that is re-declared but unused is `i18n`'s own dead-copy guard's job,
// while a live page *using* the sentence would pass that guard.
func TestNoPageSaysItIsNotBuiltYet(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "i18n", "copy.go"))
	if err != nil {
		t.Fatalf("read the copy tables: %v", err)
	}
	for _, phrase := range []string{"arrives in a later round", "将在后续迭代中到来"} {
		if strings.Contains(string(source), phrase) {
			t.Errorf("i18n still ships %q: a page of this build may not say it is not built yet", phrase)
		}
	}
}
