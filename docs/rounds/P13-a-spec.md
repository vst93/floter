# P13-a 规格：终端会话宿主（S1）——程序活在 app 之外

**意图一句话**：把「终端会话」从 app 进程里搬出去：一个独立宿主进程持有 PTY + 有界环形缓冲 +
末屏，app（或任何客户端）通过 unix socket attach；退出 app 只 detach，重开 reattach，
程序与滚回都还在。

**为什么现在做**：用户 2026-10-30 晚定的新指令（`.pending-bugs.md` 顶部「终端会话做真持久」弧的 S1）。
现状证据（HEAD `ebecf011` 实测）：

- `internal/terminalui/terminalui.go:820-833` `Close`/`CloseAll`：会话就是进程内的
  `*terminal.Terminal`，`CloseAll` 逐个 `Term.Close()` —— **退出 app = 程序被杀**。
- `internal/shell/shell.go:690-694`、`:1405-1410`：两条退出路径都调 `Terminal.CloseAll()`，
  注释自己写着「一个留在关闭面板后面的程序就是没人能碰的程序」——那是**没有宿主时**的结论。
- `internal/terminalui/terminalui.go:496-560` 的长注释：视觉恢复（`Snapshot()` 回灌）已被
  明确废掉，理由是「新 shell 继承了死程序的模式」。**那句注释同时给出了正解**：`Snapshot()`
  的文档读者是「一个 attach 到服务器所持有的会话的终端」——也就是「客户端没有程序」的终端。
  本轮的宿主 + Conn 客户端正是这个形状，于是回灌重新合法，且不再污染任何活程序。
- mygo v0.7.3 已备好客户端那一半：`plugins/terminal/terminal.go:88-120` `Options.Conn`
  （喂什么显示什么）+ `ResizeByConn`，以及 `plugins/terminal/mirror.go:18-40` 的 `Mirror`
  （「服务器持有会话、客户端画」的官方形状）。
- 不能引 herdr-core（Rust 守护进程，见指令里的取舍）；不能引 cgo。PTY 自己写
  （`/dev/ptmx` + ioctl，约 130 行，纯 Go，与 mygo 的 MIT 实现同形）。

## 落地项（位置 → 现状 → 应改成）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1-1 | 新包 `internal/termsession` | 不存在 | 会话宿主：`Options` / `Info` / 帧协议 / 环形缓冲 / PTY / 宿主进程 / 客户端 |
| 1-2 | `internal/termsession/session.go` | — | `Ring`（默认 4 MiB，超出丢最旧）、`Info`（id/name/state/exitCode/**seq**/cols/rows/pid/dir）、帧编解码（`H` `S` `O` `E` `L` / `I` `R` `D` `K`）、`startPTY` |
| 1-3 | `internal/termsession/pty_unix.go` | — | darwin/linux：`/dev/ptmx` + grantpt/unlockpt/ptsname 的 ioctl、`TIOCSWINSZ`、`Setsid+Setctty` |
| 1-4 | `internal/termsession/pty_other.go` | — | Windows：`openPTY` 返回 `ErrUnsupported`（ConPTY 那一半留给 S1b；**不假装支持**） |
| 1-5 | `internal/termsession/host.go` | — | `Start`/`Run`/`Stop`：读 PTY → 环形缓冲 + 广播；程序结束 → `StateExited` + `E` 帧；无客户端 `Linger`（默认 30s）后自退；socket 0700/0600 |
| 1-6 | `internal/termsession/client.go` | — | `Dial` 读 `H` + `S`（末屏进缓冲），之后 `O` 续流；`Read/Write/Resize/ResizePixels/Close` —— **直接满足 `terminal.Options.Conn` 的接口** |
| 1-7 | `cmd/floter/main.go` | 只有 app 一条路径 | 最前面一条隐藏子命令 `floter __termhost ...`，在 `RequestSingleInstanceLock` **之前**分流 |
| 1-8 | 落后处理（S1 的最小版） | — | 客户端队列（256 帧）满 = `lagged`：先补一帧 `L`，再补一帧**当前 `S`**，然后续流；绝不无限堆 backlog（credit 流控留给 S2） |

**不做的（本轮明确）**：app 侧接线（把 `terminalui` 的会话换成 `Dial`）留给 **S1b**——
它要改 `NewTerminal` 注入面与 `EnsureSession`/`CloseAll` 的语义，风险与本轮不是一件事。
本轮交付的是**宿主 + 协议 + 客户端 + 一个进程级的存活证明**，并且可以在真机上手动验
（`floter __termhost` 起宿主，`--socket` attach）。

## 红线（零改动）

高度链、键盘模型、会话管理（`maxSessions`/`trimSessions`/`CloseSession` 的语义）、设置写回、
i18n 文案、命令接线、`docs/liquid-glass.md` 的全部视觉 token。本轮**不碰 UI**，只加包与一条入口分流。
`terminal-snapshot` 那条「不回灌」的既有结论**继续有效**：回灌只允许发生在**没有程序**的客户端终端里。

## 门槛（基线用实测当前值：HEAD `ebecf011`，2805 PASS / 0 FAIL / 59 SKIP / 2084 测试函数）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 只许涨
GOOS=linux/windows/darwin go build ./...  # 过
```

## mutation 计划（每条都会让哪个 pin 变红）

1. 把 `Ring.Write` 的溢出分支改成「保留最旧」（`drop := 0`）→
   `TestTheRingKeepsTheNewestBytesWithinItsBound` 红（尾部字节不再是最后一次写入）。
2. 把 `serve` 里的 `addClient` 挪到 `H`/`S` 之前 → `TestTheAttachScreenComesBeforeLiveOutput` 红
   （首帧 `S` 与 `O` 的交错顺序被打乱）。
3. 把宿主 `watch()` 里的 `pty.Wait()` 之后不清 `StateRunning` → `TestTheHostReportsTheProgramsExit` 红。
4. 把 `client.Read` 对 `TagLost` 的处理去掉（不丢 pending）→ `TestALaggingClientResyncsOnTheScreen` 红。
