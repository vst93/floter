# P11-cc 规格 · 列表底部那条带永远只有一行——长句子不再折到列表上

## 意图（一句话）

`feedbackRow`（toast / 警告）与 `confirmPowerRow`（关机/重启确认）的句子在**固定 30u 的带**里
默认换行，一句长话就折成两行、越出带子的矩形，把上面列表最后几行的字压住；把它们钉成
**一行 + 省略号**，与同一条带里的 `heldHintRow` 和确认按钮已有的 `SingleLine()` 一致。

## 为什么现在做

选题顺序：用户实测反馈 F1–F4 已收完（P11-bg/bc/bd/bj/bb 逐条落地）→ `docs/raycast-design.md`
§6 全 `已落地`，只剩拍板项 → P11-cb「仍待」第 1 条整张是 13 条**拍板项**、第 2 条是上一轮
自己实测出来的尺子盲点（PTY 行画不进 `Tester`）→ 按用户定的优先级落到「界面质感与交互
（含用户报的 bug）」自己找下一处。

**证据（本轮实测，HEAD `11c2c03`）**：带子是高 30u 的 `Absolute` 行
（`internal/launcher/launcher.go:3049-3050`，`Height(t.Space(7.5))`），而句子的 `Text` 没有
`SingleLine()`（`launcher.go:3091`）：

| 句子 | 实测矩形 |
|---|---|
| `Copied`（6 字节） | `{X:41 Y:460 W:662 H:22}` —— 一行 |
| `Uninstalled ` + 4×`very-long-extension-name-`（112 字节） | `{X:40 Y:404 W:664 H:44}` —— **两行** |
| 120 个工具的列表滚到末行 | 最低行 `app-20` 的下沿 `y=419.0` |

于是同一帧里 `app-20` 的标题（`{X:54 Y:395 W:54 H:24}`）与下一个列表行的字（`{X:54 Y:446 W:29 H:24}`）
都落在句子的矩形里，仓库自己的「字压字」尺子当场报红：

```
long toast: "Launcher results" {X:0 Y:60 W:720 H:383} is painted over "Uninstalled very-long-…" {X:40 Y:404 W:664 H:44} (overlap 664.0x39.0)
long toast: "app-20" {X:54 Y:395 W:54 H:24} is painted over "Uninstalled very-long-…" (overlap 54.0x15.0)
long toast: "app" {X:54 Y:446 W:29 H:24} is painted over "Uninstalled very-long-…" (overlap 29.0x2.0)
```

**不是假想的句子**：带子里的字来自 `internal/shell/notices.go` 的 `panelFeedback(message, …)`，
而 `message` 是 `notifyCompletion` 用 `i18n.Notifications` 拼的，里面带**用户数据**
（`IntegrationInstalled(entry.Name)`、`IntegrationInstallFailed(id)`、
`IntegrationDisconnected(name)` 等，`internal/shell/shell.go:851-968,2036-2076,2440-2880`）。
一个 npm 包名/扩展名足够长就越线——`IntegrationDisconnected` 的中文句子本身就带
「，外部文件和系统工具均已保留」（`internal/i18n/copy.go:950`）。

窗口只会为这条带长 **30u**（`Geometry.Feedback` 是 `bool`，`launcher.go:1237`；`height.go:44`
的 `launcherStatusUnits = 7.5`），带子也是 `Absolute` 且 `Height(Space(7.5))`——**它长不高**，
所以折行的句子只能越出自己那块矩形压在列表上。

## 落地项（位置 → 现状 → 应改成）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/launcher/launcher.go:3091` | `ui.Text(c, a.toast).FontSize(…).FontWeight(weight).TextColor(tone).Grow(1)` —— 无 `SingleLine()`，长句折两行（实测 H=44，带子 30u） | 追加 `.SingleLine()`（框架语义：一行 + 结尾 `…`，`ui/api_gen.go:5558`） |
| `internal/launcher/power.go:150` | `ui.Text(c, a.powerMessage()).FontSize(…).FontWeight(560).TextColor(t.Warning).Grow(1)` —— 同一族、同一条带、同一个缺失；今天两句问话都能装进一行（en 56 字节）所以没显形 | 同样追加 `.SingleLine()`，让「带子里三句话」只有一种排法（`heldHintRow` 的提示、确认按钮的标签早就有一行约束） |

**同族的另外两处故意不动**（写进报告）：

- `internal/terminalui/terminalui.go:1336` 的失败便条：它挂在画布顶上、**没有固定高度**
  （`Padding` 撑高），折行只是「多盖几行输出」，不是「越出自己的矩形」；而终端的失败原因
  是用户读它才知道为什么没启动，截断在这里代价更大。
- 带子里的句子**截断**而不折行，是这一轮在红线内唯一正确的答案：要显示全文就得让带子长成
  两行，那是高度链（`Geometry.Feedback` 由 `bool` 改成计数）——写进「需用户拍板」。

## 行为契约零改动红线

高度链（`launcherStatusUnits` / `Geometry.Feedback` / `feedbackBand` / 列表底部 padding 全部
一个字节不动）、键盘模型、会话管理、设置写回、i18n 文案、命令接线——**零改动**。本轮只给
两个 `Text` 加一个排版修饰符，墨、面、字号、内边距、时钟、优先级顺序都不动。

## 门槛命令 + PASS 基线（本轮开跑前实测）

```
gofmt -l cmd internal                # 空
go vet ./...                         # 净
GOOS=windows go vet ./...            # 净
GOOS=darwin  go vet ./...            # 净
go test -count=1 ./...               # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2744（本轮实测基线，0 FAIL / 54 SKIP）
三平台 go build ./...                # 过
```

## mutation 计划（≥2 条，每条要说清让哪个 pin 变红）

- **M1**：把 `launcher.go:3091` 的 `.SingleLine()` 删掉（回到 HEAD）→ 新 pin
  `TestTheStatusBandKeepsItsSentenceOnOneLine` 必须红（H 44 ≠ 22，且 `frameaudit` 的
  「`app-20` 压在句子上」重现）。
- **M2**：把 `power.go:150` 的 `.SingleLine()` 删掉 → 预计**绿**（今天的问话装得下一行），
  如实写进报告的「绿变异也是账」：它守的是形态一致，不是这一句的当前宽度。
- **M3**：把 `feedbackRow` 的带子 `Height(t.Space(7.5))` 改成 `Height(t.Space(10))` →
  预计**绿**（句子已经一行，带子变高只是列表少一行；高度链不在本 pin 的守卫里）——同样如实记账。
- **M4**：把 `SingleLine()` 换成 `MaxLines(2)`（等于不裁）→ 新 pin 必须红，用来证明
  这条 pin 真的在量「折行」，不是量「有没有这个调用」。
