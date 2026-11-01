# P13-bp 规格 · 插件窗口列表臂的行盘：这一族最后一块没戴 E1 一对线的面

## 意图

`docs/liquid-glass.md` §2 把 E1 写死成「面 + 顶受光边 + 底对侧暗线」。§6.1 第 1-2 / 1-8、
§6.2 第 2-3、§6.2b-2 第 9-16 把**启动器的结果行、28u 图标板、设置页分组卡与命令块**一块块
收进这条判据。`internal/shell/detach.go` 的 `detachedList`（`plugin-window__row` 行盘）是
同级里最后一块没进过账的：它在 `tokens.Control` 面上直接 `Children`，**一条线都没有**。

**为什么现在做**：这是「状态反馈 / 只读面」这一格剩下的最实的一条，而且它同时是
P13-aa「仍待」明写的那条缺口——「census 还缺一条**反向**判据：『面』出现在
`Hovered()` / `Pressed()` 臂里就必须有一对线。这一族现在只能靠人肉找。」

## 先量（改前，`ui.Tester`，720×460，窗口自己的地面 `tokens.Opaque`）

| 外观 | 地面 | 冠线 | 面 | 底内线 | 左列 | 右列 |
|---|---|---|---|---|---|---|
| 浅 | 246 | **229** | **229** | **229** | 229 | 229 |
| 深 | 20 | **44** | **44** | **20** | 44 | 44 |

三读同值 ⇒ 这块盘**除了「面」以外什么都没有**。浅色档它比地面暗 17 级（一行灰块），
深色档它的面 44 与地面 20 差 24 级但四条边一条线都没有——`LitFace` 的两条 1px 内线
一条都不存在。

## 逐条落地项

| # | 位置（改前行号已核） | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/shell/detach.go:671-673`（`detachedList` 的 `plate`） | `ui.Column(...).FillWidth().Gap(Space(0.5)).Padding(Space(2), Space(2.5)).Radius(RadiusSM).Background(tokens.Control)`，无任何内线 | `plate = tokens.LitFace(plate)`（顶 `LensRim` + 底 `LensEdge`）。**几何一个字节不动**：`LitFace` 只加两条 1px 内线，padding box 不变 |
| 2 | `internal/ci/flatface_contract_test.go`（新） | 无 | 面级 census：一个函数体同时有 `Background(<面 token>)` + `Padding(` + `Radius(` 就必须有一条线（`LitFace(`/`DimFace(`/`.Lens(`/`LensInset(`/`InsetShadow(`/`Border(`）。补 P13-aa「仍待」明写的反向判据 |
| 3 | `internal/shell/plugindowlistface_test.go`（新） | 无 | 像素 pin：顶内线是受光边、底内线是暗线、**两侧回到面本身**、面仍离地面 ≥6 级；深浅两套。第二行是 opt-in 出图 |

## 行为契约零改动红线

高度链（`detachedRowPadY`/`detachedRowPadX`/`detachedRowGap`/`detachedBodyInset`）、
键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字节不动**。
`LitFace` 是 `InsetShadow(0, ±1, 0, 0, …)`，只改两条内线像素，不改盒子。
列表的 `Selectable()` 与 copy-on-select、分页触发的几何读法（`detached.scroll.MaxY`）
都不碰。

## 门槛命令 + PASS 基线（本轮前实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 3203（基线）
go test -count=1 -v ./... | grep -c -- "--- SKIP"   → 114（基线）
GOOS=linux/windows/darwin go build ./... → 三平台过
```

测试函数基线 **2355**。

## mutation 计划（≥2 条）

1. **M1** 删掉 `plate = tokens.LitFace(plate)`（回到改前）→ `plugindowlistface_test.go` 的
   「顶内线不是受光边」「底内线不是暗线」两条必红。
2. **M2** `LitFace` → `Lens`（四层透镜，环画在最后）→ 「左/右列不许有『线』」两条必红
   （环压住 rim、并在左右两列各画一条边）。
3. **M3** `LitFace` → 裸 `InsetShadow(0, 1, 0, 0, LensRim)`（只有顶边的「半对」，
   P13-aa 在电源 execute 键上判过的形状）→ 「底内线不是暗线」必红。
4. **M4** 把 `plate = tokens.LitFace(plate)` 从 `detachedList` 里删掉后跑 `internal/ci` 的
   `TestNoPaddedPaneIsFlat` → 必红并点名 `shell.detach.go:<行>`（census 的反向回测）。

每条都要 `md5sum -c` 逐字节还原 + `touch` 后复跑绿。
