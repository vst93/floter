# P13-dq 规格：帧高的两条腿各有名字（库存尺 / 出货尺），先钉尺子再收口

**轮号**：P13-dq（接 P13-dp）｜合约 `docs/liquid-glass.md` §6.86
**选题**：`P13-dp`「仍待」第 1 条 = `§6.76` 拆轮表的下一格 `P13-dg-l`（`internal/launcher` 的取景收口）。

## 1. 意图

`§6.76` 把 `P13-dg-l` 记成「21 条红」。本轮先把它量准：不是 21 条，
是 **118 条断言散在 17 个文件 / 21 个测试函数**里，而且其中一条（`outputsubtitle`）
是产品墨的问题，不是取景问题。所以本轮**不整包翻牌**（一次把 `render` 换成设计尺
会让 21 个函数同时红，改不完），而是把**尺子本身**落地并钉死，剩下的账逐表面交割。

一句话：**这一轮让「夹具的帧高」从一句算术变成一个可命名的决定**——库存尺
（4.0 + `font = 14` 兜底）与出货尺（3.6 + 实测行盒 19/17）各有名字、各有数字，
下轮起每收口一个表面就换一个表面的取景。

## 2. 证据（本轮实测，行号已核）

| 量 | 值 | 出处 |
|---|---|---|
| 出货尺 `Spacing` | 3.6 | `internal/theme/theme.go:464`（`t.Spacing = round1(t.Spacing * f)`） |
| 出货尺 `FontSize` | 10.8 | 同文件 `:463` |
| 行盒（`Ag`，标题 12.6） | 19 | `launcher.go:1297-1307` 的 `MeasureText` |
| 行盒（`Ag`，正文 10.8） | 17 | 同上 |
| 库存尺十行帧高 | **488** | `frameHeight(maxResultSlots)` |
| 出货尺十行帧高 | **439** | `themedFrameHeight(maxResultSlots)` |

`frameHeight` 此前只有一条腿：`Geometry{Spacing: 4.0}`，`Font/TitleLine/SubtitleLine`
全缺省 → `Height()` 里 `titleLine = font + rowTitleStep = 16`、`subtitleLine = 14`。
出货尺下这两条腿一起落到 36u 的行地板上，所以**行上看不出差**；差在页头/小节这些
只按行盒算的带上（`§6.76` 的编号族 9 → 10 就是它）。

## 3. 逐条落地项

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `internal/launcher/view_test.go` `frameHeight` | 库存尺，没喂行盒 | 保名、保语义，注释写明它是**库存尺**的帧 |
| 2 | 同文件 新增 `themedFrameHeight` | — | 出货尺：`Spacing 3.6` + 实测 `TitleLine/SubtitleLine`，收口每格时换成它 |
| 3 | 同文件 新增 `designMetrics` | — | 按 `App.View` 的量法（`MeasureText(0, Span{"Ag", …})`）量出货尺的四个数 |
| 4 | 新文件 `internal/launcher/frameruler_test.go` | — | 2 条 pin：尺子四数、两帧高 488/439、`themedFrameHeight == Geometry.Height()+2`；探针解析的是 `settings.Default()` |
| 5 | `docs/liquid-glass.md` §6.86 | — | 记这轮的量与账：118 条断言 / 17 文件 / 21 函数，按类交割 |
| 6 | `.pending-bugs.md`「仍待」 | — | 21 条逐条登记，写清每条的处置（V / S / 产品墨） |

## 4. 行为契约零改动红线

高度链（`Geometry.Height` 与调用方）、键盘模型、会话管理、设置写回、i18n 文案、
命令接线：**零改动**。本轮只动测试脚手架的命名与注释，`git diff` 可证产品代码为空。

## 5. 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净（含 windows / darwin）
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux|windows|darwin go build ./...
```

基线（P13-dp 实测）：**3322 PASS / 0 FAIL / 153 SKIP / 2485 测试函数**。
本轮新增 2 个测试函数 → 期望 **3324 PASS / 2487 函数**（SKIP 不变）。

## 6. mutation 计划（≥2，实跑）

- **M1**：`designMetrics` 里标题行盒的 `FontSize + rowTitleStep` → `FontSize`。
  期望红：`the shipped line boxes are title 17.00 / subtitle 17.00, want 19 / 17`。
- **M2**：`designMetrics` 把主题装成 `ui.LightTheme()`（库存）再量。
  期望红：`the shipped line boxes are title 22.00 / subtitle 22.00, want 19 / 17`。
- **M3（预期静默，写进报告）**：`themedFrameHeight` 喂 `TitleLine: 0, SubtitleLine: 0`
  —— 十行帧高不变（36u 行地板赢），照实登记为这条 pin 的盲点。
