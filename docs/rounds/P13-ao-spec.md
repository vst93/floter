# P13-ao 规格 · 终端状态灯：一盏灯只有「亮」与「暗」，而它有三个状态、只有一档墨

`docs/liquid-glass.md` §6 无 `未开始` 行，按纪律走通用队列的「状态反馈」这一格
（P13-an「仍待 3」把「`terminalui.statusDot` 的墨在两套外观下的级数」列成下一轮候选）。
一个表面一件事：**终端浮条左上角那一枚 1.75u 的灯**。

## 意图（一句话）

把「有程序在跑 / 没有程序在跑」从**动效**搬到**形**上：实心 = 亮，空心环 = 暗。
现在这两件事只靠呼吸和 0.30 的 alpha 差说，而呼吸自己会走到底下那一档里去。

## 为什么现在做（实测证据，非猜测）

`internal/terminalui/terminalui.go:1634` `statusDot` 现在只有两个分支：

| 分支 | 位置 | 画的是什么 |
|---|---|---|
| 「结束了」 | `terminalui.go:1644` `if a.exited()` | `dot.Background(inks.Tertiary).Opacity(0.7)` —— 实心点，`TextTertiary` @0.7，静止 |
| 其余（含**空白页**） | `terminalui.go:1660` 起 | 同一个 `TextTertiary`，实心，靠 `Draw` 里的呼吸改 alpha/尺寸 |

三条由此量出来的账：

1. **空白页说了假话。** `internal/terminalui/terminalui.go:906` 的空白页也画浮条，注释写着
   「its dot says the page holds no live session」；但 `exited()`（`terminalui.go:1726`）在
   `a.Current() == nil` 时返回 `false`，于是**一个会话都没有的页面上，灯按「活着」在呼吸**。
2. **两个状态的范围重叠。** 呼吸的 alpha 是 `1-0.5*breath`（`terminalui.go:1712`），即
   **1.00 ↔ 0.50**；结束态固定在 **0.70**。于是每个 2.4s 周期里约 **40%** 的时间，活着的点
   比「已经结束」的点还暗——静帧（截图、`ReduceMotion` 之外的中途帧）分辨不出谁是谁。
3. **alpha 差本身不够。** 0.30 的 alpha 差在深色下面约 33 级（`TextTertiary` ≈ 110,110,118，
   P8-e 报告实测），浅色下面同样约 33 级——够「看得出不一样」，不够「一眼知道是哪个状态」，
   而且方向还随着呼吸翻面。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `terminalui.go:1634` `statusDot` 的「结束了」臂 | 实心圆盘 `Background(Tertiary).Opacity(0.7)` | **空心环**：`dot.Draw` 里 `p.Stroke(r, inks.Tertiary, r.W/2, t.Space(0.25))`（1 DIP，`Painter.Stroke` 画在盒子内缘，盒子仍是 7 DIP，不动布局） |
| 2 | 同上，空白页那一臂 | 落到「活着」那支，呼吸 | 与「结束了」同一支：新增 `a.live()`（`session != nil && !a.exited()`），`!a.live()` 走空心环 |
| 3 | `terminalui.go:1634` 的文档注释 | 「a still grey dot once the program has ended」 | 写成形差：实心=有程序在跑（呼吸是加固），空心环=没有（从未开始 / 已结束 / 已退出） |
| 4 | `barchrome_test.go:150` `TestTheTerminalBarDotDropsItsHaloAndWearsTertiary` 的 held 臂 | 断言**中心**像素 = `Tertiary`@0.7 | **S**：断言中心 = 底色（空心）、环上像素 = `Tertiary` 满档；「无光晕」那条（盒宽与 live 相同、环外两列是底色）保留 |
| 5 | `accentguard_test.go:125` `TestTheExitedStatusDotStaysAGreyDot` | 断言中心像素 = `Tertiary`@0.7 | **S**：色相守卫改成量**环上**的像素（仍是灰、仍是中性），中心另判为空心 |
| 6 | `docs/liquid-glass.md` §6.3 第 3-6 行 + 新增 §6.12 | 「不画受光边」 | 3-6 追加「形态另案 → §6.12」；§6.12 写这一轮的账（含「exited 的 exit code 不上灯」= `故意不做`，文字里已有 `· exit 3`） |

### 明确不做

- **不给灯加受光边**（§6.3 3-6 的旧判）：1.75u 的圆加冠线就是噪点。
- **不让灯的色相回答 exit code**：`identityTitle` 已经写 `· exit 3`（`terminalui.go:1569`），
  一个面只说一句话（§1 禁令 2）。
- **不改呼吸的周期/缓动/keyframe**（2400ms / 30fps / 0.5 / 0.88）：§5 的动效契约。
- **不改浮条的几何**：盒子仍是 `Space(1.75)` = 7 DIP，`Shrink(0)`，`PassThrough()`。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**一个字不动**。
本轮只改「暗」那一档的画法，以及 `exited()` 之外新增一个**只读**判定 `live()`。

## 门槛命令 + PASS 基线（本轮开工实测）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3050（P13-an），只许涨
GOOS=linux/windows/darwin go build ./...            # 三平台过
```

## mutation 计划（实跑，≥2 条）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | 空心环换回 `dot.Background(inks.Tertiary).Opacity(0.7)` | 新的 `TestTheEndedDotIsARing…`（中心必须是底色）与改过的 barchrome held 臂 |
| M2 | `!a.live()` 改回 `a.exited()`（空白页重新呼吸） | 新 pin 的「无会话 = 空心环」臂 |
| M3 | 环的 alpha 换成 0.7（`inks.Tertiary.Alpha(0.7)`） | 新 pin 的环上像素 = 满档那条 |
