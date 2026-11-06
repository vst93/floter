# P13-hk · 一格编辑器状态没有主人

起点 `53007b9b`，开盘工作区为空。选题落在任务书临时主线的「状态反馈 / 细节打磨」通用队列，
并接 P13-hi「仍待」第 2 条（提交失败与宿主规范化的组合）的**上半截**：状态在谁手里。

## 意图

这一族全是同一形状：**一格编辑器状态挂在 App 上（为了跨帧不丢），但它的主人（控件、表单、
对话框）一旦消失，这格状态没有退役路径**。下一次同类手势读到的是上一次的答案。

证据（全部 `file:line` 动手前核对过）：

1. `settingsui.go:1167`：`if !a.recording && !a.customRecording && c.Shortcut(0, ui.KeyEscape)` ——
   两个录制器的取消键**要求录制器仍是"焦点路径上"的**。捕获面板只在自己持键盘时收得到键
   （`custom.go:88-104` / `settingsui.go:4019-4033` 的 `HandleInput`）。点一下别的控件，
   `recording` 仍是 true，而全局快捷键仍被 `syncShortcutSuspension` 挂起（`settingsui.go:1119`）。
2. `custom.go:399-406`：`commitCustomCommand` 在 `next == "" || next == action` 时**直接 return**，
   `customEdits[key]` 留着。回车是一个提交手势，行里却继续画着从未写回的文字。
3. `settingsui.go:3216-3240`：`closeConfig` 退役了草稿、错误、字段错误，**没有**退役
   `configListScrolls`（`config.go:1177-1187` 的 `id\x00key`）。同一个 schema 重开时列表框
   停在旧偏移上。
4. `uninstall.go:131-137` / `uninstall.go:262`：`uninstallDraft[id]` 在取消、Escape、
   成功移除三条路径上都不删。取消一次卸载，再开一次，四个勾还是上一次的答案。
5. `pluginsheet.go:59-90`：草稿只在**同一张面**内按 schema 剪枝；浮层整个关掉时无人退役。
   `shell/pluginconfig.go:180-196` 用同一个 `a.Settings` 画每一个插件的浮层，于是重开读到
   上次的 `1e` / 上次的正文。
6. `settingsui.go:6433-6442` / `6704-6713`：`requestDisable` / `requestRemove` 开新问句时
   不清 `disableError` / `removeError`；`disableConfirmation`（`:6634-6638`）与
   `removeConfirmation`（`:6905-6909`）在错误存在时**用错误替换整句问句**。

## 落地（11 项，跨 settingsui 与 shell 两个组件）

| # | 位置 | 现象 | 改动 |
|---|---|---|---|
| 1 | `settingsui.go` `View` 的 Escape 分支 | 录制中点了别的控件，Escape 关掉整个设置页而不是取消录制 | `recording` 成为第一个分支：裸 Escape 先还回录制 |
| 2 | 同上 | 同上，自定义快捷键的草稿行 | `customRecording` 第二分支，并把 `customRecordRow` 收回 `customDraftRow` |
| 3 | `custom.go` `commitCustomCommand` | 空/等值回车留下未写回的草稿 | 回车即退役该行草稿 |
| 4 | `settingsui.go` `closeConfig` | 关掉表单后列表框停在上次偏移 | 按 `id\x00` 前缀退役 `configListScrolls` |
| 5 | `uninstall.go` Cancel 路径 | 取消后重开带上次勾选 | 新增 `resetUninstallDraft`，三条关闭路径都调 |
| 6 | `uninstall.go` Escape 路径（`dismissed`） | 同上 | 同上 |
| 7 | `uninstall.go` 成功移除 | 同上 | `finishComponentizedUninstall` 里调 |
| 8 | `uninstall.go` 帧末写回 | 关闭那一帧的 `draft` 又把已退役的草稿写回 | 只在对话框仍打开时写回 |
| 9 | `pluginsheet.go` 新增 `ClosePluginSheet` | 关掉浮层后正文/数字草稿与滚动偏移都留着 | 一次性退役草稿、偏移与 `sheetPlugin` |
| 10 | `shell/shell.go` `noteResidency` | 浮层关闭那一帧没有人通知面板 | 检测 `ConfigOpen()` 由真变假，调 `ClosePluginSheet` |
| 11 | `settingsui.go` `requestDisable` / `requestRemove` | 新问句沿用旧失败句 | 开问句时删掉该 id 的旧失败 |

## 接续：插件配置写回（第 12–17 项，来自上一份抢救稿）

`stash@{1}`（`42dce75c`，orphan 2026-11-06 09:29）与上面同一轮被中断的另一半。同一条账的两端：
控件这一帧**画什么**，与它交给宿主、宿主真正**存什么**。

| # | 位置 | 现象 | 改动 |
|---|---|---|---|
| 12 | `plugincfg/schema.go` `coerce`（**未接**） | 多选集合里 schema 未列举的值被丢弃 | 抢救稿改成保留；会让不可见的旧值无法在 UI 取消，且与 `TestCoerceShapesValuesLikeTheReferenceNormalizer` 的既有 pin 冲突 → 本轮**不接**，维持丢弃 |
| 13 | `settingsui.go` `pluginTextCommits` | 回车交原文、宿主去空白后框还画原文，直到失焦 | 记一帧提交原文，下一帧对照宿主回值：变了就退草稿 |
| 14 | `pluginsheet.go` `sheetField` Text | 回车交与宿主相同的文本也派发一次写 | 只有 `edit != text` 才 `Change` |
| 15 | 同上 | 提交记录随字段改型/消失残留在 App 上 | 按当前 schema 的 Text 字段退役 |
| 16 | `pluginsheet.go` `boundedValue` / `quantize` | 读数印界外值；网格锚在 0；取整后越上界 | 网格锚 `Min` + 仅在 `Max > Min` 时夹界，读数/派发/初值同一函数 |
| 17 | `pluginsheet.go` `sheetNumber` / `sheetNumberSlot` | 数字框只夹界不落格，派发 3.7 而宿主存 4 | 与滑块同走 `boundedValue`，初值走 `initialNumberText` |

第 12 项的取舍写进报告「未接」段：它改的是设置写回语义（B 类），不是本轮可擅自扩的范围。

## 红线

窗口高度链、键盘模型（除上面那一条 Escape 优先级）、会话管理、设置写回、i18n 文案、命令接线
零改动。没有新页面、新依赖、新架构。产品改动只碰「谁退役哪一格状态」。

## 门槛与基线（本轮实测）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux/windows/darwin go build ./...
```

基线（起点 `53007b9b`，`LC_ALL=zh_CN.UTF-8`）：见 report 的实测段。

## 变异计划

- M1：`View` 的 Escape 分支退回「`!a.recording && !a.customRecording` 才处理」→ 第 1/2 项 pin 红。
- M2：`closeConfig` 去掉 `configListScrolls` 前缀清理 → 第 4 项 pin 红。
- M3：`requestDisable` 去掉 `delete(a.disableError, id)` → 第 11 项 pin 红。
- M4：`noteResidency` 去掉 `ClosePluginSheet` 调用 → 第 10 项（shell 侧）pin 红。

每条都要实跑、看到真红、`md5sum -c` 逐字节还原、`touch` 后再跑一次。
