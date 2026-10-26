# P9-z 规格：Windows 启动器后缀阶梯只留一个答案

轮号：P9-z（P9-x 已由 `ffc032d` 收官并推送，任务书里的交接项过期，本轮按「选题」第 3 条
→ `.pending-bugs.md` P9-x「仍待」第 5 条执行）

## 意图（一句话）

`.exe/.cmd/.bat/.com` 这四个后缀在仓库里有 **5 份字面量**，分别被「接受（是不是程序）」
「生成（探哪个文件名）」「剥掉（命令叫什么）」三种方向读；把这张表收进一个包
`internal/progname`，让「Windows 启动器后缀有哪几个」只有一处可改。

为什么现在做：这是上一轮（P9-x「一个文件算不算程序」）的**同族另一半**——上一轮统一了
「判据」，这一轮统一「拼写」。P9-x 收尾时明确记了「与『是不是程序』不同族（是拼写），
故本轮不动；先核参考写法」，参考已核（见下），可以动手。

## 证据（改前 HEAD `ffc032d`，行号本轮实测）

| # | 位置 | 形态 | 读法 |
|---|---|---|---|
| 1 | `internal/progtype/progtype.go:34` `windowsProgramExtensions` | `[]string{".exe",".cmd",".bat",".com"}` | 接受（`filepath.Ext` 比表） |
| 2 | `internal/tools/catalog.go:370` `candidateNames` | `name+".exe", +".cmd", +".bat", +".com", name` | 生成（Windows 探针阶梯） |
| 3 | `internal/inventory/curated.go:60` `CuratedStem` | 同上四件，`CutSuffix` 剥 | 剥（裸名 → 词干，先 lower+TrimSpace） |
| 4 | `internal/extensions/custom.go:301` `executableCommand` | 同上四件，`CutSuffix` 剥 | 剥（路径 → 词干） |
| 5 | `internal/shell/shell.go:2708` `executableName` | 与 #4 **逐行相同** | 剥（路径 → 词干） |

`#4` 与 `#5` 除函数名与注释外逐字节相同（`diff` 实跑过），是复制粘贴的两份。

### 参考（`main:src-tauri/src/`）核对结论

- 参考**没有**这张表的单一答案，而且方向也不全：
  - `commands/extensions.rs:847` `executable_display_name` = `Path::file_name()`，**不剥后缀**；
  - `commands/apps/windows.rs:538` `is_launchable_target` = 三件 `.exe/.bat/.cmd`（**没有 `.com`**）；
  - `commands/apps/windows.rs:353` `executable_name` = `file_stem()`（剥**任意**扩展名）。
- ⇒ 我们的四件表是 mygo 重写自己加的（剥启动器后缀比 `file_stem` 更准：`foo.tar.gz` 不会被削成
  `foo.tar`）。所以本轮**不改语义**，只收口；参考的三处差异写进新包的包注释，避免下一个人
  「顺手统一成参考」把行为改掉。

### 故意保持更窄的三处（本轮只加 pin，不改行为）

| 位置 | 集合 | 为什么更窄 |
|---|---|---|
| `internal/apps/shortcut.go:188` `IsLaunchableTarget` | 3（无 `.com`） | 与参考 `windows.rs:539-549` 逐字对齐：开始菜单里 `*.com` 快捷方式历史上是 DOS 程序残留；**当前无任何测试**，本轮补 pin |
| `internal/extensions/paramrun.go:98` `ProgramIsCmdScript` | 2（`.cmd`/`.bat`） | 问的是「要不要经 `cmd.exe`」，`.exe`/`.com` 自己能起（`cmdshim_test.go` 已钉） |
| `internal/apps/registry.go:193` | 1（`.exe`） | 问的是「注册表的 `DisplayIcon` 是不是程序路径」，注册表只写 `.exe` |

## 落地项（位置 → 现状 → 应改成）

1. **新增 `internal/progname/progname.go`**（零依赖，只 import `path/filepath`、`runtime`、`strings`）：
   - 包注释：这张表被**三个方向**读（接受/生成/剥）；三处更窄的集合属于别的问题，各自钉住。
   - `var launcherSuffixes = []string{".exe", ".cmd", ".bat", ".com"}`（唯一一处字面量）。
   - `IsLauncher(name string) bool`：`strings.HasSuffix(strings.ToLower(name), suffix)` 任一成立。
   - `Stem(name string) string`：`TrimSpace` → `ToLower` → 剥一个后缀。
   - `FromPath(path string) string`：`Stem(filepath.Base(path))`。
   - `Ladder(platform, name string) []string`：`platform == ""` 回退 `runtime.GOOS`；`windows` →
     `name+suffix…, name`；其它 → `[]string{name}`（保持 `candidateNames` 的顺序与内容）。
2. `internal/progtype/progtype.go:34` 删 `windowsProgramExtensions`，`windowsProgram` 改调
   `progname.IsLauncher(path)`（`progtype` 已 import 别家包：本轮新增 `floter/internal/progname`）。
   语义等价：`filepath.Ext` 比表 ≡ 后缀比表（`.EXE` 大小写、`.exe` 单名、`a.exe.exe` 三种边界
   都要在新包 pin 里覆盖）。
3. `internal/tools/catalog.go:363-372` `candidateNames` 保留函数名（测试钉着它）但函数体改成
   `return progname.Ladder(platform, name)`；注释保留「读的是 key 不是 `runtime.GOOS`」那半句。
4. `internal/inventory/curated.go:58-66` `CuratedStem` 保留函数名（`TestCuratedStemIsTheOneSpellingRule`
   钉着它）但函数体改成 `return progname.Stem(name)`。
5. `internal/extensions/custom.go:297-307` **删** `executableCommand`；两处调用点
   （`:256`、`:417`）改 `progname.FromPath(...)`。
6. `internal/shell/shell.go:2705-2714` **删** `executableName`；两处调用点（`:2556`、`:2686`）改
   `progname.FromPath(...)`。`apps.executableName(argv []string)` **不是同族**（它剥的是 argv 的
   路径前缀，不剥后缀、不改大小写，对齐参考 `executable_name`）——不动。
7. **新增 pin**：`internal/progname/progname_test.go` 5 条（见下）+ `internal/apps/shortcut_test.go`
   1 条 `IsLaunchableTarget`（当前零覆盖，是本轮唯一碰到的「更窄集合」缺口）。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
- 视觉（墨/面/阴影/半径/布局）：**零改动**（本轮不是视觉轮，不出图，报告说明）。
- 生成物字符串不变：`Runtime.ExecutableNames` 仍是小写、无后缀；Windows 探针阶梯仍是
  `.exe/.cmd/.bat/.com/裸名` 这个顺序；`CuratedStem` 仍 lower+TrimSpace+剥一件。

## 门槛命令 + PASS 基线（本轮实测，改前）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
go test -count=1 ./...                    # 全绿（33 个有测试的包 ok）
--- PASS 2041   --- SKIP 15   --- FAIL 0  # 实测
测试函数 1661                              # grep -h "^func Test" 求和
go list ./... | wc -l                     # 35（33 有测试 + 2 no test files）
```

目标：PASS ≥ 2041（新增 pin 只许涨）、SKIP 仍 15、测试函数 ≥ 1661、包数 36（+1 新包）。

## mutation 计划（每条说明会让哪个 pin 变红）

- **M1** 把 `launcherSuffixes` 里的 `".com"` 删掉 →
  `TestStemDropsOneLauncherSuffix`（`rg.com` → `rg.com`）、
  `TestIsLauncherSeesTheFourAndNothingElse`、`TestLadderIsTheWindowsOne…` 三条红；
  `inventory.TestCuratedStemIsTheOneSpellingRule`（`{"rg.com","rg"}` 那行）也会红。
- **M2** `Stem` 去掉 `ToLower` → `TestStemDropsOneLauncherSuffix` 的 `RG.EXE` 红，
  `TestCuratedStemIsTheOneSpellingRule` 的 `RG.EXE` 红（跨包证明收口有效）。
- **M3** `Ladder` 的 Windows 分支去掉尾随裸名 → `TestLadderIsTheWindowsOne…` 与
  `tools.TestTheProbeLadderFollowsThePlatformKeyNotTheHost` 红。
- **M4** `FromPath` 改成 `Stem(path)`（不取 base）→ `TestFromPathIsTheStemOfTheBaseName` 红。
- **M5** `IsLaunchableTarget` 加上 `.com` → 新增的 `apps` pin 红（证明「更窄」被钉住）。

还原后 `md5sum -c` 逐字节核对 + `touch` 复跑。

## 报告

`/tmp/floter-p9y-report.md`，增量写。
