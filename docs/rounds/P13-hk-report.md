# P13-hk · 一格编辑器状态没有主人（+ 抢救两条 stash）

起点 `53007b9b`，开盘工作区为空。本轮先把两条 `stale-kill` 抢救稿接完（它们各是同一族
的一半），再按族收口。规格 `docs/rounds/P13-hk-spec.md`，合约新增 §6.186。

## stash 清点（开盘第一件事）

| ref / SHA | 判断 | 处置 |
|---|---|---|
| stash@{0} / `a8c28455`（orphan 2026-11-06 10:16） | 有独有工作：11 条 pin + 产品修复 5 文件，reverse-check 退 1 | `git stash apply` 接上、跑门槛、**拆成两个 commit**（8ed57011 / 0ec9b679+eca65898），归档 `refs/archive/floter/P13-hk/a8c28455…` 后 drop |
| stash@{1} / `42dce75c`（orphan 2026-11-06 09:29） | 有独有工作：插件写回 6 项 + 5 条 pin，reverse-check 退 1 | 接上、门槛、**拆成两个 commit**（9b9303f1 / 4c500ae2），归档 `refs/archive/floter/P13-hk/42dce75c…` 后 drop |
| stash@{0}–{7}（旧 8 条：422483b4 / 8af94a07 / 7e4aa710 / 1e3aec94 / 363f5a42 / 9b8ce23c / 90d314b2 / 7a9f6e13） | reverse-check 全部退 1，SHA 与 P13-hh/P13-hi 结案表完全一致 | **保留原件**，未静默丢弃 |

两条 stash 都不是「已被 HEAD 覆盖」，也没有一条能整块合入：`stash@{1}` 里
`plugincfg/schema.go` 那一段改的是**设置写回语义**（多选集合保留 schema 未列举的值），
它会让不可见的旧值无法在 UI 里取消，并且与既有 pin
`TestCoerceShapesValuesLikeTheReferenceNormalizer` 的两条子用例（`an unknown check box is
dropped` / `a check box set with nothing valid is empty`）直接冲突。**这一段不接**，
其余五段接上。理由写在下面「未接」段。

## 逐条：现象 → 改动 → 验证

### 第一批（写回账：控件画什么 = 宿主存什么）· commit 9b9303f1

1. **文本字段回车后框里画的是交上去的原文**
   - 现象：回车那一帧 `sheet.Change(field.Key, edit)` 把原文交给宿主，宿主按
     `plugincfg.coerce` 去首尾空白（全空白记成空值）。框里此前一直画着原文，
     于是「框写 `"  /tmp/profile  "`、插件存 `"/tmp/profile"`」要等到**失焦**才对齐——
     同一个字段在同一帧有两个说法。
   - 改动：新增 `App.pluginTextCommits`，只记「回车交上去、还没读回宿主答案」的那一帧。
     下一帧拿它对照宿主回给这块面的值：变了（收过/清空）就退掉草稿改画宿主值；一个字节
     没动说明写被拒（`sheet.Failed`），草稿留着给用户重试（§6.45 契约不动）。
   - 验证：`TestTheTextBoxShowsTheValueTheHostStored`（en/zh）。改前原始串
     `en: the box still paints the text that was sent: "  /tmp/profile  " (host holds "/tmp/profile")`。
2. **回车交一份和宿主一样的文本也派发一次写**
   - 现象：`if input.Submitted() { sheet.Change(...) }` 无条件派发。写成功是白写一遍
     同一个值；写被拒就在一块**没改过**的面上亮出「保存失败」。
   - 改动：只有 `edit != text` 才 `Change`，并把交上去的原文记进 `pluginTextCommits`。
   - 验证：`TestEnterOnUnchangedTextDoesNotWrite`。变异 M1（`if edit != text` → `if true`）
     实跑红，原始串 `Enter on an unchanged field wrote it: [{custom_base_dir /already/stored}]`。

### 第二批（数字账：读数 / 网格 / 派发值同一条）· commit 4c500ae2

3. **界外的存量值照样印在读数上**
   - 现象：手改配置把 `max_items` 写成 9999（Max 1000）时，轨道按框架自己的区间停在
     头上，旁边读数仍印 `9999 items`。一条横线说「到这里了」，另一处说「9999」。
   - 改动：`boundedText` 先过 `boundedValue`（量化 + 只在 `Max > Min` 时夹界），
     读数就是「拖一下会存下的那个数」。
   - 验证：`TestTheBoundedReadoutStaysInsideTheSchemaBounds`（深浅）。改前
     `dark=false: the readout does not show the stored value clamped to the bound: ["Probe" "Number" "10000 items"]`。
4. **量化网格锚在 0，不锚在字段的 `Min`**
   - 现象：`quantize(value, step) = round(v/step)*step`；`plugincfg.coerce` 的 `clampStep`
     是 `round((n-lo)/step)*step + lo`。`Min: 1, Step: 10` 的字段在 1.5 处被量化成 **0**——
     一个比下界还小、宿主立刻夹回 1 的数，而这一帧的读数、滑块位置、派发值都在说 0。
   - 改动：`quantize(field, value) = round((v-Min)/Step)*Step + Min`，与 `clampStep` 同一条。
     `Min` 落在网格上的内置字段（`max_items` / `history_days`）逐字节不变。
   - 验证：`TestTheStepGridIsAnchoredAtTheMinimum`。改前 `quantize(1.5) = 0`（< Min 1）。
5. **取整后越上界**
   - 现象：96 最近的网格点是 100，而 `Max` 是 99；`coerce` 会夹回 99，于是读数（100）
     与插件真正存下的数（99）差一格。
   - 改动：`boundedValue` 量化之后再夹界；`Max <= Min`（零值）当「没写界」，不夹。
   - 验证：同上，`boundedValue(96) = 100, want 99` / `boundedText(96) = "100", want "99"`。
6. **数字框只夹界不落格**
   - 现象：`sheetNumber` 派发 `clampNumber(field, v)`，`Step: 1` 的字段把 **3.7** 交上去，
     宿主再按网格收成 4：这一帧的派发值与插件实际存下的数不是同一个。
   - 改动：数字框与滑块统一走 `boundedValue`；无草稿的初值走 `initialNumberText`（同样夹
     界，`70000` 在 `Max: 65535` 的字段里不再印 `70000`）。
   - 验证：`TestTheNumberBoxHandsOverTheSnappedValue`。
7. **提交记录随字段改型/消失残留**
   - 现象：`pluginTextCommits` 若不随字段退役，会重定位到下一个占同一 key 的字段。
   - 改动：`PluginSheet` 每帧按当前 schema 的 Text 字段退役该前缀下的记录。
   - 验证：由第 1 项的 pin 覆盖（记录被消费即删）。

### 第三批（关闭的面退役自己那一格状态）· commit 8ed57011

8. **快捷键录制中移开焦点后，裸 Escape 关掉整个设置页**
   - 现象：`settingsui.go:1174` 原来是 `if !a.recording && !a.customRecording && c.Shortcut(...)`——
     取消键要求录制器仍在焦点路径上。点一下别的控件，`recording` 仍是 true、全局快捷键仍
     被挂起，而 Escape 落到「关页」分支，录制器再也没法交还。
   - 改动：`recording` / `customRecording` 成为 Escape 的**前两个分支**；自定义录制行归位到
     `customDraftRow`。
   - 验证：`TestBuiltInRecorderGivesUpAfterFocusMoves` / `TestCustomRecorderGivesUpAfterFocusMoves`。
     变异 M3（退回原条件）实跑红：`a bare Escape left the built-in recorder armed after the focus moved`。
9. **自定义命令回车是提交手势，却留下没写回去的草稿**
   - 现象：`commitCustomCommand` 在 `next == "" || next == action` 时直接 return，`customEdits[key]`
     留着；行里继续画从未写回的文字（尾随空格或清空的输入都算）。
   - 改动：空值/等值也 `delete(a.customEdits, key)`，行回到存储的 action。
   - 验证：`TestCustomCommandNoOpEnterDropsTheDraft`。
10. **关掉配置表单后列表框停在上次偏移**
    - 现象：`closeConfig` 退役了草稿、错误、字段错误，**没有**退役 `configListScrolls`
      （`config.go` 的 `id\x00key`）。同一个 schema 重开时，列表从旧偏移开始。
    - 改动：按 `id + "\x00"` 前缀退役该表单的所有列表框偏移。
    - 验证：`TestClosingTheConfigFormForgetsItsListOffsets`。
11. **取消卸载后重开带上次勾选**
    - 现象：`uninstallDraft[id]` 在 Cancel、Escape、成功移除三条路径上都不删；取消一次再开，
      四个勾还是上一次的答案。
    - 改动：新增 `resetUninstallDraft`，三条关闭路径都调；并且帧末写回只在对话框**仍打开**时
      发生（否则关闭那一帧的 `draft` 会把刚退役的草稿又写回去）。
    - 验证：`TestUninstallCancelForgetsThePartialSelection` /
      `TestUninstallEscapeForgetsThePartialSelection`。变异 M4（`resetUninstallDraft` 空转）
      实跑红：`a cancelled dialog reopened with the abandoned ticks: {RemoveHostConfig:false
      RemoveToolData:true RemoveArtifacts:true}`。
12. **插件配置浮层关掉后草稿与滚动偏移都留着**
    - 现象：`shell/pluginconfig.go` 用**同一个** `a.Settings` 画每一个插件的浮层；草稿只在同一张
      面内按 schema 剪枝。浮层整个关掉时无人退役，重开读到上次的 `1e` / 上次的正文 / 上次的
      滚动位置。
    - 改动：新增 `App.ClosePluginSheet()`（退役正文草稿、数字草稿、`PluginScroll.Y`、
      `sheetPlugin`）；`shell/shell.go` 的 `noteResidency` 检测 `Launcher.ConfigOpen()` 由真变假
      那一帧调用它，并新增 `pluginConfigOpen` 记上一帧。
    - 验证：`TestP13HKPluginDraftAcrossClose` / `TestP13HKPluginNumberDraftAcrossClose` /
      `TestClosingThePluginSheetForgetsItsScroll`（settingsui）+ `TestClosingThePluginSheetRetiresItsEditorState`
      / `TestEscapeClosingThePluginSheetRetiresItsEditorState`（shell）。变异 M5（`noteResidency`
      去掉调用）实跑红：`the escaped sheet kept its scroll offset: Y = 42`。

### 第四批（新问句不带旧失败）· commit 0ec9b679 + eca65898

13. **取消后再问同一个集成，面板上是上一问的失败**
    - 现象：`disableConfirmation` / `removeConfirmation` 在错误存在时**用错误替换整句问句**。
      上一问的 `disk is full` 留在 `disableError[id]` 里，新问句根本看不见。
    - 改动：`requestDisable` / `requestRemove` 开新问句时删掉该 id 的旧失败。
    - 验证：`TestAFreshDisableQuestionDropsTheOldFailure` / `TestAFreshRemovalQuestionDropsTheOldFailure`
      （既有 pin 零改，从红转绿）。
    - **过程偏差（保留）**：`0ec9b679` 用 hunk 分段补丁暂存，`git diff` 在暂存后 hunk 编号
      重新从 1 开始，`n>=3` 的过滤器只取到了 `requestRemove` 的两段，`requestDisable` 的
      两段漏在索引外被一起提交。复跑单测发现 `TestAFreshDisableQuestion` 仍红，补
      `eca65898` 单独提交修正。**没有把红的提交算作过门槛**。

## 未接（显式记录，不算完成）

- `stash@{1}` 里 `internal/plugincfg/schema.go` 的 `coerce` 多选分支：改成「schema 未列举的
  值按原样保留」。它改的是**设置写回语义**（B 类），副作用是 UI 看不见的值再也取消不掉
  （用户只勾一个框，另外两个不可见的值仍留在集合里），并且与
  `TestCoerceShapesValuesLikeTheReferenceNormalizer` 的两条子用例直接冲突。按纪律「必须改变
  既有写回语义才能做好的，写进需拍板」，**不接**，维持「丢弃未列举值」。

## 门槛（实测，最终 HEAD `eca65898`）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净（rc 0）
GOOS=windows go vet ./...                              # 净（rc 0）
GOOS=darwin  go vet ./...                              # 净（rc 0）
GOOS=linux   go build ./...                            # 过
GOOS=windows go build ./...                            # 过
GOOS=darwin  go build ./...                            # 过
LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4 go test -count=1 -v ./...  # rc 0
```

- **PASS 3806 / FAIL 0**；`func Test` 计数 **2797**。
- 基线（起点 `53007b9b`，P13-hi 收尾实测）**3764 PASS / 0 FAIL / 2773 函数**。
- 台账 **V 0 / B 0 / S 0**：既有断言一个字未改，新增 21 条 pin。

## 变异（5 条实跑，全部看到真红并逐字节还原）

| # | 注入 | 变红的 pin | 原始失败串 | 还原 |
|---|---|---|---|---|
| M1 | `if edit != text` → `if true` | `TestEnterOnUnchangedTextDoesNotWrite` | `Enter on an unchanged field wrote it: [{custom_base_dir /already/stored}]` | `git checkout` + md5 一致 + `touch` 复跑绿 |
| M2 | 宿主换值时 `delete(a.pluginTextDrafts, key)` → `_ = 0` | `TestTheTextBoxShowsTheValueTheHostStored` | `en: the box still paints the text that was sent: "  /tmp/profile  " (host holds "/tmp/profile")` | 同上 |
| M3 | Escape 条件退回 `!a.recording && !a.customRecording && …` | 两条录制器 pin | `a bare Escape left the built-in recorder armed after the focus moved` | 同上 |
| M4 | `resetUninstallDraft` 的 `delete` → `_ = id` | 两条卸载 pin | `a cancelled dialog reopened with the abandoned ticks: {RemoveHostConfig:false RemoveToolData:true RemoveArtifacts:true}` | 同上 |
| M5 | `noteResidency` 去掉 `ClosePluginSheet()` 调用 | shell 两条 pin | `the escaped sheet kept its scroll offset: Y = 42` | 同上 |
| M6 | `boundedValue` 的夹界改 `if false && …` | 读数 / 网格 pin | `dark=false: the readout does not show the stored value clamped to the bound: ["Probe" "Number" "10000 items"]`；`initialNumberText(70000) = "70000", want "65535"`；`boundedValue(96) = 100, want 99` | 同上 |

每条还原后都 `md5sum -c` 逐字节核对，再 `touch` 源文件复跑一次（避开测试缓存）。

## 图

`FLOTER_PNG=1 go test -run TestTheTextBoxShowsTheValueTheHostStored ./internal/settingsui/` 导出
`/tmp/floter-P13-hk-text-adopted-en.png` / `-zh.png`（78274 / 96944 字节）。

图上看到的一处变化：文本框里画的是宿主存下的 `/tmp/profile`（**没有**首尾那两个空格），
行尾的字段槽宽度、下方错误行位置与输入框焦点环都未变——这轮改的是**这一帧画哪个字符串**，
不是槽面。

## 偷了什么（qiaomu-design · craft-loop 工艺密度菜单）

按「已有方向，只要打磨」路径（`craft-loop.md` + `preflight.md`），不走 Phase 2 四方向预览。

- **第 5 项「多层染色阴影」的反向用法**：本仓没有 blur，层级靠面差 + 受光边 + 阴影三层。
  本轮取的是它的**记账原则**——同一层级的三条账（读数、控件位置、派发值）必须由同一个
  函数产出。落成具体数值：`boundedValue` 成为唯一出口，`quantize` 锚点从 `0` 改为
  `field.Min`（`Min:1 / Step:10` 下 1.5 → **1**，改前 **0**），夹界条件从「总是」改为
  「`Max > Min`」（`Field{}` 的零值不再被误当 0 界）。
- **第 7 项「编辑细节 · hairline」的记账版**：一条状态只允许有一个主人。落成
  `pluginTextCommits` 的**生命周期只有一帧**（提交帧写下、下一帧消费即删），以及
  `ClosePluginSheet()` 一次退役四格状态（正文草稿 / 数字草稿 / `PluginScroll.Y` / `sheetPlugin`）。
- **第 10 项「收尾彩蛋 · hover 才出现的细节」的对偶**：不该出现的细节要能消失。落成
  三条关闭路径（Cancel / Escape / 成功移除）都调 `resetUninstallDraft`，且帧末写回加
  「对话框仍打开」守卫——关掉的那一帧不能把刚退役的草稿写回去。

## preflight 逐条

- **A 反 AI 套路**：本轮零新增视觉元素、零新增颜色/渐变/字体，不适用紫蓝渐变、纯黑、
  外发光、斜体等条目；**通过**（本轮改的是字符串与状态归属）。
- **B 设计读取一致性**：改动全部落在既有槽面内——输入框宽度、错误行位置、`PluginScroll`
  的滚动范围、卸载对话框的四个勾的默认值，逐条与 `docs/liquid-glass.md` §1–§5 的
  面差/受光边/圆角刻度无冲突；**通过**。
- **C 动效抽查**：本轮无动效改动（无过渡、无位移、无时长）。**不适用**，但已确认
  `PluginScroll.Y = 0` 是瞬时复位而非动画，不会与既有 `motion` 档冲突。
- **D2 中文页**：新增注释与报告为中文；用户可见文案**零改动**（i18n 表未动）。既有
  「`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2 正文 ≥14px」的冲突**沿用需拍板**
  状态，本轮不擅自动字号。
