# P13-ff 报告 · 集成页的导出/导入行：在飞那一句话，比它自己按灭的两枚胶囊还安静

**读取为**：桌面设置面板的集成页，面向正在装/搬工具的人，用「近黑/近白底 + 三级面 + 发丝线 +
紧凑行」的轻便液态玻璃语言，倾向 Raycast 系的设计家族。三拨盘本轮不动：`VARIANCE` 3、
`MOTION` 2、`DENSITY` 7。
**功能契约**：点 Export / Import → 两枚胶囊熄灯 → 行尾一句话说明哪一个在跑 → 结束后出现导入
报告。本轮一个动作都没变，只改「这句话取自哪一档墨」。

## 一句话

导出/导入在飞时，行里唯一还在说话的只有行尾那句 `Exporting…`；它此前穿 `t.TextMuted`，
在深色外观上比它自己按灭的两枚胶囊**还安静 1 级**（104 对 105）。本轮把它抬到本仓既有的
「在飞的词」那一档（`TextSecondary` + 500），于是同一行里出现两个高度：熄掉的面、和站在它
前面的那句话。

## 选题与证据

`docs/rounds/P13-fe-report.md`「仍待」第 1 条：通用队列 · **状态反馈**的另一半（忙 / 进行中），
先量再改。禁用 / 只读面那一半已在 P13-eu / P13-s / P13-t / P13-aq 收口，不重领。

把这一族逐个数过：启动器输出页页脚（`footerLoading`，有环、有 `footerWord` 的两档墨）、
配置工具条的 `Saving…`（P13-el）、更新行的 `Installing…`（P13-dk / P13-aq）、插件表单的
`正在检查本机工具链…`、时间线的 `正在探测…`。**只有集成页的 transferRow 那一句没有落在任何
一档上**：它穿的是这一页「永远不变」的帮助句的墨。

实测（生产取景：`settings.NewStore` 设外观 → `c.SetTheme(a.tokens(c).Theme)` → 720×1600 →
点 `Integrations` → `TransferBusy = "export"`；量的是这一行的卡面，取样带在 import 胶囊右缘
与在飞句左缘之间）：

| 帧 | 卡面 | 在飞句墨权重 | Export 胶囊（已禁用） | Import 胶囊（已禁用） | 差 |
|---|---|---|---|---|---|
| 浅色 | 237 | **107** | 105 | 105 | **+2** |
| 深色 | 27 | **104** | 105 | 105 | **−1** |

代码：`internal/settingsui/settingsui.go:2115-2119`（`transferRow` 的 `switch a.TransferBusy`
两臂），逐字为
`ui.Text(c, copy.IntegrationsExporting).FontSize(a.tokens(c).Caption).TextColor(t.TextMuted)`。

## 逐条落地

| # | 位置 | 现状 → 应改成 | 状态 |
|---|---|---|---|
| 127-1 | `transferRow` 的 export 臂 | `t.TextMuted`（无字重）→ `tokens.TextSecondary` + `FontWeight(500)` | `已落地` |
| 127-2 | `transferRow` 的 import 臂 | 同上（两臂收口到 `busyLine`，一个函数一条声明） | `已落地` |
| 127-3 | 字号 / 盒 / 行内中线 | 不动：仍是 `tokens.Caption`，仍居中于两枚胶囊的中线，仍在它们右侧 | `已落地（pin）` |
| 127-4 | 给这句话加一枚转圈环 | 不动，写进 §6.127 的 `未开始（需拍板）`：环一进来，行的盒高就不再由胶囊决定（环比 15 的 caption 盒高），会顶高整行 | `需拍板` |
| 127-5 | `freshnessRow` 的 `正在探测…` | 不动：那是 unknown / running / success / degraded / failed **共用一档墨**的时间线，只抬 running 一臂会让同一行断档，属另一轮 | `未开始` |
| 127-6 | 墨 / 面 / 动效 / 高度链 / 键盘模型 / i18n / 命令接线 | 一字节不动 | `已落地（不变）` |

**依据（本仓对「在飞的词」本来就有两条规矩，这一处是唯一没跟上的）**：

- 启动器页脚（`launcher/output.go` 的 `footerWord`）：在飞 `TextSecondary` + 500，静止
  `TextMuted` 且不带字重 —— 在飞比静止重一档；
- 设置页配置工具条（`config.go`，P13-el）：在飞与落地**同档**（`TextTertiary`），因为它有
  一枚落地后的孪生词（`Saved`），换词不换墨。

这一句**没有孪生词**（导出结束它整句消失），所以走第一条。

## 台账

**V 0 / B 0 / S 0** —— 一条既有 pin 都没改。`TestIntegrationTransferButtons`（点 Export /
Import 的接线、两句话都在）、`TestTheBusyActionPillWearsTheDimFace`、`TestEveryRowActionLocksWhileAnOperationIsInFlight`
全绿：本轮不动面、不动字阶、不动接线。

新增 3 条（1 条 opt-in 出图）：`TestTheTransferBusyLineOutranksTheButtonsItKilled`（深浅两套，
在飞句的墨权重必须比它按灭的两枚胶囊高 ≥ 15 级；实测 +25 / +22，改动前 +2 / −1）、
`TestTheTransferBusyLineKeepsTheCaptionBoxAndTheMidline`（盒高 = 这一页 caption 行的盒高、
中线 = 两枚胶囊的中线、左缘在胶囊右侧——钉「本轮只改墨」）、`TestDumpP13FFTransferBusyPNGs`。

## mutation（三条实跑）

| # | 注入 | 结果 |
|---|---|---|
| M1 | `busyLine` 的墨 `tokens.TextSecondary` → `tokens.TextTertiary` | 红 ×2：`light busy=95 exportPill=105 importPill=105` / `the busy word does not outrank the pills it locked: word=95 pill=105`；`dark busy=83 … word=83 pill=105` |
| M2 | `busyLine` 的字号 `tokens.Caption` → `tokens.Body` | 红 ×2：`the busy word left the caption step: box {313.8 171.59999 62 17}, the page's hint line {186.8 130.29999 517 15}`（深浅各一条） |
| M3 | 等价写法（builder 调用顺序调换：`TextColor` 提到 `FontSize` 前） | **全绿**（已知覆盖边界：pin 钉的是档，不是调用顺序） |

每条都 `md5sum -c` 逐字节核对还原（`internal/settingsui/settingsui.go`
`b22be7dbf7ad7f484e426d9dc9b417ac`），并 `touch` 后复跑一次避免测试缓存复用。

## 出图（深浅两套）

`before` 用 `git worktree` 在 HEAD `0ae13ec9` 渲（把本轮的 dump 测试拷进去，生产代码是 HEAD 的），
`after` 是工作区。裁的是 transferRow 一带（360×53）。

| 帧 | diff | bbox |
|---|---|---|
| `floter-P13-ff-transfer-light.png` | **314 px** | `(134,22)-(189,33)` |
| `floter-P13-ff-transfer-dark.png` | **314 px** | `(134,22)-(189,33)` |

文件：`/tmp/p13ff/{before,after}/floter-P13-ff-transfer-{light,dark}.png`。

**这张图上量到的一处变化**：差分 bbox 是 56×12，正好是那**一个词**的墨盒（字重让字宽从 55 长到
57，右边多出 2px 的抗锯齿边）——两枚胶囊、卡面、行的下沿一个像素都没动。也就是说这一轮在图上
只发生了「一句话变重」，没有别的东西跟着动。

## 门槛（实测，HEAD 前 `0ae13ec9` → 本轮）

```
gofmt -l cmd internal                              空
go vet ./...                                       净
GOOS=windows go vet ./...                          净
GOOS=darwin  go vet ./...                          净
go test -count=1 ./...                             0 FAIL
PASS 计数                                           3400 → 3402
SKIP                                               188 → 189（新增的 opt-in 出图）
测试函数                                            2566 → 2569
三平台 build                                        linux / windows / darwin 全过
```

## 这轮为什么让它更轻、更有空间感

同一行里，「面」已经在说「这两枚按不动了」（P13-aq 的禁用面：熄冠线、墨落一档）。那么「字」
就必须说「所以现在在发生什么」。此前两者几乎同重（+2 / −1），行内没有层级，只有一个平面：
用户看到三个一样灰的东西，读不出哪个是状态、哪个是失效的控件。现在是 130 对 105（浅）、
127 对 105（深）——**面退、字进**，一行里出现了两个高度。

## 偷了什么（qiaomu-design · `craft-loop.md` 工艺密度菜单）

- **第 6 项（编辑细节：hairline、大写字距标签这一类「编辑性」细节）** → 落成**状态词的档位**：
  在飞句 `tokens.Caption` + `FontWeight(500)` + `tokens.TextSecondary`（出货档 9.9px / 500 /
  浅 `#6B6B70` 深 `#9A9AA0`），与它脚下那两枚禁用胶囊的 105 级明确分开（实测 130 / 127）。
  取的是「同一块面里每个小角色都要有自己的档」，落成的是**三个具体数值**（字号档名、字重 500、
  墨档名），不是「让它明显一点」。
- **第 12 项（材质细节：内边框高光、glass 折射边）** → 落成**面退字进的对照**：本轮不新画任何
  边，而是让已经存在的「禁用面」（熄冠线 + 墨落一档，实测 105）与「在飞的词」（130 / 127）
  形成一对——材质的分层不靠加线，靠**同一行里两个量级的差**。这是本仓 §2 判据（面差分家，
  不靠画框）在**文字层**的等价物。
- 第 5 项（多层染色阴影）**不适用**：本轮不动任何面与影（差分 bbox 只有那一个词）。
- 两轮制：第一轮先量（把这一族六个「在飞」面逐个点过，只有这一处没落在任何一档上）→
  四视角里「工程师」（同一行两个重量相同，读不出主次）与「读者」（眯眼三秒，看不出哪个按钮
  在跑）各挑出一处 → 第二轮只改这一句，并把它钉在盒与中线上。

## preflight 逐条（`preflight.md`）

- **第 0 步 · 用户偏好账本**：`P-5 可读性优先于气质`（硬禁令）直接命中本轮——「唯一说明在发生
  什么的字」读不出来，是这一条要治的病；`P-1 禁斜体`、`P-2 先看后选`（本仓走「已有方向，只要
  打磨」的自动车道，不出预览页）本轮无冲突。
- **A 反 AI 套路**：零新增视觉语汇（不动色相、不加影/发光/渐变/斜体、不加新元素），A 节无一项
  被触发；「争夺注意力的元素 ≤ 1」——这一行里现在只有一个，就是那句话。
- **B 设计读取一致性**：读取仍是「近黑/近白底 + 三级面 + 发丝线 + 紧凑行」的轻便液态玻璃；
  记忆点仍是「键盘优先、一行一句话」；本轮只把一句话的**档**从帮助句挪到状态句，读取不变。
- **C 动效抽查**：本轮不动动效（`Transition` / `ContentTransition` / 呼吸全部原样）；也因此
  `prefers-reduced-motion` 无需新增兜底。
- **D2 中文页**：zh 臂与 en 臂走同一条代码路径（同一 `busyLine`），字号/字重/墨档逐值相同，
  中文串 `正在导出…` / `正在导入…` 沿用全角省略号；**未过项**：正文 10.8px / caption 9.9px
  对 D2「正文 ≥ 14px」仍是沿用中的**需拍板**项（第 74 份证据），本轮不擅自改字阶——本轮动的
  是墨与字重，字号一字节没动。
- **E 三秒测试**：眯眼看这一行——此前三块一样灰；现在两枚安静的胶囊 + 一句清楚的字。

## 仍待

1. **下一格**：§6.127 的 `127-4`（在飞的词要不要带环，需拍板）之外的**同族最后一处**：
   `freshnessRow` 的时间线（unknown / running / success / degraded / failed 共用一档墨），
   要改得整族一起改。**先量再改。**
2. **需拍板（沿用，未动）**：见 `.pending-bugs.md` 顶部「仍待」第 2 条。
