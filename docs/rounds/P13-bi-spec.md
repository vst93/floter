# P13-bi 规格：插件窗口里「同一个运行的结局」没有画出来

## 意图

**插件窗口（detached / pinned output window）里，一次有输出的失败运行与一次成功的运行，
在屏幕上逐像素相同——结局没有被画出来。** 本轮把捕获输出页（§6.26，P13-bg）刚立起来的
那条三档阶梯接到同一个运行的第二个表面上：窗口正文上方补一条**结局带**（结局 / 中点 / 事实）。

为什么现在做：P13-bh「仍待」第 3 条第 ② 项就是这条（跨表面统一），而且它是**状态反馈**这一格里
最硬的一种——不是「画得不够重」，是**没画**。

## 证据（HEAD `2930aca6`，实测行号）

- `internal/shell/detach.go:338-341`：正文的四条状态行只在 `running` / `failed != ""` /
  空输出 / 未运行这四个臂里出现；**有输出的那两个臂（文本 / 列表）一条状态也没有**。
- `internal/shell/detach.go:664-672`（`applyDetachedRun`）：`detached.failed` 只在
  `runErr != nil && view.Text == ""` 时才写——**有输出的失败运行连一句话都不留**。
- `internal/launcher/output.go:453-470`（`runStatus`）：结局与事实（时长 / 截断）被拼成
  一个字符串 `"Exit code 3  ·  0.4s"`，两个寄存器在**字符串这一层**就已经糊在一起，
  所以窗口哪怕想画也只能整句一个墨。
- `internal/launcher/output.go:144-150`（`OutputFor`）：`Status` 已经是整句，
  没有分开的字段给别的表面用。

实测（`ui.Tester` 720×460，`a.detachedView`，文本臂，窗口地面 `tokens.Opaque` 浅 246 / 深 20）：

| 臂 | 窗口自己的文字里有没有这个结局 | 结局墨 |
|---|---|---|
| 成功（`Finished  ·  0.4s`） | **没有**（`tt.HasText("Finished")` 为假） | — |
| 失败（`Exit code 3  ·  0.4s`） | **没有** | — |
| 超时（`Timed out  ·  0.4s`） | **没有** | — |

三个结局在窗口上**一个字节都不差**（差不是「小」，是 0：它们都不存在）。

## 逐条落地项

### 1. `launcher.OutputView` 拆出两个字段（`internal/launcher/output.go`）

- 位置：`OutputView` 结构体（`:27-70`）+ `OutputFor`（`:144-150`）+ `runStatus`（`:453-470`）。
- 现状：`Status` 是 `结局 + "  ·  " + 事实` 的一整句，别处无法分开取。
- 应改成：新增 `Verdict`（结局那句）与 `Facts`（时长 / 截断，可能为空）；`runStatus`
  改为 `runStatusParts` 返回两段，`Status` 仍由同一段拼接逻辑产出（**逐字节不变**，
  §6.26 的 pin 与像素不动）。

### 2. 窗口正文上方补一条「结局带」（`internal/shell/detach.go`）

- 位置：`detachedView`（`:205-217`）的 `Column` 三个孩子之间。
- 现状：bar → body，中间什么都没有。
- 应改成：bar → **`detachedVerdict`** → body。`detachedVerdict` 只在**有内容要展示**时画
  （`ran && !running && failed == "" && view.Verdict != ""`），否则整行不存在——
  running / failed / 空输出 / 未运行四个臂的正文语句仍是那一段的**唯一**结论（§6.24 不动）。
- 三档阶梯（照抄 §6.26 的前三档，第三档这里是**事实**而不是键位图例）：

  | 段 | 墨 | 字重 | 字号 |
  |---|---|---|---|
  | 结局 | `tokens.TextSecondary`（失败 → `t.Warning`，色相是语义） | 580 | `tokens.Caption` |
  | 中点 `·` | `tokens.TextTertiary.Alpha(0.7)` | — | `tokens.Caption` |
  | 事实（时长 / 截断） | `tokens.TextTertiary` | 500 | `tokens.Caption` |

- 内边距：左右各 `detachedBodyInset`（14u，与正文字、列表板同一条内边），上 14u、下 0——
  正文自己的 14u 上内边距就是它与结局带之间的呼吸，**不重复给**。

### 3. 合约与账本

- `docs/liquid-glass.md` 新增 **§6.28**（逐行状态表）。
- `.pending-bugs.md` 顶部新增本轮小节与「仍待」。

## 行为契约零改动红线

- **高度链**：窗口是固定尺寸的独立窗口（`mygo.NewWindow`），正文是 `Scroll`；
  新增一行在流里、`Shrink(0)`，`MaxY` 由布局自己算——面板高度链、主窗口、设置页一个字节不动。
- **键盘模型**：无新键、无新焦点、无新 `Focusable`。
- **会话管理**：不碰 `internal/termsession`。
- **设置写回 / i18n 文案 / 命令接线**：零改动（`Verdict` / `Facts` 复用 `runStatus` 已有的
  `copy.OutputSucceeded` / `OutputFailed` / `OutputTimedOut` / `OutputTruncated` 与
  `extensions.FormatRunDuration`，没有新串）。
- **不碰**：pinned 窗口（便条没有运行）、bar（§6.7 不动）、正文四条状态行（§6.24 不动）。

## 门槛命令 + PASS 基线（实测，HEAD `2930aca6`）

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3160
go test ./... 2>/dev/null | grep -c "^func Test"    # 基线 2327
GOOS=linux|windows|darwin go build ./...    # 过
```

## mutation 计划（≥2 条，每条说清会让哪个 pin 变红）

1. `detachedVerdict` 的失败臂去掉 `ink = t.Warning` → `TestTheFailedRunKeepsTheWarningHue`
   的暖度断言（`r−b ≥ 25`）变红。
2. `detachedView` 里删掉 `a.detachedVerdict(...)` 调用 → `TestTheDetachedWindowSaysTheRunsVerdict`
   的 `tt.Find(view.Verdict)` 变红（这正是改前的状态）。
3. 事实那段抬到 `TextSecondary` → `TestTheVerdictOutranksItsFacts` 的差级断言变红。
4. `detachedVerdict` 的门去掉 `detached.failed != ""` → `TestTheEmptyRunHasNoVerdictBand`
   变红（空输出臂会多出一条结局带）。
