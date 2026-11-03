# P13-fe · 启动器最后两处「body ± N」，与把字阶账扩到这个包

## 意图（一句话）

启动器里最后两处「正文的偏移量」（结果行标题 `t.FontSize + 2`、捕获输出页状态条
`t.FontSize - 1`）收口到阶梯档，并让 `internal/ci` 的字阶账**从设置页扩到启动器**——
从此这个包里再出现 `…FontSize ± …` 就红。

## 为什么现在做（证据）

`docs/liquid-glass.md` §6.74（P13-de）把启动器的九处逐处量过，落地 4 处，其余 5 处判
`未开始（取景）`，理由写死：「`render()` 不 `c.SetTheme(a.tokens(c).Theme)`，于是同一个
函数里 `t := c.Theme()` 是框架库存的 12px、`a.tokens(c)` 是设计阶梯的 10.8，夹具在量一个
生产上不存在的混排界面。**先收口取景，再搬这 5 处，再把 `internal/ci` 的字阶账从
`internal/settingsui` 扩到 `internal/launcher`**」。

取景那一半已经由 P13-ec（`render()`）、P13-ed（`renderIn`）、P13-ee（`renderAt`）收口
（§6.98 / §6.99 / §6.100，三族全部 `已落地`），所以 §6.74 的前置条件**今天成立**。复核
剩下的 5 行：

| # | 位置 | 现状 | 本轮 |
|---|---|---|---|
| de-5 / de-11 | 助记提示的绘制与量算 | 已在 P13-eb（§6.86）落地为 `tokens.Caption` | 表已过期，本轮只更新状态 |
| de-2 | `launcher.go` 行盒测量（标题行） | `MeasureText(… t.FontSize + rowTitleStep)` | 改成 `tk.Title` |
| de-3 | `launcher.go` 行盒测量（副标题行） | `MeasureText(… t.FontSize)` | 改成 `tk.Body`（同值，只为名字一致） |
| de-6 | `launcher.go` 结果行标题的绘制 | `FontSize(t.FontSize + 2)` | 改成 `tokens.Title` |
| de-9 | `output.go` 捕获输出页状态条 | `FontSize(t.FontSize - 1)` | 改成 `tokens.Caption` |

实测两把寄存器在四个步长上的行盒（`internal/ci/fontladder_contract_test.go` 的表，
参考字体）：`+2` → 18 / 19 / 22 / 23，`title` → 17 / 19 / 22 / 23；`-1` → 13 / 15 / 17 / 19，
`caption` → 14 / 15 / 17 / 19。**出货档（0.9）两者重合**，所以这是 tiny 档才看得见的
一致性；用户把「界面大小」调到最小一格时，结果行的标题与输出页的状态条此前是这一页
仅有的两件**不跟步长走**的东西。

## 逐条落地项

1. `internal/launcher/launcher.go`（`View` 里量行盒那一段，当前 `:1303-1311`）：
   `c.MeasureText(0, ui.Span{Text: "Ag", Size: t.FontSize + rowTitleStep})` →
   `Size: tk.Title`；`Size: t.FontSize` → `Size: tk.Body`。`tk` 已在上面取好（`tk := a.tokens(c)`）。
   两个 `a.TitleLine` / `a.SubtitleLine` 的**语义不变**（还是「画出来的那一行的盒」），
   只是取的名字从偏移量换成阶梯档。
2. `internal/launcher/launcher.go`（结果行标题，当前 `:2659`）：`FontSize(t.FontSize + 2)`
   → `FontSize(tokens.Title)`（该作用域里 `tokens := a.tokens(c)` 已存在）。
3. `internal/launcher/output.go`（状态条，当前 `:696`）：`FontSize(t.FontSize - 1)`
   → `FontSize(tokens.Caption)`。
4. `internal/launcher/height.go`（`titleLine = font + rowTitleStep` 的回退）**不动**：它是
   「还没量过时的地板」，不是画出来的寄存器；补一句注释说明它为什么不进阶梯。
5. `internal/ci/fontladder_contract_test.go`：把单包账改成**按包的表**（`settingsui` 30 处
   下限 + 两条具名例外；`launcher` 10 处下限、零例外）。判据 ①②③ 一字不改。
6. `docs/liquid-glass.md`：§6.74 的 5 行状态改成 `已落地（P13-fe / P13-eb）`，新增 **§6.126**
   记本轮；§6 差距表仍无 `未开始` 行。

## 行为契约零改动红线

高度链（`Geometry.Height` / `RowCount` / `rowTitleStep` 的回退）、键盘模型、会话管理、
设置写回、i18n 文案、命令接线：**一字节不动**。本轮只换「字号取自哪个名字」。

## 门槛命令 + PASS 基线（实测，HEAD `33775080`）

```
gofmt -l cmd internal                      # 空
go vet ./...                               # 净
GOOS=windows go vet ./...                  # 净
GOOS=darwin  go vet ./...                  # 净
go test -count=1 ./...                     # 3396 PASS / 0 FAIL / 187 SKIP
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3396
三平台 build                                # 过
```

测试函数 2563 → 预期 **2565**（+2：`TestTheLaunchersTextSizesAreLadderRungs` 扩包后的
两条包级子测试仍算一条函数；新增 `TestTheResultRowsTitleIsTheLaddersTitle` 与
`TestTheOutputStatusStripIsACaption`）。

## mutation 计划（≥2，逐条说明会让哪个 pin 变红）

- **M1**：把 `output.go` 的状态条退回 `t.FontSize - 1` → `internal/ci` 的账（launcher 包）
  必须红，报错串里带 `output.go:NNN` 与 `t.FontSize - 1`；同时
  `TestTheOutputStatusStripIsACaption`（tiny 档量画出来的盒）必须红。
- **M2**：把结果行标题退回 `t.FontSize + 2` → 同上两条（ci 账 + `TestTheResultRowsTitleIsTheLaddersTitle`），
  且 `TestTheLauncherHeightsStayOnTheLadder` 在 tiny 档会多出 18 那一档（若该 pin 只在出货档跑则绿，
  报告里写明它的覆盖边界）。
- **M3（等价写法，预期全绿）**：`FontSize(tokens.Title)` → `FontSize(tk.Title)`（同一档的两个
  名字）→ 账与 pin 都绿，用来证明 pin 钉的是**档**不是字面量。
