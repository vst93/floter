# P13-dj 规格：About 卡片第四行的名字，是全卡最重的一行字

选题来源：任务书「下一轮 = 细节打磨；下一格从状态反馈的另一半（禁用 / 只读面）或空态满态里挑，
**先量再改**」+ P13-di「仍待」第 1 条（About 页的只读面）。表面 = **设置页 → About 的那张身份卡**。

## 意图（一句话）

§6.11 把这张卡前三行改成「名字是旁注、值是内容」，**第四行（`Project` 链接）留在了旧语法上**：
它现在既是全卡唯一可点的一行，又是全卡字号最大、字重最重、墨最深的一行字 ——
名字列从 11px/500/`TextSecondary` 突然跳到 15px/580/`TextStrong`，卡片的左列不再是一条列。

## 为什么现在做（证据，行号实测于本轮 HEAD `ef1e4ecd`）

| 位置 | 现状 |
|---|---|
| `internal/settingsui/settingsui.go:4200-4203` | `a.row(c, copy.AboutProject, "", func(){ ui.Link(...).FontSize(t.FontSize) })` |
| `internal/settingsui/rows.go:908-910` → `1127` → `1160` | `a.row` 走 `rowAligned` → `rowTitle` |
| `internal/settingsui/rows.go:1166` | `rowTitle` 的名字写死 `tokens.Emphasis` / 580 / `TextStrong` |
| `internal/settingsui/settingsui.go:4313-4329` | 同一张卡前三行已经改走 `cardRow`：名字 `tokens.Caption` / 500 / `TextSecondary` |

所以同一张卡、同一列、相邻两行，名字是两把不同的尺子。§6.11 的 14-5 当时写的是
「形态留一行，只换墨」—— 但只有前三行换了墨，第四行连墨都没换（它连 `keyValue` 都没走）。

**这不是「可点的那一行应该更重」**：§6.10 已经把「只读 / 可点」的分家交给**形**与**链接自己的色**
（`ui.Link` 的链接墨），不交给字阶。一个字阶在同一列里跳两级，读起来是「这一行是标题」，
而它其实只是这张卡第四行事实。

## 落成项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `settingsui.go:4200-4203` | `a.row(...)`，名字吃 `rowTitle` 的 emphasis 档 | 直接走 `cardRow`（同 `keyValue` / `factRow`）：名字 `tokens.Caption` / 500 / `TextSecondary` + `SingleLine().Ellipsis`，链接仍在**控制槽**里、仍由 `ui.Link` 画（链接自己的色号不变），字号保持 `t.FontSize` |
| 2 | 行盒 | 44px 地板 + 分隔线 + 1px 行距 + 两端对齐 | **一个像素都不动**（`cardRow` 框架原样） |
| 3 | 为什么链接不抬到 `tokens.Emphasis` | — | 链接的强调是**色**（`ui.Link`），不是字重；而 URL 是这一页最长的字符串，14 → 15px 会把控制槽再撑宽 7%，窄窗下先吃标签列（§6.11 的 14-8 已经在躲这件事）。**故意不抬** |
| 4 | 新像素 pin `internal/settingsui/aboutlinkink_test.go` | 这张卡的第四行没有墨账 | 四行的名字同档（名字墨差 ≤ 1 级、名字盒高相同、四行 X 同列）、第四行名字 ≤ 150 级、链接仍比名字重 ≥ 60 级、四行仍按 45px 递进 × 深浅两套，附 opt-in 出图 |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线全部零改动：
`Project` 这一行仍然只有 `ui.Link` 一个可点目标、`Label` 仍是 `copy.AboutProject`、链接 URL 不变。
视觉轮只改墨。

## 门槛命令与 PASS 基线（本轮实测，HEAD `ef1e4ecd`）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... && GOOS=darwin go vet ./...  # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 3312
```
基线：**3312 PASS / 0 FAIL / 149 SKIP / 2469 测试函数**（P13-di 收尾实测）。

## mutation 计划（≥2 条，实跑）

- **M1**：把第 1 项改回 `a.row(...)`（旧语法）→ `aboutlinkink_test.go` 的名字档断言必红
  （第四行名字回到 15px/580/`TextStrong`）。
- **M2**：名字改成 `tokens.Body` / 620 / `TextStrong`（「更粗一点」的另一种猜法）→ 名字 ≤150 级
  与「四行同档」两条红。
- **M3**：把链接字号改成 `tokens.Emphasis`（第 3 项的反面）→ 溢出/宽度断言红（若本轮落了它才成立，
  否则记「不适用」并说明）。
