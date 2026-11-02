# P13-ds 规格：状态带 `30u → 27`、编号族的视口上沿 `60 → 54`

## 意图

`§6.86` 把启动器的 21 条夹具债摊成六类，`§6.87` 交割了第一类里「字段行」那一族（`54u → 48.6`、
`56u − 1 → 49`、`60 → 54`）。本轮按同一手续交割**同一表面剩下的两处**：编号族与状态带。

一句话：**这两条 pin 一直量着 app 从不画的那个帧。** 它们读的是框架库存主题的 `Spacing`（4.0），
而 app 出货时画的是 3.6（`§6.87` 的 `designUnit()` = 0.9）。于是 30u 的带在夹具里是 30px、
在真机上 27px；编号族的视口上沿在夹具里 60、在真机上 54。pin 绿着，画面早就不一样了 ——
这正是「细节打磨」阶段最坏的一种绿：它让「改一个像素」和「改一整个刻度」都读不出差别。

## 证据（本轮实测，行号当场核过）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/launcher/numberband_test.go:46` | `render(t, a)`（库存尺帧 `frameHeight(10)` = 488） | `renderShipped(t, a, false)`（出货尺 `themedFrameHeight(10)` = 439） |
| `internal/launcher/numberband_test.go:91` | `ui.LightTheme().Space(fieldRowUnits + breathUnits)` = `4.0 × 15` = **60** | `spacing × 15`（`designMetrics()`）= **54** |
| `internal/launcher/numberband_test.go:159` | `render(t, a)` | `renderShipped(t, a, false)` |
| `internal/launcher/numberband_test.go:188` | `charged - up != 30`（「30u 状态带」写死成 30px） | `30 × designUnit()` = **27**，用 `tenths()` 比对 |
| `internal/launcher/numberband_test.go:309,351`（dump，opt-in） | `renderIn`（库存尺） | `renderShipped`，并把 PNG 落到 `/tmp/floter-P13-ds-*.png` |

产品的带高不是新数：`launcher.go:3349` 的 `FeedbackBand()` 就是 `c.Theme().Space(7.5)`，
出货 3.6 下 = 27；`height.go:44` 的 `launcherStatusUnits = 7.5`（= 30u）。本轮**不改产品**，
只把 pin 挪到它一直在画的那个刻度上。

## 落地项

1. `TestTheNumberedFamilyStartsOnARowYouCanRead`
   - 取景换 `renderShipped(t, a, false)`；
   - 视口上沿的期望从 `ui.LightTheme().Space(…)` 换成 `designMetrics()` 的 `spacing × 15`，
     读成 `tenths()` 整数比较（不加容差带）。
2. `TestHoldingTheModifierKeepsTheTerminalHint`
   - 取景换 `renderShipped(t, a, false)`；
   - 带的期望 = `tenths(30 × designUnit())` = 270（即 27.0px），报错串改成
     「want the reference's 30u status band (27.0 at the shipped step)」。
3. `TestNumberBandDump` / `TestATenRowListDump` 的取景换 `renderShipped`，PNG 路径带轮号
   （dump 是 opt-in，不计 PASS 基线）。
4. 注释：每条 pin 顶部加一行 `P13-ds 重定基线：旧值 → 新值`，写明旧值来自库存尺。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一概不碰**。
- 产品代码只改**一处、零像素、零行为**：`height.go` 里状态带的两处手写 `7.5` 改成
  `launcherStatusUnits`（同一个常量，同一值）。这不是新决定，是把一条**已经在漂的拷贝**
  收回去 —— 变异 M3 就撞上了它：把常量 7.5 改成 8.0，pin **不红**，因为计费用的
  是 `7.5 * spacing` 这个字面量，常量只喂了视图。改完 M3 立刻红（`29.0, want 27.0`）。

## 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                                   # 空
go vet ./...                                            # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 基线 3325，只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## mutation 计划（≥2 条，各自说明会让哪个 pin 变红）

- **M1**：把 `TestHoldingTheModifierKeepsTheTerminalHint` 的期望改回 30 → 若产品真按 3.6 画，
  报 `grew the window by 27, want the reference's 30u status band (30.0)`。这条证明「带在出货尺上是 27」。
- **M2**：把视口期望改回 `ui.LightTheme().Space(…)`（60）→ 报 `the family's viewport starts at
  y=54, want … 60`。这条证明编号族的视口沿确实随刻度走。
- **M3**（回归）：把 `launcherStatusUnits` 7.5 → 8.0 → 视图的带变 28.8、计费 29 →
  新 pin 报 `grew the window by 29.0, want the reference's 30u status band (27.0 …)`；
  还原后 `md5sum -c` 逐字节核对 + `touch` 复跑。
  **注意**：M3 在收回那两处 `7.5` 字面量**之前**是静默的（常量不喂计费）——那正是本轮顺手收掉
  的第二份拷贝。
