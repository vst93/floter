# P11-cq 规格：外壳这一帧也只拷一份设置——会话行不许按行数拷

## 意图（一句话）

`P11-co` 给 `launcher` / `terminalui` / `settingsui` 三张表面都装了「一帧一份设置」的
memo，**唯独漏了外壳（`internal/shell`）自己**：`shell.View` 每帧仍直接 `Store.Snapshot()`
三次，而它的 `Settings.Sessions` 闭包**每一条会话拷两份**——16 条会话 = 32 份整份深拷贝。
本轮把外壳的绘制期读点也接到同一把尺子上。

## 为什么现在做（证据，本轮实测，不是抄旧报告）

选题顺序：用户实测反馈 F1–F4 已收尾 → 合约 §6 全 `已落地`（只剩拍板项）→
P11-cp「仍待」第 1 条整张是 15 条拍板项、第 2 条是死代码（单删无害但也不是本族）、
第 3 条是两份拷贝的记账、第 4 条要求「先量再领」、第 5 条是尺子盲点 →
按用户定的「界面质感与交互（含用户报的 bug）→ 基础功能」自选，挑的是**帧里的真开销**
（F4 原话里的「流畅性」），并且它是 `P11-co` 那一族**唯一没铺到的表面**。

本轮用探针 + 插桩实测（`runtime.Caller(1)` 打点，跑完立刻 `cp` 还原并 `md5sum` 核对）：

```
n=0   会话：sessions 页一帧 4 份      闭包本身 0 份
n=5   会话：sessions 页一帧 14 份     闭包本身 10 份
n=10  会话：sessions 页一帧 24 份     闭包本身 20 份
n=16  会话：sessions 页一帧 36 份     闭包本身 32 份   ← 上限 maxSessions = 16
```

闭包本身 = **每条会话 2 份**，与 `Title` 有没有设置无关地各拷一次
（`terminalui.go:416` 的 `LabelOf`、`:427` 的 `NoteOf` 各 `Store.Snapshot()` 一次；
两条 `return` 分支都拷，所以「有标题」和「没标题」都躲不过）。

插桩计数（2 条会话，四种表面各一帧，`uniq -c` 排序）：

| 读点 | 每轮触发次数 | 何时 |
|---|---|---|
| `terminalui/terminalui.go:427`（`NoteOf`） | 80 | 绘制期，每会话每帧 |
| `terminalui/terminalui.go:416`（`LabelOf`） | 80 | 绘制期，每会话每帧 |
| `shell/shell.go:4234`（`View` 开头的 `s`） | 49 | 绘制期，每帧 |
| `shell/notices.go:332` | 49 | 绘制期，每帧 |
| `shell/notices.go:331` | 49 | 绘制期，每帧 |
| `shell/shell.go:4197`（`appShortcuts` 的 `⌘W` 分支） | 43 | 绘制期，每帧 |
| `settings/snapshotmemo.go:43`（memo 的 miss） | 3 | 每版本一次（**正常**） |

即外壳自己一帧 **3 份**（`:4234` / `notices.go:331` / `notices.go:332`），
外加 `appShortcuts` 那 1 份（`settings` 表面每帧走到、终端/启动器表面不走，见下表）。

帧成本（`go test -bench`，720×580，sessions 页）：

```
BenchmarkSessionsPage16   350047 ns/op   13096 B/op   238 allocs/op
BenchmarkSessionsPage0     92503 ns/op    3416 B/op   100 allocs/op
```

**16 条会话让一帧慢 3.8×、对象多 2.4×**，而这 16 条正是 `maxSessions` 的上限
（`terminalui.go:286`）——也就是说这是这张表面**最坏情况下的常态**，不是边角。

## 逐条落地项（动手前已按当前 HEAD 读代码核对行号）

### 1. `terminalui` 的两个「按索引问会话」的读取函数不再各拷一份

- 位置：`internal/terminalui/terminalui.go:407-441`（`LabelOf` / `NoteOf`）
- 现状：两个函数各自第一行 `i18n.For(a.Store.Snapshot().Language)`。外壳的
  `Settings.Sessions` 闭包（`internal/shell/shell.go:1042-1057`）对**每一条会话**都
  调这四个函数（`LabelOf` / `NoteOf` / `DirOf` / `SizeOf`），于是每会话 2 份深拷贝。
- 应改成：两个函数改读本表面已有的 `a.settings()`（`terminalui.go:1053`，
  `settingsMemo.Get(a.Store)`，P11-co 已经装好的那把尺子）。
  `DirOf` / `SizeOf` 不读设置，**不动**。
- 为什么安全：`Memo` 的键是「store 指针 + version」，`Update` 的成功与回滚都推版本
  （`settings/snapshotmemo.go` 的注释与 `store.go` 的 `version`），所以 memo 给的
  永远是当前值；这里只读 `Language`，一个整份值的子字段。
- 红线：**`NoteOf` 的措辞与 ` · exit N` 后缀一个字节不动**（`sessions_test.go` 的
  en/zh 两臂守着）。

### 2. 外壳 `View` 一帧只取一份设置

- 位置：`internal/shell/shell.go:4233-4236`（`s := a.Store.Snapshot()`）、
  `internal/shell/notices.go:331-332`（`theme.ForPreferences(a.Store.Snapshot(), …)`
  与 `i18n.For(a.Store.Snapshot().Language)`）
- 现状：三处各自深拷贝一次，同一个值。
- 应改成：`shell.App` 加一个 `settings.Memo` 字段（与另外三张表面的字段同名同形），
  加一个 `func (a *App) settings() settings.Settings`（与 `settingsui.go:5255` /
  `terminalui.go:1053` 同一形状）；`View` 开头改读 `a.settings()`，
  `toastViewport` 改读 `a.settings()`。
  `notices.go` 的 `theme.ForPreferences` 拿到的必须是同一份值——它读的字段
  （`Theme` / `GlassStep` / 外观相关）与 `View` 开头 `launcher.Resolve` 读的是同一份。
- 红线：`theme.ForPreferences` 的入参是 `settings.Settings`，值不变即像素不变。

### 3. `appShortcuts` 读本帧那一份，不读磁盘上的那一份

- 位置：`internal/shell/shell.go:4163-4200`（三处 `settings.Shortcut(a.Store.Snapshot(), …)`：
  `:4178` 终端的新命令、`:4189` 终端的外部终端、`:4197` 设置的新命令）
- 现状：每次按键判定都深拷贝一次；`settings` 表面**每帧**都走到 `:4197`。
- 应改成：一次 `snapshot := a.settings()` 放函数开头，三处读它。
- **判准**：这不是「事件期读点」。`appShortcuts` 是 `View` 里调的（`:4254`），
  它读的是**这一帧**的绑定，和 `View` 开头那份必须一致——所以它属于绘制期，
  该走 memo。（真正的键事件处理（`HandleInput` 之类）在别处，本轮不碰。）

### 4. `terminalui` 的 `Label()`（当前会话名）同一族，顺手收口

- 位置：`internal/terminalui/terminalui.go:690-696`
- 现状：`a.Store.Snapshot().Language`，`deadcode` 报它**没有调用点**
  （`go run golang.org/x/tools/cmd/deadcode ./...` 本轮实测列出
  `internal/terminalui/terminalui.go:692:15: unreachable func: App.Label`）。
- 处置：**同一族，但本轮只改读法、不删**——删它属于另一件事（见「仍待」）。
  改读 `a.settings()` 之后它与 `LabelOf` 是同一个答案的两种入口，读法必须一致，
  否则下一个人照着 `Label` 写新代码就又漏了。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一个字节不动**。
- 本轮只改「设置值从哪里读」，不改它读出来的内容：**零像素**。
- 不新增任何 `t.Skip`（除已标注 opt-in 的截图 dump）；不删任何测试函数。

## 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

本轮在**干净 worktree** 上量的基线（不是抄旧报告）：

- PASS **2788** / FAIL 0 / SKIP 57 / 测试函数 **2073**（`grep -rn "func Test" | wc -l`）。
- `go test -count=1 ./...` 退出码 0。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

- **M1**：把 `terminalui.LabelOf` / `NoteOf` 改回 `a.Store.Snapshot()` →
  新的「一次闭包只拷一份」pin 红（读 `Store.Snapshots()` 的差值，而不是读
  `Memo.Builds()`——后者在有人绕过 memo 时仍然绿，这是 P11-co 已经踩过的坑）。
- **M2**：把 `shell.View` 开头的 `s := a.settings()` 改回 `a.Store.Snapshot()` →
  「一帧一份」pin 红。
- **M3**（故意不设 pin，报告里写明理由）：`appShortcuts` 三处 `a.settings()` 改回
  `a.Store.Snapshot()` 后，**memo 的 `Builds()` 计数看不出差别**（三处都发生在
  同一个版本内，第 2、3 次命中 memo 与「各自拷一份」在 `Builds()` 上等价），
  只有 `Store.Snapshots()` 的差值能看见——所以 M3 必须用 `Snapshots()` 作为尺子，
  否则这条 pin 是假的。

## 出图

`ui.Tester` 导出 `/tmp/floter-P11-cq-sessions-{dark,light}.png`（16 条会话的
sessions 页，最坏情况）与 `/tmp/floter-P11-cq-launcher-{dark,light}.png`；
**前后逐字节比对**（本轮只改读法，正确结果就是 md5 相同）。
