# P13-ak 规格：审批记录是「规格表」，不是「设置列表」

## 意图

集成卡里那张折叠的「审批记录」块有四行**事实**（`批准时间` / `清单摘要` / `磁盘摘要`，
外加没记录时的 `未记录`）——上面没有任何东西可以按。它们此前穿的是普通设置行：
emphasis 档（`rowTitle`：`t.FontSize+1` / 580 / `TextStrong`）留给**设置的名字**，
第二行是副标题（`t.TextMuted` / 500）。在一张记录上这是反的：**事实的名字**占了最响的墨，
**事实本身**（时间戳、摘要——用户展开这块唯一想看的东西）是这一行上最轻的字。

实测（英文帧，`inkSpan`，luma 跨度）：

| 帧 | 名字（旧） | 值（旧） | 名字（新） | 值（新） |
|---|---|---|---|---|
| light | **202** | 101 | 122 | **202** |
| dark | **198** | 94 | 115 | **198** |

所以这轮：**两档墨对调，盒子一个像素不动**。

## 为什么现在做（证据）

- `.pending-bugs.md` P13-aj「仍待」第 3 条：`updateRow` 的只读臂（「当前版本: vX」+「已是最新」）
  与可编辑行**长得一样**，先量只读行的墨。量与改都指向同一族：**只读行 = 一个已经确定的答案**。
  P13-s/t/u 已经把「禁用」三层分完（那是一个不能用的控件，淡一道）；「只读」是另一半，
  它不是禁用，它不淡，它只是把重量交给答案本身。
- `internal/settingsui/settingsui.go:2397` `approvalDetail` 的四行正是全仓最纯的一族只读行
  （`a.row(..., func () {})` 空控制槽），且共用 `cardRow` 的框架，所以一处改、四行同时成立。
- `docs/liquid-glass.md` §6 无 `未开始` 行，按纪律走通用队列的「状态反馈 / 细节打磨」。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/rows.go`（新增 `factRow`，紧邻 `codeRow` 之前） | 只有 `row`/`rowNote`/`rowAligned` 这一族，它们的墨语义是「设置名 + 副标题」 | 新增 `factRow(c, name, value)`：`name` = `tokens.Caption` / 500 / `TextSecondary`；`value` = `t.FontSize+1` / 580 / `TextStrong`；其余（`cardRow` 框架、44px 地板、分隔线、`rowLineGap()` 的 1px、文字列、空控制槽）逐字沿用 |
| 2 | `settingsui.go:2399`（`record == nil` 臂） | `a.row(c, copy.ApprovalRecord, copy.ApprovalRecordNone, func () {})` | `a.factRow(c, copy.ApprovalRecord, copy.ApprovalRecordNone)` |
| 3 | `settingsui.go:2405` | `a.row(c, copy.ApprovalRecordApprovedAt, approvalApprovedAt(record.At), func () {})` | `a.factRow(...)` 同文案 |
| 4 | `settingsui.go:2414` | 清单摘要行 | `a.factRow(...)` |
| 5 | `settingsui.go:2415` | 磁盘摘要行 | `a.factRow(...)` |
| 6 | 块标题行 `a.row(c, copy.ApprovalRecord, "", func () {})` | 它是块的标题，不是记录 | **不动** |
| 7 | `internal/settingsui/factrow_test.go`（新） | 无墨账 | `inkSpan` + `TestTheApprovalRecordPutsTheFactInTheEmphasisRung`（三条账 × 深浅两套）+ `TestDumpP13AKApprovalRecord`（opt-in 出图） |
| 8 | `docs/liquid-glass.md` **§6.10** | 无这一行 | 13-1..13-4 全 `已落地` |

### 行为契约零改动红线

- 高度链：行还是 `cardRow` 的 `floor = max(Space(11), 44)`，两行的字号之和不变 →
  实测行位置与列不移动（pin 里断言值仍在名字下方、两者 `x` 相同）。
- 键盘模型 / 会话管理 / 设置写回 / 命令接线：本轮零触碰。
- i18n：**零文案改动**，只换哪一档墨画哪个既有字符串；空记录臂的句子原样。
- 视觉：只换墨与字阶，不新增面、不新增线、不加影（§2 判据 2：E1 不画影）。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # HEAD 实测 3044 → 3045
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

SKIP：HEAD 实测 91 → 92（+1 = 本轮的 opt-in 出图）。测试函数：2247 → **2249**。

## mutation 计划

- **M1** `factRow` 换回旧墨（名字 strong emphasis / 值 muted body）→ 期望
  `TestTheApprovalRecordPutsTheFactInTheEmphasisRung` 两条账同时红（值不足一档 + 名字过响）。
- **M2** 两行顺序对调（值在上、名字在下）→ 期望顺序账红（值不在名字下方）。
- 两条都要 `md5sum -c` 逐字节还原 + `touch` 复跑一次绿。
