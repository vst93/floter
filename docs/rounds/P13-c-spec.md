# P13-c 规格：会话生命周期（S3 的第一半）——宿主留下记录，list 看得见、kill 够得着

**意图一句话**：给每个会话在 socket 旁边留一份**记录**（状态 / 退出码 / pid / 已连接的客户端数 / 起始时刻），
于是「这台机器上有哪些会话、它们怎么了、我能把它杀掉吗」不需要靠 attach 去猜——这正是 herdr-core
清单里第 5 条（记录 + 事件模型，而不是轮询状态）在本仓的落法。

**为什么现在做**：用户在 2026-10-30 晚定的分片顺序是 S1（宿主）→ S2（协议）→ **S3（生命周期）**，
S1/S2 已在 `1bb0efd4` / `ea24b24a` 落地。S3 里**不需要改交互**的那一半就是这一轮：

- `internal/termsession/session.go:43` 定义了 `StateLost`（「宿主没了，程序跟它一起没了」），
  **全仓没有任何地方写过它**（`grep -n "StateLost" internal/ --include=*.go` 只有定义与文档）。
  一条定义了却永远不出现的状态，就是这一轮要变成事实的东西：进程一旦消失，会话不是「还在」，
  而是**丢失**，而且这件事必须能从一个不 attach 的读者眼里看出来。
- 现在没有任何办法列出会话：`DefaultSocket()`（`host.go:748`）按宿主 pid 命名，读者只能
  `ls` 缓存目录再一个个 `Dial`——而 `Dial` 会真的挂上去（改客户端计数、拿屏、可能触发重绘）。
- `.pending-bugs.md` P13-a「仍待」第 3 条自己写着「协议层剩下的是客户端不告而别时宿主怎么知道」。

S3 的另一半（`terminal-snapshot` 与 capture 通道迁移）在下面「不做的」里说清楚。

## 落地项（位置 → 现状 → 应改成）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1-1 | 新文件 `internal/termsession/registry.go` | 不存在 | `Record`（`Info` + socket + 宿主 pid + clients + started）、`WriteRecord`/`ReadRecord`/`RemoveRecord`/`ListRecords`/`LookupRecord`/`DefaultDir`/`Alive` |
| 1-2 | `host.go` `Start` | 只建 socket 与 PTY | 建完写第一份记录（`h.writeRecord()`） |
| 1-3 | `host.go` `watch` / `readLoop`（resize 臂）/ `attach` | 改了内存里的 `Info` 就完了 | 每次改完**落一次盘**：状态变化（exited/killed）、尺寸、客户端数升降 |
| 1-4 | `host.go` `Stop` | 删 socket | 连记录一起删（`RemoveRecord`） |
| 1-5 | `host.go` `DefaultSocket` | 自算缓存目录 | 走新的 `DefaultDir()`，socket 与记录同目录 |
| 1-6 | `command.go` | `__termhost` / `__termattach` | `__termlist [-json] [-dir]`、`__termkill -id|-socket [-dir]` |
| 1-7 | `cmd/floter/main.go` | 两条子命令在锁前分流 | 四条都在锁前分流 |
| 1-8 | `internal/ci/termsession_contract_test.go` | 只钉 `__termhost` 一个名字 | 钉**每一条**会话子命令都在 `RequestSingleInstanceLock()` 之前 |

**清零 `StateLost`**：`ListRecords` 读到「宿主 pid 已经不存在」的记录时，把返回值的 `State` 改成
`StateLost`，**不改文件**——记录是别人写的，读者不该替它下葬。收尸是 `__termkill` 的活：
连不上的会话 → 报 `lost` → 删记录与 socket。理由（也是这一轮的取舍）：宿主的死就是程序的死
（PTY master 随之关闭，前台进程组收到 SIGHUP），所以「宿主不在」=「会话不可达」= 丢失，
而**绝不重跑原命令**（herdr-core 第 3 条的 keep 语义）。

## 不做的（本轮明确，写清理由）

- **`terminal-snapshot`**：`P11-y` 已经把写快照的那条链整个删掉（`grep -rn "terminal-snapshot" --include=*.go`
  只剩 `internal/terminalui/view_test.go:344` 那条**反面 pin**）。它现在不是「待迁移」，是**已被取代**：
  会话活在宿主进程里，重开 app 是 reattach，没有屏幕要回灌。本轮不动那条 pin。
- **capture 通道**（`extensions.RunCaptured`，1 MiB/流截断 + 300s 超时）：把它接到 termsession 会改变
  扩展后台运行的**语义**（有界捕获 → 活会话），那是交互改动，写进 `.pending-bugs.md` 的「需用户拍板」。
- **S1b**（`terminalui` 接 `Dial`、`CloseAll` 从「退出即杀」变 detach）仍等拍板，不动。
- **`FOCUS` 帧**：`P13-b` 已判「故意不做」，不翻案。

## 红线（零改动）

高度链、键盘模型、会话管理（`maxSessions`/`trimSessions`/`CloseSession` 语义）、设置写回、i18n 文案、
命令接线、`docs/liquid-glass.md` 的全部视觉 token。本轮不碰 UI，只加记录与两条子命令。

## 门槛（基线用实测当前值：HEAD `ea24b24a`，2859 PASS / 0 FAIL / 61 SKIP / 2119 测试函数）

```
gofmt -l cmd internal                     # 空
go vet ./... / GOOS=windows / GOOS=darwin # 净
go test -count=1 ./...                    # 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 只许涨
GOOS=linux/windows/darwin go build ./...  # 过
```

## mutation 计划（每条让哪个 pin 变红）

1. `ListRecords` 不再看宿主 pid（`if !Alive(rec.HostPid)` 去掉）→ `TestAListReportsAHostThatIsGoneAsLost` 红。
2. `Start` 不写第一份记录（删掉 `h.writeRecord()`）→ `TestTheRecordSurvivesTheHostSoASessionCanBeListed` 红。
3. `attach` 的 defer 里不落盘客户端数（删 `h.writeRecord()`）→ `TestTheRecordTracksTheClientsAndTheState` 红。
4. `__termkill` 连不上时直接返回 1 而不收尸（`RemoveRecord`/`os.Remove` 去掉）
   → `TestTheKillForgetsASessionWhoseHostIsGone` 红。
