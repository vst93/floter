# P11-cp 报告 · 抽屉的一次拖拽只写一次盘

**一句话**：终端外观抽屉的字号 / 行高两个滑轴以前**每帧写一次文件**（22 帧的拖拽 =
17 次落盘），现在与设置页同一扇 180ms 窗：拖的时候值照样实时，停手后**写一次**。

## 0. 选题与证据

P11-co「仍待」第 2 条是唯一一条**已经量到、还没做**的开销。它在 P11-co 里被记成
「要先拍板」，但拍板证据在参考实现里，不需要问：

- `main:src/hooks/useSettings.ts:444-500`：`changeFontSize` 与
  `changeTerminalLineHeight` 各自 `clearTimeout` + `setTimeout(persist, 180ms)`。
  原文注释：「a drag emits a tick per pixel and `changeGeneralSetting` has no
  debounce, so it would write the settings file dozens of times per gesture」。
- `main:src/App.tsx:3925-3926`：终端抽屉（`variant="strip"`）的
  `onChangeFontSize` / `onChangeLineHeight` **正是**这两个 mutator。

所以本仓的「每帧一写」是移植时漏掉的一处，不是设计选择。其余七行（family /
padding / cursor shape / blink / select-copy / paste-safe / palette）是**离散选择**，
参考也是立即写（`changeGeneralSetting`，同文件 `:462-478`），**本轮一个没动**。

## 1. 实测（改前 / 改后）

量法：`internal/terminalui/dragwrites_test.go`，900×600、抽屉打开、22 帧的一次拖拽
（press + 20 次 move + release）。

| | 拖拽期间写盘 | 拖拽结束时 `Store.Version()` | 盘上最终值 |
|---|---|---|---|
| 改前 | **17 次**（每帧一次） | 22 | 最后一格 |
| 改后 | **0 次** | 22 | 最后一格（停手后 1 次） |

「17 次」不是我算的：P11-co 收下的测量件跑出来的原话是
`the drawer: 22 frames of one drag produced 17 accepted setting writes`；本轮 M1
变异把它还原成同一句数字（见 §4）。

**值仍然实时**：`Store.Preview` 与 `Update` 走同一条 clone → mutate → normalize →
compare 链，都推进 `version` 并通知监听者，所以终端的字与行高**拖的时候就在变**。
变的只有落盘次数。这一点被 pin 的两个半边分别钉住（见 §3）。

## 2. 落地项（位置 → 现状 → 应改成）

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `internal/settings/store.go` 的三条真写路径 | 只有 `snapshots` 计数器，**看不见「写了几次盘」** | 新增 `writes uint64` + `Writes()`，三条路径统一走 `writeLocked`（计数在锁内、在 `save` 之前，所以测试换掉 `save` 时计数仍如实；`Preview` 不计） |
| 2 | 新 `internal/terminalui/debounce.go` | 不存在 | `settingsDebounce = 180ms`；`preview` / `scheduleSettingsWrite` / `flushSettingsWrites` / `expireSettingsWrites` / `Flush` |
| 3 | `internal/terminalui/drawer.go:39` 的 `set` | 立即 `Update`，**错误被 `_ =` 吞掉** | 立即 `Update`，成功即**关掉在跑的窗**（那次写已把 preview 的值一并落盘，`Commit` 随后是空操作，不许再写一次）；失败 → `ShowFeedback(i18n.Settings.SaveFailed)` |
| 4 | 同文件 `:150`（字号）/ `:177`（行高） | `a.set(...)` | `a.preview(...)` |
| 5 | `internal/terminalui/terminalui.go` 的 `View` 开头 | 没有到点检查 | `a.frameAt = c.Now()` + `a.expireSettingsWrites(c)`；窗还在跑时 `deadline.Wake` 武装**一次**唤醒（P11-bq 的规则，不是 `AnimationFrame`） |
| 6 | `internal/shell/shell.go` 的三个出口（`Open` / `Hide` / `quit`） | 只 flush 设置页 | 各补 `a.Terminal.Flush()` |

### 第 6 条是真 bug，不是洁癖

窗是靠**帧**到点关的（`deadline.Wake`）。而隐藏的窗口**不再画帧**——`deadline` 的
唤醒永远不到，那次拖拽就**静默丢了**：用户松手前换页 / 收窗口，字号变了一格、盘上
还是旧值。修前红、修后绿的那条 pin 就是它（§3 第 3 条）。

## 3. 新增 pin（5 条，全部实测）

1. `internal/settings` · `TestTheStoreCountsOnlyTheWritesItAttempts`：`Preview` 不涨、
   `Commit`/`Update`/`Save` 各涨一次、无事发生的 `Update`/`Commit` 不涨、**被拒绝的
   写仍算一次尝试**且值回滚到 `confirmed`。
2. `internal/terminalui` · `TestADrawerDragWritesTheFileOnce`：**两个半边都读**——
   22 帧拖拽里 `Version()` 必须涨 ≥3（值实时；只读次数的话「干脆不预览、松手才写」
   也能绿），且 `Writes()` 恰好 +1、盘上是最后那一格、静止三帧不再写。
3. `internal/shell` · `TestLeavingTheTerminalSurfaceWritesTheDraggedValue`：拖到一半
   **不等到点**直接 `Open(SurfaceLauncher)`，盘上必须是拖到的值。**修前红**。
4. `internal/shell` · `TestHidingTheWindowWritesTheDraggedValue`：同一条规则的另一出口。
5. `internal/terminalui` · `TestAPreviewAndAWritePaintTheSameFrame`：同一组值分别经
   `Preview` 与 `Update` 后导出的 PNG **逐字节相同**，且带**控制臂**（未改值的帧必须
   与它们不同，否则两臂都画的是出厂帧、比较什么也没证明）。

`TestTheDrawerWritesNothingWhileTheDragIsStillMoving` 是第 2 条的翻面：窗内 `os.Stat`
连文件都不存在，过了窗才 1 次。

## 4. 变异（4 条实跑变红 + `md5sum -c` 逐字节还原）

| # | 注入的缺陷 | 变红的 pin 与原始报错串 |
|---|---|---|
| M1 | 字号滑轴 `a.preview` → `a.set`（即还原成改前） | `TestADrawerDragWritesTheFileOnce`：`the drag wrote the file 17 times while its window was open, want 0` |
| M2 | `flushSettingsWrites` 的 `Commit` 挖掉 | 同 pin：`the drag wrote the file 0 times, want 1 (22 frames)`；`TestTheDrawerWritesNothingWhileTheDragIsStillMoving`：`the frame past the window wrote 0 times, want 1` |
| M3 | `shell.Open` 新加的 `a.Terminal.Flush()` 删掉 | `TestLeavingTheTerminalSurfaceWritesTheDraggedValue`：`leaving the terminal wrote 0 times, want 1` |
| M4 | `preview` 里 `Store.Preview` 挖掉（只排窗、不推值） | 第 2 条 pin 的**值半边**：`a 22-frame drag advanced the store 0 times: the value did not follow the pointer`；另两条：`the drag never moved the font size off the shipped 14` |

M1 报出的 17 与 P11-co 测得的 17 是同一个数——改前行为被完整还原，不是「差不多」。

**一条不设 pin**：`set` 成功后不清窗。它在两种写法下写次数都正确（`Commit` 本是
空操作），唯一可观察的差别是「多调一次空 Commit」——为它造 pin 只能造假断言。
报告里记这一条，而不是留一条守不住的断言。

还原后 `md5sum -c` 五个文件全部「成功」，再 `touch` 全部改动文件复跑一次全量
（避免测试缓存复用）。

## 5. 视觉证据：零像素

拖拽中与松手后 `Store` 的值完全相同，所以画面必然逐字节相同——但这值得**量**，
不是嘴上说。同一份 opt-in dump（`FLOTER_PNG=1`，900×620，抽屉打开）在**干净
HEAD worktree** 与本轮各出一张：

```
2bf98fc414488cb47b1cc31e2088f1b1  /tmp/floter-P11-cp-terminal-dark-before.png
2bf98fc414488cb47b1cc31e2088f1b1  /tmp/floter-P11-cp-terminal-dark-after.png
cece54886a88fd35708e6b863232f34e  /tmp/floter-P11-cp-terminal-light-before.png
cece54886a88fd35708e6b863232f34e  /tmp/floter-P11-cp-terminal-light-after.png
```

**我实测看到了什么**（在 `...-light-after.png` 上取像素）：画布是纯白
`(255,255,255)`，一直铺到 y≈420；从 y=420 起整宽是抽屉条带的 `Control` 面
`(234,234,235)`；y=540 行 x=880 处是读数墨 `(179,179,180)`。深色同一处：
画布 `(24,24,27)` → 条带 `(32,32,34)` → 读数墨 `(96,96,99)`。也就是说**画布与
面板之间没有第二圈框**（P11-z 的规则仍在），抽屉是画布的**兄弟**、贴边、整宽，
不是压在上面的浮层。

## 6. V / B / S 台账

- **V = 0**：无墨、无面、无几何改动；两套外观 dump 前后 md5 相同。
- **B = 0**：高度链 / 键盘模型 / 会话管理 / 设置写回语义 / i18n 文案 / 命令接线
  零改动；**既有测试一条没改**（`grep` 确认本轮未触碰任何既有 `_test.go`，只新增
  与替换测量件）。
- **S = 1**：`App` 多三个字段（`debouncing` / `debounceAt` / `frameAt`）、
  `Store` 多一个计数器；`set` 的语义从「无条件写」变成「写并关窗」；`drawer.go` 的
  两条滑轴改走 `preview`。

## 7. 门槛（本轮实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                    → 全绿，exit 0
go test -count=1 -v ./... | grep -c PASS  → 2788   （基线 2782，+6）
                                        SKIP → 57    （基线 58，−1：测量件变成 pin）
                                        FAIL → 0
测试函数（`^func Test`）                  → 2073   （基线 2068，+5 = 6 新增 − 1 被替换的测量件）
GOOS=linux|windows|darwin go build ./...  → 三平台过
```

## 8. 仍待 / 留给下一轮

1. **抽屉的写没有「合并同族」的收口**：`set` 现在会关窗，但**离散行**（palette 等）
   一次点击就是一次真写。参考也是立即写，所以不是缺陷；只是若将来某行变成连续控件，
   必须走 `preview`。这一条写进 `debounce.go` 的注释里，不另立待办。
2. `internal/settingsui` 的 `settingsDebounce` 与本轮 `terminalui` 的
   `settingsDebounce` 是**同一个参考常数的两份拷贝**（两个包互不 import）。若第三处
   也需要，才值得抽一个包；两处就抽，是为一个数建一个包。
3. 老拍板项与尺子盲点原样有效（见 `.pending-bugs.md`）。
