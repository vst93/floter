# P11-b 规格：剪贴板监视器的记忆——只在写成功之后记，只属于一次会话

方向来源：优先级 2（合约里未落地的条目）→ 实际是 P11-a 报告「仍待」第 9 条点名的剪贴板族
最后一块，但**侦察后改了题**：轮询间隔 600ms vs 参考 900ms 属「行为节奏」（见 §4 拍板表），
而侦察中发现监视器**记忆契约本身**有两处与参考不同，且整条监视循环**一条 pin 都没有**。

## 0. 意图

`App.captureClipboard` 的 `lastClipboardHash` 是「上次已经记下的内容」，它是监视器唯一的记忆。
参考把它放在**任务内部**、并且**只在写入成功时**才更新；本仓把它挂在 `App` 上、写入**之前**就更新。
于是两处可见差异，且循环行为无任何测试钉住。

## 1. 证据（本轮实测行号）

参考（`main` 分支，`src-tauri/src/clipboard_history/monitor.rs`）：

- `:178-186` `capture_result_hash`：`Ok(())` → `Some(hash)`；`Err` → `None`，
  注释「will retry」；对应单测 `:479-487` `failed_capture_is_retried_on_the_next_poll`。
- `:443` `let mut last_hash: Option<String> = None;`——记忆建在**被 spawn 的任务里**，
  `:445-455` 每次轮询 `last_hash.take()` 交给 `poll_once`，返回 `Some` 才写回。
- `:425-436` `start()` 幂等（已有 monitor 直接返回）；`:463-476` `stop()` 置 cancel + abort；
  `mod.rs:436-442` `sync_runtime` 开关两条分支都幂等 → **一次会话 = 一次 start/stop**。
- `:438-443` 900ms 定时器，先 `tick().await` 跳过立刻触发的那一拍（启动不读剪贴板）。

本仓（`internal/shell/system.go`，本轮实测）：

- `:441-466` `watchClipboard`：`clipboardWatching` 幂等 ✓；`time.NewTicker` 第一拍在 interval
  之后 ✓（等价于参考的「跳过立即那一拍」）；采样在 goroutine 内同步调用 → 天然不重叠 ✓
  （Go `Ticker` 通道容量 1，落后时丢拍不补，等价于 `MissedTickBehavior::Delay`）。
- `:487-491` 文件路径：`a.lastClipboardHash = capture.Hash` 在 `AddFilesCapture` **之前**。
- `:500-507` 文本/图片路径：`a.lastClipboardHash = plan.Hash` 在 `AddText`/`AddImageCaption`
  **之前**。
  → 一次写入失败（磁盘满、目录不可写、索引被占用）就被记成「已记下」，**永不重试**，
  与参考 `capture_result_hash` 相反。
- `:441-449` 会话开始**不清记忆**：`a.lastClipboardHash` 挂在 `App` 上跨会话存活。
  参考每次 `start()` 都从 `None` 起手。可见后果：清空历史后关掉再打开剪贴板历史，
  当前剪贴板里的内容**不会被重新记下**（参考会）。
- `internal/shell/capture_test.go` 覆盖了 `captureClipboard` 的**单次采样**，但
  `watchClipboard` 整个循环（`system.go:441-466`）**零 pin**：
  `grep -rn "watchClipboard\|clipboardPoll\|clipboardInterval\|lastClipboardHash" --include=*_test.go`
  → 无输出。

## 2. 落地项

1. `internal/shell/system.go:487-497`（文件分支）
   - 现状：先 `a.lastClipboardHash = capture.Hash`，再 `AddFilesCapture`，出错只 log。
   - 改成：先写，`err != nil` → log + `return false`（不记）；写成功才
     `a.lastClipboardHash = capture.Hash`，返回 `added`。
2. `internal/shell/system.go:498-519`（文本/图片分支）
   - 现状：先记 hash，再写，出错只 log。
   - 改成：同样「写成功才记」；`added=false, err=nil`（已是最前一条，折叠过）仍然记——
     与参考 `capture_result_hash(Ok(()))` 一致。
   - `selfCopied` 早退（`:503-506`）保持不记（本仓自有规则，参考没有写回抑制）。
3. `internal/shell/system.go:450-452`（会话边界）
   - 现状：goroutine 起手直接建 ticker，记忆沿用上一会话。
   - 改成：goroutine 第一句 `a.lastClipboardHash = ""`，注释引用 `monitor.rs:443`。
     放在 goroutine 内（而不是 `watchClipboard` 里 ticker 之前）是因为**只有这个 goroutine
     写这个字段**，放进去才与「一次会话」严格同生；也避免和测试/UI 线程抢写。
4. 新增 `internal/shell/watch_test.go`：把循环的契约钉住（5 条，见 §3）。
5. 代码注释里补一句「落后不补拍」的依据（`Ticker` 通道容量 1），免得下一个人以为要自己节流。

### 行为契约零改动红线

- 采样间隔 `clipboardPoll = 600ms` **不动**（§4 拍板表）。
- `captureClipboard` 的读取顺序（文件 → 图片+文本）不动；`selfCopied` 抑制不动；
  `planCapture` 不动；`clipboard.Store` 不动；设置写回、i18n、命令接线全不动。
- 本轮唯一的产品行为变化：**写失败会重试**、**新会话不再信任旧会话的记忆**。

## 3. 门槛与 pin

新增 pin（`internal/shell/watch_test.go`，全部 B 类：行为，测试钉的是产品当前应有的行为）：

| pin | 钉什么 | 现状 |
|---|---|---|
| `TestTheWatcherWaitsAnIntervalBeforeItsFirstRead` | 启动后一个 interval 内不读剪贴板 | 已符合（防回归） |
| `TestTheWatcherRecordsACopyOnce` | 同一次复制被多拍轮询只记一条 | 已符合 |
| `TestTheWatcherStopsWhenTheSettingTurnsOff` | 关掉开关后循环在一个 interval 内停、不再读 | 已符合 |
| `TestANewWatcherSessionForgetsWhatTheLastOneRecorded` | 新会话把当前剪贴板重新记下 | **改前红** |
| `TestAFailedCaptureIsRetriedOnTheNextPoll`（text / files 两个子测试） | 写失败不记忆，下一拍重试成功 | **改前红** |

门槛（本轮实测基线见报告 §3）：

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨（基线 2211）
GOOS=linux|windows|darwin go build ./...               # 三平台
```

非视觉轮：不出图。

## 4. 拍板表（本轮不实现）

- **轮询间隔 600ms vs 参考 900ms**（`system.go:419` vs `monitor.rs:21`）：本仓每分钟 100 次采样，
  参考 66 次。改它只影响「复制后多久进历史」与后台唤醒次数，属用户可感的节奏，需拍板；
  本轮不动，写进 `.pending-bugs.md` 拍板表。

## 5. mutation 计划

1. 把 `captureClipboard` 的文本分支改回「写之前就记 hash」→ `TestAFailedCaptureIsRetriedOnTheNextPoll` 红。
2. 删掉 goroutine 里的 `a.lastClipboardHash = ""` → `TestANewWatcherSessionForgetsWhatTheLastOneRecorded` 红。
3. 文件分支改回「写之前就记 hash」→ 同一测试的 `/files` 子测试红。
4. 删掉「开关关掉就 return」那一段（循环不再看开关）→
   `TestTheWatcherStopsWhenTheSettingTurnsOff` 红。
   - 实测记录：把 `return` 改成 `continue` **不会**变红——pin 钉的是「关掉开关后不再采样」，
     不是「goroutine 真的结束」（`continue` 只是空转，仍会留下一个活着的 goroutine，
     再打开时就有两个采样器）。要钉后者需要一个可观测的会话计数，属另一轮，写进报告「仍待」。
