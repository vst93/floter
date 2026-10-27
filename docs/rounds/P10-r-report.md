# P10-r 报告：候选 `Sources` 的顺序就是判据——表按参考声明序，合并时按表排

规格 `docs/rounds/P10-r-spec.md`。方向：P10-q 报告 §5.6 的遗留建议（参考对候选的
`sources.sort()`，本仓是发现顺序）。**先量后做**，量完发现遗留建议的判断只对一半，见 §0。

## §0 量出来的结论：排序不是给界面看的，是给下一个人看的

| 问题 | 参考 | 本仓（改前） | 结论 |
|---|---|---|---|
| `sources` 的顺序 | `insert()` 每次 push 后 `sources.sort()`（`inventory.rs:996-998`），`Ord` = 枚举声明序（`:24-40`） | `mergeInto` 只 `append`（`inventory.go:188-190`）= **发现序** | 真分歧 |
| 「声明序」那张表 | `Path, Desktop, Dpkg, Rpm, Pacman, Flatpak, Snap, Nix, Brew, LaunchServices, Registry, Scoop, Chocolatey, WinGet, Wsl` | `discoverySources`（`inventory.go:50-61`）注释自称「in the order the reference declares them」，实际 `chocolatey` 在 `scoop` **前** | **表的注释是假的**，顺序与参考相反 |
| 谁读这个顺序 | 候选选择器的 sources 行（前端 `toolCandidates` 的 sources） | `internal/shell/system.go:735-736` 把 `Sources` join 进选择器详情 —— **UI 确实显示**，但它读的是 `extensions.ExecutableCandidate.Sources`，恒为 `["path"]` | 今天无读者 → 不改界面，只把表与合并改对 |

所以 P10-q 那条「先确认 UI 是否显示 sources」的答案是：显示，但**不是这一份**。
`inventory.Candidate.Sources` 的顺序目前没有读者；真正的缺陷是**一张自称参考序、
实际不是的表**。下一个人（比如把选择器接回 inventory 的那一轮）会照它读。这轮把表改对，
并让合并按表排——顺序从此是判据，不是副产品。

## §1 落地

- `internal/inventory/inventory.go:50-61`：`discoverySources` 的 `SourceScoop` /
  `SourceChocolatey` 换回参考声明序（`scoop` 在前）。注释同时点明它是**顺序的唯一来源**。
- 新增 `sourceRank`（`inventory.go:71-78`）：返回 source 在表里的下标，表外值排末尾；
  注释写清「`Sources` 按声明序而不是发现序，因为参考的 `insert` 每次都排」。
- `mergeInto`（`inventory.go:205-211`）：`append` 之后 `sort.SliceStable` 按 `sourceRank`
  排。稳定排序让同序来源保持先到的位置（参考的 `sort()` 也是稳定语义的 `slice::sort_by`）。
- 新 `internal/inventory/sourcesorder_test.go`（2 函数）：
  ①`TestTheVocabularyIsInTheReferenceOrder` 逐项断言十项真值表 + `scoop` 在 `chocolatey`
  前 + 表外值排末尾；②`TestSourcesAreListedInTheReferenceOrder` 以**逆声明序**驱动合并
  （`winget, flatpak, desktop, path`，再补一次 `desktop` 验去重），断言结果
  `[path desktop flatpak winget]`——`append`-only 的旧形状会答 `[winget flatpak desktop path]`。

产品代码只改一个列表项次序 + 一处排序。`Priority` 的算术、`ranked()` 的比较链、
`discover*` 的走法、`extensions.ExecutableCandidate.Sources` 恒为 `["path"]` 的现状
**一字未动**。

## §2 台账

**V=0 / B=0 / S=1**。S = `Candidate.Sources` 的顺序（旧结构：发现序 → 新结构：声明序）；
`discoverySources` 的两项次序同步改（表的顺序本身就是新结构的一部分）。
既有 pin 全部零改动通过（`inventory_test.go:16-42`、`desktopentry_test.go:333`、
`sourcetable_test.go:85-140` 都不依赖来源顺序）。没有放宽断言，没有加容差，没有 `t.Skip`。

## §3 门槛（本轮实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2137（基线 2135，HEAD 1465ad7 实测）
--- SKIP                                               → 15（=）
--- FAIL                                               → 0
测试函数（^func Test 计数）                              → 1739（基线 1737，+2）
GOOS=linux/windows/darwin go build ./...                → 三条都过
```

本轮非视觉，不出图（无墨与面的改动）。

## §4 变异（3 条实跑变红 + 还原）

原始报错串（`go test -count=1 ./internal/inventory`，逐条注入后摘录）：

1. 删掉 `mergeInto` 里的 `sort.SliceStable` 整块：
   `--- FAIL: TestSourcesAreListedInTheReferenceOrder` /
   `sourcesorder_test.go:61: Sources = [winget flatpak desktop path], want the declaration order [path desktop flatpak winget]`
2. `discoverySources` 的 `scoop`/`chocolatey` 换回旧次序：
   `--- FAIL: TestTheVocabularyIsInTheReferenceOrder` /
   `sourcesorder_test.go:29: discoverySources = [path desktop flatpak snap nix brew launch-services chocolatey scoop winget]` /
   `want the reference's declaration order [path desktop flatpak snap nix brew launch-services scoop chocolatey winget]`
3. 比较符 `<` → `>`：
   `--- FAIL: TestSourcesAreListedInTheReferenceOrder` /
   `sourcesorder_test.go:61: Sources = [winget flatpak desktop path], want the declaration order [path desktop flatpak winget]`

每条注入后 `cp` 还原、`md5sum -c /tmp/md5.txt` 逐字节核对（三次全部「成功」），
再 `touch internal/inventory/*.go` 复跑 `./internal/inventory ./internal/shell ./internal/extensions`
三条全绿（避开测试缓存）。

## §5 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 47 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. register 只留 `cmd`、`open` 是否拒一切 query、未知宿主是否改成静默忽略，三条拍板项照旧。
4. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
5. 本轮新记的**故意不同**（不改，已 pin 过的延续）：`extensions.ExecutableCandidate.Sources`
   恒为 `["path"]`（参考的选择器候选来自 inventory 的全来源）；`discoverySources` 缺参考的
   五项（`dpkg/rpm/pacman/registry/wsl`，参考自己也不产出，见 P10-n）。
6. 下一轮建议：`inventory` 这一族（来源表 / 桌面条目 / 路径解析 / 跨根优先级 / 排名算术 /
   来源顺序）已全部收口，**别再在这族找小修**。下一步按优先级只剩「三平台一致性」里的
   具体表面——建议从 `internal/apps` 的图标来源（Windows exe 图标 / macOS icns / Linux
   `Icon=`）挑一条，先量三平台各自取不到图标时的回退是否一致。
