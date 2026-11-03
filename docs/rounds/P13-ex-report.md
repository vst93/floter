# P13-ex 报告 · 抽屉的满态下沿：这一族最后一把刀

规格 `docs/rounds/P13-ex-spec.md`，合约新增 `docs/liquid-glass.md` §6.119。
产品代码 **1 处**（`internal/terminalui/drawer.go`：一个 `ui.Box` + 一段注释），
新 pin 2 条 + opt-in 出图 1 条。

## 一句话：这轮为什么让它更轻、更有空间感

滚到中间时，网格的最后一行不再在窗口底边被一刀切断，而是**在条带自己的面上淡出去**
——内容于是读成「从一块面底下过去」，而不是「在一条线上被裁掉」。同一族（§6.16–§6.19、
§6.113、P13-as）此前只剩这一块面还在刀切。

## 落地

`internal/terminalui/drawer.go` 的 `a.drawer`，条带之后：

```go
ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(t.Space(1.5)).PassThrough().
    Material(glass.ScrollEdge{Bottom: true, Background: theme.FaceOver(panel, tokens.Control)})
```

三个数各有出处：

- **5.4（`Space(1.5)`）**：条带自己的 `Padding(t.Space(2.5))` = 9，于是
  `软边 5.4 + 净空 3.6 = 9`（`Space(1.5) + Space(1) = Space(2.5)`）。内边距长在**高度链**
  上（抽屉高 → 画布高 → PTY 行数），视觉轮不许动它，所以门改开在几何上：滚到底时
  5.4 的边整个落在 9px 的空白里，空白上没有东西可淡，边自己消失。
- **不透明的面**：`Control` 是一层 0.07 的薄罩，`ui.Color.Alpha` 是**乘**——直接交给
  `ScrollEdge` 会把 0.85 乘成 0.06（§6.17/§6.18 同一条）。面板的面又是调色板底色乘
  `CanvasAlpha`，所以 `FaceOver(FaceOver(地, 面板), Control)` 整条链一起压。
- **`PassThrough`**：不吃指针，抽屉里的行与关闭钮一个都没被盖住。

## 实测（640×280，抽屉溢出 `MaxY=54.4`，深浅各一遍）

九档滚动位置（`Y = MaxY·k/10`）上，窗口最后一行（y=279）的墨展开度：

| 外观 | 改前最坏 | 改后最坏 | 同批帧边上方那一带 |
|---|---|---|---|
| 浅 | **147** | **35** | 169 |
| 深 | **216** | **51** | 217 |

滚到底（`Y = MaxFloat32`）：底部 5 行两套外观**逐行 0**——边在空白上一像素都没留。

## 出图

`/tmp/p13ex/{before,after}/floter-P13-ex-drawer-{light,dark}-{mid,end}.png`
（`before` 用 `git worktree` 在 HEAD `6f16cf1c` 渲，同一个测试文件拷过去跑）：

| 帧 | diff | bbox |
|---|---|---|
| light mid | **108 px** | `(25,276)-(627,279)` |
| dark mid | **108 px** | `(25,276)-(627,279)` |
| light end | **0 px** | — |
| dark end | **0 px** | — |

`mid` 的 bbox 正好是 280 高的窗口最后 4 行（276–279）= 5.4px 的边落成的那条带；
`end` 两张**逐字节相同**，证明这层边是几何自门控的，不会在空白上留下一道灰带。
**这张图上看到的一处变化**：mid 帧里窗口最底那 4 行、原来能读到字的笔画，现在与条带
自己的面同色——字在那儿「淡出去」而不是「被裁掉」。

## V / B / S 台账

**V 0 / B 0 / S 0**。`internal/terminalui` 全套零改动全绿（含 `TestTheDrawerScrollsItsLastRowsIntoView`、
`TestTheDrawerDoesNotScrollWhenItFits`、`drawerband` / `drawerseam` / `drawermetrics` 一族）。
`MaxY` 在 640×280 上仍是 **54.4**（pin 里钉住，高度链零改动）。

## mutation（实跑，`md5sum -c` 逐字节还原 + `touch` 复跑）

- **M1** 边高 → `0`（等于回到改前）：
  `light: the drawer's last pixel row reads 147 levels of ink (limit 70) while the rows above it carry 169` /
  `dark: … reads 216 levels … (limit 70) … carry 217` —— 第①条红，第②条绿（没有边，当然没有带）。
- **M2** 去掉 `Bottom: true`（边画到条带上沿去）：`light: … reads 135 levels … (limit 70) … carry 169`，红。
- **M3** 把 `ControlEdge`（一条发丝的颜色）交给 `ScrollEdge`：**全绿**——这两条 pin 钉的是
  「下沿退后 / 到底不留带」这一对**行为**，不钉色号。色号由 `FaceOver` 从条带自己的
  `Control` 推出来（一个来源），所以没有为它再写一条 pin；这是本轮**已知的覆盖边界**，
  写在这里而不是装作没有。

## 用 skill 的账（`qiaomu-design`）

开工前读了 `SKILL.md` → `references/user-preferences.md`（335 行）→ `craft-loop.md` →
`preflight.md`；按本轮表面（一个滚动容器的下沿）只读这四份，没有全库通读。

**偷了什么**（逐条落成具体数值）：

1. **`craft-loop.md` 工艺密度菜单第 12 项「材质细节 · 内边框高光 / glass 折射边」** ——
   落成 `glass.ScrollEdge{Bottom: true}` 的 0.85→0 的软边（不是渐变白边，不是模糊贴图），
   高度 5.4px，色号 `FaceOver(FaceOver(地, 面板), Control)`。
2. **同菜单第 5 项「多层染色阴影」的同一手法（多层合成而非单层灰）** —— 落成面的
   三层合成链 `t.Background → panelFill → tokens.Control`，而不是把 `Control` 直接交给
   `ScrollEdge`（那会乘成 0.06，边淡了个寂寞）。这条是 §6.17/§6.18 已经立过的反直觉，
   本轮第一次在终端页上复用它。
3. **`craft-loop.md` 的「两轮制」** —— 第一轮出 mid/end 四张帧 + 四视角评审（见下）；
   第二轮只做一件事：把「边高」这个唯一的数从两个候选里定下来（`Space(2)`=7.2 会让净空
   只剩 1.8，`Space(1.5)`=5.4 让「软边 + 净空 = 条带自己的 `Space(2.5)`」这笔账整好成立），
   再出图确认 `end` 帧 diff = 0、`mid` 帧的 bbox 正好是那条带。

**四视角评审**（各挑一处）：

- **艺术总监**：这块面上唯一「设计过」的动作就是这条边——它必须读成「内容从面底下过去」，
  所以边高不能大到把一行读没（5.4px 只软化最后四行像素）。
- **工程师**：`Absolute().Bottom(0)` 在面板列里 = 窗口底边；条带的内边距没动，`MaxY` 仍 54.4。
- **甲方**：三秒看完，抽屉溢出时「下面还有」这件事现在由淡出说出来，不再靠「字被切一半」猜。
- **读者**：mid 帧最底四行的字退到与面同色——正是它该退的地方。

## preflight 逐条结论

- **A 反套路**：无渐变白边、无外发光、无模糊贴图冒充玻璃；软边是 `glass.ScrollEdge`
  自己的 AppKit 曲线，不是 CSS 高光。**过**。
- **A2 进攻三步**：DNA/工艺密度见上「偷了什么」3 条；两轮制已走（见上）。**过**。
- **B 设计读取一致性**：一句话记忆点 =「内容从面底下过去」；明暗两套各跑一遍，
  没有混主题（`tokens.Control` 与 `FaceOver` 都读当前主题）。**过**。
- **C 动效抽查**：本轮零动效（滚动本身不是本轮引入的），没有 `transition`、没有入场。
  **不适用**（不是「静默跳过」：这块面本来就没有动画）。
- **D2 中文页**：本轮不新增任何可见文案（`Label` 也没加），中文排版与字号问题沿用
  需拍板项（`tokens.Caption` 9.9px 对 preflight「正文 ≥ 14px」），**本轮不动**。

## 仍待 / 下一格

1. **下一格**：通用队列 · **文案密度**（`emptyState` 一族的结论句之外，设置页各分组的
   提示句与读数句读起来是否同一档密度；先量再改）。
2. **需拍板（沿用，未动）**：`tokens.Caption` 9.9 / 正文 10.8 对 preflight D2 /
   启动器两行结果行 36px 盒装 27 行墨 / §6.111 / §6.113-5 / §6.114-4 / `104-2` /
   便条行 rank 1.28 / 终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. **本轮证伪的一条**：抽屉的**上沿**没有补边。理由不是「忘了」——条带的上沿位置由
   内容高度决定（它是面板列里自然高度的孩子），一层绝对定位的顶边盒子只能挂在面板列上，
   拿不到条带的顶 y；而那条上沿已经有一条 `LensRim` 发丝在命名那道缝（P13-d）。
   要补它得先给条带一个视口包装层，那是布局改动，不是墨与面。
