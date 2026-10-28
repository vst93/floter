# P11-af 报告：终端浮条不再挡住它下面的画布

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 全表
`已落地`，只剩 `⌘K` 动作面＝拍板）→ 上一轮「仍待」第 1 条（同上）→ **界面质感与交互**。
本轮取 F3「不要 floating/overlay 观感」在**输入侧**的后果：终端页顶部 28u 带整条是窗口
拖拽区，画布最上面两行输出收不到指针。规格 `docs/rounds/P11-af-spec.md`，合约更新在
`docs/raycast-design.md` §6.12。

## 1 缺口（根因，读框架源码确认）

`titleBar` 的 `bar` 是 `Absolute().Top(0).Left(0).Right(0).Height(7u)` 且**整条**
`.DragWindow()`。框架的命中表：

- `ui/input.go:297`：`interactive = flagClickable | flagFocusable | flagEditable |
  flagSelectable | flagDragWindow | flagDraggable | flagTrackPointer`；
- `ui/runtime.go:1208-1218`：`commitElement` 把**每个**非 `flagPassThrough`、非 invisible 的
  元素都记进 `rt.hits`（普通 `Box`/`Row` 也算），`ui/input.go:146-161` 只取最上面那一支的
  祖先链。

所以浮条（画布之上的绝对定位兄弟）把 28u 整条带变成命中目标：点不进画布，drag-select 起
不来。终端的 `CopyOnSelect`（抽屉里就有这个开关）在这两行上完全失效——**这是浮层不只是
视觉、还会吃掉输入的真 bug**，不是审美问题。

## 2 改法（一条规则 + 一个自证）

- 浮条 `bar`、它的 identity 行、状态点：**全部 `PassThrough()`**（不再是命中目标）。
  identity 行仍是 `Grow(1)`（布局不动），它不再吃指针。
- 窗口拖拽手柄收窄成**会话标题自己的字**：`.DragWindow()` 只出现在标题 `ui.Text` 上
  （文字是 inline 命中，只覆盖字形），于是这一页仍有 title bar 可拖，而标题右边到按钮之间
  那条带全部交给画布。标题的 tooltip（截断时的全名）跟着回到字上——挂在 `PassThrough`
  行上的 tooltip 永远悬停不到。
- 揭示改读**指针位置**：`_, _, hovered := bar.PointerPosition()`，`revealed := hovered ||
  bar.FocusWithin()`，阴影同用这一个 `hovered`。`PointerPosition` 在 `PassThrough` 元素上
  仍算包含关系（`ui/input.go:1265-1274`），所以悬停照旧把标题与按钮浮现出来，而**不需要**
  这条带是命中目标。`bar.Hovered()` 读的是命中链（`ui/input.go:1134-1148`），`PassThrough`
  之后它为假——这正是本轮必须换判定的原因，也是 M2 能变红的原因。
- 位置、28u 高、按钮槽位、抽屉与 resident note 的几何、键盘模型、会话管理、设置写回、
  i18n 文案：**零改动**。

## 3 台账

- **V=0 / B=0 / S=0**：没有旧 pin 钉过「浮条是不是命中目标」。既有的悬停类 pin
  （`barchrome_test.go:34`、`barshadow_test.go:38,40`、`barink_test.go:139`、
  `view_test.go:528`、`raycast_test.go:87,353,355`）都用 `Move(x,14)` 悬停在带子里，改后
  仍绿——它们量的墨/面/阴影一个字节没变。
- 新增 pin（`internal/terminalui/barreach_test.go`）：
  - `TestTheTerminalBarIsNotTheWindowsDragHandle`（源码契约，按 ast 取 `titleBar` 与
    `statusDot` 的函数体）：浮条链是 `AlignItems(ui.Center).PassThrough()`、identity 是
    `Label(identityTitle).PassThrough()`、`statusDot` 有 `PassThrough()`、
    `DragWindow()` **只有一处**且落点语句以 `ui.Text(c, identityTitle)` 开头、揭示读
    `PointerPosition()`。
  - `TestTheBarRevealsUnderAPointerItDoesNotCatch`（像素/行为）：指针清空时带子里没有墨；
    指针停在 x=600（`barTitle` 的墨在 x≈300 就结束，那里没有任何可命中元素）仍把标题揭示
    出来（墨量从 0 涨起来，且墨的右缘 <400，证明指针不在字上）；指针离开后重新折回 0。
  - `TestDumpP11AFBarPNGs`（opt-in `FLOTER_PNG=1`，+1 SKIP）。函数 1919 → **1922**。

## 4 mutation（实跑，`md5sum -c` 逐字节还原后 `touch` 复跑绿）

- **M1** 浮条 `PassThrough()` → `DragWindow()`：
  `barreach_test.go:108: the bar itself is a drag handle again: the whole 28u band would take the pointer`
  + `:111 the bar is not PassThrough…` + `:123 the bar has 2 DragWindow() calls, want 1`。
- **M2** `revealed := hovered || …` → `revealed := bar.Hovered() || …`：
  `barreach_test.go:61: the bar revealed no title under a pointer it does not catch`（`PassThrough`
  下 `Hovered()` 读命中链，永远假——揭示链整条死掉；既有 `barchrome`/`barshadow` 的悬停帧同理）。
- **M3** 摘掉标题的 `DragWindow()`：
  `barreach_test.go:123: the bar has 0 DragWindow() calls, want 1 (the session title is the page's title bar)`。

## 5 门槛（实测）

gofmt 空（`gofmt -l cmd internal` 无输出）· `go vet ./...` 净 · Windows/Darwin vet 净 ·
**PASS 2357**（基线 2355，HEAD `3c88036` 实测）· SKIP **23**（基线 22 + 本轮 1 条 opt-in
dump）· FAIL **0** · 函数 **1922**（基线 1919）· 三平台 `go build ./...` 过。

> 诚实记录：门槛那一轮 `internal/settingsui` 的
> `TestTheConfigSaveWearsItsSavingWordWhileInFlight` 在我同时跑三份 `go vet` 的负载下红过
> 一次（时序类）。单独复跑与随后的干净全量都绿（本轮 2357/0/23），与本轮改动无交集
> （`internal/settingsui` 一行未动）；按 B 类先跑，判为既有 flake，不改测试。

## 6 图上实测

`/tmp/floter-P11-af-bar-{rest,revealed}-{light,dark}.png`（860×320，终端页，标题
`the quick brown fox session name`）：

- `revealed` + 浅色：标题与状态点在 **x=12 起**画出来，墨的右缘约 **x=300**；而触发揭示的
  指针在 **x=600**——带子的中段**没有字也没有按钮**，它之所以还能揭示，只能来自指针位置。
  标题带下沿有一条淡投影（`ShadowContact`），就是 reference 的 `.terminal-bar:hover`。
- `rest` + 浅色：整帧白纸（画布），带子**一个像素都没画**——静止时第一/第二行输出仍贴最
  上沿，浮条不再覆盖内容。
- 深色两张同构：`rest` 是纯画布底色，`revealed` 只有左上的点与标题。

## 7 仍待

1. **`⌘K` 动作面**仍是合约里唯一未落地的条目（§4.6），要新增表面 + 新 chord → **需拍板**
   （原样有效）。
2. 拍板清单原样有效（P11-aa 的默认浅色安静档、P11-z 的搜索框 caret、P11-y 的滚动缓冲、
   P9-g 表那一批、P11-ae 的 `CanvasAlpha`）。
3. **需拍板（本轮缩窄）**：浮条现在只有「标题的字」是拖拽手柄。如果实机上觉得可抓的地方
   太小，可以把卡片外圈（`padX` 10u）之外再加一条显式 grip——那会动几何，属拍板。
4. 侦察结论（别当 TODO 重领）：`identity` 的 `Label` 留在 `PassThrough` 行上只作无障碍名，
   不产生命中；`PointerPosition` 每帧一次包含判定，无需缓存；`barButton` 的 hover 面仍是
   子元素命中（揭示后才可点），与改前一致。
