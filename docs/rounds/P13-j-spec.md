# P13-j 规格 · toast 的动作 chip：最后一个 `未开始` 的面，与它那条唯一的边界

> 轮号：**P13-j**｜表面：`shell/notices.go` 的 toast 动作 chip（`.app-toast__action`）
> 单一来源：`docs/liquid-glass.md` §6.6 第 6-3 行（`未开始`）＋ §6.2b 第 9-12 行
> 前置：P13-i（设置页十处 `Lens` → `LitFace`，把这一行留成「下一张表面」）

## 1. 意图（一句话）

把 toast 里那枚动作 chip 从**四层透镜**（rim + 宽 sheen + 暗线 + 四边 `ControlEdge` 环）
退成 **E1 的一对面**（顶 `LensRim` + 底 `LensEdge`，无环），并**明确回答**它在 §6 里挂着
的那个问题：它没有自己的 `Border`，环就是它唯一的边界——**不补边框**，因为面差本身
已经把它从 toast 的 `Float` 面上分出来了（本轮量出来给证据），而补 `Border(1)` 会把
padding box 缩 1px（`insetShadow` 从 padding edge 算起），是几何改动，越出「只改墨与面」。

为什么现在做：P13-i 清点全仓 `Lens` 的 12 个调用点，10 个已退，剩下 2 个——本行
（`未开始`）与 `control.SegmentedIndex`（`故意不做`，accent 环就是它的身份线）。
这一行是 §6 表里**唯一**剩下的 `未开始`；做完它，视觉这一支才算真的收口。

## 2. 证据（本轮动手前实测，行号已核）

- `internal/shell/notices.go:433-441`：chip 是 `ui.Row(c)` + `Height(t.Space(5.5))`（22u）
  + `Padding(0, t.Space(2))`（8u）+ `Radius(RadiusSM)` + `Background(tokens.Control)`，
  **没有 `Border`**；`notices.go:440` 是 `tokens.Lens(action)`。
- `internal/theme/theme.go:1056-1061`：`Lens` 四层按 rim → sheen → edge → 四边环追加，
  后加的盖在上面；`insetShadow` 的 hole 从 padding edge 算起，于是 `spread=1` 的环与
  `y=1` 的 rim **落在同一个像素行**——P12-a 抬到 0.09 的受光边被压在环下面。
- 上一轮同类（P13-h / P13-i）实测：`Lens` 的顶内线在浅色档是**暗线**（比面暗 9 级），
  深色档是 0.09 白 + 0.07 白叠在同一行。
- toast 的面是 `Float`（E2），chip 坐在它上面 → 按 §2 判据 1，chip 是**这一层里的 E1**，
  不许有影、不许再开一个浮层。

## 3. 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| J-1 | `notices.go:440` | `tokens.Lens(action)` | `tokens.LitFace(action)`；注释重写成「E1 的一对面 + 为什么不需要边框」 |
| J-2 | `internal/shell/glasscensus_test.go:52-58` | `.app-toast__action` 条目 `marker: "tokens.Lens(action)"`，理由写着「没有孤立渲染」（早已过期：`notices_test.go` 就能渲染带动作的 toast） | 翻成 `dropped`，`test:` 指向新 pin |
| J-3 | 新增 `internal/shell/toastactionface_test.go` | 无 | 像素 pin：①顶内行亮于面 ②底内行暗于面 ③左右两侧列 = 面（无环）④块外仍是 toast 面（E1 无影）⑤**面差**：chip 的面与 toast 的面至少差 4 级（这是「不补边框」的全部理由）＋ opt-in dump |
| J-4 | `docs/liquid-glass.md` §6.6 6-3 / §6.2b 9-12 / §6.6 末尾那段「下一轮的第一条就是 6-3」 | `未开始` | `已落地`（附实测像素），并把「表里没有 `未开始` 行」这句话改成事实 |
| J-5 | `internal/settingsui/lensface_test.go` 里 `TestNoSettingsSurfaceControlCallsTheFourLayerLens` 的注释 | 说「树里还剩两个调用点」 | 改成「只剩 `control.SegmentedIndex` 一处（9-11，故意不做）」 |

**不做**：不给 chip 补 `Border`（几何改动，且 §1 禁止画框分层）；不动 `control.SegmentedIndex`。

## 4. 行为契约零改动红线

高度链（chip 仍是 22u）、toast 的入场/落位/折叠、键盘模型（chip 的 `Focusable` 与
`Clicked` 顺序）、i18n（`toast.Action` 原样）、命令接线（`OnAction` + `CloseToast`）
一个字节都不动。本轮只把一处材质调用从四层换成一对面。

## 5. 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                       → 必须空
go vet ./...                                → 必须净
GOOS=windows go vet ./...                   → 必须净
GOOS=darwin  go vet ./...                   → 必须净
go test -count=1 ./...                      → 全绿
go test -count=1 -v ./... | grep -c "--- PASS" → 只许涨
GOOS=linux/windows/darwin go build ./...    → 三平台过
```

**基线（本轮开工前实测）**：PASS **2917** / FAIL **0** / SKIP **67** / 测试函数 **2153**。
预期本轮：PASS +（2 外观 × 1 pin + 1 父测试 + dump 的 SKIP）。

## 6. mutation 计划（≥2 条，每条说明让哪个 pin 变红）

| # | 注入 | 预期红 |
|---|---|---|
| M1 | `notices.go` 换回 `tokens.Lens(action)` | J-3 的③（左右侧列 = 环的墨）＋ census 的 dropped 条目 |
| M2 | `theme.Tokens.LitFace` 删掉底对侧那行 | J-3 的②（底内行不再暗于面） |
| M3 | chip 加一条 `Shadow(0, 4, 12, 0, ShadowAmbient)` | J-3 的④（块外不许有影） |
| M4 | chip 的 `Background(tokens.Control)` 删掉（只剩 toast 的面） | J-3 的⑤（面差归零 → 没有边界） |
| M5 | chip 加 `Border(1, ControlEdge)` | J-3 的⑤/③（面缩 1px，侧列读到边框的墨） |
