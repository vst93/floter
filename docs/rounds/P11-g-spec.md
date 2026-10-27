# P11-g 规格：一道算式只有一个说法——`calc` 拥有它的规范形、它的上限和它的拒绝理由

## 意图

**一句话**：计算器这一族里，**同一道算式有三处各自定义「它是什么」**——
`calc.Eval` 自己剥一次 `=`、搜索页再剥一次（顺序还不一样）、计算器模式第三次；
**长度上限参考有、本仓没有**；**算式算不出来时 Enter 会去复制一条无关的历史行**。
本轮把「规范形 / 上限 / 拒绝理由」三件事全部收到 `internal/calc` 一个包里，
Enter 的归属也按参考的 `calculatorEnterAction` 钉死。

## 为什么现在做（证据）

选题优先级：用户实测四条全卡在实机复测、合约 `docs/raycast-design.md` §6 全「已落地」、
上一轮「仍待」第 1 条同样卡实机 → 按用户定的「三平台一致性 → 基础功能 → 像素细节」自查，
落在**基础功能**，且**三平台同错**（这三个包都没有 `GOOS` 分支）。

病灶（`git rev-parse HEAD` = `57bdbb0`，行号是本轮实测）：

### 1. 「用户输入的算式」有两个规范形

- `internal/calc/calc.go:29-31`（`Eval`）：`TrimSpace` → `TrimPrefix("=")` → `TrimSpace`。
- `internal/launcher/search.go:916`：`TrimSpace` → `TrimPrefix("=")` → `TrimSpace`（抄了一遍）。
- `internal/launcher/calculator.go:128`：`TrimPrefix("=")` → `TrimSpace`——**顺序不同**。
  于是输入 ` =1+1`（等号前有一个空格）在计算器模式里标题是 `=1+1`，在搜索页里是 `1+1`。
  更实质的后果在**历史身份**上：`calculator.Entry` 的折叠判据是 `Expression` 文本
  （`internal/calculator/calculator.go:280-283` 的 `existing.Expression == expression`），
  所以同一道算式会按输入时的空格/等号写成两条不同的历史行。

### 2. 长度上限：参考有 `CALCULATOR_MAX_EXPRESSION_LENGTH`，本仓没有

- 参考 `main:src/calculator.ts:110-112`：`CALCULATOR_MAX_EXPRESSION_LENGTH = 256`，
  `evaluateExpression` 在解析**之前**拒绝超长（`:191-193`，`calculator.error.tooLong`）。
- 本仓 `internal/calc/calc.go:28-46` 的 `Eval` 只做 `hasArithmetic` 与解析，
  **没有任何长度上限**：整段粘贴进搜索框/计算器模式的文本会一路进递归下降解析器
  （`parseExpr → parseTerm → parseUnary → parsePower → parsePostfix → parsePrimary`，
  括号还能自嵌套），这是用户定的第 3 优先级「稳定性」上的一个敞开的口子。

### 3. 算式算不出来时，Enter 落到历史行上

- `internal/launcher/launcher.go:2445-2453`（`activate` 的计算器模式分支）：
  Enter 只做「跑 `results[Selected].Run`」。
- `internal/launcher/calculator.go:126-140`（`calculatorItems`）：**只有算得出来**才追加
  `calc:<expression>` 这条待算式行；算不出来（`1+`、`1/0`、`sqrt(-1)`）时列表里**只有历史行**，
  而 `Selected` 仍是 0 → **Enter 复制的是列表第一条历史行**。
  历史为空时更糟：`results` 为空，Enter **什么都不做**（静默）。
- 参考的规则是字段文本自己拥有 Enter（`main:src/calculator.ts:429-435` 的
  `calculatorEnterAction`：`trimmed.length > 0 && trimmed !== lastEvaluated → "evaluate"`），
  失败时走 `showLauncherFeedback(evaluation.errorKey)` 并**留在模式里**
  （`main:src/App.tsx:1406-1419` 的 `evaluateCalculator`），不会去复制别的行。
- 这是本仓 P9-o 已经立过的那条规矩（「完成与失败不再静默」）在计算器上的欠账。

## 落地项

### 1. `calc.Expression` —— 规范形的唯一产生者

- 位置：`internal/calc/calc.go`（新增，放在 `Eval` 之前）。
- 现状：没有这个函数，三处各写一遍（见上）。
- 应改成：`func Expression(raw string) string`——`TrimSpace` → 去掉**一个**前导 `=` →
  `TrimSpace`。`Eval` 的第一句改成调它；两个 launcher 面也改调它。
  语义与现在的 `Eval` 完全一致（`Eval` 原本就是这个顺序），所以这是「把已有的答案提出来」，
  不是「换一个答案」。

### 2. `calc.MaxExpressionLength` + `calc.ErrTooLong`

- 位置：`internal/calc/calc.go`。
- 现状：无上限、无哨兵。
- 应改成：`const MaxExpressionLength = 256`（参考的 `CALCULATOR_MAX_EXPRESSION_LENGTH`），
  `var ErrTooLong = errors.New("calc: the expression is too long")`；
  `Eval` 在 `hasArithmetic` **之前**按**码点**计数（`utf8.RuneCountInString`，与 JS 的
  `String.length` 对 BMP 一致）拒绝，返回 `ErrTooLong`。
  `ErrNotAnExpression` 的判定保持在前（空串仍然是「不是算式」，不是「太长」）。

### 3. `calc.ErrNotFinite` —— 「没有有限值」也是一个哨兵

- 位置：`internal/calc/calc.go:44-46`（非有限结果）与 `:444`/`:454`（除零、模零）。
- 现状：三处各自 `errors.New("calc: …")`，调用方无法分辨「解析不了」与「没有有限值」。
- 应改成：三处都返回 `ErrNotFinite`（`1/0`、`1%0`、`sqrt(-1)` 在参考里都是同一个
  `calculator.error.notFinite`，`calculator.ts:199-201`）。其余解析错误保持原样（`CalculatorInvalid`）。

### 4. 搜索页与计算器模式都用同一个规范形

- `internal/launcher/search.go:911-916`：`value, err := calc.Eval(a.Query)` 保留（`Eval` 内部已经规范），
  但 `expression` 改成 `calc.Expression(a.Query)`——删掉就地那串 `TrimSpace`/`TrimPrefix`。
- `internal/launcher/calculator.go:128`：`expression := calc.Expression(a.calculatorQuery())`。

### 5. 复制的那一行也只有一个产生者

- 位置：`internal/launcher/calculator.go:237-243`（`calculationText`）。
- 现状：`return expression + " = " + result`，与 `internal/calculator/calculator.go:153`
  （`Entry.Label`）是同一句话的两份实现。
- 应改成：`return calculator.Entry{Expression: expression, Result: result}.Text(a.CopyMode)`
  ——待算式与历史行从此走同一个 `Text`/`Label`。

### 6. 计算器模式的 Enter 归字段所有

- 位置：`internal/launcher/launcher.go:2445-2453`。
- 现状：只跑选中行（见病灶 3）。
- 应改成：进入「跑选中行」之前先问一句——**字段里有文本、而这文本没有产出待算式行**
  （＝`calc.Eval` 拒绝了它）→ 走新增的 `runCalculatorExpression()`：
  失败时 `WarnFeedback` 说明原因（`ErrTooLong` / `ErrNotFinite` / 其它＝invalid），
  **留在模式里**；成功则与今天完全一样（记录 + 按复制模式复制 + 离开模式 + 隐藏窗口）。
  字段为空、或选中行是历史行（`item.calc != nil`）时行为**一字不动**：
  空字段仍然是「复制选中的历史行」（参考 `calculatorEnterAction` 对空串返回 `"run"`），
  用户按下键把选中移到历史行上仍然复制那一行（既有 pin
  `TestCalculatorModeEvaluatesRecordsAndCopies` 的后半段）。

### 7. 三条新文案（EN/ZH）

- `internal/i18n/i18n.go` 的 `Launcher` 结构体（`CalculatorEmpty` 旁）加三个字段，
  `internal/i18n/copy.go` 的 `en`/`zh` 各补一行，文案照参考 `i18n.ts:897/898/899`：
  `CalculatorInvalid`「Invalid expression / 表达式无效」、
  `CalculatorNotFinite`「Result is not a finite number (division by zero?) /
  结果不是有限数值（是否除以零？）」、`CalculatorTooLong`「Expression is too long / 表达式过长」。

### 8. pin

`internal/calc/calc_test.go`（4 条）：
- `TestTheExpressionHasOneNormalForm`：`Expression` 对 `"1+1"` / `" 1+1 "` / `"=1+1"` / `" = 1+1 "` 都是 `1+1`；`"=="`、`"  "` 是 `""`。
- `TestALongExpressionIsRefusedBeforeItIsParsed`：256 码点通过、257 拒绝为 `ErrTooLong`；
  257 个 `(` 也必须是 `ErrTooLong`（不是栈上跑到底）。改前红（现在会去解析）。
- `TestDivisionByZeroIsTheNotFiniteRefusal`：`1/0`、`1%0`、`sqrt(-1)` 都是 `errors.Is(..., ErrNotFinite)`；
  `1+` 仍然**不是** `ErrNotFinite`（是解析错误）。改前红。
- `TestTheSearchPageAndTheModeAgreeOnTheExpression`：`Eval(" =1+1")` 与 `Expression(" =1+1")` 一致
  （`Eval` 内部用的就是它），钉住「一处定义」。

`internal/launcher/calculatorenter_test.go`（3 条，新文件）：
- `TestABrokenSumSaysWhyInsteadOfCopyingAHistoryRow`：计算器模式里历史有两条、输入 `1+`、
  Enter → **没有复制任何东西**、模式仍在、`toast` 是 `CalculatorInvalid`。改前红（复制了历史行）。
- `TestADivisionByZeroSaysItHasNoFiniteValue`：输入 `1/0` + Enter → `CalculatorNotFinite`，
  没有复制、没有记录（`fixture.added` 空）。
- `TestALongSumIsRefusedAndNotRecorded`：输入 300 个 `1+`…（构造 300 码点的算式）→ `CalculatorTooLong`。
- `TestTheModeStillCopiesAChosenHistoryRow`：字段有文本（`10*3`，既是待算式也是历史行）时
  用 ↓ 把选中挪到**历史行**上 → Enter 复制的仍是那一行的 `Text(copyMode)`（既有契约不破）。

## 行为契约零改动红线

- **高度链**：不新增行、不改 `height.go`；失败只走既有的反馈行（`a.toast`，与
  `CalculatorFavoriteFailed` 同一条通路）。
- **键盘模型**：`activate` 的进入条件不变，只多一个「字段文本没产出待算式行」的判据；
  空字段、选中历史行、↓/↑ 移动后的 Enter 语义一字不动。
- **会话管理 / 设置写回 / 命令接线**：零改动。
- **i18n 既有文案**：零改动（只**新增**三条）。
- `calculator.Store` 的写路径（P11-d 的 `mutateLocked`）、`CalculatorFetchLimit`、
  `calculatorItems` 的排序与 chip 过滤：零改动。
- `calc` 的求值语义（优先级、`!`、`^` 右结合、函数表、`Format` 的打印策略）：零改动。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2235（只许涨）
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 基线 15
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

测试函数基线 **1806**（只许涨）。

## mutation 计划（≥2 条，实跑）

1. 删 `Eval` 里的长度检查 → `TestALongExpressionIsRefusedBeforeItIsParsed` 变红。
2. `Eval` 改回 `strings.TrimSpace(strings.TrimPrefix(expr, "="))`（不回第二遍 `TrimSpace`）
   → `TestTheExpressionHasOneNormalForm` 变红（`" =1+1"` 会留下 `1+1` 之外的空隙…实测看哪条红）。
3. 删 `activate` 里的新判据 → `TestABrokenSumSaysWhyInsteadOfCopyingAHistoryRow` 变红。
4. 把 `ErrNotFinite` 换回 `errors.New("calc: division by zero")` → `TestDivisionByZeroIsTheNotFiniteRefusal` 变红。
