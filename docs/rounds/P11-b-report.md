# P11-b 报告：剪贴板监视器的记忆——只在写成功之后记，只属于一次会话

规格 `docs/rounds/P11-b-spec.md`。方向：P11-a 报告「仍待」第 9 条（剪贴板轮询间隔 600 vs 900）
**侦察后改题**——间隔属行为节奏（写进 `.pending-bugs.md` 拍板表），而侦察发现监视器的
**记忆契约**有两处与参考不同、并且整条监视循环**一条 pin 都没有**。

## 1. 侦察：参考的记忆放在哪、什么时候更新

`main:src-tauri/src/clipboard_history/monitor.rs`：

- `:443` `let mut last_hash: Option<String> = None;`——**记忆建在被 spawn 的任务里**，
  `:445-455` 每拍 `last_hash.take()` 交给 `poll_once`，只有返回 `Some` 才写回。
- `:178-186` `capture_result_hash`：`Ok(())` → `Some(hash)`；`Err` → `None`（注释「will retry」）。
  对应单测 `:479-487` `failed_capture_is_retried_on_the_next_poll`。
- `:425-436` `start()` / `:463-476` `stop()` 都幂等；`mod.rs:436-442` `sync_runtime` 两条分支幂等
  → **一次会话 = 一次 start/stop**，会话结束记忆随之消失。
- `:438-443` 900ms 定时器，先 `tick().await` 跳掉立刻触发的那一拍（启动不读剪贴板）；
  `:439` `MissedTickBehavior::Delay`（落后不补拍）。

本仓改造前（`internal/shell/system.go`，本轮实测行号）：

| 位置 | 现状 | 后果 |
|---|---|---|
| `:487-491` | 文件分支先 `a.lastClipboardHash = capture.Hash`，再 `AddFilesCapture` | 写失败也记住 |
| `:500-507` | 文本/图片分支先记 hash，再 `AddText`/`AddImageCaption` | 写失败也记住 → **永不重试** |
| `:450-452` | goroutine 起手直接建 ticker，记忆挂在 `App` 上跨会话 | 清空历史后关掉再打开，当前剪贴板内容不会被重新记下 |
| `:441-466` | 整个循环 | **零 pin**：`grep -rn "watchClipboard\|clipboardPoll\|clipboardInterval\|lastClipboardHash" --include=*_test.go` → 无输出 |

循环里**已经对的三件事**（本轮只补 pin，不改代码）：`clipboardWatching` 幂等；`Ticker` 第一拍在
一个 interval 之后（= 参考跳过立即那拍）；采样在 goroutine 内同步调用 → 天然不重叠，且 Go
`Ticker` 通道容量 1，落后丢拍不补（= `MissedTickBehavior::Delay`）。这三条的依据写进了代码注释。

## 2. 落地

1. 文件分支（`system.go:487-497`）：写在前，`err != nil` → log + `return false`（不记）；
   写成功才记 hash，返回 `added`。
2. 文本/图片分支（`system.go:498-521`）：同样「写成功才记」；`added == false, err == nil`
   （折叠过的重复）**仍然记**——与参考 `capture_result_hash(Ok(()))` 一致。
   `selfCopied` 早退保持不记（本仓自有规则，参考没有写回抑制）。
3. goroutine 第一句 `a.lastClipboardHash = ""`（`system.go:451-458`）：一次会话一份记忆。
   放在 goroutine 内而不是 `watchClipboard` 里，因为**只有这个 goroutine 写这个字段**，
   放进去才与会话严格同生，也不和 UI 线程抢写。
4. 新增 `internal/shell/watch_test.go`（5 条 pin，见 §3）。
5. 代码注释补上「落后不补拍」与「写成功才记」的依据行号，免得下一个人自己加节流。

产品行为只变两点：**写失败会重试**、**新会话不再信任旧会话的记忆**。高度链、键盘模型、会话管理、
设置写回、i18n、命令接线、`captureClipboard` 的读取顺序（文件 → 图片+文本）、`planCapture`、
`clipboard.Store` 全部零改动。

## 3. 门槛（本轮实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 34 ok + 2 no test files，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2218（基线 2211，+7：5 个测试 + 2 个子测试）
go test -count=1 -v ./... | grep -c -- "--- SKIP"      → 15（=）
测试函数                                                → 1789（基线 1784，+5）
GOOS=linux|windows|darwin go build ./...                → 全过
```

台账：**V=0 / B=5 / S=0**（5 个测试，其中 `TestAFailedCaptureIsRetriedOnTheNextPoll` 跑 text / files 两个子测试）。五条 pin 都是行为类（B）：三条钉住**已经对的行为**（启动静默、
一次复制一条、关掉不再采样），两条钉住本轮修好的行为（新会话重新记下、写失败重试），
**改前两条是红的**（见 §4 变异 1、2 —— 那正是修复前的代码）。

非视觉轮：本机渲染输出逐字节不变，**不出图**。

## 4. mutation（4 条实跑，3 条按预期变红，1 条抓到 pin 的边界）

1. 文本分支改回「写之前就记 hash」（修复前的代码）：
   ```
   --- FAIL: TestAFailedCaptureIsRetriedOnTheNextPoll (0.00s)
       watch_test.go:215: the failed capture was remembered as
       "41e50568f5685cbca9ee53c7c275868010f95a8f9bba36bf6c1193c78b378e49",
       so it would never be retried
   ```
2. 删掉 goroutine 里的 `a.lastClipboardHash = ""`：
   ```
   --- FAIL: TestANewWatcherSessionForgetsWhatTheLastOneRecorded (5.01s)
       watch_test.go:186: timed out waiting for the new session to record the clipboard again
   ```
3. 删掉「开关关掉就 return」那一段（循环不再看开关）：
   ```
   --- FAIL: TestTheWatcherStopsWhenTheSettingTurnsOff (0.25s)
       watch_test.go:158: the watcher kept reading the clipboard 5 times after the
       setting was turned off
   ```

4. 文件分支改回「写之前就记 hash」（同上，另一条写路径）：
   ```
   --- FAIL: TestAFailedCaptureIsRetriedOnTheNextPoll (0.00s)
       --- FAIL: TestAFailedCaptureIsRetriedOnTheNextPoll/files (0.00s)
           watch_test.go:235: the failed capture was remembered as
           "933a112d92c2e6eb9ec2b325fc1c814a36b2bfb0f6b7d2ae772d33463ffaf260",
           so it would never be retried
   ```

**边界（诚实记录）**：把 `return` 改成 `continue` **不会**变红——pin 钉的是「关掉开关后不再采样」，
不是「goroutine 真的结束」。`continue` 会留下一个活着的 goroutine，再打开开关就有两个采样器
（并共享 `lastClipboardHash` 而无锁）。要钉「永远只有一个采样器」需要一个可观测的会话计数，
本轮不做，写进 `.pending-bugs.md` 仍待第 6 条。

每条还原后 `md5sum -c` → `internal/shell/system.go: 成功`，`touch` 两个文件后
复跑 `go test -count=1 -run 'Watcher|FailedCapture' ./internal/shell/` → `ok … 2.012s`。

## 5. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 57 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）+ 本轮新增一条：**剪贴板轮询间隔 600 vs 900**。
3. 剪贴板族只剩一条行为差：「复制回来的条目该不该置顶」（要拍板，牵连四处 pin）。
4. 本轮发现未做：**一个采样器**的可观测性（§4 边界）。
5. 本轮发现未做：`clipboard.Store` 的 `AddText`/`AddImageCaption` 在 `saveLocked` 失败时条目已经
   插进内存（`insertLocked` 在 save 之前），失败的写入在面板里仍可见、重启后消失；参考
   `mutate_history`（`mod.rs:119-139`）是否回滚未核。属「写失败的回滚语义」，另一轮。
6. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
7. icns 落盘缓存（P10-t）、SVG/XPM 图标（P10-s）、非 macOS 快捷键词序与 `Esc`/`Escape`（P10-y）。
8. 下一轮建议：接第 5 条——**写失败的回滚语义**（`internal/clipboard` 一处 + 参考 `mutate_history`
   逐条核对 + pin），它比剩下的小差更贴「基础功能」。若判为需拍板，则转
   `internal/extensions/cmdshim.go` 的 `cmd.exe /D /S /C` 与参考 `cmd /c` 拼写核对。
