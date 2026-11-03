# P13-fc 报告：终端空态的那句话，比它自己的第一条出路还贴得近

轮号 P13-fc（HEAD 前 P13-fb）。方向：通用队列 · **空态满态**（P13-fb「仍待」第 1 条）。
表面 = **终端页空态**（`internal/terminalui/terminalui.go` 的 `View` 空态臂）。
合约新增 **§6.124**。

## 结论一句话

空态是**一句话 + 两条出路**：这句话是那两条出路的标题，两条出路是彼此的同级。三件东西
此前挂在**同一条** `Gap(t.Space(2.5))` 上，墨上实测标题→出路 1 是 **11.3px**、出路 1→
出路 2 是 **13.6px** —— 标题离自己的第一条出路比两条同级出路彼此之间还近 2.3px，
三件读成一条等距竖排清单，句尾直接黏在第一个能按的行上。按 §4.3 的规矩（「只有这一对里
有一件是**可以按的面**的那一对才抬开」）把外层抬一整档到 `Space(5)`，两条出路留在自己的
内列里不动：三档变成 **11.3 / 13.6 / 20.3**。

## 量的事实（900×600 真页面，en/zh × 深/浅四臂**逐值相同**）

| 探针 | 改前 | 改后 |
|---|---|---|
| 标题 `EmptyTitle` | 130×18 @(385.0, 260.4)，底 278.4 | 130×18 @(385.0, **255.9**)，底 273.9 |
| 出路 1 `EmptyNew` | 94×17 @(418.5, 289.7)，底 306.7 | 94×17 @(418.5, **294.2**)，底 311.2 |
| 出路 2 `EmptyBack` | 74×17 @(438.5, 320.3) | 74×17 @(438.5, **324.8**) |
| 键帽 `↩` | 11×15 @(393.9, 290.7) | 11×15 @(393.9, **295.2**) |
| 标题底 → 出路 1 顶 | **11.3px** | **20.3px** |
| 出路 1 底 → 出路 2 顶 | **13.6px** | **13.6px** |
| 整块高（居中） | 77.2 | **86.2**（标题上移 4.5、两条出路下移 4.5） |
| 失败态：标题底 → 原因顶 | 9.0px | **9.0px**（陈述内列不动） |
| 失败态：原因底 → 出路 1 顶 | 11.3px | **20.3px** |

中文臂（`暂无终端会话` / `新建空白会话` / `回到搜索`）与英文臂**逐值相同**——这是空气的
改动，不是文字的改动。

## 落地

`internal/terminalui/terminalui.go`（空态臂，只此一处）：

1. 外层 `Gap(t.Space(2.5))` → **`Gap(t.Space(5))`**（一整档，9u → 18u）。
2. 标题 + 失败原因收进**陈述内列**（`Gap(t.Space(2.5))` 不动，墨上 9.0px）；
   两条 `emptyLine` 收进**出路内列**（`Gap(t.Space(2.5))` 不动，墨上 13.6px）。
3. 两个内列 `.AlignItems(ui.Center)`：失败句比标题宽时，标题仍居中于「标题 + 原因」这一块，
   而不是被顶到左边（这是拆内列必须补的一手，否则失败态会露馅）。
4. `c.Shortcut(0, ui.KeyEnter)` 的注册位置与顺序**不动**（仍在两条出路之前）。
5. `emptyLine` 的墨 / 键帽 / `LitFace` / 悬停-按下两档、`a.Err` 的文案与墨、
   标题的 `FontSize+1` + `TextStrong`：**一字节不动**。

## 台账

**V 0 / B 0 / S 0** —— 生产代码只改了这一块空态内部的空气；没有任何既有 pin 变红
（`TestEachTerminalPageRungHasItsOwnProducer` 钉的是**盒高**，本轮盒高一个没动；
`TestTheEmptyStateRegisterIsTheStatementsRegister` 走 AST，缩进变了、调用链没变）。

新增 pin：`internal/terminalui/emptyspace_test.go`

- `TestTheEmptyStatesSentenceSitsAboveItsWaysOut`：en/zh × 深/浅四臂，标题→出路 1 = 20.3px、
  出路 1→出路 2 = 13.6px，且前者至少比后者宽 5px（**排序**判据：字体度量变了不误报，
  但「标题贴得比同级还近」一定红）。
- `TestTheEmptyStatesReasonStaysWithItsSentence`：失败态两套外观，标题→原因 ∈ [7, 11]px、
  原因→出路 1 = 20.3px、且原因离自己那句话比离出路更近。
- `TestDumpP13FCEmptySpacePNGs`（opt-in `FLOTER_P13FC_DIR`）：900×600 空态两帧。

## 门槛（全部实测）

- `gofmt -l cmd internal`：空
- `go vet ./...` + `GOOS=windows go vet ./...` + `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**3394 PASS / 0 FAIL / 186 SKIP**
  （基线 = HEAD `a148ff47` 开工时实测 **3386 / 0 / 185**；+8 PASS = 两条新 pin 及其
  6 个子臂，+1 SKIP = 新增 opt-in 出图）
- `go test -count=1 -v ./... | grep -c -- "--- PASS"`：**3394**（基线 3386，只涨不跌）
- 测试函数：2557 → **2560**（只增不减，没有新增 `t.Skip` 之外的空洞）
- `GOOS=linux/windows/darwin go build ./...`：三平台全过

## mutation（3 条实跑 + `md5sum -c` 逐字节还原）

1. **M1** 外层 `Gap(t.Space(5))` → `Gap(t.Space(2.5))`（退回旧的一条缝）：红，深浅 × 中英
   四臂，报错串
   `light: the statement sits 11.3px above its first way out, not 20.3 — the empty state's outer gap moved (§6.124)`
   + `the statement is 11.3px from its first way out and the two ways out are 13.6px apart — the sentence reads as the first item of the list, not as its title (§6.124)`。
2. **M2** 出路内列 `Gap(t.Space(2.5))` → `Gap(t.Space(5))`：红，
   `light: the two ways out are 22.6px apart, not 13.6 — they are peers and their gap is not the statement's (§6.124)`，
   并且排序判据也红（`the statement is 20.3px … and the two ways out are 22.6px apart`）。
3. **M3** 陈述内列 `Gap(t.Space(2.5))` → `Gap(t.Space(0))`：**第一次是绿的**——pin 2 原版只有
   上界（「原因不许变成新段落」），塌到 0 反而「更贴」。补上下界后红：
   `light: the failure reason is 0.0px below its own sentence — welded to the title, two lines of one sentence read as one cramped line (§6.124)`。
   这是本轮抓到的一条**覆盖边界**：一条间距 pin 只钉一侧，另一侧就是黑洞。
4. 还原：`cp` 回原文件 + `md5sum -c`（`internal/terminalui/terminalui.go: 成功`）+ `touch` 复跑
   全绿（避免测试缓存复用）。

## 出图

`/tmp/p13fc/{before,after}/floter-P13-fc-empty-{light,dark}.png`（`before` 用 `git worktree`
在 HEAD `a148ff47` 渲，同一个 dump 测试；`after` 在本工作区渲）。

- 像素差 **3470（浅）/ 3478（深）**，bbox `(386,259)-(515,341)` —— 整块空态，不是一处小数点。
- 墨行实测（浅色）：改前标题墨 264-276 / 出路 1 墨 294-301 / 出路 2 墨 325-332；
  改后 259-271 / 299-306 / 329-336。
- **这张图上看到的一处变化**：标题的墨与第一条出路的墨之间从 18 行空变成 28 行空，
  而两条出路之间仍是 23-24 行——句尾不再黏在第一个能按的行上，两条出路第一次读成一组。

## 用 skill 的账（`qiaomu-design`）

开工前读了 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` → `preflight.md`
（本轮是「已有方向、只要打磨」，没有走 Phase 2 的四方向预览/HTML 预览页，也没有等点选）。

**偷了什么**（逐条落成具体数值/动作）：

1. `craft-loop.md` 二·工艺密度菜单**第 6 项「编辑细节」**的「先量后改」：落成 11.3 / 13.6 两条
   墨缝的实测，以及改后的 20.3——先量，再决定抬多少。
2. `craft-loop.md` 三·两轮制**四视角评审的「工程师」视角**（对不齐/间距不一致）：落成
   「同一条 `Gap` 服务三个不同角色」这个诊断——三件东西挂在一条缝上，层级只能靠运气。
3. `craft-loop.md` 二·工艺密度菜单**第 5 项「多层染色阴影」**：不适用（空态是 E0 页面级面，
   §2 判据 2 不给它投影），本轮**故意不做**，不硬凑密度。
4. `user-preferences.md` **P-5「可读性优先于气质」**：落成拒绝「加一条分割线/抬字号」这类
   更响的做法——只动空气，墨一个字节没改，标题仍是 `TextStrong`、出路仍是 `TextSecondary`。
5. `preflight.md` **D「空态/加载态/错误态齐全；超长文本不破版」**：落成失败态那一臂的独立
   pin（原因句与它的标题是一块，且与出路保持抬开的一档）。

## preflight 逐条结论

- **A 反 AI 套路**：通过。无渐变/发光/纯黑/斜体/竖线装饰；本轮只动三块空气。
- **B 设计读取一致性**：通过，且**本轮就是这一条抓到的**——「陈述」与「可点的行」是两种东西，
  同一条缝把它们读成一种。
- **C 动效抽查**：通过（不适用）。本轮不触碰动效；悬停/按下两档的 pin 照旧全绿。
- **D2 中文页**：通过（不适用字号/字体栈）。`zh` 两臂与 `en` 逐值相同：盒、缝、整块高一致，
  没有因为中文行短而出现另一套节奏。
- **不适用并说明**：A2（DNA 注入/工艺密度 ≥5）——本轮不是新建界面；A5（四方向预览）
  ——用户已给死方向，走「已有方向只要打磨」车道。

## 仍待

1. **下一格**：§6.124 的 `124-5`——**启动器无结果态**的密度（同一族的下一个表面，
   一轮一个表面，本轮不做）。先量：无结果句与它的字段/动作之间的缝。
2. **需拍板（沿用，未动）**：见 `.pending-bugs.md` 顶部「仍待」第 2 条（字号阶梯对
   preflight D2 的 14px、启动器结果行盒高、§6.111/§6.113/§6.114 的尾巴、终端弧 S1b/S1c/S2 尾）。
