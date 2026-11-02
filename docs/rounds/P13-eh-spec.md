# P13-eh 规格：权限块的分组标题比它自己的卡片标题更响

## 意图（一句话）

把「分组标题」这一角色在设置页里的第六个拼法收回到前五个用的那一档——
`Caption + 620 + TextSecondary`——让**卡内子分组**不再比它上面的**卡片标题**更重。

为什么现在做：`docs/liquid-glass.md` §6 的差距表已经没有 `未开始` 行，选题回通用队列的
「层级 / 状态反馈」。上一轮（P13-eg）「仍待」第 1 条建议先量满态，但满态那一半在
§6.48 / §6.49 / §6.69 / §6.70 已经量完并落地，剩下的是空态（两条都写着「需拍板」）。
于是取不需要拍板的那一半：**同一角色在同一页里有两个拼法**。

## 证据（动手前逐处核对过行号）

| # | 位置 | 现状 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go:1351`（`permissionTier` 的 `label`） | `FontSize(tokens.Caption).FontWeight(650).TextColor(tokens.TextStrong)` |
| 2 | `internal/settingsui/rows.go:302-311`（`card` / `cardLabeled`）与 `rows.go` 的 `rowTitle` | 卡片标题 = `tokens.Title + 620 + TextStrong` |
| 3 | `internal/settingsui/settingsui.go` 四处小节标题 | `tokens.Caption + 620 + TextSecondary` |
| 4 | `internal/settingsui/pluginsheet.go` | 浮层分组标题 = `tokens.Title + 620` |

于是层级是**反的**：父级（卡片标题，13.5pt / 620 / TextStrong）之上，子级（卡内分组，
9.9pt / **650** / **TextStrong**）用更重的字面和同样的墨。字重不是「两级台阶」里的
一级，而是一个把子级推到父级之上的旋钮。

## 逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `settingsui.go:1351` | `650` + `TextStrong` | `620` + `TextSecondary`（与同页另外五个分组标题同一拼法） |
| 2 | 同处注释 | 无 | 写明「旧值 → 新值」与理由（父 > 子 由字号 + 墨承担） |

**不改**：字号（`tokens.Caption` 是 D2 的悬案，属「需拍板」）、行盒、间距、任何文案、
`permissionTier` 的调用点与参数。

## 行为契约零改动红线（逐条声明）

- 高度链：行盒不变（pin 里量了：tier 标签的墨带高 == 同字号参照行的墨带高）。
- 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：一行未碰。
- 视觉轮只改墨与面：本轮只改**墨**（`TextStrong → TextSecondary`）与**字重**（650 → 620）。

## 门槛命令与 PASS 基线（本轮实测，不用旧报告的数）

```
gofmt -l cmd internal                     # 空
go vet ./... / GOOS=windows|darwin vet    # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3334 → 3336
GOOS=linux|windows|darwin go build ./...  # 过
```

测试函数 2506 → **2508**（新增 2 条：pin + opt-in 出图）。

## mutation 计划（≥2 条，实跑）

- **M1**：把产品行改回 `650 / TextStrong` → 应打红 `contrast`（220 对 136）、
  「still wearing the stronger ink」与 `ink mass`（47498 对 20952）三条。
- **M2**：只回墨不回重（`620 / TextStrong`）→ 应打红 contrast 与 mass（字重对、墨错）。
- **M3**：只回重不回墨（`500 / TextSecondary`）→ 只打红 mass（**最暗像素量不出字重**，
  §6.51 判据 2 的同一课）。

判不准按 B 先跑：本轮 B（行为/接线）零改动，全部测试零修改必须全绿。
