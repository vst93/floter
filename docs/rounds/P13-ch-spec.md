# P13-ch 规格：参数编辑器的空槽说的是一句结论，却和它上面的提示一样淡

起点 `cae6e5a2`（P13-cg）｜表面：**设置 → 集成 → 添加本地工具 → 参数编辑器**（一个表面一轮）
选题：P13-cg「仍待」第 1 条候选 ①（通用队列 `空态满态`）

## 意图（一句话）

`ParamsEmpty` 那句是**这一段的结论**（「脚本不带参数也能正常运行」），但它今天用的是
`Caption + TextTertiary`——与它上面那行**提示**同一个 token、同一档墨，实测**逐像素同值**；
把它抬到 §4.7 的**陈述档墨**，让「这一段没有东西可填，而且这没问题」成为一句话，而不是
第二行灰字。

## 为什么现在做（证据，实测）

渲染 `Integrations → Add a local tool`（720×3200，浅/深两套），量文本盒内**离面色最远**的像素：

| 部件 | 盒 | 墨级差（浅 / 深） |
|---|---|---|
| 卡片的标题 `ParamsTitle`（`parameditor.go:66`，`Caption` + 600 + `TextMuted`） | `{191,968.5,59,15}` | **128 / 147** |
| 提示 `ParamsHint`（`parameditor.go:78`，`Caption` + `TextTertiary`） | `{191,1000,511,15}` | **100 / 96** |
| 空态句 `ParamsEmpty`（`parameditor.go:89`，`Caption` + `TextTertiary`） | `{200,1034,493,15}` | **100 / 96** |

**提示与结论同值（100/100、96/96）**：卡片里最淡的两行是「说明」和「结论」，而结论没有
比说明重一个级。这正是 `docs/raycast-design.md` §4.7 / `settingsui/empty.go` 的 P8-b 判据
「**陈述压过提示**」要防的那件事——全仓五处空态里，四处（会话页 / 自定义快捷键 / 集成页 /
`custom.go:50`）走 `emptyState` 的 Emphasis + `TextStrong`，只有这一处还是参考 CSS 的旧值。

## 顺带量掉的两个猜想（都不成立，写进合约）

1. **「虚线槽的边看不见」——不成立**。`parameditor.go:87` 的 `Border(1, tokens.Hairline)` +
   `BorderDashed` 实测：面色 **246**，虚线像素 **224**（浅）/ 面色 **15**，虚线 **19**（深），
   即 **22 级**（与实线发丝线同一条：0.09 压在同一张面上）。虚线是 3 on / 3 off，槽的四条边
   都在；槽的盒子 `y=1025..1057`、高 **33**（1 + 8 + 15 + 8 + 1），`Padding(Space(2))` = 8u
   上下都对。**不动。**
2. **「图标板叠两道淡」——不在这一轮**（见「仍待」）。

## 落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/parameditor.go:89-90` | `ui.Text(c, copy.ParamsEmpty).FontSize(tokens.Caption).TextColor(tokens.TextTertiary)` | `FontSize(tokens.Caption).FontWeight(580).TextColor(tokens.TextStrong)` |
| `internal/settingsui/parameditor.go:82-84` 的注释 | 写着「the tertiary caption」，与 P8-b 之后的事实相反 | 改成「参考是 tertiary caption，P8-b 之后本仓的陈述档」并写明旧值→新值 |
| `docs/liquid-glass.md` | 无 §6.51 | 新增 §6.51（含「槽不是一页」的判据：尺寸留在 Caption，只抬墨；以及上面两条量掉的猜想） |
| `internal/settingsui/paramempty_test.go` | 不存在 | 新 pin 三条（陈述 ≥ 提示 + 60 级 × 深浅；槽几何不变 33px / 8u；虚线仍 ≥ 15 级）+ opt-in 出图 |

**尺寸为什么不跟着抬到 `Emphasis`**：`emptyState` 的句子是**一页的**结论，这一句是**一个槽**里的
注记——槽是 28u 高的表单格，13px 的句子会把一个「待填的格子」读成「一段正文」。所以这一轮
只抬**墨**与**字重**（与 §6.23 的 23-1 同一条：那一处也只抬墨，没动尺寸）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案（`ParamsEmpty` 一个字不改）、命令接线
**全部不动**。只改墨与字重；槽的几何（1px 虚线 / 8u padding / 28u 下限 / `RadiusSM`）不动。

## 门槛命令 + PASS 基线（本轮实测起点）

```
gofmt -l cmd internal                     → 空
go vet ./... / GOOS=windows|darwin        → 净
go test -count=1 ./...                    → 3248 PASS / 0 FAIL / 128 SKIP / 2404 测试函数
GOOS=linux|windows|darwin go build ./...  → 过
```

## mutation 计划（≥2 条，说明会让哪个 pin 变红）

- **M1**：把 `TextColor(tokens.TextStrong)` 改回 `tokens.TextTertiary` → `paramempty_test.go`
  的「陈述 ≥ 提示 + 60 级」两条臂（浅/深）必须红。
- **M2**：把 `FontWeight(580)` 删掉 → 字重臂（陈述的墨级差随字重变化）红；若级差不变则该臂
  不成立，改钉「同一 token 下 580 与 400 的墨级差」。
- **M3**：把 `Padding(t.Space(paramEditorPlatePad))` 改成 `Space(1)`（4u）→ 槽几何臂（33px）红。
