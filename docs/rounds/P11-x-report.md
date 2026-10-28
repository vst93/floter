# P11-x 报告：一个插件设置字段的可选值只有一个答案

规格 `docs/rounds/P11-x-spec.md`。方向按用户定的「三平台一致性 → 基础功能 → 像素细节」，
落在**基础功能**，接 P11-w「一个值域的 id 清单只有一个答案」的「仍待」第 4 条。

## 1. 本轮做了什么（增量记录）

- `internal/valuedomain`：新增三张 id 表 + 七个具名常量 —— `BrowserSortOrders`
  （`relevance/recent/alphabetical/visits`）、`BrowserSearchFields`（`all/title/url`）、
  `CalculatorCopyModes`（`full/result`）。序＝拾取器序。
- `internal/settings/browser.go`：删掉 `BrowserSortOrders`/`BrowserSearchFields` 两个**死**
  `[]string`（全仓只有定义处）与 `browserSortOrderSet`/`browserSearchFieldSet` 两个 set map；
  两个默认值改读 `valuedomain`；两个归一化改成 `Pick(…)`（与 `NormalizeTheme` 同一写法）。
- `internal/settings/calculator.go`：两个模式常量 = `valuedomain` 的同名常量；
  `NormalizeCalculatorCopyMode` 从「只比较 `result`」改成 `Pick(…)`。
- `internal/browser/search.go`：`SearchField`/`SortOrder` 常量的值改读 `valuedomain`（零字面量）；
  `orderResults` 的 `switch`+`default` 换成 `sortOrders` 比较器表（`relevance` 故意不在表里，
  它就是合并顺序）；字段判定收成 `fieldColumns` + `columnsFor`，`matchesTerms` 与
  `matchesClause` 都读它（此前一处是 `switch`、一处是两个 `==` 的组合）。
- `internal/calculator/calculator.go`：`CopyFull`/`CopyResult` = `valuedomain` 常量。
- `internal/i18n`：4 张 `[]Option` 字面量（en/zh × 浏览器排序、搜索字段、计算器复制模式）
  → `optionsFor(valuedomain.X, …)`；标签逐字搬进 `internal/i18n/options.go`（文案零改动）。
- pin：`internal/browser/domain_test.go`（3 条新函数）、`internal/i18n/options_test.go` 两条既有
  测试各 +3 条 domain、`internal/ci/valuedomain_contract_test.go` 的源码扫描 +3 个禁用字面量、
  加载器一致性 +3 条、新增 `TestTheCalculatorCopyModesHaveOneAnswer`。

## 2. V / B / S 台账

- **V = 0**：非视觉轮，零像素/颜色/尺寸改动，不出图。
- **B = 0**：既有断言零改动、零删除；`internal/settings`、`internal/settingsui`、
  `internal/plugincfg`、`internal/shell`、`internal/launcher` 的产品代码与测试都没碰，
  全绿。落盘 id、拾取器行序、标签文本、默认值、归一化的接受集合与回落值逐条不变。
- **S = 4**（结构真变了，均无既有断言需要改）：
  1. 三个插件值域的 5 处拼写（settings 的死 `[]string` + set map、`internal/browser` 的常量字面量、
     `internal/calculator` 的常量、`internal/i18n` 两张表 × 两种语言）→ `internal/valuedomain`
     各一张表 + 具名常量。
  2. `orderResults` 的 `switch` + `default` → `sortOrders` 比较器表；`relevance` 不在表里是
     文档化的 no-op，不再是「未知 id 的落点」。
  3. 字段判定从两处（`matchesTerms` 的 `switch`、`matchesClause` 的两个 `==` 组合）→
     `fieldColumns` + `columnsFor` 一处。
  4. `internal/i18n` 4 张 `[]Option` 字面量 → `optionsFor(valuedomain.X, labels)`
     （P11-w 同族最后三张表收尾）。

## 3. 门槛实测（本轮收尾，全量复跑）

| 门槛 | 实测 |
| --- | --- |
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `GOOS=windows go vet ./...` | 净 |
| `GOOS=darwin go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿，**42 ok** + 2 no test files，0 FAIL |
| `--- PASS` | **2343**（起点实测 2339，+4） |
| `--- SKIP` | **15**（无新增） |
| 测试函数 | **1902**（起点 1898，+4：browser 3 + ci 1；i18n 两条既有测试各扩 3 条 domain，不增函数） |
| 三平台 `go build ./...` | 过 |

产品代码净 **+243 / −157**（大头是 `search.go` 的比较器表与 `valuedomain.go` 的注释）。

## 4. mutation（4 条实跑变红 + 逐字节还原 + touch 复跑）

1. `valuedomain.BrowserSortOrders` 加一个 `"pinned"` →
   `internal/browser`: `TestEveryOfferedSortOrderIsImplemented` 红
   （`3 comparators for 5 offered orderings` / `ordering "pinned" implemented = false, want true`）；
   `internal/i18n`: `TestPickerOptionsMatchTheStoredDomains` 红
   （`en/browser sort orders: id "pinned" has no label`）、`TestPickerOrderIsTheShippedOrder` 红
   （`browser sort orders: 5 rows, want 4`）、`TestEveryLanguageIsComplete` 红
   （`BrowserSortOrders[4].Label is declared but empty`）。
2. `settings.NormalizeBrowserSortOrder` 只回 `relevance` →
   `internal/ci`: `TestTheLoaderAndThePickableDomainsAgree` 红
   （`browser sort orders: the loader turns the offered id "recent" into "relevance"`）；
   同时既有 B pin `TestBrowserPluginDefaultsAndRoundTrip` 也红（`SortOrder:relevance`，证明旧 pin 也在这个面上）。
3. `internal/browser` 的 `fieldColumns` 删掉 `FieldTitle` →
   `TestEveryOfferedSearchFieldIsImplemented` 红（`field "title" has no column rule, so it silently matches everything`）、
   `TestSearchMatchesAndOrders` 与 `TestMatchesClause` 红。
4. `settings/browser.go` 写回 `[]string{"relevance", "recent", "alphabetical", "visits"}` →
   `internal/ci`: `TestValueDomainsHaveOneAnswer` 红
   （`internal/settings/browser.go:31 spells a value domain again ([]string{"relevance", "recent")`）。

每条：注入 → 看到红 → `cp` 还原 → `md5sum -c` 三文件逐字节核对（全部「成功」）→ `touch` 后
复跑受影响包，全绿。收尾再全量复跑一次，PASS 仍是 2343。

## 5. 仍待（P11-x）

1. 用户实机复测四条，连续第 79 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器 `loadLocked`
   + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序 + `Esc`/`Escape`
   + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS` + `Name` 缺失回落文件名主干
   + `Hidden`/`NoDisplay` 大小写）。
3. `internal/theme.Resolve`（`internal/theme/theme.go:38-50`）仍是对 `auto`/`light`/`dark` 的
   `switch`——消费者不是清单，本轮仍未动（P11-w 记过，继续挂着）。
4. **下一处候选（同族最后一张表）**：`i18n.ShortcutsActions`（`internal/i18n/copy.go` 的
   `plugin:clipboard`…`command` 八个 id）。它与本轮三张表不同形：id 带 `plugin:`/`action:`
   前缀语法（`settings.ClassifyCustomShortcut`），消费端是 `internal/shell/custom.go` 的
   dispatch `switch` 加 `internal/settingsui/custom.go` 的「Command line…」模式，且
   `settings` 侧是八个独立常量而不是一张表。要做就得同时收「前缀语法 + 派发清单」，
   是本轮 1.5–2 倍工作量，留作单独一轮。
5. `internal/settings.CalculatorRetentionDays`（`[]int{0,1,7,30}`）与
   `internal/calculator.RetentionDays` 是同一批窗口的两份拷贝——**值域但不是字符串 id**
   （拾取器把 int 格式化成文案，`i18n` 没有对应 `[]Option`），与本轮判据相邻但不同形，
   未动；若要收，需要先决定「0=永不」这类哨兵值算不算值域成员。
