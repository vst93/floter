# P11-cr 报告：19 个没人调用的函数

## 1. 选它做什么

选题顺序第 3 档「上一轮仍待第一条」。`P11-cq` 的「仍待」第 1 条就是这张死代码清单，
而且它被 **P11-cn / P11-co / P11-cp / P11-cq 连续四轮**记为「不是任何一轮的族，建议
单独一轮清」。用户实测 F1–F4 全部收尾、合约 §6 全部 `已落地`（只剩拍板项），所以这一档
就轮到它。

它不是视觉轮、不是拍板项、不碰任何行为契约——正是「一轮只做一件事」的独立切片。
规格 `docs/rounds/P11-cr-spec.md`。

## 2. 开工前的两件事（都核过）

### 2.1 抢救 stash：两条都已被后续轮次覆盖

任务书顶部挂着两条「待回收」：

| stash | 判定 | 证据 |
|---|---|---|
| `d2458db04b`（2026-10-28） | **已回收**（P11-ae 记过） | `git stash list` 里没有它（在游离提交上），P11-ar 已复核无可接续 |
| `9aff92d6db`（2026-10-30 05:37） | **已回收**（P11-co 记过） | 本轮复核：`git diff HEAD 9aff92d6db` 里**没有一个文件只在 orphan 一侧**（`comm -13` 空）；它相对 HEAD 的「新增」全部是**反向的删除**——即它的树是 P11-co/cp/cq 之前的状态，后三轮把它的未跟踪载荷 `probe6_test.go` 收成了 `terminalui/dragwrites_test.go`。`git ls-tree` 比对：两棵树只差 HEAD 多出的 16 个文件（P11-co/cp/cq 的产物），orphan 没有一件 HEAD 缺的东西 |

另外 `stash@{0}`（"P11-cq work"，2026-10-30 07:40）也是同一件事：`git diff 'stash@{0}' HEAD`
显示**源文件完全相同**，差的只有 P11-cq 自己的 spec/report/dump/测量件（都已进 HEAD）。
**两条都不再试合**，本体保留不删。

### 2.2 基线（干净 worktree 实测）

```
PASS 2799 · FAIL 0 · SKIP 58 · 测试函数 2079
gofmt -l cmd internal → 空 · go vet ./... → 净 · 三平台 build → 过
```

## 3. 根因：不是「没人用的代码」那么简单

清单本身是 `deadcode` 的读数，但**逐条读实现**之后，19 条分成三类，每一类的「为什么
它在那里」不一样：

- **A 类（转发壳）**：`inventory.LookPath`（一行转发 `pathlist.Lookup`）、
  `spawn.terminalEmulators`（一行转发 `terminalEmulatorsFor`）、
  `settingsui.rowLines`/`rowTop`（一行转发 `rowAligned`）、
  `settings.LoadDefault`（`Path()` + `Load()` 的两行组合）。
  它们是「有一个更具体的名字」的旧 API 面，调用方后来直接读了底层那个。
- **B 类（被合并掉的旧渲染路径）**：`settingsui.text`（一行 `row` 包一个 textInput）、
  `settingsui.freshnessDetail`（拼一整句 detail）。这两条**曾经真的画过东西**，
  后来 `configform.go` 的表单渲染与 `freshnessSection` 的寄存器化渲染接手了。
  `freshnessDetail` 的注释自己写着「kept for the panel's own readers (the shell's
  tests and diagnostics)」——**那些读者不存在**（`grep` 零命中）。
- **C 类（从没接上的入口）**：`extensions.PreparedCustom.Discard`（「用户放弃连接」
  的清理入口）、`launcher.firstSelectable`（空页默认选中）、`terminalui.App.Label`
  （会话名，外壳读的是 `LabelOf(index)`）、`usage.Store.FilePath`、
  `extensions.Store.Paths`、`RunErrorKeys`（注释说「for the i18n symmetry test」，
  那条测试也不存在）。**这一类最值得记**：它们是「设计里写了、产品里没接」的接口。

### 3.1 一条真正的连带发现（不是死代码，是死文案）

删 `freshnessDetail` 之后 `go test ./...` **红了一条我没预期的**：

```
--- FAIL: TestNoDeadCopy (3.31s)
    deadcopy_test.go:36: i18n.Copy.FreshnessProbeSource is declared but never read outside internal/i18n
```

`FreshnessProbeSource`（"from the last re-scan" / "来自上次重新扫描"）的**唯一**读者
就是 `freshnessDetail`。真正画 freshess 的是 `freshnessSection`，它读的是
`FreshnessHealthSource`（健康报告那个来源）与 `FreshnessCommandsUnknown`——
「上次重新扫描」这个来源在**现在的界面上根本不存在**（`freshnessblock_test.go`
的 `TestTheFreshnessNeverProbedWordIsItsOwnNode` 也证明只有 Never / health 两种来源）。

仓库自己的 `TestNoDeadCopy` 判据就是「一个 key 只在本包内被读 = 死文案」，所以这条
按同一条规则处置：**删掉 `FreshnessProbeSource` 字段与两套译文**。这是**规格外的一步**，
如实记在这里——它是本轮删除的**二阶后果**，不是我原本要做的第二件事。

## 4. 落地（逐条）

### 4.1 生产代码：删 18 个函数 + 2 处孤立 import

| 文件 | 删掉 | 连带 |
|---|---|---|
| `internal/calculator/calculator.go` | `SortNewestFirst` | 无（`:415` 自己那份 `sort.SliceStable` 仍在，`sort` import 保住） |
| `internal/extensions/custom.go` | `PreparedCustom.Discard` | 无（`Commit` 的失败分支自己调 `p.cleanup()`，那才是唯一需要清包的时刻） |
| `internal/extensions/install.go` | `Prepared.Version` | `Name` **留下**（`shell.go:878` / `:2440` 真的在读它）；`Name and Version` 的注释改成 `Name` |
| `internal/extensions/runfailure.go` | `RunErrorKeys` | 无（闭集本身仍由 `KnownRunErrorKey` 读） |
| `internal/extensions/store.go` | `Store.Paths` | 无（`s.paths` 包内 3 处直读） |
| `internal/inventory/sources.go` | `LookPath` | **删 `pathlist` import**（它成为孤立） |
| `internal/launcher/launcher.go` | `App.firstSelectable` | 无。**它的规则没有丢**：`clampSelection` 早就在做同一件事（`Selected` 落在不可选行时 `nextSelectableIn` 往前挪一格） |
| `internal/settings/settings.go` | `LoadDefault` | 无（`Path()` 仍被 `migrate.go:58` 与 `windowstore.go` 用） |
| `internal/settingsui/rows.go` | `rowLines`、`rowTop` | 无（`rowAligned` 是真正实现，`iconRow` 等仍在调） |
| `internal/settingsui/settingsui.go` | `SelectedPage`、`text`、`freshnessDetail` | 见 §3.1 的连带 |
| `internal/shell/system.go` | `numeric` | **删 `encoding/json` import**（本轮实测该文件只有 `:421` 那一处用 `json.`） |
| `internal/spawn/spawn.go` | `Terminal`、`terminalEmulators` | `runtime` **保留**（`:40/:100/:150` 仍在用） |
| `internal/terminalui/terminalui.go` | `App.Label` | 无。外壳读的是 `LabelOf(index)` |
| `internal/usage/usage.go` | `Store.FilePath` | 无（`s.path` 包内 3 处直读） |

### 4.2 测试文件里的 6 个孤儿 helper

`internal/launcher/filters_test.go` `hasItem`、`internal/launcher/rowtrailing_test.go`
`strongestInk`/`channelDiff`、`internal/settingsui/raycast_test.go` `renderThemed`、
`internal/shell/detachlist_test.go` `detachedTokens`/`launcherColor`。

删它们**不影响 PASS 计数**（不是 `Test*`），但各自带走一处 import：
`internal/shell/detachlist_test.go` 的 `image/color` 与 `floter/internal/theme` 因此
孤立，一并删掉（`go vet` 报的正是 `"image/color" imported and not used`）。

**`renderThemed` 删前把它的知识搬走了**：它带着「浅色半边一直是白，是 pin 的稳定
读数面」这条判据，而 `renderIn` 的注释里已经有同一句话（`dark` 是画帧**前**设的，
因为那是视图自己解析用的那个 flag），所以那段话不丢。

### 4.3 一条新的 pin（两条测试函数）

`internal/ci/deadcode_contract_test.go` + 账本 `internal/ci/deadcode_baseline.txt`（24 行）：

- `TestTheDeadCodeBaselineStaysDeleted`：账本里每个 `(路径, 符号)` 在**它原来那个包**
  的所有 `*.go` 里都不许再有 `func` 定义（正则兼容 `func Name(` 与 `func (p T) Name(`
  两种接收者形状）；**并且账本条目数必须恰好 24**——加一条要有自己的 `deadcode` 测量，
  删一条是「让 pin 绕过自己」的标准手法。
- `TestTheDeadCodeFalsePositiveStaysOut`：唯一留下那条 `deadcode` 读数
  （`shell` 的 `dressWindowsFrame`）**不许进账本**，且它的 `!windows` 桩与
  `chrome_windows.go` 的调用点都必须还在。

**为什么 pin 是源码扫描而不是跑 `deadcode`**：那个工具是 `go run …@version`（要联网），
且**三平台跑不了**——本轮实测 `GOOS=windows` 下它自己 `fork/exec …deadcode.exe:
exec format error`。所以 CI 形状的 pin 不能调它；工具量出来的东西写成本轮账本，
pin 守的是真正会烂的那件事：**谁把孤儿函数加回来**。

## 5. 台账（V / B / S）

- **V = 0**：一个墨、半径、字号、padding、gap 都没动。
- **B = 0**：既有 pin **零改动**全绿。删除后跑 `go test -count=1 ./...` 唯一红的是
  `TestNoDeadCopy`——那是**产品回归的镜子**（死文案），改的是产品（删 `FreshnessProbeSource`），
  不是测试。
- **S = 0**：没有一条断言因为结构变化而改写。
- **新增 2 条 `Test*`**（§4.3），测试函数 2079 → **2081**。

## 6. 零像素（本轮的「出图」）

这轮不是视觉轮，所以证据是**前后逐字节相同**，不是「我看到了什么变化」：

| 表面 | 外观 | 修前 md5 | 修后 |
|---|---|---|---|
| settingsui（设置页） | dark | `e9b3dbde920c94f9` | **逐字节相同** |
| settingsui（设置页） | light | `0f0b8229f7aa3e1b` | **逐字节相同** |
| launcher（空页） | dark | `855b85928a116bfd` | **逐字节相同** |
| launcher（空页） | light | `ed23f1d3983170e9` | **逐字节相同** |
| terminalui（抽屉） | dark | `4df49b69e82b0281` | **逐字节相同** |
| terminalui（抽屉） | light | `6ce1dd47cf72d54d` | **逐字节相同** |

实测命令（修前图存 `/tmp/cr-before/`，修后图跑同一批 dump 测试存 `/tmp/cr-after/`）：

```
for f in /tmp/cr-before/*.png; do cmp -s "$f" "/tmp/cr-after/$(basename $f)" \
  && echo IDENTICAL || echo DIFFERS; done
```

**6 对全部 IDENTICAL**（输出见 §7 上方）。
**为什么这轮没有新图**：18 条删除里没有一条在绘制路径上（§3 的三类都不是画东西的代码），
所以正确的读数是「像素一个没变」，而不是「换了个样子」。

## 7. 门槛（实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
GOOS=linux|windows|darwin go build ./...  → 三平台过
go test -count=1 ./...                    → 全绿（exit 0）
--- PASS  2799 → 2801
--- FAIL  0
--- SKIP  58（不变）
测试函数  2079 → 2081（+2，§4.3）
```

**一条既有 flake（如实记，不是本轮引入）**：`-v` 全量跑时
`internal/shell` 的 `TestDeepLinkConnectAndRegister` 报过
`TempDir RemoveAll cleanup: unlinkat … directory not empty` —— 那是 TempDir 清理
与后台 goroutine 的竞态。本轮在**干净的 HEAD** 上单跑 6 次全绿，在改动后的树上再单跑
5 次也全绿；`go test -count=1 ./...`（不带 `-v`）两次都是 exit 0。与死代码删除无关
（它走的是 extensions 连接路径，本轮没碰那条路径）。

## 8. 变异（5 条实跑，全部还原并逐字节核对）

| # | 注入 | 报出的原文 | 判定 |
|---|---|---|---|
| **M1** | 把 `shell/system.go` 的 `numeric` 加回去 | `../../internal/shell/system.go:1015: numeric is back — the ledger says it has no caller …` | 红 |
| **M2** | 账本**加**一行不存在的符号 | `the dead-code ledger lists 25 entries, this round wrote 24: adding a claim needs its own deadcode measurement …` | 红 |
| **M3** | 账本**删**一行（`usage/usage.go FilePath`） | `the dead-code ledger lists 23 entries, this round wrote 24: … removing one is how the pin would get around itself` | 红 |
| **M4** | 把 `terminalui.App.Label` 加回去 | `../../internal/terminalui/terminalui.go:691: Label is back …` | 红 |
| **M5** | 删掉 `chrome_other.go` 的 `dressWindowsFrame` 桩 | `../shell/chrome_other.go dropped the !windows stub of dressWindowsFrame: the windows arm (chrome_windows.go) calls it …` | 红（**这条是反面 pin 的价值**：它证明「别删这条假阳性」不是一句注释，是一条会咬人的断言） |

每条：注入 → 跑 → 抄报错 → 还原 → `md5sum -c` 逐字节核对 → `touch` 再复跑一次绿。
M2/M3 是**同一条 pin 的两个方向**（放宽与收窄），都实跑过——只测一个方向的话，
「把账本清空」这条路仍然通。

## 9. 复跑 `deadcode`：25 → 1

删除后本轮重跑 `deadcode -test ./...`，输出从 25 行降到**1 行**：

```
internal/shell/chrome_other.go:18:6: unreachable func: dressWindowsFrame
```

剩下那条是**假阳性**（`!windows` 桩 + 工具在 Linux 上编译看不到 windows 那支），
已写进账本底部的注 + §4.3 的反面 pin。

## 10. 仍待（P11-cr）

1. **老拍板项原样有效**（见 `.pending-bugs.md` 的「需用户拍板」表 + 搜索框 caret /
   默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 / `AccentEdge` 3:1 / P11-an 便条投影 /
   P11-bd 拖拽手柄 / P11-be `Permissions` 用词 / P11-bf `configDiscardBar` 12u /
   P11-bg 顶边失败带 / P11-bo 空页置入终端行 / P11-bv `AppIcons[].Label` /
   P11-bw 十行预算里分组头吃一行 / P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
2. **`deadcode` 的假阳性只能靠人判**：`GOOS=windows` 下工具自己跑不起来
   （`exec format error`），所以「windows 专属分支里的桩」这一类**永远**要靠读代码。
   下一轮若再跑 `deadcode`，看到 `chrome_other.go` 那条**别再当新发现**。
3. `settingsDebounce` 仍是两份拷贝（`settingsui` 与 `terminalui` 各自一份，同一个参考
   常数，两个包互不 import）。第三处需要时才抽包。
4. `terminalui` 的 `Term.Dir()`（OSC 7）/ `Term.Size()` 每帧各问一次插件：形状相同
   但**不是全量匹配**，别当同族重领，要先量。
5. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器裁掉的行不进
   `Texts()` / PTY 自己画的行不在高度尺子上）。
6. **`shell.go` 里还有 40+ 处 `a.Store.Snapshot()`**，绝大多数是**事件期**读点，
   按 P11-co 的规矩故意保留。下一轮若要再收，必须逐处判「绘制期还是事件期」。
7. **`TestDeepLinkConnectAndRegister` 的 TempDir 清理竞态**（§7）是一个**既有的**
   flake，本轮没修（不属于死代码族）。要不要专门收一轮，看它复现频率。
