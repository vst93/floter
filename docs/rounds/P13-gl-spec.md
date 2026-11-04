# P13-gl 规格 · E2 的左/右竖边（第三对边 + 那条死 token）

> 轮次 P13-gl｜合约 §6.157｜上游 P13-gk（§6.156，E2 下沿）
> 本支总目标：细节打磨 + 轻便液态玻璃 + 空间感（**这一轮为什么更轻/更有空间感**：
> 它证明浮层的四条边里，两条竖边**已经**被影与线撑住了，因此不需要再加一条亮线——
> 少一条墨，盒子读起来更薄，而层级不减。）

## 1. 意图

E2（浮层）的**顶边**（§6.155）与**下沿**（§6.156）都已进过帧。第三对边——**左/右竖边**——
从未量过；同时 `theme.Tokens.EdgeHighlightX`（浅 `0.6` / 深 `0.04`，「两侧的耳语描边」）
**全仓一次都没被画过**（`git grep EdgeHighlightX internal` 只有 `theme.go` 的定义与赋值）。
本轮回答：竖边靠什么分离？那条耳语该不该画？

为什么现在做：P13-gk 报告「仍待」第 4 条亲自点名这一格；§6 差距表没有 `未开始` 行；
用户实测无新条目。

## 2. 证据（动手前读代码确认的现状，行号为本轮实测）

| 位置 | 现状 |
|---|---|
| `internal/shell/notices.go:369-370` | toast 盒子：`Padding(Space(2), Space(3))`、`Radius(RadiusMD)`、`Background(Float)`、`Border(1, InputStroke)` |
| `internal/shell/notices.go:397-401` | 两条臂各一行 `InsetShadow(0, 1, 0, 0, tokens.EdgeHighlight)`：**冠线只在上头** |
| `internal/theme/theme.go:290` | `EdgeHighlightX` 字段存在；`theme.go:812` 浅 `0.6`、`theme.go:931` 深 `0.04` |
| `git grep -n EdgeHighlightX internal` | 除 `theme.go` 外**零调用点** → 死 token |
| `internal/shell/floatedge_test.go:55` | `floatEdgeFrame` 给出 before/after 两帧、盒子 `top`、✕ 的 rect（本轮复用） |
| `internal/shell/floatbottom_test.go` | §6.156 的下沿量尺（本轮复用它的 `floatBottomArm` 做交叉校验） |

## 3. 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/shell/floatvertical_test.go`（新，2 个函数） | 竖边无任何帧内读数 | 量尺 + 字面量 pin（左右 × 深浅共 8 组）+ opt-in dump |
| 2 | 同上 | 「一根 `InputStroke` 两条边一个数」没人守 | 纸面侧 `(线 − 面)` 与同帧下沿的 `(border − face)` 差 ≤ 4 |
| 3 | 同上 | 竖边分离者无说法 | `abs(线 − 面) ≥ 12` + 符号随地面；纸面侧浅 `地面 − 影 ≥ 24`、深 `≤ 8` |
| 4 | 同上 | 侧边该不该有受光边 | `abs(inner − 面) ≤ 2`；`EdgeHighlightX` 候选实测 +4/+9 → **故意不做** |
| 5 | 同上 | 左右两个地面会被误读 | 两侧影拿走「右 > 左」（浅 61 > 31、深 8 > 3） |
| 6 | `docs/liquid-glass.md` | §3 没提死 token；§6 无这一条 | §3 加一段「死 token，已实测」；新增 §6.157（含反判表与 157-1..6 台账） |
| 7 | 产品代码 | — | **零改动**（`internal/shell/notices.go` 只在反判时临时注入，已逐字节还原 md5 `021aa9cf…`） |

## 4. 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：本轮**一个都没碰**。
只新增一个 `_test.go` 与两处文档；产品代码字节零改动。

## 5. 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3484（P13-gk 实测）→ 本轮 +2
GOOS=linux/windows/darwin go build ./...
```

测试函数数基线 **2648**（P13-gk）→ 本轮 +2（pin + opt-in dump）。

## 6. mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `notices.go` 两条臂各加 `InsetShadow(±1, 0, 0, 0, EdgeHighlightX)`（即反判候选） | 157-1 字面量（`inner` 249→253 / 24→33）、157-4 |
| M2 | 拿掉 toast 的 `Border(1, InputStroke)` | 157-1 字面量、157-2（竖线 −34 vs 下沿 −81）、157-3（深色「no vertical line」） |
| M3（备） | `theme.ElevThrow` 宽影/接触影改值 | 157-1、157-3（影拿走的级数） |
