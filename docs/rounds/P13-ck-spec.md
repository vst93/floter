# P13-ck 规格 · 分页脚注：拉取结束的那一帧，词横移 7.5px

> 选题：P13-cj「仍待」第 1 条（通用队列 `状态反馈`）。合约新增 §6.54。
> 前一句是「先量」——量完，那一轮猜的两件事都不成立（见下「量过、否掉」），
> 真正立得住的是第三条：**这一行的两种状态是两套内容，而这一行是居中布局**。

## 意图

捕获输出页的分页脚注（`internal/launcher/output.go:326` `outputFooter`）有两档会画字：
「在飞」（环 + `正在加载…`）与「到底了」（只有 `没有更多结果`）。两档的**内容宽度不同**，
而行是 `Justify(ui.Center)`——于是拉取结束的那一帧，整行内容重新居中，词横移，环整个消失。
这一行是这一页的最后一条带，也是唯一一条**换状态时整行都在动**的带。

改成：环的**轨道两档都画**，活动象限只在「在飞」时画、并且只在「在飞」时申请帧。
一圈合上 = 到底了，一圈在转 = 还在拉。宽度不变，词因此几乎不动（残差见判据 2）。

## 为什么现在做

- 通用队列 `状态反馈` 的下一格；P13-bj 把这一行的**墨**收成了两档，但没管**位置**。
- 证据（实测，装 app 主题、与 `shell.View` 同一条生产路径，窗口 720×488）：

  | | 词盒 x | 环盒 | 行内容左沿 |
  |---|---|---|---|
  | 在飞 | **331.0** | 313–325，峰值 139（活动象限） | 313.0 |
  | 到底了 | **323.5** | 无 | 323.5 |
  | 差 | **7.5px** | 环整枚消失 | 10.5px |

## 落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `output.go:341`（`outputFooter` 的子元素） | `if state == footerLoading { a.footerSpinner(c) }` —— 只有「在飞」建环 | `if state == footerLoading \|\| state == footerEnd { a.footerSpinner(c, state == footerLoading) }` |
| `output.go:414`（`footerSpinner`） | 无参数，永远转：`motion.Value` + `AnimationFrameRate(60)` + 旋转 + 活动象限 | 收一个 `live bool`；`!live` 时只画 `hairline` 轨道（闭合圆），**不申请帧、不画象限、不旋转**，然后 `return` |
| `output.go:422` 起 | —— | 新增 `if !live { … settled … return }` 分支；`live` 分支逐字节不动 |

**不改**：墨（在飞 `tokens.TextSecondary` + 500 / 到底了 `t.TextMuted`）、字号
（`t.FontSize - 1`）、行高（`MinHeight(Space(5.5))` = 19.8px）、`Gap(Space(2))`、
`Padding`、环的尺寸（`Space(3)` = 12px）与描边（1.5px）、`footerMore` 的空行、
`footerWord` 的三臂、`outputFooterState` 的状态机。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**。
本轮只改「这一行画什么」，不新增、不删除任何元素盒（环的盒子本来就在，
只是从「只在一档存在」变成「两档都在」，宽度相同）。

## 量过、否掉（不做的理由）

1. **「行高被挤压」**：行带 468.2→488 = **19.8px**，正好是 `MinHeight(Space(5.5))`
   （`Space(5.5)` = 5.5 × 3.6 = 19.8）。换窗口高度 440 / 488 / 520 三次，行带恒为
   19.8 —— **不是挤压**，是它自己的地板。
2. **「这一页有两个地板，差 9 级」**：到底了 `TextMuted` 118 / 图例 `TextTertiary` 109
   （浅 9 级），深色 118 / 89（29 级）。看着像浅色的档差塌了，但合约 §6.29 与 §6.26
   是**有意**让图例坐在最低一档（它是 chrome，到底了是内容）。两处不在同一条带里，
   不需要互相分辨。**不改**。
3. **「在飞比结局还响」**：在飞 139 / 结局 137（差 2）。两者都是 `TextSecondary`，
   分在两条带上（页脚 y=470 / 条子 y=71），各是各自那一条带的上限。**不改**。

## 门槛（本轮基线，实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3257 → 3260
GOOS=linux/windows/darwin go build ./...
```

- 基线（HEAD `1f266f5b` 实测重记）：PASS **3257** / SKIP **129** / 测试函数 **2410**
  （上轮报告记 3258/2411，实测 HEAD 上各低 1）。
- 目标：PASS **3260**（+3 条 pin）/ SKIP **130**（+1 条 opt-in dump）/ 函数 **2414**。

## mutation 计划

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `if state == footerLoading \|\| state == footerEnd` 退回只 `footerLoading` | `TestThePagingFooterHoldsItsWordAcrossStates`（7.5px）+ `TestThePagingFooterKeepsTheRingWhenTheListEnds`（环峰值 0） |
| M2 | 静止分支也画活动象限（`TextSecondary` 四分之一） | `TestThePagingFooterKeepsTheRingWhenTheListEnds`（gap −21） |
| M3 | `footerSpinner(c, state == footerLoading)` 改成 `footerSpinner(c, false)` | `TestThePagingFooterStillTurnsWhileLoading`（22 vs 139）+ P13-bj 的 `TestThePagingFooterSpinnerSharesTheLiveWordInk` |
