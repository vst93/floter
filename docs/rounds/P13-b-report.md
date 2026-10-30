# P13-b 报告：会话协议的第二片——按字节的额度流控 + 带像素的尺寸

**这一轮做了什么**：把宿主对客户端的输出从**帧数**上限换成**字节额度**（客户端消费多少、
回多少），并补上 herdr-core 的第二级「落后上限」；`RESIZE` 现在带格子的像素尺寸，
`TIOCGWINSZ` 里 `xpixel/ypixel` 不再是 0；顺带修掉 `Client` 上一条真 bug——两处 goroutine
同时写 socket 会把帧头和载荷交错。

**方向一句话**：herdr-core 的五条机制里第 2 条（credit 流控 / 落后重绘 / RESIZE）在本仓落地；
它不改任何交互，只让「谁欠谁多少字节」变成协议里的事实。

**为什么这一片而不是 S1b**：S1b（把 `terminalui` 接到 `Dial`）会改**会话管理语义**
（`CloseAll` 从「退出即杀」变成「detach」），已写进 `.pending-bugs.md`「需用户拍板」，
按红线不擅自做。S2 里与交互无关的那一半（协议层）本轮做完。

## 一、落地项

| # | 位置 | 落成 |
|---|---|---|
| 1-1 | `session.go` 常量 | `clientWindow = 256 KiB`（起步额度）、`clientBacklog = 1 MiB`（丢多少算「真落后」）、`creditFlush = 4 KiB`（客户端回额度的批） |
| 1-2 | `session.go` 协议注释 + tag | 新增 `C`（客户端 → 宿主：我又能收 n 字节），并写明 `H/S/E` 是状态、不过额度 |
| 1-3 | `host.go` `conn` | `credit`（剩余额度）、`dropped`（上次重绘以来丢的字节）、`newConn`（队列 + 满额度） |
| 1-4 | `host.go` `conn.admit` | 输出先扣额度再入队；额度或队列任一不行 → 丢这一帧并累加 `dropped`；`dropped ≥ clientBacklog` → `behindLocked`（丢积压 + 一条 `L`）。状态帧不过额度，直接入队 |
| 1-5 | `host.go` `readLoop` | `tagCredit` → `conn.grant`（加额度，上限 `clientWindow`；负数/0 忽略） |
| 1-6 | `host.go` `writeLoop` | 重绘的快照与 `clearLag` 收进**同一把** session 锁：中间落地的广播若被当「落后」丢掉，它的字节已经过了快照，那才是真丢数据 |
| 1-7 | `host.go` `Client` | `wmu` 写锁 + `send()` 唯一出口；`owed`/`flushing`：消费 `creditFlush` 字节回一次额度，回填走 goroutine（不在终端的读取路径上阻塞） |
| 1-8 | `host.go` `encodeResize`/`decodeResize` | 8 字节（cols, rows, cellW, cellH）；4 字节旧载荷仍按「像素未知」解析 |
| 1-9 | `host.go` `Client.ResizePixels` | 真发像素（此前是 `return c.Resize(...)`） |
| 1-10 | `pty_unix.go` | `setWinSize(f, cols, rows, xpixel, ypixel)`；`PTY.ResizePixels`；`PTY.Resize` 传 0（未知） |
| 1-11 | `pty_other.go` | 同形 `ResizePixels`，返回 `ErrUnsupported`（**不假装支持**） |

**FOCUS 帧：故意不做。** 核过之后它已经是既有事实：`mygo@v0.7.3/plugins/terminal/view.go:290`
在 mode 1004 打开时 `t.in.push(vt.EncodeFocus(focused))`，焦点变化作为 `ESC[I`/`ESC[O`
走 `Conn.Write` → 本仓的 `I` 帧 → 程序的标准输入。再开一条帧就是同一个事实的第二条路。
（写进 `session.go` 的注释与 `.pending-bugs.md`，不再当待办。）

## 二、实施中改掉的一个设计（真机烟测逼出来的）

**第一版**是「额度耗尽 = 立刻重绘」。真机烟测（`seq 1 300000`，1.9 MB）：

```
收到的字节 18 863 585    SEQ-DONE 出现 55 次
```

原因：客户端**画屏幕的时候一刻没有额度**，于是屏幕期间每来一帧就再重绘一张屏幕——放大 10 倍。
herdr-core 的流控本来就是**两级**：额度管节奏，backlog 上限管「真的落后了」。照做，第二级取
**1 MiB**（环形缓冲 4 MiB 的四分之一，重绘一定能把丢掉的那些收回来）。同一发烟测：

```
收到的字节 2 202 414     SEQ-DONE 出现 2 次   最后一行 300000 在
```

这条写进了 `admit` 的注释和 `clientBacklog` 的注释，免得下一个读者把「立刻重绘」当成漏掉的优化。

## 三、行为契约

- `internal/terminalui`、`internal/shell`、`internal/settings*`、`internal/i18n`：**一行未动**。
  只有 `internal/termsession`（S1 新包，还没有任何表面接上）与它的测试。
- `__termhost` / `__termattach` 的参数不变；`internal/ci` 的偏移合约继续绿。
- `Info` 仍然只带格子数：像素是终端的，不进会话记录。

## 四、V/B/S 台账

| 类 | 结论 |
|---|---|
| **V** | 无（本轮没有视觉改动） |
| **B** | 2095+18 个既有测试函数里，**2 条**因机制改变重定基线，其余零改动全绿 |
| **S** | 1 条：`TestALaggingClientResyncsOnTheScreen` → **`TestAFullQueueDropsTheFrameInsteadOfGrowingTheBacklog`**。旧名守的是「队列满 = 立刻 `L`」，那正是本轮换掉的机制；新名守它真正守住的东西——**队列有界**（丢这一帧，不排队），并注明 `P13-b 重定基线`。重绘时机改由 `TestAStalledClientIsRedrawnOncePerBacklog` 守 |

新增 pin 6 条（`credit_test.go` 5 / `resize_unix_test.go` 1）：

1. `TestTheHostSendsOnlyWhatTheClientTook` —— 不读不回的客户端拿满一个窗口（262144 字节）就不再收到输出，且之后收到的是重绘通知。
2. `TestTheClientGivesBackWhatItTook` —— 终端消费 4 KiB，网络上出现一帧 `C 4096`。
3. `TestAStalledClientIsRedrawnOncePerBacklog` —— 额度耗尽只是节奏，丢满 1 MiB 才重绘，且只通知一次、之后不再收帧。
4. `TestAClientCannotWidenItsOwnWindow` —— 回填超过一个窗口会被夹回；负数被忽略。
5. `TestTheClientWritesOneFrameAtATime` —— 并发 `Write`/`ResizePixels` 时每一帧都能被解析（`yieldingConn` 在每次写之后让出调度，因为真 socket 缓冲满时就是这样；不靠运气）。
6. `TestTheResizeCarriesTheCellSize` —— `ResizePixels(100,30,9,18)` 之后 `TIOCGWINSZ` = row 30 col 100 **xpixel 900 ypixel 540**；格子数只有 4 字节的 `Resize` 把像素写成 0（未知）。

## 五、门槛（全部实测）

```
gofmt -l cmd internal            → 空
go vet ./...                     → 净
GOOS=windows go vet ./...        → 净
GOOS=darwin  go vet ./...        → 净
go test -count=1 ./...           → 0 FAIL
PASS 计数                        → 2859（基线 2853）
SKIP                             → 61（基线 61）
测试函数                         → 2119（基线 2113）
go test -race ./internal/termsession/ → ok（本轮新增并发路径单独跑过）
GOOS=linux/windows/darwin build  → 全过
```

## 六、mutation（5 条实跑，4 条变红，1 条如实记「没抓到」）

1. **admit 不扣额度**（`c.credit -= len(f.payload)` → `_ = f.payload`）
   → 红：`credit_test.go:75: the host sent 64 frames (4194304 bytes) and never resynced the client`
2. **Read 不回额度**（`c.owed += n` → `_ = n`）
   → 红：`credit_test.go:129: read pipe: i/o timeout`（网络上永远等不到 `C` 帧）
3. **尺寸不带像素**（`ResizePixels` 传 0,0）
   → 红：`resize_unix_test.go:37: the terminal's size = {row:30 col:100 xpixel:0 ypixel:0}, want {row:30 col:100 xpixel:900 ypixel:540}`
4. **Client 写不持锁**（`c.wmu.Lock()` 去掉）
   → 红：`credit_test.go:237: the host could not read the frames the client wrote: termsession: frame payload of 5 bytes belongs to another tag`
   （第一次跑这条 pin **挂住**而不是失败：读端读到坏帧就退出，写端永远堵在 `net.Pipe` 上。已把读端改成「坏帧之后继续排空」——pin 必须失败，不许挂住。）
5. **第二级门槛永不到点**（`c.dropped >= clientBacklog` → `false`）
   → 红：`credit_test.go:75: the host sent 4 frames (262144 bytes) and never resynced the client` 与
   `credit_test.go:175: the stalled client's queue holds 0 frames, want just the resync notice`
6. **（没抓到，如实记）** 去掉 `admit` 开头的 `if c.lagged { return }`
   → 仍绿。原因：`behindLocked` 已把 `credit` 清零，落后态下输出本来就入不了队，那条短路是**双保险**不是判据。留着，但不谎称它被 pin 住。

每条都 `cp` 还原后 `md5sum -c` 逐字节核对（`host.go` `447b803265073e6b7fe608d901e20286`、
`pty_unix.go` 同法），并 `touch` 后再跑，避免测试缓存。

## 七、真机验证（这一片的意义）

```
$ floter __termhost -program /bin/sh -socket /tmp/s2/s.sock -cols 120 -rows 40 -linger -1s &
$ floter __termattach -socket ... -type $'seq 1 300000; echo SEQ-DONE\r' -wait 8s
sh running seq=1 120x40 pid=2695962     收到 2 202 414 字节，SEQ-DONE 2 次，最后一行 300000 在
$ floter __termattach -socket ... -type $'echo REATTACH-OK\r' -wait 2s
sh running seq=1 120x40 pid=2695962     ← 同一个 pid，detach 之后程序还在
$ floter __termattach -socket ... -kill -wait 1s
（socket 已删，宿主已退）
```

## 八、仍待

1. **S1b** 不变，仍等拍板（`terminalui` 接 `Dial`，`CloseAll` → detach）。
2. **S1c（Windows ConPTY）**：`openPTY` 要 `CreatePseudoConsole` + 进程属性表。
3. **S3**：list / kill / 结束原因 / 崩溃后重连；`terminal-snapshot` 与 capture 通道迁移。
4. **S2 剩下的**：`FOCUS` 已判「故意不做」（理由见上）；`RESIZE` 的像素已做。
   协议层还剩「客户端断线时宿主如何得知」（现在靠 socket EOF）。
5. **视觉队列**：`docs/liquid-glass.md` §6.3 终端页——弧做完再回。
