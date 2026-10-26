# P10-a 规格：「剥不剥扩展名」两套策略并存——按参考落成两条钉住的分歧，并让 apps 内部只留一个「任意扩展名」实现

## 意图

`apps.ShortcutTargetName`（`internal/apps/shortcut.go:199`）与 `apps.desktopFileStem`
（`internal/apps/apps.go:710`）剥的是**任意**扩展名，而 `internal/progname` 的 `Stem`
（`internal/progname/progname.go:63`）只剥四个**启动器**后缀。本轮先核参考，再决定
「合并」还是「两策略 + 钉分歧」，最后把 apps 内部那份「任意扩展名」的算术收成一个函数。

为什么现在做：这是 P9-z「仍待」第 5 条、也是任务书「现在该做什么」的交棒条目。
`internal/progname` 的包注释（`progname.go:30-36`）已经把「参考自己没有单一答案」写下来了，
但**没有任何 pin 拦住后人把两套规则合并**，也**没有 pin 拦住 apps 内部两个实现各自漂**
（一个走 `LastIndexByte` 手写，一个走 `filepath.Ext`）。两处都没有直接单元测试：
`grep desktopFileStem internal/apps/*_test.go` 零命中。

## 参考实测（`git show main:src-tauri/src/commands/apps/{windows,linux}.rs`）

| # | 位置 | 规则 | 对应我们的 |
|---|---|---|---|
| 1 | `windows.rs:321-324` `executable_name` | `Path::file_stem` → **剥任意**最后一段扩展名 | `apps.ShortcutTargetName` |
| 2 | `linux.rs:434-440`（`.desktop` 的 `file_stem` → `identifier_aliases`） | `Path::file_stem` → **剥任意** | `apps.desktopFileStem` |
| 3 | `linux.rs:458-466` `executable_name`（`Exec=`/`TryExec=`） | `Path::file_name` → **一个都不剥** | `apps.executableName` |
| 4 | `linux.rs:412-416`（无名条目的兜底名） | `Path::file_stem` → 剥任意 | `apps` 的 `name` 兜底（无独立函数） |

结论：**参考自己就有两套规则**（剥任意 / 一个都不剥），而「只剥启动器后缀」是
**我们独有**的第三条（`progname.go:32-36` 已写明理由：`foo.tar.gz` 不该被改名 `foo.tar`，
而连接程序的命令词确实该把 `rg.exe` 读成 `rg`）。合并 = 要么让连接程序的命令词变成
`report.tar`，要么让 Windows 别名变成 `rg.exe.txt`——两个都是产品回归。

**因此本轮的决定是：两策略保留，用 pin 把分歧钉死，不合并。** 属 B 类（测试零改动必须全绿）。

## 逐条落地项

### 1. apps 内部的「任意扩展名」只留一个实现

位置：`internal/apps/shortcut.go:196-217`、`internal/apps/apps.go:708-717`

现状：
- `desktopFileStem`：`filepath.Ext(base)`，`len(ext) < len(base)` 时切掉；
- `ShortcutTargetName`：`extensionOf(base)`（`LastIndexByte(name,'.') > 0` 手写）+ `TrimSuffix`，
  且 `ext == "" || ext == base` 时返回 `""`。

两处算术不同（一个用 `filepath.Ext`，一个手写 `LastIndexByte`），语义都是 Rust `Path::file_stem`，
但边界靠各自的小守卫巧合对齐：`".desktop"`（`filepath.Ext` 给整名，手写给 `""`）、`"app."`
（都切成 `"app"`）、`"no-ext"`（`filepath.Ext` 给 `""`，手写给 `""`）。

应改成：新增 `apps.fileStem(base string) string`——最后一段 `.`（且不在首位）之后切掉；
没有可剥的扩展名时**原样返回**（Rust `file_stem` 语义）。两个调用点都走它：
- `desktopFileStem(path) = fileStem(filepath.Base(path))`；
- `ShortcutTargetName`：`stem := fileStem(base); if stem == base { return "" }; return stem`
  （「没有扩展名 → 无名」这条**策略**留在 `ShortcutTargetName` 里，是它的私有约定，
  参考 `windows.rs:321` 那里其实会返回整名，但那条路径被 `IsLaunchableTarget`
  的 `.exe/.bat/.cmd` 判据挡在前面，行为不可达，因此**不改行为**）。

行为契约零改动：三条现有 pin 必须零改动全绿——
`TestTheTargetNameIsTheAlias`（`shortcut_test.go:167`）、`TestUninstallersAreNotRows`
（`shortcut_test.go:140`）、`TestScanWindowsDerivesWhatTheTargetSays`（`shortcut_test.go:181`）。

### 2. 两条分歧的 pin（新增测试，`internal/apps/filestem_test.go`）

- `fileStem` 表：`rg.exe → rg`、`report.tar.gz → report.tar`、`.desktop → .desktop`、
  `no-ext → no-ext`、`app. → app`、`` → ``、`foo.bar.desktop → foo.bar`。
- `desktopFileStem` 表（当前**零直接 pin**）：`/usr/share/applications/org.kde.konsole.desktop
  → org.kde.konsole`、`kde4/konsole.desktop → konsole`、`/x/.desktop → .desktop`、
  `x/no-ext → no-ext`；并钉组合结果 `identifierAliases(desktopFileStem(...))`：
  `org.kde.konsole.desktop → [konsole, kde]`。
- **分歧 pin**（本轮的核心）：同一批名字喂两个包，断言两者**不同**且各自钉死：
  | 名字 | `apps.fileStem` | `progname.Stem` |
  |---|---|---|
  | `rg.exe` | `rg` | `rg`（同） |
  | `rg.exe.txt` | `rg.exe` | `rg.exe.txt` |
  | `report.tar.gz` | `report.tar` | `report.tar.gz` |
  | `RG.CMD` | `RG` | `rg`（大小写也不同） |
  这样任何一个方向上的「顺手合并」都会立刻变红，并在失败信息里看到两套规则的差。

### 3. 交叉引用写进注释（不新增行为）

- `internal/apps/shortcut.go:196` 的 `ShortcutTargetName` 注释补一句：这是
  `windows.rs:321` 的 `file_stem`（剥任意），与 `progname.Stem`（只剥启动器后缀）是
  **两条不同的问题**，pin 在 `filestem_test.go`。
- `internal/apps/apps.go:708` 的 `desktopFileStem` 注释补同样的指向（`linux.rs:434`）。
- `internal/progname/progname.go:30` 那段「参考自己没有单一答案」补上 apps 侧的 pin 名字，
  让后人从任一端都能走到另一端的证据。

## 门槛命令 + PASS 基线（本轮实测，改前 HEAD `92239be`）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

改前基线（本轮开头实测，与任务书记录逐项相等）：**PASS 2048** / **SKIP 15** /
**36 包** / **1668 测试函数**。本轮新增 3 个测试函数 → 期望 **PASS 2051**、函数 **1671**，
SKIP 不变。

## mutation 计划（≥2 条，实跑变红后还原）

- **M1**：`fileStem` 改成「只剥最后一段扩展名但把 `report.tar.gz` 也切成 `report`」
  （即误合并成「按第一个点切」）→ `fileStem` 表红 + 分歧 pin 红。
- **M2**：`ShortcutTargetName` 改成直接 `return progname.Stem(base)`（真合并）→
  `TestTheTargetNameIsTheAlias` 里 `WXWork.exe → WXWork` 变 `wxwork`（大小写）必红，
  分歧 pin 也红。
- **M3**：`desktopFileStem` 改回 `filepath.Ext` 版但去掉 `len(ext) < len(base)` 守卫
  → `/x/.desktop` 变 `""`，`desktopFileStem` 表红。
