# P11-x 规格：一个插件设置字段的可选值只有一个答案

## 意图

「浏览器结果怎么排 / 搜索匹配哪一段 / 计算器回车复制什么」这三问，每个答案都是一个
**落盘的字符串 id**，而它们的 id 清单今天各有 3–5 份拷贝。本轮把这三张清单收进
`internal/valuedomain`，让「加一个选项要改几处」变成「改一张表 + 该消费点自己的实现」，
并把「漏改一处不会报错」的地方补上 pin。

为什么现在做：这是 P11-w「一个值域的 id 清单只有一个答案」的**直接续集**（该轮报告
「仍待」第 4 条点名了这几张表），同族、同形状、同风险，且是用户定的
「三平台一致性 → 基础功能 → 像素细节」里的**基础功能**。

## 证据（行号为本轮实测）

1. **同一份清单在同一个文件里写了两遍**
   - `internal/settings/browser.go:28-31`：`BrowserSortOrders`（`[]string`）、
     `BrowserSearchFields`（`[]string`）与 `browserSortOrderSet`、`browserSearchFieldSet`
     （`map[string]bool`）是同一批 id 的两种写法。
   - **而且这两个 `[]string` 是死代码**：全仓 `grep` 只有定义处（`internal/settings` 的测试
     也没读它们），拾取器读的是 `internal/i18n` 的表，加载器读的是 set map。
2. **第二份消费者是带类型的常量，且不认识新 id 时静默降级**
   - `internal/browser/search.go:16-24`（`FieldAll/FieldTitle/FieldURL`）与 `:27-36`
     （`SortRelevance/SortRecent/SortAlphabetical/SortVisits`）把同一批 id 再拼一遍；
     `internal/shell/shell.go:2152-2153` 把落盘字符串**直接强制转换**成这两个类型。
   - 降级路径：`orderResults`（`search.go:158-186`）的 `default` 把未知排序当成
     `relevance`（合并顺序），`matchesTerms`（`:128-152`）与 `matchesClause`（`:199-232`）
     把未知字段当成 `all`。**加一个 id 而忘了这里的 case，就是「选得中、存得下、画错结果、
     没有测试红」。**
3. **计算器复制模式有三处拼写**
   - `internal/settings/calculator.go:34-39` 的 `CalculatorCopyFull/Result`、
     `internal/calculator/calculator.go:56-60` 的 `CopyFull/CopyResult`、
     `internal/i18n/copy.go:879-883` 的选项表。
   - 加载器 `NormalizeCalculatorCopyMode`（`settings/calculator.go:109-119`）只和白名单里的
     一个常量比较，其余一律回 `full`：新增第三种模式而漏改这里＝**用户的选择被静默改写**。
4. **拾取器的表按语言各写一遍**：`internal/i18n/copy.go:845-857`（en）、`:1691-1703`（zh）、
   `:879-883`（en 计算器）、`:1725-1729`（zh 计算器）。P11-w 已把设置页的 7 张表改成
   `optionsFor(valuedomain.X, labels)`，这 3 张是剩下的同类表。

三处今天**恰好同答案**，所以本轮不是修可见 bug，是消掉「加一个选项要改 3–5 处、漏一处不报错」
的形状——与 P11-w 同一判据。

## 落地项（位置 → 现状 → 应改成）

### 1. `internal/valuedomain/valuedomain.go`

- 现状：8 张 id 表 + `ScaleFactors` + `Has` + `NormalizeAppIcon` + `DefaultAppIcon`。
- 改成：新增三张表，序＝拾取器序，并给每个 id 一个具名常量（让别的包不必再写字符串字面量）：
  ```go
  const (BrowserSortRelevance="relevance"; BrowserSortRecent="recent";
         BrowserSortAlphabetical="alphabetical"; BrowserSortVisits="visits")
  var BrowserSortOrders = []string{BrowserSortRelevance, ...}
  const (BrowserFieldAll="all"; BrowserFieldTitle="title"; BrowserFieldURL="url")
  var BrowserSearchFields = []string{BrowserFieldAll, ...}
  const (CalculatorCopyFull="full"; CalculatorCopyResult="result")
  var CalculatorCopyModes = []string{CalculatorCopyFull, CalculatorCopyResult}
  ```
  包注释补一句：这三张是**插件设置块**的值域（落盘在 `browser_plugin` / `calculator_plugin`
  里），与设置页字段同一判据。

### 2. `internal/settings/browser.go`

- 现状：`BrowserSortOrders`/`BrowserSearchFields` 两个死 slice + 两个 set map；
  `DefaultBrowserSortOrder="relevance"`、`DefaultBrowserSearchField="all"` 字面量。
- 改成：删两个 slice 与两个 set map；默认值 = `valuedomain.BrowserSortRelevance` /
  `valuedomain.BrowserFieldAll`；`NormalizeBrowserSortOrder`/`NormalizeBrowserSearchField`
  改为 `Pick(valuedomain.BrowserSortOrders, 小写去空格, 默认)`（与 `NormalizeTheme` 同一写法）。
- 行为零改动：接受集合、拒绝后的落点、大小写/空格宽容度都不变。

### 3. `internal/settings/calculator.go`

- 现状：`CalculatorCopyFull/Result` 字面量；`NormalizeCalculatorCopyMode` 只比较 `result`。
- 改成：两个常量 = `valuedomain.CalculatorCopyFull/Result`；归一化改为
  `Pick(valuedomain.CalculatorCopyModes, 小写去空格, CalculatorCopyFull)`。
- 行为零改动：`" Result "` 仍落 `result`，未知仍落 `full`。

### 4. `internal/browser/search.go`

- 现状：`SearchField`/`SortOrder` 常量的值各写一遍；`orderResults` 是 `switch` + `default`；
  字段判定散在 `matchesTerms`（switch）与 `matchesClause`（两个 `==` 组合）。
- 改成：
  - 常量值 = `valuedomain.BrowserSortX` / `valuedomain.BrowserFieldX`（零字面量）。
  - `var sortOrders = map[SortOrder]func(a, b Result) bool{…}`，**`relevance` 故意不在表里**
    （它就是合并顺序，一个 no-op）；`orderResults` = `less, ok := sortOrders[order]; if !ok { return }`。
  - `var fieldColumns = map[SearchField]struct{ URL, Title bool }{…}` + `columnsFor(field)`
    作为唯一判据，`matchesTerms` 与 `matchesClause` 都读它；未知字段仍落「两列都匹配」（＝`all`，
    保持现有行为，注释写明这是文档化的回落而不是第二个答案）。
- 行为零改动：三种排序的比较器逐字搬进表；三种字段的列选择逐字搬进表。

### 5. `internal/calculator/calculator.go`

- 现状：`CopyFull/CopyResult` 字面量。
- 改成：= `valuedomain.CalculatorCopyFull/Result`。`Entry.Text(mode)` 的比较逻辑不动。

### 6. `internal/i18n`

- `options.go`：新增 `enBrowserSortOrderLabels`/`zhBrowserSortOrderLabels`、
  `enBrowserSearchFieldLabels`/`zhBrowserSearchFieldLabels`、
  `enCalculatorCopyModeLabels`/`zhCalculatorCopyModeLabels`（标签逐字搬，不译不改）。
- `copy.go`：4 处 `[]Option{…}` 字面量（en 两张、zh 两张）→
  `optionsFor(valuedomain.BrowserSortOrders, …)` 等。

### 7. pin（这是本轮的另一半）

- `internal/i18n/options_test.go`：`TestPickerOptionsMatchTheStoredDomains` 的 domain 列表 +
  3 条；`TestPickerOrderIsTheShippedOrder` 的期望行 + 3 条（钉住标签文本与行序）。
- `internal/ci/valuedomain_contract_test.go`：
  - 禁用字面量列表新增 `[]string{"relevance", "recent"`、`[]string{"all", "title", "url"}`、
    `[]string{"full", "result"}`（源码扫描，`internal/` + `cmd/`，排除 `_test.go` 与
    `internal/valuedomain`）。
  - `TestTheLoaderAndThePickableDomainsAgree` 的 domain 表 + 3 条（加载器接受每一个被提供的
    id，且拒绝一个未提供的 id）。
- `internal/browser/search_test.go`（新测试）：
  - `TestEveryOfferedSortOrderIsImplemented`：`valuedomain.BrowserSortOrders` 里除
    `relevance` 外每个 id 必须在 `sortOrders` 表里，`len(sortOrders)+1 == len(表)`，
    且表里没有表外 id。
  - `TestEveryOfferedSearchFieldIsImplemented`：`fieldColumns` 的键集合 == 表，三种字段的
    (URL,Title) 组合两两不同（这样「加一档却复用了 all 的列选择」会被抓）。
  - `TestTheTypedConstantsSpellTheDomain`：四个 `SortOrder` 常量与三个 `SearchField` 常量
    逐个等于 `valuedomain` 的对应常量（防止有人把常量值改回去）。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**零改动**。
- 落盘 id 字符串、拾取器行序与标签、默认值、归一化的接受集合与回落值**全部不变**。
- `internal/settingsui`、`internal/plugincfg`、`internal/shell` 的产品代码不改；它们的测试
  **零改动**必须全绿（B 台账）。
- 非视觉轮：不出图（无像素改动）。

## 门槛与基线（本轮开工前实测）

| 门槛 | 实测基线 |
| --- | --- |
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 |
| `go test -count=1 ./...` | 全绿，**2339 PASS** / 15 SKIP / 0 FAIL |
| 测试函数数 | **1898** |
| 三平台 `go build ./...` | 过 |

验收：PASS 只许涨（预计 +5～7：i18n 两条既有测试扩容不增函数、browser 3 条新函数），
SKIP 不得增加，函数数不得减少，三平台 vet/build 净。

## mutation 计划（≥2 条，每条说明哪个 pin 变红）

1. `valuedomain.BrowserSortOrders` 加一个 `"pinned"`（不改 `sortOrders`）→
   `internal/browser` 的 `TestEveryOfferedSortOrderIsImplemented` 红（表里没有这个 id 的比较器），
   `internal/i18n` 的 `TestPickerOptionsMatchTheStoredDomains` 红（标签缺失）。
2. `settings.NormalizeBrowserSortOrder` 回退成「只认 `relevance`」→
   `internal/ci` 的 `TestTheLoaderAndThePickableDomainsAgree` 红。
3. `internal/browser` 的 `fieldColumns` 删掉 `FieldTitle` 一项（落回 all）→
   `TestEveryOfferedSearchFieldIsImplemented` 红。
4. `settings/browser.go` 里把 `[]string{"relevance", "recent", …}` 写回去 →
   `TestValueDomainsHaveOneAnswer` 红。

每条实跑 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 后复跑。
