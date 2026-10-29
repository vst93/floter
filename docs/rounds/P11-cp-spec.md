# P11-cp · 抽屉的一次拖拽只写一次盘（与设置页同一扇 180ms 窗）

## 意图

**一次拖拽 = 一次落盘。** 终端外观抽屉的两个滑轴（字号、行高）每帧都走
`Store.Update`，而 `Update` 的成功路径**每次都真的写文件**；设置页的同源滑轴早已
按参考的 180ms 窗合并（`settingsui/debounce.go`）。本轮把抽屉补成同一条规则。

**为什么现在做**：这是 P11-co「仍待」第 2 条，也是那里唯一量到、还没做的开销；
P11-co 的报告把它记成「要先拍板」，但拍板证据已经在了——参考实现**本来就**
对这两个滑轴 debounce：

- `main:src/hooks/useSettings.ts:444-500`：`changeFontSize` 与
  `changeTerminalLineHeight` 各自 `clearTimeout` + `setTimeout(persist, SETTINGS_DEBOUNCE_MS)`，
  `SETTINGS_DEBOUNCE_MS = 180`（同文件 `:149`）。注释原话：「a drag emits a tick per
  pixel and `changeGeneralSetting` has no debounce, so it would write the settings
  file dozens of times per gesture」。
- `main:src/App.tsx:3925-3926`：终端抽屉（`variant="strip"`）的
  `onChangeFontSize` / `onChangeLineHeight` **正是**这两个 mutator。

所以本仓的「每帧一写」是移植漏掉的一处，不是行为选择；不需要拍板。

**这不是行为契约改动**：拖拽过程中值仍然是**实时**可见的（`Store.Preview` 与
`Update` 一样立刻推进内存版本并通知监听者，所以终端的字与行高仍随指针变），
变的只是**落盘的次数**——从「每帧一次」变成「停手后一次」。参考的行为就是这个。

## 现状（实测，动手前读的真实行号）

- `internal/terminalui/drawer.go:39-41`：`func (a *App) set(mutate)` →
  `_ = a.Store.Update(mutate)`。**没有 debounce、没有错误处理**（写失败被静默吞掉）。
- `internal/terminalui/drawer.go:140-152`（字号）与 `:167-179`（行高）：
  `drawerRange(...).Changed()` 之后调 `a.set(...)`。这两条是**唯一的滑轴**，
  每帧一次 `Changed()` 就是一次真写。
- 其余七行（family/padding/cursor shape/blink/select-copy/paste-safe/palette）
  是**离散选择**：一次点击一次写，参考也是立即写（`changeGeneralSetting`，
  `useSettings.ts:462-478`，无 debounce）。**它们不动。**
- 量法在 `internal/terminalui/dragwrites_test.go`（P11-co 收下的测量件）：
  `FLOTER_MEASURE=1 go test -run TestMeasureDrawerDragWrites ./internal/terminalui/`
  → `the drawer: 22 frames of one drag produced 17 accepted setting writes`。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settings/store.go` 的三条 `st.save(...)` 调用点（`Update` 成功路径、`Commit`、`Save`） | 只有 `snapshots` 一个计数器，**看不见「写了几次盘」** | 加 `writes uint64` + `Writes()`；三条真写路径各自 +1（走一个 `writeLocked` 帮助函数，所以注入的 `st.save` 被换掉时计数仍如实）。`Preview` 不计（它不写盘） |
| 2 | 新 `internal/terminalui/debounce.go` | 不存在 | `settingsDebounce = 180ms`（与 `settingsui` 同一个参考常数，但**各自一份**：两个包不互相 import，注释交叉引用）；`App` 加 `debouncing bool` + `debounceAt time.Time`；`preview(mutate)` = `Store.Preview` + 重开窗；`expire(c)` = 到点 `Commit`；`Flush()` 立刻提交 |
| 3 | `internal/terminalui/drawer.go:39-41` 的 `set` | 立即 `Update`，错误吞掉 | 立即 `Update`，**成功即清掉在跑的窗**（那次写已经把 preview 的值一并落盘，`Commit` 随后会是空操作，所以不许再写一次）；失败 → `ShowFeedback(i18n.Settings.SaveFailed)` |
| 4 | 同文件 `:140-152` / `:167-179` 两条滑轴 | `a.set(...)` | `a.preview(...)`：值立刻进内存（拖拽仍实时），落盘交给窗 |
| 5 | `internal/terminalui/terminalui.go` 的 `View` 开头 | 没有任何到点检查 | `a.expireSettingsWrites(c)`（`deadline.Wake` 武装一次唤醒，不是 `AnimationFrame`——P11-bq 的规则） |
| 6 | `internal/shell/shell.go` 的三个出口 | `Open` 只 flush 设置页（`:3411-3413`）；`Hide`（`:4126`）与 `quit`（`:671`）只 flush 设置页 | 三处都补 `a.Terminal.Flush()`：离开终端表面、窗口隐藏、退出。**这是真 bug 不是洁癖**——隐藏后不再画帧，`deadline` 的唤醒永远不会到，那次拖拽就静默丢了 |

## 行为契约零改动红线

- **高度链**：抽屉的 `MinHeight`/`Gap`/`Padding`/`drawerCellFloor` 一个数不动；
  本轮零像素（拖拽中与松手后的帧，`Store` 的值完全相同，所以画面逐字节相同）。
- **键盘模型**：滑轴仍是框架 `ui.Slider` 的箭头/拖动语义，没有新增按键。
- **会话管理**：`Refresh` 仍由 store 监听者驱动；`Preview` 也通知监听者，所以
  「拖的时候终端的字已经在变」这条不变。
- **设置写回**：`Store` 的「失败回滚到 `confirmed`」不变；本轮只减少写次数。
- **i18n 文案**：只**复用**既有 `i18n.Settings.SaveFailed`（"Could not save that
  change."），不新增字符串。
- **命令接线**：无。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                 # 必须空
go vet ./...                          # 必须净
GOOS=windows go vet ./...             # 必须净
GOOS=darwin  go vet ./...             # 必须净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 2782，只许涨
GOOS=linux|windows|darwin go build ./...
```

基线（干净 worktree、本轮动手前实测）：**2782 PASS / 0 FAIL / 58 SKIP /
2068 测试函数**。

## 新增 pin（三条）

1. `internal/settings`：`TestTheStoreCountsOnlyTheWritesItAttempts` —— `Preview`
   不涨计数、`Update`/`Commit`/`Save` 各涨一次、无事发生的 `Update`/`Commit` 不涨。
2. `internal/terminalui`：`TestADrawerDragWritesTheFileOnce` —— 22 帧的一次拖拽：
   `Store.Version()` 涨很多（值实时），`Store.Writes()` **恰好 1**，且盘上就是最后
   那一格的值；窗还没到时 0 次。
3. `internal/shell`：`TestLeavingTheTerminalSurfaceWritesTheDraggedValue` ——
   拖到一半直接换表面（不等到点），盘上必须是拖到的值。**这一条修前红**（那次写
   永远不会发生），是本轮唯一能证明「静默丢写」被修掉的断言。

## mutation 计划（≥2 条，各自说明会让哪个 pin 变红）

- **M1**：`preview` 改回 `a.set`（即立即写）→ `TestADrawerDragWritesTheFileOnce`
  红在「恰好 1 次」（实测 17 次）。
- **M2**：`expireSettingsWrites` 的 `Commit` 删掉 → 同一条 pin 红在「盘上是最后
  那一格的值」。
- **M3**：`shell.Open` 里新加的 `a.Terminal.Flush()` 删掉 →
  `TestLeavingTheTerminalSurfaceWritesTheDraggedValue` 红（盘上还是拖拽前的值）。
- **M4**：`set` 成功后不清窗 → 不指望变红（`Commit` 本就是空操作），**这一条
  写成「不设 pin」并在报告里说明为什么**：它能被观察到的唯一方式是写次数，
  而写次数在两种写法下都正确。不许为它造一条假 pin。

## V / B / S 预期

- **V = 0**：无墨、无面、无几何改动。
- **B = 0**：高度链 / 键盘 / 会话 / 写回语义 / i18n / 接线全部零改动；既有测试
  零改动必须全绿。
- **S = 1**：`App` 多两个字段、`Store` 多一个计数器；`set` 的语义从「无条件写」
  变成「写并关窗」。
