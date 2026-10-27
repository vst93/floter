# P10-o 报告：工具配方表的三张表就是判据

规格 `docs/rounds/P10-o-spec.md`。基线 HEAD `15e53c8`（P10-n）。

## ① 意图与证据

`internal/tools` 是旧版 `tool_catalog.rs` 的移植，移植时把参考的**三张表压成了一张**：

- 参考的探测名是 `ProbeCandidates { macos, linux, windows }`
  （`main:src-tauri/src/extensions/tool_catalog.rs:66-86`），本仓是扁平的 `Probes []string`。
- 参考的包管理器带 `platforms`（`:171-206`），本仓的 `Manager` 没有这个字段。
- 参考有六条「表自己得成立」的不变量（`r67_every_tool_has_a_recipe_on_every_platform` 等），
  本仓的 `TestCatalogIsConsistent` 只查了 id 唯一 / 探测名非空 / 配方里的 manager 存在。

后果是**同一台 macOS 上，一个名叫 `fdfind` 的文件会被当成 `fd` 已安装**，而参考明写
「`fdfind` 是 Linux 的拼写，macos 的表只有 `fd`」，并有测试钉住
（`r67_tool_detection_uses_the_platform_candidates`，`:698-717`）。
本仓甚至把相反结论钉成了一条 pin：`internal/tools/catalog_test.go:96,110` 在 `PlatformMacOS`
下写入 `fdfind` 并断言 `fd` 已安装。

## ② 落地

### 2.1 探测名按平台分表（产品改动，S）

`internal/tools/catalog.go`：

- 新 `type ProbeTable map[string][]string` + 三个方法/构造：
  - `ProbesEverywhere(names ...string) ProbeTable` —— 三平台同名的一行式（十二条里十条用它）；
  - `For(platform)` —— 空键回落宿主（与同包 `candidateNames` 同一条兼容规则），
    未知键也回落宿主：答案必须是某个真实平台的表，不能是空表；
  - `All()` —— 按 macos→linux→windows 去重取并集，给搜索用。
- `Entry.Probes` 由 `[]string` 改 `ProbeTable`；`fd` 与 `bat` 是仅有的两条需要写全三列
  （`fdfind`/`batcat` 只在 linux）；`Probe()` 改读 `entry.Probes.For(platform)`。
- `internal/shell/shell.go:2562`（`detectedInstallCommand`）同样改成
  `entry.Probes.For(platform)` —— 它是「发现的可执行名 → 目录里的配方」的匹配，
  本来就该用**这个平台**的名字表。

### 2.2 搜索 haystack 只留一个答案（顺带收口）

同一段「id + name + keywords + probes 拼串」原先有三份：`State.Matches`（`catalog.go:376-386`）、
`internal/launcher/tools.go:46-48` 与 `:82-84`。新增 `State.SearchHaystack()`（小写、含
`Probes.All()` 并集），三处都读它。搜索词与平台无关：知道它叫 `batcat` 的人在任何平台都该搜得到，
所以 haystack 用并集而不是当前平台那一列。

### 2.3 包管理器声明平台（纯增数据 + pin）

`Manager.Platforms []string`（brew=macos+linux、winget=windows、pacman/apt/dnf=linux、
npm/cargo/pipx=三平台，照参考 `PACKAGE_MANAGERS`）+ `ManagerPlatforms(id)`。
**`DetectManagers` 一字未动**：参考的 `detect_package_managers` 也不按平台过滤，
平台字段只服务于「配方表不得把 winget 写到 linux 上」这条校验。

### 2.4 六条表不变量（新文件 `internal/tools/catalogcontract_test.go`，6 个函数）

1. `TestEveryToolHasARecipeOnEveryPlatform` —— 每工具三平台都有配方。
2. `TestEveryRecipeNamesAManagerValidOnThatPlatform` —— 每条配方的 manager 在该平台有意义
   （+ `ManagerPlatforms` 本身：未知 id 为 nil，winget/pacman/apt/dnf 单平台，brew 两平台，
   npm/cargo/pipx 三平台）。
3. `TestTheProbeTableIsScopedToItsPlatform` —— `fdfind`/`batcat` 只在 linux 算数，`fd`/`bat` 三平台都算；
   并落到 `Probe()` 的实际结果；空键/未知键回落；`All()` 并集去重。
4. `TestTheSearchHaystackIsTheUnionOfEveryPlatformsProbes` —— `bat` 命中 `bat`/`batcat`/`高亮`，
   `ripgrep` 命中 `rg`，不命中别的工具的探测名，haystack 是小写，空查询不命中。
5. `TestTheCatalogStaysACuratedDozen` —— 目录 10–14 条、id 唯一、关键词 2–5 个且不重复非空。
6. `TestLaunchHintsAreWellFormed` —— argv 非空、`NeedsTerminal` 只与 `Launch` 同时出现、
   只有 flameshot（`[flameshot gui]`，不需要终端）与 lazygit（需要终端）带 launch。

### 2.5 改掉那条钉反的 pin（S）

`internal/tools/catalog_test.go:93-120`：`Look(macos)` + `fdfind` ⇒ fd **未**安装；
`Look(linux)` + `fdfind` ⇒ 已安装（linux 半在 Windows 宿主上跳过正断言，沿用
`platformcriteria_test.go:76-88` 的既有写法）。注释写明
`P10-o 重定基线：旧结构（一张扁平探测表）→ 新结构（按平台分表）`。

## ③ 台账

- **V = 0**：无视觉数值，本轮不出图（非视觉轮）。
- **B = 0**：行为断言零改动——`InstallCommand`/`ChosenRecipe`/`Look` 的既有断言、
  launcher 的 `install:`/`invoke:` 行断言全部零改动全绿。
- **S = 1**：`Entry.Probes []string` → `ProbeTable`（一张表 → 按平台分表）。连带改动只有
  **构造**，不是断言放宽：`catalog_test.go` 一条改结论（并注明旧结构→新结构）、
  `launcher/filters_test.go` 5 处与 `extensions/probejudgement_test.go` 1 处把字面量换成
  `tools.ProbesEverywhere(…)`。

## ④ 门槛（全部实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿，EXIT=0
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2128（基线 2122，+6）
grep -c -- "--- SKIP"                                  # 15（=）
grep -c -- "--- FAIL"                                  # 0
包数（ok + no test files）                              # 36（34 个有测试）
测试函数                                               # 1730（基线 1724，+6）
GOOS=linux/windows/darwin go build ./...               # 三平台过
```

## ⑤ 变异（5 条实跑变红 → 还原 → `md5sum -c` 逐字节 → `touch` 复跑）

基线：`md5sum internal/tools/catalog.go` = `51f8cbe20b74fb1df46a1c37b92514b8`，
每条还原后 `md5sum -c /tmp/mut.md5` 均输出「成功」，随后 `touch` 避免测试缓存复用。

- **M1**（fd 的 macos 表加回 `fdfind`）：
  ```
  --- FAIL: TestLookDetectsToolsAndManagers (0.00s)
      catalog_test.go:114: fdfind was detected as fd on macos, where that name does not exist
  --- FAIL: TestTheProbeTableIsScopedToItsPlatform (0.00s)
      catalogcontract_test.go:135: fd probes "fdfind" on macos: got true, want false
      catalogcontract_test.go:144: a macos probe resolved fdfind
  ```
- **M2**（删掉 npm 的 `PlatformWindows`）：
  ```
  --- FAIL: TestEveryRecipeNamesAManagerValidOnThatPlatform (0.00s)
      catalogcontract_test.go:73: tldr uses npm on windows, where it is not available (it declares [macos linux])
      catalogcontract_test.go:102: ManagerPlatforms(npm) = [macos linux], want three platforms
  ```
- **M3**（`Probe()` 里 `For(platform)` 换成 `All()`）：
  ```
  --- FAIL: TestLookDetectsToolsAndManagers (0.00s)
      catalog_test.go:114: fdfind was detected as fd on macos, where that name does not exist
  --- FAIL: TestTheProbeTableIsScopedToItsPlatform (0.00s)
      catalogcontract_test.go:144: a macos probe resolved fdfind
  ```
- **M4**（`SearchHaystack` 去掉探测名并集）：
  ```
  --- FAIL: TestTheSearchHaystackIsTheUnionOfEveryPlatformsProbes (0.00s)
      catalogcontract_test.go:181: bat does not match "batcat"
      catalogcontract_test.go:189: ripgrep does not match its own binary name rg
      catalogcontract_test.go:194: the haystack "bat bat 高亮 highlight cat gaoliang" does not carry "batcat"
  ```
- **M5**（删掉 lazygit 的 `NeedsTerminal`）：
  ```
  --- FAIL: TestLookDetectsToolsAndManagers (0.00s)
      catalog_test.go:141: the terminal hint is wrong
  --- FAIL: TestLaunchHintsAreWellFormed (0.00s)
      catalogcontract_test.go:265: a full-screen TUI needs a real terminal
  ```

还原后复跑：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` EXIT=0。

## ⑥ 复核：这一轮到底改了什么用户可见的东西

只有一处，而且是把一个不该成立的「已安装」去掉：macOS/Windows 上一个叫 `fdfind`（或 `batcat`）
的可执行文件不再让 launcher 把 `fd`/`bat` 当成已装。搜索侧反而更宽：`batcat`/`fdfind` 在任何平台
都仍然搜得到（haystack 用并集）。安装命令、配方选择顺序、行的 ID/标题/详情/来源全部一字未动。

## ⑦ 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 44 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）：**#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`；外加 register 只留 `cmd`、`open` 是否拒一切 query、
   未知宿主是否改成静默忽略。
3. `internal/tools` 这一族收官：来源表（P10-n）、配方表的三张表与本轮六条不变量都已钉住。
4. 下一轮建议：`internal/extensions/candidates.go` 的候选排名（唯一还没按参考逐条核对过的排序面）。
