# P13-s 规格 · 禁用面：面还在，光熄了

## 意图

设置页的**禁用控件**不是「一块不能碰的面」，而是一块**几乎不存在的面**：因为禁用同时被
淡了两次。这一轮把它改回一次淡 + 一个「熄灯」的材质，让禁用态第一次有 z 位置。

## 为什么现在做（证据）

`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律转通用队列的「状态反馈」格；P13-q 做的是
这一格的**另一半**（按下档），这一轮做**禁用 / 只读**这一半。

根因不在设置页的调用点，而在**两个淡相乘**：

1. 框架的画笔给 `Disabled` 的子树乘 **0.5**：`/home/tar/go/pkg/mod/github.com/egoist/mygo@v0.7.3/ui/paint.go:257-259`
   ```go
   saved := p.opacity
   if e.flags&flagDisabled != 0 {
       p.opacity *= 0.5
   }
   ```
2. 产品在**外面**又叠了一道 `.Opacity(0.4)`：
   - `internal/settingsui/rows.go:648`（`busyActionButton`）
   - `internal/settingsui/config.go:289`（`configActions` 的 Restore defaults）
   - `internal/settingsui/settingsui.go:2958`（`configOpenToolRow` 的 Open configuration）

有效透明度 = 0.2。实测（真设置页地面，`extensionGlyphActionButton` 的禁用臂）：

| 外观 | 地面 | 静息面 | 静息顶内线 | 静息底内线 | 禁用面 | 禁用顶内线 | 禁用底内线 |
|---|---|---|---|---|---|---|---|
| 浅色 | 247 | 230（−17） | 232（+2） | 216（−14） | **243（−4）** | 243（= 面） | 240（−3） |
| 深色 | 15 | 39（+25） | 58（+19） | 31（−8） | **19（+4）** | 19（= 面） | 18（−1） |

也就是说：禁用胶囊的**面台阶只剩 4 级**、**底对侧只剩 1~3 级**——E1 的一对线双双掉进
噪声，面自己离地面 4 级。用户说的「平、糊、旧」「所有东西挤在同一个 z 平面上」，在状态
反馈这一侧的算术来源就是这张表。

## 落地项

位置 → 现状 → 应改成。

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/theme/theme.go` 新方法 `Tokens.DimFace` | 不存在；禁用面沿用 `LitFace`（顶 `LensRim` + 底 `LensEdge`） | **新材质**：只画底对侧 `LensEdge`，**不画受光边**。受光边是「这块面可以被碰」的记号（§3），禁用面熄灯 |
| 2 | `internal/settingsui/rows.go:475` `extensionActionFace` 的 `disabled` 臂 | `disabled` 只影响 hover / press 臂，材质仍走 `LitFace` | 面保持 `Control`（框架 0.5 之后正好是半个台阶），材质走 `tokens.DimFace(button)` |
| 3 | `internal/settingsui/rows.go:648` `busyActionButton` | `Disabled(true).Opacity(0.4)` | `Disabled(true)`（淡交给框架那一道 0.5） |
| 4 | `internal/settingsui/rows.go:584` `extensionGlyphActionButton` | 无 `disabled` 参数，内部恒传 `false` | 加 `disabled bool`，透传给 `extensionActionFace` |
| 5 | `internal/settingsui/config.go:286` Restore defaults | 先建面、再 `.Opacity(0.4)` | 把 `busy` 直接传进面；外面只留 `Disabled(true)` |
| 6 | `internal/settingsui/settingsui.go:2955` Open configuration | 同上（`!openable`） | 同上 |

**为什么不新增 token**：`ControlDim` 一开始是候选（`Control` 的一半）。实测否决——
`ControlDim` 自己还会再被框架的 0.5 乘一次，落到浅色 **243（−4）**，与改前逐像素相同。
「禁用面 = 静息面 × 0.5」这一档**已经由框架提供**，产品再补一档只会把两个淡叠成三个。

**为什么不把字也补回来**：字跟着那一道 0.5 走（有效 0.5，改前 0.2）。0.2 的墨在浅色档只有
43 级对比，低于 `references/user-preferences.md` P-5 的可读性下限；0.5 是 105 级，既读得清
又仍然明显「淡」。

## 预期实测（改后）

| 外观 | 地面 | 禁用面 | 台阶 | 顶内线 | 底内线 | 墨对比 |
|---|---|---|---|---|---|---|
| 浅色 | 247 | **238** | −9（改前 −4） | = 面（无受光边） | 231（−7，改前 −3） | 106 级（改前 29） |
| 深色 | 15 | **27** | +12（改前 +4） | = 面 | 24（−3，改前 −1） | 106 级（改前 29） |

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字不改**。
- `Disabled(true)` 仍然在（点击/焦点/指针语义由框架负责），`Cursor(ui.CursorDefault)` 仍然在。
- 几何（高度 / 圆角 / padding / 圆角胶囊形状）不动：本轮的 diff 只有墨。
- 三层 `Opacity(0.4)` 的删除**只**发生在这三处禁用臂上；`rows.go` 其余 `Opacity` 调用
  （`.settings-reset` 的 0.6/0.78、`plate.Opacity(0.62)` 等）不动。

## 门槛命令 + PASS 基线（本轮实测，改前）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 2970 PASS / 0 FAIL / 75 SKIP
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2970
GOOS=linux/windows/darwin go build ./...               → 过
测试函数数                                              → 2184
```

## mutation 计划（每条都要让某个 pin 变红）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | `extensionActionFace` 的 `disabled` 臂退回 `tokens.LitFace` | `TestTheDisabledActionPillKeepsAFaceAndLosesItsCrown` / `TestTheBusyActionPillWearsTheDimFace`（顶内线变成受光边） |
| M2 | `busyActionButton` 加回 `.Opacity(0.4)` | 同上两条（面台阶掉回 4 级、底内线掉到 1~3 级）+ `TestTheDisabledActionPillInkIsOneFadeNotTwo`（墨对比 317 → 129） |
| M3 | `configOpenToolRow` 加回 `.Opacity(0.4)` | 同上（A 条走的就是这条路径） |
| M4 | `DimFace` 改成空函数（不画底对侧） | 两条 face pin 的第三条判据（底内线） |
