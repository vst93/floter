# P11-bf 规格 · 卡行「下面那块」只有一个横向列

## 意图（一句话）

`cardRow` 的 `below` 槽（一行底下展开的东西：权限审计、配置表单、批准记录的权限块）
以前是行的**兄弟**、直接排在卡自己的列里，于是它从**卡的边**起排——比它上面每一行的
标签列左一个 inset（集成卡 8u）；其中两块自己补了这个 inset，其余没补。本轮把
「卡的内容列从哪起」收成**一个答案**：`cardRow` 给整个 `below` 槽上 inset，补过的那两块
不再自己补。

## 为什么现在做

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完，用户新报的 `Super+N` 编号渲染
已由 P11-bb 收完）→ 合约未落地条目（`docs/raycast-design.md` §6 全表 `已落地`，只剩
`⌘K`＝拍板）→ 上一轮「仍待」第 1 条是五条拍板项，第 2 条是文案拍板，**第 3 条**是
「`approvalDetail` 的 Approved permissions 那一块的 pills 与两个 caption 标签同样没有
横向内缩（x=188 vs 行的 196）——同一族、不同一块」→ 界面质感与交互（用户 2026-10-28
定的优先档）。

它是**看得见的**：本轮实测（`720×900`，`auditFixture` + 一条 stale 批准记录）——

| 元素 | 修前 x | 修后 x |
|---|---|---|
| 行标签 `Approved permissions` | 200 | 200 |
| 块内 caption `Floter enforces` | 192 | 200 |
| 块内 caption `Declared, not blocked` | 192 | 200 |
| 药丸文字 `Write the clipboard` | 201 | 209 |
| stale 横幅句子 | 204 | 212 |

一个块里三个左边缘（192 / 201 / 204），而它们上面每一行都从 200 起。

## 现状（file:line，动手前实测）

- `internal/settingsui/rows.go:1035-1045`（旧）：`cardRow` 把 `below` 的每个回调当**兄弟**
  直接画在卡的 `ui.Column` 里——没有 `Padding`、没有 `Margin`。行的内容盒在
  `rule(1) + Padding(vertical, inset-1)` 之后，正好是 `inset`（`rows.go:339` 的
  `rowInset`：设置卡 14u，集成卡 8u）。
- `internal/settingsui/settingsui.go:2987-2995`（旧）：`permissionAudit` 自己补
  `Padding(0, inset, t.Space(1), inset)`，注释写着「it is drawn on the card's content
  column」——其实它并不在，是它自己把自己挪过去的。
- `internal/settingsui/config.go:161-169`（旧）：`configFields` 同样自己补
  `Padding(0, inset, 0, inset)`。
- `internal/settingsui/settingsui.go:2329` 的 `approvalDetail`：把两块 caption 与药丸
  放进 `below` 槽，**没补**，于是整块左移一个 inset。
- 参考：`main:src/ExtensionsPanel.tsx:2712-2755` 的 `.extension-approval-record` 在
  `.extension-detail-block` 里，块内所有元素共用一个左边缘
  （`main:src/styles/extensions.css:2124-2168`，`.extension-approval-record` 是
  `display:grid; gap:12px`，没有任何元素自带横向偏移）。

## 逐条落地项

1. **`cardRow` 的 `below` 槽上列** — `internal/settingsui/rows.go:1041-1056`：
   现状：`for _, extra := range below { extra() }` 直接画在卡的列里。
   应改成：`if anyBelow(below) { ui.Column(c).FillWidth().Padding(0, inset).Children(...) }`。
   垂直内边距为 0，所以行高与 y 坐标不变。
2. **`permissionAudit` 不再自己补** — `settingsui.go:2987`：`Padding(0, inset, t.Space(1), inset)`
   → `Padding(0, 0, t.Space(1), 0)`（保留它自己的 1u 顶/底节奏）。`rowInset` 调用与 `inset`
   变量删除。
3. **`configFields` 不再自己补** — `config.go:161`：`Padding(0, inset, 0, inset)` 整段删除，
   `rowInset` 调用与 `inset` 变量删除。
4. **新增 pin** `internal/settingsui/belowcolumn_test.go`：
   - `TestNoBelowBlockReappliesTheCardsInset`（源码契约：`permissionAudit` / `configFields`
     的函数体里不许再出现 `rowInset(`）；
   - `TestTheApprovalBlocksPermissionsSitOnTheCardsTextColumn`（几何：两块 caption 的 x
     **等于**行标签的 x；药丸文字在列内；stale 横幅的句子不在列左）；
   - `TestTheConfigFormSitsOnTheCardsTextColumn`（几何：表单第一个字段标签的 x 等于
     `Configuration` 行标签的 x）。
5. **视觉证据**（opt-in，`FLOTER_PNG=1`）：`TestDumpP11BFBelowColumnPNGs` →
   `/tmp/floter-P11-bf-approval-<light|dark>.png`、`/tmp/floter-P11-bf-config-<light|dark>.png`。

## 行为契约零改动红线（自检）

- 高度链：`below` 槽的垂直内边距是 0，行的 `MinHeight(floor)`、卡的行计数器、分隔线
  全不动。**唯一可能长高的是「文字因为窄了 16u 而换行」**——本轮实测无一处换行
  （stale 横幅的句子在 398u 里本来就只占一行）。
- 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：零改动（只动几何与两处自补
  padding）。
- 视觉轮只改「墨与面」：本轮只改**横向位置**，颜色、字号、字重、圆角、面、文案全不动。

## 门槛命令 + PASS 基线（实测，本轮开工前）

```
gofmt -l cmd internal                          # 空
go vet ./...                                   # 净
GOOS=windows go vet ./...                      # 净
GOOS=darwin  go vet ./...                      # 净
go test -count=1 ./...                         # FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 2437
GOOS=linux/windows/darwin go build ./...       # 过
```

测试函数基线 **1980**（`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go`），
SKIP 基线 **40**。

## 台账预判

- **V = 0**：没有旧 pin 钉过这些块的横向位置（`TestThePermissionAuditAlignsWithTheRowLabels`
  钉的是**相等**，不是数值，且修完仍绿）。
- **B = 0**：高度链、键盘、会话、写回、接线零改动。
- **S = 0**：`cardRow` 签名不变（`below ...func()` 仍是同一个槽），只是槽里多一层列。

## mutation 计划（≥2 条，实跑）

| # | 注入 | 预期变红 |
|---|------|----------|
| M1 | 删掉 `cardRow` 里 `Padding(0, inset)`（回到兄弟直排） | `TestTheApprovalBlocksPermissionsSitOnTheCardsTextColumn` 的 caption 臂（-8）与 config 臂 |
| M2 | 把 `permissionAudit` 的 `Padding(0, inset, ...)` 加回去（双重内缩） | 同一条 caption 臂（+8），以及 `TestThePermissionAuditAlignsWithTheRowLabels` |
| M3 | `Padding(0, inset)` → `Padding(0, inset*2)` | 三条几何臂全红（证明断言对「多缩」也敏感） |
