# P10-n 规格：工具清点的来源表就是判据——参考能产出的十个来源逐条核对

## 意图（一句话）

`internal/inventory` 的来源表（`path`/`desktop`/`flatpak`/`snap`/`nix`/`brew`/`launch-services`/
`chocolatey`/`scoop`/`winget`）**每一个都必须是真能产出的**，且参考那边多出来的五个枚举值
（`dpkg`/`rpm`/`pacman`/`registry`/`wsl`）**必须钉成「参考自己从不产出」**——`docs/mygo-rewrite-plan.md:624`
把它们记成「未做」，这一轮把这条误记清掉，并补上核对时发现的唯一真缺口（Windows Chocolatey 的
`%ProgramData%` 根）。

## 为什么现在做（证据）

任务书「现在该做什么」建议方向就是这一条，但**先量后做**的结论和建议方向不一样：

1. **「未做」是误记。** 参考 `DiscoverySource` 枚举有 15 个值
   （`floter-main/src-tauri/src/extensions/inventory.rs:24-40`），但**产出点**只有 10 个：

   ```
   $ for v in Dpkg Rpm Pacman Registry Wsl Brew Scoop Chocolatey WinGet LaunchServices; do
       echo -n "$v: "; grep -rn "DiscoverySource::$v" --include=*.rs . | wc -l; done
   Dpkg: 0   Rpm: 0   Pacman: 0   Registry: 1   Wsl: 0
   Brew: 1   Scoop: 1   Chocolatey: 2   WinGet: 1   LaunchServices: 4
   ```

   `Dpkg`/`Rpm`/`Pacman`/`Wsl` **零次**引用（除枚举自身）；`Registry` 的那一次在
   `inventory.rs:1184`，是 `insert()` 的单元测试里随手挑的一个枚举值（用来验「空白描述不覆盖已知描述」），
   不是产出点。也就是说参考**没有任何** dpkg/rpm/pacman/注册表/WSL 的清点代码——前端
   `ExtensionsPanel.tsx:318-331` 的联合类型只是词汇表。**所以本仓不接这五个来源不是缺口，是与参考一致。**
2. **逐条比对产出点，只有一处真差**：

   | 平台 | 参考 | 本仓 | 结论 |
   |---|---|---|---|
   | linux | `inventory.rs:441-478`：desktop roots、`~/.local/share/flatpak/exports/bin`、`/var/lib/flatpak/exports/bin`、`/snap/bin`、`~/.nix-profile/bin`、`/nix/var/nix/profiles/default/bin` | `sources.go:20-36` 同一组（`linuxDesktopRoots` / `linuxExportRoots` 已收口） | 一致 |
   | darwin | `inventory.rs:848-874`：`/usr/local/bin`、`/opt/homebrew/bin`（brew）+ `/Applications`、`/System/Applications`、`~/Applications`（launch-services） | `sources.go:379-389` 同一组 | 一致 |
   | windows | `inventory.rs:911-953`：`C:\ProgramData\chocolatey\bin`、`~/scoop/shims`、`$LOCALAPPDATA\Microsoft\WinGet\Links`，**外加 `$ProgramData\chocolatey\bin`** | `sources.go:422-431` 只有前三条 | **缺 `$ProgramData\chocolatey\bin`** |

   `%ProgramData%` 被重定向到 `D:\CorpData` 的机器上，`C:\ProgramData\chocolatey\bin` 不存在，
   Chocolatey 的全部 shim 就一条都找不到；默认安装下两条是同一个目录，走两次由 `mergeInto` 按路径
   去重、结果不变——所以补这一条**只增不减**，不改任何既有行的来源或排名。
3. 顺带核到的一处**故意**分歧，本轮只钉不改：`SourceWinGet = "winget"`（`inventory.go:38`）而参考
   serde `rename_all = "kebab-case"` 给的是 `win-get`（`inventory.rs:36-38`）。这个字符串在本仓
   **不出进程**（`Candidate` 没有任何 json tag，见 `grep -n "json:" internal/inventory/*.go` 为空），
   唯一去处是 `system.go:735-736` 拼给人看的候选行。改它等于改用户看到的字，无收益，钉住即可。

## 落地项

### 1. `internal/inventory/sources.go`：把三张平台根目录表变成可注入的表

- **位置**：`sources.go:20-36`（`discoverLinux`）、`:379-389`（`discoverMacOS`）、`:422-431`（`discoverWindows`）。
- **现状**：三个 `discover*` 函数把路径字面量写在循环体里，测试只能靠 `t.Skip`/真机目录才够得着；
  `discoverWindows` 的 Chocolatey 根只有字面量 `C:\ProgramData\chocolatey\bin`。
- **应改成**：与已有的 `linuxExportRoots`（`sources.go:39-49`）同一手法——包级 `var` 表 + 一个
  `toolRoot{dir, source}` 小结构，`discover*` 只负责遍历。具体：
  - `type toolRoot struct { dir string; source DiscoverySource }`
  - `var nixRoots = func() []string`（`~/.nix-profile/bin` + `/nix/var/nix/profiles/default/bin`）
  - `var linuxPackageRoots = func() []toolRoot`（flatpak roots + nix roots + snap root，顺序照旧）
  - `var brewRoots = func() []string`、`var macAppRoots = func() []string`
  - `var windowsToolRoots = func() []toolRoot`（**新增 `%ProgramData%\chocolatey\bin`**，经
    `dedupeRoots` 去重）
  - `func dedupeRoots([]toolRoot) []toolRoot` + `func sameRoot(left, right string) bool`
    （同一目录的两种拼写：Windows 两种分隔符都合法、且比较不分大小写——所以先去反斜杠再 `ToSlash`
    再 `ToLower`；这样表里写 `C:\ProgramData\...` 与 `%ProgramData%` 展开成同一目录时只走一次，
    且在 Linux 上跑测试也判得对）。
- **不改**：`linuxExportRoots` 的签名（`desktopEntryTarget` 在读）、`linuxDesktopRoots` 的实现、
  任何 merge 语义、排名、quality。

### 2. `internal/inventory/inventory.go`：来源词汇表收成一个可断言的清单

- **位置**：`inventory.go:24-39`（`DiscoverySource` 与十个常量）。
- **现状**：常量散着，没有任何东西能证明「声明的每个来源都够得着」，也没有地方写下那五个
  参考独有值为什么不在。
- **应改成**：紧跟常量加 `var discoverySources = []DiscoverySource{…}`（顺序＝参考枚举顺序里
  本仓产出的那十个），注释写清：这是**闭集**；参考的 `Dpkg`/`Rpm`/`Pacman`/`Registry`/`Wsl`
  在参考自己的代码里从不产出（`grep -c` 为 0/1-且-1-是测试），所以这里没有、也不该有。

### 3. `internal/inventory/sourcetable_test.go`（新文件，5 个函数）

- `TestEveryDeclaredSourceIsReachable`：把上面五张表全指向 `t.TempDir()` 里的夹具
  （path 目录、一个 `Exec=<绝对路径>` 的 desktop 条目、flatpak/nix/snap/brew/`Foo.app/Contents/MacOS/Foo`/
  choco/scoop/winget 各一个可执行文件），跑 `discoverPath` + `discoverLinux` + `discoverMacOS` +
  `discoverWindows`，收集候选的 `Sources`，断言**恰好等于** `discoverySources` 集合。
  这条同时证「没有死常量」和「没有表外来源」。
- `TestTheSourceVocabularyIsClosed`：`discoverySources` 无重复、值全部 kebab-case、
  且不含 `dpkg`/`rpm`/`pacman`/`registry`/`wsl`；`SourceWinGet == "winget"` 逐字钉住
  （参考 serde 是 `win-get`，见上面 §3 条）。
- `TestTheWindowsChocolateyRootFollowsProgramData`：`%ProgramData%` 未设 / 等于
  `C:\ProgramData`（去重成一条）/ 重定向到 `D:\CorpData`（两条，第二条是
  `<ProgramData>\chocolatey\bin`）三格；另断言 `$LOCALAPPDATA` 设了才有 winget 根。
- `TestThePlatformRootTablesAreTheReferenceTables`：四张表（`linuxPackageRoots`/`brewRoots`/
  `macAppRoots`/`windowsToolRoots`）在固定环境（`HOME=/home/tester`、`ProgramData` 与
  `LOCALAPPDATA` 空）下**按值**钉住。加这一条是因为实跑变异发现前一条测试够不着生产表：
  它把表 stub 成夹具，删掉生产表里的 Scoop 根**全绿**。
- `TestTwoSpellingsOfOneRootAreOneRoot`：`sameRoot` 六格（分隔符、大小写、`\.`）+
  `dedupeRoots` 保留首个拼写。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
- 本轮的候选集合在**默认安装的机器上逐字节不变**（新增根与旧根同目录时被 `dedupeRoots` 吃掉）；
  只在 `%ProgramData%` 被重定向的 Windows 上**多出** Chocolatey 候选。
- 排名（`Priority`/`qualityBetter`）、`qualityOrder`、merge 的「首个非空描述胜」全部不动。

## 门槛命令与 PASS 基线（HEAD `8014517` 实测）

```
gofmt -l cmd internal      # 空
go vet ./...               # 净
go test -count=1 ./...     # 全绿
--- PASS 2113   --- SKIP 15   --- FAIL 0   包 36   测试函数 1719
GOOS=linux/windows/darwin go build ./...   # 过
```

本轮目标（实测）：PASS **2122**（基线 2113，+9 = 5 个新函数 + 4 个子测试），SKIP 15（=），
FAIL 0，包 36（=），函数 **1724**（+5）。

## mutation 计划与实跑结果（7 条全部实跑变红并 `md5sum -c` 逐字节还原）

| # | 注入的缺陷 | 变红的 pin |
|---|---|---|
| M1 | 删掉 `windowsToolRoots` 的 `%ProgramData%` 支 | `TestTheWindowsChocolateyRootFollowsProgramData/program_data_is_redirected` |
| M2 | `dedupeRoots` 改成 `return roots` | 同测试 `program_data_is_the_literal_path` + `TestTwoSpellingsOfOneRootAreOneRoot` |
| M3 | `discoverySources` 删掉 `SourceBrew` | `TestEveryDeclaredSourceIsReachable` |
| M4 | winget 根改挂 `SourceChocolatey` | `TestTheWindowsChocolateyRootFollowsProgramData/local_app_data_adds_the_winget_links` |
| M5 | `sameRoot` 不再折大小写 | `TestTwoSpellingsOfOneRootAreOneRoot` |
| M6 | 删掉 Scoop 根 | `TestThePlatformRootTablesAreTheReferenceTables`（**M6 第一版是绿的，见下**） |
| M7 | `nixRoots` 删掉系统 profile | `TestThePlatformRootTablesAreTheReferenceTables` |

M6 第一次实跑是**绿**的，因此补了 `TestThePlatformRootTablesAreTheReferenceTables`：原计划里
「winget 根改挂 chocolatey 会让 `TestEveryDeclaredSourceIsReachable` 变红」这条判断是错的——那条
测试把平台表 stub 成夹具，只能证明「来源可达」，证明不了「生产表接对了根」。

## 不做（写进 `.pending-bugs.md`）

- 不接 dpkg/rpm/pacman/registry/wsl：参考自己也不接（见上 §1 证据）。若将来要接，那是**新增能力**
  而非对齐，且需要拍板「多来源清点要不要显示在 Integrations 页」。
- 不改 `winget` → `win-get`：改的是用户看到的字，无收益，见上 §3。
