# P13-ay 规格：启动器的七处提示迁到 `tokens.Tip`（§6.20 的 20-2）

## 意图

`docs/liquid-glass.md` §6.20 的 **20-2** 行：启动器的七处提示仍是框架
`Element.Tooltip` 的反色平涂（`ui/tooltip.go:127-133`：`Radius(Space(1.25))` = 5px、
`inverse()` 平涂、一层 `0 2px 8px 0.2`、零条边），而同一个窗口里终端页的三处
（P13-ax）已经是 `Float` 面 + 受光边 + 两层影的 E2 浮层。**一个产品里两种提示**是这轮
要收的那条缝：它不在 z 阶梯上，深色档因为本仓没设 `ui.Theme.Inverse` 而退回 `Text`
（`ui/theme.go:71-80`），也就是**一枚近白的片子贴在深玻璃上**。

## 为什么现在做

- P13-ax 结束时把 20-2/20-3 写成了「机械迁移，不需要拍板」，这是它的下一格。
- §6 差距表没有 `未开始` 行，通用队列里「状态反馈」这一半（悬停/按下/禁用）已收口，
  提示是**同一套浮层语汇里最后一块没统一的材质**。

## 逐条落地项（行号 2026-10-31 现场核对）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/launcher.go:1882` `rowControl`（行尾 20u 控制格的 `Label(label).Tooltip(label)`） | 框架平涂 | `tokens.Tip(c, e, label)` |
| 2 | `internal/launcher/launcher.go:2615` 冲突警示点（`Label(copy.CommandConflict).Tooltip(copy.CommandConflict)`） | 同 | 同 |
| 3 | `internal/launcher/launcher.go:2627` 空格键帽（`key.Tooltip(copy.DirectOutputHint)`） | 同 | 同 |
| 4 | `internal/launcher/launcher.go:2636` 来源词（`…Tooltip(item.Source)`） | 同 | 同 |
| 5 | `internal/launcher/launcher.go:2688` 历史行星标（`Label(starLabel).Tooltip(starLabel)`） | 同 | 同 |
| 6 | `internal/launcher/launcher.go:3449` `fieldSettingsButton`（`Label(label).Tooltip(hint)`） | 同 | 同 |
| 7 | `internal/launcher/output.go:516` 捕获输出页头两枚 28u 方键（`Label(label).Tooltip(hint)`） | 同 | 同 |

链式写法要拆开：`Tip` 是 `Tokens.Tip(c, anchor, text) ui.Element`，接在最后一个
`Tooltip` 的位置上做赋值（`e = tokens.Tip(c, e, hint)`），不改任何 `Label`/`Clicked`/`Size`。

## 顺带要量的一条：`Tip` 是否吃掉了锚点间距

框架的 `Tooltip` 自己写了 `AttachTo(e, AnchorTop, AnchorBottom).Margin(0, 0, Space(1.5), 0)`
（`ui/tooltip.go:132`），而 `theme.Tokens.Tip`（`internal/theme/theme.go:1281-1297`）
只用了 `TooltipBase` 的默认 attach、**没有 margin**。若 `Margin` 就是那 1.5u 的缝，
则 P13-ax 落地的三处提示现在是**贴住锚点**的（浮面贴住锚点读成徽章，不是浮层），
这一轮一并补上并在 pin 里量出来。

**先量再改**：用同一枚锚点分别戴框架 `Tooltip` 与 `Tip`，量「锚点顶 − 提示面底」的像素差。

## 行为契约零改动红线

- 0.6s 静息 / 0.4s 热接力 / Esc 关闭 / `Description` 无障碍描述 / 80u 上限 / 锚点与间距：
  全部沿用框架的 `TooltipBase`，只换材质。
- 文案（i18n）、`Label`、命中区域、行高链、键盘模型一个字节不动。
- 提示是浮层：**不叠玻璃、不自造透明**（§1 禁令）；`Float` 是唯一的面，边只有一圈发丝 + 一条冠线。

## 门槛命令与基线（本轮实测）

```
gofmt -l cmd internal                                   # 必须空
go vet ./... / GOOS=windows,darwin go vet ./...          # 必须净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 只许涨
GOOS=linux/windows/darwin go build ./...                 # 必须过
```

基线（本机实测，改动前）：PASS **3103** / FAIL 0 / SKIP 102 / 测试函数 **2289**。

## mutation 计划（每条对应哪个 pin 变红）

1. `tokens.Tip` 的面退回 `t.Theme.Text`（框架的反色片子）→ 材质 pin 报
   「the tip's face is <近白> … that is the inverse chip」。
2. 删掉 `tip.InsetShadow(0, 1, 0, 0, t.EdgeHighlight)` → 深色冠线 pin 变红
   （顶行低 4 级以上）。
3. 删掉 `AttachTo(...).Margin(0, 0, Space(1.5), 0)` → 间距 pin 变红（缝 6 → 0）。
4. 把其中一处调用改回 `.Tooltip(...)` → census pin 变红（七处必须逐处点名）。
