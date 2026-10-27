# P10-r 规格：候选 `Sources` 的顺序就是判据——表按参考声明序，合并时按表排

## 意图

参考在合并候选时对 `sources` 排序（`main:src-tauri/src/extensions/inventory.rs:996-998`：
`entry.sources.push(source); entry.sources.sort();`，`DiscoverySource` 的 `Ord` 是**声明序**
`:24-40`）。本仓的合并只 `append`（`internal/inventory/inventory.go:188-190`），而
「声明序」的唯一来源 `discoverySources`（`inventory.go:50-61`）注释自称「in the order the
reference declares them」，实际把 `chocolatey` 排在 `scoop` **之前**，与参考相反
（参考 `:24-40`：… `Registry`, `Scoop`, `Chocolatey`, `WinGet`, `Wsl`）。

为什么现在做：P10-q 报告 §5.6 遗留（「`sources.sort()` 在本仓是发现顺序，先确认 UI 是否显示
sources」）。量完的结论是——**UI 确实显示 sources**（`internal/shell/system.go:735-736`
把它们 join 进选择器详情），但今天只有 `extensions.ExecutableCandidate.Sources`
（恒为 `["path"]`）走到那里；`inventory.Candidate.Sources` 的**顺序**尚无读者。所以这不是
「加排序让界面变好看」，而是把**一张自称是参考序、其实不是的表**改对，并让顺序成为判据
（否则下一个人接 UI 时会读到一张错表）。产品只改一个列表项的次序 + 一次排序，行为零改动。

## 落地项（动手前已核对真实行号，HEAD `1465ad7`）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/inventory/inventory.go:50-61` `discoverySources` | `… SourceLaunchService, SourceChocolatey, SourceScoop, SourceWinGet` | `… SourceLaunchService, SourceScoop, SourceChocolatey, SourceWinGet`（参考声明序） |
| `internal/inventory/inventory.go:188-190` `mergeInto` | `append` 后不排序 → 发现序 | `append` 后按 `discoverySources` 的下标稳定排序 |
| 新增 `sourceRank` | 无 | 返回该 source 在 `discoverySources` 中的下标；表外值排末尾（`len(discoverySources)`） |
| 新 `internal/inventory/sourcesorder_test.go` | 无 | 两条 pin：①表的次序 = 参考声明序（`scoop` 在 `chocolatey` 前）；②乱序合并（`winget, flatpak, desktop, path`）后 `Sources` = `[path desktop flatpak winget]`，且重复来源不重复入表 |

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一行不动**；`Priority` 的算术、
`ranked()` 的比较链、`discover*` 的走法与顺序**一字不动**。本轮无墨无面（不出图）。
`extensions.ExecutableCandidate.Sources` 恒为 `["path"]` 的现状**不改**（参考的候选来自
inventory；本仓候选是 PATH-only，属已 pin 的故意不同，见 P10-q §0）。

## 门槛与 PASS 基线（HEAD `1465ad7` 实测，见报告 §3）

```
gofmt -l cmd internal                              → 空
go vet ./...                                      → 净
go test -count=1 ./...                            → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS" → 基线 2135
测试函数计数（^func Test）                         → 基线 1737
GOOS=linux/windows/darwin go build ./...           → 三条都过
```

## mutation 计划（≥2 条，实跑）

1. 删掉 `mergeInto` 里的排序调用 → `TestSourcesAreListedInTheReferenceOrder` 变红
   （得到发现序 `[winget flatpak desktop path]`）。
2. 把 `discoverySources` 里 `SourceScoop` / `SourceChocolatey` 换回旧次序 →
   `TestTheVocabularyIsInTheReferenceOrder` 变红。
3. （附加）把 `sourceRank` 的比较改成 `>` → 排序反转，同一 pin 变红。
