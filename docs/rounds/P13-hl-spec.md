# P13-hl · 禁用墨：一个状态，一个深度

起点 `1765f2cd`（P13-hk），开盘工作区为空。规格日期 2026-11-06。

## 接续校正

本规格来自 e0f58d758c 抢救稿，旧测量不是本次验收数字。当前行号已读核对：
theme.go:247/667/703/838、rows.go:616/764/1095、settingsui.go:6476/6543、
uninstall.go:335/444、shell/detach.go:398。基线日志 3806 PASS / 0 FAIL / 2797 函数，退出状态说明见报告。
八项可见修复：胶囊、扩展图标、会话图标、中性确认键、确认键图标、卸载忙态键、固定卡说明、输出窗口重跑键。
每两项提交推送一次；token 为实现手段，不单独凑项。分批相关回归，最终整套门槛。

设计读取：桌面设置和输出工具条，面向高频键盘用户；轻便液态玻璃，冒险度 2/10（稳定）、
动效 2/10（沿用）、密度 7/10（紧凑）。禁用仍可读动作名，不误认作可点；可用态照常操作。
Apple 供体 48% 退让墨转译为框架已有 0.5；近黑/近白沿用 #1B1B1D/#F7F7F8；
交互蓝继续只服务可操作状态，不给禁用态加蓝。
craft-loop #6 用语义墨区分状态；#11 图标与标签同色；#12 保留 DimFace，禁用受光边为 0。
不为凑五项新增装饰。高度链、键盘、保存、会话、文案、接线均不改。

## 意图

禁用面的**材质**已经在 P13-s / P13-t / P13-u / P13-aq / P13-ba 收成「框架那一道 0.5」；
这一轮修的是同一族的另一半——**淡之前的墨**：每个调用点挑自己那一档
（`TextStrong` / `TextSecondary` / `TextMuted` / `TextTertiary` / `Warning`），0.5 之后
同一个「现在不能用」在同一页上有 47~113 级的六种深浅，其中三处低于本仓自己用了十几轮的
60 级可读性地板。

证据（都是本轮实测，浅 / 深，对比级）：

- `rows.go:616` `extensionActionFace` 的 `disabled` 臂：`ink = TextStrong` → 105 / 113；
- `rows.go:764` `extensionIconButton` 的 `disabled` 臂：`ink = TextSecondary` → 70 / 69；
- `rows.go:1095` `sessionIconButton` 的 `disabled` 臂：`ink = TextMuted` → **59 / 61**；
- `uninstall.go:444` `uninstallPartRow` 的 `Fixed` 卡：标题 105、提示 **55**；
- `shell/detach.go:355` 的「重跑」键：`TextTertiary` → **47 / 53**（全仓最低）；
- `settingsui/settingsui.go:6476` `barButton`：禁用臂 `TextStrong`（105）且**图标没有
  `TextColor`**——danger 键「字是警告红、图标是主题黑」，禁用键的图标停在 105。

## 逐条落地

1. **新档 `theme.Tokens.Disabled`**（`internal/theme/theme.go`）：**淡之前**的墨，浅
   `#58585C` / 深 `#B4B4BC`。0.5 之后落 74~84 级。
2. **胶囊族**（`rows.go` `extensionActionFace`）：禁用 ink → `Disabled`（105/113 → 74/78）。
   六个调用点共用：`disabledActionButton` / `busyActionButton` / `extensionGlyphActionButton`
   ×3 / `configActions` 的 defaults。
3. **30u 透明小格**（`extensionIconButton`）：70/69 → 79/84。
4. **28u 会话小格**（`sessionIconButton`）：59/61 → 79/84（跨过地板）。
5. **确认带胶囊**（`settingsui.go` `barButton`）：105/113 → 74/78；danger 保留 `Warning`。
6. **确认带胶囊的图标**（同处）：补 `.TextColor(ink)`——图标跟着字走（danger → 警告族；
   禁用 → 74/78）。
7. **在飞卸载胶囊**（`uninstall.go` `uninstallActionButton`）：中性 105 → 74/78。
8. **固定卸载卡**（`uninstallPartRow` 的 `Fixed` 臂）：标题 105 → 78、提示 55 → 84。
9. **插件窗口「重跑」键**（`shell/detach.go`）：47/53 → 78/80。

## 行为契约零改动红线

只改墨：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、面差/受光边/外投影/
圆角/间距全部零改动。`DimFace`、`lockedControl`、`Cursor` 语义不动。

## 门槛

`gofmt -l cmd internal` 空；`go vet ./...` 三平台净；`go test -count=1 ./...` 全绿；
PASS 计数只许涨；三平台 `go build ./...` 过。

## 变异计划（≥2）

- M1：把 `rows.go` 三处 `ink = tokens.Disabled` 换回 `tokens.TextStrong` →
  `TestEveryDisabledLabelLandsOnOneRung` 红（胶囊 105/106 越上沿、小格越过下沿）。
- M2：把浅色 `Disabled` 换成 `#A8A8AC` → 同一 pin 红（胶囊 34 级，越过下沿）。
- M3：删掉 `barButton` 图标的 `.TextColor(ink)` → `TestTheBarButtonsIconWearsTheWordsInk`
  红（danger 读 83 = 主墨，禁用图标读 105）。
