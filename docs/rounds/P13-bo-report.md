# P13-bo · 报告：插件关闭那一页的答案，此前画在列表行的静息墨里

- 规格：`docs/rounds/P13-bo-spec.md`
- 合约：`docs/liquid-glass.md` **§6.33**（5 行：4 `已落地` / 1 `已落地（复核）`；无 `未开始`）
- 表面：**启动器**（`internal/launcher`）
- 台账：**V 0 / B 0 / S 0**

## 一、这轮为什么让它更轻、更有空间感

不是「更亮」，是**层级只由差说话**。同一页上，一句话要么是背景（占位句、分组标题），要么是
结论（这一页的答案）。此前这一页的结论与它自己的占位句**同墨**（逐字节相同，浅 116 / 深 164），
用户读到的就是「一张空卡 + 一句灰字」——所有东西挤在同一个 z 平面上。换到陈述齿之后，
页面上只有一样东西是「答案」，其余都在它下面 **81 级**（两套外观同值）。层级不是靠加线、
加底、加框做的——这正是轻便液态玻璃这一支的做法（`docs/liquid-glass.md` §2 判据 3 的反面：
**不加层，只调面**）。

## 二、先量（改前，`ui.Tester`，720 帧、真启动器地面）

地面：浅 **253** / 深 **23**。九条读数，两套外观、中英两语**同形**：

| 表面 | 代码路径 | 字阶 / 字重 / 墨 | 浅 | 深 | 对地面（浅 / 深） |
|---|---|---|---|---|---|
| 剪贴板关闭（status 行） | `row()` 的 `item.status` 臂 | `FontSize` / 480 / `TextMuted` | **116** | **164** | **137 / 141** |
| 浏览器关闭 | `emptyMessage` | `Emphasis` / 400 / `TextStrong` | 27 | 237 | 226 / 214 |
| 浏览器无配置 | 同上 | 同上 | 27 | 237 | 226 / 214 |
| 普通无结果 | 同上 | 同上 | 27 | 237 | 226 / 214 |
| 输入框占位句「Type to search」 | 字段占位 | `FontSize` / — / `TextMuted` | **116** | **164** | 137 / 141 |
| 分组标题「Recently launched」 | `row()` 的 heading 臂 | `FontSize` / 500 / `TextSecondary` | 108 | 156 | 145 / 133 |
| 结果行标题「Recent 00」 | `rowBody` | `FontSize+2` / 580 / `TextStrong` | 27 | 242 | 226 / 219 |

三个结论：

1. **这一页的答案与输入框的占位句逐字节相同**（浅 116 / 深 164）。
2. **同一件事的两条路径相差 89 / 73 级**。
3. **`TextSecondary` 不是可用的中间档**：它与 `TextMuted` 只差 **8 / 8 级**，且深色档上
   比 `TextMuted` **更暗**（133 对 141）——同一页的「次一档」在两个外观里方向相反。
   所以这一轮是**换齿**，不是抬档。

## 三、落地（只改墨）

`internal/launcher/launcher.go` 的 `item.status` 臂：

| # | 项 | 旧 | 新 |
|---|---|---|---|
| bo-1 | 句子 | `TextColor(t.TextMuted)`（116 / 164） | `TextColor(tokens.TextStrong)`（**27 / 237**） |
| bo-2 | 图标列 | `TextColor(tokens.TextTertiary)`（144 / 112） | `TextColor(tokens.TextSecondary)`（**145 / 133**） |
| bo-3 | 盒子（30u 带 / `Space(2)` / `Space(2.5)` gap / 28u 列 / 字面 14 / `SingleLine`） | — | **一个像素不动**（高度链红线：`height.go` 的 `RowInfo.Status`） |
| bo-4 | 指针 / 键盘 / 编号 | 无底板、`rowSelectable` 排除、`assignNumbers` 不给编号 | **不动**（行为契约红线） |

**为什么 33-2 必须跟着动**：`TextTertiary` 是「比**旧句子**安静一档」的算法产物。句子上了
陈述齿之后，它在两套外观上方向还相反（浅色档比句子暗、深色档比句子亮 52 级）。换成
`TextSecondary` 后图标两套外观都在句子之下 **81 级**。

**参考的处置（写进合约）**：`.launcher-status`（`main:src/styles/launcher.css:1480-1491`）
确实是 `color: var(--text-muted)`，`clipboardStatusRow`（`main:src/plugins/clipboard/mode.ts:52-55`）
也确实走 `statusRowBase` 的 muted 注。**这一轮显式偏离参考**，理由是本仓 §4.7 已经把「一页的
陈述」定成 `TextStrong`（P8-b 设置页 / P13-bd 终端页 / P13-be 插件窗口 / P13-bg 捕获输出页），
启动器这一页因为走的是**行**的代码路径而漏在参考旧值上。

## 四、门槛（全部实跑）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 3203
GOOS=linux/windows/darwin go build ./... → 三平台过
```

| 项 | 基线（本轮前） | 本轮 | 差 |
|---|---|---|---|
| PASS | 3198 | **3203** | **+5** |
| FAIL | 0 | **0** | 0 |
| SKIP | 113 | **114** | +1（opt-in dump） |
| 测试函数 | 2349 | **2355** | +6 |

新增 pin 文件 1 个：`internal/launcher/statusnote_test.go`（5 条 pin × 深浅两套 + 1 opt-in dump）。
测试函数计数 2349 → 2355 的 6 条里，5 条是 pin，1 条是 dump。

## 五、变异（4 条实跑，全部真红；`md5sum -c` 逐字节还原后 `touch` 复跑绿）

`internal/launcher/launcher.go` 改动前 md5 = `60e8c63d02122590b127048ef71fe1db`；四条注入
之后 `md5sum -c /tmp/p13bo.md5` 每次都是 **成功**。

**M1 · 句子退回 `TextMuted`**（原报错串）：

```
--- FAIL: TestTheOffPluginsNoteIsOnTheStatementsTooth
    light: note sentence 116 vs ground 253 = 137 levels
    the off-plugin note (light) sits 137 levels off the ground (116 vs 253); a page whose whole
    content is one sentence must state it at the statement rung (≥180) — it used to read the
    same as the field's own placeholder
    dark: note sentence 164 vs ground 23 = 141 levels
--- FAIL: TestTheOffPluginsNoteMatchesTheOtherPagesAnswer
    light: clipboard-off note 116, browser-off note 27 (-89 apart)
    the same statement («the plugin is off») is -89 levels apart depending on which plugin
--- FAIL: TestTheNoteIconIsOneStepUnderItsSentence
    light: sentence 137 levels off the ground, icon column 145 (-8 apart)
```

**M2 · 图标退回 `TextTertiary`**：

```
--- FAIL: TestTheNoteIconIsOneStepUnderItsSentence
    light: sentence 226 levels off the ground, icon column 109 (117 apart)
    the note's icon column (light) sits 117 levels under its own sentence (109 vs 226 off the
    ground); §4.7's footnote puts the mark one step beneath the sentence it introduces — 40..110
    dark: sentence 214 levels off the ground, icon column 89 (125 apart)
```

**M3 · 句子冒充结果行标题（`FontSize+2` / 580）**：

```
--- FAIL: TestTheNoteIsNotAResultRowsRegister
    the status arm no longer contains "FontSize(t.FontSize).FontWeight(480)"; the off-plugin
    note is a page's statement on the list's own 30u band, not a result row
    the status arm now wears a result row's title register (+2 / 580); a note is not a result
```

**M4 · 行上长出一块底板（`Background(tokens.Control)`）**：

```
--- FAIL: TestTheNoteIsNotAResultRowsRegister
    the status arm grew a plate or a lit face; a status note is information, not a door
    (it has no pointer state and no key)
```

## 六、出图（自己用眼睛复核）

`FLOTER_P13BO_DUMP=1 FLOTER_P13BO_TAG=<before|after>`：

- `/tmp/floter-P13-bo-clipboardoff-light-before.png` / `-after.png`
- `/tmp/floter-P13-bo-clipboardoff-dark-before.png` / `-after.png`

**这张图上看到的一处变化**（`light-before.png` 对 `light-after.png`）：墨块 `x=54..446
y=84..106` 那一带，`before` 里的句子是一团**灰**（最暗 116，与它上方输入框的占位句同一个灰色），
`after` 里同一行变成**近黑**（最暗 27，与「Recent 00」的标题同一个黑），而左侧 28u 图标列里
那枚 info 环同步**从 tertiary 变成 secondary**（144 → 145，浅色档几乎不动，深色档 112 → 133
明显变浅——那正是它此前在深色档比句子还亮的那 52 级）。行的**高度、左右内边距、图标大小、
位置一个像素没动**（`before` / `after` 两图的墨盒都是 `54,84 392x22`）。

## 七、qiaomu-design skill 的用法

**读了**：`SKILL.md` → `references/user-preferences.md`（全）→ `craft-loop.md`（全）→
`preflight.md`（全）。本轮表面是「启动器的一页只有一句话」，材质/空间看 `craft-loop.md`，
收尾看 `preflight.md`。走的是**「已有方向，只要打磨」**那条路径（`SKILL.md` 的路由表
「已有方向，只要打磨/反 AI 味/加动效」→ `craft-loop.md` + `preflight.md`），
**不走** Phase 2 的四方向预览（用户 2026-10-30 已把方向给死：轻便液态玻璃），
也没出 HTML 预览页。

### 偷了什么（逐条：菜单第几项 → 落成哪个具体数值）

`craft-loop.md` 的「工艺密度 ≥ 5」菜单里，本轮落在启动器的这一页只取三项，其余**故意不取**
（这一页只有一句话，凑密度就是加噪音）：

1. **第 6 项「编辑细节」→ 字阶/字重的角色分家**。菜单原文是「章节编号（01/02）、页边注、
   hairline、大写字距标签」。落到本仓：这一页的答案不再借列表行的寄存器
   （`FontSize` / 480 / `TextMuted`），而是**同一个盒子里换齿**——`FontSize` / 480 /
   `TextStrong`。角色由**墨**分，不由字号分（字号一动就是高度链红线，`P13-bd` 已实证
   把 `tokens.Title` 换进终端页会把盒高 23 推到 19）。
2. **第 12 项「材质细节」→ 图标次文字色**。菜单原文是「内边框高光、glass 折射边、纸张齿孔、
   印刷套准偏移」——即「同一块料上第二个小件也必须被量过」。落到本仓：`TextTertiary` 是
   **旧句子**的下一档；句子换齿之后它就必须跟着换（§4.7 注脚「图标次文字色」，
   `settingsui.emptyState` 的同一个数）。这是本轮唯一一处「连带改动」，也是本轮
   最容易被漏掉的一处。
3. **第 5 项「多层染色阴影」→ 反向取用：这一页**不做材质**。菜单要的是「拒绝单层灰阴影」；
   本轮把同一条纪律反过来用在「不加层」上：这一页的答案**不许**长出底板 / 受光边 /
   投影（贴进 §6.33 33-4，并钉成结构 pin `TestTheNoteIsNotAResultRowsRegister`）。
   `craft-loop.md` 的「最后一轮做减法」正是这一条。

另外取 `craft-loop.md` 的**两轮制**：第一轮（改墨）→ 渲染对图（`before` / `after` 四张 PNG）
→ 四视角评审 → 第二轮只修一处。

### 四视角偏执评审（各挑一处，共 4 条）

- **艺术总监**：这一页在改前**最平庸**的一点就是「唯一的内容是全页最淡的东西」——它不是
  排版问题，是**层级定型**问题：任何一页的答案都必须比它的提示重。改后这一页有了唯一的
  焦点，而它仍然是最轻的一行（无底板、无编号、不在数字族里）。
- **工程师**：`TextSecondary` 看上去像「中间那一档」，实测在**深色档反而比 `TextMuted` 更暗**
  （133 对 141）——如果按直觉选它，浅色档会得到「答案只比占位句重 8 级」，深色档会得到
  「答案比占位句还轻」。**这是一处会骗人的直觉**，已写进合约 §6.33 的对照表。
- **甲方**：三秒看完，记住了什么？改前：一张空卡。改后：「这个插件是关着的，去哪儿开」——
  这一页的**功能契约**（告诉用户发生了什么 + 下一步在哪）在视觉上第一次成立。
  i18n 文案本身没动（句子里本来就有「可在"设置 > 集成"中启用」）。
- **读者**：眯眼通读，改前那一句与上方输入框的占位句连成一片灰；改后两者分成两件事
  （答案 / 提示），中间隔着 81 级。

### preflight 逐条（A / B / C / D2）

- **A 反 AI 套路**：**过**。无渐变、无纯黑、无外发光、无大标题渐变字；本轮没有新字体；
  全页无 `font-style: italic`（P13-bh 已把全仓最后两处斜体清掉，本轮没引入）；
  无 Hero、无三等分卡片、无眉批、无假数据、无空词；感叹号 0。**未过项：无**。
- **A2 进攻三步**：**过**。DNA 注入见上（`craft-loop.md` 第 6 / 12 / 5 项）；
  工艺密度 3 项（这一页只有一句话，按 `craft-loop.md` 的「密度不等于吵闹」**故意不凑满 5**，
  理由写在上面的「偷了什么」第 3 条）；两轮制已走（对图 + 四视角 + 只修一处）。
- **A4 功能契约**：**过**。这一页的功能契约是「告诉用户这个插件关着，以及去哪儿开」——
  答案成了页面上最重的一件；i18n 文案一字未改，出口（设置 > 集成）仍由那句话自带。
- **A4.1 Carbon 决策门**：**过**。empty / unavailable 两类此前已区分（`ClipboardDisabled`
  对 `ClipboardEmpty`）；本轮不动语义，只让 unavailable 这一支在视觉上读成结论。
  disabled / read-only 未混淆（这一行本就没有指针态，`rowSelectable` 排除）。
- **B 设计读取一致性**：**过**。本轮只有一个主题（当前外观），没有混主题；
  一句话的记忆点：「插件关着」这件事第一次读起来像结论。
  动效气质一致：本轮**不动效**（`docs/liquid-glass.md` §5 一个字未改）。
- **C 动效抽查**：**过（不适用）**。本轮零动效改动。这一行没有 hover / press 臂
  （`iconPlate` 那条路不经过它），所以 §5 的 120ms 颜色过渡、`scale(0.986)` 按下、
  `prefers-reduced-motion` 兜底这一组条目在本轮**没有可抽查的对象**——不是跳过，
  是这一轮的产品改动里没有动效。诚实记账：**本轮无法用动效证据举证**。
- **D2 中文页**：**部分过，一处需拍板（沿用）**。
  - 中文正文/UI 用系统字体栈（本仓 `FontStack`，本轮未动）：**过**。
  - 中西文之间 / 中文与数字之间有空格：本轮未改文案（i18n 一字未动）：**过（复核）**。
  - 全角标点：中文文案 `"剪贴板历史已关闭。可在“设置 > 集成”中启用。"` 用的是全角句号与
    全角引号，但 `>` 是半角——那是本仓全部中文文案统一的写法（原文如此，**本轮不动**）。
    **过（复核）**。
  - **正文 ≥ 14px：未过，且是已挂在「需拍板」的既有张力。**
    这一页的句子用的是 `t.FontSize`。实测（`theme.For(s, false)`，本轮真量）：

    | `UIScale` | `FontSize` | `Caption` | `Body` | `Emphasis` |
    |---|---|---|---|---|
    | `tiny` | 9.60 | 8.80 | 9.60 | 10.40 |
    | **`small`（默认）** | **10.80** | 9.90 | 10.80 | 11.70 |
    | `default` | 12.00 | 11.00 | 12.00 | 13.00 |
    | `large` | 13.20 | 12.10 | 13.20 | 14.30 |

    本仓的默认档 `small` 下正文是 **10.80px**，所以 D2 的「正文 ≥ 14px」在这一页上
    本来就不过——这是**全仓**的既有张力（本仓的 `--text-body: 11px × ui-scale` 是参考
    自己的梯子，不是 web 的 14px 基线），**不是本轮引入的**，也不是本轮能改的
    （改字阶 = 改高度链，行为契约红线；`P13-bd` 已实证）。**沿用挂账，写进「仍待 2」。**
  - **中文行高 ≥ 1.5：过。** 行盒由框架给：实测墨盒高 **22**，句子字号 10.80
    ⇒ 行高 ≈ **2.04** ≥ 1.5。
  - 诚实记账：本轮**没有**新的 D2 未过项；上面这一条是**既有账**的复核，不是本轮新增。

## 八、仍待（P13-bo）

1. **同族普查，未做**：`item.status` 只有**一个**生产点（`search.go` 的 `clipboardItems`），
   所以启动器侧这一族本轮结案。但 `internal/shell/detach.go` 的 `detachedListNote`
   （§6.24 24-5，**故意不抬**，理由是「它说的是**列表**，不是这一页的结论」）与本轮的
   `status` 行是**同一个形状、两个判据**。两者的分界目前靠理由而不是靠一条可断言的规则：
   「这一页只有它」→ 陈述齿；「它是列表的从属注」→ 静息齿。**下轮可把这条分界写成
   一个共享判据**（例如「列表非空时它是注，列表为空时它是结论」），但那会同时改
   `detach.go` 与 `launcher.go` 两条表面，属跨表面，按「一个表面一轮」的纪律**另开一轮**。
2. **需拍板（沿用）**：`tokens.Caption` 9.90px / `tokens.Body` 10.80px 与 preflight D2；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾（P13-bm 仍待 1）；终端弧 S1b / S1c / S2 尾。
3. **下一格候选**：① 上面第 1 条的分界规则；② `.pending-bugs.md` P13-bn「仍待 3」的
   「集成卡里同一件事说两遍」；③ 通用队列「间距节奏」：先量 `settingsui` 卡片里
   8/9/11/13u 的混用（不许先改）。
4. **抢救 stash 复核**：`stash@{0}`=`afc38b247c`（`metaSeparator`，HEAD 里出现 0 次，
   已被 P13-af 的 `metaFact` 取代）；`stash@{1}`=`114b7d1585`（只改 `.pending-bugs.md`，
   文字已在文件里）。**两条无需再试合**（与 P13-bm / P13-bn 的结论一致）。
