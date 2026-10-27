# P11-e 报告：剪贴板开关的一次开关就是一次会话

规格 `docs/rounds/P11-e-spec.md`。方向：P11-b 报告「仍待」第 4 条（**一个采样器**的可观测性，
零 pin）。侦察后它不是「缺 pin」，而是**生命周期契约与参考不同**，并且那条差异用户看得见。

## 1. 侦察：参考的会话边界在哪

`main:src-tauri/src/clipboard_history/`：

- `mod.rs:436-442` `sync_runtime`：**每次设置变更**无条件 `monitor::start` 或 `monitor::stop`，
  两条分支都幂等（注释：「callers need not diff first」）。
- `monitor.rs:425-436` `start`：槽位非空直接返回（幂等）；空则 spawn 新任务，
  **`last_hash` 建在任务里**（`:443`，P11-b 已核对）。
- `monitor.rs:463-476` `stop`：`slot.take()` → `cancel.store(true)` → `handle.abort()`——
  **当场**结束，不等下一拍；`:445-455` 循环每拍只看自己的 cancel 标志。

本仓改前（`internal/shell/`，本轮实测行号）：

| 位置 | 现状 | 后果 |
|---|---|---|
| `system.go:441-477` | 循环每拍读设置，发现关掉才 `clipboardWatching = false; return` | 关掉后采样器活到**下一拍**（600ms）才退 |
| `system.go:466` / `shell.go:1142` | goroutine 写 `clipboardWatching`，UI 线程读同一字段 | 跨 goroutine 无同步写 |
| `shell.go:1142` | `if state.Enabled && !a.clipboardWatching` | **快速开关**（关→下一拍前又开）：flag 仍 true → 不起新会话；老 goroutine 下一拍看到 Enabled 又是 true → 继续跑，**记忆没清** |
| `watch_test.go:84-97` `stopWatcher` | 只证明「不再读了」 | 证明不了 goroutine 结束，也覆盖不到快速开关 |

快速开关的用户可见后果：**关掉 → 清空历史 → 马上打开**，当前剪贴板内容**永远不会被重新记下**，
直到剪贴板内容变一次。改前对照实测（见 §4）就是这一条超时。

## 2. 落地

- `internal/shell/system.go`：新增 `clipboardSession{stop, done chan struct{}}`（= 参考的
  `MonitorHandle`：`stop` 是 `cancel`，`done` 是可观测的结束点）。`watchClipboard` 幂等
  （已有一个就返回，注释指 `monitor.rs:425-436`）；循环改成 `select { <-stop: return; <-ticker.C: }`，
  **唯一的出口是 stop 通道**，删掉每拍读设置那段（它顺带在 goroutine 里写 UI 线程的字段）。
- 新增 `stopClipboard`：`nil` 直接返回（幂等，`monitor.rs:463-476`）；`clipboardSession = nil`、
  `clipboardDraining = session.done`、`close(session.stop)`。两个字段都只由起停方（UI 线程）写。
- `internal/shell/shell.go:1136-1151`：设置变更处理器改成参考 `sync_runtime` 的形状——
  `if state.Enabled { watchClipboard() } else { stopClipboard() }`。
- **记忆的交接是排队的**：`clipboardDraining` 把「已被叫停但可能还在一次 capture 里」的会话
  交给下一个会话，新 goroutine 起手 `<-draining` 之后才 `lastClipboardHash = ""`。
  没有这一句，旧 capture 的哈希会落在新会话的清空之后（一次慢剪贴板读取就能触发），
  等于把本节要修的那个 bug 在微秒尺度上重新引入。
- `clipboardSessions` 计数：一次 enable 一个会话（可观测）。

## 3. 台账

**V=0 / B=4 / S=0**。规格预估 S=1，实测 S=0：结构确实变了（`clipboardWatching bool` →
`clipboardSession` 指针），但**没有任何既有断言引用那个字段**，`watch_test.go` 的 5 条 watcher pin
（`TestTheWatcher*`、`TestANewWatcherSessionForgetsWhatTheLastOneRecorded`、`TestAFailedCaptureIsRetried…`）
**零改动全绿**。新增 4 条全是行为 pin。纯行为轮，不出图（`docs/raycast-design.md` 不动）。

## 4. 门槛（本轮实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 34 ok + 2 no test files，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2231（基线 2227，+4）
--- SKIP                                               → 15（=）
--- FAIL                                               → 0
测试函数（grep -rh "^func Test"）                       → 1802（基线 1798，+4）
GOOS=linux|windows|darwin go build ./...               → 全过
```

## 5. 变异（4 条实跑 + 1 条改前对照）

改前对照（`git stash` 掉两份产品文件，只留一条只用 HEAD 符号的 pin）：

```
--- FAIL: TestPreChangeQuickToggle (5.32s)
    prechange_test.go:25: timed out waiting for the new session to record the clipboard again
```

| # | 注入 | 实测报错 |
|---|---|---|
| M1 | 设置处理器删掉 `else { a.stopClipboard() }` | `session_test.go:39: the sampler is still registered after the history was turned off` / `:72: timed out waiting for the new session to record the clipboard again` / `:113: 1 sessions after off and on again, want 2` / `:170: the stopped sampler never ended after its capture returned`（4 条全红） |
| M2 | 删掉 goroutine 起手的 `a.lastClipboardHash = ""` | `:72: timed out waiting for the new session to record the clipboard again` / `:174: timed out waiting for the new session to clear the memory` |
| M3 | `clipboardSessions++` 挪到早退之前 | `:107: 2 sessions after a settings change that left the switch on, want 1` |
| M4 | 删掉 `<-draining`（交接不排队） | `:174: timed out waiting for the new session to clear the memory` |

每条注入后 `cp` 回原文件并 `md5sum -c` 逐字节核对（`9711cdd9…` shell.go / `deece0c3…` system.go），
`touch internal/shell/*.go` 后再复跑一次，避免测试缓存复用。

## 6. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 60 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）：**#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 无扩展名返回 `""`、register 只留 `cmd`、`open` 是否拒一切 query、
   未知宿主是否静默忽略、**剪贴板轮询间隔 600 vs 900**、「复制回来的条目是否置顶」。
3. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
4. icns 落盘缓存（P10-t）、SVG/XPM 图标（P10-s）、非 macOS 快捷键词序与 `Esc`/`Escape`（P10-y）。
5. **本轮发现、未做（要拍板）**：P11-d「仍待」第 6 条不变——参考的计算器每次写都从磁盘重读
   （`update` 里 `load_index()`），本仓 `Store` 只在 `NewStore` 读一次。剪贴板 P10-w 已用
   `loadLocked` 处理过同一问题；计算器要不要刷新是语义选择。
6. 下一轮建议：剪贴板族与计算器族都已收口（记忆 / 写失败回滚 / 会话边界），离开这两个族。
   按「三平台一致性 → 基础功能 → 像素细节」自己找下一处；`internal/spawn` 的 detach 两臂
   已由 P10-z 收口、`internal/extensions/cmdshim.go` 的 `cmd.exe /D /S /C` 是**故意**的
   （`/S` 是自有引号契约，`i18n/copy.go` 的注入说明依赖它），都别重领。
