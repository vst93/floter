# P11-be 规格：权限审计块与诊断块各归其位

## 0. 意图（一句话）

集成卡片里那两块「折叠出来的东西」各有各的错：**权限审计的开关被画了两遍**（行尾一枚胶囊 + 折叠块里一条整宽按钮），
**诊断块没有卡里的一行该有的框**（无分隔线、无内缩，块标签贴在上一行的 pills 上，正文从卡片边缘起排）。

## 1. 为什么现在做（证据）

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完，`Super+N` 编号已由 P11-bb 收完）
→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K`＝拍板）→ 上一轮「仍待」（**全是拍板项**）
→ **界面质感与交互逻辑**（用户 2026-10-28 定的优先档，压过一致性收尾）。

证据全部由本轮实测（`FLOTER_PNG=1 go test -run TestDumpP8DIntegrationAndSessionPNGs ./internal/settingsui/`，
`/tmp/floter-p8d-integrations-light.png`，860×1400）：

### 1.1 一个开关画了两遍

`internal/settingsui/settingsui.go:2471-2478`（行）与 `:2948-2957`（折叠块）：

```go
labelClicked := a.row(c, copy.IntegrationsPermissions, a.permissionLine(integration, copy), func() {
    if a.actionButton(c, copy.IntegrationsPermissions).Clicked() {   // ← ① 行尾胶囊
        a.toggleAudit(integration.ID)
    }
}, func() { a.permissionAudit(c, copy, integration) })
```

```go
func (a *App) permissionAudit(...) {
    ...
    if a.actionButton(c, copy.IntegrationsPermissions).Clicked() {   // ← ② 折叠块里的整宽按钮
        ...toggleAudit...
    }
    if !a.auditOpen[integration.ID] { return }
```

`cardRow` 的 `below` 回调**每帧都画**（`internal/settingsui/rows.go:1035-1040`：`for _, extra := range below`），
所以 ② 在**折叠状态也在**——一个 `ButtonBase` 落在卡的列里，整宽拉伸。图上：
行标签 `Permissions`（x=197）、行尾胶囊 `Permissions`（x=790…830）、**整宽按钮 `Permissions`（x=185…845，y=418…443）**。
同一块面上「Permissions」出现三次，其中一次是一条 660u 宽的按钮。

### 1.2 诊断块不是卡里的一行

`internal/settingsui/settingsui.go:2159-2218` 的 `diagnosticsSection` 与 `diagnosisCheckRow`：

- 块标签是裸 `ui.Row`（`:2166`），块正文是裸 `ui.Text` 与 `ui.Grid`，**都不走 `cardRow`**，
  于是没有 `cardRow` 给每行的分隔线与 `Padding(Space(2.25), inset-1)`；
- `diagnosisCheckRow` 的 grid 是 `Padding(t.Space(2), 0)`（`:2206`），横向零内缩。

实测（同一张图，像素/矩形）：

| 元素 | 现在 | 同一张卡里该对齐的邻居 |
|---|---|---|
| 块标签 `Diagnostics` 左边缘 | **x=188** | `Approval record` 行标签 **x=197** |
| 检查行状态词 `Healthy` 左边缘 | **x=188** | 同上 **x=197** |
| 块标签上方 | **没有分隔线**，距 `Read files` pill 底边 ≈ 6u | 上面每一块之间都有 `Hairline` 分隔线 + 44u 行地板 |

图上读出来的后果：`Diagnostics` 那行字紧贴 `Read files` 那颗 pill，像它的说明；`Healthy` / `Healthy answered`
两行从卡片边缘起排，比上面所有标签都往左探出 9u。

## 2. 逐条落地项

> 动手前已按当前 HEAD（`05101b3`）复核行号。

### 2.1 一个开关只有一个按钮

- **位置**：`internal/settingsui/settingsui.go` `permissionAudit`（现 `:2948-2957`）
- **现状**：函数开头无条件画一枚 `actionButton(c, copy.IntegrationsPermissions)`，折叠时也在。
- **应改成**：删掉这枚按钮（连同它那段 `auditOpen` map 初始化）。开关留在**行的控制槽**里——
  与卡里每一行同一个位置；行标签本身也仍然可点（`cardRow` 的 `names.Clicked()` → `labelClicked`），
  所以**折叠/展开的能力一点没少**，少的只是同一条命令的第二个按钮。

### 2.2 诊断块成为卡里的一行

- **位置**：同文件 `diagnosticsSection`（现 `:2159-2218`）
- **现状**：裸 Row + 裸 Text + 裸 Grid，无分隔线、无内缩。
- **应改成**：整块走 `a.cardRow(c, ui.Start, nil, rowFace{}, func() {}, func(){ ... })`，
  把「块标签 + 结论词 + 检查行」画进 `cardRow` 的**标签列**（`draw`）。这样：
  分隔线、行地板、`inset` 全由 `cardRow` 给，与 `approvalDetail` 的每一行同一套框；
  块标签的**字号/字重/墨一个字节不动**（`tokens.Caption` / 620 / `TextSecondary`）——
  `healthfaces_test.go` 的 `TestTheDiagnosticsBlockLabelIsTheBlocksOwnStep` 正钉着
  「块标签是 caption 档，不许穿行标签的 emphasis 档」，本轮不碰它。

## 3. 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线**零改动**：

- 不新增、不改写任何 `internal/i18n` 条目；不换用别的 key（`IntegrationsPermissions` 仍只用于行标签与行尾胶囊）。
- 折叠状态仍是 `App.auditOpen`，开关仍只有 `toggleAudit` 一个写入口。
- 诊断块的出现条件仍是 `integration.diagnosisRan()`（`:2527`）；`!diagnosisRan()` 那条空态分支原样保留。
- 页面高度链、卡片几何（`cardGeometry` 的 inset/titleFloor/lineGap）、行地板 44u 不改。

## 4. 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                    → 全绿（基线 FAIL 0）
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 基线 2435
三平台 go build ./...                      → 过
```

测试函数数基线 **1977**（`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go`），本轮只增不减。

## 5. mutation 计划（≥2 条，每条说清让哪个 pin 变红）

| # | 注入 | 期望变红 |
|---|---|---|
| M1 | 把删掉的 `actionButton(c, copy.IntegrationsPermissions)` 加回 `permissionAudit` 开头 | `TestThePermissionAuditHasOneToggle`（源码契约：`actionButton(c, copy.IntegrationsPermissions)` 出现 1 次） |
| M2 | `diagnosticsSection` 退回裸 `ui.Column`（不套 `cardRow`） | `TestTheDiagnosticsBlockSitsOnTheCardsTextColumn`（块标签/状态词左边缘 ≠ 行标签左边缘） |
| M3 | 只在 `cardRow` 外面补一个 `Padding`，但不进 `draw`（横向内缩有、行框没有） | 同上（X 对齐仍红）或分隔线断言红 |

每条都实跑、抄原始报错串、`md5sum -c` 逐字节还原、`touch` 后复跑绿。

## 6. 视觉证据

`/tmp/floter-P11-be-audit-<light|dark>-<closed|open>.png`（opt-in dump，`FLOTER_PNG=1`）。
报告写「这张图上看到的一处变化」。
