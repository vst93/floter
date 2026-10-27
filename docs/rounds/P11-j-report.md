# P11-j 报告 · 一个键被占用的判据只有一套（结果族算一整族）

## 0. 一句话

`select_result` 是一个**族**（存族头、应答整族数字），但「这个和弦被占了吗」有两个答案：
自定义全局快捷键那条认族，应用自己的快捷键那条只认字面 —— 于是设置页接受 `Ctrl+5`/`Cmd+5`，
启动器第 5 行照样画着 `5` 键帽，按下去却被应用键先吞掉。本轮把这个问题收成**一个**判据。

## 1. 根因（file:line，动手前逐一读过）

| 位置 | 改前 |
|---|---|
| `internal/settings/shortcutmap.go` `CustomShortcutConflict` | 自己写循环：`Equal` **或**（候选是 `select_result` 且 `SameResultFamily`）→ 认族 |
| `internal/settings/shortcutmap.go` `ShortcutRecordingOf` | 自己写循环：只有 `Equal` → 不认族 |
| `internal/settings/shortcutmap.go` `SetShortcut` | 只校验「动作 id 认识 + 和弦能解析」→ 写入口能造出冲突 |

死键是怎么形成的（三平台同错，无 GOOS 分支）：

1. 录 `new_command = Cmd+5` → `ShortcutRecordingOf` 返回 `ShortcutRecorded` → 落盘；
2. `internal/launcher/launcher.go:787` 第 5 行仍画 `⌘5`（键帽取自 `ShortcutSelectResult`）；
3. `internal/launcher/fieldkeys.go:155-172` 的 `appChordKeys` **先**比应用键、命中即 `return true`
   吞掉事件，`pendingDigit` 永不置位 → 按 `⌘5` 不选第 5 行，反而走 `launcher.go:709-720`
   的 `ShortcutNewCommand` → `Actions.Dismiss`（把窗口收起来）。

`ShortcutRefused` 自己的注释写着「one key never runs two things」，实现只兑现了字面那一半。

## 2. 落地

1. **新增唯一判据** `shortcutShadowedBy(s Settings, accelerator string, ignore ...string) string`：
   一次 `ShortcutsOf(s)` 取整张表（改前每个候选动作各调一次 `Shortcut`，O(n²) 重算），
   逐动作比字面 `Equal`，或该动作是 `select_result` 且 `SameResultFamily`，返回占用的动作 id。
2. `CustomShortcutConflict` → 改调它，只保留自己那条**故意的**排除（`ShortcutToggleWindow`：
   召出键由 `internal/shell/custom.go` 拿 `a.summonKey` 单独判，注释已写明）。
3. `ShortcutRecordingOf` → 改调它（`ignore` = 本动作），族算一整族。
4. `SetShortcut` → 同一条判据：被别的动作（含结果族）占用的和弦**拒绝写**、返回 `false`；
   重写自己已有的绑定仍然成功（`ignore` 含自己）。写入口从此不会产出两个动作同一个键。

未动：`FromKey` / `Normalize` / `NormalizeResultShortcut` / `Display` / `Badge` / `Equal`，
以及 `internal/launcher`、`internal/shell`、`internal/settingsui` 的任何接线与文案。

## 3. V / B / S 台账

- **V = 0**（非视觉轮，不出图）。
- **B = 0 改动**：`internal/launcher`、`internal/shell`、`internal/settingsui`、`internal/terminalui`
  的既有 pin 一行未改，全绿。
- **S = 1**：`internal/settings/shortcutmap_test.go`
  「a digit outside the head: `Cmd+Shift+5` → `ShortcutRecorded`」这条断言**钉的就是缺陷**
  （族里第 5 个数字被判成「没人占」）。改为 `ShortcutRefused`，注释写明
  `P11-j 重定基线：旧判据只看族头（Equal）→ 新判据看整族`；同时补一条正例
  `Cmd+Alt+5`（同一个数字、**别的**修饰键 → 仍是 `ShortcutRecorded`），确保不是把族判据
  放宽成「带数字就拒」。

新增测试函数 1 个：`TestSetShortcutRefusesAChordTheResultFamilyHolds`
（写入口的四条：族内数字拒 + 不落盘 / 族外同数字收 / 重写自己收 / 别人字面持有拒）。

## 4. 门槛（全部实测）

```
gofmt -l cmd internal                 → 空
go vet ./...                          → 净
go test -count=1 ./...                → 全绿，34 ok + 2 no test files（36 行）
--- PASS 计数                          → 2257（基线 2256）
--- SKIP                              → 15
--- FAIL                              → 0
测试函数                              → 1828（基线 1827，+1）
GOOS=linux/windows/darwin go build ./... → 三平台全过
```

**基线抖动说明（如实记录）**：本轮改前第一次全量跑 `grep -c -- "--- PASS"` 得 **2255**，
紧接着两次复跑都是 **2256**（`--- FAIL` 两次都是 0、`--- SKIP` 都是 15）。取 **2256** 为基线
（与上一轮报告一致）；这条 ±1 抖动不是本轮引入的（改前就在），未定位到具体测试，留作观察项。

## 5. 变异（3 条实跑，逐条变红并 `md5sum -c` 还原）

改前 `md5sum internal/settings/shortcutmap.go` 存档 → 注入 → 跑 → `cp` 还原 → `md5sum -c` → `touch` 复跑。

| # | 注入 | 变红的 pin（原始报错串） |
|---|---|---|
| 1 | `shortcutShadowedBy` 里族判据改成 `if false && action == ShortcutSelectResult && …` | `--- FAIL: TestShortcutRecordingRefusesAChordAnotherActionHolds`<br>`shortcutmap_test.go:237: another digit of the family: got 0, want refused`<br>`--- FAIL: TestSetShortcutRefusesAChordTheResultFamilyHolds`<br>`shortcutmap_test.go:256: a digit of the result family was stored for another action` |
| 2 | `SetShortcut` 的守卫改成 `if false && shortcutShadowedBy(*s, normalized, action) != ""` | `--- FAIL: TestSetShortcutRefusesAChordTheResultFamilyHolds`<br>`shortcutmap_test.go:256: a digit of the result family was stored for another action` |
| 3 | `shortcutShadowedBy` 里忽略集改成 `if false && containsAction(ignore, action)` | `--- FAIL: TestSetShortcutRefusesAChordTheResultFamilyHolds`<br>`shortcutmap_test.go:270: rewriting an action's own binding was refused` |

三次 `md5sum -c` 均输出 `internal/settings/shortcutmap.go: 成功`。

## 6. 这张「图」上看到的一处变化（非视觉轮的等价物）

无 PNG（本轮只动判据，不动墨与面）。可观察的变化是一张**表**：设置页 → 快捷键 →
把 `new_command` 录成 `⌘5`（macOS）/ `Ctrl+5`（Windows、Linux）时，该行下面现在会出现
`copy.ShortcutRejected` 那句（复用既有文案，未新增 i18n 键），而不是录上一个
启动器第 5 行「画着却按不动」的死键。

## 7. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 65 轮。
2. 需拍板清单不变（见 `.pending-bugs.md` P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 +
   计算器每次写要不要 `loadLocked` + `usage.Record` 写失败是否回滚 + 非 macOS 快捷键词序 +
   `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`）。
3. 本轮**故意没做**的两件同族小事（都属「要不要改变既有可见行为」，不擅自做）：
   - `ShortcutToggleWindow` 在快捷键表里的值是否该跟着 `a.summonKey` 实时同步：
     现在表里它是 `defaultShortcuts()` 的 `Ctrl+Space`（或上次录进表的值），而真正生效的
     召出键在 `a.summonKey`。`ShortcutRecordingOf` 拿表里的值判冲突，所以「先录召出键、
     再录应用键」时两者可能对不上。要改就得决定「表里的 toggle_window 到底是展示还是真值」。
   - 结果族的**键帽**在数字被应用键占用时是否该隐藏（现在是照画）。
     修了判据之后新的冲突已经录不进来，只剩历史设置文件可能带进来；
     要不要对老文件做一次清洗/迁移，需要拍板。
4. 下一处候选：`internal/inputtext`（68 行 / 7 条）的编辑行为三平台是否同一套；
   `internal/pinyin`（77 行 / 3 条）只被 `internal/apps/darwin.go` 使用，是否需要补一条
   「非 macOS 也要拼音首字母」的答案（现在 macOS 独有，属**平台不一致**，但改它就是加功能，需拍板）。
