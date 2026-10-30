# P13-d 报告：终端页表面（`docs/liquid-glass.md` §6.3）

> 视觉队列第 3 片（§6.1 启动器 = P12-b、§6.2 设置页 = P12-c）。规格
> `docs/rounds/P13-d-spec.md`。**一个表面一轮**：这一轮只动终端页。

## 1 意图与「为什么现在」

`docs/liquid-glass.md` §6 是唯一权威选题队列；§6.3 全行 `未开始`。P13-a/b/c 那弧
（终端会话宿主）下一片 S1b **改会话管理语义 = 需用户拍板**
（`.pending-bugs.md`「需用户拍板」P13-a 第 1 条），所以按选题纪律（0 → 1 → 2 → 3）
回到排在弧之后的视觉队列，领 §6.3。

**这轮为什么让它更轻 / 更有空间感**（视觉轮必答的一句话）：

> 终端页此前只有**两档**在场——底（面板 + 画布，两者都是 E0）与一条**四面环的控件透镜**
> （悬停的图标按钮）；中间那档**面**（E1）在这页上一次都没画过，于是画布、抽屉、键帽、
> 浮条全挤在同一个 z 平面上。而唯一像「浮」的那件东西（悬停的标题条）又只压了一道
> **紧贴边的重影**，读起来是「底下加深一块」而不是「浮起来一张纸」。
> 本轮把 E1 的受光边接到**抽屉的缝**与**空态键帽**上、把 E2 的**两层影**接到浮条上，
> 并把图标按钮那个四层环退成一条受光边——**两档之间的差值第一次出现在这一页**。

## 2 落地项（位置 → 现状 → 改成 → 状态）

行号是动手前实测重核的（不是旧报告里的值）。

| # | 位置 | 现状（旧） | 落成（新） | 状态 |
|---|---|---|---|---|
| 3-1 | 面板 `terminalui.go:870` | E0：`panelCorner` + `panelFill` 一层面，平台自己的 `panelEdge` 是它的框 | **不动**；两条几何 pin（`panelCorner`=12、`mountCorner`=10）继续钉 | `已落地`（P11-z 已守） |
| 3-2 | 会话挂载块（画布）`terminalui.go:944` | E0：一层填充，P11-z 删掉了那圈 `Border(1, edge)` | **改判为「故意不做」**，见 §3 | `故意不做`（附理由） |
| 3-3 | 外观抽屉的缝 `drawer.go:113` | `HairlineSoft`（0.055）一条分区线 | **E1 的 `LensRim`**（0.09）；底对侧不补 | `已落地` |
| 3-4 | 标题条 hover 影 `terminalui.go:1498-1499` | 宽几何（`0 6px 16px`）+ **紧贴边**墨 `ShadowContact`，一条 | 宽影→`ShadowAmbient` **+** `0 1px 2px` 的 `ShadowContact`（§2 判据 3：E2 两层影） | `已落地` |
| 3-5 | 底部常驻便条 / 顶部失败条 | 整宽带子 | **不动**（P11-bc/bg 定审：贴边的带子不是浮层） | `故意不做` |
| 3-6 | 状态灯 | 1.75u 中性点 + 呼吸 | **不动**（1.75u 画不出受光边） | `故意不做` |
| 3-7 | **新增**｜悬停的图标按钮 `terminalui.go:1424` | `Lens(button)`：四层，第四层是**四边 `ControlEdge` 的环** | **`LitFace(button)`**：顶受光边 + 底对侧暗线，**没有环**、也没有那片宽 sheen | `已落地` |
| 3-8 | **新增**｜空态键帽 `terminalui.go:986` | 只有**一半**受光边（顶边一条 `LensRim`），浅色档白压白 | `tokens.LitFace(badge)` —— 与 P12-b 给启动器键帽的是**同一个函数** | `已落地` |

**3-7 顺手收掉的一条过期注释**：那一处原文写着「The lens is the window's rim at
≤0.04 alpha, below the §1 gloss cap」——**那句话已不成立**：P12-a 把 `LensRim` 重定到
0.09（§1 末尾的旧值→新值表）。本轮把注释与代码一起改到新代。

**3-8 的落成尺寸是量出来的**（不是照抄算术）：键帽 20u 高、1px 外圈 `ControlEdge`，
`LitFace` 的两条内线落在**边框内**的第一行与最后一行。实测（`light`）：
外圈 216 → 顶内线 **232**（亮 16 级）→ 面 230 → 底内线 **216/221**（暗 9~14 级）。
旧版没有最后那条，所以键帽底边与行底同色（230 = 230）。

**改判 3-2 的理由**（两条，写进合约 §6.3）：

1. 画布**是**这一页的**底**（终端内容面），不是浮在底上的块；§2 的判据「它长在底上，
   压不出影子」说的是**块**，而画布与面板之间不是「块放在底上」，是同一张面换个色号。
2. P11-z 刚按用户实测 F3 退掉的就是这一页的**第二层框**（`Border(1, edge)`，聚焦时
   还是 accent 红）。给画布补受光边 = 在 F3 的同一处加回第二层分界，只是换了颜色——
   那正是用户读到的「类玻璃边」。

## 3 V / B / S 台账

- **V（视觉数值）1 条**：`internal/terminalui/raycast_test.go` 的
  `TestTheTitleBarHoverShadowIsShadowContact`。旧值「一条宽 `ShadowContact`」是本支
  要退掉的**旧设计**，所以它从「被钉住的那条」翻成「必须偏离的那条」。pin 的**结构**
  （参照帧逐像素相减）与那条「不是字面值」的断言**一字未改**，只是参照从一条变成
  **一叠**（`probe(...throws)`），并按新契约加了两条：`toBePair == 0`（活浮条 = token
  两层影）与 `toBeOld != 0`（不许退回单层）。注释写明 `P13-d 重定基线：旧值 → 新值`。
- **V（视觉数值）1 条**：`internal/shell/glasscensus_test.go` 的 `.toolbar-button:hover`
  条目。它原本钉「那个四层透镜调用必须还在」，现在按 §6.3 第 3-7 行翻成 `dropped`
  （「那个调用不许回来」），并把新面指给 `terminalui.TestTheHoveredBarButtonIsAFaceNotARing`。
  **这是同一张 census 的既有机制**（同文件里 `.settings-sidebar__item:hover` 与
  `.settings-section__card` 两条早就是 `dropped`），不是新增豁免。
- **B 0 条**：其余 2131 条测试函数**零改动**全绿（`TestTheDrawerSeamIsOnItsTopEdge`、
  `barshadow_test.go`、`residentstrip_test.go`、`feedbackstrip_test.go`、两条几何 pin
  全部原样通过——缝的颜色变了，位置、宽度、只有一条这些结构没变）。
- **S 0 条**。
- 新增 pin **4 条**（`internal/terminalui/litedge_test.go`，每条在深/浅两套外观上跑）+
  **1 个 opt-in dump**（`FLOTER_PNG=1`）。

**新 pin 怎么量**（与 P12-c 同一个手法，不猜渲染器的合成算术）：把两条候选线各自
**源叠加**在同一个面上当参照，要求被量到的那条离它「应该」那条更近；并且先断言两条
候选本身分得开（`< 3` 级就 `Fatal`），否则这个比较永远不会红。**pin 里没有一处写死
渲染器合成出来的像素值**。

## 4 门槛（全部实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿，0 FAIL
GOOS=linux/windows/darwin go build ./...                → 过
```

| 量 | 基线（P13-c 实测，本轮 stash 掉改动重跑确认） | 本轮 | 差 |
|---|---|---|---|
| PASS | 2869 | **2878** | +9 |
| FAIL | 0 | 0 | 0 |
| SKIP | 61 | 62 | +1（新的 opt-in dump） |
| `func Test*` | 2131 | **2135** | +4 |

（`func Test*` 的基线用 `git stash --include-untracked` 清树后重数，与本轮工作树逐个 `^func Test[A-Za-z0-9_]+\(` 匹配对比。）

## 5 变异（4 条实跑全红 + `md5sum -c` 逐字节还原 + `touch` 复跑绿）

| # | 注入 | 变红的 pin | 原始报错（抄自 `/tmp/p13d-mutations.log`） |
|---|---|---|---|
| M1 | 浮条 hover 影写回**单条宽 `ShadowContact`** | `TestTheTitleBarHoverShadowIsShadowContact` | `11200 pixels under the bar differ from the E2 pair (ShadowAmbient + ShadowContact); liquid-glass.md §6.3 第 3-4 行 / §2 判据 3 names the two layers` + `the bar's hover shadow is byte-identical to the single-layer ShadowContact throw the previous generation painted: the contact layer is missing (§2 判据 3)` |
| M2 | 抽屉的缝写回 `HairlineSoft` | `TestTheDockedDrawerWearsTheE1LitEdge`（light+dark） | `[light] the drawer's seam {217 217 217} is not brighter than its own face {230 230 230}: the E1 lit edge is missing` + `[light/dark] the drawer's seam … is nearer the old weak hairline than the E1 lit edge` |
| M3 | 图标按钮写回 `Lens` | `TestTheHoveredBarButtonIsAFaceNotARing`（light+dark） | `[light] the button's left column at mid height is {215 215 215} but its face is {228 228 228}: a ring is drawn around the whole box` |
| M4 | 空态键帽写回「只有顶边」 | `TestTheEmptyKeycapWearsTheE1Pair`（light+dark） | `[light] the keycap has no counter-side: its last inner row {230 230 230} is not darker than its face {230 230 230}` + `counter-side = {230 230 230}, want LensEdge over its face ({216 216 216})` |

还原核对：`md5sum -c /tmp/p13d-md5.txt` → `internal/terminalui/terminalui.go: 成功` /
`internal/terminalui/drawer.go: 成功`（**exit 0**）；随后 `touch` 两个文件再复跑
4 条 pin → `exit=0`（避开测试缓存）。

## 6 出图（自己用眼睛复核过）

`/tmp/floter-P13-d-drawer,bar,empty}-dark,light}-before,after}.png`（before = 生产改动
`git stash` 掉、只留未跟踪的 dump 跑一次；after = 还原后再跑）。

**这张图上看到的一处变化**（`bar` 那一对，x=450 逐行读数）：

- 浅色：带下第一行 `(231,231,231)` → `(193,193,193)`，第 9 行 `(245,245,245)` →
  `(238,238,238)` —— 阴影**在紧贴的那几行明显更重、衰减更快**，尾端却更干净：
  从「一团摊得很开的灰」变成「贴着边一笔 + 往外一笔淡的」。
- 深色：`(16,16,18)` → `(8,8,10)` 在紧贴那行，第 12 行 `(22,22,25)` → `(21,21,24)`
  —— 同一条带**有了接触影**，它此前在深色档下几乎看不出浮起来。

另两处（同一批图）：

- `drawer`：整条缝**逐行只变 1 个像素行**（`y=384`，全宽 900 px）——
  `(217,217,217)` → `(232,232,232)`（浅）/ `(51,51,52)` → `(58,58,59)`（深）。
  这正是「**只换墨、不动几何**」在像素上的读数。
- `empty`：两处 86/82 个像素的变化，**全在键帽的底边内侧那两行**
  （浅 `(230,230,230)`→`(221,221,221)`→`(216,216,216)`；
  深 `(39,39,40)`→`(38,38,38)`→`(48,48,49)`）；顶边那两行**一个像素没动**。
  键帽第一次有了受光边的对侧。

## 7 偷了什么（`craft-loop.md` 工艺密度菜单，逐条落成具体数值）

按任务书要求，本轮的「偷」全部来自 `references/craft-loop.md` 的**工艺密度菜单**，
每一项都落成仓库里可指的数值（不是「参考了某种感觉」）：

| 菜单项 | 落成 |
|---|---|
| **#5 多层染色阴影**（「2-3 层、染底色色调，拒绝单层灰阴影」） | 浮条 hover 影从**一条**变**两条**：`0 6px 16px ShadowAmbient`（环境影）+ `0 1px 2px ShadowContact`（接触影）——两层各自读 token，随外观与框透明度缩放（`theme.ElevThrow` 那条链）。 |
| **#12 材质细节**（「内边框高光、glass 折射边」） | 抽屉的缝 `HairlineSoft`(0.055) → `LensRim`(0.09)：一条**内**受光边，不是画框（外圈描边零改动）。空态键帽补 `LensEdge` 对侧——**顶亮一分、底暗一分**这个「让块站起来」的机制就来自这一项的「内边框高光」。 |
| **#10 收尾彩蛋**（「hover 才出现的细节」） | 悬停的图标按钮：四层环 → `LitFace` 一条受光边 + 一条对侧暗线。它只在指针停上时出现，是这一页最小的一处「细节」，也正好是 §2 要退掉的那一层。 |

`craft-loop.md` 的**两轮制**：第一轮（上述四处落地）→ 出 6 张图 → 四视角评审
（下节）→ 第二轮只修评审挑出的问题。

### 四视角评审（各挑一处，写下来）

- **艺术总监**：三档里最平庸的一处是 **3-2 画布**——它至今是「一片纯色」。**没有改**，
  理由见 §3 的改判：在这一页给画布再加一条线，就是把 F3 的第二层框加回来。
  真正该补的是「底 / 面 / 浮」中**缺的那一档**，所以补的是 3-3 / 3-7 / 3-8。
- **工程师**：M2 注入时我实测到浅色档的缝 `217 < 230`（**比自己的面还暗**）——
  也就是说 `HairlineSoft` 在浅色档上读起来是**一条凹槽**，不是一条缝。
  这条读数写进了新 pin 的第一条断言（`litLighter(seam, face)`）。
- **甲方**：三秒看完，这页记住的是「抽屉与画布之间那条线亮了」——以前它与
  「分组卡之间的发丝」是同一条（0.055），现在它是**块**的边（0.09）。
- **读者**：眯眼通读——**没有问题**（本轮不加字、不动字号、不动对比度；`canvasInks`
  的可读性地板 `barFaceContrast = 4.5` 一个字节没动）。

## 8 preflight（`references/preflight.md`）逐条过

- **A 反 AI 套路**：通过。这一轮**不产生任何新元素**——只改三条线的墨与一条影的层数。
  无渐变、无 glow、无斜体、无新文案、无新卡片。**A 节里唯一相关的一条**是
  「无左侧竖线装饰」（P-38）：本轮没有加任何竖线；抽屉那条缝是**横**的，且它本来就存在。
- **B 设计读取一致性**：通过。设计读取 = 「已有方向（轻便液态玻璃），只要打磨」，
  走 `craft-loop.md` + `preflight.md` 的**打磨模式**；**未走** Phase 2 的四方向预览
  （任务书写死：方向已由用户给死，自动车道无人点选，**不许出 HTML 预览页、不许停下等**）。
  记忆点一句话：**「同一个 z 平面上的两档，第一次分开了」**。
- **C 动效抽查**：**部分不适用**——本轮**没有新增任何动效**（无 transition、无 keyframe、
  无入场）。相关的既有动效一条没动：行/按钮的 90ms 颜色过渡、E2 浮层 160ms 入场、
  点的 30fps 呼吸（`AnimationFrameRate`，只重画不重建）、`ReduceMotion` 兜底。
  新 pin 里显式 `Advance(300ms)` 越过 90ms 过渡窗再读数——**这是本轮在动效上的唯一动作，
  且它是一次「等它落定」的收尾，不是新动效**。
- **D2 中文页追加项**：通过（**无法不适用**——终端页本身是英文界面，但本轮新增的
  注释与报告是中文）。中文行高、字号、字重（400/500/600）本轮零改动；**中西文之间、
  数字与单位之间留空格**这一条已在本轮新增的注释里遵守（如「`LensRim`(0.09)」
  「旧值 → 新值」）。**没有**引入任何装饰中文字体。
- **A2 进攻三步核验**：DNA 注入 = 上表三条（全部来自 `craft-loop.md` 的菜单，逐项落成
  数值）；工艺密度 = 3 项（低于「≥5」的营销页门槛，**如实记**：本仓是终端类功能页、
  不是营销页，`SKILL.md` 的拨盘表给功能页的 `DENSITY` 是 5-7，而**一轮只做一个表面**
  的任务书纪律把可落项压到 3 项——下一轮领 §6.4 时这条会补齐）；两轮制 = 已发生（§7）。
- **A2.1 执行者路由**：通过。当前 Codex 直接执行，**未调用 K3**（用户本任务未点名，
  P-46 生效）。
- **A5 预览交付门**：**不适用**（任务书明令不出四方向预览页，理由见 B）。

## 9 仍待（追加在 P13-c 之后）

1. **S1b（仍等拍板）**：`terminalui` 接 `Dial`；`CloseAll` 从「退出即杀」变 detach。
2. **S1c（Windows ConPTY）**；`Alive` 在 Windows 恒 false。
3. **客户端不告而别时宿主怎么知道**（P13-b 遗留）。
4. **capture 通道迁移**（P13-c 的拍板项）。
5. **视觉队列下一站 = §6.4 扩展页**（4-1 集成行面 / 4-3 扩展动作按钮受光边复核；
   4-2 已判「故意不做」）。§6.5 文件页、§6.6 toast 两层影在其后。
6. **本轮的一处观察**：§6.3 第 3-2 行从「未开始」改判为「故意不做」，
   `docs/liquid-glass.md` §6.3 现已**空表**（除两条早就判 `故意不做` 的）。
