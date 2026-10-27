# P11-m 报告 · 计算器的写入口也走规范形

## 1. 本轮做了什么

`internal/calc` 与 `internal/calculator` 的边界只留一个答案：P11-g 把「一道算式只有
一个说法」收口在**调用方**（`calc.Expression`），但存储的**写入口** `Store.Add` 还自带
一个更弱的归一化（`strings.TrimSpace`，不去前导 `=`）。本轮让写入口也走 `calc.Expression`，
于是 `=1+1` 与 `1+1` 在持久集合里是同一行，而不是两行。

## 2. 逐条落地

- `internal/calculator/calculator.go` `Add`：`expression = strings.TrimSpace(expression)`
  → `expression = calc.Expression(expression)`，注释写明「写入口也走 `calc` 的规范形」。
- `internal/calculator/calculator.go` import：加 `"floter/internal/calc"`（`calc` 不 import
  `calculator`，无环）。
- 新增 pin `TestAddFoldsTheCanonicalSpellings`（`calculator_test.go`）：`Add("=1+1")` 与
  `Add("1+1")` 与 `Add("  = 1+1  ")` 折成一行且存的是 `1+1`。

## 3. V / B / S 台账

- **V**（视觉数值）：0（非视觉轮）
- **B**（行为/度量/接线）：既有 `calculator` 12 条测试零改动全绿（它们都传规范串，
  `calc.Expression` 对它们是恒等变换）。
- **S**（结构真变了）：0

## 4. 门槛（实测）

```
gofmt -l cmd internal     → 空
go vet ./...              → 净
go test -count=1 ./...    → 全绿（exit 0）
PASS 计数                 → 2265（基线 2264，+1）
SKIP 计数                 → 15（不变）
三平台 build              → linux/windows/darwin 全过
```

## 5. mutation（实跑变红后还原）

1. `Add` 里 `calc.Expression(expression)` 改回 `strings.TrimSpace(expression)`
   → **build 失败**：`"floter/internal/calc" imported and not used`（证明该 import 只有
   这一处用途；也是红的，但不是断言红）。
2. `calc.Expression` 删掉 `text = strings.TrimPrefix(text, "=")`
   → `TestAddFoldsTheCanonicalSpellings` 红：
   `the leading = was kept: "=1+1"`。
3. `Add` 折叠判据 `existing.Expression == expression` 改成 `!=`
   → `TestAddFoldsAndKeepsFavorites` 红：
   `entries = [{... Expression:2+2 ...}]`（重跑堆了/吞了行）。

三条均以 `md5sum -c` 逐字节还原（`calculator.go` 1869c0fc…、`calc.go` 8174b0a4…），
还原后 `go test ./internal/calculator/` 绿。

## 6. 为什么现在做（选题依据）

- 用户实测四条仍卡实机（需实机），合约 §6 全「已落地」。
- 上一轮「仍待」第 5 条正是「`internal/calc` 与 `internal/calculator` 边界是否只有一个
  答案」——P11-g 收口了调用方，写入口是这条契约的最后一块未收口的地方。
- 按用户优先级（三平台一致性 → 基础功能 → 像素细节），这是**基础功能/一致性**，
  无 GOOS 分支，三平台同一答案。

## 7. 仍待

1. 用户实机复测四条，连续第 68 轮。
2. 需拍板不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器
   `loadLocked`（每次写是否从磁盘重读）+ `usage.Record` 写失败是否回滚 + 「搜索页答案行」
   + 非 macOS 快捷键词序 + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布
   `dist/SHA256SUMS`）。
3. `calc`/`calculator` 边界：**规范形**两侧已同一答案（P11-g + 本轮）；剩下唯一未决的是
   「每次写要不要从磁盘重读（`loadLocked`）」，那是**拍板项**，不擅自动。
