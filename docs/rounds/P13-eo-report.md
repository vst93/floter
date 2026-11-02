# P13-eo 报告：浮层的动作胶囊是圆角矩形，而三处都写着「胶囊」

表面：**插件配置浮层（plugin sheet）**。合约新增 `docs/liquid-glass.md` **§6.110**。
规格 `docs/rounds/P13-eo-spec.md`。

## 意图

**一句话**：浮层里那枚会毁掉数据的动作胶囊，是整块面上唯一一枚**不是胶囊的胶囊**——
四角是 8px 的圆角矩形，而参考的 CSS、它自己的注释、以及守它的那条 pin 都写着「999px / pill」。

## 为什么现在做（上一格先量、然后否掉）

`P13-en`「仍待」第 1 条把下一格指到「浮层的层级」：分组标题（`Title` 12.6 / 620 /
`TextStrong`）与它下面的字段标签（`Emphasis` 11.7 / 580 / `TextStrong`）只差 **0.9pt**。
量完：**设置页的卡标题与行标签是同一个关系**（`rows.go:187` `sectionHeading` 对
`rows.go:1252` `rowTitle`，同样是 `Title`/620/`TextStrong` 对 `Emphasis`/580/`TextStrong`）
——那不是浮层的病，是这一代字阶本身；只改浮层会让两个表面说两种话（跨表面），
记入 `需拍板`。于是按纪律量同一批帧里的另一格，它倒了。

## 先量（改前实测）

单独渲染在黑底上（危险的发丝是暖墨、中性的冠线是白墨，两者都读得到），取**第一行有墨的行**，
读它离盒左沿的内缩：

| 面 | 盒高 | 首行内缩 | 实际曲线 |
|---|---|---|---|
| 浮层动作胶囊（中性） | 25.2 | **5** | `RadiusSM` 8 —— 圆角矩形 |
| 浮层动作胶囊（危险） | 25.2 | **5** | 同上 |
| 扩展页动作胶囊（`actionButton`，对照） | 27 | **11** | `Space(3.75)` = 半高，胶囊 |

三处证据都说这一枚**应该**是胶囊：

1. 参考 `main:src/styles/plugin-config.css:241` —— `border-radius: 999px`；
2. `internal/settingsui/pluginsheet.go` 自己的注释 ——「a `999px` corner (half the 28u
   height, so the shape is a pill)」；
3. 守它的 pin `pluginsheetface_test.go` —— `if inset := leftmost - box.X; inset < 5`，
   后面那句是「want a pill's **wide curve**」。

**pin 在 5 上过**：阈值 5、读数 5，余量 0。一条「宽曲线」的判据坐在圆角矩形的读数上，
是这一族最典型的「错着也绿」。

## 落地

`internal/settingsui/pluginsheet.go`（产品代码 1 行 + 注释 1 段）：

```go
button := ui.ButtonBase(c).
    MinHeight(t.Space(7)).
    Padding(0, t.Space(2.5)).
    Radius(t.Space(3.5))   // 改前：tokens.RadiusSM
```

- 半径 = **半高**（`Space(7)` 的一半），与扩展页 `actionButton` 的 `t.Space(3.75)` 对
  `t.Space(7.5)` 是同一条算术。
- **半径不进布局**：盒高、10u 侧 padding、标签两端的内缩一个字节不动。

### 关键取舍：为什么半径必须随 `--ui-scale`（推翻 P8-b 那一行的理由）

`docs/raycast-design.md:326` 记着这是 P8-b 的**有意**改动（`Radius(t.Space(3.5))` →
`RadiusSM`(8)，理由「literal，不随 `--ui-scale`」）。这条理由在**胶囊**上失效：胶囊的
半径只能是**半高**，而半高正是那个随档走的数。把半径钉成字面 8，换一档界面大小盒更高、
8 不变，胶囊就漂成圆角矩形。参考的 `999px` 就是「永远半高」的写法，本仓的等价物是
`Space(3.5)`。**旧值 → 新值：`RadiusSM`(8) → 半高（`Space(3.5)`）**，`raycast-design.md`
那一行已改成指向 §6.110 并写明旧值→新值。

## 台账

**V 2 / B 0 / S 0**

| 类 | 位置 | 处置 |
|---|---|---|
| V | `pluginsheetface_test.go` 的角判据 | 阈值 `inset < 5` → **`< 8`**（注释写 `P13-eo 重定基线：旧阈值 5 → 新阈值 8`） |
| V | `raycast_test.go` `TestTheOverlayButtonCornerIsLiteralAtEveryStep` | 改名 `...IsHalfItsHeightAtEveryStep`，判据从「角是字面 8、两档读数相同」换成「token 是半高 + 两档读数必须**不同**」；旧值→新值写在函数注释里 |
| B | 其余全部 | **零改动**，全绿 |

## 门槛（本轮实测）

```
gofmt -l cmd internal                 → 空
go vet ./...                          → 净
GOOS=windows go vet ./...             → 净
GOOS=darwin  go vet ./...             → 净
go test -count=1 ./...                → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 3351（基线 3348）
SKIP 计数                              → 173（基线 172）
测试函数数                             → 2522（基线 2520）
三平台 build（linux / windows / darwin） → 全过
```

PASS +3 = 新 pin 的深浅两套（+2）+ 一条 opt-in 出图之外的既有子测试没有变化，多出的 1 条
来自改名后的 `TestTheOverlayButtonCornerIsHalfItsHeightAtEveryStep` 的深浅两套里此前被
`t.Fatalf` 截断的那一条恢复。SKIP +1 = 新 pin 的 opt-in 出图。

## mutation（3 条实跑，逐字节还原）

| # | 注入 | 原始报错串 |
|---|---|---|
| M1 | `Radius` 退回 `tokens.RadiusSM` | `corner insets: small 3, large 3` / `the corner did not grow with the box: small 3, large 3 — a literal radius reads the same at both steps, which is the value this round retires (P13-eo)` |
| M2 | `Radius` 压到 `t.Space(2)`（不到半高） | `[neutral] the pill's first inked row starts 5 px in: that is the 8px rounded rectangle this round retires (it read 5), not a stadium's half-height curve (want ≥8)` |
| M3 | 侧 padding `Space(2.5)` → `Space(4)` | `[neutral] the label's leading inset = 15.400002, want the reference's 10u (9)` |

三条都实跑变红、`md5sum -c` 逐字节核对还原。

## 出图

`/tmp/floter-P13-eo-pill-{before,after}-{light,dark}.png`（400×160，两臂单独渲染）。

```
compare -metric AE  → 浅 302 / 深 442
bbox                → 浅 77×67+40+41 / 深 78×68+40+40
```

bbox **正好是两枚胶囊自己的矩形**（x 从 40 起、y 40/83.2 起）——页与字一个像素没动，
这证明「只改四角的墨、不改盒」。

**这张图上看到的一处变化**：两枚胶囊的左右两端从「带一点圆角的矩形」变成真正的胶囊，
而它们的宽、高、字位与那两条发丝的位置都没动——同一枚按钮，只是不再像旧版 macOS 的方角键。

## 用 qiaomu-design skill（「偷了什么」）

**读取为**：自绘桌面应用的配置浮层，面向正在改插件参数的用户，用「轻便液态玻璃」的克制
语言，走 `craft-loop.md` 的「已有方向，只要打磨」车道（**不**走 Phase 2 的四方向预览）。
拨盘：视觉冒险度 **4**（功能页，不许新花样）、动效强度 **3**（本轮不动效）、信息密度 **7**。

**从 `craft-loop.md` 的工艺密度菜单取了第 12 项「材质细节」，落成具体数值**：

- 参考 CSS 里这一枚的三个值就是 DNA（`craft-loop.md` §一的「偷具体值，不偷感觉」）：
  `border-radius: 999px`（`plugin-config.css:241`）→ 本仓 **`t.Space(3.5)` = 半高**；
  `height: calc(var(--u) * 28)` → **`t.Space(7)`**（不动）；`padding: 0 calc(var(--u) * 10)`
  → **`t.Space(2.5)`**（不动）。三个值里只有一个变了，另外两个是这一轮的**反证**。
- 第 12 项在本轮是**退役**一种材质细节，不是加一种：圆角矩形是旧 macOS 按钮的纹理，
  胶囊是这一代的那一支；本轮**不加**任何高光、渐变或白边（§1 禁令 3）。
- **第 5 项「多层染色阴影」判「故意不做」**：E1 不投影（§2 判据 2），一枚 28u 的胶囊上
  叠阴影就是把它抬成浮层，与「一个表面一轮、只改墨与面」冲突。

**两轮制**（`craft-loop.md` §三）：第一轮只改 `Radius` → 渲染 → 旧的
`TestTheOverlayButtonCornerIsLiteralAtEveryStep` 当场变红（它钉的正是被推翻的那个值）→
第二轮把**那条 pin 重定基线**而不是把产品改回去，并把「两档读数必须不同」写成新判据
（这才是「胶囊随档」的可测形式）。

### 四视角偏执评审（各挑一处最弱）

- **艺术总监**：这一格是**形状语法**，不是表情——它让浮层的按钮与扩展页的按钮终于同族，
  但浮层整体仍然只有「面 + 发丝 + 字阶」三层，没有任何「这块面是浮起来的」信号。那属
  层级/材质，留给后面的轮次。
- **工程师**：`Radius(t.Space(3.5))` 依赖「`Space(3.5)` 恰是 `Space(7)` 的一半」这条
  隐含关系；pin 里显式断言了它（`tc.tokens.Theme.Space(3.5) == Space(7)/2`），所以将来
  有人改 `Space` 的取整时会在 token 层先红，而不是在像素上漂。
- **甲方**：三秒能看出「这枚按钮圆了吗」？——能，浅色档 302 个像素、深色档 442 个，
  全部落在两枚胶囊的四角上。
- **读者**：胶囊的曲线让「按这里会毁掉东西」这枚危险键读起来像一枚**键**，而不是一块
  被切了角的色片——同一个字阶下，形状是它唯一的身份。

## preflight 结论（逐条，不能过的写为什么）

- **A 反 AI 套路**：过。无渐变、无发光、无 `#000000` 纯黑（探针里的黑底只是测试背景）、
  无斜体（P-1）、无三等分卡片、无「革命性/无缝/赋能」类文案（本轮一个字文案都没改）。
- **B 设计读取一致性**：过。声明的拨盘（冒险度 4 / 动效 3 / 密度 7）与产出一致：只动了
  一个半径参数，没有新色、没有新动效、没有新增控件、没有动布局。
- **C 动效抽查**：**不适用**（本轮零动效改动；浮层的 160ms 入场在 §5 里，未触碰）。
  记在这里而不是静默跳过。
- **D2 中文页**：**仍未过**，且**不是本轮引入的**——`tokens.Caption` 9.9px / 正文 10.8px
  对 preflight D2 的「正文 ≥ 14px」。这是第 **64** 份证据，仍在 `.pending-bugs.md` 的
  「需拍板」里：改字阶要动整个应用的界面尺寸阶梯（`--u` 的每一步），是行为契约级决定。

## 仍待（下一轮）

1. **下一格（首选）**：**状态反馈的另一半——禁用/只读面**在浮层里的空白：浮层的字段
   没有禁用态，但**动作行在飞**时没有任何「已发出」的反馈（`sheetActionButton` 无 busy
   臂）。这一条要先量：按下确认之后到 `Run` 返回之间，那一行是否**逐帧相同**；若相同，
   它就是「按了没反应」——属状态反馈，不属新功能。**需先确认 `Run` 的签名是否带回调**
   （没有回调就只做「按下那一帧的 press 面」，那是既有 `pressed` 臂）。
2. **需拍板（沿用 + 第 64 份证据）**：`tokens.Caption` 9.9px 对 preflight D2 /
   浮层页头字号上阶梯（`104-2`）/ 浮层分组标题与字段标签只差 0.9pt（与设置页同关系，
   只改浮层即跨表面）/ 启动器两行结果行 36px 盒装 27 行墨 / 便条行 rank 1.28 /
   终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 /
   `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 /
   路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. 抢救 stash（第六十八次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）仍判定
   与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
