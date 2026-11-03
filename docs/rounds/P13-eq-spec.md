# P13-eq 规格 · 过滤器 chips：一条做不到的缓动，与两条没人守的契约

**意图**：把「点过滤器」这一格上**声明了但不存在**的动效删掉，并给这一格此前零覆盖的
两条行为/几何契约补上 pin。表面 = **启动器的过滤器 chips 行**（`internal/launcher/filters.go`）。

**为什么现在做**：上一轮（P13-ep）「仍待」第 1 条（§6.111）等拍板，转通用队列的
**状态反馈**。这一族（禁用 / 只读 / 在飞 / 失败）在设置页与浮层已收口，剩下没量过的是
**这一枚 chip**：它悬停换墨、点一下换过滤器，而全仓没有任何测试碰过它。

## 动手前实测（HEAD = 29170c71）

| # | 量的是什么 | 读数 |
|---|---|---|
| 1 | `filters.go` 的 chip 上那条 `Transition{Colors: true, Duration: theme.ColorStep}` | 框架的 `ElementTransition` 只补间**自己的背景色与边框色**（`mygo@v0.7.3/ui/transition.go:17-19`、`:41-43`：「its background and border colors」）。这枚 chip **没有背景也没有边框**，它的墨在子元素 `ui.Text` 上；**文字色不在补间列表里** → 这条缓动是空声明，悬停是瞬变 |
| 2 | 点一枚 chip 之后键盘还在字段上吗 | **在**。`a.Query` 仍收字（探针实测 `"" → "zz"`）。成立的原因是按键由外壳统一路由，而 `internal/launcher` 里 `Focus()` 只有一个调用点（`FocusSearch`）——**没有任何一处把焦点还给字段**，所以这是一条靠巧合成立的路径 |
| 3 | chips 的词挂在哪一列 | chip x = **14.4**（= `Space(4)`，字段那一列）；结果行标题 x = **48.6**（= 54u 文本列）；差 **34.2** = 一整个图标列 |

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `filters.go:210` 的 chip 建树 | `Transition(ui.ElementTransition{Colors: true, Duration: theme.ColorStep})` | **删掉**（框架补不了文字色；§0 不许拿面/渐变冒充一个没有的机制），并把理由与框架行号写在原地；`floter/internal/theme` 的 import 随之退场 |
| 2 | `internal/launcher/chipfocus_test.go`（新） | 无 | `TestAChipClickKeepsTheFieldTyping`：点 `Text` chip → `clipboardFilter == filterText` 且随后打字进得了字段 |
| 3 | 同上 | 无 | `TestTheChipsHangOnTheFieldsColumnNotTheRows`：首枚 chip 的 x 与首行标题的 x 之差 ≥ 25（一整个图标列），并 log 实测 `14.4 / 48.6 / 34.2` |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**零改动**。本轮产品代码只删
一条**不产生任何像素**的声明（前后帧 AE 应为 0），几何与墨一个字节不动。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal      → 空
go vet ./...               → 净（含 GOOS=windows / darwin）
go test -count=1 ./...     → 全绿
--- PASS 计数              → 3353（基线 3351）
测试函数                    → 2525（基线 2523）
SKIP                       → 174（不变）
三平台 build                → 过
```

## mutation 计划（3 条，实跑）

- **M1** chips 行内缩 `Space(4)` → `Space(12)`：`TestTheChipsHangOnTheFieldsColumnNotTheRows`
  必红（gap 34.2 → 5.4），证明这条 pin 读的是几何而不是常数。
- **M2** `setFilter` 的 `a.clipboardFilter = value` → `_ = value`：`TestAChipClickKeepsTheFieldTyping`
  必红（`the chip chose "all", want "text"`），证明 pin 读的是「点下去真的生效」。
- **M3（对照）** 行内缩 `Space(4)` → `Space(3)`：必须仍绿（gap 30.6 ≥ 25），证明 pin 不是
  钉死一个常数。
