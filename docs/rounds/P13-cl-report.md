# P13-cl 报告 · 重新检查键按下之后，它自己长出来 15px

规格 `docs/rounds/P13-cl-spec.md`，合约新增 `docs/liquid-glass.md` **§6.55**。

## 这轮为什么让它更轻、更有空间感

一枚键的**轮廓**在按下的那一瞬间不该动。集成页的重连 / 重新检查键此前有两个面：
静息是**一行字**，在飞是**环 + 一行字**——环是凭空**加**上去的（10px + 5px 间距），
而胶囊是这一行最右的那个控制，右沿钉住，于是多出来的 15px 全部长在**左沿**上：
手指按着的那条边在按下的一瞬往外推 15px。参考那边不会动，因为环是**顶替**已有的
RefreshCw（`ExtensionRow.tsx:305,325,364`），槽本来就留着。这一轮把那枚 glyph 补回来，
两个面重新变成同一枚键的两张画。

## 改法（两个文件、三处）

| # | 位置 | 改前 | 改后 |
|---|---|---|---|
| 1 | `settingsui.go` `repairAction` 静息臂 | `a.actionButton(c, label)`（一行字） | `a.extensionGlyphActionButton(c, repairGlyphName, label, Space(7.5), Space(3.75), Body, disabled)`（RefreshCw + 一行字） |
| 2 | 同处「另一操作在飞」臂 | `a.disabledActionButton(c, label)` | 同一个 glyph 面 + `disabled` **建造时**传入（P13-ba 的规矩），另补 `.Disabled(true).Cursor(ui.CursorDefault)`——`extensionGlyphActionButton` 只把 `disabled` 交给材质，不设框架的禁用位 |
| 3 | `rows.go` `actionSpinner` | 环 `Size(Space(2.5))`（10px） | `Size(Space(3.5))`（14px）= 静息 glyph 的盒子 |

`repairGlyphName = "refresh"`（新增常量）：两臂的语义都是「再跑一次搜索」
（`repairTooltip`：`reconnectUnboundHint` 说的就是搜索会重跑），一个 glyph 对两臂都成立。

**不动的**：`actionButton` / `disabledActionButton` 的其它 ~10 个调用点（它们没有在飞臂，
加 glyph 就是给每一个 pill 添墨）；`extensionActionFace`；pill 的高度 / 圆角 / 字号 /
padding / gap；`repairBusy` 的生命周期与 `rowBusyExceptRepair` 的语义；i18n 文案。

## 实测（760×1200 帧，深浅各一遍）

| 臂 | 词盒 x | 那一格最左墨列 | 那一格墨距 |
|---|---|---|---|
| 静息（浅 / 深） | 552.0 / 552.0 | **545** / **545** | 186 / 176 |
| 在飞（浅 / 深） | 552.0 / 552.0 | **545** / **545** | 96 / 93 |

改后两臂的**词盒 x 与那一格的最左墨列逐像素相同**（545 = 545），深浅同值。
在飞臂的墨距只有静息的一半（96 对 186），因为那一臂是 `Disabled`——框架给整棵子树乘了
0.5（§6.2b-6 的那道淡），pin 因此量**距离**而不是绝对墨：深色档的墨本来就比面**亮**，
一个「最暗像素」阈值只能描述两臂中的一臂。

## 变异（实跑）

- **M1**（静息臂退回 `actionButton`，即去掉 glyph 槽）：**红**，两条臂都红——
  `rest: mark left=-1 mark delta=16` / 深 `22`，报错
  `the resting pill has no mark left of its word` + `no mark ink on the pill's leading edge (rest -1, busy 545)`。
  `md5sum -c` 逐字节还原、`touch` 复跑绿。
- **M2**（环退回 10px）：**不红**，实测 `mark left` 两臂都是 **545**、`mark delta` 96 → **82**（深 93 → 79）。
  照实写在这里：**这一版布局里环的尺寸既不改变词盒 x，也不改变那一格最左的墨列**，
  所以第 3 处改动是**对齐静息 glyph 的盒子**（两张画同宽），不是「词不搬家」的承重墙；
  承重墙是 M1 补回的那枚槽。`md5sum -c` 还原、`touch` 复跑绿。
- **M3**（计划里的反向对照）**取消**：M2 已经演了那个角色（几何改了、pin 不红），
  再加一条同形的对照只是重复。

## 门槛（全部实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 全绿
go test -count=1 -v ./... | grep -c PASS → 3260 → 3263
GOOS=linux/windows/darwin go build ./... → 过
```

SKIP 130 → **131**（新增的 1 条是 opt-in 出图）。测试函数 2414 → **2415**。
**V 0 / B 0 / S 0**：没有任何旧 pin 需要重定基线，`go test ./internal/settingsui/` 一次全绿。

## 出图

`/tmp/floter-P13-cl-repairpill-{rest,busy}-{before,after}-{light,dark}.png`（整帧 dump，opt-in
`FLOTER_P13CL_DUMP=1`）。`before` 是 M1 注入时抓的（静息臂没有 glyph 的旧画）。
`rest` 帧前后 md5 不同（`3c36389c…`/`48c3c5d0…` → `1681fb70…`/`1f3f8bd2…`）：静息臂的胶囊
左沿往左多出一格，那一格里是一枚 RefreshCw，词仍在原列。`busy` 帧前后**逐字节相同**
（`90cb3fef…` / `506bcf00…`）——这一轮改的是静息臂；在飞臂的环两帧都是 3.5u，所以那张画
没动，这也正是「一个键的两个面同宽」该有的证据形状：**动的那一面追上了不动的那一面**。

## preflight（qiaomu-design，逐条结论）

- **A 反套路**：无斜体、无渐变文字、无外发光、无紫蓝渐变、无三等分卡片、无左侧竖色条
  ——这一轮只加了一枚参考自己的 lucide 图标（RefreshCw），与面板里其它 lucide 记号同族。
  **过**。
- **B 设计读取一致性**：本仓单一来源是 `docs/liquid-glass.md`；本轮落的是它 §3「受光边与
  阴影」之外的一条**几何**判据（一个键的两个面必须同宽），并写进 §6.55。**过**。
- **C 动效抽查**：环的转速与相位一个字节没动（`actionSpinnerTurn` / `actionSpinnerPerTurn`
  未改），只在按下那一帧起转；`ReduceMotion` 下仍停在静止环。**过**。
- **D2 中文页**：本轮零文案改动（i18n 红线），中文页只跟着同一个 pill 变形。**过**。
- **A2 工艺密度（偷了什么）**：从 `craft-loop.md` 的工艺密度菜单取**第 12 项「材质细节」**
  ——「一个部件自己的接缝要落在它自己的格线上」：静息 glyph 14px / 间距 5px 就是那一格，
  在飞的环必须落进同一格（`Size(Space(3.5))`），而不是按自己好看的大小画（10px）。
  落成的具体数值：`Space(3.5)` = 14px（与 `extensionGlyphActionButton` 的 glyph 同值）、
  `Space(1.25)` = 5px（`--extensions-inline-gap`，未改）。

## 仍待

见 `.pending-bugs.md` 顶部「仍待（P13-cl）」。
