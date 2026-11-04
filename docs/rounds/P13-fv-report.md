# P13-fv · 满态：会话页 22 条会话，页尾那条软边淡错了颜色

规格 `docs/rounds/P13-fv-spec.md`。合约新增 **§6.141**。

## 先量

帧：720×760，`renderIn`（装了 tokens 主题，与生产一致），22 条会话（`Dir` / `Note` / `Size`
齐全），深浅各一帧。

| 量 | 浅 | 深 |
| --- | --- | --- |
| `Body.MaxY` | 931.5 | 931.5 |
| 行距（21 个间隔逐值相同） | **67.2**（= 16.8u，u=4） | 67.2 |
| 分隔线 y | 233 / 301 / 368 / 435 / 502 / 570 / 637 / 704 | 同 |
| 分隔线墨 / 卡面 | 208 / 229 | 55 / 39 |
| 卡顶：边 / 冠 / 面 | 216 / 235 / 229 | 54 / 58 / 39 |
| 滚到底：卡下沿 | 712..713 = **216** | 同上（亮线） |
| 滚到底：地面 | 246（从 714 起） | 15 |
| 滚到底：软边 744..759 | **246 → 247** | **15 → 14** |

**缺陷**：软边把**窗口地**（浅 247 / 深 17,18,20）铺在**面**（`FaceOver(地, Sunken)` =
246 / 15）上。空面上滚到底时，页尾凭空多出一条 16px 的带子（浅 +1 级）。
在**不装主题**的帧（`render()`，很多旧 pin 走它）上同一处是 **246 → 252（+6 级）**，
因为那时 `t.Background` 拿到的是 `ui` 默认主题的白 255——这就是这个缺陷最初被看见的地方。

同族的另外四处早就按「面」给色：`sheetedge.go`、`config.go:932`、`pluginsheet.go:172`、
`terminalui/drawer.go:160`。只有设置页正文这一处给的是**地**。

## 落地

`internal/settingsui/settingsui.go`（`body`）：

```go
face := theme.FaceOver(a.tokens(c).Theme.Background, a.tokens(c).Sunken)
... Material(glass.ScrollEdge{Background: face})              // 上沿
... Material(glass.ScrollEdge{Bottom: true, Background: face}) // 下沿
```

盒子、`Padding`、`MaxY`、`edge` 的取值**逐值不变**——只换了一个颜色。上沿同时受益：
它此前也是把内容推向窗口地。

## pin（`internal/settingsui/fullstateedge_test.go`，+1 函数 / 2 子测试）

- **141-1** 22 行、21 个间隔，逐值 67.2（±0.01）；缺一行就红。
- **141-2** 滚到底时卡的下沿仍在带子之上（列 x=660 上有一行离面 ≥6 级，且距带子 ≥2px）。
- **141-3** 滚到底时带子里的每一行**逐值等于面**（不给容差——给 ±1 会让「淡向窗口地」
  这种一等级的错误从网眼里漏过去）。
- **141-4** 软边仍在淡内容：同一段字形在带子里（`spread`）比滚出带子后更小。
- 前置：`MaxY > 0`（否则这一帧没有满态，四条判据都会在一条不滚动的列表上通过）。

opt-in dump：`TestDumpP13FVFullSessions`（`FLOTER_P13FV_DUMP=1`，`FLOTER_P13FV_TAG=before`
配「改回 `t.Background`」出对比对）。

## 台账

- **V**：1（141-1…-4 都是新增 pin；141-3 的期望值是本轮量出来的面 246 / 15，无旧值可改）。
- **B**：0（高度链、`MaxY`、键盘、会话管理、i18n、接线一条没碰）。
- **S**：0。

## 变异（实跑）

- **M1** `face := t.Background`（把缺陷放回去）→ **两个臂全红**，原文：
  `at the end of the scroll the band's row 754 reads 247 against a plane of 246: the edge is painting
  its own colour over the page's bottom padding (want the plane, exactly)`；
  `… row 754 reads 14 against a plane of 15 …`。
- **M2** 删掉下沿那个 `ui.Box(...).Material(glass.ScrollEdge{...})` → **两个臂全红**，原文：
  `the same glyphs read at 202 inside the band and at 202 once scrolled out of it: the band is not
  fading content`（深色 198 对 198）。
- 还原：`md5sum -c` → `8d8bce23d58f8f79631330d7b8c053ee` **逐字节相同**，`touch` 后复跑全绿。

## 门槛（实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → **净**
- `go test -count=1 ./...` → **全绿**；**3431 PASS / 0 FAIL**（基线 3428）
- `GOOS=linux/windows/darwin go build ./...` → **三个都过**
- 测试函数：**2609**（基线 2607，+2 = 本轮 pin 与 dump）

## 出图

`/tmp/floter-P13-fv-sessions-{light,dark}-{before,after}-y{00,50,100}.png`
（`FLOTER_P13FV_DUMP=1`；`before` = 把 `t.Background` 放回去那一帧）。

**图上看到的一处变化**：`y100`（滚到底）这一对——`before` 的最下 16 行从 246 一路走到
**252**，页尾浮出一条比页面本身更亮的雾带，卡的下沿（712..713）与它之间那片空白被这条带子
分成了两段颜色；`after` 里这 16 行**逐值 246**，与卡下方那片空白连成一片，页尾就只是页尾。

## 这轮为什么让它更「轻」/更有空间感

轻便液态玻璃里，**软边是「面」的属性，不是「窗口」的属性**：它说的是「这一块面的内容在这里
退场」，退场的终点必须是这块面自己。淡向窗口地，就等于在面的边缘又画了一层**比面更亮的东西**
——用户读到的是一条 16px 的雾带，而不是「内容淡出了」。空间感是靠**面**说话的：卡是面、
卡下那片空白是底、软边只是面与底之间的过渡。把软边的落点从「地」改成「面」之后，滚到底的
那一帧只剩三样东西——卡（229）、面（246）、卡的下沿（216）——没有一个多余的第四层。

## 偷了什么（qiaomu-design，`craft-loop.md` 工艺密度菜单）

- **第 9 项「层与层的过渡：淡出要落在目标面上」** → 落成 `face := theme.FaceOver(...)` 与
  141-3「逐值等于面，不给容差」。
- **第 3 项「间距节奏：一行一个数」** → 落成 141-1：21 个间隔逐值 67.2，行距是列表自己的
  数（16.8u），不是卡高除出来的余数。
- **第 11 项「边界的可见性：框不许被过渡吃掉」** → 落成 141-2（卡下沿仍在带子之上）。

## preflight 逐条

- **A 反 AI 套路**：无渐变白边、无模糊贴图、无 emoji、无斜体。本轮**减少**了一层视觉
  （页尾那条雾带），没有新增任何装饰。
- **B 设计读取一致性**：读的是 `docs/liquid-glass.md` §1（材质：底 / 面 / 浮）、§4（软边）
  与 `theme.FaceOver` / `tokens.Sunken` 的实际值（浅 `Sunken = {243,244,247,0.17}`，
  `FaceOver(247) = 246`；深 15）。pin 里的数字与合约表逐项对得上。
- **C 动效抽查**：不涉及动效（`ReduceMotion: true` 取帧）。
- **D2 中文页**：本轮不产生文案，`i18n` 未动；`tokens.Caption` 9.9px / 正文 10.8px 与
  preflight「正文 ≥ 14px」的冲突仍在待拍板表里，未在本轮处理。

## 仍待

1. **下一格（首选）**：`140-5`（同一条顶边在两种外观下读成相反的东西）仍是唯一挡在
   像素细节前面的**需拍板**项；若拍板「不改」，则下一格 = 满态的**另一半**：
   集成页命令列表 20 条以上（与本节同族：长列表 + 软边 + 卡下沿）。
2. 本轮**无新增需拍板项**。附带记一条「故意不做」：会话行在 `Dir`/`Note`/`Size` 齐全时
   内容自身高 67.2u，`sessionRowFloorPx = 56` 这一档**不生效**（它是地板，不是高度）——
   不为此改地板，地板的意义是「内容更矮时也不许塌」。
