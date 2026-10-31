# P13-aq 规格：禁用面只淡一道——同一条链上最后四枚「两道路淡」

## 意图（一句话）

同一个「不可用」状态，全仓还有四处被**两次**淡化（产品自己 `.Opacity(0.x)` × 框架给
`Disabled` 子树的 `0.5`），面被洗成半透明背景，字被洗到读不出；这一轮把它们收成一道淡，
并让「不能碰」的面**熄灯**（不戴受光边）。

为什么现在做：P13-s / P13-t / P13-u 已经把这一族的三处（`extensionGlyphActionButton`、
锁定表单的字段与多选框、卸载对话框在飞的面）改成「一道淡 + `DimFace`」，并写下判据
「两道淡乘起来不是设计，是一道淡画两遍」。同一判据下仍有四处留着旧写法，属**同族未收口**。

## 逐条落地项（行号已按 HEAD `02aaaab7` 核对）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/rows.go:611`（`extensionIconButton` 的 `case disabled`） | `button.Opacity(0.42).Cursor(ui.CursorDefault).Disabled(true)` → 有效 0.21 | 只留 `.Cursor()` + `.Disabled(true)`；褪色由框架那一道 0.5 给 |
| 2 | `internal/settingsui/rows.go:958`（`sessionKillConfirm` 的 disabled 臂） | `pill.Opacity(0.4)…Disabled(true)` → 0.2，且 `tokens.LitFace(pill)` 在臂外，禁用时**仍然点着受光边** | 去掉 `.Opacity(0.4)`；受光边只在 `!disabled` 时画（§3：受光边是「这块面可以被碰」的记号） |
| 3 | `internal/settingsui/rows.go:573`（`extensionPrimaryButton`） | 无 `disabled` 参数，冠线 `InsetShadow(0,1,0,0,LensRim)` 无条件画 | 加 `disabled bool`：为真时画 `.Disabled(true).Cursor(Default)` 且**不画冠线**；`config.go:343` / `settingsui.go:6635` 两处调用点把 flag 传进来、自己不再 `.Disabled(true)` |
| 4 | `internal/settingsui/config.go:345`（配置页 Save） | `save.Disabled(true).Opacity(0.4)` → 0.2 | 由 #3 接管：`a.extensionPrimaryButton(c, saveLabel, saveDisabled)` |
| 5 | `internal/settingsui/settingsui.go:3894`（`updateButton` 的 disabled 臂） | `button.Opacity(0.6)…Disabled(true)` → 0.3，冠线仍然点着 | 去掉 `.Opacity(0.6)`；冠线只在 `!disabled` 时画 |

**不在本轮内的同族（写进报告与 `.pending-bugs.md`）**：`uninstall.go:442` 的
`card.Opacity(0.6)`（fixed 组件的**卡片**，卡片自身不是 `Disabled`，褪色落在整棵子树上，
其中还套着一个 `Disabled` 的勾选框）——它要连带回答「根上禁用还是逐件禁用」，属另一片。

## 行为契约零改动红线

不动：键盘模型、高度链、会话管理、设置写回、i18n 文案、命令接线、命中范围。
`Disabled(true)` 保留在每一处（命中拒绝不变）；`Cursor` 语义不变；`Scale` / `Transition`
不变。只改墨：褪色次数与受光边。

## 门槛与 PASS 基线（实测，HEAD `02aaaab7`）

```
gofmt -l cmd internal                                # 空
go vet ./... / GOOS=windows|darwin go vet ./...       # 净
go test -count=1 ./...                                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3056
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 97
测试函数（只数 cmd/ + internal/）                       # 2263
GOOS=linux|windows|darwin go build ./...              # 三个都过
```

## mutation 计划（每条会让哪个 pin 变红）

- M1：把 #1 的 `.Opacity(0.42)` 加回去 → `TestTheDisabledIconKeepsItsInk` 红（字被洗到
  < 100 级对比）。
- M2：把 #3 的冠线改回无条件 `InsetShadow(0,1,0,0,LensRim)` → `TestTheDisabledPrimaryCTA-
  TurnsItsLightOff` 红（冠线行亮于面）。
- M3：把 #2 的 `LitFace` 改回臂外无条件 → `TestTheDisabledKillPillTurnsItsLightOff` 红。
- M4：把 #5 的 `.Opacity(0.6)` 加回去 → `TestTheDisabledUpdateCTATurnsItsLightOff` 红
  （面与地面差 < 门槛）。
