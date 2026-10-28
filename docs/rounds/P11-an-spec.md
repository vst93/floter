# P11-an 规格 · 浮动层的投影跟着框的透明度缩放（`--elev-shadow-scale`）

## 意图

同一个透明度滑块位置，参考只给一个观感：`main:src/styles/base.css:178` 把**每一档
elevation 的投影**都乘上 `--elev-shadow-scale = calc(0.45 + 0.9 * (1 - var(--main-opacity)))`，
框越薄、影子越轻。本仓把 **scale = 1 那一档的字面值**抄进了四个浮动层，于是滑块只动
填充、不动投影——`internal/launcher/launcher.go:2965-2972` 与 `internal/terminalui/…`
的注释甚至写着「a function of the frame's own alpha, so its 0.2 is scaled here exactly
as the terminal note's is」：那句话描述的耦合从来没有实现过。用户 2026-10-28 的方向是
「界面、质感」与「轻」，这条正是「薄框配重影」。

## 为什么现在做

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩
`⌘K`＝拍板）→ 上一轮「仍待」第 1 条（`AccentEdge` 3:1＝拍板）→ **界面质感**。这一档里
不需要拍板、且有代码自证的一条：字面投影 + 注释承诺的缩放不存在。

## 逐条落地

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/theme/elevation.go` | 不存在 | 新增 `ElevScale(opacity uint8) float32`（参考公式，0..100 夹取）、`ElevThrow(e, t, opacity)`（`--elev-3` 两层：10/26 @0.30、1/2 @0.20，各乘 scale）、`ElevContact(e, t, opacity)`（接触边那一层） |
| `internal/launcher/launcher.go:2973`（警告状态行） | `Shadow(… ui.RGBA(0,0,0,0.2))` | `theme.ElevContact(feedback, t, s.MainOpacity)` |
| `internal/settingsui/uninstall.go:166-167`（卸载对话框） | 两条字面 `0.3` / `0.2` | `theme.ElevThrow(panel, t, s.MainOpacity)` |
| `internal/terminalui/terminalui.go:1204-1205`（退出便条） | 同上 | `theme.ElevThrow(note, …)` |
| `internal/terminalui/terminalui.go:1254-1255`（失败提示） | 同上 | `theme.ElevThrow(notice, …)` |

几何（10/26 与 1/2 的 `--u` 步、圆角、边框、`LensRim`）零改动：本轮只动两条 alpha。

## 零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字节不动**。这一轮只改
「墨」。

## 门槛（本轮实测基线）

```
gofmt -l cmd internal        空
go vet ./...                 净（含 GOOS=windows / GOOS=darwin）
go test -count=1 ./...       全绿，PASS 2372 → 2374，SKIP 27，函数 1937 → 1939
三平台 build                 过
```

## mutation 计划

1. `ElevScale` 恒返回 `1`（＝旧的未缩放字面值）→ `TestTheElevationScaleFollowsTheFrameAlpha`
   必须红（`the default frame's scale is 1; the port painted 1 (unscaled)`）。
2. 把 `terminalui` 的一处改回 `Shadow(… ui.RGBA(0, 0, 0, 0.3))` →
   `TestFloatingThrowsFollowTheFrameScale` 必须红。
