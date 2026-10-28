# P11-ar 报告 · 一份新的捕获输出从它自己的开头开始

**轮号**：P11-ar（上一轮 P11-aq，HEAD `d91ece0`）
**规格**：`docs/rounds/P11-ar-spec.md`
**台账**：B 类（行为轮）· V=0 · S=0 · 既有 pin 零改动
**门槛**：PASS **2384**（基线 2381）· FAIL 0 · SKIP 29 · 测试函数 **1947**（基线 1946）·
gofmt 空 · `go vet ./...` 净 · Windows/Darwin `go vet` 净 · 三平台 build 过
**变异**：3 条实跑变红并逐一还原（`md5sum -c` 逐字节核对 + `touch` 复跑绿）

## 1. 选题

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩
`⌘K` 动作面＝拍板）→ 上一轮「仍待」第 1 条 → **界面质感与交互逻辑**（用户 2026-10-28 定：
界面/质感/交互优先）。上一轮「仍待」第 1 条（全仓 `Absolute()` 逐个量「预留高度 vs 自然
高度」）本轮逐条否掉（见 §5），于是按用户定的优先级在「交互逻辑」这一档里实测到这条
真 bug。

## 2. 根因

输出的两个滚动状态是 **App 级**的：

- `internal/launcher/launcher.go` 的 `Output ui.ScrollState`（`Output` 的滚动偏移）与
  `OutputList ui.ListState`（输出列表的可见行/选中）。
- 它们**必须**是 App 级——`ui.Scroll` 的 `TrackScroll(&a.Output)` 与 `ui.List` 的
  `&a.OutputList` 需要同一份状态跨帧保留偏移，否则每帧回顶。

但 `internal/launcher/output.go` 的 `showOutput`（`showOutput` 换上一份**新** `*OutputView`）
**从不重置**这两个状态。于是它们被**跨视图**复用：

- 打开一份 300 行的文本捕获，滚轮到 `Output.Y = 400`；
- `Esc` 关掉（`leaveOutput` 只把 `a.output = nil`，不碰 `a.Output`）；
- 再跑**另一份** 300 行的捕获 → 新页面打开在 `Output.Y = 400`（实测），用户看到的是
  它的第 ~18 行起步，第一行不见了。
- 列表页同理：第二份长列表打开在 `visible first = 9`（实测），不是第 0 行。

字段上那句注释 `Output keeps its scroll offset` 描述的是**跨帧**要求（同一份输出下一帧仍
保留偏移），不是「跨视图」决定——它从没表达「第二份输出该沿用第一份的偏移」。这与
P11-ao 修的是同一条规则（换一个设置页从顶上开始，`internal/settingsui` 的 `selectPage`）：
**一份新内容从它自己的开头开始。**

## 3. 落地

`internal/launcher/output.go · showOutput`：在换上 `a.output = &view` **之前**重置一次

```go
a.Output = ui.ScrollState{}
a.OutputList = ui.ListState{}
```

- 只在 `showOutput`（一份新捕获换上来）里重置；**逐帧重建路径一动不碰**，否则滚轮每帧
  被归零（M3 实测会把 `TestTheOutputPageScrollsWithTheWheel` 打红）。
- `loadMoreOutputPage`（`output.go:179`）是**同一视图**续页，不经过 `showOutput`，分页
  续页不受影响。
- 顺带把 `launcher.go` 里 `Output` 字段的注释改成显式说明「同一视图内保留，新捕获上来时
  重置（`showOutput`）」，免得下一个人再把它读成「跨视图也保留」。

`ui.ListState{}` 归零是安全的：列表的 `Key` 是 `c.Key("launcher.output.list")`（在上下文上，
不在 `ListState` 里），没有别处写 `a.OutputList.Key`（`grep -rn "OutputList\." internal/`
只有 `ScrollIntoView`/`AtEnd`/`Visible` 三处读）。

## 4. 变异（3 条实跑，逐条还原）

基线 `md5sum internal/launcher/output.go = ffe05ccf4546f5505c7f3d4faf648355`。

| # | 注入 | 变红的 pin | 原始报错串 |
|---|---|---|---|
| M1 | 删掉 `a.Output = ui.ScrollState{}`（留列表那条） | `TestAFreshOutputStartsAtItsBeginning/text` | `outputfresh_test.go:61: a fresh text output opened at Y = 400 (the previous page was at 400), want 0: the new capture starts mid-way instead of at its first line` |
| M2 | 删掉 `a.OutputList = ui.ListState{}`（留文本那条） | `TestAFreshOutputStartsAtItsBeginning/list` | `outputfresh_test.go:100: a fresh list output starts at visible row 9 (the previous list was at 9), want 0: the new capture opens scrolled down` |
| M3 | 把重置挪到 `outputBody`（每帧执行） | `TestTheOutputPageScrollsWithTheWheel`（既有 pin） | `wheel_test.go:57: a wheel notch left the page at Y = 0: the wheel does not reach the output's scroll container` |

每条：注入 → 看到红 → `cp` 还原 → `md5sum -c` 成功后 → `touch internal/launcher/output.go`
再复跑一次绿（避免测试缓存复用）。

## 5. 上一轮「仍待」第 1 条：逐条否掉（别当 TODO 重领）

P11-aq 留下「全仓其余 `Absolute()` 逐个量预留高度 vs 自然高度，剩下 `actionBarFooter` 与
`terminalui` 的浮条」。本轮逐条读代码实测，**两处都不需要动**：

- **`actionBarFooter`（`internal/launcher/launcher.go:2821` 起）**：它不是「浮层盖住可滚内容」。
  它是正常流的兄弟列（`Column(c).FillWidth().Shrink(0).Height(band)`），**上面**的滚动器
  `Grow(1)` 消化剩余高度，行滚到分割线就停在 footer 上沿（注释原话）。它下面的 `HairlineSoft`
  是 `Absolute` 但 `PassThrough`，只是分割线。**没有内容在它下面**，`PassThrough` 不适用。
- **`terminalui` 的浮条**：`titleBar` 已在 P11-af 里 `PassThrough`（`terminalui.go:1309`）；
  `residentNote`（`:1195`）与 `feedbackNote`（`:1247`）是**有自己填色**的浮卡（`Background(tokens.Float)`），
  与「无底的浮带」不是同一形状——它们本来就是可点/可读的浮层，不是死区。P11-af 的判据是
  「透明浮带自己声明透明」，这两张卡有底，不在判据内。
- 其余 `Absolute()` 复核：`filters.go:199` 已 `PassThrough`（P11-ap）；`launcher.go:1305` 字段行
  `Background(tk.FieldFill)` 不透明（设计如此）；`launcher.go:1324` 拖拽带 `DragWindow` 要命中；
  `launcher.go:1495`/`:2853` 是 `PassThrough` 的分割线；`power.go:145`/`launcher.go:2949` 是带填色的
  确认/提示带；`settingsui/rows.go:794` 是 `PassThrough` 的 5u 徽点；`settingsui/uninstall.go:38`
  是模态背景；`settingsui/settingsui.go:1475`、`shell/shell.go:4465` 已 `PassThrough`。

## 6. 视觉证据（改前/改后第二份输出的第一帧）

- `/tmp/floter-P11-ar-second-before.png`（去掉重置）：第二份输出 `Y = 400`，正文顶部
  （y≈119–133）是 `filler line yyy…`——**第一行 `FIRST-LINE-MARKER` 被滚掉了**。
- `/tmp/floter-P11-ar-second-after.png`（本轮）：第二份输出 `Y = 0`，正文顶部
  （y≈124–134）是那块更宽的**大写 `FIRST-LINE-MARKER`**——从它自己的第一行开始。

**我实测看到了什么**：同一段操作（滚一份长输出到 400 → `Esc` → 再跑一份长输出），改前
第二份页面的第一帧从上一次的偏移处起步、第一行不可见；改后第二份页面从第一行画起。列表
臂的读数同样是 `visible first 9 → 0`。

## 7. 台账

- **B 类**（行为轮）：既有 pin 一条没改，全绿。
- **V = 0**：没有旧 pin 钉过「第二份输出的初始偏移」。
- **S = 0**：没有结构变更。
- 新增 `internal/launcher/outputfresh_test.go`，`TestAFreshOutputStartsAtItsBeginning`
  两个子测试（text / list），并引用既有 `TestTheOutputPageScrollsWithTheWheel` 作反面守卫。
- 函数 1946 → **1947**；PASS 2381 → **2384**（父 + text + list）；SKIP 29 → 29。

## 8. 仍待（P11-ar）

1. **`⌘K` 动作面**仍是合约里唯一未落地的条目（§4.6）：页脚画着键帽但 chord 没绑定。
   要新增表面 + 新 chord → **需拍板**（原样有效）。
2. 五条拍板项原样有效（caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1），外加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
3. **新发现（需拍板，本轮未做）**：输出文本页 `ui.Scroll` 只接滚轮/触控板，**没有键盘滚动**——
   在输出页里 `↑/↓` 只走列表（`moveOutputSelection` 在 `!IsList` 时直接 return），文本页
   没有 PageUp/PageDown/Home/End。参考侧 `useAppKeyboard.ts:232` 用 `Shift+PageUp/PageDown`
   给**终端**翻页；输出文本页在参考里是普通滚动容器（浏览器默认给键盘）。本仓键盘模型
   属零改动红线，故不擅自加，列入拍板。
4. 侦察结论（本轮实测否掉，别当 TODO 重领）：见 §5，`actionBarFooter` 与 `terminalui`
   浮条都已是正确形状。
