# P13-bj 报告 · 分页脚注的两个状态此前一档墨（Δ = 0）

轮号 **P13-bj**｜分支 `mygo-rewrite`｜规格 `docs/rounds/P13-bj-spec.md`｜
合约新增 **§6.29**（6 行：5 `已落地` + 1 `故意不做`）

## 一句话

捕获输出页列表底部那一行分页脚注有两个会画字的状态——**「正在加载…」** 与
**「没有更多结果」**——HEAD 上它们**逐像素同墨**（对地面峰值差 **118 / 118**，浅深两套外观
一个字节不差）：一件「正在发生的事」和一件「到此为止的边界」用同一个声音说话。本轮把它拆成
两档：在飞（环 + 词，一个物体）抬到活跃档并戴 500 字重，到底了留在静息档**一个字节不动**。

## 这轮为什么让它更轻、更有空间感

空间感 = **差**。这一行此前只有一个高度：**不管它在做什么，都是同一档墨**。换状态时屏幕上
唯一的变化是字变了——环消失、词换了，而**这一行的 z 没有任何变化**。于是这一行读起来
不像「一个正在进行的动作」也不像「一个边界」，只像一句附在列表下面的灰字。

改后这一行第一次有两个高度（对地面，浅 / 深）：在飞 **139 / 139**（配一枚同墨的环）
→ 到底了 **118 / 118**（列表的地板）。同时它仍然**明确低于内容**（行标题 216 / 219）——
抬的是「这一行内部的两件事」，不是「这一行本身」。

## 落地（`internal/launcher/output.go`）

| # | 位置 | 改前 | 改后 |
|---|---|---|---|
| 1 | `outputFooter`（`:291`） | 一个 `text` 变量 + 一句写死的 `.TextColor(t.TextMuted)`（`:310`） | 拆出 `footerWord(state, copy, tokens, t) (string, ui.Color, int)`：墨与字重的**唯一出处** |
| 2 | 在飞 | `t.TextMuted` + 默认字重 | `tokens.TextSecondary`（浅 `#6B6B70` / 深 `#9A9AA0`）+ **500** |
| 3 | 到底了 | `t.TextMuted` | **不动**（改前改后整帧 md5 相同，见「出图」） |
| 4 | `footerSpinner`（`:372`）活动象限 | `t.TextMuted` | `tokens.TextSecondary`（与词同墨：环 + 词是一个物体）；轨道仍是 `tokens.Hairline` |
| 5 | 字阶 | `t.FontSize - 1` | **不动**（改字阶 = 改行盒 = 改高度链） |

**为什么环必须一起抬**：环的**活动象限**和词此前同墨，这是对的——词是环的标签。只抬词会让
一行里出现两个互不相干的声音（实测 M1 变异：环 139 / 词 118，pin 直接报「the pair is not
one ink」）；只抬环会让标签比它标注的东西还轻。

**为什么到底了不许动**：它本来就在静息档，这一轮要修的是「两件事一档墨」，不是「两件事都
不够重」。把地板也抬起来，列表最后一行就会和它的边界打架（M2 变异：到底词 138，与在飞
只差 1 级）。

## 门槛（全部实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` → **净**；`GOOS=windows go vet ./...` → **净**；`GOOS=darwin go vet ./...` → **净**
- `go test -count=1 ./...` → **0 FAIL**
- PASS **3165 → 3168**（**+3**）｜FAIL **0**｜SKIP **110 → 111**（+1，opt-in 出图）｜
  测试函数 **2333 → 2337**（+4）
- `GOOS=linux/windows/darwin go build ./...` → **三平台全过**
- **V 0 / B 0 / S 0**：没有任何旧 pin 变红（`output_pagination_test.go` /
  `spinnerframes_test.go` 的 `HasText` / `Find` / 环在场断言全绿）；本轮只加 pin。

## 变异（4 条实跑，全红，逐字节还原后 `touch` 复跑绿）

| # | 注入 | 原始报错串 |
|---|---|---|
| M1 | 在飞的墨退回 `t.TextMuted` | `dark=false: the live word peaks at 118 over the plane, want ≥ 130 — it is not in the active register` / `the live word and the end word are one ink again (gap 0, want ≥ 12)` / `the ring peaks at 139, its word at 118 — the pair is not one ink` / `the live word is not the heavier one (mass ratio 1.13, want ≥ 1.25)`（三个 pin 全红） |
| M2 | 到底了也抬到 `tokens.TextSecondary` | `dark=false: the end word peaks at 138 over the plane, want ≤ 122 — the list's floor left its rest rung` / `the live word and the end word are one ink again (gap 1, want ≥ 12)` |
| M3 | 环的活动象限留在 `t.TextMuted` | `dark=false: the ring peaks at 118, its word at 139 — the pair is not one ink` |
| M4 | `footerLiveWeight` 500 → 400 | `dark=false: the live word is not the heavier one (mass ratio 1.13, want ≥ 1.25)` |

`md5sum -c` 每条后逐字节核对通过（`7b84b38291cf0101273110313c941214`），最后 `touch`
产品与测试文件再复跑一次绿。

## 出图（自己用眼睛复核过）

`/tmp/floter-P13-bj-footer-{light,dark}-{loading,end}-{before,after}.png`
（`FLOTER_P13BJ_DUMP=1 FLOTER_P13BJ_TAG=<before|after>`）

- **`end` 的四张前后 md5 逐字节相同**（`c88f56d4…` 深 / `d490301a…` 浅）——到底了那一帧
  这一轮**没有动过一个像素**，这是设计意图，也是这条改动的边界证据。
- **`loading` 的前后像素差 bbox = `(313,472)-(405,484)`**（浅 479 px / 深 480 px）——正好是
  环 + 词那一行，页面上没有第二个像素动过。
- **这张图上看到的一处变化**（浅色 `loading` 前后对着看）：环的转动那一象限从几乎和轨道
  分不开的浅灰，变成能一眼指出「哪四分之一在转」的深灰；旁边那句 `Loading more…` 也跟着
  沉了一档，和环读成**一个**正在转的东西，而不是「一个环 + 一句旁白」。

## 四视角评审（craft-loop 两轮制，各挑一处最弱）

| 视角 | 最弱的一处 | 处置 |
|---|---|---|
| 艺术总监 | 这一行仍然只是「一行居中的灰字」——它没有成为这个页面的记忆点 | **接受**：分页脚注不该是记忆点；它的职责是**不打断**列表，这一轮把它从「含糊」推到「清楚」就够 |
| 工程师 | `footerWord` 里 `footerEnd` 拿的是 `t.TextMuted`（框架槽），`footerLoading` 拿的是 `tokens.TextSecondary`（app 调色板）——两个来源 | **已核对**：生产路径 `shell.View` 先 `c.SetTheme(surface.Tokens.Theme)`，`t.TextMuted` 就是 `tokens` 的 muted，两个来源在屏幕上同值；pin 也按生产路径装主题后再量 |
| 甲方 | 三秒看完，「这页还在加载」和「这页到底了」现在分得开了吗？ | **分得开了**：差 21 级 + 一个字重档；且到底了仍然比内容轻（118 vs 216） |
| 读者 | 13px 的灰字在浅色地面上读起来费劲吗？ | **没有变差**：到底了原样不动（118），在飞反而更清楚（139）；`TextSecondary` 在浅色地面上是 4.5:1 以上（§1 的正文墨阶） |

**第二轮只改了一处**：量到「去掉字重后墨质量比 1.13、带字重 1.34」之后，把字重从「顺便加的」
确认为**必须项**——峰值只反映最暗那一点，与字重无关，所以字重只能靠**墨质量**（每单位面积
的墨）来 pin。这条决定了 `TestThePagingFooterLiveWordIsTheHeavierOne` 的度量方式。

## 偷了什么（craft-loop 工艺密度菜单，逐条对号）

| 菜单项 | 落成的具体数值 |
|---|---|
| **第 6 项 · 编辑细节**（章节编号 / 页边注 / hairline / **大写字距标签**） | 这一行是**标签**不是**句子**：字重 **500**（`footerLiveWeight`），墨 `TextSecondary`，与它标注的环同档——「一个标签 + 它标注的东西」是同一个物体 |
| **第 12 项 · 材质细节**（内边框高光 / glass 折射边） | 自绘没有折射边，等价物是**墨阶**：在飞 **139** / 到底了 **118** / 行标题 **216**（对地面峰值差），三级，差值 21 与 77 都能在 1x 屏一眼分开 |

**没有取 1（氛围层）/ 2（::selection）/ 3（focus 环）/ 4（签名交互）/ 5（多层染色阴影）/
7（展示排版）/ 8（流体字号）/ 9（滚动条）/ 10（彩蛋）/ 11（图形资产）**——理由：本轮的表面是
**一个已有的页脚行**，不是新页面。craft-loop 的「每页 ≥ 5 项」是**按页**的密度门槛，
这一页的那五项在 §6.1（启动器）/ §6.16~§6.20（面板与浮面）/ §6.26（状态行）里已经落过；
给一个 20px 高的行硬塞氛围层或彩蛋，是密度表演，不是密度。

**DNA 供体**：本轮**没有**从 58 站库取供体，理由：方向已被用户钉死（轻便液态玻璃），
而仓库已有**单一来源合约**（`docs/liquid-glass.md`）。本轮偷的是同一份合约里 §6.26 / §6.28
**已经量过的阶梯数值**（结局 139 / 事实 89 / 图例 109），把它复用到分页状态上——
同一个应用里「同一档墨」必须指同一个亮度，这比引入第二个供体更值钱。

## preflight 逐条结论

**A · 反套路扫描**：无渐变、无纯黑、无外发光（自绘，墨是实色 token）；**无斜体**
（`internal/` 生产代码 `.Italic()` 调用点为 0，P13-bh 收的）；无三等分卡片、无眉批、
无假数据、无空词。**过**。

**B · 设计读取一致性**：这一页是「捕获输出页的列表」，面向「刚跑完一个插件命令、
正在翻它的结果」的用户，用「安静、可扫读」的语言。记忆点：**列表的边界不吵**。
动效气质一致（环仍是 0.7s 一转、`ReduceMotion` 静止）。**过**。

**C · 动效抽查**：本轮**没有新增动效**——只换了环的颜色。既有环的时长 700ms（不在
「UI 动效 ≤ 300ms」的范围内，但它是**循环进度指示**，不是转场；`motion-craft.md` 对
indeterminate spinner 的判据是「不能停、不能抖」，0.7s 一转是参考的原始值）；只画
stroke，不动 width/height；`ReduceMotion` 兜底在（`footerSpinner` 的 `if`）。**过（无新增）**。

**D2 · 中文页追加项**：文案是 i18n 的（`正在加载…` / `没有更多结果`），字号 13px
（`t.FontSize - 1`）、字重 400/500、全角省略号，中文与西文不混排。**⚠️ 一条不过**：
「正文 ≥ 14px」——这一行是 **13px**。**不能过，且本轮不许改**：改字阶 = 改行盒 = 改高度链
（§6.24 / §6.25 / §6.26 / §6.28 已五次撞红），这一条与 `tokens.Caption` = 11px 的张力
是**同一个需拍板项**，本轮是它的**第六份证据**，已写进 `.pending-bugs.md`「需用户拍板」。

## 仍待（P13-bj）

1. **需拍板（沿用，本轮第六份证据）**：13px / `tokens.Caption` 11px 与 preflight D2
   「正文 ≥ 14px」的张力。**改字阶 = 改高度链**。
2. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾。
3. **下一格候选**：①`settingsui.go:5714` 的 `metadataCell` 标签（`Caption` + `TextTertiary`）
   与 `:5683` / `:6847` 的同类从属标签——先量它们与主体的关系（P13-bf 的教训：从属行
   不该一律抬）；②`shell/pinned.go` 便条窗口的正文只有一档字阶（`tokens.Body` 1.45 行高，
   它是一段快照，**可能本来就对，先量**）；③启动器空态（无查询、无最近）的三段字阶——
   P13-bd 只修了终端页空态，启动器的空态还没量过。
4. **抢救 stash（本轮复核）**：`stash@{0}` = `afc38b247c`（`metaSeparator`）已在 HEAD；
   `stash@{1}` = `114b7d1585` 只改本文件、文字已在文件里。**两条都无需再试合。**
