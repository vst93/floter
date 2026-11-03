# P13-ff 规格 · 集成页的导出/导入行：在飞那一句话，比它自己按灭的两枚胶囊还安静

**轮号**：P13-ff ｜ **分支** `mygo-rewrite` ｜ **基线** HEAD `0ae13ec9`（P13-fe）
**队列**：通用队列 · **状态反馈 · 忙 / 进行中**（P13-fe「仍待」第 1 条）
**表面**：集成页（Integrations）的 `transferRow` —— Export / Import 两枚胶囊 + 行尾那句
`Exporting…` / `Importing…`。一轮一个表面，不跨页。

## 意图

导出/导入在飞时，行里两枚胶囊一起熄灯（P13-aq 的禁用面），整行的**唯一信号源**只剩行尾那
一句话。这句话必须比它自己按灭的那两枚胶囊重一档；它此前与页面上永远不变的帮助句同一档墨。

## 为什么现在做（证据，实测）

生产取景（`settings.NewStore` 设外观 → `c.SetTheme(a.tokens(c).Theme)` → 720×1600 → 点
`Integrations` → `TransferBusy = "export"`），量的是**这一行的卡面**（在 import 胶囊右缘与
在飞句左缘之间取一条竖带）：

| 帧 | 卡面 | 在飞句墨权重 | Export 胶囊（已禁用） | Import 胶囊（已禁用） |
|---|---|---|---|---|
| 浅色 | 237 | **107** | 105 | 105 |
| 深色 | 27 | **104** | 105 | 105 |

也就是说：浅色上这句只比它按灭的按钮重 **2** 级，深色上比它们**还安静 1 级**。同一页里
「永远不变」的帮助句（`IntegrationsHint`）与它同档（`t.TextMuted`）。

代码位置：`internal/settingsui/settingsui.go:2115-2119`（`transferRow` 的 `switch a.TransferBusy`
两臂），逐字为
`ui.Text(c, copy.IntegrationsExporting).FontSize(a.tokens(c).Caption).TextColor(t.TextMuted)`。

## 逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `transferRow` export 臂 | `TextMuted`，无字重 | `tokens.TextSecondary` + `FontWeight(500)` |
| 2 | `transferRow` import 臂 | 同上 | 同上（两臂收口到一个 `busyLine`） |
| 3 | 字号 / 盒 / 行内中线 | `tokens.Caption`，居中于两枚胶囊 | **不动**（钉住：本轮只改墨） |
| 4 | 环（spinner） | 没有 | **不动**：写进 §6.127 的 `未开始（需拍板）` |

**依据**：本仓对「在飞的词」已有两条规矩——启动器页脚（`launcher/output.go` 的 `footerWord`）
在飞 `TextSecondary` + 500、静止 `TextMuted`；设置页配置工具条（`config.go`，P13-el）在飞与
落地**同档**，因为它有一枚落地后的孪生词。这一句没有孪生词（结束整句消失），走第一条。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一字节不动**。本轮只改墨与字重；
`TransferBusy` 的取值与语义、两个 `Actions` 的接线、`transferReport` 的渲染都不碰。

## 门槛命令 + PASS 基线

```
gofmt -l cmd internal                                 必须空
go vet ./...                                          必须净
GOOS=windows go vet ./...                             必须净
GOOS=darwin  go vet ./...                             必须净
go test -count=1 ./...                                0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"     基线 3400，只许涨
GOOS=linux|windows|darwin go build ./...              三平台全过
```

基线（HEAD `0ae13ec9` 实测）：**3400 PASS / 0 FAIL / 188 SKIP / 2566 测试函数**。

## mutation 计划

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `busyLine` 的墨 `tokens.TextSecondary` → `tokens.TextTertiary` | `TestTheTransferBusyLineOutranksTheButtonsItKilled`（浅 95 / 深 83 对胶囊 105） |
| M2 | `busyLine` 的字号 `tokens.Caption` → `tokens.Body` | `TestTheTransferBusyLineKeepsTheCaptionBoxAndTheMidline`（盒 17 对页面的 caption 行 15） |
| M3 | 等价写法（builder 调用顺序调换） | **全绿**，已知覆盖边界（钉的是档，不是调用顺序） |

每条都要 `md5sum -c` 逐字节核对还原，并 `touch` 后复跑一次（避免测试缓存复用）。
