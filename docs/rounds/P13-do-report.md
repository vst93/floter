# P13-do 报告 · 只读的集成 ID：一张卡里的两套语法

## 一句话

一张卡里每一行都是「标题 + 值」，只有只读的那一行把**标题**降了两档、把**值**抬成最响 ——
本轮把它放回这张表自己的语法，只读的记号只剩「这一行没有控件面」。

## 选它为什么（证据）

选题来自任务书「细节打磨 · 状态反馈」的后半（禁用 / 只读面）+ P13-dn「仍待」第 1 条。
那条待办说「只读行与可编辑行同档墨」，量下来**正好相反**：不等的是**标题**。

| 墨（`internal/settingsui/idfactrung_test.go`，720×2200） | 改前浅 / 深 | 改后浅 / 深 |
|---|---|---|
| `Integration ID` 的标题 | **122 / 115** | **202 / 198** |
| `local.abc` 的值 | **202 / 198** | **101 / 94** |
| 下面 `Name` 行的标题（可编辑行） | 202 / 198 | 202 / 198 |

同一列 x=201.8、同一 44px 行距，标题差 80 级、值比下面每个字段里的字都响。
值的 x 从 636.8 → **646.8**（等宽 caption 更窄，仍贴卡片右沿）。

## 落地

- `internal/settingsui/settingsui.go` 新增 `fixedRow`：`cardRow` + 标题 `rowTitle` 档
  （`tokens.Emphasis` / 580 / `TextStrong`，`Row{FillHeight, Center}` 居中）+
  值 `deepLinkValueFont` mono / `tokens.Caption` / 500 / `TextMuted`（可选中）。
- 调用点 `settingsui.go:5148` 换成 `a.fixedRow(c, copy.LocalToolId, a.toolEditing)`。
- `keyValue` 本体一字未动（About 那张卡没有一行按得动，§6.11 对它仍然成立）。

红线：高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线 **零改动**；
这一行本来就没有交互，改的只是墨与字体。

## 台账（V / B / S）

- **V 1**：`internal/settingsui/readonlytools_test.go` 的
  `TestTheIntegrationIdRowIsAValueNotASetting`（P13-dl 时代**单独**量了这一行就结案，
  没有把它和下面的行放在一起看）。重定基线：name `122 [107 107 112]` → `202 [27 27 29]`
  （深 `115 [154 154 160]` → `198 [237 237 239]`），value `202 [27 27 29]` →
  `101 [128 128 135]`（深 `198 [237 237 239]` → `94 [133 133 141]`）；
  「值在行的末端」那条**一字未动**，仍绿。
- **B 0**：没有任何行为 pin 变红。
- **S 0**：结构没有变化（仍是 `cardRow`，仍是一行）。
- 新增：1 条 pin（`TestTheIntegrationIDKeepsTheFormRowGrammar`，深/浅各跑一遍：标题与下面那行
  同档、值不许 ≥ 标题、标题与值在同一条光轴上）+ 1 条 opt-in dump。

## mutation（2 条实跑，逐条抄原始报错）

- **M1**：标题换回 `Caption` / 500 / `TextSecondary`
  → 红：`light: the read-only row's title resolves 122 levels but the editable row's title under it resolves 202 — one card, one row grammar (§6.84)`、
  `light: the id's name resolves 122 [107 107 112], want the form row's title rung 202 [27 27 29] (§6.84)`（深色各一条同形）。
- **M2**：值换回 emphasis / 580 / `TextStrong`（并删掉因此空转的 `t := a.cardTheme(c)`）
  → 红：`light: the read-only value resolves 202 levels and the row's own title only 202 — in a settings row the title is the loud line, the value is the answer (§6.84)`、
  `light: the id's value resolves 202 [27 27 29], want the data rung 101 [128 128 135] (§6.84)`（深色各一条同形）。
- 两条都还原：`md5sum -c` 逐字节核对通过（`internal/settingsui/settingsui.go: 成功`），
  再 `touch` 复跑一次：`ok floter/internal/settingsui 1.301s`。
- 顺带发现：新 pin 最初写的是 `valueInk-nameInk > 0`，M2 下 202-202 差值正好是 0，**不红**
  —— 已在第二轮改成 `>=`，重跑 M2 才真的红（那次红就是上面第二段报错）。

## 门槛（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3322（基线 3321）
--- FAIL                                               # 0
--- SKIP                                               # 155（基线 154，+1 = 本轮 opt-in dump）
测试函数                                               # 2485（基线 2483，+2）
GOOS=linux/windows/darwin go build ./...               # 过
```

## 出图（4 张）

`/tmp/floter-P13-do-idrow-<light|dark>-<before|after>.png`（整页 720×2200 帧）。

图上看到的一处变化（浅色，ID 行 y=240..270 的墨带）：改前 `Integration ID` 的字形只占 8 行墨、
比右边 `local.abc` 细弱；改后标题占 12 行墨、与下面 `Name` 的标题同重，而右边的 `local.abc`
退回安静的一档、并排成等宽（`##.##.####.#..##.##.##` 的固定字距），两者仍落在同一行墨带里。

## 偷了什么（qiaomu-design）

- 从 `references/craft-loop.md` **工艺密度菜单第 12 项「材质细节：内边框高光 / 套准」** 的近亲
  ——「同一版面上的两套刻度」——落地为：**一张卡只许有一套行档**（标题 emphasis/580/strong、
  值 caption/500/muted），并把「不可改」从墨移到结构（无控件面）。具体数值：
  标题 `tokens.Emphasis`=11.8px / 580 / `TextStrong`（浅 202 / 深 198）；
  值 mono `tokens.Caption`=9.9px / 500 / `TextMuted`（浅 101 / 深 94）。
- 从 `references/preflight.md` **A4.1「disabled / read-only / hidden 语义没有混淆」** 取判据：
  只读的语义必须由一个可指认的结构承担，而不是由「更小的字」暗示。
- 从 `references/user-preferences.md` **P-5「可读性优先于气质」**：值从 emphasis 降到 caption
  之后仍然可选中、仍贴右沿（x=646.8），没有为了安静而牺牲「能把 ID 抄走」。

## preflight 结论

- **A（反套路）**：通过。本轮没有引入渐变、发光、斜体、装饰色；只换了两个既存 token 的档位。
- **B（设计读取一致性）**：通过。方向仍是「轻便液态玻璃 + 空间感」；本轮不新增材质，只把层级
  收回到既有 token 上（§2 的「靠结构说层级」）。
- **C（动效抽查）**：不适用 —— 这一行没有动效，本轮也没有新增；既有的 1.6s 确认窗口未触碰。
- **D2（中文页）**：**仍未过（第 40 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与
  「正文 ≥ 14px」的硬线冲突，属全局字号阶梯问题，需用户拍板（沿用）。

## 仍待（P13-do）

1. **下一格（细节打磨 · 空态满态）**：`paramempty`（「The script runs fine without parameters.」）
   上一轮判定「已量、不改」，但那条空槽在**深色**下的虚线对比度还没量过；先量，量完再决定。
2. **P13-dg-t / -l / -s**：`internal/terminalui` 与另外两个画面包的夹具取景收口
   （§6.75/§6.76/§6.77），本轮一字未动，仍在队里。
3. **需拍板（沿用 + 第 40 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第四十二次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
