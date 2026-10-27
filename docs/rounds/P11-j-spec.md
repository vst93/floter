# P11-j 规格 · 一个键被占用的判据只有一套（结果族算一整族）

## 1. 意图

「这个和弦是不是已经有人应答了」这个问题，本仓有**两个答案**：

- 给**自定义全局快捷键**用的 `settings.CustomShortcutConflict`（`internal/settings/shortcutmap.go:157-176`）
  认整族：`select_result` 存的是族头（用户选的修饰键 + 数字 `1`），但**任何数字**都在族里，
  所以 `Ctrl+Shift+7` 也算被占（`SameResultFamily`，`internal/shortcuts/shortcuts.go:300-311`）。
- 给**应用自己的快捷键**用的 `settings.ShortcutRecordingOf`（`shortcutmap.go:271-291`）
  只比字面：`shortcuts.Equal(existing, normalized)`，族头 `Ctrl+1` 之外的 `Ctrl+5` **不算被占**。

后果（三平台同错，与 GOOS 无关）：

1. 设置页 → 快捷键 → 把 `new_command` 录成 `Cmd+5`（macOS）或 `Ctrl+5`（Windows/Linux），
   `ShortcutRecordingOf` 返回 `ShortcutRecorded`，页面**接受**并落盘。
2. 启动器列表上第 5 行仍然画着 `⌘5` 键帽（`internal/launcher/launcher.go:787`：
   `a.badge(a.shortcut(settings.ShortcutSelectResult), strconv.Itoa(number))`）。
3. 但 `appChordKeys`（`internal/launcher/fieldkeys.go:155-172`）先比应用键、命中就 `return true`
   **吞掉**事件，`pendingDigit` 永远不置位 → 第 5 行的键帽是个**死键**：画着，按下去不选行，
   反而把窗口收起来（`launcher.go:709-720` 的 `ShortcutNewCommand` → `Actions.Dismiss`）。

即：页面说「录上了」，界面画着键帽，按键却被别人吃掉。这正是 `ShortcutRefused` 自己注释里写的
契约「one key never runs two things」（`shortcutmap.go:250-255`）没有兑现的那一半。

## 2. 为什么现在做

- 用户实测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志）连续第 65 轮卡在实机复测，
  本仓无可改代码；`docs/raycast-design.md` §6 差距清单全部 `已落地`，剩下两条明写「本版不做 / 需拍板」。
- 上一轮「仍待」第 1 条同样卡实机 → 按用户定的优先级 4「三平台一致性 → 基础功能 → 像素细节」
  自查。本条属**基础功能 + 平台一致性**（macOS / Windows / Linux 走同一份 `ShortcutRecordingOf`，
  没有 GOOS 分支，Linux CI 上钉住三平台同一答案），不牵涉任何需拍板的交互取向：
  拒绝一个「录了也不会生效」的键是把行为对齐到参考（`useShortcutCapture.ts:112-136` 的
  `capture` 就是拒绝冲突和弦），不是新交互。

## 3. 逐条落地项（动手前已读代码确认真实行号）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settings/shortcutmap.go:271-291`（`ShortcutRecordingOf`） | 冲突判据是 `shortcuts.Equal(Shortcut(s, candidate), normalized)`，只认字面 | 走新的唯一判据 `shortcutShadowedBy`，把 `select_result` 当**一整族**判（`SameResultFamily`） |
| 2 | `internal/settings/shortcutmap.go:157-176`（`CustomShortcutConflict`） | 自己写了一遍同样的循环，族判据在循环体里 | 改调 `shortcutShadowedBy`，只保留自己那条**故意的**排除（`toggle_window` 由 `shell/custom.go:95` 拿 `a.summonKey` 单独判） |
| 3 | `internal/settings/shortcutmap.go:205-215`（`SetShortcut`） | 只校验「动作 id 认识 + 和弦能解析」，写入口能造出冲突 | 同一条判据：被别的动作（含结果族）占用的和弦拒绝写，返回 `false`；重写自己已有的绑定仍然成功 |
| 4 | `internal/settings/shortcutmap.go` 新增 `shortcutShadowedBy(s, accelerator, ignore...)` | 不存在 | 一次 `ShortcutsOf(s)` 取整张表（现在是每个候选动作各调一次 `Shortcut`，O(n²) 重算），逐动作比：字面 `Equal`，或该动作是 `select_result` 且 `SameResultFamily` |

**行为契约零改动红线**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一律不动。
本轮只改「哪个和弦算被占用」这一个判据及其三处调用，不新增文案（`copy.ShortcutRejected` 复用）、
不新增 UI 状态、不动 `FromKey`/`Normalize`/`Display`/`Badge`。

## 4. 台账预判

- **B**（行为/度量/接线）零改动：`internal/launcher/*`、`internal/shell/*`、`internal/settingsui/*`
  的既有 pin 必须全绿（`appChordKeys` / `recordShortcut` / `setShortcut` 的接线一行不动）。
- **S** 一条：`internal/settings/shortcutmap_test.go:231-233`
  现在断言 `ShortcutRecordingOf(family, ShortcutCopySelection, "Cmd+Shift+5") == ShortcutRecorded`，
  钉的正是上面第 3 步那个死键。改成 `ShortcutRefused`，注释写「旧判据只看族头 → 新判据看整族」。
- **V** 无（非视觉轮）。

## 5. 门槛命令与 PASS 基线

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（本轮实测，见报告 §3）：PASS **2256** / 36 行（34 ok + 2 no test files）/ 15 SKIP / 0 FAIL /
测试函数 **1827**。

## 6. mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. **把族判据摘掉**：`shortcutShadowedBy` 里删掉 `candidate == ShortcutSelectResult && shortcuts.SameResultFamily(...)`
   那一段 → `TestShortcutRecordingRefusesAChordAnotherActionHolds`（`Cmd+Shift+5` → 期望 `Refused`）
   与新增的 `TestSetShortcutRefusesAChordTheResultFamilyHolds` 一起变红。
2. **把 `SetShortcut` 的守卫摘掉**：`SetShortcut` 里去掉 `shortcutShadowedBy` 调用
   → 新增的写入口 pin（`SetShortcut(NewCommand, "Cmd+5")` 必须 `false` 且不落盘）变红。
3. **让忽略集失效**：`shortcutShadowedBy` 里不跳过 `ignore` → `TestShortcutRecordingOf...`
   的「the action's own chord → Unchanged」变红（自己的绑定把自己判成冲突）。
