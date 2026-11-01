# P13-bt · 报告：只读行的墨——数据不当标题，标题不当值

> 规格 `docs/rounds/P13-bt-spec.md`｜合约 `docs/liquid-glass.md` §6.38（新）
> 产品改动：`internal/settingsui/rows.go`（新 `dataRow` / `labelRow`）、
> `internal/settingsui/settingsui.go`（3 个调用点）。

## 一、这轮为什么让它更轻、更有空间感

一张卡片上「谁响」必须等于「谁是答案」。此前集成页有两处**没有控件**的行反着戴墨：一条
二进制**路径**和一块**标题**，都和它们上面/下面的答案一样响——读者读到的层级是平的，
于是「空间感」只能靠画框补，越补越旧。把强调档收回到答案身上之后，同一条卡片里第一次有了
**三档墨**（值 202 / 名字 122 / 数据 101），层级由**墨**分家而不是由线分家。

## 二、先量（改前，`inkSpan` = 文本盒内最亮−最暗的 luma 差）

| 表面 | 读数（浅 / 深） |
|---|---|
| Detected 行：工具名 `Ghost Tool` | **202 / 198** |
| Detected 行：路径 `/usr/bin/ghost` | **208 / 208** ← 与工具名同档 |
| Detected 行：描述 | 121 / 121 |
| 审批块：标题 `Approval record` | **202 / 198** ← 与三个值同档 |
| 审批块：事实名字（`Approved at` 等） | 122 / 115 |
| 审批块：事实值（时间 / 摘要） | **202 / 198** |
| 局部工具表单：边界说明 | 121 / 121 |
| 局部工具表单：权限开关行 | 208 / 208 |

两条结论：①路径比描述响 87 级、与工具名同档；②块标题比它引入的值**逐级相同**、
比这些值的名字响 80 级。第三组（边界说明 121 vs 开关 208）**不是缺陷**——散文比控件轻是对的。

## 三、落地

| # | 位置 | 落成 |
|---|---|---|
| 1 | `rows.go` `dataRow`（新）；`settingsui.go:5859` | 路径改走 `cardRow` + 一行 mono `Caption`/500/`TextMuted`（与会话页 cwd 的 `rowLine{Mono:true}` 同一个函数值），单行 `Row.FillHeight().AlignItems(ui.Center)` 在 44px 行盒里居中（同 `keyValue`） |
| 2 | `rows.go` `labelRow`（新）；`settingsui.go:2440` | 块标题改走 `Caption`/620/`TextSecondary`——与本块自己的 `Enforced` / `Disclosure` 组标签同一个函数值 |
| 3 | `settingsui.go:2465` / `:2477`（`ApprovalRecordPermissions` 两条臂） | 同一个「没有控件的组标签」形状 → `labelRow`，与 #2 同档；无记录臂的 `None` 作为 `sublabel` 落在同尺寸的安静档（500/`TextMuted`） |

**只换墨，不换盒**：行框架、44px 地板、分隔线、1px 行距、卡片高度逐像素不变（下面的像素
diff 盒就是证据：diff 只落在文字行内，没有一行落在卡片边或规则线上）。

## 四、改后实测

| 表面 | 改前 | 改后（浅 / 深） |
|---|---|---|
| Detected 路径 | 208 / 208（18px 行盒） | **101 / 94**（15px 行盒，居中） |
| 审批块标题 | 202 / 198 | **121 / 114** |
| 权限标签 | 202 / 198 | **121 / 114** |
| 事实名字 | 122 / 115 | 122 / 115（不动） |
| 事实值 | 202 / 198 | 202 / 198（不动） |

## 五、V / B / S 台账

- **V 0**：`go test ./internal/settingsui/` 一次全绿，没有任何钉旧数值的 pin 需要重定基线。
- **B 0**：行为/度量/接线 pin 零改动全绿（`integrationrowgeometry_test.go` 的 `ApprovalRecord`
  只读 x 列，`belowcolumn_test.go` 只读 x 列，两处读数不变）。
- **S 0**：没有结构断言需要改（`dataRow` / `labelRow` 仍走 `cardRow`，行框架没变）。

## 六、门槛（全部实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 44 个包 ok，0 FAIL
--- PASS 计数                → 3213（基线 3210，+3）
--- SKIP 计数                → 118（基线 117，+1 = 本轮 opt-in dump）
测试函数数                   → 2367（HEAD 2363，+4 = 3 pin + 1 dump）
GOOS=linux|windows|darwin go build ./... → 三平台全过
```

## 七、变异（3 条实跑变红 + `md5sum -c` 逐字节还原）

还原基线：`rows.go` `d080813825053d40d25bef4098937f2a`、
`settingsui.go` `2d3535ee91521377ee213e7dcc45373e`（`md5sum -c` 两行全「成功」）。

| # | 注入 | 实测失败串（原样） |
|---|---|---|
| M1 | `dataRow` 回到旧标题墨（`FontSize(t.FontSize+1)` / 580 / `TextStrong`） | `readonlyink_test.go:78: light: the path resolves 202 levels and the tool's name 202 — the path must sit a whole rung below the name (§6.38)` |
| M2 | `labelRow` 回到旧标题墨（`FontSize(t.FontSize+1)` / 580 / `TextStrong`） | `readonlyink_test.go:130: light: the block's title resolves 202 levels — a title over a record is a label, not the answer (§6.38)`（并连带 `:134` 的「标题不得与值同档」） |
| M3 | `dataRow` 的墨从 `TextMuted` 抬到 `t.Text`（爬到名字那一档） | `readonlyink_test.go:78: light: the path resolves 202 levels and the tool's name 202 …` + `:87 the path resolves 202 levels and the description above it 101` |

三次注入后都 `cp` 回备份、`md5sum -c` 逐字节核对成功，并 `touch` 两个文件后复跑一次确认回到绿。

**一个过程记录（写在报告里，免得下轮再踩）**：`codeRow` 与 `dataRow` 的文本行前缀逐字相同
（`ui.Text(c, value).Font(deepLinkValueFont).FontSize(tokens.Caption)`），第一次的 M3 打在了
`codeRow`（深链胶囊）上，跑出来「零变化」；换成 `TextColor` 之后半段才定位到 `dataRow`。

## 八、出图

| 文件 | 说明 |
|---|---|
| `/tmp/floter-P13-bt-detected-{before,after}-{dark,light}.png` | Detected 列表（720×1600） |
| `/tmp/floter-P13-bt-approval-{before,after}-{dark,light}.png` | 审批记录块（760×1400） |

逐像素 diff（PIL，RGB 全等才不计）：detected 深 **667** / 浅 **667**，差异盒
`x[215..298] y[298..313]`；approval 深 **1597** / 浅 **1597**，差异盒 `x[195..329] y[438..664]`。
**这张图上看到的一处变化**：Detected 卡片里 `/usr/bin/ghost` 那一行从 18px 高的标题字缩成
15px 的等宽小字、并且从贴顶变成在行盒里居中（y 294.1 → 299.6），它上面工具名那行一个像素没动；
审批块里只有标题行与权限标签行变淡（202 → 121），三个事实值与卡片的边、规则线全不动。

## 九、用了 `qiaomu-design`（打磨车道，不走 Phase 2 预览）

读的路：`SKILL.md` → `references/user-preferences.md`（P-1 禁斜体 / P-5 可读性 / P-7 功能契约
→ 本轮都不触碰：没有新增斜体、没有把 9.9px 的 `Caption` 再往下压、没有改任何交互）→
`references/craft-loop.md`（材质·空间）→ `references/preflight.md`。

**偷了什么（逐条对工艺密度菜单）**

1. **第 3 项「类型刻度：只允许 N 档，一页里不许出现第 N+1 档」** → 落成**两个具体函数值**：
   数据 = `Caption`/500/`TextMuted`（= `rowLine{Mono:true}`）、块标签 = `Caption`/620/
   `TextSecondary`（= 本块 `Enforced`/`Disclosure`）。不是新造档，是把已有的两档用在该用的地方。
2. **第 6 项「编辑细节：hairline、大写字距标签、页边注」的反面——「层级靠结构不靠音量」** →
   落成验收：块标签与它下面的名字**同尺寸**，只靠字重（620 vs 500）+ 行框架（44px 行 + 规则线）
   分家；实测两条标签 121/114 与名字 122/115 只差 1 级，这是**故意**的。
3. **两轮制（Render → Critique → Refine）的工程师视角**（「谁在这张卡上最响」）→ 这轮唯一的
   refine 就是它；艺术总监视角的同一处说法是「一张规格表不该读成五行一样响」。

**preflight**

- **A（反 AI 套路）**：过。没有新增渐变、白边、模糊、`glass` 叠 `glass`；没有加装饰、没有加图标；
  两处改动都只是把一段文字换到已有的一档墨上。
- **B（设计读取一致性）**：过。只用既有 token（`tokens.Caption` / `TextSecondary` / `cardTheme().TextMuted`
  / `deepLinkValueFont`），没有写死色值或尺寸；深浅两套读数逐位对应（101/94、121/114），
  说明没有引入明度相关分支。
- **C（动效抽查）**：**不适用（本轮零动效）**——`dataRow` / `labelRow` 没有 transition/animation，
  行的 hover/press 只换面；`go test ./internal/settingsui/` 的 `FramePending` 相关 pin 全绿。
- **D2（中文页）**：**过一半、一条老账**。本轮只换墨与字重，不改字号档位（`Caption` 仍是
  `Caption`），所以中文页的字号张力既不因本轮变好也不变坏；`tokens.Caption` 9.9px 与
  preflight「正文 ≥ 14px」仍是**需拍板**项（第 8 份证据：本轮把「数据/标签用 caption 档」
  又钉了两处，动字号会连带这两条线，所以更要先拍板再动）。

## 十、仍待

1. **需拍板（第 8 份证据）**：`tokens.Caption` 9.9px / `tokens.Body` 10.8px 与 D2 的「正文 ≥ 14px」。
   本轮把「路径/块标签 = caption 档」钉成了像素事实，改字号必须同时重定这些线。
2. **下一格候选**：`settingsui.go` 的 `a.row(c, "", "", func() {})`（`:2788`，控件独占行）与
   `stackedRow` / `actionRow` 左右内边距的三种写法（P13-bs 记的账）——两者都属「间距节奏」，
   **先量再改**。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；路径截断保头弃尾；终端弧 S1b / S1c / S2 尾；§6.32 的 32-4。
4. **P13-bo 仍待 1 原样挂着**：列表从属注 / 这一页结论的分界要写成共享判据，跨表面，另开一轮。
5. **抢救 stash 复核**：`stash@{0}` = `33e03d1f6a`，其 tree 与 `6bee2c19`（P13-bp）**逐字节相同**
   （`git rev-parse stash@{0}^{tree}` == `git rev-parse 6bee2c19^{tree}` == `6d442dc5310b…`），
   即那次抢救没有带来任何未提交工作，无需接续（P13-bs 已记同一条，本轮复核仍成立）。
