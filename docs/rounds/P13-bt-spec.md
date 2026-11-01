# P13-bt · 规格：只读行的墨——数据不当标题，标题不当值

> 总目标（2026-10-30）：细节打磨 + 轻便液态玻璃 + 空间感。§6 差距表已无 `未开始` 行，
> 选题回通用队列的「状态反馈」：**只读面**（P13-ak / P13-an 的第三、第四处同族）。
> 合约新增 `docs/liquid-glass.md` §6.38。

## 一、意图

集成页上两处**没有控件的行**还在戴「设置行」的强调墨（`rowTitle`：`t.FontSize+1` / 580 /
`TextStrong`）——于是一个路径和一个块标题，跟这一页真正的答案一样响：

| 表面 | 现状（实测，720 宽，深浅同值） |
|---|---|
| Detected 列表里的路径行 | `/usr/bin/ghost` ink span **208**，与它上面工具名 `Ghost Tool` 的 **208** 逐级相同；比描述 **121** 响 87 级 |
| 审批记录块的标题行 | `Approval record` ink span **202/198**，与它下面三个**值**（时间/清单摘要/磁盘摘要）的 **202/198** 逐级相同，比这些值的**名字**（122/115）响 80 级 |

`inkSpan` = 文本盒内最亮与最暗像素的 luma 差（= 面到墨的距离，同 `factrow_test.go`）。

**为什么现在做**：`P13-ak` 已经把「记录的名字不当标题、值占强调档」钉成了像素事实，
`P13-an` 又把 About 页的路径值钉成同一条；这两处是同一族里最后两个**反着戴**的行——
一个数据戴着标题的墨，一个块标题戴着答案的墨。做完之后，集成页上「谁响」与
「谁是答案」是同一句话。

## 二、判据（写进 §6.38）

1. **强调档只留给答案**。一块只读内容里，`TextStrong` / `+1pt` / 580 属于**值**；
   名字、块标题、路径、旁注都不占它。
2. **块标题 = 组标签那一档**：`tokens.Caption` + `FontWeight(620)` + `TextSecondary`——
   与本块里已有的 `Enforced` / `Disclosure` 组标签（`settingsui.go:2477` 一带）**同一个函数值**，
   层级靠**行框架**（44px 行 + 规则线）区分，不靠再加一档墨。
3. **路径 = 行内已有的「值是数据」那一档**：mono（`deepLinkValueFont`）+ `Caption` +
   500 + `TextMuted`，与 `rowLine{Mono: true}`（会话页的 cwd）**同一个函数值**。
4. **只换墨，不换盒**：行框架、44px 地板、分隔线、1px 行距、文字列、卡片高度逐像素不变。

## 三、逐条落地

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go:5859`（`detectedSection` 的 `below`） | `a.row(c, tool.Path, "", func() {})` → `rowTitle` 强调档，span 208 | `a.dataRow(c, tool.Path)`（新 helper，rows.go）：`cardRow` + 一行 mono `Caption`/500/`TextMuted`，`Row.FillHeight().AlignItems(ui.Center)` 单行居中（同 `keyValue` 的做法） |
| 2 | `internal/settingsui/settingsui.go:2440`（`approvalDetail` 的块标题） | `a.row(c, copy.ApprovalRecord, "", func() {})` → span 202/198 | `a.labelRow(c, copy.ApprovalRecord)`（新 helper）：`cardRow` + `Caption`/620/`TextSecondary` |
| 3 | `internal/settingsui/settingsui.go:2477`（`ApprovalRecordPermissions` 行） | 同一个「没有控件的组标签」形状，却戴强调档 | `a.labelRow(c, copy.ApprovalRecordPermissions)`，与 #2 同一档；`staleBelow` 照旧作为 `below` 传入 |

**故意不做（附实测理由）**：`settingsui.go:5373` 的 `a.row(c, "", copy.PermissionBoundary, func() {})`
——那句「这些开关什么都不隔离」的说明 span **121**，而它下面真正的开关行 **208**：
散文比控件轻 87 级**是对的**（说明不是答案，开关才是）。这条不是反着的层级，不动。

## 四、行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线**零改动**。本轮只改墨与字重，
不新增控件、不加 `Disabled`、不改任何文案 key，不触碰 `detectedAction` 的三条臂。

## 五、门槛（实测基线，开工时实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿
--- PASS 计数                → 基线 3210
--- SKIP 计数                → 基线 117
测试函数数                   → 基线 2365（`git grep -c '^func Test'`）
三平台 build                 → 全过
```

## 六、变异计划（≥2 条，每条说明会让哪个 pin 变红）

| # | 注入 | 应红的 pin |
|---|---|---|
| M1 | `dataRow` 的 `TextMuted` → `TextStrong`（回到标题墨） | `readonlyink_test.go` 的「路径必须比工具名轻 ≥ 60 级」 |
| M2 | `labelRow` 的 `Caption`/620 → `t.FontSize+1`/580/`TextStrong`（回到标题墨） | 同文件「块标题不得与值同档（差 ≥ 40 级）」 |
| M3 | `dataRow` 去掉 `Row.FillHeight().AlignItems(ui.Center)` | 同文件「路径那一行与工具名同一行盒的光轴」（y 偏移 > 1px） |

## 七、出图

`FLOTER_P13BT_DUMP=1 go test ./internal/settingsui/ -run TestDumpP13BT` →
`/tmp/floter-P13-bt-{before,after}-{dark,light}.png`（Detected 列表 + 审批记录块同框）。
