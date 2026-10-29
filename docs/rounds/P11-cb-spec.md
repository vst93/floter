# P11-cb 规格 · 终端页本身进「字盒高度」尺子（第二把尺子的第四个表面）

## 意图（一句话）

把 P11-cz 那类「字号暴涨 / 行距失控」的守卫补到**终端页本身**（不是外观抽屉），
把上一轮 P11-ca「仍待」第 2 条点名的标题栏 / 浮条两档（22 / 28）量下来，并**实测**
说清 PTY 自己画的行（20）为什么这把尺子看不见——**产品代码零改动**，只加尺子。

## 为什么现在做

`.pending-bugs.md` 的 P11-ca「仍待」第 1 条是 13 条**拍板项**（要用户点头，不能擅自
实现），于是按选题顺序落到第 2 条：

> **终端页另外两段还没进高度尺子**：标题栏 / 画布上方浮条（22 / 28）与 PTY 自己画的行
> （20）——与抽屉的 20/22 同数不同源，要各自量；PTY 行高跟用户设置走，可能钉不成常量。

三处表面各有一份阶梯（launcher 六档、settingsui 八档、terminalui 抽屉五档），终端页是
第四处，且它的档位来自**另一批**产生者（浮条按钮、键帽图例、空态标题），不能照抄。

## 落地项（先实测，后写断言）

测量法：`frameaudit.Heights`（叶子 run 的去重高度）+ `tt.Find(文字)` 拿每个 run 的盒子，
四臂 × en/zh × 深/浅 × 四尺寸。实测结果（本轮 HEAD 上量的，不是旧报告的数）：

| 档 | 产生者（本轮实测行号） | 哪段字 |
|---|---|---|
| 15 | `internal/terminalui/terminalui.go:964` `tokens.Caption` | 空态键帽的图例（`↩` / `Ctrl+W`） |
| 22 | `:1485` `t.FontSize` | 浮条的身份行（`Terminal` / `Terminal · Exited · code 3`） |
| 22 | `:966` `t.FontSize` | 空态两行的标签（`New blank session` / `Back to search`） |
| 22 | `:1262`、`:1264`、`:1336` | 常驻便条两句 / 失败便条 |
| 23 | `:861` `t.FontSize + 1` | 空态的标题（`No terminal session yet`）——全页唯一一处 |
| 28 | `:1366` `Size(t.Space(7), t.Space(7))` | 浮条上的按钮（28u 方钮，文字即按钮标签） |

- 四臂：`empty`（无会话）/ `live`（有会话，浮条已揭示）/ `resident`（`ResidentAt(0,3)`）
  / `feedback`（`ShowFeedback(...)`）。
- 空态画全集 `{15,22,23,28}`；另外三臂只画 `{22,28}`（没有键帽，也没有空态标题）。
- 64 帧逐帧同一组（4 臂 × 2 语言 × 2 外观 × 4 尺寸：640×360 `MinTerminalWidth` ×
  `MinTerminalHeight` / 720×360 / 720×560 / 1100×700）。
- 两条 pin：
  - `TestTheTerminalPageStaysOnTheHeightLadder`（`heights.Ladder` + `Missing`，`tol=0.5`，
    `MinHeight=10` 裁碎片）；
  - `TestEachTerminalPageRungHasItsOwnProducer`（每档钉到它自己的那段字 + 高度，三臂：
    empty-en-light / empty-zh-dark / resident-en-light）。
- 新文件 `internal/terminalui/pageheight_test.go`；**不改任何既有测试**。

## 明确不做 / 实测做不到

- **PTY 自己画的行（20）**：live 臂的阶梯只有 `{22,28}`；画布那条 run 是
  `Terminal input`（整块画布 = 祖先，被叶子规则排除），插件按单元格画的字**一条都没进**
  `tt.Texts()`。所以「终端里的字变高 / 被裁」这把尺子量不到——写进报告 §3 与「仍待」，
  不许加个假档位糊过去。
- 行高跟用户设置走（`TerminalLineHeight`），就算看见也不该钉成常量。
- 不改产品代码；不改 `barHeight` / 空态几何 / 浮条揭示逻辑。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线——**全部零改动**。本轮只加
测试文件与文档。既有的 32 帧「字压字 / 出窗」审计（`frameaudit.Audit`）不动。

## 门槛命令与 PASS 基线（本轮开跑前实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（P11-ca 收尾实测）：**PASS 2675 / 0 FAIL / 54 SKIP / 2041 测试函数**。
本轮新增 2 条测试函数 + 67 个子测试（64 ladder + 3 producer），实测 PASS 2675 → 2744。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. **M1** `terminalui.go:861` 空态标题 `t.FontSize + 1` → `t.FontSize`：
   23 从空态阶梯消失 → `Missing` 报「no run of height 23.0 was painted」。
2. **M2** `terminalui.go:964` 键帽图例 `tokens.Caption` → `tokens.Body`（15 → 17）：
   15 消失 + 17 不在阶梯上 → 两条同时红。
3. **M3** `terminalui.go:1366` 浮条按钮 `Space(7)` → `Space(8)`（28 → 32）：
   `Ladder` 报「height 32.0 is not on the ladder」。
4. **M4（预期绿，也是账）** `barHeight`（`terminalui.go:1407`）`Space(7)` → `Space(9)`：
   条的**高度**不是任何一段字的高度，整帧阶梯一个字节不动——所以「浮条变高」这把尺子
   看不见，别把它当守卫。

每条：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 再复跑。
