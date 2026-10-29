# P11-bj 报告 · 搜索页的字段带变成面板自己的第一条带

规格 `docs/rounds/P11-bj-spec.md`。选题：`.pending-bugs.md` 顶部「⚠️ 用户实测反馈
（2026-10-28）」的 **F4**（最高优先级）。F4 三条里第 3 条（按住 Super 的「进入终端」
固定底部、不重排、不重复）已由 P11-bb 落地，本轮做 F4-1；F4-2 本轮**没做成**，
原因与实测数字见下面 §4（不静默跳过）。

## 1. 根因与落地

**现象**：搜索页输入框是一块**自己的面**叠在面板上 —— 用户读作「像一块浮在上面的卡片」
「不连续边框」「外边距过大」。

**根因（file:line，本轮实测）**：`internal/launcher/launcher.go` 的字段行
（`c.Key("launcher.fieldRow")`，`Absolute().Top(0).Height(fieldRow)`）画
`Background(tk.FieldFill)`，而它**是面板的子节点**，面板自己是 `tk.Sunken`；字段行的
下沿又是那条 `HairlineSoft`（`Top(fieldRow - 1)`）。于是同一条纵向列上有两个面 + 一条线：

| 带 | 浅色（修前） | 深色（修前） |
|---|---|---|
| y=0..54 字段 | `(250,250,251)` | `(28,28,31)` |
| y=55 发丝 | `(236,236,237)` | `(40,40,43)` |
| y=56.. 面板 | `(253,253,254)` | `(23,23,25)` |

**落地**：
1. 字段行**不再画面**（删掉 `.Background(tk.FieldFill)`）。那条带就是面板自己的
   `Sunken`，一层，不是两层；唯一还标出边界的，是契约要求的那 1px 下沿（R31 的
   「输入框下方只保留 1px」，用户当时点名要的）。`Padding(0, Space(4))`、`Height(fieldRow)`
   （56u，R37 与设置页头同高）一个数没动。
2. 字段面原来是**不透明遮罩**，去掉之后滚上去的行会从查询文字底下透出来（R21 那笔账）。
   所以列表不再靠 `Padding(listEdge, …)` 把内容顶下去，而是**前面放一条 `listEdge` 高的
   流内占位**，滚动区从带子下沿开始 —— 行根本不进那条带。`a.listRect` 因此直接读
   `list.Bounds()`，原先手写的 `Y += listEdge; H -= listEdge` 删掉（盒子自己已经在下面，
   编号家族读到的视口宽度/高度不变）。
3. `theme.Tokens.FieldFill` 标 `Deprecated`（产品代码零读取）：token 表留一行，记录这个
   面曾经存在、以及为什么被拿掉。

**修后实测**（同一台机器、同一帧、同一像素点）：

| 带 | 浅色（修后） | 深色（修后） |
|---|---|---|
| y=0..54 字段 | `(253,253,254)` == 面板 | `(23,23,25)` == 面板 |
| y=55 发丝 | 仍是那条（pin 断言与上下邻居都不同） | 同左 |
| y=56.. 面板 | `(253,253,254)` | `(23,23,25)` |

**我实测看到了什么**：`/tmp/floter-P11-bj-fieldband-{light,dark}-{before,after}.png`。
浅色那张里，修前 y=0..54 是一条**比下面略灰的实心带**（250 对 253），带子下沿一条更深的
线，三者拼出「一张贴上去的卡」；修后带子与面板同一块面，只剩一条 1px 的线横在 56u 处
（而不是一条线把两块面隔开）。深色那张更明显：修前 28 vs 23，带子是**浮起来的那块**；
修后整片 23，标题/查询文字直接坐在面板上。

## 2. 台账（V / B / S）

- **V=1**：字段带的像素（新 pin 读它）—— 旧值 `FieldFill` 的两档 → 面板自己的 `Sunken`。
- **S=1**：`TestTheChipsBandLeavesTheRowsBeneathItToThePointer` → 改名
  `TestTheChipsBandSitsAboveTheScroller`。它原来的前提是「行会滚到 chips 带底下，所以带子
  必须让出指针」（P11-ap）。现结构下带子是流内预留的，**行根本不进带子**，前提消失；
  断言改成更强的那件事：`a.Results()` 里没有任何一行的矩形落在带子里（+ chips 仍可点）。
  同一个文件里的 opt-in dump `TestChipBandHoverDump` 同步改掉它对「带子里有一行」的依赖。
- **B=0**：高度链、键盘模型、会话、设置写回、i18n、命令接线零改动；既有行为 pin 一条没改。

## 3. 门槛（本轮实测）

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净
GOOS=windows go vet ./...      # 净
GOOS=darwin  go vet ./...      # 净
go test -count=1 ./...         # 全绿（0 FAIL）
PASS 计数                      # 2444 → 2448（+2 pin 的两臂 + 1 pin + 1 opt-in dump 的 SKIP）
测试函数数                     # 1991 → 1994
三平台 build                    # linux/windows/darwin 全过
```

新增 pin：
- `TestTheFieldBandIsThePanelsOwnSurface`（`fieldsurface_test.go`，两外观）：
  字段带像素 == 面板像素；下沿那条线与上下邻居都不同。
- `TestNoRowScrollsUnderTheFieldBand`：30 个应用 + 最近启动，滚两格后
  **没有一行的 `y` 小于带子下沿**（`fieldRow + breath` = 60）。
- `TestDumpTheSearchPagesFieldBand`（opt-in `FLOTER_PNG=1`）：本轮的证据图。

## 4. mutation（实跑，注缺陷 → 看到红 → 还原 → md5 核对 → touch 复跑）

- **M1** 把 `.Background(tk.FieldFill)` 加回字段行：
  ```
  --- FAIL: TestTheFieldBandIsThePanelsOwnSurface/light
      the field band is {250 250 251 255} and the panel is {253 253 254 255}: the field still paints a face of its own
  --- FAIL: TestTheFieldBandIsThePanelsOwnSurface/dark
      the field band is {28 28 31 255} and the panel is {23 23 25 255}: the field still paints a face of its own
  ```
- **M2** 删掉列表前面那条 `listEdge` 占位：
  ```
  --- FAIL: TestNoRowScrollsUnderTheFieldBand
      the row "Terminal" is drawn at y=8, above the field band's edge (60): the scroller starts under the band
      the row "Clipboard history" is drawn at y=0, above the field band's edge (60): ...
  ```
- 还原：`md5sum -c /tmp/bj.md5` → `internal/launcher/launcher.go: 成功`，`touch` 后两条 pin 复跑绿。

## 5. F4-2 为什么本轮没交付（实测，不是没做）

F4-2（空查询不再列内置指令）我按参考实现改了：`otherResults` 的空查询分支删掉
`out = append(out, a.commands()...)` —— 参考 `useLauncherCatalog.ts:1138-1166` 的空查询
分支就是只 `return recentRows`，六条 `SYSTEM_COMMANDS` 只进有查询时的打分（`:1201` 起）。
实测后果：`go test ./internal/launcher/` **21 条 pin 变红**，因为这些 pin 的**夹具**就是
「空页面上那六行内置指令」—— `TestTheFieldAndTheRowsShareTheReferenceTextColumn`、
`TestTheListIsTheCentredReferenceColumn`、`TestEveryBuiltInRowLeadsWithItsOwnMark`、
`TestThePowerRowsWearTheWarmPlate`、`TestThePowerGlyphsWearTheSystemIconInk`、
`TestDrawnPitchIsTheChargedRow`、`TestClickingThroughTheScrollEdge`、`TestScrollingTheResultList`、
`TestTheRowBlockIsTheInsetRoundedPill`、`TestTheSelectedRowWearsThePerAppearanceQuietTint`、
`TestThePointerTakesTheSelectionOnEntry`、`TestARestingPointerDoesNotFightTheKeyboard`、
`TestTheRowTrailingBadgeIsTheReferenceActionSlot`、`TestLauncherShowsTheCatalogWithTheFieldFocused`
等。它们量的都是**行**的几何，而行的来源换了：这是 **B 类重写夹具**，不是改产品。
一轮 40 次工具调用的预算里做不完（**要求每条 pin 改成用 recents 作夹具、并把「六行内置指令」
留在 Catalog 的 pin 里**），按纪律**回退这部分、不在门槛不过的情况下提交**，记进
`.pending-bugs.md` 的「仍待」并写清下一步怎么做 —— 这是本轮唯一没交付的 F4 条目。

## 6. 仍待（下轮起点）

1. **F4-2**：删 `otherResults` 空查询里的 `a.commands()`，然后把上面那 21 条 pin 的夹具
   从「空页面上的六行命令」换成「最近启动的若干应用」（`testApp()` 加 `SetApps` + `SetRecent`
   的夹具助手），并把「六条内置指令仍在 Catalog/打分里」单独钉一条 pin。
   **注意**：`Results()` 里 `powerItems()` 在空查询时返回 nil（`power.go:33-37`），所以
   空页不会因为这条留下命令行。
2. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / `P11-an` 的便条投影 / `P11-be` 的 `Permissions` 行 / `P11-bf` 的
   `configDiscardBar` 12u margin / `P11-bg` 的顶边失败带覆盖画布顶部两行。
3. `assignOutputNumbers`（`output.go:826`）仍是 `OutputList.Visible()` + 闸门（P11-bi 记下的
   同族不同表面）。
