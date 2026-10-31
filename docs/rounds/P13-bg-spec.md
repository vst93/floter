# P13-bg 规格 · 捕获输出页的状态行：三种结局长得一模一样

轮号 **P13-bg**｜表面：**启动器 · 捕获输出页的条子（strip）**｜合约新增 **§6.26**

## 意图（一句话）

捕获输出页标题下那一行 `view.Status + "  ·  " + outputHint(...)` 把**这一页的结局**
（`Finished` / `Exit code 3` / `Timed out`）与**它自己的按键提示**
（`Enter copies · Esc closes`）用**同一档墨、同一个字重**画出来，只隔一个中点；
更要紧的是**三种结局逐像素相同**——一次失败的运行和一次成功的运行在条子上长得一样。

这轮把它拆成三档：结局（`TextSecondary` + 580，**失败/超时换 `t.Warning`**）、中点
（`TextTertiary` 0.7）、提示（`TextMuted` + 500，不动）。

## 为什么现在做（证据）

- 队列：`docs/liquid-glass.md` §6 全表 `已落地` / `故意不做`，没有 `未开始` 行，所以按
  任务书回**通用队列**——这一条同时落在「状态反馈」与「层级」两格里（第 1 条验收轴）。
- 实测（720 帧、真启动器地面；浅 253 / 深 23，`internal/launcher/output.go:546`）：

| 臂 | 墨 | 浅 darkest / 对地面 | 深 darkest / 对地面 |
|---|---|---|---|
| `Finished` | `TextMuted` | **116 / 137** | **164 / 141** |
| `Exit code 3` | `TextMuted` | **116 / 137** | **164 / 141** |
| `Timed out` | `TextMuted` | **116 / 137** | **164 / 141** |
| 标题（命令行） | `t.Text` | 25 / 228 | 244 / 221 |

  三个臂的 ink **一个字节都不差**：`OutputView.Status` 只是一串字，条子上没有任何东西
  知道这次运行是成功还是失败（`internal/launcher/output.go:144` 造 view 时没有这个信息）。
  这是「状态反馈」这一格最硬的一条：**状态没有画出来**。
- 第二个事实：那一行里两件事共用一个寄存器。`Finished`（信息）与
  `Enter copies · Esc closes`（键位图例）在 11px 的同一档灰里，只隔一个 `·`。
  层级只由差说话，两件事同一档就是「糊」。

## 逐条落地项（行号本轮实测）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/output.go:32-45` `OutputView` | 只有 `Status string`，没有「这次跑挂了没有」 | 加 `Failed bool`（非零退出 / 超时 / 根本没起来） |
| 2 | `internal/launcher/output.go:140` `OutputFor` | 只填 `Status` | 同时 `view.Failed = run.TimedOut || !run.Success` |
| 3 | `internal/launcher/output.go:546` 状态行 | `ui.Text(Status + "  ·  " + hint).FontSize(FontSize-1).TextColor(TextMuted)` | `ui.RichText(c, a.outputStatusSpans(...)...).FontSize(FontSize-1)`：三段各自的墨与字重 |
| 4 | 新增 `outputStatusSpans` | — | 结局 `TextSecondary`/580（`Failed` 时 `t.Warning`/580）、中点 `TextTertiary.Alpha(0.7)`/400、提示 `TextMuted`/500 |

**不动的**：标题、两枚 28u 方键、条子的 padding、`FontSize-1` 的字阶、行的行盒
（三段同一个字号，行盒一个像素不动）、`outputHint` 的文案与两种说法、i18n 一个字节不改。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线——**全部不动**。
这轮只改这一行的墨与字重；`OutputView` 加一个只读布尔（由 `OutputFor` 自己算，没有任何
调用方需要改）。视觉轮只改墨与面。

## 门槛命令 + PASS 基线（本轮开工实测）

```
gofmt -l cmd internal
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux/windows/darwin go build ./...
```

基线：**3154 PASS / 0 FAIL / 107 SKIP / 2319 测试函数**（P13-bf 收尾实测）。

## mutation 计划（≥2，每条说明让哪个 pin 变红）

- M1：`Failed` 恒 `false`（去掉 `run.TimedOut || !run.Success`）→
  `TestAFailedRunIsNotTheSameLineAsASuccessfulOne` 红（失败臂与成功臂的墨又一样了）。
- M2：结局退回 `TextMuted` → `TestTheOutputStatusLineSeparatesItsVerdictFromItsLegend` 红
  （左右两半的差 < 15）。
- M3：`Failed` 臂不给 `t.Warning`（只抬到 `TextStrong`）→ 第二条 pin 的色相断言红（`r−b`）。
- M4：提示跟着抬到 `TextSecondary` → 第一条 pin 红（两半的差塌掉）。
