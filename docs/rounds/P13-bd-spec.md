# P13-bd 规格 · 终端页空态的三件东西，和启动器空页是**同一句话的两个说法**

轮号：**P13-bd**（上一轮 P13-bc）。表面：**终端页的空态**（`internal/terminalui/terminalui.go`
的 `View` 空态臂 + `emptyLine`）。选题来源：通用队列「空态满态」（任务书「下一格从『状态反馈』
的另一半（禁用/只读面）或空态满态里挑，**先量再改**」）。

---

## 一、意图（一句话）

**同一个应用里「一页空着，请做点什么」只许有一种读法**：一句陈述 + 出路；终端页此前把
**标题当说明**（最暗的那一档）而把**两条出路当句子**（更亮的一档），轻重正好反过来，
读起来是「一句灰提示 + 两条亮线」。

## 二、为什么现在做（证据，本轮实测）

`docs/raycast-design.md` §4.7（= `liquid-glass.md` §6.1 表 1-6 的「保持 E0」）给空态钉的
是**一句陈述**：`Emphasis` 步 + `TextStrong`，无底板、无图标、无 accent。P8-b 把
`settingsui/empty.go` 抬到了这一档（会话页 / 自定义快捷键 / 集成页三个空态）。

终端页是**同一族里唯一没抬的那一个**，而且不只是「没抬」——它把阶梯**上下颠倒**了：

| 件 | 位置 | 现状（`terminalui.go:883`、`:968`） | 实测墨（浅 / 深） |
|---|---|---|---|
| 标题 `EmptyTitle` | `View` 空态臂 | `FontSize(t.FontSize + 1)` + **`TextMuted`** | lo **114** / **21** |
| 两条出路 `emptyLine` | 同处 + `:968` | 键帽 `Caption` + 标签 **`TextTertiary`**（hover 抬到 `TextSecondary`） | lo **142** / **21** |
| 页面自身 | — | — | 252 / 21 |

浅色档：标题 114 是**最暗的一件**，出路 142 比它亮 **28 级**；深色档标题与出路**同值 21**
（`textMuted` `#85858D` 与 `textTertiary` `#6E6E76` 在这块面板上都压到同一档）。

对照 `settingsui` 已经落地的同一族（P8-b / P13-aj / P13-ap 实测）：句子 `TextStrong`
（浅 `#1B1B1D` ≈ 27 级）、标记 `TextSecondary`、理由 `TextTertiary`，**句子最重**。

也就是说：本仓现在有**两种**空态语法，一个抬了、一个没抬，而没抬的那个恰好把主句画成了
全页最淡的墨。

## 三、逐条落地项

位置 → 现状 → 应改成。**行号是动手前 `grep -n` 复核过的 HEAD 行号**（`dd49b2a0`）。

### 3-1 标题从「说明」变成「陈述」

- 位置：`internal/terminalui/terminalui.go:883`
  `ui.Text(c, copy.EmptyTitle).FontSize(t.FontSize + 1).TextColor(t.TextMuted)`
- 现状：`TextMuted`（§4.7 的**旧值**——「body + muted」，合约原话：读起来像一条错误提示）。
- 改成：`tokens.Emphasis` + `tokens.TextStrong`，与 `settingsui.emptyState` 的句子**同一条**。
- **字号不动**：`t.FontSize + 1` **保留**。本轮先试过换成 `tokens.Title`（14 × 0.9 = 12.6），
  实测把这一行的盒高从 **23 推到 19**，`TestTheTerminalPageStaysOnTheHeightLadder` 立刻红
  （`height 19.0 is not on the ladder [15 22 23 28]`）——那是**几何**不是墨，属行为契约红线。
  换回 `t.FontSize + 1` 后盒子逐像素回到 23。这一条就是「视觉轮只改墨与面」的现场证据。

### 3-2 出路退到「可以碰，但不是句子」

- 位置：`internal/terminalui/terminalui.go:968` `emptyLine`
- 现状：标签 `tokens.TextTertiary`（hover `TextSecondary`）——比标题亮，读成第二、第三个陈述。
- 改成：静息 `tokens.TextSecondary`，hover `tokens.TextStrong`。
  这样三档成阶梯：标题 `TextStrong` ≫ 出路 `TextSecondary`（可碰）≫ 键帽 `TextSecondary`
  @ `Caption`（它本来就是这个档，**不动**）。hover 是**同一格往上走一档**，不是换色相。
- 为什么不是「出路再压到 `TextTertiary`」：`TextTertiary` 在这块面板的深色档上与
  `TextMuted` 同值（21），压下去等于把出路从「可以碰」抹成「不可碰」——§3 的判据是
  「受光边 / 墨的亮度说这块面可以被碰」，hover 那一档也就无处可去。

### 3-3 键帽（`emptyLine` 的 badge）**不动**

`badge` 已经是 `tokens.LitFace` 的 E1（§6.3 表 3-8，P13-d 落地），`Caption` 墨也是合约里
键帽的档。本轮一个字节不动——这是「同一族里只改真正颠倒的那两件」的纪律。

### 3-4 新增 pin：`internal/terminalui/emptyregister_test.go`

四条账，深浅两套外观各跑一遍（8 个子测试）：

1. **标题是这一页最重的一件**：标题墨 lo ≤ 出路墨 lo − 20 级（浅 / 深各一）。
2. **标题在 §4.7 的档上**：标题墨 lo 必须与 `tokens.TextStrong` 在同一块面板上的合成值
   **逐字节相同**（不写死像素，用同一帧的 `TextStrong` 参照物：见 3-5）。
3. **出路不是陈述**：出路墨 lo > 标题墨 lo + 20 级（即 1 的镜像，写成独立断言以免
   一侧退化成另一侧的复制）。
4. **hover 抬一档**：指针落在出路行上，它的墨比静息**更亮**（lo 至少 −10 级），
   且键帽的冠线一个字节不动。

### 3-5 参照物怎么取（不造脆弱断言）

第 2 条不写死 `#1B1B1D`：同一帧里画一条**隐藏的参照文本**是不行的（会进 `Texts()`）。
改用**同帧已有的一件 `TextStrong` 墨**——终端页浮栏的身份行（`identity`，`tokens.Text`）
不在 `TextStrong` 上。所以这一条**降级成**：标题墨 lo ≤ 面板 lo − 100 级（浅）/ ≥ 面板 + 100
（深），即「陈述级的对比」，并把**逐字节等值**那一半留给 `theme` 自己的 token 账
（`TestTheEmptyStateRegisterIsTheStatementsRegister` 只断言调用的是
`tokens.TextStrong`——用 `go/ast` 读 `terminalui.go` 的那个调用点，与 P13-p 的查询形状同源）。

**这就是本轮的一处取舍，写进报告**：墨的逐字节等值在这里拿不到参照物，所以拆成
「像素级对比下界」+「AST 级 token 等值」两条，而不是一条写死像素值的断言。

## 四、行为契约零改动红线（逐条点名，本轮全部不动）

| 契约 | 本轮 |
|---|---|
| 高度链（`pageheight_test.go` 钉的 23 / 22 / 15 / 22） | **不动**（3-1 只换墨与字阶，盒子不变） |
| 键盘模型（`c.Shortcut(0, ui.KeyEnter)` → `EnsureSession`） | **不动** |
| 会话管理（`EnsureSession` / `Actions.NewCommand`） | **不动** |
| 设置写回 / i18n 文案（`EmptyTitle` / `EmptyNew` / `EmptyBack` 三句原文） | **不动** |
| 命令接线（`emptyLine` 的 `run`） | **不动** |

## 五、门槛命令 + PASS 基线（本轮实测，不抄旧报告）

```
gofmt -l cmd internal                  # 必须空
go vet ./...                          # 必须净
GOOS=windows go vet ./...             # 必须净
GOOS=darwin  go vet ./...             # 必须净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（HEAD `dd49b2a0`，本轮开工前实测一次，见报告 §门槛）：

- PASS **3132** / FAIL **0** / SKIP **104** / 测试函数 **2306**（P13-bc 的收尾数字，本轮复量确认）。

## 六、mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

| # | 注入的缺陷 | 预期变红的 pin |
|---|---|---|
| M1 | 标题退回 `t.TextMuted` | `emptyregister_test.go` 账 1（标题 vs 出路）与账 2（对比下界） |
| M2 | 出路退回 `tokens.TextTertiary` | 账 3（出路必须比标题淡）+ 账 4（hover 抬一档） |
| M3 | hover 臂退回静息档（删掉 `if line.Hovered()`） | 账 4 的 hover 半边 |
| M4 | `go/ast` 普查：标题那一行改回 `.TextColor(t.TextMuted)` 以外的**非 token 色**（如 `ui.Hex("#1B1B1D")`） | 账 2 的 AST 半边（token 等值） |

## 七、出图

`/tmp/floter-P13-bd-terminal-empty-{light,dark}-{before,after}.png`，opt-in
`FLOTER_P13BD_DUMP=1`。眼睛复核一处：标题与两条出路在同一帧里的**轻重关系**。
