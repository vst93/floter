# P13-eo 规格：浮层的动作胶囊是圆角矩形，而三处都写着「胶囊」

表面：**插件配置浮层（plugin sheet）**。合约新增 `docs/liquid-glass.md` §6.110。

## 意图

**一句话**：浮层里那枚会毁掉数据的动作胶囊，是整块面上唯一一枚**不是胶囊的胶囊**——
它的四角是 8px 的圆角矩形，而参考的 CSS、它自己的注释、以及守它的那条 pin 都写着
「999px / pill」。

## 为什么现在做

上一轮（P13-en）的「仍待」第 1 条把下一格指到「浮层的层级」。先量：浮层的分组标题
（`Title` 12.6 / 620 / `TextStrong`）与它下面的字段标签（`Emphasis` 11.7 / 580 /
`TextStrong`）只差 0.9pt，而**设置页的卡标题与行标签是同一个关系**（`rows.go:187`
`sectionHeading` 对 `rows.go:1252` `rowTitle`）——那不是浮层的病，是这一代字阶本身；
只改浮层会让两个表面说两种话，属跨表面，记入 `需拍板`。

于是按纪律量同一批帧里另一格（状态反馈 / 形状一致性），它倒了。

## 现状（改前实测）

`pluginSheetFrame` / 单独渲染的探针（`ui.Tester`，400×200，黑底），动作胶囊的盒
**28px 高**，顶行起逐行的左沿内缩：

| 面 | 盒高 | row1 | row2 | row3 | row4 |
|---|---|---|---|---|---|
| 浮层动作胶囊（中性臂） | 28 | **5** | 4 | 3 | 2 |
| 浮层动作胶囊（危险臂） | 28 | **5** | 3 | 2 | 1 |
| 扩展页动作胶囊（`actionButton`） | 30 | **11** | 9 | 7 | 6 |

浮层那枚的曲线是 `RadiusSM`(8) 的圆角矩形；扩展页那枚是 `Radius(t.Space(3.75))` = 半高，
即胶囊。三处证据说浮层这一枚**应该**是胶囊：

1. 参考 `main:src/styles/plugin-config.css:241` —— `border-radius: 999px`；
2. `internal/settingsui/pluginsheet.go:459` 自己的注释 —— 「a `999px` corner (half the
   28u height, so the shape is a pill)」；
3. 守它的 pin `pluginsheetface_test.go:104` —— `if inset := leftmost - box.X; inset < 5`
   后面那句是「want a pill's **wide curve**」。

**pin 在 5 上过**（阈值也是 5，余量 0）。一条「宽曲线」的判据卡在圆角矩形的读数上，
是这一族最典型的「错着也绿」。

`docs/raycast-design.md:326` 记着这是 P8-b 的**有意**改动（`Radius(t.Space(3.5))` →
`RadiusSM`(8)，理由「literal，不随 `--ui-scale`」）。这一代重定它，理由写在新条目里：
**胶囊的半径只能是半高，而半高跟着 `--ui-scale` 走**——把半径钉成字面 8，换一档界面大小
就从胶囊漂成圆角矩形（28u 高的盒在 `large` 档更高，8 不变）。参考的 `999px` 是「永远是
半高」的写法，本仓的等价物是 `t.Space(3.5)`（= `Space(7)` 的一半，随档缩放）。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| eo-1 | `pluginsheet.go` `sheetActionButton` 的 `Radius` | `tokens.RadiusSM`（8，字面，不随档） | `t.Space(3.5)`（半高，随档；与扩展页 `actionButton` 的 `t.Space(3.75)` 同一条算术） |
| eo-2 | `pluginsheetface_test.go` 的角判据 | `inset < 5`（在 5 上过，余量 0） | 重定基线到**胶囊的读数**：`inset < 8`，注释写 `P13-eo 重定基线：旧阈值 5 → 新阈值 8` |
| eo-3 | `docs/raycast-design.md:326` 那一行 | 记着 `RadiusSM`(8) 为已落地 | 顶部已声明由 `liquid-glass.md` 升代；这一行加一句「P13-eo 升代：8 → 半高」 |
| eo-4 | 盒的几何 | 28u 高 / 10u 侧 padding / 标签两端的缩进 | **一个字节不动**（半径只改四角的墨，不改布局） |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一律不动**：本轮只改
`Radius` 一个参数，它不进布局。

## 门槛与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                 → 空
go vet ./...                          → 净
GOOS=windows go vet ./...             → 净
GOOS=darwin  go vet ./...             → 净
go test -count=1 ./...                → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 基线 3348
测试函数数                             → 基线 2520
```

## mutation 计划（≥2 条）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `Radius` 退回 `tokens.RadiusSM` | 新 pin 的「胶囊曲线」判据（角内缩回到 5） |
| M2 | `Radius` 抬到 `t.Space(5)`（超过半高） | 新 pin 的「不许超过半高」判据（框架把半径夹到半高，读数不变即证明夹取生效；若夹取不生效则曲线变平） |
| M3 | 侧 padding `Space(2.5)` → `Space(4)` | 盒的几何 pin（标签缩进 / 盒宽），证明「只改角、不改盒」 |
