# P13-b 规格：会话协议的第二片——额度流控（按字节）与带像素的尺寸

## 意图一句话

把宿主对客户端的「输出」从**帧数**上限换成**字节额度**（客户端消费多少、回多少额度），
并让 `RESIZE` 带上格子的像素尺寸——这是 herdr-core 五条机制里第 2 条的完整落地。

## 为什么现在做（证据）

- 任务书（2026-10-30 晚）分片顺序：S1 宿主 → **S2 协议** → S3 生命周期 → S4 OSC 7501。
  S1 已由 P13-a 落地；S1b（把 `terminalui` 接到 `Dial`）要改**会话管理语义**，
  已写进 `.pending-bugs.md`「需用户拍板」，**不能擅自做**（行为契约零改动红线）。
  于是本轮做 S2 里与交互无关、纯协议的那一半。
- 现状的流控是**帧数**：`internal/termsession/session.go:185-188` `clientQueue = 256` 帧。
  一帧的上限是 `frameSize = 1 MiB`（`session.go:88`），所以一个不读的客户端最多能被堆
  256 帧 ≈ **8 MiB 的旧输出**（宿主按 32 KiB 读，实测常见 32 KiB/帧 → 8 MiB），
  比宿主自己的环形缓冲（4 MiB）还大——「有界」只是帧数上有界，字节上没有。
- 更关键的是宿主**不知道客户端消费到了哪**：写出去就当成送达。
  herdr-core 的做法是客户端**消费多少回多少额度**，宿主按额度发；额度耗尽就对端重绘，
  绝不无限堆 backlog。本仓现在没有「回额度」这条路。
- `ResizePixels` 是**退化实现**：`host.go` 里 `func (c *Client) ResizePixels(...)` 直接
  `return c.Resize(cols, rows)`。PTY 的 `TIOCSWINSZ` 里 `xpixel/ypixel` 也恒为 0
  （`pty_unix.go` `setWinSize`）。程序读 `TIOCGWINSZ` 的像素尺寸会拿到 0，指针报告
  （模式 1016）也就没有依据。
- 顺带一个真 bug：`Client` 的 `Write`/`Resize`/`Close` 会从**不同 goroutine** 落到
  同一个 socket 上（插件的 `tellConn` 就是在 goroutine 里调 `Resize`，
  `mygo@v0.7.3/plugins/terminal/terminal.go:1201-1240`），而 `writeFrame` 是**两次**
  `Write`（5 字节头 + 载荷）。两条并发的 `writeFrame` 可以交错成 `头/头/载荷/载荷`，
  那一帧从此解不出来。`Client` 现在没有写锁。

## 落地项（位置 → 现状 → 应改成）

### 1. 额度流控（按字节）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `session.go` 常量 | 只有 `clientQueue = 256` 帧 | 新增 `clientWindow = 256 << 10`（起步额度，对齐 herdr-core 的 256 KiB）与 `creditFlush = 4 << 10`（客户端回额度的批大小） |
| `host.go` `conn` | `mu`、`lagged` | 加 `credit int`：客户端**还没用完**的额度（字节），挂在 `c.mu` 下 |
| `host.go` `Session.broadcast` | 队列满（帧数）→ 丢积压 + 一条 `L` | 拆成 `conn.admit`：`tagOutput` 先扣额度，扣不动 = 落后 → `conn.behind`；帧数满仍走同一条 `behind`（两级保险） |
| `host.go` `readLoop` | `I/R/D/K` 四种 | 加 `tagCredit`：`C <4 字节 n>` = 「我又能收 n 字节」 |
| `host.go` `writeLoop` | 发完重绘屏后清 `lagged` | 不变（额度不清零：屏幕在额度之外，重绘后由客户端按消费回填，上限仍是 `clientWindow`） |
| `session.go` 协议注释 | 只有 `H S O E L` / `I R D K` | 补 `C` 的方向与语义 |

**为什么额度只算 `O`**：`H`（记录）、`E`（结束）、`S`（屏幕/重绘）是**状态**，不是增量——
按 herdr-core 的说法，落后时就是「直接重绘当前屏」，屏幕必须能发出去，不能等额度。
额度只约束「增量输出」，那才是会无限长的那条流。

**边界**（必须写进代码注释）：额度耗尽 = 立即 `behind`（丢积压 + 一条 `L`），
宿主**不另设 backlog 缓冲**。所以本仓的「落后上限」就是额度本身（256 KiB），
没有 herdr-core 的第二层 4 MiB 队列——一层就够，而且严格有界。

### 2. 客户端回额度

| 位置 | 现状 | 应改成 |
|---|---|---|
| `host.go` `Client` | `c`、`mu`、`pending`、`closed` | 加 `wmu sync.Mutex`（写锁，见 §3）与 `owed int`（攒着没回的额度） |
| `host.go` `Client.Read` | 从 `pending` 抄出 n 字节就返回 | 抄出后 `owed += n`；`owed >= creditFlush` 时发 `C owed` 并把 `owed` 清零 |
| `host.go` `Client.Close` | 发 `D` 再关 | 不变（额度不用回，宿主随连接一起丢） |

**为什么批量回**：一次 `Read` 回一帧 `C` 是每条读取多 5 字节，而 `Read` 由终端的绘制节奏驱动，
可能很小。4 KiB 一批是「池子有 256 KiB、批 4 KiB」——攒不满一批时客户端手里一定还有额度，
不会死等（这条要写进注释，否则下一个读者会以为丢尾部额度会卡住）。

### 3. 写锁（顺带修的真 bug）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `host.go` `Client` 的 `Write`/`Resize`/`ResizePixels`/`Close`/`Kill` | 各自直接 `writeFrame(c.c, …)` | 全部走 `c.send(tag, payload)`，`c.wmu` 下一帧一帧地写 |

理由见「为什么现在做」最后一条：插件的 `tellConn` 在 goroutine 里 `Resize`，
用户键入在另一条路上 `Write`，两条并发的 `writeFrame` 会在 socket 上交错。

### 4. 带像素的尺寸

| 位置 | 现状 | 应改成 |
|---|---|---|
| `host.go` `encodeSize`/`decodeSize` | 4 字节：`cols,rows` | `encodeResize(cols,rows,cellW,cellH)` = **8 字节**（四个大端 uint16）；`decodeResize` 解析 8 字节，**4 字节旧载荷仍按 0 像素读**（同一二进制里没有旧对端，但解析器不该因为短载荷崩） |
| `pty_unix.go` `setWinSize` | `ws.xpixel, ws.ypixel` 恒 0 | 收 `cols, rows, xpix, ypix`；`startPTY` 传 0，`Resize` 传 0，新增 `ResizePixels` 传 `cols*cellW, rows*cellH` |
| `pty_other.go` | `Resize` 返回 `ErrUnsupported` | 加同形的 `ResizePixels`，同样 `ErrUnsupported` |
| `host.go` `readLoop` 的 `tagResize` | `decodeSize` → `pty.Resize` | `decodeResize` → 有像素就走 `pty.ResizePixels`，没有就走 `pty.Resize` |
| `host.go` `Client.ResizePixels` | 退化成 `Resize` | 真的发 8 字节 |
| `host.go` `Client.Resize` | 4 字节 | 4 字节保留（像素未知），`readLoop` 按短载荷读 |

**不改**：`Info.Cols/Rows` 仍是格子数（记录是对外契约，像素不进记录）。

### 5. FOCUS 帧：故意不做（附理由）

任务书 S2 列了 `FOCUS`。核过之后**不加帧**：终端的焦点变化**已经**作为输入字节到达宿主——
`mygo@v0.7.3/plugins/terminal/view.go:290` 在 `mode 1004` 打开时 `t.in.push(vt.EncodeFocus(focused))`，
也就是 `ESC[I` / `ESC[O` 走 `Conn.Write` → 本仓的 `I` 帧 → 程序的标准输入。
再开一条 `FOCUS` 帧就是**同一个事实的第二条路**，而且会让「程序从哪里读到焦点」不再唯一。
这一条写成结论，不再当成待办。

## 行为契约零改动红线

- **键盘模型 / 高度链 / 会话管理 / 设置写回 / i18n / 命令接线**：本轮**一行不改**。
  `internal/terminalui`、`internal/shell`、`internal/settings*` 全不动；只有
  `internal/termsession`（S1 新包，还没有任何表面接上它）与它的 `_test.go`。
- `__termhost` / `__termattach` 两个子命令的**参数**不变（`internal/ci` 的偏移合约继续绿）。
- 旧载荷兼容：4 字节 `R` 仍能解析。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal            → 空
go vet ./...                     → 净
GOOS=windows go vet ./...        → 净
GOOS=darwin  go vet ./...        → 净
go test -count=1 ./...           → 0 FAIL
grep -c -- "--- PASS"            → 2853（本轮开工前实测）
测试函数（grep -rn "^func Test"） → 2113（同上）
GOOS=linux/windows/darwin build  → 全过
```

## mutation 计划（≥2 条，说明每条让哪个 pin 变红）

1. `conn.admit` 里不扣额度（`c.credit -= len(payload)` 删掉）
   → `TestTheHostSendsOnlyWhatTheClientTook` 红：不读的客户端会拿到超过窗口的输出。
2. 客户端 `Read` 不回额度（`c.owed += n` 删掉）
   → `TestTheClientGivesBackWhatItTook` 红：网络上不会出现 `C` 帧。
3. `setWinSize` 的 `xpixel/ypixel` 恒 0（`ResizePixels` 传 0）
   → `TestTheResizeCarriesTheCellSize` 红：`TIOCGWINSZ` 读回来是 0。
4. `Client.send` 不用 `wmu`（回到各自 `writeFrame`）
   → `TestTheClientWritesOneFrameAtATime` 红：并发写把帧头搅在一起。
5. 第二级门槛永不到点（`c.dropped >= clientBacklog` 恒假）
   → `TestAStalledClientIsRedrawnOncePerBacklog` 红：真落后的客户端永远等不到重绘。
