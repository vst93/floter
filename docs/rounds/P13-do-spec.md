# P13-do 规格 · 工具表单里那一行只读的集成 ID：同一张卡里的两套语法

## 意图一句话

一张卡里每一行都是「标题 + 值」，只有一行是例外 —— 那行是**只读**的集成 ID。
只读的记号应该是「这一行没有控件面」，**不该**是「这一行的标题小两档、值比谁都响」。

## 为什么现在做（证据，全部本轮实测）

选题来自任务书「细节打磨 · 状态反馈」的后半（禁用 / 只读面）+ P13-dn「仍待」第 1 条
（`rows.go` 的只读行与可编辑行同档墨）。量下来那句话要**倒过来读**：

- 只读 ID 行走的是 `keyValue`（`settingsui.go:5148`，R9-3：后端铸造、不可改），
  也就是 About 事实表那一套 **P13-an §6.11** 的层级：名字 caption / 500 /
  `TextSecondary`，值 emphasis / 580 / `TextStrong`。
- 同一张卡里，它下面每一行可编辑的行走 `rowTitle`：标题 emphasis 11.8 / 580 /
  `TextStrong`。
- 实测（`internal/settingsui/idfactrung_test.go`，720×2200，`Integration ID` = `local.abc`）：

| 位置 | 浅色 | 深色 |
|---|---|---|
| ID 行的标题（caption 9.9/500/`TextSecondary`） | **122** | **115** |
| ID 行的值（emphasis/580/`TextStrong`） | **202** | **198** |
| 下方 `Name` 行的标题（可编辑行） | **202** | **198** |

同一个 44px 行距、同一个文字列（x=201.8），**标题差 80 级**、值却比下面每一个字段里的
字都响 —— 用户第一眼读到的是「这一行不属于这张表」，而真正该读到的「这一行不能改」
只写在「没有控件面」这件事上，被这套反过来的墨盖住了。

## 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `settingsui.go` 新增 `fixedRow` | 无（该行借 `keyValue`） | `cardRow` + 标题 = `rowTitle` 档（`tokens.Emphasis` / 580 / `TextStrong`，居中）；值 = 程序自己起的名那一档（`deepLinkValueFont` mono / `tokens.Caption` / 500 / `TextMuted`，P13-bt §6.38） |
| `settingsui.go:5148` 调用点 | `a.keyValue(c, copy.LocalToolId, a.toolEditing)` | `a.fixedRow(c, copy.LocalToolId, a.toolEditing)` |

不动的东西：`cardRow` 的 44px 地板 / 分隔线 / 1px 行距 / 左右两端、行高与卡片高度、
值的 `.Selectable()`（用户要能把 ID 抄走）、`keyValue` 本体（About 那张卡没有一行按得动，
P13-an 的层级对那张卡仍然成立）、i18n 文案、R9-3 的「新建时这一行不出现」。

## 行为契约零改动红线（逐条核对）

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**一字未动**。
  只改这一行的墨与字体，不改任何可交互路径 —— 这一行本来就没有交互。

## 门槛命令 + PASS 基线（上轮实测 = P13-dn）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 3321
GOOS=linux/windows/darwin go build ./...               # 过
```

## mutation 计划（≥2 条，每条说清哪个 pin 会红）

1. **M1**：把 `fixedRow` 的标题换回 `tokens.Caption` / 500 / `TextSecondary`
   → `TestTheIntegrationIDKeepsTheFormRowGrammar` 的「one card, one row grammar」在深/浅
   两套外观各报一条红（标题 122 vs 202）。
2. **M2**：把 `fixedRow` 的值换回 `tokens.Emphasis` / 580 / `TextStrong`
   → 同一条 pin 的「in a settings row the title is the loud line」在深/浅各报一条红
   （值 202 = 标题 202）。
