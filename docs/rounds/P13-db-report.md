# P13-db 报告 · 插件配置弹层：那句「这条没写进去」不再挂在浮层的框上

规格 `docs/rounds/P13-db-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.71**｜起点 `f2048493`（P13-da）。

**这轮为什么让它更有空间感（一句话）**：一块浮层上只有一条**关于字段**的句子，此前它和浮层
自己的名字挂在同一条边框列上（`14.0`），比它讲的那批字段（`29–30`）还左出 16px——句子于是
读成「这块面的旁注」。收进字段列之后，弹层里「名字 / 分组 / 字段 / 解释」落在两条列上，
各归各的层，正是 §4.3「间距属于成对的两件东西」在一处浮层上的同一条规则。

## 1. 量法（可复现）

`renderPluginSheet`（`internal/settingsui/pluginsheetframe_test.go`：720×900、`c.SetTheme(a.tokens(c).Theme)`、
深浅各一帧），用 `tt.Find` 取盒、`inkLeft` 扫盒中线第一列墨（盒右端取样地面，同 `inkRows` 的取法），
夹具是只会话、只有一个分组的一个开关字段（字符串唯一，避开 `Find` 命中重复文本）。

| 节点 | 改前（浅 / 深） | 改后（浅 / 深） |
|---|---|---|
| 浮层标题 `Probe plugin` | `14.0 / 14.0` | `14.0 / 14.0`（不动） |
| 写失败线 `Could not save that change.` | **`14.0 / 14.0`** | **`27.0 / 27.0`** |
| 字段标签 | `30.0 / 29.0` | `30.0 / 29.0`（不动） |

改前：失败线离浮层列 `0.0`、离字段列 `16.0 / 15.0`；改后：离字段列 `3.0 / 2.0`
（一个字形 left bearing）、离浮层列 `13.0`。

## 2. 改了什么

- `internal/settingsui/pluginsheet.go` `PluginSheet` 的 `sheet.Failed` 臂：
  `FontSize(t.FontSize-1)` → **`FontSize(tokens.Caption)`**，
  加 **`Padding(0, t.Space(3.5))`**（与它下一行的分组标题同一个缩进）。
- 同一处的注释补了 P13-db 的账：为什么它挂字段列、标题为什么留在浮层列
  （标题是这块面的名字，与设置页页头行同为 4u）。
- 新 pin：`internal/settingsui/sheetcaptionline_test.go`
  `TestTheSheetsWriteFailureHangsOnTheFieldsColumn`（浅/深两臂）+ opt-in 出图
  `TestDumpP13DBSheetCaptionLine`（`FLOTER_P13DB_DUMP=1`）。

**量过、不改**（两处，都写进报告不进代码）：

1. 分组标题的字号仍是 body 的偏移量（`t.FontSize+2` = 12.8），阶梯上是 `tokens.Title`（12.6）：
   在装了设计主题的取景里**同一个行盒**（`h=18`，差分里一个像素都没变）。而换成阶梯会让
   `sheetcutedge_test.go` 的 `TestThePluginSheetEdgesAddNoGeometry` 变红——不是因为设计变了，
   是因为**那条 pin 的取景没装设计主题**（`pluginSheetFrame` 只取了 `tokens.Theme.Background`），
   它把 `t.FontSize` 读成框架库存的 14，量到的标题盒是 `90×22`（本仓是 `96×17`）。
   这是 P13-cm「pin 必须 `c.SetTheme(a.tokens(c).Theme)` 取景」那条口径教训的**第二次**
   现形，记在 §6.71 的「仍待」里：**先修取景，再谈字号**。
2. 浮层标题 `t.FontSize`（body）：它是面的名字，挂在浮层列上，**故意不做**。

## 3. V / B / S 台账

- **V 0**：没有既有视觉数值被重定基线（`TestThePluginSheetEdgesAddNoGeometry` 在最终形态下全绿，
  一条既有 pin 没动）。
- **B 0**：`go test -count=1 ./...` 全绿；凡是钉行为的 pin 零改动。
- **S 0**：结构没变（还是那个 `ui.Text`，只加了内边距）。

## 4. 门槛（实测）

| 项 | 实测 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 / 净 / 净 |
| `go test -count=1 ./...` | 全绿（中途 `TestThePluginSheetEdgesAddNoGeometry` 红过一次，见 §2-1，收窄改动后转绿） |
| PASS | **3307**（基线 3304，+3 = 新 pin 的两臂 + 它自己） |
| FAIL | 0 |
| SKIP | **144**（基线 143，+1 = 本轮 opt-in 出图） |
| 顶层测试函数（`^=== RUN   Test…$` 口径） | **2452**（基线 2450，+2） |
| `GOOS=linux/windows/darwin go build ./...` | 过 / 过 / 过 |

## 5. mutation（2 条，实跑变红 + 逐字节还原）

**M1**（把三处改动整份退回 HEAD = 改前的产品）：

```
--- FAIL: TestTheSheetsWriteFailureHangsOnTheFieldsColumn (0.10s)
    --- FAIL: TestTheSheetsWriteFailureHangsOnTheFieldsColumn/light (0.09s)
        sheetcaptionline_test.go:78: the write-failure line's ink starts at x=14.0 — 16.0 from the fields' column (x=30.0) and 0.0 from the sheet's own column (x=14.0): it reports a write the fields refused, so it hangs where they do, not on the sheet's frame
    --- FAIL: TestTheSheetsWriteFailureHangsOnTheFieldsColumn/dark (0.01s)
        sheetcaptionline_test.go:78: the write-failure line's ink starts at x=14.0 — 15.0 from the fields' column (x=29.0) and 0.0 from the sheet's own column (x=14.0): ...
```

**M2**（给浮层**标题**行加 `Padding(0, t.Space(3.5))`，让名字也跟进字段列）：

```
--- FAIL: TestTheSheetsWriteFailureHangsOnTheFieldsColumn (0.12s)
    --- FAIL: TestTheSheetsWriteFailureHangsOnTheFieldsColumn/light (0.11s)
        sheetcaptionline_test.go:78: the write-failure line's ink starts at x=27.0 — 3.0 from the fields' column (x=30.0) and 0.0 from the sheet's own column (x=27.0): ...
        sheetcaptionline_test.go:86: the sheet's title starts at x=27.0, want the sheet's own column x=14.4
    --- FAIL: TestTheSheetsWriteFailureHangsOnTheFieldsColumn/dark (0.01s)
        sheetcaptionline_test.go:86: the sheet's title starts at x=27.0, want the sheet's own column x=14.4
```

两条都用 `cp` / 逆变换还原 → `md5sum -c` 逐字节核对（`internal/settingsui/pluginsheet.go: 成功`），
再 `touch` 测试文件复跑 → `ok floter/internal/settingsui`（不吃缓存）。

## 6. 出图

`FLOTER_P13DB_DUMP=1 go test -run TestDumpP13DBSheetCaptionLine ./internal/settingsui/`
（`FLOTER_P13DB_TAG=before` 是在改前代码上跑的那一对）：

```
/tmp/floter-P13-db-pluginsheet-light-before.png   ←→ -light-after.png
/tmp/floter-P13-db-pluginsheet-dark-before.png    ←→ -dark-after.png
```

逐像素差分（720×900）：**浅 793 px / 深 794 px，bbox `(14,42)-(152,53)`**——差分只落在写失败线
那一行的墨上，没有第二条边、没有第二个盒子动。

**这张图上看到的一处变化**：题头下那行暖色的 `Could not save that change.` 从贴着左边界
（和「Clipboard history」同一个字头）右移了 13px，落在「Keep a clipboard history」这条分组
标题的字头上；它下面每一条字段的标签都没动。

## 7. 用了设计 skill（用户 2026-10-30 要求）

读的路径：`SKILL.md`（Router Rules 的「已有方向，只要打磨」车道）→
`references/user-preferences.md`（P-1 斜体硬禁令、P-5 可读性优先）→
`references/craft-loop.md`（§二工艺密度菜单 + §三两轮制四视角评审）→
`references/preflight.md`（A / B / C / D2 的条目）。**没有**走 Phase 2 的四方向预览车道，
没有出 HTML 预览页。

### 偷了什么（逐条落到具体数值）

1. 从 `craft-loop.md` §二工艺密度菜单取 **第 6 项「编辑细节」**（菜单原例：hairline、
   页边注、大写字距标签）→ 落成「**句子挂它讲的那一列**」这个动作：`Padding(0, Space(3.5))`
   = **12.6px**（出厂档），与分组标题的缩进逐字节同一个数，于是墨从 `14.0` → `27.0`。
2. 同菜单取 **第 12 项「材质细节」**（菜单原例：内边框高光、接缝）→ 落成「**一块浮层只有
   两条列**」这条判断：名字在浮层列（`t.Space(4)` = **14.4**）、分组与字段在卡列
   （`27.0` / `29–30`）——层级用列表达，不新增一条线、不新增一块面。
3. 同菜单取 **第 5 项「多层染色阴影」的反面**（菜单原例是「拒绝单层灰阴影」）→ 落成
   「**这一处不戴任何材质**」的显式取舍：这条句子是 caption 墨 + `Warning`，不加面、
   不加冠线、不加位移（浮层里的 E1 由字段槽与胶囊承担，句子是句子）。
4. 从 `craft-loop.md` §三的**四视角评审**取「工程师：哪里对不齐/间距不一致」这一问
   → 落成量法本身（`inkLeft` 扫墨，而不是比盒子）。
5. 诚实记账：12 项菜单本轮落 **3 项**，低于 ≥5 的密度要求。理由与 P13-cz / P13-da 相同——
   这是一轮**一个句子、一条列**的收口轮，硬凑 5 项签名动作必然越界到材质与 accent。

### preflight 逐条结论

- **A 反 AI 套路**：无渐变文字 / 无纯黑 / 无外发光 / 无斜体（P-1）/ 无三等分卡片 /
  无左侧竖线装饰 / 无 emoji。**过**。
- **A2 进攻三步**：DNA 注入 ✔（本仓的 `liquid-glass.md` §4 刻度，不是新造的视觉）；
  工艺密度 **3/5**，如实记在上面并说明原因 ✖；两轮制「量 → 判 → 只改一处」✔。
- **B 设计读取一致性**：pin 用 `renderPluginSheet` 的取景（**装了设计主题**），量的是墨与列；
  深浅两套同一条结论。**过**。同轮发现**另一条** pin 的取景没装主题（§2-1），已挂「仍待」，
  不在本轮偷偷改它。
- **C 动效抽查**：本轮不含动效面。**不适用**。
- **D2 中文页**：本轮没有改任何文案，中文页一个字没动；但这条句子在中文页上仍是
  `tokens.Caption`（11px），低于 D2 的「正文 ≥ 14px」——本仓既有口径，第 28 份证据，
  **需拍板**，不许擅自改。**结论：字号一项不过，原因是需拍板项，已挂账。**

## 8. 仍待

1. **先修取景（不需要拍板，下一格的首选）**：`sheetcutedge_test.go` 的 `pluginSheetFrame`
   没装设计主题，于是 `TestThePluginSheetEdgesAddNoGeometry` 量的是框架库存的 `t.FontSize`
   （14），标题盒钉在 `90×22`（生产是 `96×17`）。修法：`c.SetTheme(a.tokens(c).Theme)` 后
   重新钉 `MaxY` 与标题盒，同轮把弹层剩下三处偏移量字号（`t.FontSize+2` / `t.FontSize+1` /
   `t.FontSize-1`）一起换到阶梯——**那时才是可测的**。
2. **需拍板（沿用 + 第 28 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：会话行 cwd 有无导致 56/72 两种节拍；扩展页空态没有出路；会话页空态
   说错对象；终端页会话抽屉 / 标签对失败只加字不换墨；终端外观抽屉行要不要面；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（第三十次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
