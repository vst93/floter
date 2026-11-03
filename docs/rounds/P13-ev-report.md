# P13-ev 报告 · 堆叠行的三对，此前只有一条缝

**方向一句话**：一条 `Gap(3.6)` 管了三对——本轮把它拆成「一句」与「一件可以按的面」两档：
标题与它的解释读 6px（与控件在右边的行**逐像素相同**），控件从 5/7px 退到 9/11px。

**这轮为什么让它更轻、更有空间感**：控件是这一行里唯一一件能按的东西，此前它比那句话
自己的两行之间还近（5px 对 9px），读起来是「解释的下半句」。现在一句话是 6px、一件面是
9/11px —— **层级由缝分档，不靠再加一圈边**（§4.3 的规矩，代入这张面自己的刻度）。

- 规格：`docs/rounds/P13-ev-spec.md`；合约新增 **`docs/liquid-glass.md` §6.117**。
- 抢救 stash：`stash@{0}` = `2cf8436498` 已在 P13-eu 回收；本轮未试合其余五条（判定无未接续内容，未静默丢弃）。

## 量到的病（先量再改）

读数 = `inkBands`（阈值 12 级、阈值 3 像素、降序合并悬挂部），General 页 860×1200，深浅各一遍；
盒读数来自 `tt.Find`。

| # | 行 | 对 | 盒缝 | 墨缝（浅 / 深） |
|---|---|---|---|---|
| ev-1 | **堆叠**（`stackedRow`，「Language」） | 标题 → 解释 | **3.6** | **9 / 9** |
| ev-2 | **堆叠** | 解释 → 控件 | 3.6 | **5 / 7** |
| ev-3 | **trailing**（`row`，「Launch at startup」） | 标题 → 解释 | **1.0** | **6 / 6** |

ev-1 与 ev-3 是**同一对**（同一档墨、同一句关系）的两个读数：9 vs 6。ev-2 更硬——
控件比那句话的两行之间**还近**。`stackedRow`（`rows.go:1701`）的列上只有一条
`Gap(t.Space(1))`，它不认识「哪两件东西是一句」。

## 落地（行号为本轮实测）

| # | 位置 | 落成 |
|---|---|---|
| ev-a | `rows.go:1715` `stackedRow` 外列 | `Gap(t.Space(1))` → **`Gap(t.Space(2))`**，只留给「解释 → 控件」 |
| ev-b | 同处 | 标题与解释收进 `ui.Column(c).FillWidth().Gap(a.rowLineGap())` —— 与 trailing 行同一条缝 |
| ev-c | 墨 / 盒 / 控件 / 分隔线 / 行计数 | **一个字节不动** |
| ev-d | `internal/settingsui/stackedgap_test.go` | 两条 pin + 一个 opt-in 出图 |
| ev-e | `docs/liquid-glass.md` §6.117 | 三对三条缝 + 旧值→新值 |

**旧值 → 新值**：标题→解释 `Space(1)`（3.6，墨 9）→ `rowLineGap()`（1.0，墨 **6**）；
解释→控件 `Space(1)`（3.6，墨 5/7）→ `Space(2)`（7.2，墨 **9 / 11**）。
**净高度**：每个堆叠行 **+1.0px**（−2.6 + 3.6）——整页节奏不动，只有缝的分档变了。
实测（General，浅）：Language 行标题盒 `262.80` → `263.80`（上面前一行各 +1），
解释盒 `284.40` → `282.80`（离自己的标题 −1.6），控件字盒 `306.80` → `308.80`。

### 台账

**V 0 / B 0 / S 0**：`internal/settingsui` 全套（含 `TestStackedRowsKeepTheTrailingRowsTypeSteps`、
`TestTheSettingsPagesStayOnTheHeightLadder`、`TestThePluginSheet*` 一族）**零改动全绿**——
净 +1px 落在既有 pin 的容差里，所以没有任何断言需要重定基线。

## 门槛（全部实测）

```
gofmt -l cmd internal                              # 空
go vet ./...                                       # 净
GOOS=windows go vet ./...                          # 净
GOOS=darwin  go vet ./...                          # 净
go test -count=1 ./...                             # 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"  # 3365（基线 3363）
go test -count=1 -v ./... | grep -c -- "--- SKIP"  # 179（基线 178）
GOOS=linux|windows|darwin go build ./...           # 过
```

测试函数 2535 → **2538**（两条 pin + 一个 opt-in dump）。

## 变异（3 条实跑，全部还原并 `md5sum -c` 核对）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 外列 `Space(2)` → `Space(1)` | `TestAStackedRowsControlStandsApartFromItsSentence` 红：`light: the control's ink sits 5px under the hint and the hint 6px under the title — … (want ≥2px more)`；深色同条（`7px … 6px`） |
| M2 | 回到本轮的形状之前（三对一条 3.6 缝） | **两条全红**：`light: the stacked row's title→hint gap reads 9px, the trailing row's reads 6px (+3) — …`；`light: the control's ink sits 5px under the hint and the hint 9px under the title — …`（深色同） |
| M3（对照） | 内列 `Gap(a.rowLineGap())` → 字面量 `Gap(1)` | **全绿**——pin 钉的是读数，不是写法 |

## 出图（深 / 浅两套，眼睛复核）

`/tmp/p13ev/{before,after}/floter-P13-ev-stacked-{light,dark}.png`（General 页前两张卡，
裁 `(170,110)-(860,560)`）。`before/` 是同名旧帧（本轮形状之前）。

- 与旧帧 diff **38994 px（浅）/ 41894 px（深）**，bbox `(17,85)-(674,450)` ——
  整两张卡（每个堆叠行净 +1px，下面的行跟着走）。
- **这张图上看到的一处变化**：`Language` 那一行里，解释那行字**贴上了自己的标题**
  （与上一行 `Launch at startup` 的缝同宽），而 `English / 中文` 那枚分段控件
  **往下退了一整档**——三件东西现在读成「一句话 + 一件面」，此前是三条等距的线。

## 用 skill（qiaomu-design，打磨车道）

**读取为**：桌面工具的内设置面板，面向已经在用这个应用的人，用「近黑底 + 发丝线 + 受光边」
的语言，倾向**轻便液态玻璃**这一支（`docs/liquid-glass.md` 是单一来源，方向不由本轮决定）。
拨盘：VARIANCE 3 / MOTION 2 / DENSITY 6（本轮只动两条缝，所以三档都不抬）。

**偷了什么**（`craft-loop.md` 工艺密度菜单逐项）：

- **第 6 项「编辑细节」** → 落成两个具体数值：标题→解释 **1.0px 盒缝（墨 6px）**、
  解释→控件 **7.2px 盒缝（墨 9/11px）**。数值不是我拍的：6px 是这一族自己的读数
  （trailing 行，逐像素相同），9/11px 落在 §4.3 已经量过的「下一件东西」一档
  （空态 8px 一句 / 12px 下一段 / 20px 一个动作）之内。
- 其余 11 项**不取，附理由**：第 1/5/12 项（氛围层、多层染色阴影、材质细节）本轮
  一个字节都没碰墨与面（视觉轮只改缝，见红线）；第 2/3/4 项（`::selection`、focus 环、
  签名交互）在这个自绘桌面应用里没有对应面，或属于键盘模型（红线）；
  第 7/8/10 项是展示排版动作，设置面板不适用；第 9 项（自定义滚动条）不在这张面上；
  第 11 项（图形资产）本轮没有新图标。
- 没有走 Phase 2 四方向预览（用户已把方向定死；本轮是「已有方向，只要打磨」车道），
  也没有出 HTML 预览页。

**preflight 逐条**：

- **A（反 AI 套路）**：无渐变 / 无 glow / 无斜体 / 无整页居中 hero / 无三等分卡横排 /
  无假数据 —— 本轮没有新增任何装饰，`grep` 也确认没有新增 `Gradient`、`Shadow`、`Italic`。
- **B（设计读取一致性）**：一个主题（跟随设置，深浅各出一帧）；**记忆点一句话**：
  「一句话一条缝，一件面一条缝」；动效气质不变（没有新增动效）。
- **C（动效抽查）**：零新增动效；既有 120ms 颜色步进（`theme.ColorStep`）没碰；
  出图帧用 `ReduceMotion: true` 渲染，与既有 dump 一致。
- **D2（中文页）**：本轮改的是盒缝，与字形无关；中文页的同一对仍读 6px
  （`inkBands` 与语种无关），中文行高 ≥1.5 的既有账未动。**唯一不过的一项**是沿用的
  「正文 ≥ 14px」——本仓正文 10.8px，已在 `.pending-bugs.md` 的「需拍板」里挂了第 71 份证据，
  本轮不擅自改字号。

## 仍待

1. **需拍板（沿用）**：`tokens.Caption` 9.9 / 正文 10.8 对 preflight D2 / 启动器两行结果行
   36px 盒装 27 行墨 / §6.111 浮层动作失败句宽度上限 / §6.113 的 113-5 / §6.114 的 114-4 /
   浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 / 终端会话抽屉失败只加字不换墨 /
   扩展页空态只有一行出路 / 会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 /
   `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾。
2. **下一格（通用队列 · 间距节奏的下一对）**：`stackedRow` 里的**滑块行**（`a.slider`）
   与**读数行**（`effectiveOpacity`）用的是另一条列缝，还没按这一轮的三条缝量过；先量再改。
3. **证伪备选（通用队列 · 空态满态）**：`rowLineGap()` 的 1px 此前被怀疑「没按出货尺」——
   本轮量到它读成 **6px**，与空态最紧的一对（8px）同档，**1px 是对的**（改成 `Space(1)`
   实测 25 条既有 pin 变红、每个两行行 +2.6px 而墨上无收益）。这条不再当选题。
