# P13-c 报告：会话生命周期（S3 的第一半）——宿主留下记录，list 看得见、kill 够得着

规格 `docs/rounds/P13-c-spec.md`。基线 HEAD `ea24b24a`（2805 → 见下，实测 2859 PASS / 0 FAIL /
61 SKIP / 2119 测试函数）。

## 一、这轮做了什么

`StateLost` 在 `internal/termsession/session.go:43` 被 P13-a 声明过，全仓 **没有任何地方写过它**
（`grep -rn "StateLost" internal/ --include=*.go` 只有定义与它的文档）。这轮让它变成事实，并且
让「这台机器上有哪些会话、它们怎么了」不再需要 attach 去猜。

| # | 位置 | 改动 |
|---|---|---|
| 1-1 | 新 `internal/termsession/registry.go`（213 行） | `Record`（`Info` + socket + hostPid + clients + started）、`RecordPath`/`DefaultDir`/`WriteRecord`/`ReadRecord`/`RemoveRecord`/`ListRecords`/`LookupRecord`、宿主的 `writeRecord` |
| 1-2 | 新 `alive_unix.go` / `alive_other.go` | `Alive(pid)`：`Signal(0)`，EPERM 算活（别人的进程也是进程）；Windows 老实返回 false（那边还没有 PTY） |
| 1-3 | `host.go` `Start` | 起完 PTY **先落一份记录**再 `go h.read()`——记录早于任何人 attach |
| 1-4 | `host.go` `watch` / `readLoop` resize 臂 / `attach` 进出 | 状态变化、尺寸、客户端数各落一次 |
| 1-5 | `host.go` `Stop` | 记录与 socket 一起删，而且**删在 `wg.Wait()` 之后** |
| 1-6 | `host.go` `DefaultSocket` | 改走 `DefaultDir()`，socket 与记录同目录 |
| 1-7 | `command.go` | `__termlist [-dir] [-json]`、`__termkill -id\|-socket [-dir]`；并把**四条子命令收成一张表**（`SessionCommandNames` + `SessionCommands` + `IsSessionCommand`） |
| 1-8 | `cmd/floter/main.go` | 从「问两次名字」变成「问一张表」：`IsSessionCommand` → `SessionCommands[which](rest)` |
| 1-9 | `internal/ci/termsession_contract_test.go` | 合约从「`IsCommand` 在锁之前」扩到「**整个家族**在锁之前」，并钉住入口真的走那张表 |

**它为什么更「轻」**：不是视觉轮，这里说的是**负载**。会话状态从「读者必须挂上去才知道」（Dial 会
改客户端计数、拿屏、可能触发重绘）变成「读一个文件」。`__termlist` 对一个活会话**零连接**。

### 一条真 bug（实现中途自己撞出来的）

第一版把 `RemoveRecord` 写在 `Stop` 的 `wg.Wait()` **之前**。真机没暴露，是 pin 抓的
（`TestTheHostTakesItsRecordWithItWhenItStops`：`the record survived the host that wrote it: <nil>`）：
`Stop` 关 socket → 客户端的 defer 落一次「客户端数」→ **把刚删掉的记录写回来了**。
于是 `writeRecord` 加了 `closed` 短路（宿主已停就不写），`Stop` 里 `RemoveRecord` 挪到最后。
这条是这一轮唯一一处「两个 goroutine 对同一份事实的写」的正面冲突，也是这笔记录模型的真实代价。

## 二、V / B / S 台账

- **V 0**：不动任何视觉数值，`docs/liquid-glass.md` 一个字节没改。
- **B 2**（既有 pin 因**机制**换了而重定基线，都在我改动的文件里，注释写明旧行为 → 新行为）：
  1. `internal/ci/termsession_contract_test.go`：`TestTheSessionHostIsASubcommandBeforeTheSingleInstanceLock`
     断言的对象从 `termsession.IsCommand(` 变成 `termsession.IsSessionCommand(`——旧的那一条只钉
     一个名字，四条子命令里三条能绕过它。**这是收紧，不是放宽**：现在钉的是整张表 + 表与入口同源。
  2. `internal/termsession/host_test.go` 的 `TestTheRecordSurvivesTheHostSoASessionCanBeListed`
     改成**不 attach 就建宿主**（原来的 `startTestHost` 会先挂一个客户端，于是「记录里客户端数 = 0」
     这条新断言测不到）。
- **S 0**：没有改任何既有结构；`registry.go` 是新增文件，`command.go` 的两条旧函数体原样保留
  （`IsCommand`/`IsAttachCommand` 仍是 `splitCommand` 的薄封装），`SessionCommandNames` 是加法。
- 其余 **2127 条既有测试**（B）**零改动**全绿。

## 三、门槛实测（HEAD `ea24b24a` 基线 → 现在）

| 项 | 基线 | 现在 |
|---|---|---|
| `gofmt -l cmd internal` | 空 | **空** |
| `go vet ./...` | 净 | **净** |
| `GOOS=windows go vet ./...` | 净 | **净** |
| `GOOS=darwin go vet ./...` | 净 | **净** |
| `go test -count=1 ./...` | 0 FAIL | **0 FAIL** |
| `--- PASS` 计数 | 2859 | **2869**（+10） |
| 测试函数 | 2119 | **2129**（+10） |
| `--- SKIP` | 61 | **61**（没加新 skip） |
| 三平台 `go build` | 过 | **过** |
| `-race`（`./internal/termsession/`） | — | **过**（5.4s） |

## 四、mutation（5 条实跑，全部变红，逐条 `md5sum -c` 还原）

1. **`ListRecords` 不看宿主 pid**（`if !Alive(rec.HostPid)` → `if false`）
   → 红：`registry_test.go:137: the session's state = "running", want "lost": its host is gone`
   （连带 `TestTheKillForgetsASessionWhoseHostIsGone` 也红，因为 kill 走了「连不上」那条臂）
2. **`Start` 不写第一份记录**（删掉 `h.writeRecord()`）
   → 红：`host_test.go:296: the list holds 0 sessions, want the host's one record`
3. **detach 不落客户端数**（删 defer 里的 `h.writeRecord()`）
   → 红：`host_test.go:366: clients after a detach = 2, want 1`
4. **kill 丢了不收尸**（`RemoveRecord`/`os.Remove` 去掉、返回 1）
   → 红：`registry_test.go:205: the command = 1, want 0 (output "dead-1 lost\n")`
5. **（合约）入口分流挪到单实例锁之后**
   → 红：`termsession_contract_test.go:48: the session subcommands are dispatched at offset 1281,
   after the single-instance lock at 1215: a running app would refuse a host or a list and take their arguments`

每条都 `cp` 还原 + `md5sum -c /tmp/p13c.md5` 逐字节核对（`registry.go`
`1a1eaad72bad8c85938c1c26a3efb85a`、`host.go` `fd7e3ca2f86028088bffcb7bd2f91bdc`、`command.go`
`efb2a03a37012d3010a1dd818ceef1ff`），并 `touch internal/termsession/*.go` 后再跑，避免测试缓存。

## 五、真机验证（S3 这一半的意义）

```
$ /tmp/floter-p13c __termhost -program /bin/sh -socket /tmp/p13c/s1.sock -linger -1s &
$ /tmp/floter-p13c __termlist -dir /tmp/p13c
sh-2716410  running  /tmp/p13c/s1.sock  0 clients  pid 2716402
$ /tmp/floter-p13c __termattach -socket /tmp/p13c/s1.sock -type $'sleep 400 &\r' -wait 1s
sh running seq=1 80x24 pid=2716410
[?2004hsh-5.3$ sleep 400 &
[1] 2716434
$ kill -9 2716402                     # 宿主被杀（等价于崩溃）
$ /tmp/floter-p13c __termlist -dir /tmp/p13c
sh-2716410  lost  /tmp/p13c/s1.sock  0 clients  pid 2716402      ← 记录还在、状态变 lost
$ /tmp/floter-p13c __termkill -id sh-2716410 -dir /tmp/p13c
sh-2716410 lost                        ← 收尸，不是报错
$ /tmp/floter-p13c __termlist -dir /tmp/p13c
（空）
$ ps -p 2716410  → 没了（宿主的死带走了程序，PTY master 关闭）
```

活的那条路也验了：`__termkill -id <活会话>` 用 **0.020s** 返回 `killed`，宿主自己删记录与 socket，
`ls /tmp/p13c2` 空、`pgrep -fl termhost` 不再有它。**真机与 pin 说的是同一件事**：宿主活着 = 记录在、
状态是它自报的；宿主不在 = lost，且收尸是显式的一条命令。

## 六、仍待

1. **S1b（仍等拍板）**：`terminalui` 接 `Dial`；`CloseAll` 从「退出即杀」变 detach。**不改交互做不了**，
   按纪律写进 `.pending-bugs.md` 的「需用户拍板」。
2. **S1c（Windows ConPTY）**：`openPTY` 要 `CreatePseudoConsole` + 进程属性表。
   注意 `Alive` 在 Windows 现在恒 false——ConPTY 落地时它要一起改（`OpenProcess` + `GetExitCodeProcess`）。
3. **客户端不告而别时宿主怎么知道**（P13-b 遗留）：现在只有 socket EOF；`Clients` 的计数因此可能
   在「客户端进程被杀」后短暂偏大，直到 linger 窗口把宿主收掉。真正的解法是心跳/心跳超时，属 S2 尾。
4. **capture 通道迁移**（`extensions.RunCaptured`：1 MiB/流截断、300s 超时）：接进 termsession 会把
   扩展后台运行从「有界捕获」变成「活会话」，是**交互改动**，写进 `.pending-bugs.md` 的「需用户拍板」。
5. **`terminal-snapshot`：明确写「已被取代」**，不是待迁移——`P11-y` 已把写快照的链整个删掉，
   `grep -rn "terminal-snapshot" --include=*.go` 只剩 `internal/terminalui/view_test.go:344` 那条
   **反面 pin**（守「不许回灌」）。本轮没动它，也不该动。
6. **视觉队列**：`docs/liquid-glass.md` §6.3 终端页——按用户指令，弧做完再回。
