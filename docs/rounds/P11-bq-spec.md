# P11-bq 规格 · 到点的句子要一次「到点唤醒」，不是每帧重建整棵视图

## 意图

一条**到点才会变**的线（toast 自己消失、确认窗口过期、防抖到点写盘、写失败便条 5s 后收起）
现在靠 `c.AnimationFrame()` 每帧要一次帧 —— 而 `Context.AnimationFrame` 是「下一帧**重建整棵
视图**」（mygo v0.7.0 `ui/runtime.go:744-748`），不是重画。于是一条 4.5 秒不动的句子会让整页
按显示器刷新率重建 500 多次（120 Hz）。同一件事在本仓已经有正确答案：设置页的「已复制」确认
用的是 `c.After(until.Sub(now))`（`internal/settingsui/settingsui.go:3917`、`:5086`），
P11-bp 也把分页环从「每帧重建」改成「绘制期转」。

选它的依据：上一轮（P11-bp）报告「仍待」第 3 条 —— 「剩下的 `c.AnimationFrame()` 调用点
**未逐条核对**，下一轮先量『谁在一直要帧』再动手」。本轮先量，再分类，再改。

## 先量：九个调用点分两类

`grep -rn "c\.AnimationFrame()" internal/ --include=*.go | grep -v _test` 的九个点：

| 位置 | 是什么 | 期间有运动吗 | 处置 |
|---|---|---|---|
| `internal/shell/shell.go:4305` | 面板入场（`entrance < 1`） | **有**（ease 0→1） | 不动 |
| `internal/shell/notices.go:396` | toast 入场（`progress < 1`） | **有**（spring 0→1） | 不动 |
| `internal/launcher/fieldkeys.go:281` | 剪贴板删除的确认窗口 | 没有 | → `deadline.Wake` |
| `internal/launcher/power.go:103` | 关机/重启的问句窗口 | 没有 | → `deadline.Wake` |
| `internal/launcher/launcher.go:3053` | 底部 toast 的 4.5s 窗口 | 没有 | → `deadline.Wake` |
| `internal/settingsui/debounce.go:105` | 设置写盘的防抖窗口 | 没有 | → `deadline.Wake` |
| `internal/settingsui/savealert.go:55` | 写失败便条的 5s 窗口 | 入场由 `AnimateTo` 自己驱动帧 | → `deadline.Wake` |
| `internal/settingsui/settingsui.go:6067` | 三个内联确认（禁用/移除/丢弃改动的）窗口 | 没有 | → `deadline.Wake`（取最早那个到点） |
| `internal/settingsui/settingsui.go:6124` | 会话关闭的问句窗口 | 没有 | → `deadline.Wake` |
| `internal/terminalui/terminalui.go:1265` | 终端失败提示的 4.5s 窗口 | 没有（P11-bg 已删入场） | → `deadline.Wake` |

也就是说：**九个点里只有两个真的在动**（shell 的两处入场），其余七个是纯时钟。
它们不是 bug，是一条**用错了工具**的通行写法：到点的线要的是「到点唤醒一次」，不是「每帧重建」。

## 落地

### 1. 一个答案：`internal/deadline`

新包 `internal/deadline`，只有 `Wake(c *ui.Context, wait time.Duration)`：

- `c == nil` 或 `wait <= 0` → 什么都不做（调用者本来就有「帧外调用」「已经过了」两种情形）；
- 否则 `c.After(wait + wakeSettle)`（`wakeSettle = 1ms`）。

`+1ms` 不是随手加的容差：**七处时钟的判据都是严格大于**（`now.Sub(at) > window`、
`now.After(until)`），而在**恰好等于** deadline 的时刻唤醒，判据仍为假 → 会再排一个
`wait == 0` → `Wake` 什么都不做 → 这条线永远不再醒。差 1 毫秒让判据成为真值；生产里定时器
本来就只会晚到，测试里（`ui.Tester.Advance` 按精确算术推进）这一毫秒是必须的。

### 2. 七个调用点改成它

每处只换「要帧」那一步，**时钟本身、窗口长度、判据、墨、几何、文案、i18n 一个字节不动**：

| 位置 | 改成 |
|---|---|
| `fieldkeys.go:281` | `deadline.Wake(c, historyDeleteConfirm-a.now().Sub(a.armedDeleteAt))` |
| `power.go:103` | `deadline.Wake(c, powerConfirmWindow-a.now().Sub(a.pendingPowerAt))` |
| `launcher.go:3053` | `deadline.Wake(c, a.toastUntil.Sub(now))` |
| `debounce.go:105` | `deadline.Wake(c, a.debounceAt.Sub(a.now()))` |
| `savealert.go:55` | `deadline.Wake(c, saveFailedWindow-a.now().Sub(a.saveFailedAt))` |
| `settingsui.go:6067` | 三个 map 各自算 `at.Add(confirmWindow)`，取最早，`deadline.Wake(c, next.Sub(now))` |
| `settingsui.go:6124` | `deadline.Wake(c, sessionCloseWindow-a.now().Sub(a.armedSessionAt))` |
| `terminalui.go:1265` | `deadline.Wake(c, a.feedbackUntil.Sub(now))` |

**每处用各自那一支时钟算剩余时长**（launcher/settingsui 的 `a.now()` 是可注入的，测试按这个
时钟手动推进；`c.Now()` 是另一个时钟域，混用会把剩余时长算错），传进 `Wake` 的只有
`time.Duration`。

### 3. 不是「不刷新」，是「该刷的时候刷」

- 到点前：`FramePending()` 为假，`ui.Tester` 的计时器表里有一个 `After`（`tt.Advance` 到那个
  时刻会**重建一次**）；
- 到点：一次重建，判据成立 → 句子/问句/便条消失（或防抖写盘发生）；
- 期间有别的运动（入场动画、`AnimateTo`、分页环）时，帧照旧由那个运动自己驱动。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回（**防抖窗口的长度与「到点必写」**）、i18n 文案、
命令接线：全部不动。本轮只换「谁来敲门」。

## 门槛命令 + PASS 基线（本轮实测值，修前）

```
gofmt -l cmd internal                              → 空
go vet ./...                                       → 净
GOOS=windows go vet ./...                          → 净
GOOS=darwin  go vet ./...                          → 净
go test -count=1 ./...                             → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 基线 2457
... | grep -c -- "--- SKIP"                        → 基线 52
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 基线 2009
GOOS=linux/windows/darwin go build ./...           → 三份过
```

## pin（修前必须红）

三条，各包一条，判据同形：**窗口内 Advance 一秒 = 构建 0 次，但有一个到点唤醒；到点那一帧
句子自己消失**。

1. `internal/launcher/deadlinewake_test.go` · `TestTheTimedToastWakesAtItsDeadline`
   （`a.clock = tt.Now`；`a.Feedback("…")` → 一秒内 `builds` 增 0、`FramePending()` 假、
   `tt.Advance(1s)` 后 toast 文本仍在；再过 4s 后文本消失）。
   修前：一秒内 `builds == 60`（60 Hz 测试器一帧一次）。
2. `internal/terminalui/deadlinewake_test.go` · `TestTheTimedFeedbackWakesAtItsDeadline`
   （`ShowFeedback` → 一秒 0 次构建；4.5s 后 `FeedbackVisible()` 假）。
3. `internal/settingsui/deadlinewake_test.go` · `TestTheSaveFailureBannerWakesAtItsDeadline`
   （`a.Now = tt.Now`；一次被拒的写入 → 一秒 0 次构建；5s 后 `SaveFailedVisible()` 假）。

外加 `internal/ci` 不新增扫描（本轮的判据是「有没有重建」，不是「有没有某个字符串」）。

## mutation 计划（≥2，实跑）

- **M1**：`fieldkeys.go` 的 `deadline.Wake` 换回 `c.AnimationFrame()` → launcher 那条 pin 的
  「一秒 0 次构建」变红。
- **M2**：`terminalui.go:1265` 换回 `c.AnimationFrame()` → terminalui 那条 pin 变红。
- **M3**：把 `deadline.Wake` 的 `wakeSettle` 去掉（在 deadline 恰好那刻醒）→ settingsui 那条
  pin 的「5s 后消失」变红（等于 3s 窗口的那条判据，恰好相等不成立，唤醒丢失、永远不消失）。
  这条同时说明 `+1ms` 不是装饰。

## 视觉证据

本轮不转视觉，但要证明**墨没动**：launcher 的 toast 带、terminalui 的失败提示带、
settingsui 的写失败便条各出一张深浅对照图，修前/修后 `cmp` 逐字节比对
（`/tmp/floter-P11-bq-{toast,note,alert}-{light,dark}-{before,after}.png`）。
