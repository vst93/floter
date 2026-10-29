# P11-cn 报告：本帧的动作读本帧那张表（键盘 / 星标键 / 星标点击 / 删除键）

轮号 `P11-cn`。规格 `docs/rounds/P11-cn-spec.md`。HEAD `142e8e82`（P11-cm）。

## 1. 选题

用户实测反馈 F1–F4 已收尾 → 合约 §6 全 `已落地`（只剩拍板项）→ P11-cm「仍待」第 1 条
整张是 15 条拍板项、第 2/3 条是「别重领」的记账、第 4 条是尺子盲点 → 按用户定的
「界面质感与交互（含用户报的 bug）」自选。

挑中的是 P11-cl 那一族**没修完的部分**：`View` 每帧只走一遍目录是一条已经写在代码里的
规矩（`App.searchRuns` + `TestADrawWalksTheCatalogOnce`），P11-cl 只把**指针点击行**
那一支搬上了本帧的表（`App.drawn`）。本轮实测发现同族还剩三支。

## 2. 现象与根因（本轮实测）

探针（临时文件，已删）在 `720×488`、剪贴板模式两行上逐帧数 `a.searchRuns`：

```
arrow down         draws=61 over1=1  [2 1 1 1 …]      ← 第 1 个 draw 是 2
arrow up           draws=61 over1=1  [2 1 1 1 …]
cmd-D favorite     draws=80 over1=1  [2 1 1 1 …]
cmd-Backspace arm  draws=80 over1=0  [1 1 1 1 …]      ← 代价不在 draw 里
star click         draws=62 over1=1  [1 … 1 2 1 …]    ← 第 22 个 draw 是 2
typed Tab          draws=61 over1=0
typed cmd-1        draws=5  over1=0
```

四处**每一帧的第二遍全量匹配**，都发生在用户按一下键 / 点一下星的那一帧：

| 位置（HEAD `142e8e82`） | 现状 | 触发的键 |
|---|---|---|
| `launcher.go:2837` `move` | `a.nextSelectable(...)` → `a.Results()`（`:2856`） | ↓ / ↑ |
| `launcher.go:1790` `modeShortcuts` | `toggleClipFavorite()` → `selectedClip()` → `a.Results()`（`:1882`） | ⌘D |
| `launcher.go:2613` 行尾星标 `Clicked` | `toggleFavorite(item)` → `toggleClipFavorite()` → `a.Results()` | 点星 |
| `fieldkeys.go:350` `deleteHistoryRow` | `results := a.Results()` 再按 id 找一遍 | ⌘⌫ 的确认那一下 |

**P11-cl 为什么没抓住键盘那一支**：它把 `nextSelectableIn`（吃一份现成切片）改对了，但
`move` 走的是 `a.nextSelectable`——那个包装函数**自己**又 `a.Results()` 一遍。所以键盘
那一支从来没有真的改到，而 P11-cl 的计数 pin 只加了「点一下行」一臂，看不见它。

**删除键那一支的代价不在 draw 里**，在**键事件**上：实测同一次按键
`press: total=40 perDraw=[1 ×40]` —— 40 次全量匹配，40 个 draw 各 1 次，第二遍藏在
事件处理里。所以「每遍 draw 只搜 1 次」这条判据天生看不见它，pin 必须**同时**读
「这一次按键一共搜了几次」和「这一帧一共画了几次」。

## 3. 落地

### 3.1 产品代码（4 个文件）

1. **`launcher.go` · `move`**：`a.nextSelectable(...)` → `nextSelectableIn(a.drawn, ...)`。
   包装方法 `nextSelectable` 失去唯一的生产调用点，**删掉**（S：它的测试调用点改读
   `nextSelectableIn`，断言一字不改）。
2. **`launcher.go` · `modeShortcuts`**：⌘D 先取 `a.selectedHistoryRow()`（本帧的表），
   再把那个 `Item` 交给 `toggleCalculatorFavorite(item)` / `toggleClipFavorite(*item.clip)`。
3. **`launcher.go` · `toggleFavorite`**：改成把**这一行自己的 `Item`** 交下去；行尾星标的
   点击那一支于是**一次目录都不再走**（它本来就在 draw 里）。
   `selectedClip` 读 `a.drawn` 而不是 `a.Results()`。
4. **`fieldkeys.go` · `selectedHistoryRow`**：读 `a.drawn`。
5. **`fieldkeys.go` · `deleteHistoryRow`**：签名从 `(id string)` 改成 `(item Item)`，
   索引用 `a.drawn` 里那个 id 的位置。**arm 仍然是 id**——确认那一下重新读本帧的表、
   只按 id 匹配，所以「列表在 arm 底下变了」的保护一个字节没动。
6. **`calculator.go` · `toggleCalculatorFavorite(item Item)`**：同上，不再自己走目录。

**行为契约零改动**：选择落点 / 指针交接 / 行几何 / 动作执行 / 编号 / 高度链 / 键盘模型
（哪些键做什么）/ 会话管理 / 设置写回 / i18n / 命令接线——一个字节没动。改的只是
**动作从哪张表里取那一行**。**零像素改动。**

### 3.2 台账

- **V = 0**：没有任何视觉数值变化。
- **B = 0**：既有行为断言零改动全绿（`TestHistoryRowStar`、`TestTheHistoryStarAnswersThroughTheField`、
  `TestHistoryDeleteKeyDeletesACalculation` 等）。
- **S = 1**：`App.nextSelectable` 方法删除（唯一生产调用点搬到 `nextSelectableIn`）；
  `filters_test.go:136` 由 `a.nextSelectable(...)` 改成 `nextSelectableIn(results, ...)`，
  断言（箭头不落在分组头）一字不改。`deleteHistoryRow`/`toggleClipFavorite`/
  `toggleCalculatorFavorite` 的签名变宽（多一个参数），调用点全在包内。

### 3.3 一处刻意的语义收窄（写在这里，因为它不是零改动）

星标键 / 删除键 / 方向键现在读**上一帧画出来的那张表**，而不是「按下那一刻重新匹配
出来的表」。两表在正常使用下逐条相同——本轮用探针比过 12 帧 × 8 种状态（空页 / 有查询 /
无命中 / 修饰键按下与松开 / 模式里），`frame == fresh` 一次不差。唯一会不同的是
**剪贴板监听在两帧之间插入新条目**：那时「用户看着的那一行」才是对的答案，这也正是
删除族早已定下的「按 id 而不是按索引」的同一条规矩。

## 4. 测试（`internal/launcher/drawtable_test.go`，新增 4 条 pin）

`drawSearchOnce` 一把尺子读**两个半边**，四支各钉一臂：

| pin | 行为半边 | 代价半边 |
|---|---|---|
| `TestTheArrowsReadTheTableTheDrawBuilt`（↓ / ↑ 两个子测试） | 落点 == `nextSelectableIn(a.drawn, before, delta)`，且不落在分组头 | 每遍 draw 搜 1 次，且整次按键的搜索数 == draw 数 |
| `TestTheStarsChordReadsTheTableTheDrawBuilt` | 星标落在**屏幕上那一行**（entry id 不变、store 里那个 id 变成 favorite） | 同上 |
| `TestTheStarsClickReadsTheTableTheDrawBuilt` | 点击星标星的是**那一行的** entry | 同上 |
| `TestTheDeleteKeyReadsTheTableTheDrawBuilt` | arm 的 id == 屏幕上那一行的 id；确认之后那行真的没了、别的不动 | 同上（这一臂是本轮唯一能抓住「代价藏在事件里」的） |

**为什么两个半边都要读**：只读落点，第二遍全量匹配照样在；只读次数，跑错行也照样绿。
P11-cl 的教训正是前者——它的 pin 只读了点击那一支的代价。

## 5. 门槛（实测）

基线在**干净工作树**（`git worktree add /tmp/floter-base HEAD` = `142e8e82`）上量的，
不是抄旧报告：

| 门槛 | 基线（HEAD `142e8e82`） | 本轮 |
|---|---|---|
| `gofmt -l cmd internal` | 空 | **空** |
| `go vet ./...` | 净 | **净** |
| `GOOS=windows go vet ./...` | 净 | **净** |
| `GOOS=darwin go vet ./...` | 净 | **净** |
| `go test -count=1 ./...` | 0 FAIL | **0 FAIL** |
| `-v \| grep -c -- "--- PASS"` | **2770** | **2776**（+6） |
| SKIP | 56 | **56**（不变，全是 opt-in PNG dump） |
| 测试函数（`internal/` + `cmd/`） | **2056** | **2060**（+4，`drawtable_test.go`） |
| 三平台 `go build ./...` | 过 | **过** |

`TestTheArrowsReadTheTableTheDrawBuilt` 有 `down`/`up` 两个子测试，所以 +4 个函数
变成 +6 条 PASS。

## 6. 变异（3 条，全部实跑变红并逐字节还原）

### M1 · `move` 改回 `a.Results()`（键盘那一支的旧写法）

```
=== RUN   TestTheArrowsReadTheTableTheDrawBuilt/down
    drawtable_test.go:67: the down arrow: the frame's draw 1 searched 2 times, want 1 — the action walked
        the catalog again inside the draw it had already built (see App.drawn)
=== RUN   TestTheArrowsReadTheTableTheDrawBuilt/up
    drawtable_test.go:67: the up arrow: the frame's draw 1 searched 2 times, want 1 — …
--- FAIL: TestTheArrowsReadTheTableTheDrawBuilt (0.09s)
```

还原后 `md5sum -c /tmp/p11cn.md5` 三个文件全部「成功」，`touch` 复跑绿。

### M2 · `selectedHistoryRow` 改回 `a.Results()`

```
=== RUN   TestTheStarsChordReadsTheTableTheDrawBuilt
    drawtable_test.go:111: the star's chord: the frame's draw 1 searched 2 times, want 1 — …
--- FAIL: TestTheStarsChordReadsTheTableTheDrawBuilt (0.09s)
=== RUN   TestTheDeleteKeyReadsTheTableTheDrawBuilt
    drawtable_test.go:188: the delete key: the press walked the catalog 62 times over 60 draws — 2 of those
        happened on the key event, for a row the draw had already built
    drawtable_test.go:194: the delete key's confirmation: the press walked the catalog 62 times over 60 draws — 2 of those …
--- FAIL: TestTheDeleteKeyReadsTheTableTheDrawBuilt (0.03s)
```

注意**两半都被抓住**：`the star's chord` 红在「draw 里搜了 2 次」那一半，
`the delete key` 红在「按键一共搜了 62 次、只画了 60 次」那一半。这是两条不同的判据，
同一次注入同时踩中，正好说明两半都必要。

### M3 · 行尾星标点击改回绘制期重取（`a.toggleFavorite(a.Results()[i])`）

```
=== RUN   TestTheStarsClickReadsTheTableTheDrawBuilt
    drawtable_test.go:155: the star's click: the frame's draw 22 searched 2 times, want 1 — …
--- FAIL: TestTheStarsClickReadsTheTableTheDrawBuilt (0.08s)
```

第 22 个 draw 是点击那一帧的那一次——与 P11-cl 的 `[1 … 1 2 1 …]` 同一个位置。

三条全部 `md5sum -c` 逐字节还原 + `touch` 复跑绿。

## 7. 视觉证据（**零像素**，不是没出图）

本轮不改墨与面，所以证据是**两张帧逐字节相同**，而不是「看起来差不多」。
同一个 dump（`720×488`、剪贴板模式、按一下 ↓ 之后的帧），HEAD 干净工作树与本轮各出一张：

```
=== dark
7e39cc0b92ca8e3d265f4883601f5e4f  p11cn-base/p11cn-dark.png
7e39cc0b92ca8e3d265f4883601f5e4f  p11cn-mine/p11cn-dark.png
=== light
7de47e34482ffbdbfec659508cd8e70b  p11cn-base/p11cn-light.png
7de47e34482ffbdbfec659508cd8e70b  p11cn-mine/p11cn-light.png
```

逐像素扫（PIL）：`dark` 与 `light` 两套外观**各 0 个差异像素**。图上我看的是什么：
剪贴板模式两行、选中行（`first clip`）的 pill 与它行尾那颗星——和修前**一模一样**，
因为改的只是「这颗星属于哪一行从哪张表里取」，不是它画在哪。

`/tmp/floter-P11-cn-clipboard-{dark,light}.png` = `p11cn-mine/p11cn-{dark,light}.png`。

## 8. 实测代价（每一次按键 + 它那一帧一共走几遍目录）

同一个探针在干净基线与本轮各跑一次（`720×488`、剪贴板模式两行）：

| 动作 | 基线 | 本轮 |
|---|---|---|
| ↓ 方向键 | 62 | **61** |
| ⌘D 星标键 | 61 | **60** |
| ⌘⌫ 删除 arm | 62 | **60**（−2） |
| 点一下行尾星标 | 63 | **62** |

基线里多出来的那一遍，就是本轮删掉的那一次全量匹配（`Catalog()` 重建 + 每条打分 +
拼音）。目录越大，这一遍越贵——它发生在用户按下去的那一帧里。

## 9. 仍待（P11-cn）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
2. **`App.Results()` 的生产调用点只剩 4 处**，且都不是本族：
   `launcher.go:1101`（`SetQuery` 深链）、`:1514`（字段 `Changed`，刚吃进一个键）、
   `:3250`（`syncHeldModifier` 的「本帧那一遍」）——三处都是「在 draw 之外、按用户
   刚给的新输入算一次默认选择」，本来就是新输入算新表；第四处
   `:2899` 的 `firstSelectable` **没有调用点（死代码）**，本轮不擅自删（它不是本族，
   删它也不改变任何行为，留给下一轮顺手清）。
   `calculator.go` / `fieldkeys.go` 里本族的四处已全部搬上本帧的表。
3. **`terminalui` 的 `Term.Dir()`（OSC 7）与 `Term.Size()`** 每帧各问一次插件，
   是同一个形状（「每帧问一次已经知道的东西」）但**不是全量匹配**，代价量级不同——
   别当同族重领，要先量。
4. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器裁掉的行不进
   `Texts()` / PTY 自己画的行不在高度尺子上）。

