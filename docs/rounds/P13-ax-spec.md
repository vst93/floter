# P13-ax 规格：终端页的提示框戴上浮面（E2）

## 意图（一句话）
全仓最后一枚「平涂 + 糊影 + 没有边」的浮层——框架的 `Element.Tooltip`——在终端页先换成 §2 的 E2 浮：
内容在它下面，它压在其他内容上，所以它必须读成一张浮起来的纸，而不是一枚白片子。

## 为什么现在做
`docs/liquid-glass.md` §6 无 `未开始` 行，按纪律回通用队列的「状态反馈 / 浮层材质」。
证据（改前）：`ui/tooltip.go:99-105` 给的是 `Radius(Space(1.25))`（5px）、`Background(t.inverse())`
的**不透明**面、一层 `Shadow(0, 2, 8, 0, 0.2)`、零条边；本仓 `internal/theme/theme.go` 从不设
`ui.Theme.Inverse`，于是深色档退回 `Text`（近白 ≈229）——深色玻璃上的一枚白片子。
终端页两处调用：`terminalui.go` `barButton` 的 `button.Tooltip(hint)` 与会话标题上的
`Tooltip(identityTitle)`（后者是**读被截断的会话全名的唯一位置**，P13-ae 定的出口）。

## 落地项
| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/theme/theme.go`（新增 `Tokens.Tip`） | 框架 `Element.Tooltip` 的外观 | `Float` 面 + `RadiusSM` 8 + `InputStroke` 发丝 + 顶边 `EdgeHighlight` + 两层影（`0 6px 18px 0.30` / `0 1px 2px 0.20`，`--elev-3-compact` 的 scale 1） |
| `terminalui.go` `barButton` | `button.Tooltip(hint)` | `tokens.Tip(c, button, hint)` |
| `terminalui.go` 会话标题 | 链尾 `.Tooltip(identityTitle)` | `tokensOf(a, c).Tip(c, ui.Text(...).DragWindow(), identityTitle)` |

## 零改动红线
0.6s 停留 / 0.4s 热接力 / Esc / 锚点与 1.5u 间距 / 80u 上限 / `Description` 无障碍描述 /
键盘模型 / 高度链 / 会话管理 / 设置写回 / i18n 文案 / 命令接线——一条不动。

## 门槛（PASS 基线用 P13-aw 实测 3100）
gofmt 空、三平台 vet 净、三平台 build 过、`go test ./...` 全绿、PASS 计数只涨不跌。

## mutation 计划
1. `Background(t.Float)` → `Background(t.Theme.Text)`（退回框架的反色片子）→ 面像素数那条红。
2. 删掉 `tip.InsetShadow(0, 1, 0, 0, t.EdgeHighlight)` → 深色冠线那条红。
