# P13-de 规格：启动器的字号也是「阶梯上的档」，不是 body ± N

## 意图（一句话）

把 `internal/launcher` 里剩下 9 处「比正文小一档 / 大一档」的偏移量搬到字阶档位上，
并把 `internal/ci` 的字阶账从 `internal/settingsui` 一个包扩到**两个包**——
P13-dd「仍待」第 2 条的原话就是这一族。

**为什么现在做**：上一轮（P13-dd）在设置页收掉了 24 处同样的写法，并在 ci 里留了一条
AST 账；启动器是同一个病、同一个量法（四个界面步长上的行盒），且 `docs/liquid-glass.md`
§6.73 的 73-4 行还写着 `未开始`。**用户的目标是细节打磨**：偏移量不随界面步长缩放，
用户把「界面大小」调到最小一档（tiny 0.8）时同一块面上的字会同时脱离阶梯——这是
「同一句话在不同设置下是两个说法」，不是审美偏好。

证据（`ui.Tester`，参考字体，P13-dd 实测表）：

| 表达式 | tiny 0.8 | small 0.9（出货） | default 1.0 | large 1.1 |
|---|---|---|---|---|
| `t.FontSize-1`（行盒） | 13 | 15 | 17 | 19 |
| `tokens.Caption` | **14** | 15 | 17 | 19 |
| `t.FontSize+2`（行盒） | 18 | 19 | 22 | 23 |
| `tokens.Title` | **17** | 19 | 22 | 23 |

## 逐条落地项（行号=本轮动手前 `grep -n` 的真数）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| de-1 | `internal/launcher/filters.go:235` | 过滤行提示 `t.FontSize - 1` | `tokens.Caption` |
| de-2 | `internal/launcher/launcher.go:1303` | 窗口计费的行标题行盒 `Span{Size: t.FontSize + rowTitleStep}` | 具名助手 `rowTitleLineSize(tk)`（例外，见下） |
| de-3 | `internal/launcher/launcher.go:1306` | 副标题行盒 `Span{Size: t.FontSize}` | `Span{Size: tk.Body}`（同一档，名字化） |
| de-4 | `internal/launcher/launcher.go:1535` | 作用域标记的名字 `t.FontSize - 1` | `tokens.Caption` |
| de-5 | `internal/launcher/launcher.go:1633` | 字段尾部提示 `t.FontSize-1` | `tokens.Caption` |
| de-6 | `internal/launcher/launcher.go:2646` | 结果行标题 `t.FontSize + 2` | `tokens.Title` |
| de-7 | `internal/launcher/launcher.go:2743` | 删除确认句 `t.FontSize - 1` | `tokens.Caption` |
| de-8 | `internal/launcher/output.go:345` | 分页脚注的词 `t.FontSize - 1` | `tokens.Caption` |
| de-9 | `internal/launcher/output.go:695` | 捕获输出页状态条 `t.FontSize - 1` | `tokens.Caption` |
| de-10 | `internal/launcher/output.go:980` | 便条行副标题 `t.FontSize - 1` | `tokens.Caption` |
| de-11 | `internal/launcher/triggerhint.go:153` | 助记提示的**量算** `size := t.FontSize - 1`（量它画的那一档） | `a.tokens(c).Caption`（画与量必须同一个名字） |
| de-12 | `internal/ci/fontladder_contract_test.go` | 账只扫 `../settingsui` | 账改成「按包一张表」：`settingsui` + `launcher`，各自的例外名单 |

**为什么 de-2 / de-6 一起动**：结果行标题原来**画**在 `body+2`、**计费**在 `body+rowTitleStep`
（同一个 2）——两处都是偏移量。本轮把**画**搬到 `tokens.Title`，把**计费**收进一个具名
助手 `rowTitleLineSize`，于是「画的那一档」和「窗口按它收的那一档」是**同一个名字**。
计费这一处不能直接换成 `Title`：它是高度链，`Geometry.Height` 的兜底
（`height.go:252` 的 `font + rowTitleStep`）也在同一句话上，本轮**不动高度链的语义**，
只把它命名（这是 ci 账里唯一的启动器例外，理由写在例外表里）。

## 行为契约零改动红线（本轮自查）

- **高度链**：`rowUnits` / `launcherStatusUnits` / `actionBarUnits` / `FieldRowUnits` /
  `TopInsetUnits` / `TailUnits` **一个数都不改**；`Geometry.Height` 的算术不动。
  de-2 只把 `MeasureText` 的 size 从一个偏移量换成一个具名函数，函数体仍是
  `t.FontSize + rowTitleStep`。
- **键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线**：零改动（本轮只有字号与一条
  AST 账）。
- 出货档（0.9）上 `Caption` 与 `-1`、`Title` 与 `+2` 的行盒**逐值相同**（15 / 19），
  所以额度内的画面不换行、不多一行、不移动任何盒子；看得见的只有 0.1pt 的字宽。

## 门槛命令 + PASS 基线（本轮开工实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 3300
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

测试函数总数基线 **2453**（不许减少；本轮新增 2 条 pin + 1 条 opt-in 出图）。

## mutation 计划（每条说明会让哪个 pin 变红）

1. `filters.go` 的提示改回 `t.FontSize - 1` → ci 的
   `TestTheSurfacesTextSizesAreLadderRungs`（`../launcher/filters.go:…: the size is an
   expression ('t.FontSize - 1')`）变红。
2. `launcher.go` 的结果行标题改回 `t.FontSize + 2` → 同一条账变红（`…launcher.go:…`）。
3. 把 `rowTitleLineSize` 的调用点换成内联 `t.FontSize + rowTitleStep`（例外只收函数体）→
   账的 ① 臂变红（调用点写偏移）。
4. 像素臂：把 de-11 的 `a.tokens(c).Caption` 改回 `t.FontSize - 1` → 新 pin
   `TestTheNudgesMeasuredSizeIsTheSizeItIsDrawnAt` 在 tiny 档上变红（量 13、画 14）。
