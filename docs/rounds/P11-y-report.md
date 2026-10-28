# P11-y 报告：打开终端不再重放上一个会话的终端状态（用户实测 F1）

> 用户实测反馈 F1（最高优先级）：「打开终端（terminal 视图）时出现异常字符 / 莫名被
> 输入的内容」。本轮 headless 复现到字节级、定位根因、按根因修。**非视觉轮，不出图。**
> 规格 `docs/rounds/P11-y-spec.md`。

## ① 根因（一句话）

终端页把上一个会话的 `Terminal.Snapshot()`（插件的 VT 导出）`Feed` 进一个**刚刚起了新
shell 的**终端；而那份导出的读者是「a terminal attaching again to a session a server
keeps」——一个**没有程序**的终端。于是新 shell 继承了死程序的**程序侧状态**：字符集、
括号粘贴、光标闪烁、滚动区、制表位、OSC 7 工作目录、Kitty 键盘标志。其中两条是**内容**
被改写，不是装饰多画一笔。

`mygo@v0.7.0/plugins/terminal/internal/vt/terminal.go:565-590`（`VT()`）的文档原文：

> The scrollback and the screen with their styles and hyperlinks, then the
> modes, the scrolling region, the tab stops, the working directory, the
> keyboard's flags and the cursor. […] Fed to a new terminal of the same size,
> it shows the same, as a terminal attaching again to a session a server keeps.

## ② 复现（headless，无需 GUI，实测字节）

临时探针（已删，源码见 §③）用两次**真** `terminal.New`（libghostty-vt）：

```go
// 1. A: 一个留下「制表字符集 + 括号粘贴」的程序（vim/ls --color 的残留状态）
a, _ := term.New(term.Options{Command: []string{"/bin/sh", "-c",
    `printf '\033[?2004h\033(0'; sleep 20`}, Dir: t.TempDir()})
snap := a.Snapshot()

// 2. B: 一个什么都没要求的活 shell
b, _ := term.New(term.Options{Command: []string{"/bin/sh", "-c",
    `printf 'ready'; sleep 20`}, Dir: t.TempDir()})
b.Feed(snap)                       // ← App.EnsureSession 当时干的就是这一行
b.Feed([]byte("\r\nqqq-wxyz\r\n")) // 程序写纯 ASCII
b.Paste("hello")
```

实测输出（`go test -count=1 -v -run TestProbeSnapshotReplay ./internal/zzprobe/`）：

```
A snapshot (96 bytes) tail:
  \e[?12h\e[?2004h\e[3g\e[9G\eH\e[17G\eH…\e[H\e[1;1H\e[0m\e(0
B after replay:        "ready"
B after ASCII write:   "ready\n───-┬│≤≥"
B after paste:         "ready\n───-┬│≤≥\n^[[200·␤␊┌┌⎺^[[201·"
```

两条症状都对上了用户描述：

1. **异常字符**：重放的 `\e(0`（G0 = DEC 制表字符集）把新 shell 的纯 ASCII `qqq-wxyz`
   画成 `───-┬│≤≥`。这就是任务书里猜的「把 box-drawing 当内容输出」——不是画错了，
   是**字符集被死会话改掉了**。
2. **莫名被输入的内容**：重放的 `\e[?2004h` 让终端以为括号粘贴开着，于是 `Paste("hello")`
   把程序从未要求的 `\e[200~`…`\e[201~` 包了进去，shell 把它当正文回显成
   `^[[200·␤␊┌┌⎺^[[201·`。

另外，重放还带着光标闪烁（`\e[?12h`）、制表位（`\e[3g` + 一串 `\e[NG\eH`）、滚动区、
OSC 7 工作目录与键盘标志，且回放与**新 shell 的第一帧输出**竞争——谁后写谁赢。

## ③ 改动（按根因删掉这套存/读，不是过滤）

**为什么不是「只把文字贴回去」**：公开 API 只有全量的 `Snapshot()` 与 `Feed()`；那个
formatter 的 flags（`modes`/`scrollingRegion`/`tabstops`/`pwd`/`keyboard`/`screen.charsets`）
在 `plugins/terminal/internal/vt`，Go 的 internal 包本仓不可导入。要「只留文字」只能自己
过滤转义序列——正是本项目禁止的「加过滤吞掉」：它漏滚动区、漏 OSC 7、漏键盘标志，而且
漏了不报错。参考实现**根本没有这个功能**（`git grep -n snapshot main -- src/terminal
src-tauri/src/terminal` 只命中滚动区偏移），所以这不是「参考要的、我们做歪了」，是重写期
自己发明的一个没有正确读法的功能。

| 文件 | 改动 |
|---|---|
| `internal/terminalui/terminalui.go` | 删 `SnapshotPath` 字段、`SaveSnapshot`、`restoreSnapshot`、`writeFileAtomically`、`CloseSession` 里的保存、`EnsureSession` 里的 `a.restoreSnapshot(...)` 调用；删不再使用的 `os`/`log`/`statefile` 导入；原位置留一段说明（为什么删、为什么不能只贴文字、要恢复需要什么） |
| `internal/shell/shell.go` | 删 `a.Terminal.SnapshotPath = filepath.Join(paths.Root, "terminal-snapshot")`；删两条退出路径上的 `SaveSnapshot`（`CloseAll` 保留：那是进程回收，不是快照） |
| `internal/terminalui/view_test.go` | 删 `TestSnapshotSaveAndRestore`（它钉的正是被删的功能）；新增 `TestTheTerminalSurfaceReplaysNoSavedScreen` |
| `internal/terminalui/view_test.go`（新 pin） | 源码扫描 `internal/terminalui` 与 `internal/shell` 的**生产行**：`SnapshotPath` / `SaveSnapshot` / `restoreSnapshot` / `terminal-snapshot` / `.Feed(` 一律不许出现（注释行豁免）。不拿 `.Snapshot()` 当针，因为 `Store.Snapshot()` 是设置读取，到处都是 |

**回归断言怎么证明修前红**：见 §④ 两条 mutation——把任意一处加回去，pin 立刻红并打印
命中的文件:行。

## ④ 台账与变异

- **S = 1**（一套「存快照 / 读快照」结构整体删除，删 1 条测试函数 + 加 1 条扫描 pin）。
- **V = 0 / B = 0**：没有视觉数值断言被改，没有任何行为/度量/接线断言被放宽。
- 变异 **2 条实跑**（`md5sum -c` 逐字节还原 + `touch` 复跑绿）：

```
=== M1: 在终端表面加回一个 Feed() ===
--- FAIL: TestTheTerminalSurfaceReplaysNoSavedScreen (0.00s)
    view_test.go:353: terminalui.go:1537 replays a saved screen (".Feed("):
      func replayScreen(t *terminal.Terminal) { t.Feed(nil) }

=== M2: 在 shell 里加回 snapshot 路径字面量 ===
--- FAIL: TestTheTerminalSurfaceReplaysNoSavedScreen (0.01s)
    view_test.go:353: ../shell/shell.go:4926 replays a saved screen ("terminal-snapshot"):
      var _ = "terminal-snapshot"

=== 还原 ===
internal/terminalui/terminalui.go: 成功
internal/shell/shell.go: 成功
```

`touch` 两个文件后单跑该 pin → `ok floter/internal/terminalui 0.015s`（不吃测试缓存）。

## ⑤ 门槛（本轮实测，非旧报告数字）

```
gofmt -l cmd internal                              → 空
go vet ./...                                       → 净
GOOS=windows go vet ./...                          → 净
GOOS=darwin  go vet ./...                          → 净
go test -count=1 ./...                             → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2344（起点 2343）
三平台 build                                       → 过
```

SKIP 15、FAIL 0、测试函数 1902（删 1 加 1）。**唯一的行为差异**：进终端页不再看到上一次
的屏幕——那本来就是错的。想恢复「上次的滚动缓冲」需要插件提供只含文本的导出（mygo 侧
特性），已记为拍板项，本轮不做替代方案。

## ⑥ 仍待

1. **F2 + F3**（用户实测反馈第 2、3 条）下一轮做：聚焦卡边/终端画布的红 1px
   （`internal/theme/theme.go:648`、`:727` → `shell.go:4263`、`terminalui.go:875`）+
   终端画布那圈双层框（`terminalui.go:822` 面板 + `:888` `Border(1, edge)`）。
2. 拍板清单不变；**新增一条**：「上次会话的滚动缓冲要不要回来」（要回来＝请 mygo 提供
   只含文本的 `Snapshot` 变体）。
