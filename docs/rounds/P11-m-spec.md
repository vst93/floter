# P11-m 规格 · 计算器的写入口也走规范形（`calc` / `calculator` 边界只有一个答案）

**方向**：用户实测四条仍卡实机、合约 §6 全「已落地」、上一轮「仍待」第 5 条
（`internal/calc` 与 `internal/calculator` 的边界是否只有一个答案）→ 按用户定的
「三平台一致性 → 基础功能 → 像素细节」自查，落在**基础功能 / 一致性**（无 GOOS 分支）。

---

## 1. 意图（一句话）

P11-g 把「一道算式只有一个说法」收口在 **调用方**（`calc.Expression`），但
`calculator.Store.Add` 是表达式的**第二个写入口**，它自带一个更弱的归一化
（`strings.TrimSpace`），于是「`=1+1`」与「`1+1`」在存储层是两个不同算式——同一道算式
在持久集合里有两个说法。本轮让写入口也走唯一的规范形。

## 2. 证据（动手前实测）

- `internal/calculator/calculator.go:238`（`Add` 首行）：
  ```go
  expression = strings.TrimSpace(expression)
  ```
  只 trim，**不**去前导 `=`。而 `calc.Expression`（`internal/calc/calc.go:52`）是
  `TrimSpace` → `TrimPrefix("=")` → `TrimSpace`。
- 折叠判据 `calculator.go:249,256` 是 `existing.Expression == expression`（逐字节相等），
  所以 `Add("=1+1","2")` 与 `Add("1+1","2")` 现在存成**两行**，不是一行。
- 唯一生产调用点 `internal/launcher/calculator.go:129,213` 先算
  `expression := calc.Expression(...)` 再 `Add`，故**用户界面今天看不到差别**——
  这是一个潜伏在边界上的第二套归一化，不是当前可见缺陷。

## 3. 逐条落地

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/calculator/calculator.go` `Add`（约 237–239） | `expression = strings.TrimSpace(expression)` | `expression = calc.Expression(expression)`，并把注释写明「写入口也走 `calc` 的规范形，与读侧、调用方同一答案」 |
| `internal/calculator/calculator.go` import | 无 `floter/internal/calc` | 加 `"floter/internal/calc"`（`calc` 不 import `calculator`，无环——已 grep 确认） |

**行为契约零改动红线**：折叠语义、收藏前移、prune、ID 生成、`\n` 落盘、0o600 模式
一个字不动；只把「如何把用户给的字符串变成规范表达式」从 `TrimSpace` 换成 `calc.Expression`。

## 4. 台账预测

- **V**（视觉数值）：0（非视觉轮）
- **B**（行为/度量/接线）：既有 12 条 `calculator` 测试零改动必须全绿（它们都传规范串，
  `calc.Expression` 对它们恒等）。
- **S**（结构真变了）：0

## 5. 门槛命令 + PASS 基线

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

**PASS 基线**：2264（P11-l 实测）。本轮新增 1 条 pin，期望 ≥ 2265。

## 6. mutation 计划（≥2 条，实跑变红后还原）

1. 把 `Add` 里的 `calc.Expression(expression)` 改回 `strings.TrimSpace(expression)`
   → 新 pin `TestAddFoldsTheCanonicalSpellings` 变红（`=1+1` 与 `1+1` 不再折成一行）。
2. 把 `calc.Expression` 内部删掉 `TrimPrefix("=")` → 同一条 pin 与 `calc` 既有 pin 一起变红。
3. 把折叠判据 `existing.Expression == expression` 改成 `strings.Contains` → 既有
   `TestAddFoldsAPreviousEntryIntoTheTopRow` 变红（证明 pin 不是靠新测试单点撑着）。

## 7. 风险与取舍

- 低：唯一生产调用点已传规范串，`calc.Expression` 对它是恒等变换，用户界面零变化。
- 新增依赖 `calculator → calc`（两个都是叶子包，无环）。
- 非视觉轮不出图。

## 8. 成功标准

- [ ] `Add` 走 `calc.Expression`，注释写明是同一答案
- [ ] 新增 pin：`=1+1` 与 `1+1` 折成一行
- [ ] gofmt/vet/test/三平台 build 全过，PASS ≥ 2265
- [ ] mutation ≥2 条实跑变红 + `md5sum -c` 还原
- [ ] 报告 + done.log + commit + push
