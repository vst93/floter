# P10-n 报告：工具清点的来源表就是判据

规格 `docs/rounds/P10-n-spec.md`。本轮非视觉轮，不出图。

## 1. 这一轮做了什么

任务书建议的方向是「`internal/inventory` 的多来源工具清点（dpkg/rpm/pacman/flatpak/snap/nix/brew/
注册表/scoop/choco/winget/WSL）在 `docs/mygo-rewrite-plan.md:624` 仍标未做」。**先量后做**的结论和
建议方向不一样：

### 1.1 「未做」是误记——参考自己也不产出那五个来源

参考的枚举有 15 个值（`floter-main/src-tauri/src/extensions/inventory.rs:24-40`），产出点只有 10 个：

```
$ for v in Dpkg Rpm Pacman Registry Wsl Brew Scoop Chocolatey WinGet LaunchServices; do
    echo -n "$v: "; grep -rn "DiscoverySource::$v" --include=*.rs . | wc -l; done
Dpkg: 0   Rpm: 0   Pacman: 0   Registry: 1   Wsl: 0
Brew: 1   Scoop: 1   Chocolatey: 2   WinGet: 1   LaunchServices: 4
```

`Dpkg`/`Rpm`/`Pacman`/`Wsl` 除枚举自身零引用；`Registry` 那一次在 `inventory.rs:1184`，是
`insert()` 单元测试里随手挑的一个枚举值（验「空白描述不覆盖已知描述」），不是产出点。前端
`ExtensionsPanel.tsx:318-331` 的联合类型只是**词汇表**。所以本仓不接 dpkg/rpm/pacman/注册表/WSL
**不是缺口，是与参考一致**——这一条从「未做」改成「故意不接，证据在此」。

### 1.2 逐条比对产出点，找到唯一一处真差

| 平台 | 参考 | 本仓（改前） | 结论 |
|---|---|---|---|
| linux | `inventory.rs:441-478` | `sources.go:20-36` | 一致（`linuxDesktopRoots`/`linuxExportRoots` 早已收口） |
| darwin | `inventory.rs:848-874` | `sources.go:379-389` | 一致 |
| windows | `inventory.rs:911-953` | `sources.go:422-431` | **缺 `%ProgramData%\chocolatey\bin`** |

参考的 `discover_windows` 把 Chocolatey 根命名两次：字面量 `C:\ProgramData\chocolatey\bin` **和**
`$ProgramData\chocolatey\bin`。`ProgramData` 被重定向到 `D:\CorpData` 的机器上，字面量那条根本不存在，
Chocolatey 的全部 shim 一条都找不到。默认安装下两条是同一个目录，走两次由 `mergeInto` 按路径去重、
结果不变——所以补这一条**只增不减**。

### 1.3 顺带核到的一处故意分歧，只钉不改

`SourceWinGet = "winget"`，参考 serde `rename_all = "kebab-case"` 给的是 `win-get`。这个字符串在本仓
**不出进程**（`Candidate` 无任何 json tag），唯一去处是 `system.go:735-736` 拼给人看的候选行。改它
等于改用户看到的字，无收益 → 逐字钉住 + 注释写下参考的拼写。

## 2. 落地

- `internal/inventory/sources.go`
  - 新 `toolRoot{dir, source}` + `walkToolRoots`。
  - `discoverLinux` → `linuxPackageRoots()`（flatpak + nix + snap，一张表）；
    `nixRoots()` 抽成 var。
  - `discoverMacOS` → `brewRoots()` / `macAppRoots()` 两张 var 表。
  - `discoverWindows` → `windowsToolRoots()`，**新增 `%ProgramData%\chocolatey\bin`**，经 `dedupeRoots` 去重。
  - 新 `dedupeRoots` + `sameRoot`（Windows 两种分隔符都合法、路径比较不分大小写 → 先去反斜杠、
    再 `ToSlash`、再 `ToLower`；这样表里 `C:\ProgramData\...` 与 `%ProgramData%` 展开成同一目录时
    只走一次，且在 Linux 上跑测试也判得对）。
  - `linuxDesktopRoots` 由 `func` 声明改成 `var`（与其余四张表同形，测试可注入）。
- `internal/inventory/inventory.go`：新 `discoverySources`——闭集词汇表，注释写下五个参考独有值
  为什么不在（附 `grep` 计数证据）。
- `internal/inventory/sourcetable_test.go`（新文件，5 个函数）：
  `TestEveryDeclaredSourceIsReachable`、`TestTheSourceVocabularyIsClosed`、
  `TestThePlatformRootTablesAreTheReferenceTables`、`TestTheWindowsChocolateyRootFollowsProgramData`
  （4 子格）、`TestTwoSpellingsOfOneRootAreOneRoot`。

## 3. 台账（V / B / S）

- **V = 0**：没有视觉数值。本轮非视觉，不出图。
- **B = 0**：没有任何既有测试被改动。既有 2113 条 PASS 一条没红、一条没改。
- **S = 1**：`discoverLinux`/`discoverMacOS`/`discoverWindows` 的结构从「循环体里的路径字面量」
  变成「遍历一张表」；`linuxDesktopRoots` 由 `func` 改 `var`。断言是新增的，没有旧断言要改。

## 4. 门槛（HEAD `8014517` → 本轮，全部实测）

| 项 | 改前基线 | 本轮实测 |
|---|---|---|
| `gofmt -l cmd internal` | 空 | **空** |
| `go vet ./...` | 净 | **净** |
| `go test -count=1 ./...` | 全绿 | **全绿** |
| `--- PASS` | 2113 | **2122**（+9 = 5 新函数 + 4 子测试） |
| `--- SKIP` | 15 | **15**（=） |
| `--- FAIL` | 0 | **0** |
| 包 | 36 | **36**（=） |
| 测试函数 | 1719 | **1724**（+5） |
| `GOOS=linux/windows/darwin go build ./...` | 过 | **过** |

## 5. 变异（7 条全部实跑变红，`md5sum -c` 逐字节还原，`touch` 后复跑）

```
M1 删掉 windowsToolRoots 的 %ProgramData% 支
--- FAIL: TestTheWindowsChocolateyRootFollowsProgramData/program_data_is_redirected
    sourcetable_test.go:187: Chocolatey roots = [C:\ProgramData\chocolatey\bin],
    want [C:\ProgramData\chocolatey\bin D:\CorpData/chocolatey/bin]

M2 dedupeRoots 改成 return roots
--- FAIL: .../program_data_is_the_literal_path
    Chocolatey roots = [C:\ProgramData\chocolatey\bin C:\ProgramData/chocolatey/bin],
    want the default install walked once
--- FAIL: TestTwoSpellingsOfOneRootAreOneRoot
    dedupeRoots = [{C:\ProgramData\chocolatey\bin chocolatey}
                   {c:\programdata\chocolatey\bin chocolatey}
                   {D:\CorpData\chocolatey\bin chocolatey}], want the first spelling kept ...

M3 discoverySources 删掉 SourceBrew
--- FAIL: TestEveryDeclaredSourceIsReachable
    sources reached by the fixture walk =
         [brew chocolatey desktop flatpak launch-services nix path scoop snap winget]
    want exactly the declared vocabulary
         [chocolatey desktop flatpak launch-services nix path scoop snap winget]

M4 winget 根改挂 SourceChocolatey
--- FAIL: .../local_app_data_adds_the_winget_links
    WinGet roots = [], want [C:\Users\tester\AppData\Local/Microsoft/WinGet/Links]

M5 sameRoot 不再折大小写
--- FAIL: TestTwoSpellingsOfOneRootAreOneRoot
    sameRoot("C:\ProgramData\chocolatey\bin", "c:\programdata\CHOCOLATEY\bin") = false, want true

M6 删掉 Scoop 根（**第一次是绿的**，见 §6）
--- FAIL: TestThePlatformRootTablesAreTheReferenceTables
    windowsToolRoots = [{C:\ProgramData\chocolatey\bin chocolatey}]
    want [{C:\ProgramData\chocolatey\bin chocolatey} {/home/tester/scoop/shims scoop}]

M7 nixRoots 删掉系统 profile
--- FAIL: TestThePlatformRootTablesAreTheReferenceTables
    linuxPackageRoots = [... {/home/tester/.nix-profile/bin nix} {/snap/bin snap}]
    want [... {/home/tester/.nix-profile/bin nix} {/nix/var/nix/profiles/default/bin nix} {/snap/bin snap}]
```

`md5sum -c` 三条源文件全部「成功」；`find -newer` + `touch` 后再跑一次，PASS 仍 2122。

## 6. 这一轮自己踩的坑（写下来给下一轮）

**M6 第一版是绿的。** 原计划以为「winget 根改挂 `SourceChocolatey`」会让
`TestEveryDeclaredSourceIsReachable` 变红——实跑是 `ok`。原因是那条测试把四张平台表 **stub 成夹具**，
它只能证明「声明的每个来源都够得着」，**证明不了「生产表接对了根」**。删掉 Scoop 根、删掉 nix 系统
profile，全套测试照样绿。于是补了 `TestThePlatformRootTablesAreTheReferenceTables`——固定环境
（`HOME=/home/tester`、`ProgramData`/`LOCALAPPDATA` 空）下四张表**按值**钉住，与 `xdgroots_test.go`
钉桌面根表是同一手法。补上后 M6/M7 立刻红。

教训：**变异计划里「哪条测试会变红」是假设，不是事实**；假设不成立时不能删掉那条变异，要补断言。

## 7. 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 43 轮，本仓无可改代码。
2. 需拍板项不变：**#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、`ShortcutTargetName` 对「无扩展名」
   返回 `""`（`shortcut.go:208-210`）、register 只留 `cmd`、`open` 是否拒一切 query、
   未知宿主是否改成静默忽略。
3. **本轮新增一条已定性、不需要拍板的结论**（写进 `docs/mygo-rewrite-plan.md:624` 那一行）：
   dpkg/rpm/pacman/注册表/WSL 五个来源参考自己从不产出，本仓不接是与参考一致，不是缺口。
4. 下一轮方向：`internal/inventory` 这一族到此收官（来源表、桌面条目、路径解析、跨根优先级都已有
   pin）。建议转 `internal/tools`（12 个工具 × 8 个包管理器的配方表）或 `internal/extensions`
   的候选排名（`candidates.go`）——两者都还没做过「表即判据」式的逐条核对。
