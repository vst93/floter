# P11-bk 报告 · 空查询页只给最近启动（F4-2）

规格 `docs/rounds/P11-bk-spec.md`。选题：`.pending-bugs.md` 顶部「⚠️ 用户实测反馈
（2026-10-28）」的 **F4**，第 2 条「未输入时不展示内置指令」——它也是 P11-bj 报告 §6
「仍待」的第 1 条（上一轮按参考改过产品行，但因为 21 条 pin 的夹具就是那六行而回退）。

## 1. 根因与落地

**现象**：搜索框为空时，页面先铺六个内置指令（重启/关机/剪贴板/浏览器/计算器/终端），
再铺「最近启动」。用户原话「未输入时不展示内置指令列表」。

**根因（file:line，本轮实测）**：`internal/launcher/search.go` 的 `otherResults()`
空查询分支写着 `out = append(out, a.commands()...)`（本轮改前 `search.go:870`）。参考实现的
空查询分支只 `return recentRows`（`main:src/hooks/useLauncherCatalog.ts:1138-1166`），六条
`SYSTEM_COMMANDS` 只进**有查询时**的打分（`:1201` 起）。所以这是与参考相反的行为，不是
「少了点什么」。

**落地**：

1. 删掉那一行。空查询只剩：计算器答案（查询本身是算式时）、`section:recent` 标题与最近启动
   （`maxRecentResults = 10`，`search.go:513`）。`Results()` 与 `otherResults()` 的注释同步
   改掉「empty query shows the built-in commands」这个已经不成立的答案。
2. 命令**仍然可达**：`Catalog()`（`search.go:690`）照样 `append(a.commands()...)`，输入任何
   字符就走 `matchedCatalog()`；`restart` 由 power 行领头，`ter` 命中 terminal。
   本轮用两条 pin 把这两半钉住（见 §3）。
3. **21 条 pin 的夹具重写**（B 类：改夹具，不改产品）。新增两个共享夹具助手：
   - `internal/launcher/recentpage_test.go`：`recentPage(a, n)`（n 个 `Recent 00…`，带
     `Comment` 所以是两行行盒，与原来的内置行同形状）+ `resultIndex(a, title)`；
   - `internal/shell/recentpage_test.go`：`recentLauncher(a, n)`、`appKeysAppWithRecents()`、
     `recentRowIndex(l, title)`。
   每条 pin 的归类：
   | pin | 旧夹具 | 新夹具 |
   |---|---|---|
   | `TestTheFieldAndTheRowsShareTheReferenceTextColumn` | 空页第一行命令 | 最近启动第一行（同一行组件、同一 54u 文本列） |
   | `TestTheListIsTheCentredReferenceColumn` | 同上 | 同上 |
   | `TestTheRowBlockIsTheInsetRoundedPill` | 同上 | 同上 |
   | `TestDrawnPitchIsTheChargedRow` | 前两行命令 | 前两个最近启动 |
   | `TestTheSelectedRowWearsThePerAppearanceQuietTint` | 前两行命令 | 前两个最近启动 |
   | `TestThePointerTakesTheSelectionOnEntry` / `TestARestingPointerDoesNotFightTheKeyboard` | 第 2/3 行命令 | 第 2/3/4 行最近启动，`Selected` 的期望值改成 `resultIndex(...)`（空页多了一行分组头，硬编码的 0/1/2/3 不再是同一个意思） |
   | `TestTheGroupHeadingSitsOnTheRowsTextColumn` | 空页上还有内置行可比列 | 第二个**输入了单词的帧**里的内置行（空页不再有内置行） |
   | `TestClickingThroughTheScrollEdge` / `TestScrollingTheResultList` | 空页 6 行命令撑起列表 | `SetRecent` 撑起空页 |
   | `TestNoRowScrollsUnderTheFieldBand` | 6 命令 + 10 recents 才够滚 | 输入 `app` 的 30 个匹配（空页最多 10 行，本来就不够滚） |
   | `TestEveryBuiltInRowLeadsWithItsOwnMark` | 读 `Results()` | 读 `Catalog()`（它量的是「每一条内置行」，这一组本来就是 catalog） |
   | `TestThePowerRowsWearTheWarmPlate` / `TestThePowerGlyphsWearTheSystemIconInk` | 空页 | 输入 `o`：同一帧里既有 power 行（`power off`）又有 clipboard 行，两外观全跑 |
   | `TestLauncherShowsTheCatalogWithTheFieldFocused` | 六条命令标题 | 三条最近启动 + 「Recently launched」 |
   | `TestArrowsMoveTheChoiceAndEnterRunsIt` | Down 5 到 terminal | Down 5 到第六个最近启动，Enter 跑那个应用；wrap 回第一行（期望值全部走 `resultIndex`） |
   | `TestClickingARowRunsIt` | 点 Terminal 命令 | 点「Terminal」这个最近启动应用（`app:Terminal`） |
   | `TestLauncherLanguageFollowsTheStore` | 空页读「终端」 | 输入 `terminal` 后读「终端」 |
   | `TestStatusNotePaddingMatchesReference` | `Catalog()[0].Title` | 最近启动第一行 |
   | `TestAppsJoinTheSearchOnceTyped` | 空查询 6 行 | 空查询 0 行（夹具没有 recents），打字后应用才进列表——这正是它量的那件事 |
   | shell: `TestChargedHeightHoldsTheList` / `TestSurfaceSnapshots` / `TestSurfaceSwitchesThemeAndLanguage` / `TestTheSearchColumnIsTheReferenceUnitsAtTheShippedStep` / `TestTheCardReachesTheWindowsSideEdges` / `TestTheCardFillsTheWindowItIsGiven` / `TestTheHeldRowIsTheLastLineOfTheShippedList` / `TestHeldModifierReleaseRestoresThePageDefault` / `TestTheHeldModifierMovesTheSelectionToTheFooterAndBack` | 空页六行 | `appKeysAppWithRecents()` / `recentLauncher(...)`；语言那条改成读中文「最近启动」标题 + 输入 `restart` 后的「重启电脑」 |
   | shell: `TestHazeVeilLetsThePointerThrough` | 点空页上的「Restart the computer」/「Terminal」 | 先输入 `restart` / `terminal` 再点（它量的是「遮罩不吃点击」，行怎么来的与它无关） |
   | shell: `TestTheFrameSheenIsFixedAcrossStops` | y=90..120 那条带被六行命令占着 | 夹具补上 6 个最近启动（见 §5，这条不是回归） |

## 2. 台账（V / B / S）

- **V=0**：没有任何像素值被重定基线；行几何、字段带、卡片、页脚一个数没动。
- **B=1**：空查询的行集变了（产品行为，用户点名要的）。所有 pin 的断言强度不变：
  没有加容差、没有 `t.Skip`、没有删断言；只有**夹具来源**从「空页的六条命令」换成
  「最近启动」，以及由此产生的索引期望改为 `resultIndex(...)`（同一个事实的另一种写法）。
- **S=0**：没有结构变化（行组件、列表、几何链都没动）。

高度链、键盘模型（含 `Super+N` 编号与按住修饰键的常驻行）、会话管理、设置写回、i18n 文案、
命令接线：零改动。

## 3. 新增 pin

- `TestTheEmptyPageShowsOnlyTheRecents`（`internal/launcher/search_test.go`）：空页 ==
  `[section:recent, app Recent00..02]`，且六条命令 ID 一个都不许出现；没有 recents 的空页
  就是 0 行。
- `TestBuiltInCommandsStayReachableByTyping`：`Catalog()` 仍是 6 条；`SetQuery("restart")`
  由 `power:restart` 领头、`power-restart` 紧随；`SetQuery("ter")` 命中 `terminal`。
- 两张证据图（opt-in `FLOTER_PNG=1`）：`TestDumpTheEmptyPageShowsOnlyTheRecents`
  （`/tmp/floter-P11-bk-emptypage-{light,dark}.png`）与
  `TestDumpTheTypedPageFindsTheBuiltInCommand`
  （`/tmp/floter-P11-bk-typed-restart-light.png`）。

**我实测看到了什么（读帧文本树 + 像素）**：空页 720×488 的文本只有
`Launcher results / Recently launched / Recent 00 … Recent 05 / Search / Type to search / Settings`
—— 一条 `Restart the computer` 都没有了（改前同样一次 dump 是六条命令 + 最近启动）；输入
`restart` 的那一帧是 `Restart the computer … / restart Run in shell`，即命令仍在、只是要打字。

## 4. mutation（实跑：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑）

基线 `md5sum internal/launcher/search.go` = `2f96e5b3de937b8ae62718fd4cb79ec6`。

- **M1** 把 `out = append(out, a.commands()...)` 加回空查询分支：
  ```
  --- FAIL: TestTheEmptyPageShowsOnlyTheRecents (0.00s)
      search_test.go:356: the empty page shows [power-restart power-shutdown clipboard browser
      calculator terminal section:recent app:/Applications/Recent00.app …], want the heading and
      the three recents [section:recent app:…]
  ```
- **M2** 把 `commands()` 从 `Catalog()` 里删掉（「只删行就完事」的错解）：
  ```
  --- FAIL: TestCatalogIsAuthoredOrderAndLocalized (0.00s)
      search_test.go:40: catalog has 0 items, want 6
  --- FAIL: TestBuiltInCommandsStayReachableByTyping (0.00s)
      search_test.go:383: the catalog carries 0 rows, want the six built-in commands
  ```
- 两次还原后 `md5sum -c /tmp/bk.md5` 都是 `internal/launcher/search.go: 成功`，`touch` 后
  相关 pin 复跑绿。

## 5. 一处副产物（实测，写清楚免得下轮当回归）

`internal/shell/TestTheFrameSheenIsFixedAcrossStops` 读帧面 y=90..120 那条带的均值，
用来证明「开玻璃比关玻璃更亮」。空页改成 0 行之后，那条带被空态的「No results」占住，
0.04 的微光被淹掉（`off 413.0, frosted 413.0`）。这不是产品回归：它量的是**帧面**微光，
所以夹具里补上 6 个最近启动（和改前一样是一条有行的带）后仍绿，断言一字未改。
在 HEAD worktree 上把同一个 `search.go` 改动单独放进去复现了同一个红，确认归因在产品改动。

## 6. 门槛（本轮实测）

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净
GOOS=windows go vet ./...      # 净
GOOS=darwin  go vet ./...      # 净
go test -count=1 ./...         # 全绿（0 FAIL）
PASS 计数                      # 2448 → 2450
SKIP 计数                      # 46 → 48（+2 = 本轮两条 opt-in 截图 dump）
测试函数数                     # 1994 → 1998（+2 pin +2 dump）
三平台 build                    # linux/windows/darwin 全过
```

（SKIP 基线 46 是本机 HEAD 实测值；任务书里写的 39 是更早环境的数字。）

## 7. 仍待（下轮起点）

1. **空页没有最近启动时显示「No results / Type to search」**（本轮新看见的可见毛病）：
   `recentPage(a,0)` 的空帧文本就是 `No results / Search / Type to search` —— 空查询说
   「没有匹配项」是答错了问题；参考在空查询无 recents 时也没有这句。这是**空态文案 + 触发
   条件**，属界面质感档、不需要拍板（改 i18n 文案要小心：`NoResults` 同时被无匹配查询用）。
2. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / `P11-an` 的便条投影 / `P11-be` 的 `Permissions` 行 /
   `P11-bf` 的 `configDiscardBar` 12u margin / `P11-bg` 的顶边失败带覆盖画布顶部两行。
3. `assignOutputNumbers`（`output.go:826`）仍是 `OutputList.Visible()` + 闸门（P11-bi 记下的
   同族不同表面）。
