# P11-cq 报告：外壳这一帧也只拷一份设置

## ① 本轮做了什么

`P11-co` 给 `launcher` / `terminalui` / `settingsui` 三张表面都装了「一帧一份设置」的
memo（`settings.Memo`），**唯独漏了外壳自己**（`internal/shell`）。本轮把外壳的
绘制期读点接到同一把尺子上。

净改动 = 3 个文件的**取设置值的位置**，行为零改动、零像素。

| 文件 | 改了什么 |
|---|---|
| `internal/shell/shell.go` | `App` 加 `settingsMemo settings.Memo` + `settings()` 方法；`View` 开头、`appShortcuts` 三处绑定判定、`integrationList` 两处命令表改读它 |
| `internal/shell/notices.go` | `toastViewport` 的 tokens 与文案改读 `a.settings()`（一次，在闭包外） |
| `internal/terminalui/terminalui.go` | `LabelOf` / `NoteOf` / `Label` 改读本表面已有的 `a.settings()` |

`DirOf` / `SizeOf` **没动**：它们不读设置（`Term.Dir()` 与 `Term.Size()` 是插件自己的
字段读），与 P11-cp「仍待」第 4 条记的那件事是两回事。

## ② 为什么做它（实测证据）

用探针 + `runtime.Caller(1)` 插桩（跑完立刻还原并 `md5sum` 核对）量到的：

**闭包按会话数增长**（`Settings.Sessions()` 单独调用，每条会话拷两份）：

| 会话数 | 闭包拷几份 | 每会话 |
|---|---|---|
| 0 | 0 | — |
| 5 | 10 | 2 |
| 10 | 20 | 2 |
| 16（`maxSessions` 上限） | **32** | 2 |

两条 `Store.Snapshot()` 分别在 `terminalui.go:416`（`LabelOf`）与 `:427`（`NoteOf`）；
两个函数的**两条 return 分支都拷**，所以「有标题」和「没标题」都躲不过。

**整帧**（sessions 页，含闭包）：

| 会话数 | 一帧拷几份 |
|---|---|
| 0 | 4 |
| 5 | 14 |
| 10 | 24 |
| 16 | **36** |

插桩计数（2 条会话，四种表面各一帧，`uniq -c` 排序）证明外壳自己一帧 3 份：

```
80  terminalui/terminalui.go 427   (NoteOf)       ← 每会话
80  terminalui/terminalui.go 416   (LabelOf)      ← 每会话
49  shell/shell.go 4234            (View 的 s)
49  shell/notices.go 332
49  shell/notices.go 331
43  shell/shell.go 4197            (appShortcuts 的 ⌘W 分支)
 3  settings/snapshotmemo.go 43    ← 每版本一次（正常）
```

**帧成本**（`-benchmem`，720×580，sessions 页）：

```
                 ns/op      B/op    allocs/op
16 会话  修前  350047     13096      238
16 会话  修后  349324     11368      202     → −1728 B / −36 allocs
 0 会话  修前   92503      3416      100
 0 会话  修后   88873      3224       96     →  −192 B / −4 allocs
```

`Settings.Sessions()` 闭包单独（16 会话）：**6538 ns/op / 2112 B / 17 allocs**，
现在每帧只走一次，那 2×16 份深拷贝就没了。

16 条会话正是 `trimSessions` 强制的上限（`terminalui.go:286` `maxSessions = 16`），
所以这是这张表面**最坏情况下的常态**，不是边角。

## ③ 修后实测（同一组探针）

六张表面的**静止帧全部 0 份**深拷贝：

```
launcher       0 snapshots per frame
terminal       0
settings/general        0
settings/sessions       0
settings/integrations   0
settings/about          0
```

闭包单独（16 会话）：**0**。

## ④ pin 与 mutation

新增 `internal/shell/snapshotcost_test.go` 五条（+6 PASS，含 6 个子测试臂）：

1. `TestASessionsFrameTakesNoSettingsCopy` — 16 会话一帧 0 份
2. `TestTheSettingsCopiesDoNotGrowWithTheSessionList` — 0 会话与 16 会话同价
3. `TestTheSessionsClosureTakesNoSettingsCopy` — 闭包单独 0 份
4. `TestEverySurfacesFrameTakesNoSettingsCopy` — 六张表面逐臂（子测试）
5. `TestTheSessionsPageStillSaysWhatItSaid` — 标题与 ` · exit 3` 后缀一字不变

**尺子读 `Store.Snapshots()`，不读 `Memo.Builds()`**——这是 P11-co 踩过的坑：
`Builds()` 在有人绕过 memo 时仍然绿，而本轮修的正是「绕过去」的那些读点。

### 变异 4 条，全部实跑变红 + `md5sum` 逐字节还原 + `touch` 复跑绿

**M1**：`NoteOf` 改回 `a.Store.Snapshot()` → 红 4 条：

```
snapshotcost_test.go:72: a 16-session frame took 16 settings copies, want 0
snapshotcost_test.go:88: 0 sessions cost 0 copies and 16 cost 16
snapshotcost_test.go:101: one Settings.Sessions() call with 16 sessions took 16 settings copies
snapshotcost_test.go:136: settings/sessions took 16 settings copies on a settled frame
```

**M2**：`View` 的 `s := a.settings()` 改回 `a.Store.Snapshot()` → 六臂全红：

```
launcher took 1 settings copies ... terminal took 1 ... settings/general took 1
settings/sessions took 1 ... settings/integrations ... settings/about
```

**M3**：`appShortcuts` 的一处改回 `a.Store.Snapshot()` → **只有 settings 三臂红**
（general / sessions / integrations），launcher / terminal 绿。这与规格预测一致：
终端与启动器表面走不到那两处分支，只有设置表面每帧走到。

**M4**：`toastViewport` 改回两处 `a.Store.Snapshot()` → 六臂全红，读数 **2**（不是 1）：

```
launcher took 2 settings copies ... settings/about took 2
```

即「一帧两份」的原始缺陷被尺子直接量出来了，不是靠猜。

## ⑤ 零像素

四张 dump（sessions 页 16 会话 / launcher，各深/浅），**修前修后逐字节相同**：

```
sessions/dark   16b7b3a86f54fec54b97012b14ef514d  == 16b7b3a86f54fec54b97012b14ef514d
sessions/light  062d9a8507c145d5da978938ed2da532  == 062d9a8507c145d5da978938ed2da532
launcher/dark   7ccacbb0e92e0d2dba0e67ddca305f87  == 7ccacbb0e92e0d2dba0e67ddca305f87
launcher/light  e92f58f67677d8252e768d05b7155ec8  == e92f58f67677d8252e768d05b7155ec8
```

深/浅两张**互不相同**（上表四个 md5 两两不同），说明 dump 真的切了外观——
P9-b 那条教训（「不会响的守卫长得和通过的守卫一模一样」）在这里自证了一次。
第一次写 dump 时我漏了 `tt.SetDark`，四张图 md5 全同，是这条自证抓出来的。

**我实测看到了什么**（`/tmp/floter-P11-cq-sessions-light-after.png`）：
十六行会话，第一行 `Running · attached`（它在屏幕上），其余 `Running · detached`；
每行左侧 26px 图标槽，中间标题 + 灰色状态行（`0×0` 与启动时刻），右侧播放/垃圾桶
两个图标控件。深色版同一画面，面与墨整体下沉一档。launcher 那张是空查询的搜索行，
放大镜、占位文案、右上 `Type to search` 与快捷键提示都在原位。

dump 里我第一次把会话的 `ID` 留成 0，于是每行的垃圾桶都画成**armed**（`Terminate?` 药丸）。
那是 fixture 的错，不是产品的错：`armedSession == 0` 的含义是「没有人在问」，
而真实会话的 `ID` 由 `nextSessionID` 从非零开始发。改成 `ID: i+1` 后画面回到
静息的两图标形态。这条记在这里，免得下一个人照着旧 dump 以为是回归。

## ⑥ 门槛（全部实测）

```
gofmt -l cmd internal          空
go vet ./...                   净
GOOS=windows go vet ./...      净
GOOS=darwin  go vet ./...      净
go test -count=1 ./...         全绿（EXIT=0）
--- PASS                       2788 → 2799（+11）
--- FAIL                       0
--- SKIP                       57（不变）
测试函数                       2073 → 2078（+5）
三平台 build                   linux / windows / darwin 全过
```

基线在**干净 worktree**（`7647b1d0`）上量的：PASS 2788 / FAIL 0 / SKIP 57 / 函数 2073。

## ⑦ 判断与取舍

1. **`appShortcuts` 算绘制期还是事件期？** 它由 `View` 调用（`shell.go:4254`），读的是
   **这一帧**的绑定，必须与 `View` 开头那份一致 → 绘制期，该走 memo。真正的键事件
   处理在别处，本轮没碰。
2. **构造期的 `opts.Store.Snapshot()`（`shell.go:598/626-628/1019-1035`）不动**：
   那是 `New()` 跑一次，不是每帧；把它们也搬上 memo 只会让「这一帧的值」和
   「构造时的值」混在一个字段里。
3. **`terminalui.Label()` 是死代码**（`deadcode` 实测列出 `terminalui.go:693:15:
   unreachable func: App.Label`）。本轮**只改读法、不删**——删它属于另一件事
   （见「仍待」第 1 条）。改读法的理由是：它和 `LabelOf` 是同一个答案的两种入口，
   读法不一致，下一个人照着 `Label` 写新代码就又漏了。
4. **`integrationList` 也顺手收了**：`CommandSwitchesOf` / `CommandAliasesOf` 各拷
   一份（插桩 `shell.go:1647/1648` 各 4 次/帧）。它在 `a.Settings.Integrations` 闭包里，
   由设置页绘制期调（`settingsui.go:3555` 的 `installedIntegrations`），同族同表面。

## ⑧ 仍待（P11-cq）

1. **死代码清单**（`go run golang.org/x/tools/cmd/deadcode ./...` 本轮实测，
   全部 `unreachable func`）：
   `launcher.go:2890 App.firstSelectable`（P11-cn 起记了三轮）、
   `terminalui.go:693 App.Label`、`settingsui.go:1052 App.SelectedPage`、
   `settingsui/rows.go:734 App.rowLines`、`:862 App.rowTop`、
   `settingsui.go:4475 App.text`、`settingsui.go:5612 App.freshnessDetail`、
   `calculator.go:563 SortNewestFirst`、`extensions/custom.go:210 PreparedCustom.Discard`、
   `extensions/install.go:191 Prepared.Version`、`extensions/runfailure.go:71 RunErrorKeys`、
   `extensions/store.go:42 Store.Paths`、`inventory/sources.go:378 LookPath`、
   `settings/settings.go:267 LoadDefault`、`usage/usage.go:50 Store.FilePath`、
   `spawn/spawn.go:115 Terminal`、`:164 terminalEmulators`、`shell/system.go:417 numeric`、
   `shell/chrome_other.go:18 dressWindowsFrame`。**这些不是任何一轮的族**，
   删它们不改变行为，但一轮删 19 个是把「一轮一件事」破了——建议单独一轮「死代码清理」。
2. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
3. **`settingsDebounce` 仍是两份拷贝**（`settingsui` 与 `terminalui` 各自一份）：
   第三处需要时才抽包。
4. `terminalui` 的 `Term.Dir()`（OSC 7）/ `Term.Size()` 每帧各问一次插件：本轮查过，
   `DirOf` / `SizeOf` **不读设置**，与「一帧一份设置」不是同族，别混着领。
5. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器裁掉的行不进
   `Texts()` / PTY 自己画的行不在高度尺子上）。
6. **`shell.go` 里还有 40+ 处 `a.Store.Snapshot()`**，绝大多数是**事件期**读点
   （`custom.go` 的 `apply*`、`shell.go` 的 `SetLanguage`/`HandleInput` 之类），
   按 P11-co 定的规矩**故意保留**。下一轮若要再收，必须先逐处判「绘制期还是事件期」，
   不能一把梭。
