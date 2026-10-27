# P10-y 报告：同一个键，两种画法必须同一个词序（Display ≡ Badge）

## 0. 一句话

`Display`（设置页 / 终端提示里的快捷键文字）与 `Badge`（启动器行右侧的键帽）**各写一套
修饰键词序**，同一个绑定在一个窗口里能读出两种画法；本轮把词序收成**一个产生者**
（`chordPrefix`），按参考 `formatShortcut` 的固定序（Control → Alt → Shift → Meta，
即 ⌃⌥⇧⌘），并顺手让键帽表真正能按平台名钉住（`CmdOrCtrl` 不再看宿主）。

## 1. 证据（实测，不是推测）

修前用一段临时探针（已删）跑出来的原始输出：

```
acc="Shift+Cmd+P"            darwin display="⌘⇧P" badge="⇧⌘X" | linux display="Super+Shift+P" badge="Shift+Super+X"
acc="Shift+CmdOrCtrl+P"      darwin display="⌘⇧P" badge="⌃⇧X" | linux display="Ctrl+Shift+P" badge="Ctrl+Shift+X"
acc="Ctrl+Alt+Shift+Super+K" darwin display="⌘⌃⌥⇧K" badge="⌃⌥⇧⌘X" | linux display="Super+Ctrl+Alt+Shift+K" badge="Ctrl+Alt+Shift+Super+X"
CmdOrCtrl badge on darwin-table from linux host: "⌃1"
```

三处根因（都是同一个问题的三种表现）：

1. **词序两个来源**。`displayFor`（`internal/shortcuts/shortcuts.go:397`）把归一化文本
   **按写入序**读回来，而 `join`（`shortcuts.go:217`）排的是
   `CmdOrCtrl, Cmd, Ctrl, Alt, Shift`——**meta 在前**；`badgeFor`（`shortcuts.go:557`）用
   `Parse` 的位标志自己再排一遍——**meta 在后**。于是 `Cmd+Shift+P` 在 macOS 上
   设置页写 `⌘⇧P`、启动器键帽写 `⇧⌘P`。
2. **`CmdOrCtrl` 看宿主**。`badgeFor` 走 `Parse`，`Parse`（`shortcuts.go:24`）的
   `CmdOrCtrl` 分支读 `runtime.GOOS`，于是「按平台名钉住」的承诺是假的：Linux 宿主上
   `badgeFor("darwin","CmdOrCtrl+1","1")` = `"⌃1"`，而同一输入的 `displayFor` = `"⌘1"`。
3. **`Parse` 不认的键丢修饰键**。`Normalize` 认得 `PrintScreen` / `CapsLock` / `NumLock` /
   `ScrollLock` / 四个媒体键（`shortcuts.go:64-81`），`parseKey` 不认
   （`shortcuts.go:59-136` 没有这些 case）→ `Badge` 直接只剩键名。参考 `formatShortcut`
   走的是 `parseShortcut`+`formatKey`，修饰键保留。

参考的判据只有一条：`main:src/shortcuts.ts:283-300` 的 `formatShortcut` 先 parse 成
`{ctrl,alt,shift,meta}` 四个布尔，再**固定顺序**拼 `⌃⌥⇧⌘`（macOS）/
`Ctrl+Alt+Shift+Win|Super`（其余）。这是 Apple 自己的顺序，也是 `badgeFor` 原本就在用的
顺序——**走偏的是 `Display` 那一侧**。

## 2. 落地

| 位置 | 现状 → 改成 |
|---|---|
| `shortcuts.go:397` `displayFor` | 按写入序吐符号/词 → `chordPrefix(goos, …) + displayKey(key)` |
| `shortcuts.go` 新增 `chordOrder` / `chordModifiers` | 词序**唯一**的定义处：`{Ctrl, Alt, Shift, Meta}`；`CmdOrCtrl` 按**传入的** goos 落 meta(darwin) 或 ctrl(其余) |
| `shortcuts.go` 新增 `chordPrefix` | 一个词序、两种拼写（macOS 连写 `⌃⌥⇧⌘`；其余每词后接 `+`）；`displayFor` 与 `badgeFor` 都只调它 |
| `shortcuts.go:557` `badgeFor` | `Parse` 位标志 + 自己排序 → `Normalize` + `chordPrefix`；`enter`/`Enter`→`↩`、无法解析返回 `key` 两条现状保留 |
| 注释勘误 | `shortcuts.go:408`、`:546-547` 的 `⌘⇧⌥⌃` → `⌃⌥⇧⌘`；`internal/launcher/keycap.go:32,57` 的举例 `⌘⇧1` → `⇧⌘1`（切分逻辑只看符号个数，与词序无关） |

**没有动**：`Normalize` / `join`（存储与注册拼写，`CmdOrCtrl,Cmd,Ctrl,Alt,Shift` 序）、
`Parse` / `Match` / `Equal` / `Duplicate` / `NormalizeBinding` / `NormalizeResultShortcut` /
`SameResultFamily`、`KeyName` / `FromKey` 的产出、键帽槽几何（`keycapUnits`/`keycapGapUnits`）。

## 3. 台账

**V=0 / B=0 / S=1**。

- S 类 1 条：`TestDisplay` 的 darwin `Cmd+Shift+P` 期望 `⌘⇧P` → `⇧⌘P`（旧结构：词序取自
  存储串的写入序；新结构：词序取自参考的规范序）。断言没放宽、没加容差、没跳。
- B=0：其余全是**新增**断言（旧 pin 一条未改）。其中两条新断言在修前是红的，正好是
  本轮两个「顺手修掉」的同源 bug：`badgeFor("darwin","CmdOrCtrl+1")`（修前 `⌃1`）与
  `badgeFor("linux","Ctrl+PrintScreen")`（修前只剩 `PrintScreen`）。
- 新测试 `TestDisplayAndBadgeAgree`：13 个加速器 × 3 平台，`displayFor` 必须等于
  `badgeFor`——这条是合约本体，修前 macOS 的多修饰键行全红。

## 4. 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2204（基线 2203，+1 = 新合约测试）
--- SKIP                                               → 15（= 基线）
--- FAIL                                               → 0
测试函数                                               → 1777（基线 1776，+1）
行                                                     → 36（34 ok + 2 no test files）
GOOS=linux/windows/darwin go build ./...               → 三平台过
```

本轮非视觉轮，且**本机（Linux）上的输出逐字节不变**（Linux 下 `Ctrl+Shift+C` 等仍是原样，
变的只有 `Super+Ctrl…` 这类错序组合与 macOS 表），所以不出图——出图也看不到差异，那不是证据。

## 5. mutation（4 条，全部实跑变红并逐字节还原）

还原方式：`cp` 备份 → 注入 → 跑 → `cp` 还原 → `md5sum -c` → `touch` 复跑（避开测试缓存）。
文件指纹 `c60c4bdd724adcf9fe9ff90583d6969c  internal/shortcuts/shortcuts.go`，四次还原全部
`成功`。

1. `chordOrder` 改成 `{Cmd, Ctrl, Alt, Shift}`（meta 在前，即修前 `Display` 的行为）：
   ```
   --- FAIL: TestDisplay
       displayFor(darwin, "Cmd+Shift+P") = "⌘⇧P", want "⇧⌘P"
       displayFor(linux, "Super+Shift+K") = "Super+Shift+K", want "Shift+Super+K"
   --- FAIL: TestBadge
       badgeFor(darwin, "Cmd+Shift+P", "P") = "⌘⇧P", want "⇧⌘P"
   ```
   `TestDisplayAndBadgeAgree` **仍绿**——两面同错时它抓不到，这正是逐面表存在的理由。
2. `badgeFor` 退回改前的 `Parse` 实现：
   ```
   --- FAIL: TestBadge
       badgeFor(darwin, "CmdOrCtrl+1", "1") = "⌃1", want "⌘1"
       badgeFor(linux, "Ctrl+PrintScreen", "PrintScreen") = "PrintScreen", want "Ctrl+PrintScreen"
   --- FAIL: TestDisplayAndBadgeAgree
       darwin: displayFor("CmdOrCtrl+1") = "⌘1", badgeFor = "⌃1" — one binding, one word order
   ```
3. `chordModifiers` 的 `CmdOrCtrl` 一律落 ctrl：
   ```
   --- FAIL: TestDisplay
       displayFor(darwin, "CmdOrCtrl+Shift+P") = "⌃⇧P", want "⇧⌘P"
   --- FAIL: TestBadge
       badgeFor(darwin, "CmdOrCtrl+1", "1") = "⌃1", want "⌘1"
   ```
4. `chordOrder` 删掉 `modShift`：
   ```
   --- FAIL: TestDisplay
       displayFor(linux, "Ctrl+Shift+C") = "Ctrl+C", want "Ctrl+Shift+C"
       displayFor(darwin, "Ctrl+Alt+Shift+Cmd+K") = "⌃⌥⌘K", want "⌃⌥⇧⌘K"
   ```

## 6. 取舍 / 判不准

1. **为什么不改存储序**。`join` 写的是 `CmdOrCtrl,Cmd,Ctrl,Alt,Shift`（meta 在前），参考
   `shortcutFromEvent` 写的是 `Ctrl,Alt,Shift,Meta`。两种拼写经 `Normalize`/`Equal` 等价，
   旧设置文件两边都能读；改它等于动「设置写回」与一批既有 pin，换不来任何可见收益。
   本轮只统一**画出来**的顺序。
2. **非 macOS 平台 `Shift+Win+K` 读起来不自然**（Windows 习惯 `Win+Shift+K`）。但那是参考
   `formatShortcut` 的原文，也是旧版一直显示的样子，本轮以「与参考、与键帽一致」为先；
   若要改成平台各自的习惯序，那是新契约，得单独拍板。
3. `Esc` 是否显示成 `Escape`（参考 `formatKey` 不映射 → 显示 `Escape`；本仓 `displayKey`
   显示 `Esc`）**没动**：那是可见文案，按纪律进拍板清单，见下。

## 7. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 54 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. register 只留 `cmd`、`open` 是否拒一切 query、未知宿主是否改成静默忽略，三条拍板项照旧。
4. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
5. icns 转换落盘缓存（P10-t）、SVG/XPM 图标（P10-s）仍是拍板项。
6. 剪贴板族未动：轮询间隔 `clipboardPoll=600ms` vs 参考 `POLL_INTERVAL_MS=900`；
   写回面与折叠的交界（P10-x 报告「仍待」第 7 条）——「复制回来的条目该不该置顶」要先拍板。
7. **新拍板候选**：`Display`/`Badge` 在非 macOS 平台是否改成各平台自己的词序
   （Windows `Win+Shift+K`、Linux `Super+Shift+K`）；以及 `Esc` 是否显示成 `Escape`
   （参考如此，本仓现在是 `Esc`）。两条都是可见文案/习惯，本轮不擅自动。
8. 下一轮建议：离开 `shortcuts`，回「三平台一致性」挑一处——`internal/spawn`
   的 `detach_unix`/`detach_windows` 两臂与参考的 `spawn` 尚未逐条核对过。
