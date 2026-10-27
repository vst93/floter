# P11-e 规格：剪贴板开关的一次开关就是一次会话

方向：P11-b 报告「仍待」第 4 条（**一个采样器**的可观测性，零 pin）。侦察后它不只是「没有 pin」，
而是一条**与参考不同**的生命周期契约（见 §1 第 2 条），所以本轮既补可观测性也修行为。

## 1. 意图

**把开关当作会话边界**：关掉剪贴板历史，采样器**当场**结束；再打开就是**新的一次会话**，
带着空的记忆重新读一遍剪贴板。为什么现在做——这是用户能看见的：

1. 本仓关掉历史后，采样器要等到**下一拍**（生产值 600ms，`system.go:419`）才自己发现并退出，
   `clipboardWatching` 也由那个 goroutine 写（`system.go:464-468`），而 UI 线程在
   `shell.go:1142` 读同一个字段——**跨 goroutine 无同步写**。
2. 更要紧的是快速开关：关掉→在下一拍之前又打开时，`shell.go:1142` 的
   `state.Enabled && !a.clipboardWatching` 看到 `clipboardWatching` 仍为 true → **不启动新会话**；
   老 goroutine 下一拍看到 `Enabled` 又是 true → 继续跑，**记忆（`lastClipboardHash`）没被清空**。
   于是「关掉 → 清空历史 → 马上打开」之后，当前剪贴板内容**永远不会被重新记下**，直到它变一次。
   参考不会这样：`clipboard_history/mod.rs:436-442` 的 `sync_runtime` 每次设置变更都无条件
   `start`/`stop`，`stop` 是 `cancel.store(true)` + `handle.abort()`（`monitor.rs:463-476`）——**当场**结束；
   `start` 看到槽位空了就 spawn 一个新任务，`last_hash` 建在新任务里（`monitor.rs:443`，P11-b 已核对）。
3. 这条链路**一条 pin 都没有**：`stopWatcher`（`watch_test.go:84-97`）只证明「不再读了」，
   既证明不了 goroutine 真的结束，也覆盖不到第 2 条的快速开关。

## 2. 落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `shell.go:380-381` | `clipboardWatching bool` | 换成 `clipboardSession *clipboardSession`（`nil` = 没在跑）；新增 `clipboardDraining chan struct{}` 与 `clipboardSessions int` |
| `system.go:441-477` | 起手建 ticker，靠**每拍读设置**发现该退出，并在 goroutine 里写 `clipboardWatching` | 起手 `select` 一个 `stop` 通道；退出只由 `stop` 决定；goroutine 只写会话自己的东西 |
| `shell.go:1136-1147` | `if state.Enabled && !a.clipboardWatching { a.watchClipboard() }` | 两条分支都幂等，照参考 `sync_runtime`：`if state.Enabled { a.watchClipboard() } else { a.stopClipboard() }` |
| 新增 `stopClipboard` | 无 | `session := a.clipboardSession`；`nil` 直接返回（幂等）；`clipboardSession = nil`、`clipboardDraining = session.done`、`close(session.stop)` |
| `system.go:457` | goroutine 起手 `a.lastClipboardHash = ""` | 前面加 `<-draining`（若上一次会话还没退干净）：**记忆的交接是排队的**，新会话清空之前，旧会话的最后一次写必须已经落地 |
| `system.go:464-468` | 循环里 `if !Enabled { clipboardWatching = false; return }` | 删掉（设置面已经负责起停）；退出走 `select` 的 `stop` 分支 |

`clipboardSession` 就是参考的 `MonitorHandle`（`monitor.rs:418-423`）：`stop` 通道 = `cancel`，
`done` 通道 = `JoinHandle` 的可观测结束点。

## 3. 红线（零改动）

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、`captureClipboard` 的读取顺序
（文件 → 图片+文本）、`planCapture`、`clipboard.Store`、轮询间隔 `clipboardPoll = 600ms`
（属拍板项，本轮不动）、`selfCopied` 抑制。**只改「谁在什么时候采样」，不改「采样什么」。**

## 4. 新增 pin（`internal/shell/session_test.go`）

1. `TestTurningTheHistoryOffEndsTheSamplerNow`：间隔 5s（测试里没有一拍会到），关掉后
   `clipboardSession == nil` 且 `done` 在 500ms 内关闭。
2. `TestTogglingOffAndOnWithinOneIntervalIsANewSession`（行为 pin，改前也编译）：记录一次 → 清空历史 →
   在一个间隔内关掉再打开 → 当前内容必须被重新记下。改前：超时。
3. `TestOneEnableIsOneSession`：开着时再改一次设置（容量变化）不新开会话；关→开算第二次；
   `stopClipboard` 连调两次不 panic。
4. `TestTheNewSessionTakesOverOnlyAfterTheOldOneIsGone`：用可控的 `clipboardSample` 把第一次会话卡在
   capture 里 → 关 → 开 → 放行；旧会话把记忆写成 A，新会话在 `<-draining` 之后才清空 →
   断言此刻 `lastClipboardHash == ""` 且 `len(Entries()) == 1`。去掉 `<-draining` 即红。

## 5. 门槛与基线（P11-d 收尾实测）

```
gofmt -l cmd internal                              → 空
go vet ./...                                       → 净
go test -count=1 ./...                             → 34 ok + 2 no test files，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2227（基线 2227，本轮预期 2232）
--- SKIP                                           → 15（=）
测试函数                                            → 1798（预期 1802）
GOOS=linux|windows|darwin go build ./...           → 全过
```

## 6. mutation 计划

- M1 删掉 OnChange 里的 `else { a.stopClipboard() }`（退回「等下一拍」）→ pin 1、2 红。
- M2 删掉 goroutine 起手的 `a.lastClipboardHash = ""` → pin 2 红。
- M3 把 `a.clipboardSessions++` 挪到「已经在跑」的早退之前 → pin 3 红。
- M4 删掉 `<-draining` → pin 4 红。

台账预期 **V=0 / B=4 / S=1**（`clipboardWatching bool` → 会话结构：结构真变了，pin 与注释按新结构写）。
纯行为轮，不出图。
