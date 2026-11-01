# P13-ce 报告：捕获输出页的满态收口——折叠是语法，条子下的呼吸不跟着滚

规格 `docs/rounds/P13-ce-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.48**｜
起点 `15df3d51`（P13-cd）

## 这轮为什么让它更轻、更有空间感

上一轮留下一句话：「下沿是 `foldAlpha`，顶边却是行滚到条子下面被硬切，两端不成对」。
**先量，量完这句一半不成立**：列表两端其实都走同一个比例规则（顶 0.40 / 底 0.70）。
真正不成对的是**那道 4u 呼吸**——它此前是列表自己的 padding，属于**内容**，于是滚一行
就被内容吃掉：实测顶边那行被折边切成两半的字尾落在 **y=102**，而条子的最后一行墨在
**y=100**，中间只隔 **2px**。正文分支更糟，它连折边都没有，滚起来第一行字是**满墨 228**
贴在条子上。现在这 16px 归**页面**：裁剪边固定在 118，条子下面永远有 16px 的空气。
**页头是页头，内容是内容**——这是空间感的第一条，比任何材质都便宜。

## 改了什么

| 文件 | 改动 |
|---|---|
| `internal/launcher/output.go` | 列表分支：4u 呼吸移到**裁剪之外**（`ui.Column(...).Grow(1).FillWidth().MinHeight(0).Padding(header,0,0,0)` 包住 `ui.List`，列表自己 `Padding(0, Space(2), 0, Space(2))`）；正文分支同一形状（`Column.Fill().Padding(header,0,0,0)` + `Scroll.Padding(0, …)`） |
| `internal/launcher/outputedges_test.go` | 新 pin 4 条 + 一条 opt-in 出图 |
| `docs/liquid-glass.md` | 新增 §6.48（含「为什么这里不能上 `ScrollEdge`」的判据） |

**没有改的**：`foldAlpha` 本身（折边是这一族的语法，P13-v / P13-w / P13-x）、
`view.Text`、`pluginTextRows`、`OutputList`、`PinText`、`outputText()`、`outputSpans`、
键盘模型、i18n 文案。**高度链零改动**——少掉的 16px 视口高度与多出来的 16px 空气是同一笔。

## 实测（720×488 帧，深浅同值）

| | 改前 | 改后 |
|---|---|---|
| `a.outputListRect` | `{0,102,720,364}`（含 4u 呼吸） | `{0,118,720,348}`（就是裁剪盒） |
| 条子最后一行墨 → 视口上沿 | **2px**（100 → 102） | **19px**（99 → 118） |
| 滚一行后条子下的 16px 带 | 列表 **92** / 正文 **228**（浅）、**88** / **221**（深） | **0** / **0**（两套） |
| 顶边被切行的墨（`Center` 滚动，露出 0.85） | 满墨 228（顶边不折的变异帧） | **196**（理论 193） |
| 下沿被切行的墨（露出 0.85） | 194 | **194**（不变） |
| 静止帧四张图 md5 | `8a401b18…` / `3c575bcc…` / `5d537ad9…` / `53ed8aef…` | **逐字节相同** |
| 滚动帧图上差分 | — | 列表 **317 px**，包围盒 `16,102..91,111`；正文 **507 px**，包围盒 `13,109..179,117`——全部落在 102..117 这条带里，别处一个像素没动 |

图上看到的一处变化：滚动帧里条子下面那一行「被切成两半的字尾」整条消失了，条子与内容
之间第一次有一条 16px 的空气带（`/tmp/floter-P13-ce-list-{light,dark}-{before,after}.png`）。

## 判据（写进 §6.48 的两条）

1. **呼吸不滚**：条子最后一行墨之下的 16 行在**任何滚动位置**上墨为 0。
2. **两端成对**：被折边切到的行，墨 ≈ 可见比例 × 整行墨（±12.5%）。

**为什么不是 `glass.ScrollEdge`**（设置页 §6.16 / §6.17 的做法）：启动器卡片是**窗口层的
半透明面**（§1），下面没有一块不透明的面可以重放；`ScrollEdge` 在这里只能拿一枚假的不透明
色去盖玻璃，那正是 §0 禁止的「拿背景色回放的渐变遮罩」，也是 `foldAlpha` 注释里早写下的
禁令。**卡片上的收口只能是内容侧的**——这条现在写进合约了，下一轮不用重猜。

## 偷了什么（qiaomu-design · craft-loop.md）

**DNA 注入（`references/design-systems/apple/DESIGN.md`，3 个具体值）**

1. `Spacing System: Base unit 8px`（`:185`）——这一轮那 16px 正好是基数的 ×2。它此前被写成
   列表的 padding（= 内容），落地改成**页面的**带：同一个数值，换了一个归属。
2. `Compression within, expansion between`（`:199`，内容内部压缩、容器之间扩张）——这正是
   「页头与正文之间那块空气不许被内容吃掉」的判据本身：**之间**的空间不属于任何一边。
3. `No visible grid lines or gutters — spacing creates implied structure`（`:194`）——所以
   这一页**不加**任何可见的边/发丝/渐变遮罩：折边（内容退后）+ 呼吸（页面的带）就是结构，
   与 §0 的禁令同向。

**工艺密度（craft-loop.md 第二节菜单，逐项指认）**

- **第 6 项「编辑细节」→ 落地**：页头带 4u(16px) 固定，条子墨底到内容第一条墨 ≥ 18px。
- **第 9 项「自定义滚动条」→ 反向落地**：没有滚动条；「还有多少」由 `foldAlpha` 的
  比例刻度说（顶 0.85 → 196、下 0.85 → 194）。
- **第 12 项「材质细节」→ 反向落地**：这一轮**不引入任何材质**（不上 `ScrollEdge`、
  不上渐变遮罩），理由是卡片本身是半透明层，假面会把玻璃画死。
- **第 5 项「多层染色阴影」→ 故意不做**：滚动区在 E1 以内，§2 判据 3 说阴影只给 E2 浮层。
- **第三节「两轮制」→ 落地**：第一轮=量（九个滚动位置/两套主题），评审=判据（两端成对、
  呼吸不滚），第二轮=只改归属，重新出图复核（静止帧 md5 逐字节相同＝没有顺手改别的）。

## preflight 逐条（A / B / C / D2）

- **A 反 AI 套路**：无渐变、无发光、无装饰性边、无居中 Hero、无三等分卡片——本轮**删**了
  一处墨而不是加，方向上与 A 一致。**过**。
- **B 设计读取一致性**：本页读作「命令输出的阅读面」，记忆点是**页头那句结论 + 下面那条
  干净的空气带**；深浅两套各自单主题，没有混；动效为零（滚动是用户手势，不加动画）。**过**。
- **C 动效抽查**：本轮没有新增动效，也没有改滚动行为（`TrackScroll` 原样）；`foldAlpha`
  是逐帧几何量、不是 transition。**过**（无新增项）。
- **D2 中文页**：本页没有中文文案改动（i18n 零改动）。**唯一不能过的一条仍是沿用项**：
  `tokens.Caption` 11px 与 D2「正文 ≥ 14px」冲突——这是**第 9 份证据**，仍列在
  `.pending-bugs.md` 的「需拍板」里，不擅自改（改它会动整页的字号阶梯）。
- **A2 进攻三步**：DNA 3 个值、密度 5 项、两轮制——都在上面逐条指认了。

## V / B / S 台账

**V 0 / B 0 / S 0**。旧 pin 零改动全绿（`outputfold_test.go`、`headingfold_test.go`、
`outputnumbers_test.go`、`outputfresh_test.go`、`wheel_test.go`、`output_pagination_test.go`
一条都没动——视口盒与行盒同源，所以折边的比例仍然自洽）。

## 门槛（实测）

- PASS **3237 → 3241**｜FAIL **0**｜SKIP **125 → 126**｜测试函数 **2392 → 2397**
- `gofmt -l cmd internal` **空**｜`go vet ./...` **净**｜`GOOS=windows go vet` **净**｜
  `GOOS=darwin go vet` **净**｜三平台 `go build ./...` **过**

## 变异（3 条实跑，全部变红并逐字节还原）

- **M1** 列表分支退回旧形状（外层 padding 去掉、列表 `Padding(header, …)`）→
  `TestTheOutputListNeverScrollsIntoTheHeaderBand` 红：`band ... (y=100..115) carries ink
  92`（深 88），`viewport top y=102`；`TestTheOutputViewportStartsOnItsFirstRow` 红：
  `the list's clip starts at y=102 but its first row at y=118`。`md5sum -c` 还原 + `touch` 复跑绿。
- **M2** 折边只在**下沿**生效（`fold < opacity && box.Y >= viewport.Y`）→
  `TestTheOutputFoldRecedesTheSameAtBothEnds` 红：`top: share 0.850 → ink 228（want 193）`，
  下沿仍是 194。还原复跑绿。
- **M3** 正文分支退回旧形状 → `TestTheOutputTextNeverScrollsIntoTheHeaderBand` 红：
  `(y=101..116) with ink 228`（深 221）。还原复跑绿。

## 仍待（P13-ce）

1. **下一格（`空态满态` 的第三块）**：启动器**结果列表**（`launcher.go` `rowBody`）读同一条
   规则，但它的页头是字段行（绝对定位、不透明），滚动体在它下面——**先量**它有没有同一道
   「呼吸被内容吃掉」，再决定要不要照这一轮改。**先量再改**。
2. **需拍板（沿用 + 第 9 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；路径截断保头弃尾；按下档要不要保留那一对线（`pointerFaceAllow` 两条）；
   终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（本轮第七次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
