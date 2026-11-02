# P13-eg 报告 · 启动器最后一批夹具换到出货尺 —— 换尺之后一条红都没有

**一句话**：`internal/launcher` 里最后 13 处自己搭取景的夹具收进 `themedView`，
把 4 处「夹具已经换尺、断言还 quote 库存 `Space()`」的常数换成 `spaceU()`，
并补一条**能分辨两把尺子**的 pin —— 因为换尺本身一条红都没出。

规格：`docs/rounds/P13-eg-spec.md`。合约：`docs/liquid-glass.md` 新增 **§6.102**。
表面 = **测试夹具**（不是界面）。产品代码 **0 行**。

## 先量：换尺之后 0 红

13 处裸 `ui.NewTester(func(c){ a.View(c) }, …)` 换成 `themedView(a)` 后，整包**一次全绿**
（`ok floter/internal/launcher 17.6s`）。原因和 P13-ee 同一条：这一族的断言全是**相对读数**
（`sameColumn` 两个列互比、`HasText`、行序、快照计数、benchmark 计时），
对尺子不敏感 —— 换对换错都绿。

所以本轮真正的交付有三件，换尺只是第一件：

1. 13 处裸夹具 + 4 处地面自钉夹具收口（这一包此后**只有一个取景入口**）。
2. 4 处**藏在出货夹具里的库存尺常数**（下面 §2）。
3. 一条能分辨两把尺子的 pin（§3），否则「换尺」这件事下次还能悄悄退回去。

## §1 13 处裸夹具

`actionbarfooter:915`、`bench:29/41`、`filters:117/1118`、`headingcolumn:37/52/124`、
`headingfold:296`、`snapshotpin:21`、`view:1029`（另 4 处 `dimplate:36`、`plateface:67/275`、
`plate_lens:24` 是「地面自钉」那一类，见 §2）。

**帧高一个没动**：帧高那一族已由 P13-ef 收口；`filters:1118` 的 `+120` 保留，因为它的判据是
「行滚不滚」，动帧高会把判据一起换掉。

## §2 出货夹具里的库存尺常数（这一轮真正的收获）

`render` / `renderIn` 在 P13-ed/ef 已换到出货尺，但四条断言还在 quote **库存**的 `Space()`：

| 位置 | 旧值（库存尺） | 出货尺真值 | 结果 |
|---|---|---|---|
| `litedge:87` 面板 8u 沟 | `int(ui.LightTheme().Space(2))` = 8 | 7.2 | 探针多往外读 1px，仍绿 |
| `filesmarks:147` 同上 | 8 | 7.2 | 同上 |
| `raycast:390` 字段带 56u | `int(Space(14))` = 56 | 50.4 | 采样点低 5px，仍绿 |
| `tipglass:317` 尖端 1.5 空 | `Space(1.5)` = 6 | 5.4 | ±1 容差正好吃掉 0.6px，仍绿 |

**这四条都是「错着也绿」** —— 它们正是那种「看着有 pin、其实量错了」的债务。
换成新助手 `spaceU(n) = designUnit()*4*n`（落在 `view_test.go` 的 `designUnit` 旁边），
换完仍全绿，但读数回到了真值。

`plateface` 的两条探针更硬：它们把盒子写死成 `int(ui.LightTheme().Space(7))` = **28**，
出货尺的板是 `X 47.4 W 25.2` → painter 画 **47..72（26px）**，于是「左右两侧的列与面同色」
这条断言读的是**盒子外面的地面**：

```
the plate's left column at mid height is {246 246 248 255} but its face is {229 229 230 255}
— a 18-level ring is drawn around the whole box
```

修法：探针量**布局给的盒子**（`a.iconPlate(c,…).Bounds()`），再用新 `pixelBox`
（`floor(min)..ceil(max)`，painter 的格子）落到整数像素；悬停小格的 20u 从
`Space(5)` = 20 换成 `designUnit()*20` = 18。四条 pin 全部重新变绿 —— 而且这次量的是**真的那块板**。

`dimplate` / `plate_lens` 补了 `tt.SetDark(...)`：`a.tokens(c)` 读的是 `c.Theme().Dark`，
不切就是把深色的面合成在白纸上（P13-ee 的教训）。

## §3 能分辨两把尺子的 pin

`TestTheSectionHeadingStartsAtTheShippedTextColumn`（`headingcolumn_test.go`），两臂各钉一把尺：

| 尺 | heading X | heading Y | heading H | title Y | title H |
|---|---|---|---|---|---|
| 库存（裸 `ui.NewTester`） | 54 | 66 | 22 | 100 | 24 |
| 出货（`themedView`） | **48.6** | 59.4 | **17** | **88.5** | 19 |

`48.6 = 54u × 0.9`，`17` 是出货尺的 caption 行盒（`designMetrics`）。
第二臂把库存的数留在文件里：谁把夹具改回裸 `ui.NewTester`，第一臂立刻读 54/22 并报错。

## 台账

- **V 6**：`litedge:87`、`filesmarks:147`、`raycast:390`、`tipglass:317` 的常数；
  `plateface` 的盒子（写死 28 → 布局盒）；悬停小格 20 → 18。
- **S 0**：结构没变（取景入口的收口已在 P13-ef 完成，本轮只是把最后的使用者接上）。
- **B 0**：没有一条行为 pin 变红。

## 门槛（本轮实测）

```
gofmt -l cmd internal                                   → 空
go vet ./...                                            → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...    → 净
go test -count=1 ./...                                  → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       → 3334（HEAD 3333，+1）
                                                    SKIP → 166（HEAD 165，+1 = opt-in 出图）
测试函数（git grep "^func Test"）                        → 2506（HEAD 2504，+2）
三平台 go build                                          → 过
```

## 变异（3 条，全部实跑变红并还原，`md5sum -c` 逐字节核对）

- **M1** `pixelBox` 把盒子四周各外扩 1px（= 退回「28u 那个盒子」）：
  `TestTheIconPlateIsAFaceNotARing` 红 ——
  `the plate's left column at mid height is {246 246 248 255} but its face is {229 229 230 255}
  — a 18-level ring is drawn around the whole box`（light/dark 各 2 条，共 4 条），
  外加 `the plate's crown {246 246 248 255} is nearer the floating EdgeHighlight`。
- **M2** 悬停小格退回 `int(ui.LightTheme().Space(5))`：
  `TestTheHoveredRowControlIsAFaceNotARing` 红 ——
  `the hovered 20u control's left column at mid height is {246 246 248 255} but its face is
  {220 220 222 255} — a ring is drawn around the box`（light/dark 各 2 条）。
- **M3** 新 pin 的出货臂退回裸 `ui.NewTester`：新 pin 红 ——
  `the section heading starts at x=540 on the shipped ruler, want 486 (54u)`、
  `the first row's title starts at y=1000, want 885`、
  `the heading's line box is 220 tall, want the shipped caption line 170`。
- 还原后 `md5sum -c` 四个文件全部「成功」。

## 出图（自己复核过）

`/tmp/floter-P13-eg-plate-{stock,shipped}-{light,dark}.png`（120×80，`FLOTER_PNG=1` 的
`TestDumpP13EGPlateRulerPNGs`，把**两把尺子**各画一帧），
另 `/tmp/floter-P11-ac-*.png` 两张标题列对照。

**这张图上看到的一处变化**：同一块图标板，库存尺 `layout {X:46 Y:26 W:28 H:28}` →
painted **46..73**；出货尺 `layout {X:47.4 Y:27.4 W:25.2 H:25.2}` → painted **47..72**。
板**小了一圈（28 → 26 画出来的像素）、右下挪了 1.4px**，而 E1 的签名一个值都没变：
浅色 `crown {235 235 236}` / `face {229 229 230}` / 左列 `{229 229 230}`（= 面，无环）；
深色 `crown {69 70 74}` / `face {51 52 56}` / 左列 `{51 52 56}`。
也就是说：§2 的「面 + 一条受光边、没有环」在**用户真看见的那个尺寸**上成立，
此前它是在一个 28px 的盒子上被验证的。

## 用了 qiaomu-design（`craft-loop.md` + `preflight.md`，已有方向只要打磨那条路）

**偷了什么**（逐条：菜单项 → 落成的具体数值）：

1. **工艺密度第 12 项「材质细节：内边框高光」** → 受光边的**验证尺寸**从 28px 的库存盒
   改到 25.2px 的出货盒（painted 26px），并要求 `crown` 在**同一块面**上仍严格亮于
   `face`：浅 235 > 229、深 69 > 51。材质断言不能在一张用户看不到的卡上成立。
2. **第 5 项「多层染色阴影 / 面档」的度量前提** → 地面自钉的四个夹具里，
   `ground` 与 `face` 的差是这条 pin 的全部信号（浅 246 vs 229 = 18 级）；
   探针一旦多读 1px，读到的是地面而不是面 —— 本轮把「盒子」从常数换成布局读数，
   就是为了让这 18 级差落在**面**上。
3. **第 6 项「编辑细节：hairline」** → 8u 沟、56u 字段带、1.5u 尖端空三处
   从库存尺的 8 / 56 / 6 换成出货尺的 7.2 / 50.4 / 5.4（`spaceU`）；
   hairline 的**位置**比 hairline 的**颜色**更容易写错，因为写错 1px 仍然绿。
4. **三·两轮制（渲染 → 评审 → 只修 3-5 处）** → 第一轮只换尺，跑出 0 红；
   评审结论是「0 红 = 这一族量的是相对关系」，于是第二轮只做三件事：
   四条常数、两条探针的盒子、一条分辨尺子的 pin，没有顺手大改。

**preflight 逐条结论**：

- **A（反 AI 套路）**：通过/不适用 —— 本轮不产界面，没有渐变、glow、斜体，
  也没有用「渐变白边冒充玻璃」（`liquid-glass.md` §0 的硬禁令未被触碰）。
- **B（设计读取一致性）**：通过 —— 一个仓库一个调色板来源（`internal/theme`）；
  本轮把最后 13 个绕过它的夹具收回 `themedView`，并用 `git grep` 证明这一包只剩一个
  取景入口（`themedView` 是唯一装主题的地方，`designMetrics` 的探针除外）。
- **C（动效抽查）**：不适用 —— 不动动效；`dimplate` / `plate_lens` 新增的 `SetDark`
  不带 `Advance`，因为这两条读的是几何与静态像素，不是颜色过渡（P9-c 的教训只对读颜色的 pin 成立）。
- **D2（中文页）**：**未过（沿用，第 56 份证据）** —— `tokens.Caption` 9.9px / 正文 10.8px
  对「正文 ≥ 14px」。本轮没动字号；仍在 `.pending-bugs.md` 的「需用户拍板」里。

## 仍待（P13-eg）

1. **下一格（首选）**：回通用队列的**细节打磨**——`状态反馈` 的两半（禁用面 §6.38/§6.63、
   只读面 §6.84）都已落地，剩下的可见缺口是**空态/满态**：扩展页空态只有一行结论没有出路、
   会话页空态说错对象 —— 这两条都写着「需拍板」（文案/交互语义），**不能擅自改**。
   所以下一格取**不需要拍板**的那一半：满态（列表撑满/结果被截断时的计数与折边反馈）
   在 `internal/launcher` 与 `internal/settingsui` 各量一次，先量再改。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px 对 preflight D2「正文 ≥ 14px」（第 56 份证据）/
   便条行 rank 1.28 / 终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. 抢救 stash（第六十次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）仍判定与
   HEAD 无未接续内容，本轮未试合、未静默丢弃。
