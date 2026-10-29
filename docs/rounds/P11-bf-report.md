# P11-bf 报告 · 卡行「下面那块」只有一个横向列

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完，用户新报的 `Super+N` 编号渲染
已由 P11-bb 收完）→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K`＝拍板）→ 上一轮「仍待」
（P11-be 第 1/2 条是拍板项，第 4 条是侦察结论）→ **第 3 条：`approvalDetail` 的
「Approved permissions」块的 pills 与两个 caption 没有横向内缩，x=188 vs 行的 196**
→ 界面质感与交互逻辑（用户 2026-10-28 定的优先档）。规格 `docs/rounds/P11-bf-spec.md`。

## 1. 根因：一个槽，三个左边缘

`cardRow`（`internal/settingsui/rows.go:966`）的 `below` 槽画的是「这一行底下展开的东西」：
权限审计、配置表单、批准记录的权限块。它以前是行的**兄弟**，直接排在卡自己的
`ui.Column` 里——没有 `Padding`、没有 `Margin`，于是从**卡的边**起排。

而行的内容盒在 `rule(1) + Padding(vertical, inset-1)` 之后，正好是 `inset`
（`rowInset`，`rows.go:339`：设置卡 14u、集成卡 8u）。于是同一个问题有了三个答案：

| 谁 | 做法 | 结果 |
|---|---|---|
| `permissionAudit`（`settingsui.go:2987` 旧） | 自己补 `Padding(0, inset, t.Space(1), inset)`，注释写着「it is drawn on the card's content column」 | 看起来对——**因为它自己把自己挪过去了** |
| `configFields`（`config.go:161` 旧） | 同样自己补 `Padding(0, inset, 0, inset)` | 同上 |
| `approvalDetail` 的权限块（`settingsui.go:2369`） | 没补 | caption 与药丸整块左移一个 inset |
| 其余（工具表单的 `toolIssueUnder`、自定义快捷键的 warning、配置的 discard 带…） | 没补 | 同上，只是没人报 |

本轮实测（`720×900`，`auditFixture` + 一条 stale 批准记录，`ui.Tester.Find` 的盒子 x）：

| 元素 | 修前 | 修后 |
|---|---|---|
| 行标签 `Approved permissions` | 200 | 200 |
| caption `Floter enforces` | 192 | **200** |
| caption `Declared, not blocked` | 192 | **200** |
| 药丸文字 `Write the clipboard` | 201 | **209** |
| stale 横幅的句子 | 204 | **212** |

一个块里三个左边缘（192 / 201 / 204），而它上面每一行都从 200 起。

## 2. 落地

1. **`cardRow` 给整个 `below` 槽上列**（`rows.go:1041-1056`）：
   `if anyBelow(below) { ui.Column(c).FillWidth().Padding(0, inset).Children(...) }`。
   垂直内边距为 **0**，所以 y 坐标与行高一个没动；只有 x 变了。新助手 `anyBelow`
   （`rows.go:1063`）让没有 below 的行一块多余的列都不建。
2. **`permissionAudit` 不再自己补**（`settingsui.go:2987`）：`Padding(0, inset, t.Space(1), inset)`
   → `Padding(0, 0, t.Space(1), 0)`，`inset := a.rowInset(t)` 删除。
3. **`configFields` 不再自己补**（`config.go:161`）：`Padding(0, inset, 0, inset)` 整段删除。
   实测这一处**零像素变化**（见 §5）——它以前算出来的位置与新位置逐字节相同，正是
   「同一件事两个答案」的证据。
4. **三条 pin**（`internal/settingsui/belowcolumn_test.go`）：
   - `TestNoBelowBlockReappliesTheCardsInset`（源码契约：这两块的函数体里不许再出现
     `rowInset(`，把「一个答案」钉在源码上）；
   - `TestTheApprovalBlocksPermissionsSitOnTheCardsTextColumn`（几何：两块 caption 的 x
     **等于**行标签的 x；药丸文字在列**内**；stale 横幅的句子不在列**左**）；
   - `TestTheConfigFormSitsOnTheCardsTextColumn`（几何：表单第一个字段标签的 x 等于
     `Configuration` 行标签的 x）。
5. 视觉证据 `TestDumpP11BFBelowColumnPNGs`（opt-in `FLOTER_PNG=1`）。

**顺带收掉的同族**（同一个 `cardRow` 改动，不需要各自再改）：工具表单每条 row 的
`toolIssueUnder` 警告、自定义快捷键行的 warning、`detectedSection` 里那条嵌套的 path 行、
`configDiscardBar`——它们现在都在行的内容列上。

## 3. 台账（V/B/S）

- **V = 0**：没有旧 pin 钉过这些块的横向**数值**。`TestThePermissionAuditAlignsWithTheRowLabels`
  （`extensionmetrics_test.go:172`）钉的是**相等**，不是数字；它在修到一半时红过一次
  （见 §4 M2），修完仍绿，断言一字未改。
- **B = 0**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线零改动。
  垂直内边距为 0 是这一条的实现保证。
- **S = 0**：`cardRow` 的签名不变（`below ...func()` 仍是同一个槽），槽里多一层列。
  函数 **1980 → 1984**（+3 pin、+1 opt-in dump）。

## 4. 变异（3 条实跑变红 + `md5sum -c` 还原 + `touch` 复跑绿）

| # | 注入 | 红在哪（原始报错串） |
|---|------|----------------------|
| M1 | `Padding(0, inset)` → `Padding(0, 0)`（等于回到兄弟直排） | `the tier caption "Floter enforces" starts at x=192 but the row label at x=200 (delta -8)` / `the tier caption "Declared, not blocked" … (delta -8)` / `the form's first field label starts at x=192 but the row label at x=200 (delta -8)` |
| M2 | 把 `permissionAudit` 的 `Padding(0, inset, t.Space(1), inset)` 加回去（双重内缩） | `permissionAudit asks rowInset for the card's column: the … block is padded twice and lands one inset right of the row labels` + 既有 pin `[small] the tier label starts at x=203.8 but the Permissions label at x=195.8 (delta -8)` / `[large] … x=212.2 … x=204.2` |
| M3 | `Padding(0, inset)` → `Padding(0, inset*2)` | `the tier caption "Floter enforces" starts at x=208 but the row label at x=200 (delta 8)` / 第二条 caption 同 / `the form's first field label starts at x=208 … (delta 8)` |

每条都 `md5sum -c /tmp/p11bf.md5` 逐字节核对（三个文件「成功」），`touch` 测试文件再跑一次绿。

## 5. 视觉证据（我自己量了像素，不是只看测试数字）

`/tmp/floter-P11-bf-approval-{light,dark}.png`、`/tmp/floter-P11-bf-config-{light,dark}.png`
（修后），同名的 `-before.png` 是同一帧在修前代码下的导出（`git stash` 掉三个源文件后
`FLOTER_PNG=1` 重跑同一个 dump）。

**我在图上量到的**：

- `approval-light`：整帧差异 **6110 px**，包围盒 **x=188…702、y=692…824**——正好是
  「批准权限块 + 药丸 + stale 横幅」那一块，别处一个像素没动。caption `Floter enforces`
  的墨在 `y=694` 从 **x=188** 起到 **x=196**（行标签 `Approved permissions` 的墨在 `198`，
  同一个字盒列 `195.8`；差 2px 是 `A` 与 `F` 的侧边距）。深色同位置、同一位移
  （**6117 px**，包围盒相同）。
- `config-light` / `config-dark`：**0 px 差异**（PNG 逐字节相同）。这不是「没修到」，
  而是这一块原本就自己补了同一个 inset——本轮只是把那个 inset 从块里搬到槽里，
  位置逐字节不变。它同时证明 `cardRow` 的 `Padding(0, inset)` 与被删掉的
  `Padding(0, inset, 0, inset)` 是同一个数。

## 6. 门槛（实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # FAIL 0
PASS 2440（基线 2437）| SKIP 42（基线 40；+1 是本轮 opt-in dump，另 1 条是本机环境相关的
`TestBackgroundProbesAreIsolatedFromHostTerminal/login_shell_PATH`）| 测试函数 1984（1980 + 4）
GOOS=linux/windows/darwin go build ./...   # 过
```

## 7. 仍待（P11-bf）

1. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 的终端便条投影 / P11-ba 的终端「进程已退出」便条改 inline /
   P11-bd 的「未揭示时窗口拖拽手柄接不接指针」/ P11-be 的 `Permissions` 行「标签与胶囊
   同一个词」。
2. **新观察（未改，属设计决定）**：`configDiscardBar` 用的 `noticeBand` 自带
   `Margin(0, t.Space(3))`（12u，参考 `.extension-discard-bar { margin: 0 12px }`，
   `main:src/styles/extensions.css:2455-2458`）。它现在落在行的内容列上，于是那条带的
   **面**比列再右 12u（修前只右 4u）。带子的 margin 是参考自己的写法（相对于它所在的
   抽屉内沿），本仓把它放进了行里，要不要在这条位置把 margin 归零是**设计决定**。
3. **侦察结论（本轮量过，别当 TODO 重领）**：`below` 槽里唯一的「嵌套行」
   （`detectedSection` 的 path 行）现在同时吃到槽的 inset 和它自己作为行的 inset，
   于是它的分隔线与标签都在列的右 8u。它是一条**行**（有自己的发丝与 hover 面），
   两段几何各自成立，不是重复 padding。
4. **侦察结论**：`permissionAudit` / `configFields` 之外，本轮没有第三处自补 inset 的
   below 块（`grep -n "rowInset(" internal/settingsui/*.go` 的其余命中是
   `cardRow` / `slider` / `effectiveOpacity` / `runForm` / `runOutput` / `runOutputToggle`，
   后四个不在 below 槽里，是卡列里的独立块，保持原样）。
