# P11-y 规格：打开终端不再重放上一个会话的终端状态（用户实测 F1）

## 意图

用户实测反馈 F1「打开终端（terminal 视图）时出现异常字符 / 莫名被输入的内容」。
本轮**复现、定位到根因、按根因修**：终端页把上一个会话的 `Snapshot()`（一串 VT
转义序列）`Feed` 进**已经有活程序在跑的**新会话，于是新 shell 继承了死程序的
**程序侧状态**（字符集 G0、括号粘贴、光标闪烁、滚动区、制表位、OSC 7 工作目录、
Kitty 键盘标志）。这不是「多画了几笔」，是「把 A 程序的终端状态装到 B 程序身上」，
所以症状是**内容被改写**：ASCII 变成制表符、粘贴被包上 `^[[200~`。

为什么现在做：这是用户实测反馈里唯一一条**真 bug**，且优先级写在任务书最上面。

## 复现（headless，无需 GUI，实测输出）

探针：`internal/zzprobe/probe_test.go`（临时，不入库；跑法见报告）。两次会话都用
真 `terminal.New`（libghostty-vt）：

1. A：`/bin/sh -c "printf '\033[?2004h\033(0'; sleep 20"` —— 一个设了括号粘贴
   + G0 制表字符集的程序，也就是 `zsh`/`vim` 之类程序结束时会留下的状态。
2. `snap := A.Snapshot()` —— 96 字节，尾部实测：
   `\e[?12h\e[?2004h\e[3g\e[9G\eH…\e[H\e[1;1H\e[0m\e(0`
   （光标闪烁开 / 括号粘贴开 / 制表位 / 光标 / **`\e(0` 制表字符集**）
3. B：`/bin/sh -c "printf 'ready'; sleep 20"`，`B.Feed(snap)` —— 这正是
   `App.EnsureSession` 干的事。
4. B 里程序写纯 ASCII `qqq-wxyz` → 屏上实测 `───-┬│≤≥`（**制表符**）。
5. B 里 `B.Paste("hello")` → 程序收到并被回显 `^[[200·␤␊┌┌⎺^[[201·`
   （**括号粘贴的包装被当成正文**，因为 2004 是死会话留下的）。

两条症状都指向同一句：`Feed` 把「给一个没有程序的终端重新接上一个会话」的产物
喂给了「已经有程序的终端」。

## 现状（行号为动手前实测）

| 位置 | 现状 |
|---|---|
| `internal/terminalui/terminalui.go:145` | `SnapshotPath string` 字段 |
| `internal/terminalui/terminalui.go:339` | `EnsureSession` 末尾 `a.restoreSnapshot(session.Term)` |
| `internal/terminalui/terminalui.go:501-514` | `restoreSnapshot`：读文件 → `term.Feed(data)`（第 513 行） |
| `internal/terminalui/terminalui.go:485-499` | `SaveSnapshot`：`term.Snapshot()` → 落盘 |
| `internal/terminalui/terminalui.go:465-468` | `CloseSession` 关掉屏上会话前存快照 |
| `internal/terminalui/terminalui.go:517-519` | `writeFileAtomically`（只被 `SaveSnapshot` 用） |
| `internal/shell/shell.go:1122` | `a.Terminal.SnapshotPath = filepath.Join(paths.Root, "terminal-snapshot")` |
| `internal/shell/shell.go:672`、`:1392` | 退出路径上 `a.Terminal.SaveSnapshot()` |
| `internal/terminalui/view_test.go:311-355` | `TestSnapshotSaveAndRestore`：钉住「存→读回」这套 |

`Terminal.Snapshot()` 的语义在插件里写得很清楚
（`mygo@v0.7.0/plugins/terminal/internal/vt/terminal.go:565-590`）：导出
**屏幕 + 样式 + 超链接，然后模式、滚动区、制表位、工作目录、键盘标志、光标**，
用途是「Fed to a new terminal of the same size, it shows the same, **as a
terminal attaching again to a session a server keeps**」——即「终端连回一个
仍然活着的会话」。本仓的用法（连到一个**新的活 shell** 上）与它相反。

## 应改成

**删掉这套快照存/读。** 理由不是「怕出 bug」，而是这套东西**没有正确的读法**：

- 公开 API 只有 `Snapshot()`（全量，含程序侧状态）与 `Feed()`（当程序输出写进
  屏）。没有「只导出文本」的开关——那个 formatter 的 flags 在
  `plugins/terminal/internal/vt`（Go 的 internal 包，本仓不可导入）。
- 所以「只把文字贴回去」在本仓只能靠**过滤转义序列**实现，那正是任务书禁止的
  「加过滤吞掉」：它会漏（滚动区、OSC 7、Kitty 标志都不是「装饰」），而且漏了
  不报错。
- 参考实现**没有这个功能**：`git grep -n snapshot main -- src/terminal
  src-tauri/src/terminal` 只命中滚动区偏移，没有「上次屏幕」的存/读。这是重写
  期自己发明的行为，也正是用户报的症状来源。

具体落地：

1. `internal/terminalui/terminalui.go`：删 `SnapshotPath` 字段、`SaveSnapshot`、
   `restoreSnapshot`、`writeFileAtomically`、`CloseSession` 里的保存、`EnsureSession`
   里的 `a.restoreSnapshot(...)` 调用；随之删掉不再使用的 `os`/`log`/`statefile` 导入。
2. `internal/shell/shell.go`：删 `:1122` 的路径接线与 `:672`、`:1392` 两处
   `SaveSnapshot`（`CloseAll` 保留——那是进程回收，不是快照）。
3. 测试：`view_test.go` 的 `TestSnapshotSaveAndRestore` 是**结构变了**（S）：删掉，
   换成 `internal/ci` 的源码扫描 pin
   `TestTheTerminalPageReplaysNoDeadSessionState`——`internal/shell` 与
   `internal/terminalui` 的生产行里不得再出现 `SnapshotPath` / `SaveSnapshot` /
   `restoreSnapshot` / `terminal-snapshot` / `.Feed(`。这条 pin 是「修前红、修后绿」
   的回归断言：把任何一处加回去就红。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理（`maxSessions` / trim / 切换 / `CloseAll`）、设置写回、
  i18n 文案、命令接线：**零改动**。
- 唯一的行为差异：进入终端页不再看到上一次的屏幕（回放本来就是错的）。
  想恢复「上次的滚动缓冲」需要插件提供**只含文本**的导出（mygo 侧特性）——
  已作为**拍板项**记进 `.pending-bugs.md`，本轮不擅自做替代方案。

## 门槛命令与 PASS 基线（本轮实测起点）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 起点 2343
三平台 build                     # 过
```

起点：PASS **2343** / SKIP 15 / FAIL 0 / 测试函数 **1902**（本轮实测，非旧报告数字）。
本轮删 1 个测试函数、加 1 个（ci pin），函数数应为 1902；PASS 计数应 ≥ 2343。

## mutation 计划

1. 在 `EnsureSession` 里加回一行 `session.Term.Feed([]byte("\x1b(0"))`
   → `TestTheTerminalPageReplaysNoDeadSessionState` 红（`.Feed(` 命中）。
2. 在 `shell.go` 加回 `a.Terminal.SnapshotPath = "x"`
   → 同一条 pin 红（`SnapshotPath` 命中）。
3. 把 `restoreSnapshot` 函数名换回 `restoreScreen` 并加回调用 → pin 仍应红
   （`.Feed(` 命中，证明 pin 抓的是行为不是名字）。
