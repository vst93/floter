# P13-gy 规格 · 一个指针答案，一个速度

**意图**：把「九处指针面一帧跳过去」收进 `theme.ColorStep`（120ms），让同一个指针动作在
整个应用里只讲一个速度。

**为什么现在做**：§5 第一行定的是 120ms，账本 `internal/ci/colorstep_contract_test.go`
只记到 13 处（7 个文件）。开了 `Transition` 的面在这九处之外都在 120ms 里走完，
这九处 `f0 == f400`（指针落下的那一帧就到站）。证据是这一轮实测的三帧读数
（`docs/liquid-glass.md` §6.175 的表），不是旧报告。

**同时回收**：交接残留里 `stash@{0}`（2026-11-05 12:29 `stale-kill`）这条孤儿补丁
是同一件事的半成品（产品侧 9 处 + 账本），外加第三父里的两个未跟踪探针
（`internal/shell/pointerstep_test.go` / `internal/settingsui/pointerstep_test.go`）。
本轮把它接上、补齐探针的取样时机、并扩到每个面深浅两套。

## 逐条落地项

| # | 位置（动手前核过行号） | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/pluginsheet.go` `sheetActionButton` | 无 `Transition` | `Transition{Colors: true, Duration: theme.ColorStep}` |
| 2 | `internal/settingsui/rows.go` `sessionIconButton` | 同上 | 同上 |
| 3 | `internal/settingsui/rows.go` `sessionKillConfirm` | 同上 | 同上 |
| 4 | `internal/settingsui/settingsui.go` `shortcutRecorder` | 同上 | 同上 |
| 5 | `internal/settingsui/settingsui.go` `runOutputToggle` | 同上 | 同上（动态 pin 缺，见「仍待」） |
| 6 | `internal/shell/detach.go` `barButton` | 同上 | 同上 |
| 7 | `internal/shell/notices.go` toast 动作 chip | 同上 | 同上 |
| 8 | `internal/shell/notices.go` toast ✕ | 同上 | 同上 |
| 9 | `internal/shell/pinned.go` 便条 Close | 同上 | 同上 |
| 10 | `internal/ci/colorstep_contract_test.go` `colorStepSites` | 7 文件 / 13 处 | 11 文件 / 22 处（新增 5 个文件条目） |
| 11 | 探针取样时机：`litpair_census_test.go` 的 `litFrame`、`sessionrow_test.go` 的问句、`notices_test.go` 两条、`pluginwindowbar_test.go` 的 `barTryRead`、`pressrung_test.go` 两处 | 指针落下就读面（改前读得到只因为一帧跳过去） | 读面之前 `tt.Advance(300ms); tt.Frame()`；**数值一个不改** |

## 行为契约零改动

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：一字不动。视觉轮只改「面的
过渡时长」与账本；探针改的是取样时机，不是断言数值。

## 门槛

`gofmt -l cmd internal` 空 · `go vet ./...` 净 · `GOOS=windows/darwin go vet` 净 ·
`go test -count=1 ./...` 全绿 · PASS 计数只许涨 · 三平台 `go build` 过。

## mutation（每条说明让哪个 pin 变红）

- **M1** 删掉 `sessionIconButton` 的 `Transition`（= 改前的实现）→
  `internal/settingsui/pointerstep_test.go:142` 红（`light 742 → 664` / `dark 44 → 146`），
  且 `internal/ci` 账本 `rows.go: 5 colour transitions, want 6` 红。
- **M2** 删掉 toast ✕ 的 `Transition` → `internal/shell/pointerstep_test.go:120` 红
  （`light 0 → -26114` / `dark 0 → 32886`）。
- **M3** 把 `Duration` 写成 `0` → 账本红；动态 pin 仍绿 —— 记下这条事实（0ms 与 120ms
  在「f0 还是静息面」这一维上不可分），写进 §6.175。
