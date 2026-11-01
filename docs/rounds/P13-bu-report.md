# P13-bu · 报告：列表里的状态行——「注」还是「这一页的结论」，由旁边有没有行决定

> 总目标（2026-10-30）：细节打磨 + 轻便液态玻璃 + 空间感。
> 选题 = P13-bo「仍待」第 1 条亲手写下的分界规则，落成**一个谓词**（`docs/liquid-glass.md` §6.39）。
> 规格 `docs/rounds/P13-bu-spec.md`。

## 一、这轮为什么让它更轻、更有空间感

插件窗口里**一条状态行是列表唯一的内容**时（插件回了「没有匹配」这类状态行），这一页的
**唯一一句话**落在窗口上最轻的一档——比它自己的 bar 标题还轻 3 级。于是这个窗口读起来是
「一条 bar + 一条比 bar 还淡的灰线」，页面没有结论，只有装饰。把这句话按「旁边有没有行」
升到陈述档之后，同一个窗口第一次有了两档：**这一页说了什么**（陈述）与**它在哪儿**
（chrome）。层级由墨分家，而不是靠一条线分家——这正是「空间感」缺的那一半。

## 二、先量（改前，720×460，深/浅两套，`bodyWeight` = 地面与墨的 luma 差）

| 臂 | 墨 | weight（对窗口地面） | 盒高 |
|---|---|---|---|
| 列表里**只有**一条状态行 | `Caption`+`TextMuted` 浅 `{131,131,138}` / 深 `{130,130,138}` | **115 / 110** | 15.0 |
| 状态行旁边有一条行盘 | 同上（**这一档是对的**） | 115 / 110 | 15.0 |
| 这一页的陈述（`empty` 臂） | `Emphasis`+`TextStrong` 浅 `{27,27,29}` / 深 `{237,237,239}` | **219 / 217** | 18.0 |
| bar 标题 | `Caption`+`TextMuted` | 118 / 113 | 15.0 |

第一行与第四行**差 −3**：这一页唯一的那句话比它的 chrome 还轻。

## 三、落地（3 处，1 个谓词）

1. **`internal/shell/detach.go`** 新增 `detachedStatusAlone(rows []extensions.Row) bool`：
   整张列表里没有任何非状态行时为真。判据写在**列表**上而不是行上——同一个列表里两条状态行
   不许落在两档。
2. **`detachedList`**：`alone` 为真时状态行走 `detachedStateLine(c, row.Title, detachedStatement, …)`
   （§4.7 的陈述档，与 `settingsui.emptyState`、`empty` 臂**同一对函数值**），否则走
   `detachedListNote`（参考值 `Caption`+`TextMuted`，`launcher.css:1783-1789`）。
3. **`internal/launcher/launcher.go`** 的 `item.status` 臂：**代码零改动**，补一行注释把 §6.39
   引出来——启动器的状态行只由 `search.go` 的 `clipboardItems` disabled 臂发出、且只发一条
   （`return []Item{{ID: ClipboardDisabled, …}}`），按判据恒为陈述档。两处读**同一条规则**。

## 四、改后（同一把尺）

| 臂 | 墨 | weight | 盒高 |
|---|---|---|---|
| 列表里只有一条状态行 | `{27,27,29}` / `{237,237,239}` | **219 / 217**（旧 115 / 110） | **18.0**（旧 15.0） |
| 状态行旁边有一条行盘 | 不变 | 115 / 110 | 15.0 |

- alone 臂与 `empty` 臂的陈述行**逐字节同值、同盒**（219/217、18.0）。
- note 臂**逐字节不变**：`/tmp/floter-P13-bu-window-note-{dark,light}.png` 与 before 的 md5
  完全相同（`235176a9…` / `7fc39d00…`），像素 diff **0**。
- alone 臂像素 diff 只落在那一行：差异盒 `x[12..85] y[55..66]`，深/浅各 **465** 点。
- **这张图上看到的一处变化**：alone 帧里 `3 of 4 shown` 这一句从「比标题还淡的灰」变成与
  空输出帧同重的黑（深色档同重量的浅），字面也从 15 撑到 18——窗口上唯一的那句话第一次是
  这个窗口里最响的东西；note 帧一个像素没动，同一句话在行盘旁边仍然是灰的。

## 五、V / B / S 台账

- **V = 1**：`internal/shell/stateladder_test.go` 的 `bodyArms` 里 `liststatus` 臂**重定基线**
  （注释写 `P13-bu 重定基线`）：旧臂是「列表里只有一条状态行」——那正是本轮判为陈述的形状，
  拿它当「注」的反例会让两条 pin 互相矛盾。新臂给这句注一条行盘当邻居。断言本身一个字符没改。
- **B = 0**：`go test ./internal/shell/` 除上述 V 之外没有旧 pin 变红。
- **S = 0**：没有结构重写（新增一个谓词 + 一个分支，`detachedStateLine`/`detachedListNote`
  两个函数都还在原位，`TestTheDetachedStateLadderIsThreeRungs` 照旧全绿）。

## 六、门槛（实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（exit 0）
--- PASS 计数                → 3213 → 3215（+2）
--- SKIP 计数                → 118 → 119（+1 = opt-in dump）
--- FAIL 计数                → 0
测试函数数（git grep 求和）   → 2371 → 2374（+3）
三平台 build                 → linux/windows/darwin 全过
```

## 七、变异（3 条，全部实跑变红并 `md5sum -c` 逐字节还原）

`internal/shell/detach.go` 改前 md5 = `7b119d98dec88358320993cec1c6f576`，三条变异后都
`git checkout --` 还原并 `md5sum -c` 通过（`成功`），随后 `touch` 复跑 `go test ./internal/shell/`
全绿（`ok 25.671s`，避免缓存）。

| # | 注入 | 原始报错串（抄自实跑） |
|---|---|---|
| M1 | `alone := false && detachedStatusAlone(rows)` | `dark=false: the lone status row does not outrank the bar title (by -3, want ≥ 60; line {131 131 138 255} title {128 128 135 255})`；`…is not the statement's size (15.0 vs 18.0)` |
| M2 | `alone := true` | `stateladder_test.go:236: dark=false: the list's note rose to the statement register (gap 0, want ≥ 60; note {27 27 29 255}, statement {27 27 29 255})`；`…left the chrome's register (18.0 vs title 15.0)` |
| M3 | `detachedStatusAlone` 只看第一行（`return rows[0].Status`） | `statusalone_test.go:130: dark=false statusFirst=true: the list's note rose to the statement register (gap 0, want ≥ 60 …)`——注在前的顺序红、注在后的顺序仍绿，正是「读整张列表」与「只看第一行」的分界 |

## 八、用了 `qiaomu-design`（打磨模式，未走 Phase 2）

**读了**：`SKILL.md`（Router → 「已有方向，只要打磨」那一路）→ `references/user-preferences.md`
（P-1 禁斜体 / P-5 可读性优先 / P-7 创意不许吃功能契约，逐条对照）→ 本轮表面需要的
`references/craft-loop.md`（材质·空间·工艺密度）→ `references/preflight.md`（A/B/C/D2）。
**没有**出四方向预览页、没有等用户点选——用户已经把方向给死（轻便液态玻璃），自动车道不许停。

**偷了什么（逐条对 `craft-loop.md` 的工艺密度菜单，写具体数值）**

1. **第 3 项「品牌 focus 环 → 换成本仓的『陈述环』」的反面用法**：菜单要的是「强调只用一档」，
   落成**两个函数值**——陈述 = `tokens.Emphasis` + `TextStrong` + 580（实测 219/217），
   注 = `tokens.Caption` + `TextMuted`（实测 115/110）。两档的 weight 差 **104 / 107 级**，
   而旧的两处只差 **0 级**（同一对函数值）。
2. **第 6 项「编辑细节：hairline、大写字距标签、页边注」里的「页边注」**：落成一条**判据**
   而不是一个装饰——「一句话是不是注，看它旁边有没有别的东西」。这不是新增元素，是把已有的
   一句话放对档位；像素上只动 465 点（差异盒 74×12），盒模型零改动。
3. **两轮制（Render → Critique → Refine）的工程师视角**（「哪里对不齐/状态缺失」）：本轮的
   critique 就是它——同一个形状在两个窗口上两个判据，且两条判据都是**无条件**的。
   refine 只做一件：把判据搬到列表层，两处读同一条规则。
4. **第 12 项「材质细节：内边框高光」的**反面**——「层级不靠加线」**：本轮**没有**新增任何
   受光边/阴影/边框；窗口的地面、bar 的 hairline、行盘的 E1 一对线一个像素没动
   （note 臂 md5 相同即证据）。

**preflight 逐条**

- **A（反 AI 套路）**：过。没有新增渐变/白边/模糊/发光，没有新增图标或装饰，没有 `font-style`
  斜体（本轮零排版改动），文案零新增（沿用既有 i18n key）。
- **A2（进攻三步）**：DNA 注入 = 参考实现自己的 `.plugin-window__status`（`launcher.css:1783-1789`）
  与 §4.7 的陈述档，**不是**新引入的外部风格；工艺密度 = 上面 4 条；两轮制 = 先量（改前表）→
  工程师视角 critique → 只修一处。
- **B（设计读取一致性）**：过。只用既有 token（`tokens.Emphasis` / `TextStrong` / `Caption` /
  `TextMuted`），零硬编码色值/尺寸；深浅两套读数逐位对应（219/217、115/110），说明没有引入
  明度相关分支。记忆点一句话：**「这个窗口里最响的是它说了什么，不是它的框。」**
- **C（动效抽查）**：**不适用（本轮零动效）**——`detachedList` 没有 transition/animation，
  状态行不在任何 hover/press 臂里（`rowSelectable` 之外），copy-on-select 与翻页触发的
  时序一个字节没动。
- **D2（中文页）**：**过一半、一条老账**。本轮不改字号档位与文案（中文页同一句话走同一对
  函数值），所以 D2 的「正文 ≥ 14px」既不因本轮变好也不变坏：`tokens.Caption` 9.90px /
  `tokens.Body` 10.80px 与 D2 的张力仍是**需拍板**项（第 9 份证据：本轮把「注 = caption 档」
  又钉了两条 pin，动字号要同时重定它们）。中西文空格/全角标点本轮零改动，不新增风险。

## 九、仍待（P13-bu）

1. **需拍板（第 9 份证据）**：`tokens.Caption` 9.90px / `tokens.Body` 10.80px 与 preflight D2
   「正文 ≥ 14px」。本轮新增的两条 pin 把 caption 档钉得更死，动字号必须先拍板。
2. **下一格候选**：① `launcher/output.go` 捕获输出页的 `row.Status`（本轮 §6.39 的 39-4 判
   `故意不做`）——它保留 28u 图标板，是**行**不是**一句话**，若要统一得先定「无盘的一句话」
   这个形状的边界；② 通用队列「间距节奏」：先量 `settingsui` 卡片里 8/9/11/13u 的混用
   （P13-bs 记的账，**不许先改**）；③ P13-bn「仍待 1」的两个层级标题（需拍板，先别动）。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **P13-bo「仍待 1」本轮结案**：那条分界规则已写成 `detachedStatusAlone` + §6.39，两处读同一条。
5. **抢救 stash 复核**：`stash@{0}` = `33e03d1f6a`，`git stash show --stat` **空**（相对其 parent
   零改动），tree 与 `6bee2c19`（P13-bp）逐字节相同；`stash@{1}` = `afc38b247c` 的 `metaSeparator`
   补丁与 HEAD 的 `metaFact` 版本重叠（`integrationMeta` 已落地）。两条都**无需再接续**，
   也**没有**静默 drop。

## 十、出图

```
FLOTER_P13BU_DUMP=1 go test ./internal/shell/ -run TestDumpP13BU
/tmp/floter-P13-bu-window-alone-{light,dark}-before.png   ← 改前（`…-before` 是运行后 cp 的）
/tmp/floter-P13-bu-window-alone-{light,dark}.png          ← 改后
/tmp/floter-P13-bu-window-note-{light,dark}-before.png / …png  ← 逐字节相同（对照组）
```
