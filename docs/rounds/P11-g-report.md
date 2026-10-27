# P11-g 报告：一道算式只有一个说法——规范形、上限、拒绝理由都收到 `calc` 里

规格 `docs/rounds/P11-g-spec.md`。方向：用户实测四条全卡实机复测、合约 §6 全「已落地」、
上一轮「仍待」第 1 条同样卡实机 → 按用户定的「三平台一致性 → 基础功能 → 像素细节」自查，
落在**基础功能**（计算器这一族），三平台同错（`calc` / `calculator` / `launcher` 三个包都没有
`GOOS` 分支，一处修三处同好）。

## 1. 病灶（`git rev-parse HEAD` = `57bdbb0`，行号本轮实测）

### 1.1 「用户输入的算式」有两个规范形

| 位置 | 剥 `=` 与空白的顺序 |
|---|---|
| `internal/calc/calc.go:29-31`（`Eval`） | `TrimSpace` → `TrimPrefix("=")` → `TrimSpace` |
| `internal/launcher/search.go:916` | `TrimSpace` → `TrimPrefix("=")` → `TrimSpace`（抄了一遍） |
| `internal/launcher/calculator.go:128` | `TrimPrefix("=")` → `TrimSpace`（**顺序不同**） |

于是 ` =1+1`（等号前一个空格）在计算器模式里标题是 `=1+1`、在搜索页里是 `1+1`。
更实质的是**历史身份**：`internal/calculator/calculator.go:280-283` 用
`existing.Expression == expression` 折叠同一道算式，所以同一道算式会按输入时的空格/等号
落成两条历史行。

### 1.2 长度上限：参考有 `CALCULATOR_MAX_EXPRESSION_LENGTH`，本仓没有

参考 `main:src/calculator.ts:110-112` 是 256，`evaluateExpression`（`:191-193`）在解析**之前**
拒绝（`calculator.error.tooLong`）。本仓 `internal/calc/calc.go` 的 `Eval` 只做词表闸门与解析，
**没有上限**：整段粘贴一路进递归下降（`parseExpr → parseTerm → parseUnary → parsePower →
parsePostfix → parsePrimary`，括号还自嵌套）。这是用户第 3 优先级「稳定性」上的一个敞口。

### 1.3 算式算不出来时，Enter 去复制一条无关的历史行

`internal/launcher/launcher.go:2445-2453` 的 `activate` 计算器分支只做「跑 `results[Selected].Run`」；
而 `internal/launcher/calculator.go:126-140` 的 `calculatorItems` **只有算得出来**才追加
`calc:<expression>` 待算式行。所以 `1+`、`1/0`、`sqrt(-1)` 时列表里只剩历史行、`Selected` 仍是 0
→ **Enter 复制的是列表第一条历史行**；历史为空时更糟：`results` 空，Enter **什么都不做**。

参考的规则是字段文本自己拥有 Enter（`main:src/calculator.ts:429-435` 的 `calculatorEnterAction`：
`trimmed.length > 0 → "evaluate"`），失败时 `showLauncherFeedback(errorKey)` 并留在模式里
（`main:src/App.tsx:1406-1419`）。这是本仓 P9-o 立的规矩（「完成与失败不再静默」）在计算器上的欠账。

## 2. 落地

- `internal/calc/calc.go`
  - 新增 `Expression(raw string) string`：`TrimSpace` → 去掉**一个**前导 `=` → `TrimSpace`。
    `Eval` 的第一句改成调它——语义与原来的 `Eval` 一字不差（原本就是这个顺序），
    所以这是「把已有的答案提出来」，不是「换一个答案」。
  - 新增 `MaxExpressionLength = 256`（参考的常量）与 `ErrTooLong`；`Eval` 在词表闸门**之前**
    按**码点**（`utf8.RuneCountInString`，与 JS `String.length` 对 BMP 一致）拒绝。
    顺序照参考：`empty → tooLong → parse`，所以 `tooLong` 压过 `invalid`。
  - 新增 `ErrNotFinite`；`1/0`、`1%0` 与非有限结果（`sqrt(-1)`、`log(0)`）都返回它。
    参考里这三者是同一句话（JS 的 `1/0` 是 `Infinity`，`calculator.ts:199-201`）。
- `internal/launcher/search.go:911-916`：`expression := calc.Expression(a.Query)`，删掉就地的
  `TrimSpace`/`TrimPrefix`；`calc.Eval(expression)`。
- `internal/launcher/calculator.go`
  - `:128` 改 `calc.Expression(a.calculatorQuery())`；
  - `calculationText` 改成 `calculator.Entry{Expression, Result}.Text(a.CopyMode)`——
    `expression = result` 这句话从此只有一个产生者（`Entry.Label`）；
  - 新增 `runCalculatorExpression`（失败 `WarnFeedback` 并留在模式里）、`calculatorRefusal`
    （`ErrTooLong` → `CalculatorTooLong`，`ErrNotFinite` → `CalculatorNotFinite`，其余 → `CalculatorInvalid`）、
    `pendingCalculation`（待算式行的身份：`calc:` 行且 `calc == nil`）、
    `calculatorEnterOwnsTheField`（字段有文本且列表里没有它的答案行）。
- `internal/launcher/launcher.go:2445`：计算器分支先问 `calculatorEnterOwnsTheField`，
  真则 `runCalculatorExpression()`；空字段与「选中历史行」的语义一字不动。
- `internal/i18n/{i18n.go,copy.go}`：新增三条文案（EN/ZH），照参考 `i18n.ts:897-899`：
  `Invalid expression` / `Result is not a finite number (division by zero?)` /
  `Expression is too long`。

台账 **V=0 / B=8 / S=0**（既有测试**零改动**全绿；新增 8 条 B pin）。

## 3. pin（8 条，全是新文件/新函数）

`internal/calc/calc_test.go`
- `TestTheExpressionHasOneNormalForm`：11 种拼写 → 同一个规范形；`Eval` 与 `Expression` 读同一份。
- `TestALongExpressionIsRefusedBeforeItIsParsed`：256 码点通过（值 138）、257 拒绝；
  257 个字母、257 个 `(` 都是 `ErrTooLong`（不是走完词表/递归）；1000 个尾随空格**不算**长度
  （`Expression` 先剥了）；303 字节 / 103 码点的多字节串**不是** `ErrTooLong`（数的是字符不是字节）。
- `TestDivisionByZeroIsTheNotFiniteRefusal`：`1/0`、`1%0`、`sqrt(-1)`、`log(0)`、`tan(0)/0`、`0/0`
  都是 `ErrNotFinite`；`1+`、`(1`、`1 2 3 +` 仍是普通解析错误（不是这三个哨兵）。

`internal/launcher/calculatorenter_test.go`（新文件）
- `TestABrokenSumSaysWhyInsteadOfCopyingAHistoryRow`：`1+` + Enter → 零复制、零记录、
  模式仍开、反馈行 `Invalid expression` 且是 warning 色。
- `TestADivisionByZeroSaysItHasNoFiniteValue`：`1/0`/`1%0`/`sqrt(-1)` → `CalculatorNotFinite`。
- `TestALongSumIsRefusedAndNotRecorded`：301 码点 → `Expression is too long`，不记录。
- `TestTheModeStillCopiesTheRowTheSelectionIsOn`：字段是有效算式时把选中挪到历史行 → 仍复制**那一行**；
  空字段 → 复制选中行（参考 `calculatorEnterAction` 对空串返回 `"run"`）。
- `TestAFreshCalculationCopiesTheEntrysOwnLine`：` =1+1 ` 记录成 `{1+1, 2}`（规范形），
  复制的正是 `Entry.Text(copyMode)`。

## 4. 门槛（实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿（34 ok + 2 no test files）
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2243（基线 2235，+8）
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 15（=）
go test -count=1 -v ./... | grep -c -- "--- FAIL"      # 0
grep -rhc "^func Test" --include=*_test.go internal cmd # 1814（基线 1806，+8）
GOOS=linux / windows / darwin go build ./...           # 三平台过
```

非视觉轮，不出图（无像素改动）。

## 5. mutation（4 条实跑，全部变红、逐字节还原、`touch` 复跑）

| # | 注入 | 变红的 pin（原始报错串） |
|---|---|---|
| M1 | 删掉 `Eval` 的长度检查 | `calc_test.go:300: Eval(257 code points) = <nil>, want ErrTooLong`；`calc_test.go:305: Eval(257 letters) = calc: not an expression, want ErrTooLong`；`calc_test.go:310: Eval(257 open parentheses) = calc: not an expression, want ErrTooLong`；`calculatorenter_test.go:90: an over-long sum copied "1+1+1+…"`、`:93: an over-long sum was recorded: [[1+1+… 151]]`、`:96: feedback = ""`、`:99: the mode closed on a refused expression` |
| M2 | `Expression` 不回第二遍 `TrimSpace` | `calc_test.go:267: Expression(" = 1+1 ") = " 1+1", want "1+1"` |
| M3 | 删掉 `activate` 里的新判据 | `calculatorenter_test.go:44: a broken sum copied "10*3 = 30" — Enter fell through to a history row`；`:50/:53/:56`；`TestADivisionByZero…: "1/0" copied "10*3 = 30"`（三条算式同形） |
| M4 | `1/0` 换回普通 `errors.New("calc: division by zero")` | `calc_test.go:338: Eval("1/0") = calc: division by zero, want ErrNotFinite`；`calculatorenter_test.go:71: "1/0" feedback = "Invalid expression"` |

每条注入后 `md5sum -c` 逐字节核对（`internal/calc/calc.go` = `8174b0a4…`，
`internal/launcher/launcher.go` = `7503e4dc…`）并 `touch` 复跑，`go test ./internal/calc ./internal/launcher` 回到 `ok`。

## 6. 需用户拍板（本轮新增一条，**未实现**）

| 项 | 本仓现状 | 另一条路 | 影响 |
|---|---|---|---|
| 搜索页（非计算器模式）的答案行，Enter 之后算什么 | **只复制裸结果**（`search.go:911-936`：`Actions.Copy(result)`），**不记历史**、不看 `copy_mode`、弹「已复制」 | 与计算器模式一致：按 `copy_mode` 复制 + 记入历史 | 参考**没有**这一行（`evaluateCalculator` 只挂在 `calculator` scope 上，`App.tsx:1420-1422`），所以两条路都没有参考答案。改成一致＝搜索页每次回车都写盘并改变默认复制文本（`2` → `1 + 1 = 2`）；既有 pin `TestCalculatorRowComesFirstAndCopies`（`search_test.go:217`） 钉的是裸结果。本轮**只报告不改**（改了就是改既有交互） |

## 7. 仍待

1. 用户实机复测四条，连续第 62 轮。
2. 需拍板不变（`.pending-bugs.md` P9-g 表 + 剪贴板轮询间隔 600 vs 900 + 复制回来是否置顶 +
   计算器 `loadLocked` + `usage.Record` 写失败是否回滚 + 本轮新增「搜索页答案行」）。
3. 计算器这一族本轮收口（规范形 / 上限 / 拒绝理由 / Enter 归属），**别在 `calc` 与
   `internal/launcher/calculator.go` 里再找小修**；剩下的都是拍板项。
4. 下一处候选：`internal/inputtext`（68 行 / 7 条 pin）与参考 `src/terminal/inputNormalize.ts`
   已逐条对照**无分歧**（两个函数一一对应，含 U+180E 不在 Zs 的注脚），所以那里**不要**重领；
   `internal/launcher` 的 `filters.go`/`keycap.go` 三个 `runtime.GOOS` 分支与 P10-y 的
   「非 macOS 词序」拍板项是同一件事，要动先拍板。
