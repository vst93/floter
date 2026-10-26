# P10-a 报告：两套「剥扩展名」策略按参考落成分歧 + apps 内部只留一个「任意扩展名」实现

方向来自任务书「现在该做什么」的交棒条目（同 P9-z「仍待」第 5 条）。
规格 `docs/rounds/P10-a-spec.md`，本报告 `docs/rounds/P10-a-report.md`，两份都进仓库。

## 1. 参考实测：它自己就有两套规则，所以本轮不合并

`git show main:src-tauri/src/commands/apps/{windows,linux}.rs`：

| # | 参考位置 | 规则 | 我们的对应 |
|---|---|---|---|
| 1 | `windows.rs:321-324` `executable_name` | `Path::file_stem` → **剥任意**最后一段扩展名 | `apps.ShortcutTargetName` |
| 2 | `linux.rs:434-440`（`.desktop` 的 `file_stem` → `identifier_aliases`） | `Path::file_stem` → **剥任意** | `apps.desktopFileStem` |
| 3 | `linux.rs:458-466` `executable_name`（`Exec=`/`TryExec=`） | `Path::file_name` → **一个都不剥** | `apps.executableName` |
| 4 | `linux.rs:412-416`（无名条目兜底名） | `Path::file_stem` → 剥任意 | `apps` 的 `name` 兜底 |

我们独有的第三条是 `internal/progname`：只剥 `.exe/.cmd/.bat/.com` 并小写
（`internal/progname/progname.go:63`），因为它的答案是**连接程序的命令词**，
`report.tar.gz` 不该被改名 `report.tar`（包注释 `progname.go:30-38` 已写明理由）。

合并两个方向都是产品回归：往 `progname` 并 → Windows 别名变 `rg.exe.txt`、`WXWork` 变 `wxwork`；
往 `apps` 并 → 命令词变 `report.tar`。**结论：两策略保留，钉住分歧。**

## 2. 落地（产品行为零改动）

### 2.1 apps 内部「任意扩展名」只留一个实现

新增 `apps.fileStem`（`internal/apps/shortcut.go:226`），Rust `Path::file_stem` 语义：
最后一段 `.`（不在首位）之后切掉，没有可剥的就原样返回。两个调用点都走它：

- `desktopFileStem`（`internal/apps/apps.go:714`）从 `filepath.Ext` + `len(ext) < len(base)` 守卫
  改成 `fileStem(filepath.Base(path))`；
- `ShortcutTargetName`（`internal/apps/shortcut.go:211`）从手写 `extensionOf` + `TrimSuffix`
  改成 `fileStem`，「没有扩展名 → 无名」这条**策略**留在自己身上（`stem == base → ""`）。

两处算术原来是两份（`filepath.Ext` vs `LastIndexByte`），边界靠各自的小守卫巧合对齐；
现在是一份。三条现有 pin 零改动全绿：`TestTheTargetNameIsTheAlias`（`shortcut_test.go:167`）、
`TestUninstallersAreNotRows`（`shortcut_test.go:140`）、
`TestScanWindowsDerivesWhatTheTargetSays`（`shortcut_test.go:181`）。

### 2.2 三条 pin（`internal/apps/filestem_test.go`，+3 测试函数）

- `TestFileStemDropsTheFinalExtension`：9 条表（`report.tar.gz → report.tar`、
  `.desktop → .desktop`、`app. → app`、`org.kde.konsole.desktop → org.kde.konsole`）。
- `TestDesktopFileStemIsTheBaseNameWithoutItsExtension`：这条规则**之前零直接 pin**
  （`grep desktopFileStem internal/apps/*_test.go` 零命中），现在钉 5 条 + 组合结果
  `identifierAliases(desktopFileStem("org.kde.konsole.desktop")) == [konsole kde]`。
- `TestTheTwoStemRulesDisagreeOnPurpose`：分歧 pin，同一批名字同时喂两个包并断言
  `same` 与否——`rg.exe` 同（`rg`/`rg`）、`rg.exe.txt` 不同（`rg.exe`/`rg.exe.txt`）、
  `report.tar.gz` 不同（`report.tar`/`report.tar.gz`）、`RG.CMD` 不同（`RG`/`rg`，大小写）。
  任何方向的「顺手合并」都会红，失败信息里直接印出两套规则的值。

### 2.3 交叉引用

`internal/apps/shortcut.go:196`、`internal/apps/apps.go:708`、`internal/progname/progname.go:30`
三处注释互相指到 pin 名字，从任一端都能走到另一端。

台账：**V=0 / B 既有 pin 零改动全绿 / S=0**。

## 3. 门槛实测（改前 HEAD `92239be`）

| 项 | 改前 | 改后 |
|---|---|---|
| `gofmt -l cmd internal` | 空 | 空 |
| `go vet ./...` | 净 | 净 |
| `go test -count=1 ./...` | 全绿 | 全绿（`--- FAIL` 0） |
| `--- PASS` | 2048 | **2051**（+3） |
| `--- SKIP` | 15 | 15（=） |
| 包数 | 36 | 36（=，34 个有测试 + 2 个 `no test files`） |
| 测试函数 | 1668 | **1671**（+3） |
| 三平台 build | 过 | 过（linux/windows/darwin） |

## 4. mutation（3 条实跑变红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑）

改前三文件指纹存 `/tmp/p10a.md5`，每条注入后 `touch` 再跑（避开测试缓存）。

**M1** `fileStem` 的 `LastIndexByte` → `IndexByte`（等价于「按第一个点切」）：
```
--- FAIL: TestFileStemDropsTheFinalExtension
    filestem_test.go:27: fileStem("report.tar.gz") = "report", want "report.tar"
    filestem_test.go:27: fileStem(".hidden.txt") = ".hidden.txt", want ".hidden"
    filestem_test.go:27: fileStem("org.kde.konsole.desktop") = "org", want "org.kde.konsole"
--- FAIL: TestTheTwoStemRulesDisagreeOnPurpose
    filestem_test.go:80: apps.fileStem("rg.exe.txt") = "rg", want "rg.exe"
--- FAIL: TestDesktopSearchAliases
    apps_test.go:344: aliases [gio org] lack "gnome"
```
连三条既有 B 类 pin 一起红——说明这条规则真的在改用户能搜到的别名，不只是字符串游戏。

**M2** `ShortcutTargetName` 直接 `return progname.Stem(base)`（真合并）：
```
--- FAIL: TestTheTargetNameIsTheAlias
    shortcut_test.go:176: ShortcutTargetName("C:\\Program Files\\WXWork\\WXWork.exe") = "wxwork", want "WXWork"
--- FAIL: TestTheTwoStemRulesDisagreeOnPurpose
    filestem_test.go:93: ShortcutTargetName = "wxwork", want WXWork (apps keeps the case)
```
既有 pin 先红（产品回归信号），分歧 pin 随后红（策略信号）。

**M3** `desktopFileStem` 回到 `filepath.Ext` 且去掉 `len(ext) < len(base)` 守卫：
```
--- FAIL: TestDesktopFileStemIsTheBaseNameWithoutItsExtension
    filestem_test.go:45: desktopFileStem("/usr/share/applications/.desktop") = "", want ".desktop"
```
三条全部 `md5sum -c` 三文件 `成功`，还原后 `gofmt -l` 空、`go test ./internal/apps/ ./internal/progname/` 绿。

## 5. 本轮没做（写进 `.pending-bugs.md`「仍待」）

- `apps.ShortcutTargetName` 对「无扩展名」返回 `""`，而参考 `windows.rs:321` 的 `file_stem`
  会返回整名。**行为不可达**（`IsLaunchableTarget` 只放 `.exe/.bat/.cmd`，注册表只认 `.exe`），
  改它只会让既有 pin 需要改断言，故本轮不动，只把理由写进注释。
- 视觉轮：本轮属 B 类，无 `ui.Tester` 出图。
