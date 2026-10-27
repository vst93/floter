# P10-y 规格：同一个键，两种画法必须同一个词序（Display ≡ Badge，三平台一致）

## 意图

一句话：**一个加速器的修饰键词序由参考的固定序决定（Control → Alt → Shift → Meta，即
⌃⌥⇧⌘），显示面（`Display`，设置页/终端提示）与键帽面（`Badge`，启动器行/页脚）必须
从同一个词序产生；键帽表要能按平台名钉住，不看宿主。**

为什么现在做（证据，实测）：

- 现状两处**各写一套词序**：
  - `internal/shortcuts/shortcuts.go:397` `displayFor` 把归一化文本**按写入序**读回来
    （`parts` 的 modifiers 顺序就是 `join`（`shortcuts.go:217`）排出来的
    `CmdOrCtrl, Cmd, Ctrl, Alt, Shift`，即 **meta 在前**）；
  - `internal/shortcuts/shortcuts.go:557` `badgeFor` 用 `Parse` 的位标志，按
    **Ctrl → Alt → Shift → Super**（即 **meta 在后**）自己再排一遍。
  - 实测（本轮 `go test` 里临时探针，已删）：`displayFor("darwin","Cmd+Shift+P")` =
    `"⌘⇧P"`，而 `badgeFor("darwin","Cmd+Shift+P","P")` = `"⇧⌘P"`。**同一个绑定，同一个
    窗口里两种画法**——设置页的快捷键文字是 ⌘⇧P，启动器行右侧的键帽是 ⇧⌘P。
- 参考只有一种词序：`main:src/shortcuts.ts` 的 `formatShortcut` 先 parse 成
  `{ctrl,alt,shift,meta}` 四个布尔，再**固定顺序**拼：`⌃⌥⇧⌘`（macOS）/ `Ctrl+Alt+Shift+Win|Super`
  （其余平台）。这正是 Apple HIG 的顺序（Control, Option, Shift, Command），也是
  `badgeFor` 已经在用的顺序。所以 `Display` 是走偏的那一侧。
- 可及性：多修饰键的自定义快捷键都会命中。设置文件里手写/旧版写下的
  `Cmd+Shift+P`、`Shift+Ctrl+Q` 一律受影响；macOS 上 `⇧⌘` 是**最常见**的组合序
  （Safari 的 ⇧⌘\、Xcode 的 ⇧⌘F），所以这不是理论分歧。
- 同族第二处：`badgeFor`（`shortcuts.go:558`）走 `Parse`，而 `Parse`
  （`shortcuts.go:24`）的 `CmdOrCtrl` 分支读 **`runtime.GOOS`**——于是「按平台名钉住」的
  承诺是假的：Linux 宿主上 `badgeFor("darwin","CmdOrCtrl+1","1")` 实测得 `"⌃1"`
  （应为 `"⌘1"`），`displayFor` 同一输入得 `"⌘1"`。两张表的可钉性不对称。
- 同族第三处（同一根因的副作用）：`Normalize` 认得而 `Parse` 不认得的键
  （`PrintScreen` / `CapsLock` / `NumLock` / `ScrollLock` / 四个媒体键，
  `shortcuts.go:64-81`）在 `Badge` 里**丢掉全部修饰键**：`Badge("Ctrl+PrintScreen", …)`
  只剩键名，而参考 `formatShortcut` 保留修饰键。两边同样是一个 `Normalize` 就能答的问题。

## 逐条落地项

1. `internal/shortcuts/shortcuts.go:397` `displayFor`
   - 现状：遍历 `parts[:len-1]`（`join` 的 meta 在前序）按顺序吐符号/词。
   - 应改成：修饰键先过一个新的 `chordModifiers(goos, modifiers)`，把它归约成
     **Ctrl → Alt → Shift → Meta 的固定序**（`CmdOrCtrl` 按 `goos` 落到 meta（darwin）
     或 ctrl（其余）），再吐符号/词。
2. 同一文件新增 `chordPrefix(goos, modifiers) string`：一个词序、两种拼写
   （macOS 连写符号 `⌃⌥⇧⌘`，其余平台每词后接 `+`）。`displayFor` 与 `badgeFor` 都只调它，
   **词序只有一个产生者**。
3. `shortcuts.go:557` `badgeFor`
   - 现状：`Parse(accelerator)` 取位标志（`CmdOrCtrl` 走 `runtime.GOOS`），再自己排一遍序。
   - 应改成：`Normalize` + `chordModifiers(goos, …)` + `chordPrefix`；`enter`/`Enter` → `↩`
     与「无法解析则原样返回 `key`」两条现状保留。副作用即修掉上面第三处（`Parse` 不认的键
     不再丢修饰键）与第二处（`CmdOrCtrl` 按传入的平台解析）。
4. 注释勘误：`shortcuts.go:408`、`shortcuts.go:546-547` 把词序写成 `⌘⇧⌥⌃`（meta 在前），
   与代码相反；`internal/launcher/keycap.go:32,57` 的举例 `⌘⇧1` 同步改成 `⇧⌘1`。
5. 测试（`internal/shortcuts/shortcuts_test.go`）：
   - `TestDisplay` 现有 darwin `Cmd+Shift+P` 的期望 `⌘⇧P` → **`⇧⌘P`**（S 类重定基线：
     词序来源从「写入序」改成「规范序」）；补写入序无关、四修饰键全序、`CmdOrCtrl` 三平台。
   - `TestBadge` 补：写入序无关、`CmdOrCtrl` 在 darwin/windows 两张表上的期望（**修前红**）、
     `Parse` 不认得的键（`Ctrl+PrintScreen`）保留修饰键（**修前红**）。
   - 新增 `TestDisplayAndBadgeAgree`：同一批加速器 × {darwin,linux,windows}，
     `displayFor(goos,a)` 必须等于 `badgeFor(goos,a,<a 的键>)`——这条是本轮的合约本体
     （**修前红**，macOS 的多修饰键行全红）。

## 行为契约零改动红线

- 不改：`Normalize` / `join`（**存储与注册拼写**，`CmdOrCtrl,Cmd,Ctrl,Alt,Shift` 序）——
  那是写回设置与注册到框架的字符串，动它等于改设置写回与既有 pin；本轮只改**画出来**的顺序。
- 不改：`Parse` / `Match` / `Equal` / `Duplicate` / `NormalizeBinding` / `NormalizeResultShortcut`
  / `SameResultFamily` 的判据；`KeyName` 与 `FromKey` 的产出拼写。
- 不改：高度链、键盘模型、会话管理、i18n 文案、命令接线、键帽槽几何（`keycapUnits` /
  `keycapGapUnits` 与 `keycapLabels` 的切分只看符号个数，与词序无关）。
- `Esc` 是否显示成 `Escape`（参考 `formatKey` 不映射，本仓 `displayKey` 显示 `Esc`）**不在本轮**：
  那是文案改动，进「需拍板」候选，见报告「仍待」。

## 门槛命令 + PASS 基线

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

本轮实测基线（HEAD `17ad172`，本轮动手前跑）：**2203 PASS / 15 SKIP / 0 FAIL /
1776 测试函数 / 36 行（34 ok + 2 no test files）**。

## mutation 计划（≥2 条，每条点名会让哪个 pin 变红）

1. `chordModifiers` 的固定序把 `modCmd` 挪到最前（meta 在前，即现状 `Display` 的行为）：
   `TestDisplay` 的 darwin `Cmd+Shift+P` 与 `TestBadge` 的 darwin 顺序行变红；
   `TestDisplayAndBadgeAgree` **仍绿**（两面同错）——这正是为什么需要那两张逐面表。
2. `badgeFor` 退回 `Parse(accelerator)`：`TestBadge` 的 `CmdOrCtrl` darwin 行
   （Linux 宿主上得 `⌃1`）与 `TestDisplayAndBadgeAgree` 变红。
3. `chordModifiers` 里删掉 `modShift`：`TestDisplay`/`TestBadge` 里含 Shift 的行全红。
4. `chordModifiers` 的 `CmdOrCtrl` 分支只落 ctrl（不看 `goos`）：darwin 的 `CmdOrCtrl` 行变红。
