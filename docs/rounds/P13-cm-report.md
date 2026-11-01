# P13-cm 报告 · 同一个结局：终端页用暖色说，会话列表用灰说

规格 `docs/rounds/P13-cm-spec.md`，合约新增 `docs/liquid-glass.md` **§6.56**。

## 这轮为什么让它更轻、更有空间感

空间感不是加投影，是**让每一行只说一件它的层级该说的事**。会话列表那一行有四个事实：
会话名（标题）、状态词、屏幕尺寸、开始时间。此前状态词与后两个**同一档墨**——于是一行
读起来是「四个同等重量的碎片」，失败与成功也只能靠三个字符（`· exit 3`）分辨。这一轮把
**结局**从事实里分出来：失败那一档走暖色（`t.Warning`，与终端页的常驻便条同一个来源），
其余三件留在静息档。一行于是有了**两档**：这一行发生了什么（可以很响）／这一行的背景资料
（一律很轻）。深浅两套外观同序，行高、行盒、字阶、字重**一个像素没动**。

## 改法（四个文件、六处）

| # | 位置 | 改前 | 改后 |
|---|---|---|---|
| 1 | `settingsui/settingsui.go` `Session` | 没有结局信息 | 加 `Failed bool`（只回答「要不要用暖色说」，不搬退出码） |
| 2 | `settingsui/rows.go` 角色枚举 | `rowSegmentTertiary` / `rowSegmentStrong` | 加 `rowSegmentWarning`（第三枚） |
| 3 | `settingsui/settingsui.go` `sessionMeta` | `Running` 二选一 | 三选一：运行中 → `Strong`；已结束且失败 → `Warning`；其余 → `Tertiary`（运行中压过失败） |
| 4 | `settingsui/rows.go` 画笔 | 只认 `Strong` | `switch` 认 `Strong` 与 `Warning`（`t.Warning`，与 `residentNote` 同一个 palette 成员） |
| 5 | `terminalui/terminalui.go` 新增 `Failed(index)` | 无 | 读 `Sessions[index].ExitCode != 0`，且必须先 `Exited`（没结束的会话没有结局） |
| 6 | `shell/shell.go` 拼列表 | 只传 `Running` | 加 `Failed: a.Terminal.Failed(i)` |

**不动的**：`Note` 的文案与 `NoteOf` 的措辞（i18n 零改动）；行几何（高度 / padding / gap /
字阶 / 字重 / 圆角）；`rowSegmentStrong` / `rowSegmentTertiary` 的语义与色值；`residentNote`；
会话的 list / attach / kill 语义。

## 实测（720×760 帧，深浅各一遍；`c.SetTheme(a.tokens(c).Theme)` 的面板主题）

| 臂 | 改前（浅 / 深） | 改后（浅 / 深） |
|---|---|---|
| 运行中 | `(27,27,29)` / `(237,237,239)` | 不动 |
| 干净退出 | `(142,142,148)` / `(110,110,118)` | 不动 |
| **失败退出** | `(142,142,148)` / `(110,110,118)` — 与干净退出**逐像素相同** | `(156,31,24)` / `(216,153,142)` = `t.Warning` |
| 同行事实（尺寸） | `(143,143,149)` / `(109,109,117)` | 不动 |
| 行地面 | `(229,229,230)` / `(39,39,41)` | 不动 |

离地（平方距离）：失败 **68426 / 54729**，同行事实 **21862 / 16011**。失败臂 `r−b` = **132 / 63**
（改前 −6 / −8），干净臂与运行中臂 `|r−b| ≤ 8`。

> 一处口径修正：这一页的 `t.Warning` 必须从**面板自己的主题**读（`c.SetTheme(a.tokens(c).Theme)`）。
> 第一版 pin 直接 `render(...)`，量到的是框架的库存 warning `#d97706` / `#f59e0b`——像素断言
> 仍然全绿，但量的不是这个设计。改的是 pin 的取景，不是产品；记在这里以免下一轮踩同一脚。

## 变异（实跑，三条）

- **M1**（`sessionMeta` 失败臂退回 `rowSegmentTertiary`）：**红**，六条——
  `light: the failed state word is not warm: {142 142 148} (r−b = -6)`、
  `light: a clean exit and a failed one still render the same colour`、
  `light: the failed ending does not stand off the ground more than its own meta text: failed {142 142 148} (21862) vs size {142 142 148} (21862)`，
  深色同形（`{110 110 118}` / `16011`）。`md5sum -c` 逐字节还原、`touch` 复跑绿。
- **M2**（角色加了、**画笔没接**：`rowSegmentWarning` 也画 `TextTertiary`）：**红**，与 M1 同样
  三条 —— 证明红的是**像素**，不是枚举有没有那一枚。还原、复跑绿。
- **M3**（反向对照：**运行中**那一臂也画 `Warning`）：**双红**——
  `light: a running session took a hue: {156 31 24} (r−b = 132)`（深 `{216 153 142}`），
  以及角色账 `running role = 2, want the strong neutral rung`。
  证明暖色是**结局**的记号，不是「这一行是重点」。还原、复跑绿。

## 门槛（全部实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c PASS → 3263 → 3267
GOOS=linux/windows/darwin go build ./... → 过
```

SKIP 131 → **132**（新增的 1 条是 opt-in 出图）。测试函数 2416 → **2421**（HEAD 实测 2416，
不是上一轮报告记的 2415——同一处 off-by-one 又一次）。**V 0 / B 0 / S 0**：没有旧 pin 需要
重定基线，`settingsui` / `terminalui` / `ci` / `shell` 一次全绿。

> 插曲：新 helper 最初取名 `renderThemed`，被 `internal/ci/deadcode_contract_test.go` 的账本
> 判红（`internal/settingsui/raycast_test.go renderThemed` 是 P11-cr 删掉的死代码，账本按
> **目录 + 符号名**判）。改名 `renderOnPanelTheme` 后过——账本不许改，只能换名。

## 出图

`/tmp/floter-P13-cm-sessionoutcome-{light,dark}-{before,after}.png`（整帧 dump，opt-in
`FLOTER_P13CM_DUMP=1`，`before` 是 M1 注入时抓的旧画）。

**差分**：`after` 对 `before` **278 px**，包围盒 `(234,292)-(299,301)`（65×9）——就是失败
那一行的状态词自己（11px caption），深浅两张**同一个盒**。这张图上看到的一处变化：
第三行的 `Exited · exit 3` 从灰变成暗红（深色档是从灰变成浅桃），而它上面的会话名、
下面的尺寸与时间、以及整行的高度**一个像素没动**。

## preflight（qiaomu-design，逐条结论）

- **A 反套路**：无斜体、无渐变文字、无外发光、无紫蓝渐变、无三等分卡片、无左侧竖色条、
  无「革命性」类文案、感叹号 0。这一轮只把一档墨换成语义色相。**过**。
- **B 设计读取一致性**：本仓单一来源 `docs/liquid-glass.md`；本轮落的是 §3 之外的**语义色**
  判据（色相归语义），并写进 §6.56。与 §6.26 的判据同序，没有引入第二种说法。**过**。
- **C 动效抽查**：本轮零动效（静态墨），`ReduceMotion` 与帧率无关。**过**。
- **D2 中文页**：零文案改动（i18n 红线）；中文页的 `会话已退出 · exit 3` 跟着同一个角色走，
  暖色不依赖语言。**过**（`tokens.Caption` 11px 仍低于「正文 ≥ 14px」——那是**沿用**的拍板项，
  不是这一轮引入的，见「仍待」第 2 条）。
- **A2 工艺密度（偷了什么）**：
  - **DNA 注入**：参考的 `.session-manager__meta` / `.session-state`（`SessionsPage.tsx:173-177`）
    只给了两档颜色，**没有**失败臂；所以偷的是**本仓自己的**两处先例，而不是参考的 CSS——
    `residentNote` 的 `ExitCode != 0 → t.Warning`（终端页）与 §6.26 为输出页定的「失败臂
    `t.Warning` 580」。落成的具体数值：色相 = `t.Warning`（浅 `#9C1F18`、深 `rgba(255,178,164,.82)`），
    字重 500、字阶 `tokens.Caption`（11px）**均未改**。
  - **工艺密度菜单**：取**第 6 项「编辑细节」**——「一行里同一类东西只有一个档」：状态词此前
    与尺寸 / 时间挤在同一档，这一轮把「结局」独立成一档，落成 `rowSegmentWarning` 与
    `r−b ≥ 40` 的暖色门槛（实测 132 / 63）。**只取到 1 项，不凑 5 项**：这一轮是单条墨的收口，
    凑密度就是加装饰，与「一轮只做一件事」和 §0 的「材质只在需要分层的地方出现」冲突。
    照实写在这里，不虚报。
- **两轮制**：第一轮 = 实现 + 像素 pin；第二轮的「四视角」只落一处——**工程师视角**：
  第一版 pin 用框架库存 warning 量（见上「口径修正」），第二轮改成面板主题取景。
  艺术总监 / 甲方 / 读者三个视角在一条 11px 状态词上没有第二处可挑，照实说明。

## 仍待

见 `.pending-bugs.md` 顶部「仍待（P13-cm）」。
