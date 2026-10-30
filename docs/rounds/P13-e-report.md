# P13-e 报告：扩展页表面（`docs/liquid-glass.md` §6.4）

规格 `docs/rounds/P13-e-spec.md`。**一个表面，一轮**：扩展页 = `internal/settingsui` 的
Integrations 面板。§6.4 的四行全部收口（两行落地、两行写死 `故意不做` 并附理由）。

**这轮为什么让它更轻 / 更有空间感**：这一页此前只有「两级半」——**底**（页面：`Sunken`
+ 外层 tint）与**面**（分组卡，P12-c 的 2-3 已经给它戴上 `LitFace`）都在场，缺的是
**被选中的那一行**：`floter://register` 链接命名的那一行只是一层 `RowSelected` 平涂，
与它脚下的 E1 卡只差一档 alpha，读起来是「卡里又刷了一块漆」。本轮把 E1 的那一对线
接到这一行上，这一页第一次出现「底 / 面 / 选中的面」三级——不是多了很多线，是
**一块面第一次从它躺着的那块面上站了起来**。

---

## 1 落地项

### 4-1 集成行面 → E1（`已落地`）

- `internal/settingsui/rows.go` 的 `rowFace` 新增 `Lit bool`（零值 = 旧行为，既有调用点
  一个字节不动）；`cardRow` 在 hover / press 换完**面**之后调用
  `tokens.LitFace(row)`——与 §6.1 第 1-2 行（启动器选中块）、§6.2 第 2-2 行（侧栏选中页）、
  §6.3 第 3-8 行（终端空态键帽）**同一个函数、同一对数值**。压在 hover/press 之后是有
  意的：指针换的是**面**的色号，不是这块面的层高。
- `internal/settingsui/settingsui.go` 的 `integrationRowFace`：`Highlighted` 臂置
  `Lit: true`。臂的顺序维持原样（先 `Broken`）——一块既坏又被链接命名的行说的是
  「坏的」那一句。
- 实测（出图逐像素差分）：

  | 外观 | 顶边（受光边） | 底边（对侧暗线） |
  |---|---|---|
  | light | `(223,223,224)` → `(224,224,225)` | `(221,221,222)` → `(208,208,209)` |
  | dark | `(65,65,66)` → `(70,70,71)` | `(65,65,66)` → `(61,61,62)` |

  浅色档顶边只抬 1 级——**白压白**（§3 末尾那条已经写死在合约里的物理事实）。
  让块真正站起来的是**底侧那条**（13 级）。这正是 P12-b 在启动器键帽上量到的同一个
  关系，现在在第三张表面上复现，而且数字同一个量级。

### 4-3 扩展动作胶囊 → 复核 + 像素 pin（`已落地`，产品代码零改动）

`extensionActionFace` 的 `tokens.Lens(button)`（`rows.go:486`）第一层就是 `LensRim`，
P12-a 已把它重定到 0.09——「受光边随升代自动到位」这一条**成立**。本轮补的是证据：
`TestTheExtensionActionPillWearsTheE1Crown` 把胶囊的顶内行与两颗参照胶囊（同一套四层
栈，**只**换那条受光边：`LensRim` vs `EdgeHighlight`）比，量到的是前者（深 160 vs
171、浅 124 vs 221），并且左右两侧的像素仍是底色（E1 不画影，§2 判据 2）。

**为什么不改成 `LitFace`**（本轮的取舍，写进合约 §6.4）：胶囊自己已经有一条
`Border(1, ControlEdge)`，而 `Lens` 的第四层是同色的 `InsetShadow(0,0,0,1, ControlEdge)`
——它们画在相邻的两行 1px 上，是**同一条边的两次上色**，不是「画框分层」的第二层。
退掉它会让胶囊的边缘从 2px 掉到 1px，那是几何改动，超出「视觉轮只改墨与面」的红线。
P13-d 退掉终端图标按钮的 `Lens` 是因为那个按钮**没有**自己的 border：那里的第四层
就是唯一的框。两处看起来是同一个动作，其实判据不同——这一条写进报告而不是悄悄做掉。

### 4-2 包 chip / 4-4 破损行（`故意不做`，附理由）

- **4-2**：与 §6.2 第 2-9 行同一条（30u 的方块加冠线就成噪点）。原有的实体/幽灵 pin
  原样通过。
- **4-4（新增行）**：破损行**不加** E1 冠线。一个面的身份是**一条**受光边（§1 的
  不变量）；破损行那条线已经被它的警告色内环占住（`Inset` 四边都在）。再叠一条白冠线
  会画进同一个 1px 行，把「这块坏了」洗成「这块被选中了」。这不是少做一步，是
  「一个面只说一句话」。结构 + 像素**两面**都钉住（`Lit == false`，且顶行不是中性亮线）。

## 2 为什么不跨表面

§6.5（文件页）与 §6.6（toast 两层影）各是另一个表面，按任务书的「一个表面一轮」
留到下一轮。§6.4 是本轮唯一动手的表面，四行全部有状态。

## 3 V / B / S 台账

- **V 2**（都是同一个 pin 文件里的同一条断言，按类一起记）：
  1. `settingsui/extensionrowface_test.go:TestTheRowFacePaintsItsPaneAndKeyline`：register
     行的顶边断言从「与自己的面同色（没有 keyline）」翻成「**亮于**自己的面，且是中性
     的线」。旧值正是本支要退掉的「一块平涂」，新值是本轮的设计意图。同一测试的后半
     （左缘不画前导规则条）**原样保留**——那是 §5 的禁止侧，一个字节没动。
  2. `settingsui/extensionrowface_test.go:TestTheIntegrationRowFaceFollowsItsState`：
     **加**两条（命名行 `Lit == true`、破损行 `Lit == false`），旧断言全部原样。这是
     收紧而不是放宽：把「哪一种面拿哪一条线」写成结构事实。
- **B 0**：其余 2133 条测试函数零改动全绿——包括读行中间平面的
  `TestTheRegisterRowIsTheNeutralSelectedRung`（不读顶边，所以不受影响）、
  `TestTheLiveIntegrationRowsLeadWithTheChip`、`TestTheSettingsCardControlsCarryTheStopLens`
  （`Lens` 未动）、`shell/glasscensus_test.go` 的 `.extensions-action-button` 条目
  （marker `tokens.Lens(button)` 还在，所以 census 不需要翻面）。
- **S 0**。

## 4 门槛（实测）

```
gofmt -l cmd internal                      → 空
go vet ./...                               → 净
GOOS=windows go vet ./...                  → 净
GOOS=darwin  go vet ./...                  → 净
go test -count=1 ./...                     → 全绿，0 FAIL
go test -count=1 -v ./... | grep -c PASS   → 2887（基线 2878，+9）
SKIP                                       → 63（基线 62，+1 = 本轮新增的 opt-in dump）
测试函数（同一条 grep 在两棵树上各跑一遍）  → 2133 → 2137（+4 = 3 条 pin + 1 条 dump）
GOOS=linux|windows|darwin go build ./...   → 三平台过
```

PASS +9 = 3 条新 pin × 深/浅两套（6）+ 3 个父测试（3）。

## 5 mutation（4 条，逐条实跑变红并 `md5sum -c` 逐字节还原 + `touch` 复跑绿）

| # | 注入 | 变红的 pin | 原始报错串（节选） |
|---|---|---|---|
| M1 | `Highlighted` 臂去掉 `Lit: true` | `TestTheNamedIntegrationRowStandsOnOneLitEdge/light` + `TestTheRowFacePaintsItsPaneAndKeyline` | `[light] the named row's crown {120 120 120} is not brighter than its own face {120 120 120}: the row is a wash of paint, not a block` / `the register row has no crown: top edge {232 232 233} is not brighter than its pane {232 232 233} (P13-e)` |
| M2 | `cardRow` 的 `LitFace` 换成手写的 `EdgeHighlight` 顶边 | `TestTheNamedIntegrationRowStandsOnOneLitEdge`（深/浅都红） | `[dark] the named row's crown {162 162 162} is nearer the E2 `EdgeHighlight` ({162 162 162}) than the E1 `LensRim` ({149 149 149})` |
| M3 | `Broken` 臂加 `Lit: true` | `TestTheIntegrationRowFaceFollowsItsState` + `TestTheBrokenRowKeepsItsWarningLineAndGetsNoCrown`（深/浅） | `the broken row asks for the E1 crown: its one line already belongs to the warning keyline (§6.4 第 4-4 行)` |
| M4 | `extensionActionFace` 删掉 `tokens.Lens(button)` | `TestTheExtensionActionPillWearsTheE1Crown/light` | `[light] the pill has no crown: its top inner row {119 119 119} is not brighter than its face {119 119 119}` |

M3 的还原第一次打错了一个括号（多了一个 `}`），被 `md5sum -c` 当场抓住
（`settingsui.go: 失败`），改正后三条文件全部 `成功`。这是 `md5sum -c` 存在的意义，
不是流程噪音。

## 6 出图（8 张）

`/tmp/floter-P13-e-integrations-{dark,light}-{before,after}.png`、
`/tmp/floter-P13-e-pill-{dark,light}-{before,after}.png`
（opt-in dump `TestDumpP13ELitEdgeFrames`，`FLOTER_PNG=1` + `FLOTER_PNG_TAG`）。

**图上看到的**（逐像素差分 + 看图）：
- 整页只有**两行**变了——`y=542`（命名行顶边）与 `y=658`（命名行底边），x 覆盖整行宽度
  （192..700）。几何（行高、缩进、圆角、chip 位置、分隔线）一个像素没动，这正是
  「只改墨与面」该有的形状。
- 浅色档放大到 2×：命名行现在顶边有一道极淡的暖白、底边有一道明显的灰——块的上下
  两侧不再同色，「卡里刷的漆」变成「卡上放的块」。深色档同理（70/61 vs 65/65）。
- 胶囊那两张 `before/after` **逐字节相同**（`cmp` 通过）：4-3 是复核，产品代码零改动。
  这是有意的证据——它证明本轮没有偷偷动过那颗胶囊。

## 7 偷了什么（qiaomu-design，`references/craft-loop.md` 工艺密度菜单）

逐条写「第 N 项 → 落成哪个数值」：

1. **#12 材质细节（内边框高光 / glass 折射边）** → `theme.Tokens.LitFace`：顶
   `InsetShadow(0, 1, 0, 0, LensRim=0.09)` + 底 `InsetShadow(0, -1, 0, 0, LensEdge=0.06)`。
   两值都是 P12-a 定下的合约值，本轮**不新造墨**，只把同一对值接到第四张表面上。
2. **#10 收尾彩蛋（hover 才出现的细节）** → 那一对线压在 `hover/press` 换面**之后**：
   指针扫过命名行时，面会换成 `SurfaceHover`，但那两道 1px 的内线**不动**——所以
   「这块是被选中的」这件事在指针经过时也不会消失。落成位置：`rows.go` `cardRow`
   的 `if face.Lit` 块在 `switch { case row.Pressed() … case row.Hovered() … }` 之后。
3. **#6 编辑细节（hairline）** → 反面账：本轮**没有**新增任何 hairline。破损行的警告环
   与命名行的受光边是**两件东西**，一条 1px 行里只许出现一条线（`rowFace.Lit` 与
   `rowFace.Inset` 互斥使用，由 4-4 的 pin 钉住）。

工艺密度 = 3 项（低于营销页要的 ≥5，如实记）：本仓是终端类**功能页**、不是营销页；
且「一轮只做一个表面」的纪律把可落项压到 3 项——上一轮 P13-d 也是 3 项，同一原因。

## 8 preflight（`references/preflight.md`）逐条

- **第 0 步 / A0 用户偏好账本**：已读 `user-preferences.md` 全文头部与生效条目。逐条对照：
  - **P-1 禁斜体**：通过（本轮零字体改动，仓库里也没有 italic）。
  - **P-5 可读性优先**：通过。`canvasInks` 的可读性地板没动；新增的两条 1px 线在文字
    的**上下边框**上，不压在字形上（图上复核过：`Register Me` / 描述 / `Running` 的
    抗锯齿边缘与 before 逐像素相同）。
  - **P-15 顶部与内容融合**：通过。命名行与分组卡的相对关系只多了两条内线，卡的
    外圈 `HairlineSoft` 一个字都没动。
  - **P-38 无左侧竖线装饰**：通过。本轮**没有**加任何竖线；`Bar` 字段一个字节没动，
    `TestTheRowLeadingRuleStaysDarkOnHover` 原样通过。
  - **P-14 无「已载入」式自语**：通过（零文案改动）。
  - **A0 自进化证据门**：本轮**无**用户新反馈、**无**自发现的可复现缺陷 → 不需要写
    `events.jsonl`（没有事件）。
- **A 反 AI 套路**：通过。这一轮**不产生任何新元素**——只加两条 1px 的内线，且
  它们是仓库里已经存在的那个函数。无渐变、无 glow、无新文案、无新卡片、无三等分卡海。
- **B 设计读取一致性**：通过。设计读取 = 「已有方向（轻便液态玻璃），只要打磨」，
  走 `craft-loop.md` + `preflight.md` 的**打磨模式**；**未走** Phase 2 的四方向预览
  （任务书写死：方向已由用户给死，自动车道无人点选，不许出 HTML 预览页、不许停下等）。
  记忆点一句话：**「卡里刷的漆，变成了卡上放的块」**。
- **C 动效抽查**：**部分不适用**——本轮没有新增任何动效。相关的既有动效一条没动：
  行/按钮的 120ms 颜色过渡、E2 浮层 160ms 入场、`ReduceMotion` 兜底。新 pin 里显式
  `Advance(300ms)` 越过 120ms 过渡窗再读数（dump 里），这是「等它落定」，不是新动效。
- **D2 中文页追加项**：通过（本轮新增的注释与报告是中文；中西文之间、数字与单位之间
  留空格这条已遵守，如「`LensRim`(0.09)」「旧值 → 新值」）。零装饰中文字体。
- **A2 进攻三步核验**：DNA 注入 = §7 三项（逐项落成数值）；工艺密度 = 3 项（如实
  低于 5，理由见 §7）；两轮制 = 已发生（出图 → 四视角评审 → 只修评审挑出的那几处，
  见 §9）。
- **A2.1 执行者路由**：通过。当前 Codex 直接执行，**未调用 K3**（用户本任务未点名）。
- **A5 预览交付门**：**不适用**（任务书明令不出四方向预览页，理由见 B）。

## 9 四视角评审（第二轮只修 3-5 处）

- **艺术总监**：最平庸的一屏是「普通行」——它们仍是卡的平面。**不改**：这一页的
  层级已经够（底/面/选中的面三层），给普通行加线就是「每一行都是卡片」的 AI 套路
  （A 节明禁）。
- **工程师**：`rowFace.Lit` 与 `rowFace.Inset` 可以同时为真，而两者都会往同一行 1px
  上画线。**已修**：破损行不置 `Lit`，并由 4-4 的 pin 从结构上钉住；将来若有人给
  破损行加 `Lit`，`TestTheBrokenRowKeepsItsWarningLineAndGetsNoCrown` 会红。
- **甲方**：三秒看完，记住的是橙色那句 `The link you opened points at this tool` 与
  现在终于「浮起来」的那一行。功能契约（这是哪个扩展、它能不能跑、怎么卸载）没变。
- **读者**：眯眼通读——**没有问题**（本轮不动字、不动字号、不动对比度）。

## 10 仍待（追加在 P13-d 之后）

1. **S1b（仍等拍板）**：`terminalui` 接 `Dial`；`CloseAll` 从「退出即杀」变 detach。
2. **S1c（Windows ConPTY）**；`Alive` 在 Windows 恒 false。
3. **客户端不告而别时宿主怎么知道**（P13-b 遗留）。
4. **capture 通道迁移**（P13-c 的拍板项）。
5. **视觉队列下一站 = §6.5 文件页**（1 行：文件列表行与 1-2 同一条规则）。
   §6.6 弹层（toast 两层影）在其后——那一条改的是**影**的层数，`ElevThrow` 已经在
   `theme` 里有摆位，但要先确认 toast 的几何不会让宽影被视口裁成硬边（2-8 的教训）。
6. **本轮的一处观察（不进代码，写给下一轮）**：`extensionActionFace` 的胶囊边缘是
   **2px**——`Border(1, ControlEdge)` + `Lens` 第四层的同色内环画在相邻两行上。
   这是 P8 时代「`--elev-0` 透镜 + `border: 1px`」的原始形状，不是本轮引入的。要不要
   收成 1px 是**几何改动**，需要拍板（见 `.pending-bugs.md` 的「需用户拍板」表）。
