# P13-eu 报告 · 只读行不答指针

**方向一句话**：这一轮把「报告一件事」的行从「答指针」这一族里摘出去——一张卡上亮起来的那一块，
从此就是**唯一能按的那一块**；只熄面，墨、盒、分隔线、可选中一个字节不动。

**这轮为什么让它更轻、更有空间感**：此前只读行在指针底下与它下面真的能编辑的字段**同一条读数**
（浅 229 → 205、深 39 → 69，逐字节相同），一张卡七行、指针走到哪儿哪儿亮，读出来是「每一行都能点」。
层级不再靠猜、也不再靠多画一条边：**答不答指针**一句话就把「面」和「浮」分开了（§2 阶梯的语言）。

- 规格：`docs/rounds/P13-eu-spec.md`；合约新增 **`docs/liquid-glass.md` §6.116**。
- 抢救 stash：本轮**接续**了 `stash@{0}`（`2cf8436498`「floter orphan 2026-11-03 11:46 [stale-kill]」，
  parent = `01e2bc35` = P13-et）。它带来的是 `rowFace.Inert` + 六个构造点 + §6.116 + 本文件的前身，
  **但没有测试、没有规格、没有报告**，而且它记录的「顺手修的既有红」在 HEAD 上确实还红着。
  本轮补 pin、补规格与报告、跑门槛与变异、出图并提交。**不是静默丢弃，是接上并完成。**

## 量到的病（先量再改）

读数 = 行盒中段、行左内边距里的一小段（3 行带，`lensMean`）；指针由 `Tester.Move` 落在行 label 中心，
120ms 面过渡跑完再读（`Advance(300ms)`）。720×2200，深浅各一遍。

| # | 外观 | 行 | 静息面 | 指针下 | Δ |
|---|---|---|---|---|---|
| eu-1 | 浅 | **只读**（`fixedRow`，集成 id） | 229 | **205** | −24 级（三通道和 −72） |
| eu-2 | 浅 | 可编辑（`row`，Name，同一张卡下一行） | 229 | 205 | −24 级 |
| eu-3 | 深 | **只读**（同上） | 39 | **69** | +30 级（+90） |
| eu-4 | 深 | 可编辑（同上） | 39 | 69 | +30 级 |

eu-1 与 eu-2、eu-3 与 eu-4 **逐字节相同**。按下去同样塌（`SurfacePressed`），而按下去什么都不发生。
`mygo@v0.7.3/ui/input.go` 的 `Hovered()`/`Pressed()` 说的是「指针在不在这儿」，不是「这行能不能点」。

## 落地（行号为本轮实测）

| # | 位置 | 落成 |
|---|---|---|
| eu-a | `internal/settingsui/rows.go` `rowFace` | 新增 `Inert bool`（零值 = 旧行为） |
| eu-b | `rows.go:1567` `cardRow` 的 `switch` | 最前一支 `case face.Inert:`（空体，面留在卡片平面上） |
| eu-c | `rows.go` `factRow:1352` / `dataRow:1410` / `labelRow:1437` | `rowFace{Inert: true}` |
| eu-d | `settingsui.go` `keyValue:4352` / `fixedRow:4385` / `diagnosticsSection:2410` | `rowFace{Inert: true}` |
| eu-e | 墨 / 盒 / 分隔线 / `Selectable` | 一个字节不动 |
| eu-f | 新 pin `TestAReadOnlyRowDoesNotAnswerThePointer` | 真页面：hover 与 press 面**零位移**；对照：下面可编辑行仍抬 −72/+90 |
| eu-g | 新 pin `TestEveryReadOnlyRowFamilyIsInert` | 五个构造器逐个量：面零位移 **且墨 span 不变**；对照行仍抬 |
| eu-h | opt-in 出图 `TestDumpP13EUReadOnlyHoverPNGs`（`FLOTER_P13EU_DIR` / `FLOTER_P13EU_TAG`） | 深/浅 × {指针在只读行, 指针在可编辑行} |
| eu-i | `docs/liquid-glass.md` §6.115 第二张表 | 表头 4 列 / 数据行 5 格 → 拆成 `现状 | 落成`、六行补 `已落地` |

### 台账

**V 0 / B 0 / S 0**：没有任何既有 pin 需要改断言。`Inert` 的零值等于旧行为，所以所有可编辑行、
扩展行、会话行、`Lit` 行的读数与几何逐字节不变（`go test ./...` 全绿即证）。

## 门槛（全部实测）

```
gofmt -l cmd internal                              # 空
go vet ./...                                       # 净
GOOS=windows go vet ./...                          # 净
GOOS=darwin  go vet ./...                          # 净
go test -count=1 ./...                             # 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"  # 3363
go test -count=1 -v ./... | grep -c -- "--- SKIP"  # 178
GOOS=linux|windows|darwin go build ./...           # 过
```

- 测试函数 2532 → **2535**（两条 pin + 一个 opt-in dump）。
- **HEAD 的真实基线是 3360 PASS / 1 FAIL / 177 SKIP**，在 `git worktree add /tmp/floter-head 01e2bc35`
  里实测——不是照抄上一轮报告。那 1 个 FAIL 就是 eu-i（§6.115 表格式），P13-et 交付时就红着。
  所以 3360 + 2（新 pin）+ 1（修好的既有红）= **3363**。

## 变异（3 条实跑，全部还原并 `md5sum -c` 核对）

| # | 注入 | 结果 |
|---|---|---|
| M1 | `case face.Inert:` → `case false:` | 两条 pin **全红**（真页面 hover 与 press 各两条 + 五个构造器各两条） |
| M2 | 只把 `dataRow` 的 `Inert: true` 去掉 | **只有**族内那条的 `dataRow` 两行红，真页面那条仍绿 |
| M3（对照） | `case face.Inert: row = row.Background(face.Face)`（零值 = 不换面） | **全绿**——pin 钉的是「面不动」，不是某个写法的形状 |

M1 的原始报错串（抄自终端）：

```
light: the read-only row's face moved {229 229 230 255} → {205 205 206 255} under the pointer (-72 levels) — nothing on that row can be pressed, so the face must stay the card's plane
light: the read-only row's face moved {229 229 230 255} → {205 205 206 255} under a press (-72 levels) — pressing it does nothing, so it must not sink either
dark: the read-only row's face moved {39 39 41 255} → {69 69 71 255} under the pointer (+90 levels) — nothing on that row can be pressed, so the face must stay the card's plane
dark: the read-only row's face moved {39 39 41 255} → {69 69 71 255} under a press (+90 levels) — pressing it does nothing, so it must not sink either
light: factRow's face moved {230 230 230 255} → {206 206 206 255} under the pointer (-72 levels) — that row reports a value, it offers nothing to press
...（keyValue / dataRow / labelRow / fixedRow 同形，深色 +90）
```

M2 的原始报错串：

```
light: dataRow's face moved {230 230 230 255} → {206 206 206 255} under the pointer (-72 levels)
dark: dataRow's face moved {39 39 40 255} → {69 69 70 255} under the pointer (+90 levels)
```

## 出图（深/浅各两张，`before/` 用 M1 注入态渲）

`/tmp/p13eu/{before,after}/floter-P13-eu-readonly-{light,dark}-{readonly,editable}.png`

| 帧 | `before` → `after` | 判定 |
|---|---|---|
| `light-readonly`（指针在只读行） | diff **22580 px**（0.0143%），bbox `515×44+188+241` —— **正好一行** | 变了 |
| `dark-readonly`（同上） | diff **22581 px**，同 bbox | 变了 |
| `light-editable`（指针在可编辑行） | `cmp` **无输出**（逐字节相同） | 零像素改动 |
| `dark-editable`（同上） | `cmp` **无输出** | 零像素改动 |

**这张图上看到的一处变化**（整行 515×44 裁片的均值）：指针压在只读行上时，
浅色 `203 203 204 → 226 226 227`、深色 `71 71 73 → 41 41 43` —— 那一行整条**回到卡片的平面**上，
不再是一条比周围亮的带子；而指针移到它下面那行可编辑字段时，两张图**一个像素都没动**，
说明这轮动的只有「只读行答不答指针」，没有碰到别的东西。

## 偷了什么（`qiaomu-design` skill，`craft-loop.md` §一/§二）

**DNA 注入（2 个供体，3 个具体值）**：

| 供体 | 偷的具体东西 | 落在哪 |
|---|---|---|
| `design-systems/apple/DESIGN.md:146`「Hover: no standard hover state — cards are static, links within them are interactive」 | **静态的东西没有 hover**：hover 是「可以点」的语法，不是「指针在附近」的语法 | `case face.Inert:` —— 六个只读构造点全部不答指针 |
| `design-systems/linear.app/DESIGN.md:29`「Secondary Surface `#28282c`: the lightest dark surface — used for hover states」 | hover 是**面的一档**（换面），不是叠一层色/加一条边 | 保留 `SurfaceHover` 作为唯一 hover 面（浅 205 / 深 69），不给只读行新画任何边 |
| `design-systems/linear.app/DESIGN.md:16-18`「semi-transparent borders `0.05–0.08`」 | 结构靠**已有的发丝线**说，不靠新增描边 | 行仍是 1px 分隔线 + 44u 行盒；本轮**零新增描边**（也避开 P-38「禁止左侧竖线装饰」） |

**工艺密度（`craft-loop.md` §二 菜单，逐项指认）**：

1. **#12 材质细节** → 面档差 24 级（浅）/ 30 级（深）从此只出现在**可按**的行上；
   只读行留在 `Control`（229 / 39）。这是菜单里「材质要服务语义」那一条在行上的说法。
2. **#4 签名交互（hover 揭示）** → hover 揭示被收窄到 6 个真的可按的构造器；6 个只读构造器
   （`factRow`/`keyValue`/`dataRow`/`labelRow`/`fixedRow`/`diagnosticsSection`）平面化。
3. **#7 编辑细节（hairline）** → 行的 1px 分隔线与 44u 行盒成为这张卡上**唯一**的结构记号，
   读数不变（`inkSpan` 在 hover 前后相等，pin eu-g 守住）。
4. **#2 ::selection 定制** → 只读行的值仍 `Selectable()`（`keyValue` / `fixedRow` / `dataRow`），
   「只读是结构，不是墨」的第三条推论：能选、能复制，只是不能按。
5. **#5 多层染色阴影** → **故意不用**：§2 判据 2 说行是 E0，它躺在卡上，不投影；
   给行加投影就是把 elevation 阶梯说糊（用户原话要的是「空间感」，不是「每行都浮」）。

## preflight 逐条

- **A（反 AI 套路）**：通过。无渐变/发光/纯黑/斜体；无三等分卡片；无假数据文案；感叹号 0；
  无眉批；**无左侧竖线装饰**（本轮零新增描边，`rowFace.Bar` 的既有语义未动，P-38 满足）。
- **B（设计读取一致性）**：通过。读取 = 原生桌面设置页 · 熟悉 floter 的开发者 · 轻便液态玻璃 ·
  Linear/Apple 的「静态无 hover」语法。全页单主题；记忆点一句话 = **「亮起来的那块就是能按的那块」**；
  动效气质一致（120ms 面过渡，无新增动效）。
- **C（动效抽查）**：通过。无新增 transition；既有 120ms ≤ 300ms；`ReduceMotion` 在两条 pin 里都开；
  hover 反馈只在指针设备上（自绘桌面应用，无触屏 hover 问题）；没有动画 width/height。
- **D2（中文页）**：**不能全过，第 2 条不过**——`tokens.Caption` 9.9px / 正文 10.8px < 14px。
  这不是本轮引入的，是 §6 全仓的既有基线，已第 **70** 份证据记在「需拍板」；本轮按纪律**不动字号**
  （改字号会推翻高度链与全部行 pin）。其余：中文用系统字体栈、行高 ≥ 1.5（行盒 44u）、
  字重只用 500/620/580、全角标点、暗底正文对比度由 token 保证——均通过。

## 两轮制（Render → Critique → Refine）

**第一轮**：`Inert` 支路 + 六个构造点（stash 的实现）+ 两条 pin + 出图。
**四视角评审（各挑一处）**：

1. **艺术总监**：修完之后，只读行**没有任何状态反馈**，一帧静态图上读不出「这行为什么不能改」。
   → 真问题，但解法是「加一句话」（提示/tooltip），属交互变更 → 进「需拍板」，不擅自加。
2. **工程师**：`keyValue` 的 label 是**右对齐**的，探针以 label 盒为锚在五族里落点不同（左侧内边距
   vs 行中段）→ 本轮修：把读数点写进 `rowInsetFace` 的注释并说明它对两种对齐都成立（都在行面内、
   都避开墨）；不引入按族分支，pin 才不会被自己的探针骗。
3. **甲方**：三秒看完记住的是「工具表单里有一行是死的」——这正是意图，但也说明**只读行与普通行
   在静态帧上完全同形**，需要靠读内容判断。→ 与第 1 条同源，进「需拍板」。
4. **读者**：只读行的值（`local.abc`，mono caption `TextMuted`）是这一行**唯一的信息**，却是最安静
   的一档墨。→ 这是 P13-do 已经拍过的层级（§6.38 `dataRow` 同族），本轮不翻案，只记在「仍待」。

**第二轮只修 1 处**（第 2 条）+ 1 处加固：族内 pin 增加「**墨 span 不变**」的断言（eu-g），
把「另一种修法＝把只读行洗淡」也钉死。其余三条按纪律进「需拍板 / 仍待」。

## 仍待

1. **需拍板（本轮新增，第 1 条）**：只读行现在**没有一句「为什么不能改」**。候选：行尾一个
   `Lock` 图标（新增墨）、或 `ui/tooltip.go:133` 那条「禁用元素不弹提示」的修法（让 tooltip 也答
   只读行）。两条都要改交互，不擅自做。
2. **需拍板（沿用）**：`tokens.Caption` 9.9 / 正文 10.8 对 preflight D2（第 70 份证据）/ 启动器
   两行结果行 36px 盒装 27 行墨 / §6.111 浮层动作失败句宽度上限 / §6.113 的 113-5 /
   §6.114 的 114-4 / 浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 / 终端会话抽屉失败只加字
   不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素
   不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾。
3. **下一格（通用队列 · 间距节奏）**：`rowLineGap()` 是字面量 1px，而 `Space(1)` = 3.6——
   行内两档墨之间的那条缝还没按出货尺量过；先量再改。
4. 抢救 stash（第七十四次）：`2cf8436498` 本轮**已回收**；`704b7643ba` 已在 P13-et 回收；
   其余五条仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
