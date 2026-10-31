# P13-an 规格：About 页的只读事实——同一条线上，名字是标题、值是内容

## 意图

`keyValue`（`internal/settingsui/settingsui.go:4106`）是全仓最后一族**只读事实行**：
About 卡片的 `框架` / `链接协议` / `设置文件`（`:3993-3995`）与工具编辑器里不可变的 `集成 ID`
（`:4890`）。它此前借用设置行的墨语义：名字 `rowTitle` 的 emphasis（`t.FontSize+1` / 580 /
`TextStrong`），值塞在右端控制槽里用**最轻**的 `t.TextMuted`。**这轮把墨对调**：名字是值的
标题（caption / 500 / `TextSecondary`），值占 emphasis（`+1pt` / 580 / `TextStrong`）。

## 为什么现在做（证据）

1. `docs/liquid-glass.md` §6 无 `未开始` 行 → 走通用队列「状态反馈 / 细节打磨」，本轮取它剩下的
   **只读**那一半。`§6.10`（P13-ak）已经把这条语义立在审批记录上，`13-5` 把
   `PermissionBoundary` / `tool.Path` / `updateRow` 记作「值已在 emphasis 档」——
   **`keyValue` 不在那份清单里，病情却一模一样**（P13-ak 报告「仍待 4」、P13-am「仍待 2②」
   都指到这一族）。
2. 这一页是**事实表**，不是设置列表：`框架 mygo 0.9`、`链接协议 floter`、
   `设置文件 /home/…/settings.json`，没有一行可以按；而「设置文件到底在哪」全仓只有这一处。
3. 实测（`ui.Tester`，760×1400，`inkSpan` = 盒子内 luma 跨度）：

   | | 名字 | 值 |
   |---|---|---|
   | light | **202** | **101** |
   | dark | **198** | **94** |

   方向是反的：小字说「它是什么」，大字说「这是什么」，差 −101 / −104。

## 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `settingsui.go:4106` `keyValue` | `a.row(c, label, "", func () { ui.Text(value).FontSize(t.FontSize).TextColor(t.TextMuted).Selectable() })`——名字走 `rowTitle` 的 emphasis，值在最轻档 | `a.cardRow(c, ui.Center, nil, rowFace{}, 值, 名字)`：名字 `tokens.Caption`/500/`TextSecondary` + `SingleLine()`，值 `t.FontSize+1`/580/`TextStrong` + `Selectable()`（保留可选中，用户要复制路径）；`cardRow` 的框架（44px 地板、分隔线、1px 行距、`rowInset`、左右两端）逐字沿用 |
| 同上，行高 | 45px（甲 18 / 值 17，中心对齐） | **不变**：名字 15px / 值 18px，行距仍 45，卡片不高一格（值 `Selectable()` 且不加 `SingleLine`，长路径的折行行为与旧版一致） |
| `internal/settingsui/keyvalueink_test.go`（新） | 无墨账 | 值比名字重 ≥60 级 / 名字 ≤150 级 / 两段墨的竖带重叠 ≥ 短者 60%（行 1 实测 8/9、行 3 12/12）/ 值在名字右侧 / 三行按 45px 递进 × 深浅两套 + opt-in 出图 |
| `docs/liquid-glass.md` §6.11 | — | 14-1..14-7 台账（两条 `故意不做`：不叠成两行、不用 `ui.Baseline`） |

## 行为契约零改动红线

- **不动**：高度链、键盘模型、会话管理、设置写回、i18n 文案（一个字节都没改）、命令接线、
  `cardRow` 的任何数值（本轮只换两个字元素上的墨与字阶）。
- 值的 `Selectable()` 保留（它是对外承诺：设置路径可复制）；长路径仍会折行（旧版也没有
  `SingleLine`），因此**不变**。
- 唯一的行为面差异：`a.row` → `a.cardRow` 少了一层 `rowTitle`。`rowTitle` 里没有点击接线，
  `cardRow` 的 `names.Clicked()` 在两端都无人读（旧版 `a.row` 传的 `control` 也不是按钮）。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # ≥ 基线
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（开工时 HEAD `21e9e23b` 亲测）：PASS **3049** / FAIL 0 / SKIP **94** / 测试函数 **2255**。

## 变异计划（≥2 条，每条说明会让哪个 pin 变红）

- **M1**：`keyValue` 的值换回 `t.FontSize` / 400 / `tokens.TextMuted`（名字保持新档）
  → `TestTheAboutFactsPutTheValueInTheEmphasisRung` 的「值比名字重 ≥60 级」变红。
- **M2**：名字换回 `t.FontSize+1` / 580 / `tokens.TextStrong`
  → 同一条 pin 的「名字 ≤150 级」（且值不再重 60 级）变红。
- **M3**：值加 `SingleLine()` 之外的行高变化源——把值改成 `t.FontSize+4`
  → 行距 45px 的 pin（`pitch < 44 || pitch > 46`）应变红（若不变红如实记录，说明行高由地板决定）。
