# P13-bs · 规格：行的文字原点——同一张卡片里的两条线，并回一条

> 合约 `docs/liquid-glass.md` §6.37（新）｜队列：通用队列「间距节奏 / 层级」（§6 无 `未开始` 行）
> 产品改动 2 处：`internal/settingsui/rows.go` 的 `cardRow`、`internal/settingsui/settingsui.go` 的 `keyValue`

## 一、意图

**一句话**：一张卡片里所有行的第一行文字必须从同一个 y 起——现在 Terminal 卡的第一行比它
下面两行低 5.9px，比别的卡片的第一行也低 5.9px，读起来那一行是「挂在半空」的。

**为什么现在做**：P13-br 的「仍待」第 1 条留下的量：`Terminal 组头→首行文字 22px`，
`Window / Menu bar 16px`，来源是首行控件类型不同（`choose` vs `checkbox`）。那一轮要求
**先量再改**，本轮就是那次量。

## 二、先量（改前，General 页 `ui.Tester` 720×2000，深/浅**完全同值**）

| 组 | 组头底 → 首行标签顶 | 组盒高 | 组头→首卡 |
|---|---|---|---|
| Window（首行 = `checkbox`，有提示） | **16.3** | 240.0 | 6.7 |
| Menu bar（`checkbox` + 提示） | **16.3** | 170.4 | 6.9 |
| **Terminal（首行 = `choose`，无提示）** | **22.2** | 355.0 | 6.7 |
| Material（`glassPick`，有提示） | **16.3** | 380.0 | 6.9 |

**根因不是分组间距，是行内容盒的居中**（`rows.go` `cardRow` 的内层 `Row.AlignItems(align)`）：

- 有提示的行：文字列高 = 标签 18 + 行距 + 提示 ≈ **36.6px**，比控件高 → 行高由文字列决定，
  居中是恒等变换 → 标签顶 = 行内边距 `Space(2.25)` = **8.1px**（实测 9.6，含字形上沿 1.5）。
- 没有提示的行：文字列只有 **18px**，控件（`ui.Select` 触发框）**33px** → 行高由控件决定，
  18px 的块在 33px 的盒里居中 → 标签顶 = 8.1 + (33−18)/2 = **15.6px**（实测 15.5）。

同一个 Terminal 卡里，第 2、3 行是 `stackedRow`（标签在自己那一列的顶边，8.1px），
于是**一张卡片里出现两条文字原点线**（15.6 与 8.1，差 7.5px；相对组头 22.2 与 16.3，差 5.9px）。

## 三、落地项

| # | 位置 | 现状 → 落成 |
|---|---|---|
| 1 | `rows.go:1376` `cardRow` 内层 `Row` 的 `AlignItems(align)` | → **`AlignItems(ui.Stretch)`**：文字列撑满行的内容盒，标签从内容盒顶边起（8.1px）。有提示的行不变（文字列本来就是最高那个），没有提示的行标签上移 5～7.5px 回到同一条原点线 |
| 2 | `rows.go:1383` 前导记号（`lead`） | 图标不参与拉伸（框架 `stretches()` 明确跳过 `kindIcon`），在 `Stretch` 下会掉到行顶 → **包一层 `Row.AlignItems(align)`**，记号保持自己的中线 |
| 3 | `rows.go:1396` 控件槽 `AlignItems(ui.Center)` | → **`AlignItems(align)`**。两个实参除 `settingsui.go:2277`（`ui.Start` 且**没有控件**）外全是 `ui.Center`，所以像素零变化；写 `align` 是说清「控件在哪条轴上对齐由调用方定」 |
| 4 | `settingsui.go:4171` `keyValue`（About 页只读事实） | 这一行把名字当**旁注**用（名字与右边的值读成一行，P13-an 的 pin 就是「两条墨带重叠 ≥ 60%」）。文字列现在从顶边起会把旁注顶上去 → 名字包一层 **`Row.FillHeight().AlignItems(ui.Center)`**，让自己在行里居中，与值保持同一光学线 |
| 5 | `internal/settingsui/roworigin_test.go`（新，1 条 pin + 1 条 opt-in dump） | 四个组（Appearance 的标签与自己组头同名，排除）的「组头底 → 首行标签顶」必须同值，spread ≤ 1.5px（渲染器亚像素吸附 6.7/7.7 的容差） |

改后实测（深/浅同值）：Window 16.2999 / Menu bar 16.2999 / **Terminal 16.2999** / Material 16.2999，
**spread 0.0001px**；五个组盒高 **一个像素都没变**（297.2 / 240.0 / 170.4 / 355.0 / 380.0）——
本轮只挪了一张卡片里的一行字，没有动任何行高、没有动分组间距。

## 四、行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字都没碰**。
`cardRow` 的签名与 `align` 语义保留（控件仍按调用方的轴对齐），只改文字列在行内的起点。

## 五、门槛与 PASS 基线（本轮实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows/darwin go vet   → 净
go test -count=1 ./...       → 全绿（0 FAIL）
--- PASS                     → 3210（基线 3209，+1）
--- SKIP                     → 117（基线 116，+1 = 本轮 opt-in dump）
测试函数                     → 2365（HEAD 2363，+2）
三平台 go build ./...        → 过
```

## 六、mutation 计划

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | `cardRow` 的 `AlignItems(ui.Stretch)` → `AlignItems(ui.Center)`（回到旧行为） | `TestTheFirstRowStartsWhereEveryOtherGroupDoes`：Terminal 22.2 对 16.3，spread 5.9px |
| M2 | `keyValue` 去掉 `FillHeight()`（旁注不再居中） | `TestTheAboutFactsPutTheValueInTheEmphasisRung`：名字与值的墨带重叠 1/3/6 行 → 「reads as two lines」 |
