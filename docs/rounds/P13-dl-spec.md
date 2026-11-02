# P13-dl 规格：权限边界那句散文被切在「不」字上

**意图一句话**：工具编辑器的权限边界说明是这段披露里唯一诚实的那半句，而它一直是**一行被截断
的副标题**；让它按散文排（换行、整宽、与它解释的块同列），而**墨一个字节都不换**。

**为什么现在做**：P13-dk 的「仍待」第 1 条（`PermissionBoundary` 与工具编辑器的 `LocalToolId`
两块只读面「盒子仍没人量过，先量再改」）+ 任务书「下一格从状态反馈的另一半（禁用 / 只读面）挑，
先量再改」。

## 先量（改前实测，`toolFormEdit`：720×2200、Integrations → Edit… → 展开 Advanced，英文）

| 面 | 盒子（浅/深同值） | 墨（浅 / 深） | 判读 |
|---|---|---|---|
| 集成 id 的名字 `Integration ID` | `x=201.8 y=247.6 w=65 h=15` | 122 / 115 | caption 档，**对** |
| 集成 id 的值 `local.abc` | `x=636.8 y=246.1 w=53 h=18` | 202 / 198 | emphasis 档，落在行尾、可选中，**对** |
| 权限边界句 | `x=215.8 y=951.3 w=448 h=17` | 101 / 94 | **一行**：墨到 375 列就停，尾列只有 **2px** 高 = 框架的省略号 |

也就是说：句子的后半段 `These controls do not create a sandbox.` **从来没被画出来**。
一段披露里「这些开关不创建沙箱」这句话被切在 `These controls do not` 上，读起来正好是反的。
同一块里它上面还压着一行**空的 emphasis 标签行**（`a.row(c, "", note, …)` 的第一个参数是 `""`），
那行 19px 什么也不说，句子则因为嵌在 `below` 列里比块标题又多缩进 14px（215.8 对 201.8）。

## 落地项

1. `internal/settingsui/rows.go` 新增 `cardProse(c, text)`：卡片自己的**散文**行 ——
   `body` 步长 / 500 / `t.TextMuted` / 整宽 / 不 `SingleLine()`，上下各 `Space(2.25)`。
   这是本仓已有的散文档 `sectionHint`（`rows.go:200-208`）的同一套墨，只是住在卡片里。
2. `internal/settingsui/settingsui.go:5559` `toolPermissionFields`：
   `a.row(c, "", copy.PermissionBoundary, func() {})` → `a.cardProse(c, copy.PermissionBoundary)`。
3. 新增 `internal/settingsui/readonlytools_test.go`：三条像素 pin（散文读完 + 不再是行 + id 行
   现状），加一条 opt-in 出图（`FLOTER_P13DL_DUMP=1`，`FLOTER_P13DL_TAG` 用来跑改前/改后两趟）。
4. `docs/liquid-glass.md` 新增 §6.81。

## 改后实测（同一夹具、同一读法）

| 面 | 盒子 | 墨 | 判读 |
|---|---|---|---|
| 权限边界句 | `x=201.8 y=931.3 w=487 h=34` | **101 / 94（不变）** | **两行读完**，尾列 **5px** = 真字形，与块标题同列 |
| 集成 id 的名字 / 值 | 逐像素不变 | 122/115、202/198 不变 | 这一块本来就对，**故意不改** |

块内高度账（`Permissions & audit` 标题底 → 第一个开关标签顶）：**74 → 71**，
即多画了 17px 的正文、少了一行 19px 的空标签，整块矮 3px。

## 行为契约零改动红线

高度链（`cardRow` 地板、`rowLineGap` 1px、`sectionGroup` 的 `Space(2)`）、键盘模型、
会话管理、设置写回、i18n 文案（`PermissionBoundary` 中英文逐字不动）、命令接线：
**零改动**。这一轮只改「一句散文怎么排」。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                     → 空
go vet ./... / windows / darwin           → 净
go test -count=1 ./...                    → 全绿
--- PASS 计数                             → 3318（基线 3315）
--- SKIP 计数                             → 152（基线 151，+1 = 本轮 opt-in dump）
--- FAIL                                  → 0
测试函数                                  → 2478（基线 2474）
GOOS=linux/windows/darwin go build ./...  → BUILD-OK
```

## mutation 计划

- **M1**：把 `a.cardProse(...)` 还原成 `a.row(c, "", copy.PermissionBoundary, func() {})`。
  预期变红：`TestThePermissionBoundaryNoteIsReadEndToEnd`（同列 Δ14、盒高 17、尾列 2px × 深浅）
  与 `TestThePermissionBoundaryNoteIsNotARow`（间距 74 ≠ 71 × 深浅）。
- **M2**：给 `cardProse` 的 `ui.Text` 加回 `.SingleLine()`（散文又被截断，盒子不动）。
  预期变红：`TestThePermissionBoundaryNoteIsReadEndToEnd` 的「盒高 17」「尾列 2px」两条 × 深浅。
- 还原后用 `md5sum` 逐字节核对，`touch` 再复跑一次（避免测试缓存）。
