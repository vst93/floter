# P13-bg 报告 · 捕获输出页的状态行：三种结局长得一模一样

轮号 **P13-bg**｜分支 `mygo-rewrite`｜规格 `docs/rounds/P13-bg-spec.md`｜
合约新增 **§6.26**（6 行：5 `已落地` + 1 `已落地（复核）`）

## 一句话

捕获输出页条子上那一行把**这一页的结局**（`Finished` / `Exit code 3` / `Timed out`）
与**它自己的键位图例**（`Enter copies · Esc closes`）用同一档墨、同一个字重画出来，
只隔一个中点；而且**三种结局逐像素相同**——一次失败的运行与一次成功的运行在条子上
长得一样。本轮把它拆成三档：结局、中点、图例；失败/超时臂拿到语义色。

## 这轮为什么让它更轻、更有空间感

层级只由**差**说话。这一行此前是**一件事**：`Finished` 与 `Enter copies · Esc closes`
在 11px 的同一档灰里，实测浅 **116**、深 **164**，**三个结局臂一个字节不差**。于是
条子上有两处「糊」：

1. **状态没有画出来**。`OutputView.Status` 只是一串字，条子上没有任何东西知道这次
   运行是成功还是失败。这是「状态反馈」这一格最硬的一条：不是差得少，是差为 0。
2. **结论与说明书同一档**。结局是这一页的**结论**，图例是它的**说明书**；同一个寄存器
   让它们读成一句话的两个从句。

改后三档（对地面，浅 / 深）：标题 **228 / 221** ≫ 结局 **145 / 133**（失败 139 / 115）
≫ 图例 **109 / 89**。这一行第一次有 z 轴。

## 落地（`internal/launcher/output.go`）

| # | 位置 | 改前 | 改后 |
|---|---|---|---|
| 1 | `OutputView`（`output.go:43`） | 只有 `Status string` | 加 `Failed bool`（`run.TimedOut \|\| !run.Success`） |
| 2 | `OutputFor`（`:149`） | 只填 `Status` | 同时填 `Failed` |
| 3 | 条子状态行（`:546` 附近） | `ui.Text(Status + "  ·  " + hint).FontSize(FontSize-1).TextColor(TextMuted)` | `ui.RichText(c, a.outputStatusSpans(...)...).FontSize(FontSize-1)` |
| 4 | 新增 `outputStatusSpans` | — | 结局 `TextSecondary`/580（`Failed` → `t.Warning`/580）、中点 `TextTertiary` 0.7、图例 `TextTertiary`/500 |

**图例为什么必须退到 tertiary**（本轮实测的取舍）：留在 `TextMuted` 时它（浅 116 /
深 164）与结局 `TextSecondary`（108 / 156）只差 **8** 级——那正是「差值太小」的原样；
深色档失败臂更是**低于**图例（141 对 164 = −23）。退一档后结局压图例 36 / 44（成功）
与 30 / 26（失败）。

**几何零变化**：三段同一个 `FontSize - 1`，`RichText` 只换墨与字重；标题、两枚 28u 方键、
条子 padding、行盒全部不动。整帧像素差 bbox 是 `(12,93)-(250,107)`——就是那一行自己。

## 门槛（全部实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` → **净**；`GOOS=windows go vet ./...` → **净**；`GOOS=darwin go vet ./...` → **净**
- `go test -count=1 ./...` → **0 FAIL**
- PASS **3154 → 3157**（**+3**）｜FAIL **0**｜SKIP **107 → 108**（+1，opt-in 出图）｜
  测试函数 **2319 → 2323**（+4）
- `GOOS=linux/windows/darwin go build ./...` → **三平台全过**
- **V 0 / B 0 / S 0**（没有任何旧 pin 变红；本轮只加 pin）

## 变异（4 条实跑，全红，逐字节还原）

| # | 注入 | 红在哪 |
|---|---|---|
| M1 | `Failed: false`（不读运行结果） | `TestTheVerdictIsNotTheSameLine…`：`the fault verdict is not in a warm hue (r−b = -5, want ≥ 40)`；`the failed run draws the same verdict ink as the successful one` |
| M2 | 结局退回 `t.TextMuted` | `TestTheOutputStatusLineSeparates…`：`dark=false finished: gap 8, want ≥ 20`（结局与图例塌回同一档） |
| M3 | 图例跟着抬到 `t.TextMuted` | 同一条：`gap 8`（浅）/ `gap -8`（深）——两个方向都测过 |
| M4 | 失败臂改用 `t.Text`（抬亮但不换色相） | `the fault verdict is not in a warm hue (r−b = -3 / -1)` |

`md5sum -c` 逐字节核对：`380de7714d5e47dbd3607e942c6ca22b`，`touch` 后复跑绿。

## 出图（自己用眼睛复核过）

`/tmp/floter-P13-bg-outputstatus-{light,dark}-{finished,failed,timedout}-{before,after}.png`
（`FLOTER_P13BG_DUMP=1 FLOTER_P13BG_TAG=<before|after>`）。整帧逐像素对照：

| 帧 | 变化像素 | bbox |
|---|---|---|
| light finished | 1479 | (12,93)-(236,107) |
| light failed | 1591 | (12,93)-(250,107) |
| light timedout | 1513 | (12,93)-(247,107) |
| dark finished | 1479 | (12,93)-(236,107) |
| dark failed | 1593 | (12,93)-(250,107) |
| dark timedout | 1513 | (12,93)-(247,107) |

**图上看到的一处变化**：浅色 `failed` 帧里，`Exit code 3` 从与右侧图例同色的灰变成
暖橙，而 `Enter copies · Esc closes` 明显退后一层——一行里第一次读得出「这是结论 /
这是说明书」。深色 `failed` 帧同一处更明显（暖橙 `245,158,11` 压在近黑上，图例掉到
`110,110,118`）。

## 偷了什么（qiaomu-design `craft-loop.md` 工艺密度菜单，逐项）

本轮读的是 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` + `preflight.md`
（打磨模式那条车道，不走 Phase 2 四方向预览——方向已由用户定死）。

- **第 6 项「编辑细节 / hairline / 大写字距标签」** → 落成「中点 `·` 单独一段墨」
  （`TextTertiary` 0.7，与 `settingsui` 事实行的同一个数）：一行里的分隔符是**标点**，
  不是内容，所以它自己一档。
- **第 12 项「材质细节 / 内边框高光」的**判据**（同一面只说一句话）** → 落成「结局换色相
  不换字阶」：`t.Warning` 是语义色（§6.19），抬字阶会把「故障」读成「更重要的正文」。
- **第 5 项「多层染色阴影」的**近亲**——「层级靠差、不靠描边」** → 落成三档墨的算术
  （228 / 145 / 109），并因此**把图例从 muted 退到 tertiary**：留在 muted 只差 8 级，
  等于没分层。
- **preflight A（反 AI 套路）**：无渐变、无发光、无装饰性强调；**B（设计读取一致性）**：
  这一页只有一个方向（结论压说明书），与 `liquid-glass.md` §4.7「陈述压过提示」同一条；
  **C（动效抽查）**：本轮零动效（只换墨，颜色过渡仍走 `theme.ColorStep` 120ms 的既有路径）；
  **D2（中文页）**：`zh` 文案一个字节不改（`已完成` / `退出码 3` / `超时` 仍在
  `FontSize - 1` 的既有字阶上，中西文间距由 `i18n` 原文自带）。
  **不能过的项**：D2「正文 ≥ 14px」——这一页的状态行是 `FontSize - 1`（11px），
  与 `tokens.Caption` 同一档张力，**改字阶=改高度链**，已记在 `.pending-bugs.md` 的
  「需拍板」表（沿用 P13-bd / P13-bf）。

## 仍待（P13-bg）

1. **需拍板（沿用，本轮又给了一条证据）**：`FontSize - 1`（11px）的状态行与 preflight
   D2「正文 ≥ 14px」的张力。**改字阶 = 改高度链**（P13-bd 已实测撞红）。必须用户拍板。
2. **下一格候选**：`internal/shell/detach.go` 的插件窗口里，同一个运行**没有**这一行
   （它只有正文状态行，§6.24 已收）——如果用户希望两处一致，那是「跨表面统一」的一轮，
   不是这一轮能顺手改的（它属另一个表面，按纪律不跨）。
3. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾。
4. **抢救 stash（沿用 P13-be 的结论，本轮未再动）**：`stash@{0}`..`stash@{12}` 全部不需再试合。
