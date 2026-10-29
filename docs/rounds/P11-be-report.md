# P11-be 报告：权限审计块与诊断块各归其位

轮号 **P11-be** ｜ 规格 `docs/rounds/P11-be-spec.md` ｜ 分支 `mygo-rewrite` ｜ 起点 `05101b3`（P11-bd）

## 0. 一句话

集成卡片里两块折叠出来的东西各自回到了它们该在的地方：**权限审计的开关只剩行尾那一枚**
（折叠块里那条 660u 宽的「Permissions」整宽按钮删掉），**诊断块成了卡里的一行**
（分隔线、行地板、行的内缩都回来了，块标签与检查行落在卡的文字列上，不再贴着上一行的 pills）。

## 1. 选题

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完；用户新报的 `Super+N` 编号渲染
已由 P11-bb 收完）→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K`＝拍板）→
上一轮「仍待」（**P11-bd 的四条全是拍板项或侦察结论**）→
**界面质感与交互逻辑**（用户 2026-10-28 定的优先档，压过一致性收尾）。

所以这一轮是自己在「界面质感与交互」里找的，证据是**图**不是旧报告：
`FLOTER_PNG=1 go test -run TestDumpP8DIntegrationAndSessionPNGs ./internal/settingsui/`
导出的集成页上，同一块面上「Permissions」出现了三次，其中一次是一条整宽按钮；
`Diagnostics` 那行字紧贴在 `Read files` 那颗 pill 上，像它的说明。

## 2. 根因（两处，各自独立）

### 2.1 一个开关画了两遍

`cardRow` 的 `below` 回调**每帧都画**（`internal/settingsui/rows.go:1035-1040`：
`for _, extra := range below`），而 `permissionAudit` 在**判断折叠状态之前**就画了一枚
`actionButton(c, copy.IntegrationsPermissions)`（旧 `settingsui.go:2951-2956`）。
于是：

- 行自己的控制槽里一枚胶囊（`:2473`）——卡里每一行的控制都在这里；
- 折叠块开头又一条整宽按钮——**折叠状态下也在**。

`tt.Texts()` 实测：修前 `"Permissions"` **3 次**，修后 **2 次**（行标签 + 行尾胶囊）。

### 2.2 诊断块不是卡里的一行

`diagnosticsSection` 的块标签是裸 `ui.Row`（旧 `:2166`），块正文是裸 `ui.Text` / `ui.Grid`，
`diagnosisCheckRow` 的 grid 是 `Padding(Space(2), 0)`（旧 `:2206`）——横向零内缩。
于是它既没有 `cardRow` 的分隔线，也没有行内缩：

| 元素 | 修前 | 修后 | 卡的行标签（基准） |
|---|---|---|---|
| 块标签 `Diagnostics` 左边缘（720 宽） | x=**192** | x=**196** | x=196 |
| 检查行状态词 `Probe-ok` 左边缘 | x=**192** | x=**196** | x=196 |
| 860 宽导出图 | x=**188** | x=**196** | x=196 |

（两个宽度下都差**一个 `rowInset` 减 1**，因为裸 Row 落在卡的边框内沿上。）

## 3. 落地

### 3.1 `internal/settingsui/settingsui.go` `permissionAudit`

删掉开头那枚 `actionButton` 与它带进来的 `auditOpen` map 初始化，函数体从
「先画按钮再看折叠状态」变成「先看折叠状态」。注释里写清了为什么：
`below` 每帧都画，所以第二个按钮在折叠时也在，而行的标签本身就是命中目标
（`cardRow` 的 `names.Clicked()` → 行的 `labelClicked`），删掉它**没少任何能力**。

### 3.2 `internal/settingsui/settingsui.go` `diagnosticsSection`

整块改走 `a.cardRow(c, ui.Start, nil, rowFace{}, func() {}, func(){ ... })`：
块标签、结论词、检查行都画进 `cardRow` 的**标签列**（`draw`）。于是分隔线、行地板、
`inset` 全由 `cardRow` 给，与 `approvalDetail` 的每一行同一套框。

**块标签的字号/字重/墨一个字节没动**（`tokens.Caption` / 620 / `TextSecondary`）——
`healthfaces_test.go` 的 `TestTheDiagnosticsBlockLabelIsTheBlocksOwnStep` 正钉着
「块标签是 caption 档，不许穿行标签的 emphasis 档」，本轮不碰它（该 pin 修前修后都绿）。

## 4. V / B / S 台账

- **V = 0**：没有既有 pin 钉过这两处的数值。`TestTheDiagnosticsBlockLabelIsTheBlocksOwnStep`
  （块标签的档位）与 `TestThePermissionAuditAlignsWithTheRowLabels` /
  `TestThePermissionAuditMovesOffTheCardEdge`（审计块的内缩）修前修后都绿，断言一字未改。
- **B = 0**：`go test -count=1 ./...` 全绿，**既有测试零改动**。`TestPermissionAuditUnfolds`
  （点 `Permissions` 展开/再点收起）仍绿——它点的本来就是行标签，不是那条整宽按钮。
- **S = 0**：没有签名或结构变更；`diagnosticsSection` 的内部走法变了，但它仍是同一个函数、
  同一个调用点、同一个出现条件。
- 新增测试函数 3 个：`TestThePermissionAuditHasOneToggle`、
  `TestTheDiagnosticsBlockSitsOnTheCardsTextColumn`、`TestDumpP11BEAuditBlockPNGs`（opt-in dump）。

## 5. 门槛（本轮实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                    → 全绿，FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 2435 → 2437（+2）
测试函数数                                  → 1977 → 1980（+2 pin、+1 opt-in dump）
三平台 go build ./...                      → 过
SKIP 40（不变；新 dump 是 opt-in，与既有 dump 同一处置）
```

## 6. 变异（3 条实跑变红 → `md5sum -c` 还原 → `touch` 复跑绿）

修前 `md5sum internal/settingsui/settingsui.go` = `e0b1a970a2a9fb7c21f5143c6de71f08`，
三条变异后都用它逐字节核对还原。

**M1 — 把整宽按钮加回 `permissionAudit` 开头**

```
--- FAIL: TestThePermissionAuditHasOneToggle (0.90s)
    auditblock_test.go:61: the Permissions toggle has 2 call sites in settingsui.go, want 1:
      the row's control slot is the only place this card puts a control, and a second button
      in the folded block is on screen even while the audit is closed
    auditblock_test.go:74: the frame shows "Permissions" 3 times, want 2
      (the row's label and the row's pill): [... "Permissions" "Permissions" "Permissions" ...]
```

**M2 — `diagnosticsSection` 退回裸 `ui.Column`（不套 `cardRow`）**

```
--- FAIL: TestTheDiagnosticsBlockSitsOnTheCardsTextColumn (0.06s)
    auditblock_test.go:104: the diagnostics block label starts at x=192 but the card's row
      labels at x=200 (delta -8): the block is not on the card's text column
    auditblock_test.go:113: the check row starts at x=192 but the card's row labels at x=200
      (delta -8): the check grid is not on the card's text column
```

**M3 — 保留 `cardRow`，但把检查行挪进 `below` 槽（标签对、检查行不对）**

```
--- FAIL: TestTheDiagnosticsBlockSitsOnTheCardsTextColumn (0.03s)
    auditblock_test.go:113: the check row starts at x=192 but the card's row labels at x=200
      (delta -8): the check grid is not on the card's text column
```

M3 只让**第二个臂**红、第一个臂仍绿——这正是两个臂各自负责一件事的证据。

## 7. 视觉证据与「我实测看到了什么」

`/tmp/floter-P11-be-audit-{light,dark}-{closed,open}.png`（860×1400，`FLOTER_PNG=1` 一次导出）：

- `light closed`：`Permissions` 行下面那条**整宽的「Permissions」按钮不见了**；
  行尾只剩那一枚胶囊。整张卡从 470 起到 1095 的堆叠里少了一段——
  行标签 y 从 470 → **440**（少了 30u 的按钮行），诊断块标签 y 从 824 → **804**。
- `light open`：审计展开后同样只有一枚胶囊；`Diagnostics` 那行现在**有一条分隔线压在它上面**
  （y=998，色 `212` 对卡面 `233`），并且它和 `Approved permissions` 左边缘对齐（x=196），
  不再贴着 `Read files` 那颗 pill 当说明。
- `dark open`：同一条分隔线与同一条文字列，深色下 `Probe-ok` / `probe answered` 两格仍在一条线上。
- 一处仍然「空」的地方：卡片底部那 43u 的空白带**不是本轮引入的**——它是卡尾的
  「动作行」（Check / Re-scan / Uninstall）在这个夹具里没有任何控件时的 44u 行地板；
  真机上那一行有三枚按钮。本轮不动它。

## 8. 仍待

1. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 的终端便条投影 / P11-ba 的终端「进程已退出」便条改 inline /
   P11-bd 的「未揭示时窗口拖拽手柄接不接指针」。
2. **新拍板候选（本轮侦察，未实现）**：`Permissions` 这个行里，标签与行尾胶囊印的是
   **同一个词**（`IntegrationsPermissions`）。要改成别的动词得**新写 i18n 文案**
   （i18n 是红线，不擅自加），或者换用既有的 `LocalToolAdvancedPermissions`
   （en「Permissions & audit」/ zh「权限声明与审计」）——那是**文案决定**，需拍板。
3. **侦察结论（本轮量过，别当 TODO 重领）**：`approvalDetail` 的「Approved permissions」
   那一块（`:2358-2378`）的 pills 与它下面两个 caption 标签同样没有横向内缩
   （实测 x=**188**，行标签 x=196），与 2.2 是**同一族**但**不同一块**——本轮的 `cardRow` 只包住了
   诊断块；把「Approved permissions」的 below 也挪进标签列是下一轮同族候选，不是缺陷重报。
4. **侦察结论**：`diagnosticsSection` 里 `!integration.diagnosisRan()` 那条空态分支
   （`:2173-2181`）在当前唯一的调用点（`:2527` 的 `if integration.diagnosisRan()`）下
   **不可达**。本轮原样保留（它是那个函数自己的完整契约，且将来若搬回抽屉就要用），
   不当缺陷改。
