# P11-bd 规格 · 终端页上没有动作的浮层不该是命中目标

## 意图（一句话）

终端页浮在画布上的三样东西——常驻便条、失败提示、未揭示时的浮条按钮——都要把指针
**整块**让给画布：参考对它们写的都是 `pointer-events: none`，而本仓的
`PassThrough()` **不沿子树传播**，于是三处都只修了一半或一处没修。

为什么现在做：用户 2026-10-28 定的顺序是「界面质感与交互逻辑 + 用户报的 bug 优先」。
P11-bc 的「仍待」第 2 条自己写着 `feedbackNote` 是「同族另一半本轮故意没动」；本轮把
这一族一次收完，并且顺手查出 P11-bc 自己那条也只修了外层。

## 证据（本轮实测，行号是 HEAD `51e8838` + 本轮改动后的位置）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/terminalui/terminalui.go:1243` `feedbackNote` | `Absolute().Top(12).Left(12)` 的浮卡，**没有任何 `PassThrough`**。参考 `main:src/styles/terminal.css:414` 明写 `pointer-events: none`。它 4.5 秒里盖住前两行输出的左半边 | 行 + 字形 + 字都 `PassThrough()` |
| 同文件 `:1191` `residentNote` | P11-bc 给**行**加了 `PassThrough()`，但它里面的两句 `Text` 是独立命中目标（`Text` 可选中 = `flagSelectable`） | 两句 `Text` 各自 `PassThrough()` |
| 同文件 `:1324` `barButton` / `:1449` `actions.Opacity(0).PassThrough()` | 只标了 `actions` 那一行；按钮自己 `Focusable()` + `Label()`，字形在按钮里。框架的命中表不看透明度（`ui/runtime.go:1196` 只排除 `flagInvisible`/`flagInert`） | `barButton` 收一个 `live bool`：未揭示时按钮与字形 `PassThrough()`，也不挂 tooltip |
| `internal/terminalui/drawer.go:107` 的关闭钮 | 同上签名 | 传 `true`（抽屉里的钮永远是活的） |

参考原文（三条都是 `pointer-events: none`）：`.terminal-resident`（`terminal.css:303`）、
`.terminal-feedback`（`:414`）、`.terminal-bar:not(:hover):not(:focus-within)
.terminal-panel__actions`（`:501-505`，注释原话 “While hidden the buttons must not catch
clicks meant for the terminal underneath”）。

### 修前实测（这是本轮存在的理由）

同一个观测点：**点在那里之后，画布有没有拿到键盘**（画布是这一页唯一响应指针的东西）。

| 点击位置 | 修前 | 修后 |
|---|---|---|
| 画布空白 `(400,300)` | 画布拿到键盘 | 画布拿到键盘 |
| 失败提示自己的字 `(89,31)`（框 `{46 20 87 22}`） | **没拿到** | 拿到 |
| 失败提示的内边距 `(20,30)` | **没拿到** | 拿到 |
| 常驻便条的句子 `(322,582)`（框 `{179 571 287 22}`） | **没拿到** | 拿到 |
| 未揭示的关闭钮中心 `(834,14)` | **把终端关了**（`closed=1`） | 什么都没发生，画布拿到键盘 |

## 落地项（逐条）

1. `feedbackNote`：`notice` 行、`ui.Icon`、`ui.Text` 各加 `PassThrough()`。
2. `residentNote`：两句 `ui.Text` 各加 `PassThrough()`（行上 P11-bc 那条留着）。
3. `barButton(c, name, label, hint, live, run)`：`live == false` 时
   `button.PassThrough()` + `icon.PassThrough()`，且不挂 `.Tooltip(hint)`
   （`Focusable()` 保留——参考的 `:focus-within` 揭示就是靠键盘还能进得去）。
4. `titleBar` 的五个 `barButton` 调用传 `revealed`；`drawer.go` 的关闭钮传 `true`。
5. 既有测试里 5 处「不悬停就点浮条按钮」的夹具补一次 `Move` + `Frame`
   （`view_test.go` 4 处、`drawer_test.go` 1 处、`shell/appkeys_test.go` 1 处）：
   断言一个没被画出来的控件能被点到，正是本轮删掉的行为；断言本身一个字没放宽。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、**所有像素**（见下）
零改动。本轮只改命中标志。

## 门槛与 PASS 基线

```
gofmt -l cmd internal                          # 空
go vet ./...                                   # 净
GOOS=windows go vet ./...                      # 净
GOOS=darwin  go vet ./...                      # 净
go test -count=1 ./...                         # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2431 → 2435
GOOS=linux/windows/darwin go build ./...       # 过
```

基线（P11-bc 实测）：PASS **2431** / FAIL 0 / SKIP **39** / 函数 **1972**。

## mutation 计划（≥2）

- **M1** 整体回到修前（`git checkout` 两个产品文件）：四条新 pin 全红；同时把 dump 的
  PNG 与修后逐字节比对，证明本轮**零像素改动**。
- **M2** 只删 `feedbackNote` 的三处 `PassThrough()`：`TestTheNoticeTakesNoPointer` 红 +
  源码契约红。
- **M3** 只删 `residentNote` 两句 `Text` 的 `PassThrough()`：`TestTheResidentNoteTakesNoPointer`
  红 + 源码契约红。
