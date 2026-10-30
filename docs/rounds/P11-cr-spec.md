# P11-cr 规格：19 个没人调用的函数（死代码清理，一条也不许删错）

## 0. 意图与选题理由

**意图一句话**：把仓库里**确实**没有任何调用点的生产函数删掉，并用一条**机器可复跑的
扫描**把这个状态钉住——以后谁再留下孤儿函数，CI 里会红，而不是靠人记。

**为什么现在做**（选题顺序第 3 档「上一轮仍待第一条」）：`P11-cq` 的「仍待」第 1 条
就是这张清单，而且它连续被 **P11-cn / P11-co / P11-cp / P11-cq 四轮**记为「不是任何一轮
的族，建议单独一轮清」。它不是视觉轮、不是拍板项、不碰任何行为契约，正好是「一轮只做
一件事」的独立切片。合约 §6 全 `已落地`（只剩拍板项），用户实测 F1–F4 全收尾。

**为什么这轮不是视觉轮**：清单里 19 条里有 12 条是 `settingsui` / `launcher` / `terminalui`
的私有方法，删掉它们**零像素**（本轮的 dump 逐字节相同就是证据）。所以这轮出不了新图，
报告里给的是**前后 dump 相同**的读数，不是「我看到了什么变化」。

## 1. 基线（本轮实测，干净 worktree）

```
PASS 2799 · FAIL 0 · SKIP 58 · 测试函数 2079
gofmt -l cmd internal → 空 · go vet ./... → 净 · 三平台 build → 过
```

## 2. 清单复核（**动手前逐条实测，不照抄旧报告**）

`go run golang.org/x/tools/cmd/deadcode@v0.52.0 -test ./...` 本轮实测输出 25 行
（`-test` 把测试也当入口，所以只报**生产代码里也没人调**的）：

```
internal/calculator/calculator.go:563   SortNewestFirst
internal/extensions/custom.go:210       PreparedCustom.Discard
internal/extensions/install.go:191      Prepared.Version
internal/extensions/runfailure.go:71    RunErrorKeys
internal/extensions/store.go:42         Store.Paths
internal/inventory/sources.go:378       LookPath
internal/launcher/launcher.go:2890      App.firstSelectable
internal/settings/settings.go:267       LoadDefault
internal/settingsui/rows.go:734         App.rowLines
internal/settingsui/rows.go:862         App.rowTop
internal/settingsui/settingsui.go:1052  App.SelectedPage
internal/settingsui/settingsui.go:4475  App.text
internal/settingsui/settingsui.go:5612  App.freshnessDetail
internal/shell/system.go:417            numeric
internal/spawn/spawn.go:115             Terminal
internal/spawn/spawn.go:164             terminalEmulators
internal/terminalui/terminalui.go:693   App.Label
internal/usage/usage.go:50              Store.FilePath
（外加 6 条是测试文件里的私有 helper，见 §3.2）
```

### 2.1 两个**假阳性**——不删，写进报告

| 符号 | 为什么报它 | 判定 |
|---|---|---|
| `internal/shell/chrome_other.go:18` `dressWindowsFrame` | 它是 `!windows` 的**桩**；真身在 `chrome_windows.go:164`，被 `:147` 的 `stripSystemChrome` 调用 | **假阳性**（`deadcode` 在 Linux 上编译，看不到 windows 那支）。`GOOS=windows` 下 `deadcode` 自己跑不起来（`exec format error`），所以这条只能靠读代码判。**不删** |
| `internal/settingsui/settingsui.go:4475` `App.text`（以及 `rowLines`/`rowTop`/`freshnessDetail`） | 它们是同一族的**公开 API 形状**，不是被删的私有 helper；但本轮实测确认**没有调用点**（见 §2.2 的逐条计数） | 见 §2.2：**删**，但要在 `docs/` 记一笔「这几个是曾经有调用点、后来被合并掉的」 |

### 2.2 逐条复核（`grep -rn "<sym>" internal cmd --include='*.go'` 只命中定义行 + 注释）

全部 19 条生产符号本轮实测**只有自己的定义行与文档注释**命中，零调用点。
本轮**逐条读实现**确认删除不改变行为，并把「删了会怎样」写下来：

| # | 符号 | 实现是什么 | 删掉的后果 |
|---|---|---|---|
| 1 | `calculator.SortNewestFirst` | `sort.SliceStable(entries, …)` 一行的**导出**排序 | 零。`calculator.go:415` 有自己的一份 `sort.SliceStable(indices,…)`（另一个方向），`sort` import 仍在用 |
| 2 | `extensions.PreparedCustom.Discard` | 调 `p.cleanup()` | 零。`Commit` 的失败分支**自己**调 `p.cleanup()`（`custom.go:198-201`），这是唯一真正需要清包的时刻；`Discard` 是「用户放弃连接」的入口，但**没有任何调用方**实现那条路 |
| 3 | `extensions.Prepared.Version` | `return p.spec.Version` | 零。`Name()` 同族但**有**调用点？——实测 `Prepared.Name` 也零调用（见 §3.1 的注），本轮**只删 `Version`**：`Name` 被 `orphans.go:44` 的 `prepared.Manifest.ID` 那条路径旁证为「形状还在用」，而 `Version` 连形状都没人读 |
| 4 | `extensions.RunErrorKeys` | 返回 `runErrorKeys` 的**副本** | 零。注释说「for the i18n symmetry test」，但那条测试**不存在**（`grep -rn "RunErrorKeys" internal --include='*_test.go'` 零命中）。闭集本身由 `KnownRunErrorKey` 读，仍在用 |
| 5 | `extensions.Store.Paths` | `return s.paths` | 零。`s.paths` 在包内有 3 处直读（`:48/:127/:331`），没有外部读者 |
| 6 | `inventory.LookPath` | `return pathlist.Lookup(name)` 一行的**转发** | 零。`pathlist.Lookup` 自己的测试在（`pathlist_test.go:58` 只是**注释**提到 `inventory.LookPath` 的历史 bug，不是调用）。删掉后 `internal/inventory` 对 `pathlist` 的 import 变成孤立 → **同时删 import** |
| 7 | `launcher.App.firstSelectable` | 遍历 `a.Results()` 找第一行可选的 | 零。**注意**：它会调 `a.Results()`，那是整份目录重建——但既然没人调，删掉只会让帧更快。`rowSelectable` 仍被别处用（`nextSelectableIn`） |
| 8 | `settings.LoadDefault` | `Path()` + `Load()` | 零。`Path()` 仍被 `migrate.go:58` 与 `windowstore.go` 用 |
| 9-10 | `settingsui.App.rowLines` / `rowTop` | 各自 `return a.rowAligned(…)` 一行的转发 | 零。`rowAligned` 是真正的实现，仍被 `iconRow` 等调用 |
| 11 | `settingsui.App.SelectedPage` | `return a.Page` | 零。`a.Page` 在包内多处直读 |
| 12 | `settingsui.App.text` | 一行 `a.row(...)` 包一个 textInput | 零。它画的正是「带标签的文本框」——**这一支现在由 `configform.go` 的表单渲染走**，`text` 是被合并掉的旧路径 |
| 13 | `settingsui.App.freshnessDetail` | 拼一整句 detail 字符串 | 零。注释自己写着「kept for the panel's own readers (the shell's tests and diagnostics)」——**那些读者也不存在**（`grep` 零命中）。真正画 freshess 的是 `freshnessSection`，它按自己的寄存器画，仍绿 |
| 14 | `shell.numeric` | 读 `float64` / `json.Number` | 零。删掉后 `system.go` 的 `encoding/json` import 变成孤立 → **同时删 import**（本轮实测 `json` 在该文件只有 `:421` 这一处使用） |
| 15 | `spawn.Terminal` | `return TerminalAt("")` | 零。`TerminalAt` 被 `shell/custom.go:224` 的 `terminalAt` 变量与 `launch.go` 用；`Terminal` 这个空目录版本没有调用方（`TerminalAt("")` 就是它的行为，要的人直接写） |
| 16 | `spawn.terminalEmulators` | `return terminalEmulatorsFor(runtime.GOOS)` | 零。测试读的是 `terminalEmulatorsFor`（`spawn_test.go:33/42/55/144`），**不是**这个包装 |
| 17 | `terminalui.App.Label` | 会话名（标题或本地化默认） | 零。`LabelOf(index)` 是真正被外壳用的那一个（`shell` 的会话页读 `LabelOf`）。**P11-cq 报告 §3 第 3 条明确把它的删除留给这一轮** |
| 18 | `usage.Store.FilePath` | `return s.path` | 零。`s.path` 包内 3 处直读（`:149/:159/:169`） |

### 2.3 一条**不删**的（同族但不是死代码）

`terminalui.App.sessionTitle`（`terminalui.go:209`）是 `Label` **唯一**的读者（`Label`
被删后它的读者变成 `:1498` 与 `:1553`）——所以它**不是**死代码，`deadcode` 也没报它。
删 `Label` 之后必须复跑 `deadcode` 确认 `sessionTitle` 没掉进清单。

## 3. 落地项（位置 → 现状 → 应改成）

### 3.1 生产代码：删 18 个函数 + 2 处孤立 import

| 文件 | 删什么 | 附带 |
|---|---|---|
| `internal/calculator/calculator.go:562-565` | `SortNewestFirst` + 其注释 | 无（`sort` 仍被 `:415` 用） |
| `internal/extensions/custom.go:208-214` | `PreparedCustom.Discard` + 注释 | 无 |
| `internal/extensions/install.go:191` | `Prepared.Version`（保留 `Name`，见 §2.2#3 的注） | `Name`/`Version` 那行的对齐注释要改（现在只剩一行） |
| `internal/extensions/runfailure.go:69-71` | `RunErrorKeys` + 注释 | 无（`runErrorKeys` 仍被 `KnownRunErrorKey` 用） |
| `internal/extensions/store.go:41-42` | `Store.Paths` + 注释 | 无 |
| `internal/inventory/sources.go:372-378` | `LookPath` + 注释 | **删 `pathlist` import**（`:11`） |
| `internal/launcher/launcher.go:2889-2897` | `firstSelectable` + 它那段注释 | 注释里提到的 `firstRunnableResultIndex` 规则仍在 `nextSelectableIn` 里，**要留一句指路** |
| `internal/settings/settings.go:266-273` | `LoadDefault` + 注释 | 无 |
| `internal/settingsui/rows.go:733-736` | `rowLines` + 注释 | 无 |
| `internal/settingsui/rows.go:858-864` | `rowTop` + 注释 | 无 |
| `internal/settingsui/settingsui.go:1051-1052` | `SelectedPage` + 注释 | 无 |
| `internal/settingsui/settingsui.go:4473-4483` | `text` + 注释 | 无 |
| `internal/settingsui/settingsui.go:5610-…` | `freshnessDetail` + 注释（**整段函数体**，注意它跨行较长，读到函数尾的 `}`） | 检查 `freshnessWhen` 是否还有别的读者（实测 3 处） |
| `internal/shell/system.go:416-427` | `numeric` + 注释 | **删 `encoding/json` import**（`:7`） |
| `internal/spawn/spawn.go:114-116` | `Terminal` + 注释 | `runtime` import 仍被 `:165`/别处用？——**删 `terminalEmulators` 后要复查** |
| `internal/spawn/spawn.go:163-166` | `terminalEmulators` + 注释 | 若 `runtime` 因此孤立则一并删 |
| `internal/terminalui/terminalui.go:691-698` | `App.Label` + 注释 | 无（`LabelOf` 是外壳真正读的） |
| `internal/usage/usage.go:49-50` | `Store.FilePath` + 注释 | 无 |

### 3.2 测试文件里的 6 个孤儿 helper（同族，一并清）

`deadcode -test` 报的（都只在本文件定义、零调用）：

| 文件 | helper |
|---|---|
| `internal/launcher/filters_test.go:383` | `hasItem` |
| `internal/launcher/rowtrailing_test.go:267` | `strongestInk` |
| `internal/launcher/rowtrailing_test.go:284` | `channelDiff` |
| `internal/settingsui/raycast_test.go:61` | `renderThemed` |
| `internal/shell/detachlist_test.go:24` | `detachedTokens` |
| `internal/shell/detachlist_test.go:172` | `launcherColor` |

**删这些是「测试函数数量不减」红线内的**：它们不是 `Test*`，删掉不影响 PASS 计数；
但 `renderThemed`/`detachedTokens` 各自带**一段解释为什么这个半边的注释**，删前把
那段话里仍然成立的判据（例如「浅色半边一直是白，是 pin 的稳定读数面」）**搬进还在的
那个 helper 的注释**，别连知识一起删。

### 3.3 一条新的 pin：死代码扫描

新增 `internal/ci/deadcode_contract_test.go`，与 `frameaudit_contract_test.go` /
`textlines_contract_test.go` 同一形状（纯源码扫描，不跑 `deadcode` 二进制——CI 里
没装它，也不该为一条 pin 装一个工具）：

- **读**一份仓库内的清单 `internal/ci/deadcode_baseline.txt`（本轮生成），每行
  `<相对路径>:<符号名>`；
- **断言一**：清单里每一个 `(路径, 符号)` 在源码里**都不再有 `func` 定义**。
  于是「删了一半」会红：谁把某个函数加回来，这条就红。
- **断言二**：清单本身**只允许缩短**——它把「本轮删了这 18 个」写成一个可核对的账，
  不是一条空断言。

**为什么不做成「自动跑 deadcode」**：`deadcode` 要 `go run …@version` 联网 + 三平台
（`GOOS=windows` 下它自己 `exec format error`，本轮实测），CI 里跑不动；而这份清单是
**一次性的收口账**，不是长期守卫。长期守卫是「清单里的符号不许回来」。

## 4. 行为契约零改动红线

- **高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线**：一个字节不动。
  删的全是零调用点函数，没有任何一条路径经过它们（本轮 §5 的变异证明：把 `numeric`
  的**调用点**造出来才会红，说明它此前确实不在任何路径上）。
- **零像素**：`settingsui` / `launcher` / `terminalui` 三张表面的 dump 前后**逐字节相同**。

## 5. 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                 → 必须空
go vet ./...                          → 必须净
GOOS=windows go vet ./...             → 必须净
GOOS=darwin  go vet ./...             → 必须净
go test -count=1 ./...                → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 基线 2799，只许涨
go test -count=1 -v ./... | grep -c -- "--- FAIL"   → 0
GOOS=linux|windows|darwin go build ./...            → 三平台过
```

**PASS 预期**：2799 → 2800 或更高（新增扫描件 ≥1 条 `Test*`）。测试函数 2079 → ≥2080。
SKIP 不变（本轮不加 opt-in）。

## 6. mutation 计划（≥2 条，实跑，说明每条让哪个 pin 变红）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| **M1** | 把 `internal/shell/system.go` 的 `numeric` **加回来**（函数体原样） | 新 `TestTheDeadCodeBaselineStaysDeleted`：`shell/system.go:numeric` 又有了 `func` 定义 |
| **M2** | 把 `internal/inventory/sources.go` 的 `LookPath` 加回来（**不带** `pathlist` import，改用 `pathlist.Lookup` 会编译错 → 所以用 `_ = pathlist.Lookup` 形式？不行。改为：加回 `func LookPath(name string) (string, bool) { return "", false }`） | 同上一条 pin |
| **M3** | 把 `internal/ci/deadcode_baseline.txt` 里的一行**删掉**（模拟「清单被悄悄缩短以放过某个函数」） | 清单长度断言（**只允许缩短**是反过来的：删行会让「删了一半」那条断言少一条靶子）——所以 M3 改成**加一行**（凭空加一个不存在的符号），断言一仍绿而**断言二**（清单只许缩短/只许是本轮那 18 个）红 |
| **M4** | `internal/terminalui/terminalui.go` 把 `App.Label` 加回来 | 断言一红（`terminalui/terminalui.go:Label`） |

每条：注入 → 跑 → 抄原始报错 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑一次。

## 7. 出图（视觉轮的替代证据）

这轮**不是视觉轮**，但仍要证明「零像素」：

- 跑 `internal/settingsui` / `internal/launcher` / `internal/terminalui` 已有的 dump
  测试（opt-in），修前 / 修后各存一份到 `/tmp/floter-P11-cr-<surface>-<dark|light>-{before,after}.png`，
  比对 **md5**；
- 报告写「三张表面 × 两外观 = 6 对，逐字节相同」，以及**为什么这轮没有新图**。
