# P10-o 规格：工具配方表的三张表就是判据（探测名按平台分表 · 包管理器声明平台 · 参考六条表不变量）

## 意图

`internal/tools` 是旧版 `tool_catalog.rs` 的移植，但移植时把参考的**三张表压成了一张**：
参考的探测名是 `ProbeCandidates { macos, linux, windows }`（`main:src-tauri/src/extensions/tool_catalog.rs:66-86`），
包管理器带 `platforms`（`:171-206`），本仓是一张扁平 `Probes []string`（`internal/tools/catalog.go:88-89`）
和一个没有平台字段的 `Manager`（`catalog.go:41-43`）。后果是**同一台 macOS 上，一个名叫 `fdfind` 的文件
会被当成 `fd` 已安装**——参考明写「`fdfind` 是 Linux 的拼写，macos 的表只有 `fd`」，并且有测试钉住这一点
（`r67_tool_detection_uses_the_platform_candidates`，`:698-717`）。

本轮把这一个答案收口，并把参考里那六条「表自己得成立」的不变量补成本仓的 pin：
现在 `TestCatalogIsConsistent`（`catalog_test.go:21-57`）只查了 id 唯一、探测名非空、配方里的 manager 存在，
参考还查了「每个平台都有配方」「配方里的 manager 在该平台有意义」「关键词 2–5 个且不重复」「目录规模 10–14」
「launch 提示良构」。本仓的**数据全都满足**这些，缺的是把它们钉住——数据一旦漂，没有东西会红。

为什么现在做：P10-n 收官了 `internal/inventory` 的来源表（同一族：表就是判据），上一轮报告的「仍待」第 4 条
点名了 `internal/tools` 的配方表。用户实测反馈为空、合约 §6 全部 `已落地`，按「三平台一致性 → 基础功能 →
像素细节」取三平台一致性。非视觉轮，不出图。

## 现状实测（动手前读代码确认，行号为 HEAD `15e53c8`）

```
$ grep -n "Probes" internal/tools/catalog.go
88:	// Probes are the executable names that mean "this tool is here".
89:	Probes []string
108,120,130,141,151,161,172,182,192,203,213,224: 十二个条目各一条
300:func Probe(...)  301: for _, name := range entry.Probes
380: strings.Join(s.Probes, " ")          // Matches 的 haystack

$ grep -n "fdfind" internal/tools/catalog_test.go
96: writeExecutable(t, dir, "fdfind")
110: t.Error("fdfind was not detected as fd")     ← 在 PlatformMacOS 上断言「算已安装」
```

- `internal/tools/catalog.go:41-57`：`Manager{ID, Name, Candidates}` + 八个 manager，**没有** `platforms`。
- `internal/tools/catalog.go:141`（fd）、`:172`（bat）：`[]string{"fd","fdfind"}` / `[]string{"bat","batcat"}`，
  不分平台。
- `internal/tools/catalog_test.go:93-120`：`Look(dirs, PlatformMacOS)` 里写 `fdfind` 并断言 `fd` 已安装——
  **这条 pin 钉住的正是与参考相反的结论**。
- `internal/launcher/tools.go:46-48` 与 `:82-84`：把 id/name/keywords/probes 拼成 haystack 的同一段代码
  抄了两份，`tools.State.Matches`（`catalog.go:376-386`）里是第三份。
- `internal/launcher/filters_test.go:223-228,275`：测试字面量直接构造 `tools.Entry{Probes: []string{…}}`。

## 落地项

1. **探测名按平台分表** — `internal/tools/catalog.go`
   - 位置：`Entry.Probes []string`（`:88-89`）+ 十二条数据（`:108`–`:224`）+ `Probe`（`:300-301`）。
   - 现状：一张扁平表，`Probe` 不分平台地遍历它。
   - 应改成：`Probes ProbeTable`，`type ProbeTable map[string][]string`，键是三个 `Platform*` 常量；
     `ProbesEverywhere(names...)` 是「三平台同名」的一行式（十二条里十条用它）；
     `ProbeTable.For(platform)` 取该平台的名字表，空键回落宿主（与同包 `candidateNames` 的兼容规则一致）；
     `ProbeTable.All()` 按 macos→linux→windows 去重取并集（给搜索用）。
     fd 改成 `{macos:{"fd"}, linux:{"fd","fdfind"}, windows:{"fd"}}`，bat 同理（`batcat` 只在 linux）。
     `Probe()` 改读 `entry.Probes.For(platform)`。
2. **搜索 haystack 只留一个答案** — `internal/tools/catalog.go` + `internal/launcher/tools.go`
   - 现状：`Matches` 里一份、launcher 里两份，都是 id/name/keywords/probes 的拼接。
   - 应改成：`State.SearchHaystack()`（小写、含 `Probes.All()` 的并集——搜索词与平台无关，
     「知道它叫 `batcat` 的人在任何平台都该搜得到」）；`Matches` 用它；launcher 两处 `Search:` 用它。
3. **包管理器声明平台** — `internal/tools/catalog.go`
   - 现状：`Manager` 没有平台字段，`DetectManagers` 只看名字在不在。
   - 应改成：加 `Platforms []string`（brew=macos+linux、winget=windows、pacman/apt/dnf=linux、
     npm/cargo/pipx=三平台，照参考 `PACKAGE_MANAGERS`）+ `ManagerPlatforms(id) []string`。
     **`DetectManagers` 不变**（参考的 `detect_package_managers` 也不按平台过滤，平台字段只服务于配方表校验）。
4. **补参考的六条表不变量** — 新文件 `internal/tools/catalogcontract_test.go`
   - 每个工具在三个平台上都至少有一条配方（参考 `r67_every_tool_has_a_recipe_on_every_platform`）。
   - 每条配方的 manager 在**该平台**有意义（参考 `r67_every_recipe_names_a_manager_valid_on_that_platform`）。
   - 探测名按平台分表的行为（参考 `r67_tool_detection_uses_the_platform_candidates`）：`fdfind` 在 linux 算 fd、
     在 macos 不算；`batcat` 同理；`rg` 是 ripgrep 的二进制名。
   - 关键词 2–5 个、互不重复、非空（参考 `r68_keywords_are_two_to_five_unique_and_non_empty`）+ 目录规模 10–14。
   - launch 提示良构：argv 非空；`NeedsTerminal` 只在有 `Launch` 时成立；带 launch 的只有 flameshot 与 lazygit
     （参考 `r69_only_gui_and_tui_tools_carry_a_launch_hint`、`r70_needs_terminal_separates_gui_from_tui`）。
   - 搜索并集：`Matches(["rg"])`、`Matches(["batcat"])` 命中，跨平台仍然搜得到。
5. **改掉那条钉反的 pin** — `internal/tools/catalog_test.go:93-120`
   - 现状：`Look(macos)` + `fdfind` ⇒ fd 已安装。
   - 应改成：`Look(macos)` + `fdfind` ⇒ fd **未**安装；`Look(linux)` + `fdfind` ⇒ 已安装
     （linux 那半在 Windows 宿主上跳过正断言，沿用 `platformcriteria_test.go:76-88` 的既有写法）。
     注释写 `P10-o 重定基线：旧结构（一张扁平探测表）→ 新结构（按平台分表）`。

## 行为契约零改动红线

- **`DetectManagers` 的返回集合不变**：平台字段是声明，不是过滤条件（与参考一致）。
- **搜索词不变**：`batcat`/`fdfind`/`rg` 仍然搜得到对应工具（`Probes.All()` 的并集）。
- **安装命令不变**：`InstallCommand` 与 `ChosenRecipe` 的选择顺序一字不动（`catalog_test.go:63-91,153-168` 零改动全绿）。
- **launcher 的行不变**：`install:` / `invoke:` 两族的 ID、Title、Detail、Source、执行路径不变，
  只把 `Search:` 的来源换成 `SearchHaystack()`。
- 不动高度链、键盘模型、会话管理、设置写回、i18n、命令接线。

## 门槛命令 + PASS 基线（本轮实测，HEAD `15e53c8`）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2122（基线）
grep -c -- "--- SKIP"                                  # 15
grep -c -- "--- FAIL"                                  # 0
包数（ok + no test files）                              # 36（34 个有测试）
测试函数（grep -rh '^func Test' internal cmd）          # 1724
GOOS=linux/windows/darwin go build ./...               # 三平台过
```

目标：PASS ≥ 2122 + 新增，函数 1724 + ≥6，SKIP 15（不变），FAIL 0。

## 台账预判

- **V = 0**（无视觉数值）。
- **B = 0**（行为断言零改动；`catalog_test.go:63-91,153-168`、`launcher` 的行断言全绿）。
- **S = 1**：`Entry.Probes []string` → `ProbeTable`（探测名从一张表变成按平台分表），
  连带 `catalog_test.go:93-120` 与 `filters_test.go` 的字面量改构造；注释写明旧结构 → 新结构。

## mutation 计划（每条实跑，看到红 → 还原 → `md5sum -c` → `touch` 复跑）

- **M1**：把 fd 的 macos 表改成 `{"fd","fdfind"}` → `TestTheProbeTableIsScopedToItsPlatform` 与改后的
  `TestLookDetectsToolsAndManagers` 必须红。
- **M2**：删掉 npm 的 `PlatformWindows` → `TestEveryRecipeNamesAManagerValidOnThatPlatform` 必须红（tldr 的 windows 配方）。
- **M3**：`Probe()` 里 `entry.Probes.For(platform)` 换成 `entry.Probes.All()` → 平台分表那条必须红
  （macos 上 `fdfind` 又算 fd）。
- **M4**：`SearchHaystack` 去掉 `Probes.All()` → 搜索并集那条必须红（`Matches(["batcat"])`）。
- **M5**：从 lazygit 条目删掉 `NeedsTerminal` → launch 良构/`r70` 那条必须红。
