# P13-fg 报告 · 集成页的新鲜度格子：一格在说答案，一格在说「还没有答案」

**读取为**：桌面设置面板的集成页抽屉，面向正在装/排查工具的人，用「近黑/近白底 + 三级面 +
发丝线 + 紧凑行」的轻便液态玻璃语言，倾向 Raycast 系的设计家族。三拨盘本轮不动：`VARIANCE` 3、
`MOTION` 2、`DENSITY` 7。
**功能契约**：`Freshness` 块读出「上次探测在什么时候 / 结果如何 / 命令数 / 来源」四条事实。
本轮一个动作、一个字、一个盒都没变，只改「这两格的值取自哪一档墨」。

## 一句话

这块格子的四条事实里，有两条**在墨上不存在**：`metadataCell` 把值写死成 `TextSecondary`，
于是 `Failed` 与 `Succeeded` 同重、`Never probed` 与 `3 min. ago` 同重。本轮把五个结果词与
两种时间值各自挂回它自己的档——**占位（静）< 答案（陈述）< 要读的答案（顶档）**。

## 选题与证据

`docs/rounds/P13-ff-report.md`「仍待」第 1 条 = `docs/liquid-glass.md` §6.127 的 `127-5`：
「那是一族状态词共用一个墨档的时间线，只抬 running 一臂会让同一行断档，**要改得整族一起改**」。
本轮就是整族一起改。禁用 / 只读面那一半已在 P13-eu / P13-s / P13-t / P13-aq 收口，不重领。

**先量（改前，720×900 生产取景，`boxFace` → `boxInk` → `nearestStep`）**：

| 帧 | 结果 | 实测墨 | 最近档 | 距 strong | 距 secondary | 距 muted |
|---|---|---|---|---|---|---|
| 浅 | success | `{110,110,115}` | secondary (3 off) | 86 | 3 | 20 |
| 浅 | degraded | `{107,107,112}` | secondary (0 off) | 83 | 0 | 23 |
| 浅 | failed | `{109,109,114}` | secondary (2 off) | 85 | 2 | 21 |
| 浅 | running | `{107,107,112}` | secondary (0 off) | 83 | 0 | 23 |
| 浅 | unknown | `{107,107,112}` | secondary (0 off) | 83 | 0 | 23 |
| 深 | success | `{151,151,157}` | secondary (3 off) | 86 | 3 | 18 |
| 深 | degraded | `{154,154,160}` | secondary (0 off) | 83 | 0 | 21 |
| 深 | failed | `{152,152,158}` | secondary (2 off) | 85 | 2 | 19 |
| 深 | running / unknown | `{154,154,160}` | secondary (0 off) | 83 | 0 | 21 |

**五个状态的格盒逐字节相同** `{451.3, 387.7, 243.5, 17}`（`Grow(1)` 的格 + `SingleLine()` 的值），
所以这是纯墨的问题，不是几何的问题——阶梯本身很宽（`secondary → strong` 83-86 级、
`secondary → muted` 18-23 级），只要词挂回自己的档，一眼就分得开。

代码：`internal/settingsui/settingsui.go` 的 `freshnessSection`（两格调用 `metadataCell`）与
`metadataCell`（值墨写死在函数里）。

## 逐条落地

| # | 位置 | 现状 → 应改成 | 状态 |
|---|---|---|---|
| 128-1 | `freshnessSection` 的 `Result` 格 | 写死 `TextSecondary` → `freshnessResultInk`：`degraded` / `failed` → `TextStrong` | `已落地` |
| 128-2 | 同上，占位臂 | `running` / `unknown` → `t.TextMuted` | `已落地` |
| 128-3 | `Last probe` 格 | `freshnessWhenInk`：`AtSeconds == 0`（`Never probed`）→ `t.TextMuted`，有值 → `TextSecondary` | `已落地` |
| 128-4 | `metadataCell` | 值墨写死 → 加 `ink ui.Color` 参数（只有这两个调用点）；标签仍固定 `TextTertiary` | `已落地` |
| 128-5 | `failed` 花不花 `Warning` | **不花**（`故意不做`，理由见下） | `故意不做（附理由）` |
| 128-6 | 字号 / 盒 / 字重 / gap / 纵向节奏 / 文案 / i18n / 命令接线 | 一字节不动 | `已落地（不变）` |

**128-5 的理由**：同一张卡上语义色已经被花掉两处——`stateMark` 的 broken 胶囊（集成自己坏了）
与 `healthDetail` 的失败卡 / `statusInk` 的 `error`。§6.108（P13-em）立的规矩是**一个事实一处
告警**；新鲜度失败在这一块里没有第二个载体，但它也不是「集成坏了」，它是**这一格的答案要读**，
所以用中性阶梯的顶档 `TextStrong`，不花 `Warning`。

**本轮写进合约的第二条判据**：「在飞」的档位取决于它是「唯一的说话人」还是「答案位上的占位」。
§6.127 那句在飞词替掉了两个熄灯按钮，是整行唯一信号源 → 必须**响**；这里的 `Probing…` 是答案位
上的占位，与 `Unknown` / `Never probed` 同类 → 必须**静**。两条规矩不冲突，它们回答的不是同一个问题。

## 台账

**V 0 / B 0 / S 0** —— 一条既有 pin 都没改。`TestTheFreshnessFactsAreTheirOwnNodes`、
`TestTheFreshnessNeverProbedWordIsItsOwnNode`、`TestTheFreshnessSectionSharesTheRowsInset`
全绿：本轮不动文案、不动盒、不动块的内缩。

新增 3 条（1 条 opt-in 出图）：`TestTheFreshnessResultWordIsAnAnswerOrAPlaceholder`
（深浅两套 × 五个状态：每个词必须落在自己的档上、五个状态的格盒逐字节相同、三档之间真的隔开
——`secondary ↔ muted` ≥ 12 级、`secondary ↔ strong` ≥ 40 级）、
`TestTheNeverProbedCellIsQuieterThanAProbedOne`（`Never probed` 落 `TextMuted`，且它的盒与
「刚探测过」那一格相同）、`TestDumpP13FGFreshnessPNGs`。

## mutation（四条实跑，原始报错照抄）

| # | 注入 | 结果 |
|---|---|---|
| M1 | `freshnessResultInk` 的 `degraded, failed` 臂 → `tokens.TextSecondary` | 红 ×4：`[light] the degraded word is painted {107 107 112 255} — nearest the secondary step (0 levels off), want the strong step {27 27 29 255}` / `[light] the failed word is painted {109 109 114 255} … want the strong step {27 27 29 255}` / `[dark] … {154 154 160 255} … want the strong step {237 237 239 255}` / `[dark] … {152 152 158 255} …` |
| M2 | `running, unknown` 臂 → `tokens.TextSecondary` | 红 ×4：`[light] the running word is painted {107 107 112 255} — nearest the secondary step (0 levels off), want the muted step {128 128 135 255}` / 同 `unknown` / `[dark] … want the muted step {133 133 141 255}` ×2 |
| M3 | `freshnessWhenInk` 恒返回 `tokens.TextSecondary` | 红 ×2：`[light] "Never probed" is painted {107 107 112 255} — nearest the secondary step (0 levels off), want the placeholder step {128 128 135 255}` / `[dark] … {133 133 141 255}` |
| M4 | 等价写法：`t.TextMuted` → `tokens.Theme.TextMuted`（同一个值） | **全绿**（已知覆盖边界：pin 钉的是档，不是取值的路径） |

M4 的第一次尝试是把 `tokens.TextStrong` 写成 `t.TextStrong` / `tokens.Theme.TextStrong`，
**两个都编译不过**（`*ui.Theme` 没有 `TextStrong`，它只挂在 `Theme` 上），所以改用 `TextMuted`
的等价路径——这一点值得记下来：`TextStrong` 只存在于 `theme.Tokens` 一侧。

每条都 `md5sum -c` 逐字节核对还原（`internal/settingsui/settingsui.go`
`6577d45f50a163add89c9dd7cd2679c3`），并 `touch` 后复跑一次避免测试缓存复用。

## 出图（深浅两套）

`before` 用 `git worktree` 在 HEAD `db782314` 渲（把本轮的 dump 测试拷进去，生产代码是 HEAD 的），
`after` 是工作区。裁的是 `Freshness` 块（420×132，失败帧）。

| 帧 | diff | bbox |
|---|---|---|
| `floter-P13-fg-freshness-light.png` | **151 px** | `(275,52)-(304,61)` |
| `floter-P13-fg-freshness-dark.png` | **151 px** | `(275,52)-(304,61)` |

文件：`/tmp/p13fg/{before,after}/floter-P13-fg-freshness-{light,dark}.png`。

**这张图上量到的一处变化**：差分 bbox 是 **29×9**，正好是 `Failed` 那一个词的墨盒——两个标签、
`Last probe` 的值、命令数、注脚、整块的高度一个像素都没动。也就是说这一轮在图上只发生了
「一个词变重」，没有别的东西跟着动。

## 门槛（实测，HEAD 前 `db782314` → 本轮）

```
gofmt -l cmd internal                              空
go vet ./...                                       净
GOOS=windows go vet ./...                          净
GOOS=darwin  go vet ./...                          净
go test -count=1 ./...                             0 FAIL
PASS 计数                                           3402 → 3404
SKIP                                               189 → 190（新增的 opt-in 出图）
测试函数                                            2569 → 2572
三平台 build                                        linux / windows / darwin 全过
```

**顺带修掉一次自伤（S 类，写进 §6.128 的教训）**：本轮的「先量」表首列表头写了「状态」，
于是 `internal/ci` 的差距表检查把 7 行测量数据读成非法状态词
（`a gap-table row carries an unrecognised status "success"; …`、
`the gap table has 463 rows, 463 of them carry a status`）。按 §6.108 已经立过的规矩
（§6 里讲「档」的表，首列不许叫「状态」）改成「结果」，合约恢复全绿。

## 这轮为什么让它更轻、更有空间感

这块格子的四条事实此前挤在**同一个 z 平面**上：读的人必须逐字读完才知道哪条有信息、哪条只是
占位。现在它们分了三层——占位（`TextMuted`，离答案 18-23 级）、答案（`TextSecondary`）、
要读的答案（`TextStrong`，离答案 83-86 级）——扫一眼就知道「哪一格还没有答案」，而块的盒高、
字号、gap 一个像素没动。层级是靠**墨差**表达的，不是靠加线、加色、加框。

## 偷了什么（qiaomu-design · `craft-loop.md` 工艺密度菜单）

- **第 6 项（编辑细节：hairline、大写字距标签这一类「编辑性」细节）** → 落成**格子里每个小角色
  各自有档**：三个具体 token（`theme.Tokens.TextStrong` / `TextSecondary` / `ui.Theme.TextMuted`），
  外加两个**实测阈值**写进 pin（`secondary ↔ muted` ≥ 12 级、`secondary ↔ strong` ≥ 40 级），
  不是「让它明显一点」。
- **第 12 项（材质细节：内边框高光、glass 折射边）** → 落成**分层不靠加东西**：本轮没有新画任何
  一条线、一枚点、一层影，层级完全由同一块面上的墨差承担（83-86 级 vs 18-23 级）。这是合约 §2
  判据（面差分家、不靠画框）在**文字层**的等价物。
- 第 5 项（多层染色阴影）**不适用**：本轮不动任何面与影（差分 bbox 只有那一个词）。
- 两轮制：第一轮先把五个结果词与两种时间值逐个点过（发现「一条墨」）→ 四视角里「工程师」
  （状态缺失：failed 与 success 同重）与「读者」（眯眼三秒，看不出哪格还没答案）各挑出一处 →
  第二轮只改这两格，并把盒逐字节钉住。

## preflight 逐条（`preflight.md`）

- **第 0 步 · 用户偏好账本**：`P-5 可读性优先于气质`（硬禁令）直接命中——「这一格到底有没有
  答案」读不出来就是可读性问题；`P-1 禁斜体`（未用斜体）、`P-2 先看后选`（本仓走「已有方向，
  只要打磨」的自动车道，不出预览页）本轮无冲突。
- **A 反 AI 套路**：零新增视觉语汇——不动色相、不加影/发光/渐变/斜体、不加新元素；「争夺注意力
  的元素 ≤ 1」成立：这一块里现在只有一个要读的东西（失败/带警告的那个词）。
- **B 设计读取一致性**：读取仍是「近黑/近白底 + 三级面 + 发丝线 + 紧凑行」的轻便液态玻璃；
  记忆点仍是「键盘优先、一行一句话」；本轮只把两格的值从「一条墨」拆回它自己的档，读取不变。
- **C 动效抽查**：本轮不动动效（`Transition` / `ContentTransition` / 呼吸全部原样），
  `prefers-reduced-motion` 无需新增兜底。
- **D2 中文页**：zh 臂与 en 臂走同一条代码路径（同一个 `freshnessResultInk` / `freshnessWhenInk`），
  字号/字重/墨档逐值相同（zh 的 `成功但有警告` / `失败` / `正在探测…` / `未知` / `尚未探测`）；
  **未过项**：正文 10.8px / caption 9.9px 对 D2「正文 ≥ 14px」仍是沿用中的**需拍板**项
  （第 75 份证据），本轮不擅自改字阶——本轮动的是墨，字号一字节没动。
- **E 三秒测试**：眯眼看这块格子——此前四个值一样灰；现在「还没答案的那格」先退下去，
  「要读的那个词」站出来。

## 仍待

1. **下一格**：`Result` 格里最长的那个词 `Succeeded with warnings`（en，约 140u）在窄窗
   （460u 抽屉）下与 `SingleLine()` 的边界——**先量再改**（是否被切、切在哪、要不要让标签先让位）。
2. **需拍板（沿用，未动）**：见 `.pending-bugs.md` 顶部「仍待」第 2 条（字号阶梯对 preflight D2、
   启动器结果行盒高、§6.111/§6.113/§6.114 的尾巴、§6.127 的 127-4、终端弧 S1b/S1c/S2 尾）。
