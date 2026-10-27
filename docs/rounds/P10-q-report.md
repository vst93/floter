# P10-q 报告：发现排名的八项评分就是判据——逐项钉住，补回缺失的 `~/bin`

规格 `docs/rounds/P10-q-spec.md`。方向：P10-o 报告遗留建议（`internal/extensions/candidates.go`
候选排名是「唯一还没按参考逐条核对过的排序面」）。**先量清楚再动手**，量完结论与建议不同，
见 §0。

## §0 量出来的结论（建议方向其实已经落地了一半）

| 排序面 | 参考 | 本仓 | 结论 |
|---|---|---|---|
| Detected 12 行的排名 | `inventory::candidate_priority` + `rank_candidates`（`main:inventory.rs:323-382`） | `inventory.Priority` + `ranked()`（`curated.go:86-119`、`inventory.go:218-236`） | 已移植，**但只有序关系 pin，算术没 pin**；缺一项输入 |
| curated 白名单 | 84 个名字（`curated_tools.rs:44-140`） | 84 个名字（`curated.go:23-45`） | **逐字相同**（`diff` 空，两侧各 84 行） |
| 候选选择器的行序 | `resolver.rs:55-108`：按 `(exact_match, version, stability, history)` 排序，只交出最高分那一档 | `candidates.go:74-127`：PATH 先序 + recordedPath 短路 | **故意不同**，等价于「history 让 recorded 唯一最优」；行序差别已 pin |

`rank_candidates` **不服务于选择器**：它只被 `commands/extensions.rs:419` 的 discovery 建议列表
调用（本仓的 Detected 列表）。所以「候选排名未核对」这一条的真实缺口是
**排名算术没有 pin + `~/bin` 这一项输入漏了**，不是选择器要加排名。

`~/bin` 实测（`HOME=/home/tar`，改前）：`~/bin/a52dec` → **1000**，`~/.local/bin/a52dec` → **1040**。
参考的 owned 表是 `~/.local/bin`、`~/.cargo/bin`、`~/.nix-profile/bin`、`~/bin`
（`inventory.rs:410-418`）——`~/bin` 是最老的那一项（Debian 的 user bin，多数桌面默认在 `PATH` 里），
本仓漏了它，等于 `~/bin` 里的工具比 `~/.local/bin` 里的同名工具低 40 分，足以掉出前 12 行。

## §1 落地

- `internal/inventory/curated.go:153-183`（`isUserOwnedDirectory`）：owned 表按参考的四项顺序
  排前，补回 `~/bin`；本仓自己的两项（`go/bin`、`.local/share/flatpak/exports/bin`）补在后并注明
  「本仓补充」。注释同时写清**第二处故意不同**：参考用裸 `starts_with`（`inventory.rs:412`），
  本仓要求目录分隔符，`~/.local/bin-old/x` 不算。
- 新 `internal/inventory/prioritycontract_test.go`（4 函数）：
  ①`TestPriorityArithmetic` 九行，逐行断言**实测整数**并写出算术（1000/1300/800/796/900/952/
  1200/1060/1260）；②`TestPriorityUserOwnedDirectories` 六项加分 + 四项不加分 + 「目录自己不算在
  自己里面」；③`TestPriorityClampsAtZero`（300 字符 variant 名 = −360 → 0）；④
  `TestRankedOrdersByQualityThenPriorityThenName`（quality 先于 priority：`native-support` 的
  `yyy` 压过 curated 的 `git`）。
- `internal/extensions/candidates_test.go` 末尾新增 `TestLookToolsIsDirectoryMajor`：钉住
  「整组 probe 按 PATH 先序」（`names=[alpha,beta]`、`dir1` 只有 `beta`、`dir2` 只有 `alpha`
  → `dir1/beta, dir2/alpha`），注释写清参考按 path id 字典序排（`resolver.rs:95-98`）而本仓按
  宿主先序，以及 `rank_candidates` 不适用于此。

产品代码只改一个列表项（`~/bin`）+ 注释。`Priority` 的系数、`ranked()` 的比较链、
`lookToolsIn` 的循环嵌套**一字未动**。

## §2 台账

**V=0 / B=0 / S=1**。S = `isUserOwnedDirectory` 的输入表多了 `~/bin`（旧结构：参考四项里少一项 →
新结构：参考四项齐全 + 本仓两项）。既有 pin 全部零改动通过
（`inventory_test.go:78,102-123`、`candidates_test.go:28` 等）；没有放宽任何断言，
没有加容差，没有 `t.Skip`。

## §3 门槛（本轮实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2135（基线 2130，HEAD cbdccba 实测）
--- SKIP                                               → 15（=）
--- FAIL                                               → 0
测试函数（^func Test 计数）                              → 1737（基线 1732，+5）
包                                                     → 36
GOOS=linux/windows/darwin go build ./...                → 三条都过
```

本轮非视觉，不出图（无墨与面的改动）。

## §4 变异（5 条实跑变红 + 还原）

原始报错串（`go test -count=1`，逐条注入后摘录）：

1. 删掉 owned 表里的 `filepath.Join(home, "bin")`：
   `--- FAIL: TestPriorityUserOwnedDirectories` /
   `prioritycontract_test.go:133: Priority(/fixture-home/bin/a52dec) = 1000, want 1040 (base 1000 + user directory 40)`
2. `curatedBonus 300 → 200`：
   `--- FAIL: TestPriorityArithmetic` /
   `prioritycontract_test.go:100: Priority(git at /usr/bin/git) = 1200, want 1300 (base 1000 + curated 300)`
3. `ranked()` 的 `return left.Name < right.Name` → `>`：
   `--- FAIL: TestRankedOrdersByQualityThenPriorityThenName` /
   `prioritycontract_test.go:194: ranked order = [yyy git zzz aaa], want [yyy git aaa zzz]`
4. 删掉 `if score < 0 { score = 0 }`：
   `--- FAIL: TestPriorityClampsAtZero` /
   `prioritycontract_test.go:167: Priority(300-character variant) = -360, want 0 (clamped)`
5. `lookToolsIn` 的两层循环对调：
   `--- FAIL: TestLookToolsIsDirectoryMajor` /
   `candidates_test.go:160: order = ".../002/alpha", ".../001/beta"; want the first directory's match first (directory-major)`

每条注入后 `cp` 还原、`md5sum -c` 三文件逐字节核对（三行全部「成功」），
再 `touch` 四个源文件复跑 `./internal/inventory/ ./internal/extensions/` 全绿（避开测试缓存）。

## §5 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 46 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. register 只留 `cmd`、`open` 是否拒一切 query、未知宿主是否改成静默忽略，三条拍板项照旧。
4. 发布候选（命名先定）：`dist/SHA256SUMS`（三 matrix 同名冲突）、macOS `install.sh`、
   Windows `install.ps1`。
5. 本轮新记的**故意不同**（不改，已 pin）：`isUserOwnedDirectory` 要求目录分隔符（参考裸
   `starts_with`）；owned 表比参考多 `go/bin` 与 flatpak exports；选择器行序按 PATH 先序
   （参考按 path id 字典序）。
6. 下一轮建议：`internal/extensions` 里 `sources.sort()`（`inventory.rs:998` 参考对候选的
   sources 排序）在本仓是发现顺序 —— 先确认 UI 是否显示 sources，再决定钉或改。
