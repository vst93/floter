# P13-gy 报告 · 一个指针答案，一个速度

分支 `mygo-rewrite`，起点 HEAD `c9152758`（P13-gx）。本轮是**交接残留清点 + 接续**：
`stash@{0}`（2026-11-05 12:29 `stale-kill`）里那条孤儿补丁正是这件事的半成品。

**一句话**：九处指针面在指针落下的那一帧直接跳到答案，另 13 处走 120ms —— 同一个指针动作
在一个应用里两种速度；本轮收成 22 处同一个速度，并给出每个面深浅两套的三帧读数。

## 逐条（现象 → 改动 → 验证）

1. **弹层胶囊**（`settingsui/pluginsheet.go` `sheetActionButton`）
   现象：`f0 == f400`，而同族的 `extensionActionFace` / `barButton` / `uninstallActionButton` 走 120ms。
   改动：加 `Transition{Colors: true, Duration: theme.ColorStep}`。
   验证：`pointerstep_test.go` 逐字节比 `rest→f0 相同 / rest→落定 不同`，深浅两套绿。
2. **会话行 28u 小格**（`settingsui/rows.go` `sessionIconButton`）
   现象：一帧跳；同族 30u / 28u / 页头小格都走 120ms。
   改动：同上。验证：`light 742 → 742 → 664` / `dark 44 → 44 → 146`。
3. **杀掉会话的暖色问句**（`rows.go` `sessionKillConfirm`）
   现象：一帧跳；电源确认带的 execute 键走 120ms。
   改动：同上。验证：`light 695 → 695 → 664` / `dark 102 → 102 → 147`。
4. **快捷键录制钮**（`settingsui/settingsui.go` `shortcutRecorder`）
   现象：三个 rung 只换面、一帧到站。改动：同上。验证：`light 690 → 690 → 664` / `dark 118 → 118 → 146`。
5. **运行输出折叠行**（`settingsui.go` `runOutputToggle`）
   现象：三档面一帧跳。改动：同上。验证：账本 `settingsui.go: 2 → 4` 处（动态 pin 缺，见「仍待」）。
6. **捕获输出窗口的条子键**（`shell/detach.go` `barButton`）
   现象：Rerun / Close 一帧跳。改动：同上。
   验证：`light 246114 → 246114 → 220114` / `dark 20228 → 20228 → 53228`。
7. **toast 动作 chip**（`shell/notices.go`）
   现象：浮层里一帧跳。改动：同上。验证：`−14000 → −14000 → −21114`（深浅同值）。
8. **toast ✕**（`shell/notices.go`）
   现象：同一块浮面上两个控件两种速度。改动：同上。验证：`light 0 → 0 → −26114` / `dark 0 → 0 → 32886`。
9. **便条 Close**（`shell/pinned.go`）
   现象：与 6 同一根条子、同一个键，同样一帧跳。改动：同上。
   验证：`light 246114 → 246114 → 220114` / `dark 20228 → 20228 → 53228`。
10. **账本扩表**（`internal/ci/colorstep_contract_test.go`）
    现象：7 文件 / 13 处。改动：11 文件 / 22 处，按文件钉死。
    验证：删掉任一处 → `rows.go: 5 colour transitions, want 6` 红。
11. **探针取样时机**（6 个测试文件）
    现象：改前这些 pin 在指针落下那一帧就读面，**读得到只因产品一帧跳过去**；产品一走 120ms
    它们读到的是淡入半路（`litpair` 读到 `+6` 级、会话问句读到与卡片同色）。
    改动：`tt.Advance(300ms); tt.Frame()` 之后再读。**数值一个没改**。
    验证：`go test ./internal/settingsui/` 全绿；两个失败项（`TestTheLitEdgeSpeaksInLevelsNotAlpha`、
    `TestTheSessionKillQuestionWearsTheWarmPill`）在改前稳定红、改后稳定绿。

## 台账

**V 0 / B 0 / S 0**：没有视觉数值改动（只加过渡），没有行为/度量改动，没有结构改动；
改的测试全是**取样时机**，断言的数值一个字节没动。

## 变异（实跑）

- **M1** 删掉 `sessionIconButton` 的 `Transition`（= 改前实现）→ 两个 pin 同时红：
  `pointerstep_test.go:142: light: session icon button answered the pointer in the frame it landed (742 → 664, -78)`
  与 `dark ... (44 → 146, +102)`；`internal/ci`：`internal/settingsui/rows.go: 5 colour transitions, want 6`。
- **M2** 删掉 toast ✕ 的 `Transition` →
  `pointerstep_test.go:120: light: toast / dismiss answered the pointer in the frame it landed (0 → -26114, -26114)`
  与 `dark ... (0 → 32886, +32886)`。
- **M3** `Duration: 0` → 账本红、动态 pin **仍绿**。这条是量出来的事实，已写进 §6.175：
  「f0 还是静息面」不能区分 0ms 与 120ms（框架在指针那一帧先画静息、次帧起算），
  这一族 pin 的两半各守一件事。
- 还原：`md5sum -c /tmp/p13gy-md5.txt`（`rows.go` / `notices.go`）逐字节成功 → `touch` → 复跑绿。

## 图

`/tmp/floter-P13-gy-{capture,note,toast}-{light,dark}.png`（`TestDumpP13GYPointerStep`，
落定帧）。这一轮改的是**时长**，静帧只能证「落在哪一档」不能证「走了多久」——像素证据是三帧
读数表。逐张复核（读像素，不是看感觉）：`capture-light` 与 `note-light` 是同一副条子、
同一枚 ✕，落定后键面读数都从 246114 降到 220114（同一 builder、同一档）。

## 偷了什么（qiaomu-design / craft-loop）

- 工艺密度菜单**第 4 项「签名交互」** → 落成「应用里每一枚面回答指针都用同一个 120ms」
  （`theme.ColorStep` = 参考 `--dur-2`），实测落定差：toast ✕ 浅 −26114 / 深 +32886。
- 菜单**第 12 项「材质细节」** → 不新加材质，改为「同一材质的两个控件同一速度」：
  toast 的 chip 与 ✕ 此前两种速度（一个 120ms 一个 0），现在一种。
- 菜单**第 6 项「编辑细节（hairline / 刻度）」** → 账本按文件钉死 22 处，
  新增一处即红（`colorStepSites`）。

## preflight

- **A（反 AI 套路）**：过。没有渐变白边、没有发光、没有模糊冒充玻璃；这一轮只改时长，
  没有引入任何新的视觉语言。
- **B（设计读取一致性）**：过。`theme.ColorStep` 是单一来源，22 处都读常量；
  账本按文件计数，不按注释。
- **C（动效抽查）**：过。§5 第一行 120ms 现在覆盖 22 处；`--dur-1`(80ms) 与 `--dur-3`(160ms)
  的语义未被混用（入场的浮层仍走 160ms）。
- **D2（中文页）**：本轮没有动任何文案、字号与行高，不适用；沿用「`tokens.Caption` 9.9px
  对 preflight D2 正文 ≥14px」那条**需拍板**（未擅自改）。

## 交接残留清点（8 条 `stale-kill`）

机械判据：把 stash 补丁里每一条 `+` 行拿去 `git show HEAD:<file>` 里找，逐条计数。

| ref | 时间 | 判断 | 处置 |
|---|---|---|---|
| `stash@{0}` | 12:29 | **有独有工作**：产品侧 9 处 `Transition` + 账本扩表（69 条 `+` 行，HEAD 全无） | **本轮接上并扩成 11 项**，`git apply -3` 干净落地 |
| `stash@{1}` | 11:59 | 已被 HEAD 覆盖（`git apply --check --reverse` 通过） | drop |
| `stash@{2}` | 11:03 | 前半（§6.169 的 169-1…169-8）已全部进 HEAD；独有部分 = 9 处 `Transition` + 探针时机，{0} 的子集 | drop（{0} 已接上） |
| `stash@{3}` | 10:32 | 同 {2}，更早 | drop |
| `stash@{4}` | 10:02 | `pinned.family` / `newPinned` / `detachedText` 注释全部已在 HEAD（P13-gw）；无独有工作 | drop |
| `stash@{5}` | 09:32 | 同 {2} 的更早版本（round 号还写成 `gu`） | drop |
| `stash@{6}` | 08:33 | 同 {5} | drop |
| `stash@{7}` | 07:31 | 已被 HEAD 覆盖（reverse-apply 通过） | drop |

`stash@{0}` 的第三父（未跟踪文件）里还有两个探针：`internal/shell/pointerstep_test.go`
与 `internal/settingsui/pointerstep_test.go` —— 已取回、改 round 号、扩到 7 个面 × 深浅两套，
并新增 `TestDumpP13GYPointerStep`。

## 仍待

- `runOutputToggle`（本轮第 5 项）只有账本 pin，没有动态三帧 pin：它的折叠行要一份带
  run outputs 的设置夹具才能单独渲染，本轮没为本条去造夹具，记在这里。
- 沿用 `.pending-bugs.md` 的「需拍板」表（`tokens.Caption` 9.9px、启动器 36px 盒、`140-5` 等）。

## 下一轮

通用队列 · **禁用 / 只读面**（15 个 `Disabled(true)` 调用点，`theme.Tokens` 里没有 `Disabled` 档）：
先量 `internal/settingsui/rows.go` 那枚「建时就禁用」的胶囊与可用态差几级、差在哪一维。
