# P13-p 规格：材质 census，不是函数名 census——设置页还剩三块「有面无线」的面

## 意图

`docs/liquid-glass.md` §6 在 P13-o 之后没有 `未开始` 行，按纪律转通用队列的「层级一致性」格。
P13-i / P13-l / P13-m 三轮都是「把某几处 `Lens` / `Control` 平涂退成 E1 的一对面」，
**但三轮用的都是函数名/单行 grep 的 census**：P13-i 数 `tokens.Lens(`，P13-m 数
`Border(1,` 与 `tokens.Control)` 是否在**同一行**。于是这一族还剩三处漏网：

| # | 位置 | 现状 | 为什么它该是 E1 |
|---|---|---|---|
| A | `internal/settingsui/rows.go` `sessionIconButton` 的 `Hovered()` 臂（28u 方格，Sessions 页） | `Border(1, tokens.ControlEdge).Background(tokens.ControlHover)`，**没有一对线** | 它的三个同胞都已经戴上了：`extensionIconButton`（30u，rows.go:549）、`paramIconButton`（28u，parameditor.go:274）、页头 `headerButton`（28u，settingsui.go:1199）。同一个应用里同一枚 28u 悬停小格，四块里三块有受光边、一块没有 |
| B | `internal/settingsui/savealert.go` `shortcutRejectedAlert`（Shortcuts 页顶的拒绝条） | `Background(tokens.Control).Border(1, tokens.ControlEdge)`，**没有一对线** | 它与 `noticeBand` 的非危险臂（§6.2b 第 9-14 行）是同一个形状：一条 `Control` 平涂 + 一圈 `ControlEdge` 画框，坐在页面的地面上。那条已经戴上了，这条没有 |
| C | `internal/settingsui/pluginsheet.go` `sheetActionButton` 的**中性**臂（28u 胶囊） | `Border(1, edge).Background(plane)`，**没有一对线** | 弹层里的一切是 E1（§2 判据 1）。同族的胶囊 `extensionActionFace`（9-1）、`barButton`（9-5）、`uninstall` 动作胶囊（9-6）都戴上了 |

**根因不是这三处，是 census 的查询方式。** 本轮除了补这三处，还要把「扫不到的 census」
换成**按函数体扫材质**的守卫（`internal/settingsui/facecensus_test.go`，go/ast 取函数源码文本）：
一个函数里只要同时出现「`Control` / `ControlHover` 的面」与「`ControlEdge` 的边」，
就必须出现 `LitFace` / `Lens` / `LensInset` / `InsetShadow` 之一，否则必须在允许表里
写明为什么它**故意**不戴（噪点 / 环已占住那一行 / 是槽不是面）。多行写法不再能躲。

## 为什么现在做（证据）

- `docs/liquid-glass.md` §6 无 `未开始` 行（P13-g 收口、P13-h/j 之后仍无）。
- `.pending-bugs.md` P13-o「仍待」第 2 条点名下一轮做 `settingsui` 的细节打磨。
- 任务书「下一轮 = 细节打磨（`settingsui` 表面）」。
- 三处都是**已存在**的面，只差两条 1px 内线，不改几何、不改交互。

## 逐条落地项

### A. `rows.go` `sessionIconButton`（Sessions 页 28u 悬停方格）

- 位置：`internal/settingsui/rows.go:789-810`（`sessionIconButton`，`case button.Hovered():` 臂）。
- 现状：`button.Border(1, tokens.ControlEdge).Background(tokens.ControlHover)`，无内线。
- 应改成：同一臂里追加 `tokens.LitFace(button)`（`Border`/`Background` 不动）。
- 静息臂不变（`Border(1, ui.Transparent)` + 透明面）：受光边只在指针把面点亮时出现，
  与 `extensionIconButton` / `paramIconButton` / `headerButton` 完全同一写法。

### B. `savealert.go` `shortcutRejectedAlert`（Shortcuts 页的拒绝条）

- 位置：`internal/settingsui/savealert.go:169-180`。
- 现状：`ui.Box(c).FillWidth().Padding(...).Radius(RadiusMD).Background(tokens.Control).Border(1, tokens.ControlEdge)`。
- 应改成：`tokens.LitFace(ui.Box(c)…)`（外圈 `ControlEdge` 是画框，`LitFace` 只加两条内线）。
- 注意：它**不是** E2（不浮、无影），与 `noticeBand` 非危险臂同一档。

### C. `pluginsheet.go` `sheetActionButton` 的中性臂

- 位置：`internal/settingsui/pluginsheet.go:365-395`。
- 现状：`button.Border(1, edge).Background(plane)`，`danger` 臂 `plane = ui.Transparent`。
- 应改成：`if !danger { tokens.LitFace(button) }`。
  **danger 臂故意不戴**：它的面是透明的（`plane = ui.Transparent`），身份线是那条
  45% 警告发丝——在一块透明的面上画一条 0.09 的白线就是 §1 禁令 3 的「类玻璃边」，
  而且它没有面可以「站起来」。与 §6.2b 第 9-14b 行（警告环占住那一行）同一个理由：
  一个面只说一句话。

### D. 守卫：`internal/settingsui/facecensus_test.go`

- `go/ast` 解析 `internal/settingsui` 的全部非 `_test.go` 文件，取每个 `FuncDecl` 的源码文本。
- 判据：`hasFace && hasEdge && !hasPair` → 必须出现在 `censusAllow` 里（每条带理由），
  否则测试红。`hasPair` = `LitFace(` / `.Lens(` / `LensInset(` / `InsetShadow(` 任一。
- 允许表只收「故意不戴」的面，每条一句话写清是哪条合约行；新面不戴就必须改这张表。
- 反向 pin：允许表里的函数如果**开始**戴一对线，也要报错（表要跟着变），避免表变成
  「历史垃圾场」。

## 行为契约零改动红线

只改墨与面：高度链、行高、padding、圆角、键盘模型、设置写回、i18n 文案、命令接线
一个字节不动。三处都是**追加**两条 inset 1px 线，不改任何布局属性。

## 门槛命令 + PASS 基线（实测当前值）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（P13-o 实测）：PASS **2948** / FAIL 0 / SKIP 73 / 函数 **2170**。

## mutation 计划（≥2 条，逐条说明会让哪个 pin 变红）

| # | 注入的缺陷 | 应该变红的 pin |
|---|---|---|
| M1 | A 处去掉 `tokens.LitFace(button)` | `TestTheSessionIconButtonWearsTheLitPair` |
| M2 | B 处把 `LitFace` 换成 `Lens`（四层环回来） | 同上族 + `TestEveryFaceWithAnEdgeWearsTheLitPair` 的反向 pin（如果它戴的是环） |
| M3 | C 处给 danger 臂也戴上 `LitFace` | `TestThePluginSheetDangerPillKeepsItsOwnLine`（反面 pin） |
| M4 | 在 `packageChip` 里删掉允许表条目 | `TestEveryFaceWithAnEdgeWearsTheLitPair` |
| M5 | 把 `sessionIconButton` 的 `LitFace` 换成 `InsetShadow(0,1,0,0,ControlEdge)`（顶线是暗线） | `TestTheSessionIconButtonWearsTheLitPair`（顶亮判据） |
