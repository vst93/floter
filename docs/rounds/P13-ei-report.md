# P13-ei · 浮层的名字是整块面上最淡的一行（墨与字重，不动字号）

选题：`P13-eh`「仍待」第 1 条（先量浮层的分组标题与设置页的分组标题是不是同一角色）。
量完的结论是**不是**（浮层的分组标题穿的是设置页**卡标题**那一档：`Title + 620 +
TextStrong` + 8u 间距，见 `pluginsheet.go:143-153` 与 `rows.go:1240` `rowTitle` 的对齐关系），
真正同一角色两个拼法的是**一页的名字**。规格 `docs/rounds/P13-ei-spec.md`，
合约新增 **§6.104**。

## 先量（设计主题、720×600、`pluginSheetFrame`、`scroll.Y = 0`，浅色档最暗像素）

| 行 | y 带 | 最暗像素 | 用的 |
|---|---|---|---|
| 浮层的名字（`Probe plugin`） | 67..78 | **128** | `TextMuted` `#808087` |
| 第一个分组标题（`First section`） | 100..109 | **27** | `TextStrong` |
| 字段标签（`Toggle number 1`） | 135..143 | **27** | `TextStrong` |
| 字段 help 行 | 144..156 | **57** | 二级墨 |

名字比它下面**每一条 help 都淡**（128 对 57），比它自己带的四个分组标题淡 **101 级**；
而 `pluginsheet.go` 原来的注释写着「the hierarchy a reader sees is name (body, muted) >
section (title) > field (label) > help」——注释说的顺序和像素给的顺序是反的，名字排最后。
同一角色在设置页页头是 `Display / 650 / Text`（`settingsui.go:1572`）。

## 落地（产品代码 1 行 + 注释 1 段）

`internal/settingsui/pluginsheet.go:97`：
`FontSize(t.FontSize).FontWeight(600).TextColor(t.TextMuted)`
→ `FontSize(t.FontSize).FontWeight(650).TextColor(t.Text)`。**字号一个字节没动。**

**为什么不动字号**（本轮的核心取舍）：名字行是 `Height(Space(7))` = 25.2px 的行盒，
名字的墨从视口顶第 **11** 行起，而 §6.72 的顶边高正好 11 行（3u = 10.8px），余量 **0.2px**。
把字号长到 `Title`(12.6) 会立刻撞上那条 pin——**实跑证明**（见变异 M3）：
`blank ends: top 11 rows at rest 45 (the row past them 219)`，
`the sheet parked at its start has ink inside its first 11 rows (spread 45)`。
所以「名字站到阶梯顶端」这一条**不进本轮**，写进「需拍板」（它要动行盒 7u 或顶留白 1u，
属于几何，不是墨）。

## 新 pin

`internal/settingsui/sheetnamerung_test.go`
`TestThePluginSheetNameIsNotTheQuietestLineOnItsOwnFace`（深浅两套，三条判据）：

1. **墨**：名字相对自身平面的对比度，必须与它自己带的分组标题同一档（±4）。
   改前 118 对 219（差 101）→ 改后 219 对 219（差 0）。深色档 219/222。
2. **几何**：名字的墨仍从视口顶第 11 行起（`name.y - band.Min.Y == 11`）。
3. **几何**：`PluginScroll.MaxY` 仍是 316.4（±0.6）。

量法：`sheetLines` 逐行取「本行中位亮度」为平面、取本行最大亮度差，按 ≤2 行间隙并成一行
——**不是**写死像素窗口（§6.102 的教训：探针的盒子只能来自布局/像素本身，不能来自常数）。
另加 opt-in 出图 `TestDumpP13EISheetName`（`FLOTER_PNG=1`）。

## 门槛（实测）

- `gofmt -l cmd internal` → 空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → 净
- `go test -count=1 ./...` → **全绿，0 FAIL**
- `go test -count=1 -v ./... | grep -c -- "--- PASS"` → **3339**（基线 3336）
- `--- SKIP` → **168**（基线 167；新增的那条是本轮 opt-in 出图）
- 测试函数实测 **2511**（本轮新增 2：1 条 pin + 1 条 opt-in 出图）
- `GOOS=linux/windows/darwin go build ./...` → 过

## 变异（3 条实跑，逐条变红并还原）

- **M1** 名字退回 `600 / TextMuted` →
  `name y=67 peak=118, section y=100 peak=219` /
  `the sheet's name reads 118 levels off its plane while the section heading under it reads 219
  (a 101-level gap): the name is the quietest line on the face it names … (want ≤4)`（深浅各一条）。
- **M2** 只升一档到 `tokens.TextSecondary` →
  `name y=67 peak=139, section y=100 peak=219 (a 80-level gap)` —— 一档不够，这条 pin 分得出来。
- **M3** 顺手把字号换成 `tokens.Title` →
  `TestThePluginSheetEdgesSitOnBlankAtBothEnds` 红：
  `blank ends: top 11 rows at rest 45 (the row past them 219)` /
  `the top edge would wash the row the user is reading … (want ≤4)`（浅 45 / 深 46），
  同时新 pin 的判据 2/3 也红。**这条变异就是「为什么不动字号」的证据。**
- 三条都用 `cp` 备份 → 改 → 跑 → 还原 → `md5sum -c`（`443551f20d5c071734d34836223fcbf6`）
  → `touch` 再跑。

## 出图（自己复核过）

`/tmp/floter-P13-ei-sheet-{light,dark}-{before,after}.png`（`FLOTER_PNG=1` 的
`TestDumpP13EISheetName`；before 是 M1 变异那一跑的同一帧）。

**这张图上看到的一处变化**：两张 720×600 的帧只有 **460 个像素**变了，全部落在
`x 18..87, y 67..78` —— 正好是名字那一行，别的什么都没动。浅色档那一行的最暗像素
**130 → 27**，这一行的墨像素（<200）**211 → 293**；深色档同一行里「亮像素」（>120）
**39 → 196**。名字从「脚注灰」变成「页头的墨」，而它下面 100 行以外的分组标题与字段
一个像素未动。

## 用了 qiaomu-design（`SKILL.md` → `references/user-preferences.md` → `craft-loop.md` + `preflight.md`，走「已有方向，只要打磨」那条路）

**偷了什么**（逐条：菜单项 → 落成哪个具体数值）：

1. **参考 DNA 注入（一）**：方向已由用户钉死（轻便液态玻璃），所以 DNA 供体取本仓**自己**
   已经落地的那一页——设置页页头 `Display / 650 / Text`（`settingsui.go:1572`）。
   偷其中两项进浮层页头：**650** 与 **`Text`**，第三项（字号 `Display`）**故意不偷**，
   理由见上面 0.2px（这是「偷 3-5 个值」里唯一被环境否掉的一个，写明而不是静默丢）。
2. **工艺密度第 6 项「编辑细节：hairline / 大写字距标签」** → 名字是这块面的「标签语法」，
   它的拼法必须与页头同族，而不是自成一套脚注墨；落成 `Text` + 650 这一对具体值。
3. **工艺密度第 12 项「材质细节」的反向用法** → 材质/面档只在需要分层的地方出现；
   用「变淡」表达层级等于拿材质当结构用，收回到墨与字重（`liquid-glass.md` §2 同一条判断）。
4. **三·两轮制** → 第一轮只做「数清楚这块面上有几条墨、各自多响」（128 / 27 / 27 / 57 那张表），
   第二轮只改**一处**产品代码，不顺手改字号、不顺手改间距、不顺手改分组标题。
5. **四视角评审（工程师视角）** → 挑出的最弱一处正是「名字是整块面上最淡的一行」；
   读者视角复核：眯眼看，这块面顶部此前没有可抓的锚点。

**preflight 逐条结论**：

- **A（反 AI 套路）**：通过 —— 不产界面、没有渐变/glow/描边字；`liquid-glass.md` §0 的四条
  硬禁令（模糊贴图冒充玻璃 / glass 叠 glass / 渐变白边当高光 / 组件层自造透明）一条未触。
- **B（设计读取一致性）**：通过 —— 一个仓库一个调色板来源（`internal/theme`），本轮用的
  `t.Text` 是既有 token，没有字面色值；浮层页头与设置页页头此后读同一个「名字」档。
- **C（动效抽查）**：不适用 —— 不动动效；新 pin 的 `SetDark` 不带 `Advance`（读静态像素）。
- **D2（中文页）**：**未过（沿用，第 58 份证据）** —— `tokens.Caption` 9.9px / 正文 10.8px
  对「正文 ≥ 14px」。本轮**没有**动字号（字号是「需拍板」项），所以这条不会因为本轮而变。

## 仍待（P13-ei）

1. **下一格（首选）**：`liquid-glass.md` §6.104 表里这一族还剩一条 **需拍板**——
   「浮层页头站到阶梯顶端（`Title` 或 `Display`）」。它要动 `Height(Space(7))` 行盒或
   scroller 顶留白 `Space(1)`，才能补回 §6.72 那 0.2px 的顶边余量；几何不是墨，不许擅自改。
   不拍板时，下一格回通用队列的**间距节奏**（`internal/launcher` 与 `internal/settingsui`
   各量一次同一角色的间距，先量再改）。
2. **需拍板（沿用）**：`tokens.Caption` 9.9px 对 preflight D2「正文 ≥ 14px」（第 58 份证据）/
   浮层页头字号上阶梯（本轮新增，见上）/ 便条行 rank 1.28 / 终端会话抽屉失败只加字不换墨 /
   扩展页空态只有一行结论没有出路 / 会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 /
   `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾。
3. 抢救 stash（第六十二次）：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）仍判定
   与 HEAD 无未接续内容，本轮未试合、未静默丢弃。

## 台账

**V 1 / S 0 / B 0**。产品代码 1 行（墨 + 字重），注释 1 段；测试新增 2 个函数。
没有容差、没有 `t.Skip`（新增的那条是标注了 opt-in 的出图）、没有放宽任何既有断言。
