# P13-cg 规格 · 一张全是便条的列表没有游标（只读面）

选题：`docs/rounds/P13-cf-report.md`「仍待 2」= 通用队列「状态反馈」的另一半
（**禁用 / 只读面**）。前 12 轮把「禁用」这一支的**材质**收干了（P13-aq / s / u / ar /
ba / bb / cb / t：受光边熄、只淡一道、面留在原地），剩下的不是材质，是**只读面有没有
游标**。

## 意图（一句话）

捕获输出页的列表里，`kind:"status"` 的行是**便条**不是门（`extensions/rows.go:34-38`）；
一份**全是便条**的列表（报告式插件）今天连游标都没有——`firstRunnable` 返回 `-1`，
于是整页没有任何一行是「被选中的」，方向键不动、Enter 不动、屏幕上也没有一句话说为什么。
这一轮给这张列表一个**只读游标**：游标落在第一行，**面色到了、灯不点**。

## 为什么现在做（证据）

- `internal/launcher/output.go:207-215`：`firstRunnable` 只认 `row.Runnable()`，
  一份没有可运行行的列表拿到 **-1**。
- `internal/launcher/output.go:820`：`selected := i == a.output.Selected && row.Runnable()`
  —— 就算 `Selected` 落在便条上，`&& row.Runnable()` 也会把选中面吞掉。两处一起才
  造成「整页没有游标」。
- `internal/launcher/output.go:429-447`：`moveOutputSelection` 只停在 `Runnable()` 的行上；
  全便条时它转一圈原地不动——**方向键哑了，而且屏幕上看不出哑了**。
- 合约 `docs/liquid-glass.md` §3：受光边是「这块面可以被碰」的记号。便条不可碰，
  所以它的游标**不能**戴受光边；但「游标在这儿」与「这行是门」是两件事，
  今天被合并成了一件（要么都有、要么都没有）。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|------|------|--------|
| 1 | `output.go` `firstRunnable` | 无可运行行 → `-1` | 无可运行行 → `0`（空列表仍 `-1`）；改名 `initialOutputSelection`，注释写清「游标 ≠ 门」 |
| 2 | `output.go` `drawOutputRow` 的 `selected` | `i == Selected && row.Runnable()` | `i == Selected`；门/便条的分档移到下面的 switch |
| 3 | 同一个 switch | `case row.Status \|\| row.Disabled:` 永远不亮 | 新增 `case selected:`（便条被选中）→ `Background(RowSelected)`，**不调 `LitFace`**（§3：灯是「可碰」的记号） |
| 4 | 标题/副标题的字重与墨 | `if selected` → 700 / `t.Text` | 只在 `selected && row.Runnable()` 时抬；便条被选中时仍是它自己的 `TextMuted`（一句话不该因为被选中就变成标题） |
| 5 | `outputText()`（复制取的那一行） | `Selected` 落在便条上会返回便条标题 | 加 `if !row.Runnable() { return "" }`——**保住今天的行为**（今天到不了这一支） |

## 行为契约零改动红线（逐条核对）

- **高度链**：不动。没有新行、没有新 padding。
- **键盘模型**：`moveOutputSelection` 的跳行规则不动（仍只停在可运行行上）；`runOutputRow`
  的 `!row.Runnable()` 提前返回不动。变化只有「`Selected` 的初值」和「它画成什么样」。
- **会话管理 / 设置写回 / i18n 文案 / 命令接线**：零改动（本轮不新增任何文案）。
- **复制**：`outputText()` 加的是**保持**现状的守卫，不是新行为。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux|windows|darwin go build ./...
```

基线（P13-cf 实测）：**3241 PASS / 0 FAIL / 126 SKIP / 2397 测试函数**。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. **M1 · 游标退回旧形状**：`firstRunnable` 的兜底 `return 0` 改回 `return -1`
   → 新 pin 的「便条行戴着选中面」与「`Selected` = 0」两臂同时红。
2. **M2 · 便条游标点了灯**：便条那一支补上 `tokens.LitFace(style)`
   → 「冠线 = 面色」臂红（冠线会比面色亮一档），门那一臂仍绿（它本来就该亮）。
3. **M3 · 复制跟着游标走**：删掉 `outputText()` 的 `!row.Runnable()` 守卫
   → 「全便条列表的复制内容仍是空」臂红。
