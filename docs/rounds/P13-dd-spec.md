# P13-dd 规格：设置页的字号是「body ± N」，不是阶梯上的档

## 意图

设置包里每一处「比正文小一点 / 大一点」的字，都是拿 `t.FontSize` 加一个**常数偏移**
写出来的（`t.FontSize-1`、`+1`、`+2`、`-3`）。偏移量不随「界面大小」那一步缩放，阶梯会，
所以同一块面上的两级字会随步长越走越近。本轮把它们全部换成阶梯上的档位，并把
`internal/ci` 的字阶账从 `pluginsheet.go` 一个文件扩到**整个 `internal/settingsui` 包**。

为什么现在做：这是 P13-dc「仍待」第 1 条（唯一不需要拍板的那一格），也是
`docs/liquid-glass.md` §6.6x 那一族的最后一段——浮层已经换完了，设置页的正文面还没。

## 先量（本轮实测，不是旧报告的数）

探针：`internal/settingsui/zzprobe`（临时，出完数即删）在四个界面步长上量同一个
`Text` 的行盒（参考字体，`ui.Tester`）：

| 表达式 | tiny 0.8 | small 0.9（出货） | default 1.0 | large 1.1 |
|---|---|---|---|---|
| `t.FontSize`（body） | 9.60 / **15** | 10.80 / **17** | 12.00 / **18** | 13.20 / **20** |
| `t.FontSize-1` | 8.60 / **13** | 9.80 / 15 | 11.00 / 17 | 12.20 / 19 |
| `tokens.Caption` | 8.80 / **14** | 9.90 / 15 | 11.00 / 17 | 12.10 / 19 |
| `t.FontSize+1` | 10.60 / **17** | 11.80 / 18 | 13.00 / 20 | 14.20 / 22 |
| `tokens.Emphasis` | 10.40 / **16** | 11.70 / 18 | 13.00 / 20 | 14.30 / 22 |
| `t.FontSize+2` | 11.60 / **18** | 12.80 / 19 | 14.00 / 22 | 15.20 / 23 |
| `tokens.Title` | 11.20 / **17** | 12.60 / 19 | 14.00 / 22 | 15.40 / 23 |
| `t.FontSize-3` | 6.60 / 10 | 7.80 / 13 | 9.00 / 14 | 10.20 / 15 |
| `t.FontSize-0.5` | 9.10 / 14 | 10.30 / 15 | 11.50 / 18 | 12.70 / 19 |

**结论一**：在出货档（small）上三者行盒**完全相同**，所以本轮在出货档上「零行盒变化」，
差的只有 0.1 点的字宽——这正是「换语言」该有的代价。
**结论二**：在 tiny 档上偏移量与阶梯**分家**：hint 行 13 对 caption 的 14、次级标题 17 对
emphasis 的 16、区块标题 18 对 title 的 17。用户把界面调小一步，设置页的**三级字同时
脱离阶梯**——这是本轮真正修的东西。
**结论三**：`-3`（`●`/`○` 状态盘）与 `-0.5`（卸载对话框的 10.5px 提示行）**不在阶梯上**，
没有档位可换，保留为具名例外（见下）。

## 落地

| 位置 | 现状 | 应改成 |
|---|---|---|
| `rows.go:181` | `.FontSize(t.FontSize + 2)` | `tokens.Title` |
| `rows.go:1166` | `+1` | `tokens.Emphasis` |
| `rows.go:1263` | `+1` | `tokens.Emphasis` |
| `rows.go:1588` | `+1` | `tokens.Emphasis` |
| `settingsui.go:1192` | `+2` | `tokens.Title` |
| `settingsui.go:1331/1333` | `-1`（层级名 + 说明） | `tokens.Caption` |
| `settingsui.go:1349` | `-3`（`●`/`○`） | `markDot(t)`（具名例外） |
| `settingsui.go:1809` | `c.Theme().FontSize - 1` | `a.tokens(c).Caption` |
| `settingsui.go:2093/2095` | `-1` | `tokens.Caption` |
| `settingsui.go:2701` | `+1`（集成名） | `tokens.Emphasis` |
| `settingsui.go:2787` | `-1`（失败原因） | `tokens.Caption` |
| `settingsui.go:2792` | `-3`（`●`） | `markDot(t)` |
| `settingsui.go:4308` | `+1`（键值/值） | `tokens.Emphasis` |
| `settingsui.go:4707` | `+1`（滑杆标签） | `tokens.Emphasis` |
| `settingsui.go:4981` | `-1` | `tokens.Caption` |
| `settingsui.go:5550` | `c.Theme().FontSize - 1` | `a.tokens(c).Caption` |
| `settingsui.go:6780` | `+1`（Run 表单标题） | `tokens.Emphasis` |
| `settingsui.go:6782/6844/6849/6873` | `-1` | `tokens.Caption` |
| `settingsui.go:6803` | `theme.FontSize-1`（`Danger`） | `tokens.Caption` |
| `settingsui.go:7230` | `-1`（`Warning` 行） | `tokens.Caption` |
| `transfer.go:104` | `-1` | `tokens.Caption` |
| `uninstall.go:395` | `t.FontSize - 0.5` | 保留（具名例外 `uninstallSubBody`） |

新帮助函数 `markDot(t *ui.Theme) float32`（`settingsui.go`，与 `uninstallSubBody` 相邻）：
`●` 是**字形**不是文字档位，它要装进一行 caption，所以是 caption 再往下；写明理由。

## 新账（`internal/ci/fontladder_contract_test.go`）

把 P13-dc 的一条账从 `pluginsheet.go` 扩成**整个包**（`../settingsui/*.go`，排除 `_test.go`）：

- 任何 `FontSize(x)` 的 `x` 只许是：`tokens.<档>`（或任何 `.<档>` 选择子，档 ∈ caption/
  body/emphasis/title/display）、`<ident>.FontSize`（这个面自己的正文档）、或**具名例外**
  `markDot(...)` / `uninstallSubBody(...)`（两个函数自带「为什么不在阶梯上」的注释）；
- 算式（`BinaryExpr`）一律红——这就是这条账要防的东西；
- 账不许空转：全文至少 30 处 `FontSize` 才算量到东西（现有 40+ 处）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一律不动；本轮只改**字号**
（以及由字号带出的行盒，已实测在出货档上为零）。

## 门槛与 PASS 基线

基线（P13-dc 实测，HEAD `e1c5ef72`）：PASS **3308** / FAIL 0 / SKIP 145 / 顶层测试函数 **2454**。
门槛照旧：gofmt 空、三平台 vet 净、`go test -count=1 ./...` 全绿、PASS 计数只涨、
三平台 build 过。

## mutation 计划（≥2 条，实跑）

- **M1**：把 `rows.go:181` 的 `tokens.Title` 退回 `t.FontSize + 2` → 期望新账在
  `rows.go:181` 报「the size is an expression」。
- **M2**：把 `settingsui.go:7230` 的 `tokens.Caption` 换成 `tokens.Caption + 1` → 同一个账报红。
- **M3**（过冲臂）：把 `markDot` 的 `-3` 改成 `-1`，并把 `ui.Text(c, mark)` 改成吃阶梯档
  —— 验证「例外必须仍然只是例外」，而不是账的漏洞。
