# P13-fg 规格 · 集成页的新鲜度格子：两个值格子，一个在说答案，一个在说「还没有答案」

**选题**：通用队列 · **状态反馈**（`docs/rounds/P13-ff-report.md`「仍待」第 1 条，即
`docs/liquid-glass.md` §6.127 的 `127-5`）。表面 = **集成页抽屉里的 `freshnessSection`**
（块标签 `Freshness` + 两格 metadata 网格 `Last probe` / `Result` + 命令行 + 健康来源注脚）。
**一轮只做一个表面**，不动命令行与注脚。

## 意图（一句话）

**同一格里，答案与「还没有答案」此前是同一个墨。** `Result` 的五个词（Succeeded /
Succeeded with warnings / Failed / Probing… / Unknown）与 `Last probe` 的两种值
（`3 min. ago` / `Never probed`）**全部穿 `TextSecondary`**，于是「探测失败了」和「成功了」
同重，「还没探测」和「探测过了」同重——这一块的四条事实里，有两条在墨上不存在。

## 为什么现在做

- §6.127 的 `127-5` 明确写着「那是一族状态词共用一个墨档的时间线，只抬 running 一臂会让
  同一行断档，**要改得整族一起改**」。本轮就是整族一起改。
- 上一轮（P13-ff）刚把「在飞的词」这一族收口，`freshnessState` 是这一族唯一剩下的成员。

## 先量（改前实测，`-run TestMeasureP13FGFreshnessResultInk`，720×900 生产取景）

量法：`boxFace`（词所在格的众数像素）→ `boxInk`（离面最远的像素）→ `nearestStep`
（最近的候选墨档 + 级差）。**格盒五个状态逐字节相同** `{451.3, 387.7, 243.5, 17}`。

| 帧 | 状态 | 实测墨 | 最近档 | 距 `strong` | 距 `secondary` | 距 `muted` |
|---|---|---|---|---|---|---|
| 浅 | success | `{110,110,115}` | secondary (3 off) | 86 | 3 | 20 |
| 浅 | degraded | `{107,107,112}` | secondary (0 off) | 83 | 0 | 23 |
| 浅 | failed | `{109,109,114}` | secondary (2 off) | 85 | 2 | 21 |
| 浅 | running | `{107,107,112}` | secondary (0 off) | 83 | 0 | 23 |
| 浅 | unknown | `{107,107,112}` | secondary (0 off) | 83 | 0 | 23 |
| 深 | success | `{151,151,157}` | secondary (3 off) | 86 | 3 | 18 |
| 深 | failed | `{152,152,158}` | secondary (2 off) | 85 | 2 | 19 |
| 深 | running / unknown | `{154,154,160}` | secondary (0 off) | 83 | 0 | 21 |

结论：**五个词、两种值，一条墨**。阶梯本身很宽（`secondary → strong` 83-86 级、
`secondary → muted` 18-23 级），所以只要把词挂到正确的档上，一眼就能分开——不需要新色、
不需要新字、不需要动几何。

## 落地项

| # | 位置（`internal/settingsui/settingsui.go`） | 现状 | 应改成 |
|---|---|---|---|
| fg-1 | `freshnessSection` 的 `Result` 格（`:6047`） | `metadataCell` 里写死 `tokens.TextSecondary` | 走新 `freshnessResultInk`：`degraded` / `failed` → `TextStrong`；`running` / `unknown` → `t.TextMuted`；`success`（及其它）→ `TextSecondary` |
| fg-2 | `freshnessSection` 的 `Last probe` 格（`:6046`） | 同上 | 走新 `freshnessWhenInk`：`AtSeconds == 0`（`Never probed`）→ `t.TextMuted`；有值 → `TextSecondary` |
| fg-3 | `metadataCell`（`:6092`） | 值墨写死在函数里 | 加一个 `ink ui.Color` 参数（只有这两个调用点） |
| fg-4 | 为什么 `failed` 不花 `Warning` | —— | 同一张卡上 `Warning` 已经被两件事花了：`stateMark` 的 broken 胶囊（集成自己坏了）与 `healthDetail` 的失败卡 / `statusInk` 的 `error`。§6.108（P13-em）立的规矩是**一个事实一处告警**；新鲜度失败在这一块里没有第二个载体，但它也不是「坏掉了」，它是**这一格的答案要读**——所以用中性阶梯的顶档 `TextStrong`，不花语义色 |
| fg-5 | 字号 / 盒 / 字重 / 行内 gap / 块的纵向节奏 / 文案 / i18n / 命令接线 | —— | 一字节不动（只换墨） |

**`running` 为什么是「静」而不是「响」**（与 §6.127 不冲突）：§6.127 那句在飞词是
**替掉了两个熄灯按钮**的唯一信号源，所以它必须响；这里 `Probing…` 是**一个答案位上的占位**，
它与 `Unknown`、`Never probed` 是同一类（还没有答案），所以它与它们同档。契约里要把这条
分界写清楚：**「在飞」的档位取决于它是「唯一的说话人」还是「答案位上的占位」。**

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。本轮只改墨，
且只改 `freshnessSection` 两格的墨。块的盒（`{451.3, 387.7, 243.5, 17}`）与整页高度
必须逐字节不变。

## 门槛（PASS 基线 = HEAD `db782314` 实测 3402 / 0 FAIL / 189 SKIP / 2569 函数）

```
gofmt -l cmd internal                              空
go vet ./...                                       净
GOOS=windows go vet ./...                          净
GOOS=darwin  go vet ./...                          净
go test -count=1 ./...                             0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"  只许涨
三平台 go build                                    全过
```

## mutation 计划（≥2，实跑并抄原始报错）

- **M1**：`freshnessResultInk` 的 `degraded, failed` 臂退回 `TextSecondary` →
  `TestTheFreshnessResultWordIsAnAnswerOrAPlaceholder` 必须红（failed 与 success 同墨）。
- **M2**：`running, unknown` 臂退回 `TextSecondary` → 同一条 pin 必须红（占位与答案同墨）。
- **M3**：`freshnessWhenInk` 恒返回 `TextSecondary` → `TestTheNeverProbedCellIsQuieterThanAProbedOne`
  必须红。
- **M4（等价写法）**：把 `tokens.TextStrong` 换成 `t.TextStrong`（同一个值）→ 应全绿，
  说明 pin 钉的是档不是写法。
