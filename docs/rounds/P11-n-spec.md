# P11-n 规格：PATH 列表只有一个切法，PATH 上找程序只有一个查法

## 意图（一句话）

「一个 PATH 形状的字符串怎么切成目录」与「PATH 上一个程序在哪」这两个问题，全仓各只有
一个实现（`internal/pathlist`），六个调用点全部转发；顺带修掉 `inventory.LookPath`
对带分隔符的名字**拼进每个 PATH 目录**从而返回错误路径的真 bug。

## 为什么现在做（证据）

优先级：用户实测四条（①透明 ②外圈/三大金刚 ③卡死 ④打包）全部卡在实机复测或拍板，
合约 `docs/raycast-design.md` §6 全「已落地」，上一轮「仍待」第 1 条同样卡实机 →
落到优先级 4 的**第一档「三平台一致性」**。PATH 是 Windows 与 Unix 差异最硬的一处，
且参考（`main` 分支 Rust）在这一族里**只有一个答案**：

| 参考位置 | 用的切法 |
|---|---|
| `main:src-tauri/src/commands/apps/linux.rs:203`（`XDG_DATA_DIRS`） | `std::env::split_paths` |
| `main:src-tauri/src/commands/apps/linux.rs:479-481`（`TryExec=` 探测） | `std::env::split_paths` |
| `main:src-tauri/src/extensions/catalog.rs:1003-1006`（PATH 上的命令扫描） | `std::env::split_paths` |
| `main:src-tauri/src/extensions/inventory.rs:505`、`:1622-1624` | `std::env::split_paths` |

本仓却有**三套**（实测行号，2026-10-28 HEAD `28a3af2`）：

| # | 位置 | 现在的切法 | 问题 |
|---|---|---|---|
| 1 | `internal/apps/apps.go:497`、`internal/apps/apps.go:176`、`internal/xdg/xdg.go:41` | `filepath.SplitList` | 正确（Windows 上会**剥引号**、空串→空切片） |
| 2 | `internal/apps/commands.go:17` | `strings.Split(path, string(os.PathListSeparator))` | 不剥引号 |
| 3 | `internal/inventory/sources.go:527`、`internal/extensions/runtimepath.go:233-241` | 同 #2（后者包一层丢空项） | 不剥引号 |

**Windows 上的真后果**：`PATH` 里带引号的条目（`"C:\Program Files\Git\cmd"`，安装器常写）
在 #2/#3 里引号会留在目录名上 → `filepath.Join("\"C:\\…\\cmd\"", "git.exe")` 这个路径
不存在 → 命令扫描（`CommandDirs` → `ScanCommands`，`internal/shell/shell.go:1515`）
与扩展运行时（`LookPath` / `SearchDirectories`）**找不到 git/node**，而 macOS/Linux
上同一个用户环境一切正常。这就是「三平台不一致」的本体。Go 标准库
`path/filepath.splitList` 在 Windows 上**显式**去引号（`strings.ReplaceAll(s, "\"", "")`），
这正是它存在的理由。

**第二个真 bug（Linux/macOS 上可复现，不依赖 GOOS）**：`internal/inventory/sources.go:526`
的 `LookPath(name)` 不判名字里有没有分隔符，直接对每个 PATH 目录做
`filepath.Join(directory, name)`：

```
PATH=/tmp/x  且  /tmp/x/bin/foo 存在可执行
inventory.LookPath("bin/foo")  →  ("/tmp/x/bin/foo", true)   ← 错：这不是 PATH 命中，是拼接巧合
```

参考的 `is_executable` 在 `linux.rs:476-478` 先 `if program.contains(std::path::is_separator)`
短路；`apps.executableAvailable`（`internal/apps/apps.go:494-496`）也短路。同一问题两个答案，
其中一个是错的。

**第三个不一致（已核，不是缺陷）**：`internal/apps/commands_test.go:47` 的 fixture 硬编码成
`"/one:"+"/two::/three"`。起初怀疑它在 Windows 上必红，实测该文件首行是
`//go:build !windows`，**只在 Unix 上编译**，所以硬编码 `:` 是正确的，本轮不动它
（台账因此是零测试改动）。

## 落地项

### 1. 新增 `internal/pathlist/pathlist.go`

一个包，两个函数，两句话：

- `Split(value string) []string` — **唯一的切法**：`filepath.SplitList` 之后丢掉空条目。
  文档写清三条：空串 → 空切片；`a::b` → `a,b`（空条目在 shell 里表示「当前目录」，
  但 floter 自己的工作目录不是用户程序待的地方，需要这条语义的只有 `Lookup` 的兜底）；
  Windows 上引号由 `filepath.SplitList` 剥掉，这就是选它而不选 `strings.Split` 的理由。
- `Lookup(name string) (string, bool)` — **唯一的查法**：
  1. 名字带分隔符（`os.PathSeparator` 或 `/`）→ 只问 `progtype.Host(name)`，不拼 PATH
     （与参考 `linux.rs:476-478`、与 `apps.executableAvailable` 现状一致）；
  2. 否则按 `Split(os.Getenv("PATH"))` 顺序找第一个 `progtype.Host(filepath.Join(dir,name))`；
  3. 都没有 → `exec.LookPath` 作为最后一句话（它管 `PATHEXT` 与「空条目＝当前目录」）。
  测试接缝 `lookupIn(path, name)`（同 `runtimepath.lookToolIn` 的先例），生产入口只有 `Lookup`。

### 2. 六个调用点全部转发

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/apps/commands.go:17`（`CommandDirs`） | `strings.Split(…, os.PathListSeparator)` + 手写丢空项 | `pathlist.Split(os.Getenv("PATH"))`，循环与丢空项删掉 |
| `internal/apps/apps.go:493-506`（`executableAvailable`） | 自己走 `filepath.SplitList` + `""→"."` | `_, ok := pathlist.Lookup(program); return ok`（分隔符短路与空条目规则随 `Lookup` 走） |
| `internal/apps/apps.go:176`（`XDG_DATA_DIRS`） | `filepath.SplitList(os.Getenv("XDG_DATA_DIRS"))` | `xdg.DataDirs()`（`xdg` 已 import；`XDG_DATA_DIRS` 只剩一个读入口） |
| `internal/xdg/xdg.go:41`（`DataDirs`） | `filepath.SplitList` + TrimSpace | `pathlist.Split` + 保留它自己的 TrimSpace 过滤（XDG 的规则归 XDG） |
| `internal/inventory/sources.go:523-540`（`LookPath`） | 自己切 + 自己拼 | `return pathlist.Lookup(name)`（保留函数名与签名；它现在**零调用点**，转发即契约） |
| `internal/extensions/runtimepath.go:233-241`（`splitPath`） | 自己切 + 丢空项 | 删除，四处调用（`:39`、`:58`、`:227`）改 `pathlist.Split` |

### 3. 新增 pin

- `internal/pathlist/pathlist_test.go`
  - `TestSplitDropsTheEmptyEntries`：`""` → 空；`"a::b"` → `a,b`；
    `"a:"+sep+"b"` 用 `string(os.PathListSeparator)` 拼（**三平台同一断言**）。
  - `TestSplitUnquotesOnWindowsOnly`：`"\"C:\\Program Files\\Git\\cmd\""` 一个条目——
    Windows 断言已剥引号，其他平台断言原样（同一测试里按 `runtime.GOOS` 分支，不 Skip）。
  - `TestLookupShortCircuitsOnASeparator`：**真 bug 的反证**——PATH 目录下有 `bin/foo`
    可执行时 `lookupIn(dir, "bin/foo")` 必须是 `false`（旧 `inventory.LookPath` 会是 true）。
  - `TestLookupWalksPathInOrder`：两个目录都有同名可执行 → 取第一个。
  - `TestLookupIsTheSameAnswerAsAppsAndInventory`：同一 fixture 下
    `pathlist.Lookup` / `apps.executableAvailable` / `inventory.LookPath` 三个入口同答案
    （放在各自包的测试里，避免循环 import；见下）。
- `internal/ci/pathlist_contract_test.go` `TestPathListsHaveOneSplitter`：扫 `internal/` +
  `cmd/` 的 `.go`（注释除外），除 `internal/pathlist` 外任何文件出现
  `filepath.SplitList`、`os.PathListSeparator`、`filepath.ListSeparator` 判红。
  白名单：`internal/extensions/runtimepath.go` 的 `strings.Join(merged, string(os.PathListSeparator))`
  是**拼**不是**切**——按行匹配 `strings.Join` 放行，并在测试注释里写明为什么。
  （`os.PathSeparator` 单数不在禁列，它是「名字里有分隔符吗」的判据。）

## 行为契约零改动红线

- **不改**：`CommandDirs` 的返回内容（丢空项、顺序）、`ScanCommands` 的结果、
  `executableAvailable` 对 `TryExec=` 的判定（Unix 上 `progtype.Host` 与 `exec.LookPath`
  同规则，Windows 上 `exec.LookPath` 只是兜底）、`xdg.DataDirs` 的 TrimSpace 过滤、
  `SearchPath()` 的合并顺序与去重、`SearchDirectories()` 的内容。
- **不动**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、任何墨与面。
- `internal/extensions/cmdshim.go` 的 `/D /S /C` 与 `internal/inputtext` 均不在本轮范围。

## 门槛命令 + PASS 基线（本轮实测起点）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 2265（P11-m 实测）
GOOS=linux/windows/darwin go build ./...   # 三平台过
```

基线：PASS **2265**、测试函数 **1840**（P11-m 报告）、SKIP 15、FAIL 0、36 行（34 ok + 2 no test files）。
本轮目标：PASS ≥ 2270（新增 5 条），函数数只增不减，SKIP 仍 15。

## mutation 计划（每条都会让哪个 pin 变红）

1. `pathlist.Split` 改回 `strings.Split`（不丢空项）→
   `TestSplitDropsTheEmptyEntries` 红 + `internal/apps.TestCommandDirs` 红（`a::b` 多出 `""`）。
2. `Lookup` 删掉「名字带分隔符就短路」那三行 → `TestLookupShortCircuitsOnASeparator` 红
   （`bin/foo` 被拼成 `<dir>/bin/foo` 而返回 true）。
3. `Lookup` 把 `Split` 换成「按 PATH 倒序」→ `TestLookupWalksPathInOrder` 红。
4. 在 `internal/inventory/sources.go` 里塞回一行 `filepath.SplitList(...)` →
   `TestPathListsHaveOneSplitter` 红。
5. `Split` 里加一行 `strings.Trim(s, "\"")`（无条件剥引号）→
   `TestSplitUnquotesOnWindowsOnly` 在 Linux 上红（多剥了不该剥的）。
