# P13-cx 报告：同一张卡上的第二块——Freshness 从卡框收回卡的行列

轮号 **P13-cx** ｜ 选题：通用队列 `空间感`（P13-cw「仍待」第 1 条）｜
规格 `docs/rounds/P13-cx-spec.md` ｜ 合约新增 **§6.67**。

## 1. 一句话

**这一轮为什么让它更有空间感**：Freshness 块此前左右都贴着卡自己的框（左 `192` = 卡面第一像素，
右 `701` = 卡框线），它的四行读起来是「卡的一部分」；收进 `rowInset` 那一列之后，块与框之间
各留 8px 空气，块的四行和它上面的行标签站在同一条竖线上——**块成了「属于这一行」的内容**，
卡的内部才第一次分出「行 / 块的列 / 卡的框」三层，而不是一层。

## 2. 先量（改前真实数字，深浅同坐标）

渲染：`freshnessPage`（720×1700，Integrations 页，一张带 freshness + health 的集成卡）。

| 事实 | 改前 | 改后 |
|---|---|---|
| 卡面横向 / 框线 | `192..700` / `191`,`701` | 不变 |
| 行标签 `Health` | `200..617` | 不变 |
| 块标签 `Freshness` | **`192..244`** | **`200..252`** |
| `Last probe` / `Result`（cell） | `192..440.5` / `452.5..701` | `200..440.5` / `452.5..693` |
| `Commands` | **`192..245`** | **`200..253`** |
| 纵向 `Freshness` / `Last probe` / `Commands` / `Health` | 394 / 409 / 449.5 / 478 | **完全一致** |

纵向的紧贴是关键：`Freshness` 的行盒底 `409` 正是 `Last probe` 的行盒顶 `409`——这四行
本来是**零间隔兄弟**。

## 3. 改法

`internal/settingsui/settingsui.go` `freshnessSection`：把四个直接子元素包进

```go
block := ui.Column(c).FillWidth().Gap(0).Padding(0, a.rowInset(t))
```

- `Padding(0, rowInset)`：块进入卡的行列（左 `200`、右 `693`），与 §6.66 的健康块同一个列。
- `Gap(0)`：**必须零**——主题的 8px gap 会让块内四行凭空长 8px，那是改坏不是打磨。

**零改动**：字阶、字重、墨（`TextSecondary` / `TextTertiary` / `TextStrong` / `T.Muted`）、
两列网格 `GapX(Space(3))`、那枚 `·`、健康来源注脚的 `Wrap()`、i18n 文案、交互与命令接线。

## 4. 新 pin（深浅各一份）

| 测试 | 钉什么 |
|---|---|
| `TestTheFreshnessBlockSitsInTheCardsLabelColumn` | 块左 = 卡上行标签左（inset 从同一帧像素读，不写死 x）；块内 `Last probe` / `Commands` 与块标签同列；元数据网格右端 = 卡内容右端；**元数据行盒紧贴块标签行盒**（纵向零变化） |
| `TestTheFreshnessNoteWrapsInsideTheColumn` | 健康来源注脚在自己的列里换行，左端同列、右端不越过卡内容右端 |

两条 pin 的坐标全部从 `tt.Find` + 像素扫描里读：卡面左右与 `rowInset` 都不写死，
所以改主题/改卡宽不会误绿。

## 5. 出图（视觉复核）

`/tmp/floter-P13-cx-freshness-{light,dark}-{before,after}.png`
（`FLOTER_P13CX_DUMP=1 FLOTER_P13CX_TAG=before|after go test -run TestDumpP13cxFreshness`）。

- 差分：浅 **2454 px** / 深 **2455 px**，bbox `(192,397)-(416,463)`——正好是块的四行文字带，
  纵向一个像素都没进 bbox 之外的位移。
- **这张图上看到的一处变化**：改前 `Freshness` 的 `F` 几乎贴在卡的左内框上（只隔 1px 框线），
  和下面 `Last probe` 的 `L` 挤成一条贴着框的竖边；改后这一列文字右边多出 8px 空气，
  与上方 `Health` 的 `H` 对齐成同一条竖线，卡内部读起来有「行标签列」这一层。
  右端 `Result` 的值（`Succeeded`）墨迹没动，动的只是它所在格的盒子——所以差分 bbox 右端停在 `416`。

## 6. 用了 qiaomu-design（打磨模式）

读：`SKILL.md` → `references/user-preferences.md` → `references/craft-loop.md` →
`references/preflight.md`。走「已有方向，只要打磨」的自动车道（用户已把方向定死为轻便液态玻璃），
**不走** Phase 2 的四方向预览、不等点选、不出 HTML 预览页。

### 偷了什么（逐条：菜单项 → 落成的具体数值）

| 偷自 | 落成 |
|---|---|
| `craft-loop.md` §一 DNA 注入：**偷参考自己的原文数值，不是「像参考」** | 板的列 = `.extension-detail-block` 的 padding，与它自己的 `h4` 同一条（`floter-main:src/styles/extensions.css:1997-2002` 那 12px 里的水平一半，即本仓 `rowInset` = `Space(2)` = **8px**）；这与 §6.66 的健康块是同一个数 |
| `craft-loop.md` §二 工艺密度菜单 **第 6 项「编辑细节：hairline / 留白节奏」** | 块的左右各 **8px** 空气（`Padding(0, 8)`），块的右端从 **701 → 693**（卡内容右端），不再压住卡框线 |
| `craft-loop.md` §二 菜单 **第 12 项「材质细节：内边框高光」的反面**——本仓不伪造玻璃（§0 禁令） | 本轮**不加**任何新面/新线/新阴影：层级只靠「列」这一件事表达。`Padding` 是几何，不是墨 |
| `craft-loop.md` §三 两轮制：先渲染 → 指问题 → 只修这几处 | 第一轮渲出 `before` PNG 并量 x（`192` vs `200`）；四视角里**工程师**挑出「块贴框」、**艺术总监**挑出「卡内部没有列」；第二轮只改一个 `Padding` + 一个 `Gap(0)`，不出第二处改动 |

工艺密度对本轮（单块列修正）只取「编辑细节」与「DNA 注入」两项 + 几何纪律一项；
其余密度项（氛围层 / 焦点环 / 签名交互 / 阴影栈）在本轮会变成 §0 明令禁止的伪造玻璃，
按 `craft-loop.md` §四 的取舍顺序（两轮制 > DNA 注入 > 密度凑满）主动放弃，并在此写明理由。

## 7. preflight 逐条结论

- **A 反套路**：过——无渐变、无纯黑、无发光、无斜体（`noitalic_test.go` 全仓 pin）、无三等分卡、
  无 AI 空词；本轮零新增文案（i18n 一个字符串都没动）。
- **A2 进攻三步**：DNA 注入 = 参考 `extensions.css:1997-2002` 的同一串数（8px 列）；两轮制 = 见 §6；
  密度 ≥ 5 项按上面理由主动弃权（单一几何修正轮，硬凑会违反 §0 禁令）。
- **A2.1 执行者**：当前 agent 直接执行，未调用 K3（用户未在本任务点名）。
- **B 设计读取一致性**：过——读取是「近黑底 + 面差 + 发丝线的轻便液态玻璃」，产出只动列，
  没有引入第二种材质；整帧仍只有一个明暗主题（浅/深分别出图）。
- **C 动效抽查**：不适用——零动效、零交互面（`Transition` 一个都没加）。
- **D2 中文页**：过（未回归）——`tokens.Caption` 11px 与「正文 ≥ 14px」的冲突是**沿用**的
  「需拍板」项（第 25 份证据），本轮没有新增或放大它；中西文空格、全角标点、字重均未触碰。
- **E 三秒测试**：改前这一块的视线被卡左内框那条 1px 线带走（文字贴着框）；改后最先看到的是
  `Freshness` 与 `Health` 同一条竖列，块属于行、行属于卡，层级读得出来。
- **P-38（左侧竖线装饰）**：不适用——本轮没有加任何竖线；§6.66 的 2px 暖棱是**警示**语义
  （参考的 `border-left: 2px solid var(--text-warning)`），不是选中/卡片强调。

## 8. 门槛（本轮实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...   → 净
go test -count=1 ./...                                 → 全绿（FAIL 0）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 3298（基线 3296）
--- SKIP                                               → 142（基线 141，新增 opt-in 出图）
测试函数（grep -rc "^func Test" --include=*_test.go）  → 2450（基线 2447，同法实测）
GOOS=linux / windows / darwin go build ./...             → 全过
```

## 9. 变异（实跑 3 条，全部变红并逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| M1 | `Padding(0, a.rowInset(t))` → `Padding(0, 0)` | 列 pin 四红：`light: the freshness block starts 0px inside the card's face (x=192), the row labels 8px (x=200): the block hangs on the frame` / `light: the metadata grid ends at x=701, want 693 (the card's content edge, face 192..700, inset 8)`（深浅各一份）；注脚 pin 两红：`light: the note runs to x=701, past the card's content edge 693 (face 192..700)` |
| M2 | 过冲臂 `Padding(0, a.rowInset(t)*2)` | 列 pin 四红：`starts 16px inside the card's face (x=208)…` / `ends at x=685, want 693` |
| M3 | `Gap(0)` → `Gap(t.Space(2))` | 纵向 pin 两红：`light: the metadata labels start at y=417, the block label ends at y=409: the block grew a gap of 8px` |

还原：`md5sum -c` 逐字节核对（`settingsui.go` `aeda3620…`「成功」），`touch` 后复跑两条 pin 绿
（不复用测试缓存）。

## 10. 台账

- **V 0**（没有旧设计数值被重定：两条 pin 全是新增）。
- **B 0**（行为/度量/接线零改动：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线）。
- **S 0**（结构只多了一层包装列，块与卡、块与行的父子语义没变）。
- 产品代码 1 文件 1 处 3 行；测试 1 文件 3 个函数（2 pin + 1 opt-in 出图）。

## 11. 仍待（P13-cx）

1. **下一格（先量）**：同族的 `metadataCell` 那一组在**别的**卡（设置卡 / 运行表单）里是否也
   直接画在卡体列——`metadataCell` 只有本块一个调用点，但 `freshnessSection` 之外还有
   `integrationMeta`（`.extension-row__meta`）与 `packageChip`：先量它们是否也贴框。
2. **需拍板**：探针列表首行上方要不要一道线（沿用 P13-cw 第 2 条）。
3. **需拍板（沿用 + 第 25 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：扩展页空态只有一行结论、没有出路；会话页空态说错了对象；
   终端页会话抽屉 / 标签对失败只加字不换墨；终端外观抽屉行要不要面；
   `launcher/filters.go` 过滤 chip 与 `launcher/power.go` 确认带 chip 的按下档；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
5. **记一笔（不改）**：`rowFace.Muted`（`.settings-row--muted` 的 0.62）当前**零调用点**。
6. **抢救 stash 核对（第二十六次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
