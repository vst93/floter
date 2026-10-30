# P13-a 报告：终端会话宿主（S1）——程序活在 app 之外

**这一轮做了什么**：新包 `internal/termsession`（宿主进程 + 帧协议 + 有界环形缓冲 + 客户端），
`cmd/floter` 加两条隐藏子命令（`__termhost` / `__termattach`）。退出 app 只 detach，重开 reattach，
程序与滚回都还在——在真机上验过（下面「真机验证」）。

**方向一句话**：把「终端会话」从 UI 的生命周期里搬出去，程序由独立宿主持有，app 变成可 attach/detach
的客户端。这是 S1 弧的第一片，视觉队列 `docs/liquid-glass.md` §6.1 排在这条弧之后。

## 一、为什么是「抄机制」而不是「引 herdr-core」

用户 2026-10-30 晚的指令已经定了取舍（不引 Rust 守护进程、不破纯 Go 三平台构建），本轮只把其中
**五条机制**落到本仓已有的原语上：

| herdr-core 的机制 | 本轮的落法 | 证据 |
|---|---|---|
| 首屏 + 增量，不是整屏快照 | attach 时先 `H`（记录）+ `S`（环形缓冲里的屏幕），再 `O`（之后的原始输出）；**不重放任何模式序列** | `host.go` `writeLoop`、`host.go` `readAttach` |
| credit 流控 + 落后重绘 | 客户端队列 256 帧，满 = 一次 `L`（丢积压）+ 一帧当前 `S`；宿主侧有界（环形 4 MiB） | `host.go` `broadcast`、`Session.screen` |
| holder / keep 语义 | 持有 PTY 的进程独立于 UI；程序结束的 `E` 帧带退出码与原因（`exited`/`killed`），**绝不重跑原命令** | `host.go` `watch`、`Info.State` |
| 状态来自记录 + seq | `Info` 带 `State`/`ExitCode`/单调 `Seq`，每次变更 +1，attach 与 `E` 帧都带它 | `session.go` `Info`、`Host.watch` |
| 记录 / 事件模型 | 同上，`seq` 就是「落后就重取快照」的判据 | `TestTheHostReportsTheProgramsExit` |

**与 `terminalui.go:496-560` 那段长注释的关系**：那段说「`Snapshot()` 回灌会污染新 shell」，
结论仍然成立，**而且它自己给出了正解**——`Snapshot()` 的文档读者是「attach 到服务器所持有会话的
终端」，也就是**没有程序**的客户端。本轮的客户端正是这个形状（`Options.Conn`），所以「首屏」
在这里不是回灌，而是**客户端终端的第一次绘制**。旧的 `terminal-snapshot` 文件路径没有被复活。

## 二、落地项

| # | 位置 | 落成 |
|---|---|---|
| 1-1 | `internal/termsession/session.go` | `State`（starting/running/exited/killed/lost）、`Info`（id/name/state/exitCode/**seq**/cols/rows/pid/dir）、帧编解码（`H S O E L` / `I R D K`）、`frameSize` 1 MiB 上限 |
| 1-2 | `internal/termsession/host.go` | `Ring`（默认 4 MiB，丢最旧）、`Session`、`conn`、`Host.Start/Serve/Stop`、`Client.Dial/Read/Write/Resize/ResizePixels/Close/Kill`、`readAttach` |
| 1-3 | `internal/termsession/pty_unix.go` + `pty_linux.go` / `pty_darwin.go` | `/dev/ptmx`、grantpt/unlockpt/ptsname 的 ioctl、`TIOCSWINSZ`、`Setsid+Setctty` |
| 1-4 | `internal/termsession/pty_other.go` | Windows：`startPTY` 返回 `ErrUnsupported`。**不假装支持**，ConPTY 留给 S1b |
| 1-5 | `internal/termsession/host.go` | socket 目录 0700 / 文件 0600；程序结束 → `E` 帧 + `StateExited`；无客户端 `Linger`（默认 30s）后自退；负 `Linger` = 程序等客户端 |
| 1-6 | `internal/termsession/command.go` | `__termhost`（`-program/-dir/-socket/-cols/-rows/-name/-linger/-ring/-env`）、`__termattach`（`-socket/-type/-wait/-kill`）、`UserShell`（登录 shell）、`Environment`（TERM/COLORTERM/LANG） |
| 1-7 | `cmd/floter/main.go` | 两条子命令在 `RequestSingleInstanceLock` **之前**分流：宿主不是 app，不能被在跑的 app 拒绝，也不能吃第二次启动的参数 |
| 1-8 | `host.go` `writeLoop` | 落后：一次 `L` + 一帧 `S` + 空 `S` 终止帧，之后续流；`lagged` 保证**只警告一次** |

**S1 明确没做的**：app 侧接线（把 `terminalui` 的会话换成 `Dial`）——那要改 `NewTerminal` 注入面与
`EnsureSession`/`CloseAll` 的语义，是 S1b 的一片，已写进 `.pending-bugs.md`「仍待」。
`terminal-snapshot` 与 capture 通道的迁移是 S3。

## 三、协议（写下来，S2 在它上面加 credit）

```
宿主 → 客户端:  H <json Info>     记录（每次 attach 一次，落后重绘时再一次）
                S <bytes>         屏幕（环形缓冲，按 1 MiB 切块）
                S <>              屏幕结束（空帧，**终止符**）
                O <bytes>         之后的原始输出
                E <json Info>     程序结束（state/exitCode/seq）
                L <>              你落后了：一次 H + 整屏 S + 空 S 跟着来
客户端 → 宿主:  I <bytes>         键入
                R <cols><rows>    尺寸（两个大端 uint16）
                D <>              detach（程序留下）
                K <>              kill（程序结束，state=killed）
```

**为什么需要那个空 `S` 终止帧**：没有它，「屏幕」与「屏幕之后」的边界是客户端猜出来的
（读完一批就停），而这正是 `TestTheDetachKeepsTheProgram` 里那次 attach 早期版本会随机漏字的
根因。有了终止帧，`readAttach` 的返回时机是**协议事实**，`TestTheAttachFrameOrder` 钉住它。

## 四、V/B/S 台账

**本轮没有视觉改动**，所以没有 V 类。B/S：

| 类 | 结论 |
|---|---|
| **B**（行为/度量/接线） | 既有 2095 个测试函数**零改动**全绿（2848 PASS，与基线 2836 相比 +12 全是本轮新增） |
| **S**（结构） | 无既有结构变动：只新增包与两条子命令；`shell`/`terminalui`/`settingsui` 一行未动 |
| **V** | 无 |

新增 pin 15 条（`session_test.go` 7 / `host_test.go` 5 / `command_test.go` 4 /
`platform_other_test.go` 1 按平台只在非 unix 上跑 / `internal/ci` 2 条合约）。

## 五、门槛（全部实测，HEAD 之前的 `ebecf011` 为基线）

```
gofmt -l cmd internal            → 空
go vet ./...                     → 净
GOOS=windows go vet ./...        → 净
GOOS=darwin  go vet ./...        → 净
go test -count=1 ./...           → 0 FAIL
PASS 计数                        → 2853（基线 2836）
SKIP                             → 61（基线 61；termsession 自身在 linux 上 0 skip）
测试函数                         → 2113（基线 2095）
GOOS=linux/windows/darwin build  → 全过
```

## 六、mutation（6 条，全部实跑变红并逐字节还原）

1. **环形缓冲不夹紧 `n`**（`if r.n > r.limit` → `if false`）
   → `TestTheRingKeepsTheNewestBytesWithinItsBound` 红：
   `ring = "qrstuvwxijklmnopqrstuvwx", want the newest 16 bytes` / `len = 24, want 16`。
2. **落后时不丢积压**（去掉 `drain(c.send)`）
   → `TestALaggingClientResyncsOnTheScreen` 红：`the lagging client got O, want the resync notice L`。
3. **终止帧写在屏幕块之前**（`S <>` 提到 `for _, chunk := range screen` 之前）
   → `TestTheAttachFrameOrder` 红：`the screen = "", want what the program printed before the attach`。
4. **程序结束不区分 killed**（`watch` 里无条件 `StateExited`）
   → `TestTheKillEndsTheProgram` 红：`the record = {... State:exited ExitCode:137 ...}, want killed`。
5. **宿主分流派到单实例锁之后**（把 `termsession.IsCommand` 的两段挪到锁下面）
   → `internal/ci` 的合约红：`the host subcommand is dispatched at offset 986, after the
   single-instance lock at 927`。
6. **客户端不画落后重绘的屏幕**（`case tagScreen` 里不追加 payload）
   → `TestAClientTakesTheScreenAfterALag` 红：`after a lag the client drew "live", want
   "SCREEN-ASCREEN-Blive"`。

每条都 `cp` 还原后用 `md5sum` 逐字节核对一致。

## 七、真机验证（这才是这一片的意义）

```
$ floter __termhost -program /bin/sh -socket /tmp/smoke/s.sock -cols 100 -rows 30 -linger -1s &
$ floter __termattach -socket ... -type $'sleep 400; echo LONG-DONE\r' -wait 1.5s
sh running seq=1 100x30 pid=2631092
$ ps -ef | grep -c '[s]leep 400'      → 2   （宿主 + 客户端都退出后，长命令还在）
$ floter __termattach -socket ... -type $'echo REATTACH-OK\r' -wait 1.5s
sh running seq=1 100x30 pid=2631092   （同一个 pid：不是新 shell）
… 屏幕里是上一次 attach 留下的 `sleep 400; echo LONG-DONE`，说明首屏是会话的屏幕
$ floter __termattach -socket ... -kill -wait 1s
$ kill -0 <hostpid>                   → 宿主已退，socket 已删
```

**「退出 floter → 重开 → attach 回去」在 S1b 之后才是 GUI 路径**；本轮验的是它下面的那一层
（宿主 + 协议 + 客户端），并且 attach 用的是同一份 `Client` 代码（`__termattach` 与未来的
`terminal.Options.Conn` 是同一个 `Client`）。

## 八、仍待

1. **S1b**：`terminalui` 的 `NewTerminal` 换成 `Dial` + `Options.Conn{ResizeByConn}`；
   `EnsureSession`/`CloseSession`/`CloseAll` 的语义要改（`CloseAll` 现在是「退出 app 就杀程序」，
   必须变成「detach，宿主按 `Linger` 收」）。**这会改会话管理的语义**，按红线先写进
   `.pending-bugs.md`「需用户拍板」。
2. **S1c（Windows）**：ConPTY 的 `openPTY`（`CreatePseudoConsole` + 进程属性表）。mygo 的
   `plugins/terminal/internal/pty/pty_windows.go` 是 MIT 的同形实现，可参照；本轮只保证
   Windows **构建/vet 通过 + 明确报 Unsupported**。
3. **S2**：credit 流控（客户端消费多少回多少）、`FOCUS` 帧、`RESIZE` 的像素尺寸。
   现在 `ResizePixels` 直接退化成 `Resize`（只传格子数）。
4. **S3**：list / kill / 结束原因 / 崩溃后重连；`terminal-snapshot` 与 capture 通道的迁移。
5. **S4（可选）**：OSC 7501 状态解析。
