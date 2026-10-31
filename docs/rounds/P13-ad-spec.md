# P13-ad 规格 · 指针的颜色过渡收成一条算术（`--dur-2` = 120ms）

## 意图（一句话）

**同一个指针动作（面抬起 / 按下 / 离开）在应用的每个面上都必须走同一条时长**——
本轮把散落在七个文件里的 12 个手写过渡收成 `theme.ColorStep`，并让
`docs/liquid-glass.md` §5 那条从来不存在于代码里的 `90ms` 显式重定基线到实测值。

## 为什么现在做（证据）

`docs/liquid-glass.md` §5 的状态表第一行写着：

> 颜色过渡（行、控件）｜90ms ease-out，只动颜色｜按帧可预测

`docs/raycast-design.md:231` 更具体地写着「颜色过渡：`rowTransition` = 90ms ease-out（Colors only）」。
**这两句都不是真的**，实测（`grep -rn "Colors: true" internal/ --include=*.go`，非测试）：

| 事实 | 行号（本轮实测） |
|---|---|
| `rowTransition` 是**行的入场**（`Colors: false`，`Enter: {Opacity: 0}`），不是颜色过渡 | `internal/launcher/launcher.go:966-970` |
| 颜色过渡实际有 **11 处手写的 120ms** | `launcher/filters.go:211`、`launcher/launcher.go:1883`、`:3450`、`launcher/power.go:181`、`settingsui/parameditor.go:267`、`settingsui/rows.go:502`、`:535`、`:563`、`settingsui/settingsui.go:1321`、`:5856`、`settingsui/uninstall.go:349` |
| 颜色过渡还有 **1 处 150ms**（唯一离群，卸载对话框的组件行） | `settingsui/uninstall.go:434` |
| 90ms 这个数字在 `internal/` 与 `cmd/` 里**一次都没出现过** | 全仓 grep |

120ms 的来源是有据的：`internal/launcher/launcher.go:3441-3444` 的注释把参考 CSS 的
`--dur-2` 记作颜色那一步（`color var(--dur-2) ease, background var(--dur-2) ease`），
transform 走 `--dur-1`（80ms）。**150ms 那一处也是参考 CSS 自己的值**
（`ComponentizedUninstallDialog.css:60-73` 的 `transition: background 150ms ease`）。

所以这一轮真正的病灶不是「数字错」，而是**同一个应用里指针的答案有三种速度**：
列表行 120ms、对话框里的组件行 150ms（慢 25%）、合约写着 90ms（谁也没实现）。
一个浮起来的纸（§2 的 E2）里的行比它脚下的列表慢四分之一，读起来就是两块不同年头的界面，
这正是用户说的「偏老气」的一个算术来源。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/theme/theme.go`（`Tokens` 附近） | 没有颜色过渡的常量 | 新增 `const ColorStep = 120 * time.Millisecond`，注释写清「指针的颜色答案只有一步：参考 CSS 的 `--dur-2`；`--dur-1`(80ms) 是 transform，`--dur-3`(160ms) 是浮层入场，都不是这一步」 |
| 2 | `launcher/filters.go:211` | `{Colors: true, Duration: 120 * time.Millisecond}` | `{Colors: true, Duration: theme.ColorStep}` |
| 3 | `launcher/launcher.go:1883` | `{Duration: 120 * time.Millisecond, Colors: true}` | `{Duration: theme.ColorStep, Colors: true}` |
| 4 | `launcher/launcher.go:3450` | 同上 | 同上 |
| 5 | `launcher/power.go:181` | `{Colors: true, Duration: 120 * time.Millisecond}` | `{Colors: true, Duration: theme.ColorStep}` |
| 6 | `settingsui/parameditor.go:267` | 同上 | 同上 |
| 7 | `settingsui/rows.go:502` / `:535` / `:563` | 同上 ×3 | 同上 ×3 |
| 8 | `settingsui/settingsui.go:1321` / `:5856` | 同上 ×2 | 同上 ×2 |
| 9 | `settingsui/uninstall.go:349` | `{Colors: true, Position: true, Duration: 120 * time.Millisecond}` | `{Colors: true, Position: true, Duration: theme.ColorStep}` |
| 10 | `settingsui/uninstall.go:434` | `{Colors: true, Duration: 150 * time.Millisecond}`（参考 CSS 的 150ms） | `{Colors: true, Duration: theme.ColorStep}` —— **重定基线：150 → 120**，理由见上；参考的出处写进注释，将来要回退是一次显式决定 |
| 11 | `docs/liquid-glass.md` §5 | 「颜色过渡（行、控件）90ms ease-out」 | 「120ms（`--dur-2`，`theme.ColorStep`）」+ 旧值→新值 |
| 12 | `docs/raycast-design.md:231-232` | 把 `rowTransition` 说成 90ms 的颜色过渡 | 拆成两行事实：`rowTransition` 是 120ms 入场（`Colors: false`），颜色过渡是 `theme.ColorStep` 120ms |
| 13 | `internal/ci/` 新增一条账 | 无 | 任何 `ElementTransition{… Colors: true …}` 都**不许**自带字面 `Duration`（唯一例外是 `internal/theme` 里那条定义），否则红 |

## 行为契约零改动红线

这一轮**只动时间，不动墨、不动面、不动几何**：

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
- 唯一可见变化是「卸载对话框里组件行的指针颜色答案从 150ms 变 120ms」；
  起止颜色、`LitFace` 的一对线、`ControlPress` 的按下档、`Position` 的过渡**全部不变**。
- 不改 `rowTransition`（120ms 入场）、不改 toast 的 160ms 入场、不改 `--dur-1` 的 80ms
  transform（框架把它当 render-time transform，本来就不走这条时长）。

## 门槛命令 + PASS 基线

```
gofmt -l cmd internal                                   # 必须空
go vet ./...                                            # 必须净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...     # 必须净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"        # 只许涨
GOOS=linux/windows/darwin go build ./...                # 三平台过
```

**基线（上轮 P13-ac 实测）**：PASS **3021** / FAIL 0 / SKIP **85** / 测试函数 **2227**。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. **M1**：把 `theme.ColorStep` 改成 `150 * time.Millisecond` → 让新的
   `TestTheColorStepIsTheReferenceDurTwo`（常量 = 120ms 的绝对值）变红。
   这一条不能拿 `ColorStep` 自己判自己，必须写死 120 的绝对数。
2. **M2**：把 `settingsui/uninstall.go:434` 退回字面 `150 * time.Millisecond` →
   让 `TestEveryColorTransitionRidesOneStep`（CI 扫描账）变红。
3. **M3**：把 `launcher/power.go` 的 `Colors: true` 整条 `Transition` 删掉 →
   让扫描账的「允许表过期」臂变红（账里记着这个位置必须有一条颜色过渡）。
