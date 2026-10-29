# P11-co 报告：一帧只拷一份设置（三张表面同一把尺子）

规格 `docs/rounds/P11-co-spec.md`。选题顺序：用户实测反馈 F1–F4 已收尾 → 合约 §6 全
`已落地`（只剩拍板项）→ P11-cn「仍待」第 1 条整张是拍板项、第 2/3 条是记账与「先量再领」、
第 4 条是尺子盲点 → 按用户定的「界面质感与交互（含用户报的 bug）」自选，挑的是**帧里
的真开销**（F4 原话里的「流畅性」）。

## 1. 真 bug（本轮实测，不是推测）

`settings.Store.Snapshot()` **不是读字段，是整份深拷贝**：`internal/settings/store.go:63-68`
调 `clone(st.current)`，而 `clone`（`:227-235`）在 `extra` 非空时新建 map 并逐个搬键。

本轮在 `Snapshot()` 里临时插桩计数（`ProbeSnapshotCount++`，量完 `cp` 还原、
`md5sum` 逐字节核对回 `b00e0fdd824d5aff09a6fedfd734f8f9`），实测**每帧调用次数**：

| 表面 | 修前 | 修后 | 说明 |
|---|---|---|---|
| `internal/launcher`（720×420，带查询） | **0** | 0 | 早已有版本 memo（旧 `launcher.go:1115-1132` 手写三字段） |
| `internal/terminalui`（800×600，一会话） | **5** | **1→0** | P11-bt 只 memo 了**调色板**，设置本体没 memo |
| `internal/settingsui` General（720×580） | **15** | **1→0** | P11-bs 同上：只 memo 了调色板 |
| `internal/settingsui` Integrations（20 集成 × 5 命令） | **53** | **1→0** | 每张卡每行都拷一遍 |

（「1→0」= 设置变过的那一帧 1 次，之后静止帧 0 次。）

分配剖面同向：`BenchmarkGeneralFrame` 的 `-alloc_objects` 里 `floter/internal/settings.clone`
占 **9.31%**（76 461 / 821 281 个对象），排第三。

**这是一族同源问题**：仓库里「版本做键」这件事有三份说法（`launcher` 手写三字段、
`theme.Memo` 的 `load func() settings.Settings`、两张表面**什么都没有**），而更热的
两张表面恰好没接。

## 2. 落地

- **`internal/settings/snapshotmemo.go`（新）**：`type Memo`，键 = **store 指针 + version**，
  `Get(*Store) Settings` / `Builds() uint64`。`nil` store 返回零值。形状对齐
  `theme.Memo`（`internal/theme/memo.go:29-63`）。`Reset()` **没写**——没有任何调用点，
  加它就是新造一条死代码。
- **`internal/settings/store.go`**：加 `snapshots uint64` 计数 + `Snapshots() uint64`
  （一次已在锁内的自增）。它是 pin 要读的那件事：pin 必须能看见「真的拷了几次」，
  只看 `Memo.Builds()` 会漏掉绕过 memo 的读法。
- **`internal/launcher/launcher.go`**：三字段（`settingsVersion`/`settingsCached`/`settingsValid`）
  换成 `settingsMemo settings.Memo`，`settings()` 变成一行 `memo.Get`。19 个读点零改动。
- **`internal/terminalui`**：`App` 加 `settingsMemo` + `settings()`；四个**绘制期**读点
  （`View` 的面板体、空页的编号徽章、`canvasInks` 的主题名、`titleBar` 的提示）改读它；
  `tokensOf` 的 loader 由 `a.Store.Snapshot` 改成 `a.settings`；`drawer.go` 两处（`drawer`
  的语言、`drawerGrid` 的整份）。
  **故意留着 `Store.Snapshot()` 的**：`OnPaste` 回调（`terminalui.go:237`，跑在终端自己的
  goroutine 上，`Memo` 不是并发安全的）、`Label`/`LabelOf`/`NoteOf`/`options`/`Pad`
  （事件期与启动期）。
- **`internal/settingsui`**：`App` 加 `settingsMemo` + `settings()`；`copy()`、`language()`、
  `page()`、`View()`、`basePlugins`、`general`、`residencyRow`、`appIcon`、`binding`、
  `integrations`、`approvalDetail`、`permissionAudit`、`permissionLine`、
  `toolPermissionFields`、`uninstall` 主投影全改读它；`tokens()` 的 loader 同 terminal。
  **故意留着 `Store.Snapshot()` 的**：`recordShortcut`（事件期）、`textinput.go` 的
  `HandleInput`（事件期）——它们不在帧里，不该被帧缓存串味。

## 3. 台账

**V=0 / B=0 / S=1**。S 那一条 = `launcher.App` 三字段换成一个 `settings.Memo`，
两个 `App` 各加一个字段与一个方法。高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n /
命令接线**一个字节没动**，**零像素**（下面 §5 有逐字节证据）。

## 4. pin（+8 函数 / +6 PASS / +2 opt-in SKIP）

- `internal/settings/snapshotmemo_test.go` · `TestAMemoServesOneSnapshotPerVersion`：
  200 次读只拷 1 次；`Update` 之后读到新值；**被拒写回滚后读到回滚值**；两个都从 version 0
  开始的 store 必须各算各的；`nil` store 是零值。
- `internal/settingsui/snapshotpin_test.go` · `TestASettingsFrameCopiesTheSettingsOnce`
  （General 与 20×5 Integrations 两臂各读两条：变过的一帧 == 1、静止帧 == 0）与
  `TestASettingsFrameDrawsTheChangedValue`。
- `internal/terminalui/snapshotpin_test.go` · 同名两条（终端页）。
- `internal/launcher/snapshotpin_test.go` · `TestALauncherFrameCopiesTheSettingsAtMostOnce`
  （对照臂：静止 0、变过 1）。

门槛（本轮在干净 worktree 上量的基线，不抄旧报告）：

```
gofmt -l cmd internal                             空
go vet ./...                                      净
GOOS=windows go vet ./...                          净
GOOS=darwin  go vet ./...                          净
go test -count=1 ./...                            FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"  2776 → 2782
测试函数数                                         2060 → 2068
--- SKIP                                           56 → 58（新增两条 opt-in：dump 与测量件）
GOOS=linux|windows|darwin go build ./...           过
```

## 5. 零像素证据（逐字节）

`internal/settingsui/p11co_dump_test.go`（opt-in，`FLOTER_PNG=1`，`FLOTER_PNG_TAG` 分前后）
对 General 与 860×900 的 20×5 Integrations 两页 × 两外观出图；「前」用 `git stash`
只暂停产品文件跑同一条 dump：

```
general-dark:        IDENTICAL (e9b3dbde920c…)
general-light:       IDENTICAL (0f0b8229f7aa…)
integrations-dark:   IDENTICAL (b86917e5a31d…)
integrations-light:  IDENTICAL (54795b7d87d6…)
```

图：`/tmp/floter-P11-co-{general,integrations}-{dark,light}-{before,after}.png`。
**我在这几张图上看到的**：`before` 与 `after` 是同一份字节——这一轮拿掉的是「同一份值读了
十五遍 / 五十三遍」，不是任何一块墨、任何一条线、任何一档间距。

## 6. 变异（3 条实跑变红 + 逐字节还原）

| # | 注入的缺陷 | 红在哪 | 原始报错串 |
|---|---|---|---|
| M1 | `Memo.Get` 命中判断去掉 `m.version == version` | `settings` 的「变过就读新值」一臂 | `after a change the memo served language "en", want zh` |
| M2 | `settingsui.general` 的 `s := a.settings()` 改回 `a.Store.Snapshot()` | 设置页 pin 两条 | `the General page: one frame took 2 settings snapshots, want 1` / `a steady frame took 1 settings snapshots, want 0` |
| M3 | `terminalui.View` 的 `stored := a.settings()` 改回 `a.Store.Snapshot()` | 终端页 pin 两条 | `one terminal frame took 2 settings snapshots, want 1` / `a steady terminal frame took 1 settings snapshots, want 0` |

还原：`md5sum -c /tmp/p11co-md5.txt`（`snapshotmemo.go`、`settingsui.go`）与
`md5sum -c /tmp/p11co-md5-term.txt`（`terminalui.go`）全部「成功」。

**M2/M3 值得单独记一笔**：两条只差一个读法，静态看是「等价重写」，只有计数 pin 能看见。
这也是为什么本轮把 `Store.Snapshots()` 做成产品侧计数器，而不是只读 `Memo.Builds()`——
后者在「有人绕过 memo」时仍然是 1，什么都不会红。

## 7. 实测开销（`-benchmem`，同机交错跑，「前」用 `git stash` 暂停产品改动）

| 基准 | B/op 前 → 后 | allocs/op 前 → 后 |
|---|---|---|
| `BenchmarkGeneralFrame` | 17 091 → **16 371** | 225 → **210** |
| `BenchmarkIntegrationsFrame` | 46 952 → **44 408** | 329 → **276** |
| `BenchmarkTerminalFrame` | 1 432 → **1 192** | 39 → **34** |

`ns/op` 在本机噪声很大（同一份代码重跑能从 221 µs 跳到 437 µs，机器上有别的负载），
所以**不拿它当结论**；分配数是确定性的，以上面这张表为准。

## 8. 抢救 stash 的回收

`stash@{0}` = `9aff92d6db`「floter orphan 2026-10-30 05:37」：**树与 HEAD 逐字节相同**
（`git diff --quiet 9aff92d6db HEAD` 通过，与三个父提交也相同），唯一没被后续轮次覆盖的是
它的**未跟踪**载荷 `internal/terminalui/probe6_test.go`。回收方式：它是一条**测量探针**
而不是接好的修复，所以按测量件收进 `internal/terminalui/dragwrites_test.go`
（opt-in `FLOTER_MEASURE=1`，只报数字不断言），并把它的发现写成下一轮候选——
**一次抽屉拖拽 22 帧产生 17 次被接受的设置写**（每次成功写都落盘）。整条**没有静默丢弃**。

## 9. 仍待（P11-co）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
2. **下一轮候选（本轮量到、没做）**：抽屉拖拽的写没有 debounce —— 22 帧的拖拽 = 17 次
   落盘写（`internal/terminalui/drawer` 的滚轴 → `Store.Update` 成功路径）。设置页那条
   同源路已经有 `settingsui/debounce.go`，抽屉这条没有。量法与数字在
   `internal/terminalui/dragwrites_test.go`；**要先拍板**「拖拽期间只改内存、松手写一次」
   算不算行为改动（它会让设置文件少写 16 次，但「拖的时候磁盘上的值」会变）。
3. `launcher.go` 的 `firstSelectable`（P11-cn 仍待第 2 条）**仍然没有调用点**：本轮没动
   （不是本族），下一轮顺手清。
4. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器裁掉的行不进
   `Texts()` / PTY 自己画的行不在高度尺子上）。
