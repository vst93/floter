package ci

import (
	"strings"
	"testing"
)

// P12-a · 视觉合约升代后的两条账：新合约存在且被旧合约指向，且差距表的状态词
// 只有四个。
//
// 这一条不是设计切片，是**账**：`docs/liquid-glass.md` 是本仓视觉的新单一来源
// （用户 2026-10-30 换的总目标：细节打磨 + 轻便液态玻璃 + 空间感），而
// `docs/raycast-design.md` 降级为「行为契约 + 已落地条目」。升代最容易发生的
// 退化是**两份合约各自漂**：有人在 raycast 里改一个数值，有人又在 liquid-glass
// 里改另一个，第三个人读哪份都对。
//
// 所以钉两件事：
//
//	① `docs/raycast-design.md` 的顶部必须写明它已由 `liquid-glass.md` 升代，并且
//	   必须给出被取代的那两条（光泽上限、分层手段）——不是一句「见新文档」。
//	② `docs/liquid-glass.md` 的 §6 差距表里，每一行的状态只能是四个词之一
//	   （`未开始` / `已落地` / `需拍板` / `故意不做`）。第五个词出现 = 有人在表里
//	   写了「大概做了」这类无法验收的话。
func TestTheLiquidGlassContractSupersedesTheRaycastOne(t *testing.T) {
	raycast := repoFile(t, "docs/raycast-design.md")
	glass := repoFile(t, "docs/liquid-glass.md")
	head := raycast
	if i := strings.Index(head, "\n## §1"); i >= 0 {
		head = head[:i]
	}
	for _, want := range []string{
		"liquid-glass.md", // the pointer
		"升代",              // the verb
		"低光泽上限",           // the first superseded rule, named
		"分层靠面",            // the second superseded rule, named
	} {
		if !strings.Contains(head, want) {
			t.Errorf("the raycast contract's header does not say it was superseded (%q missing); "+
				"two visual sources drifting apart is the one thing this round must not allow", want)
		}
	}

	// ② 差距表的状态词。
	doc := glass
	if !strings.Contains(doc, "## §6 逐表面差距表") {
		t.Fatal("the liquid-glass contract has no §6 gap table: it is the round's queue")
	}
	allowed := map[string]bool{"未开始": true, "已落地": true, "需拍板": true, "故意不做": true}
	// Only the gap tables themselves: a table whose header row names a 状态
	// column. The contract has other tables (§0's four means, §1's three rungs)
	// whose last cell is prose, and reading those as statuses would be the
	// reader's bug, not the document's.
	lines := strings.Split(doc[strings.Index(doc, "## §6 逐表面差距表"):], "\n")
	rows, withStatus := 0, 0
	inTable, statusColumn := false, -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			inTable, statusColumn = false, -1
			continue
		}
		fields := strings.Split(trimmed, "|")
		if !inTable {
			inTable = true
			statusColumn = -1
			for i, f := range fields {
				if strings.TrimSpace(f) == "状态" {
					statusColumn = i
				}
			}
			continue
		}
		if statusColumn < 0 || strings.HasPrefix(trimmed, "|---") {
			continue
		}
		rows++
		if statusColumn >= len(fields) {
			t.Errorf("a gap-table row has fewer cells than its header names (line: %s)", trimmed)
			continue
		}
		last := strings.TrimSpace(fields[statusColumn])
		if last == "" {
			continue
		}
		withStatus++
		ok := false
		for word := range allowed {
			if strings.Contains(last, word) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("a gap-table row carries an unrecognised status %q; the four words are 未开始 / 已落地 / 需拍板 / 故意不做 (line: %s)",
				last, trimmed)
		}
	}
	if rows == 0 || withStatus == 0 {
		t.Fatalf("the gap table parsed %d rows (%d with a status): the reader found nothing to check", rows, withStatus)
	}
	t.Logf("the gap table has %d rows, %d of them carry a status", rows, withStatus)
}
