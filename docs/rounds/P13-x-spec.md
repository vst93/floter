# P13-x 规格：分组头也参加折边——折边切到标签时，标签退后而不是被切

`docs/liquid-glass.md` §6 没有 `未开始` 行（P13-g 收口），按纪律走通用队列。
本轮取 **P13-w 报告「仍待」第 1 条**（也是 P13-v 留下的同族最后一块）：
启动器列表与捕获输出页的**分组头**从来不经过 `foldAlpha`。

同时回收抢救 stash：`stash@{0}`/`stash@{1}`（两条逐字节相同，
`md5sum /tmp/s0.patch /tmp/s1.patch` 相同，parent = `a95274b7` = 当前 HEAD）就是这一族工作。

## 意图

**折边切到分组头的时候，标签应该和它管辖的行一起退后，而不是保持满墨被水平切开。**

分组头是列表的一部分（它占一行的高度、随列表一起滚动），但它在代码里是
`row`/`outputRow` 的**兄弟**元素，不是行本体：两个函数都在建行之前 `return`：

- `internal/launcher/launcher.go:2250` `if item.heading { … return }`
- `internal/launcher/output.go:590` 的 `row.Group` 臂（建完标签继续往下建行，但标签本身
  不是下面那个 `style`）

`rowBody`（`launcher.go:2371-2383`）与 `outputRow` 的 `style` 各自下发一次折边墨，
所以标签从来拿不到。P13-v/P13-w 两轮把这条规则钉在行上，标签一直漏在外面。

## 现状（实测，`a95274b7`，720 宽帧）

### 启动器列表（"Recently launched"，32u 盒子）

键盘往下走，列表把标签推过视口上沿 y=60：

| 折叠比例 | 改前 `contrastIn` | 改后 |
|---|---|---|
| 1.000（整行可见，浅色） | 145 | 145（一个像素没动） |
| 0.750（浅色） | **145** | 108 |
| 0.438（浅色） | **145** | 63 |
| 0.375（浅色） | **145** | 54 |
| 0.250（浅色） | **143** | 36 |
| 1.000（整行可见，深色） | 133 | 133 |
| 0.375（深色） | **133** | 50 |
| 0.250（深色） | **133** | 33 |

改前那一列是同一组像素：同一个 8px 带，露出 1/4 和露出 3/4 的标签**逐像素相同**。
它不表达「下面还有」，它只表达「界面坏了」——一个被水平切开的汉字。

### 捕获输出页（`Group B`，32u 盒子）

| 折叠比例 | 改前（浅色） | 改后（浅色） | 改前（深色） | 改后（深色） |
|---|---|---|---|---|
| 1.000 | 137 | 137 | 141 | 141 |
| 0.938 | — | 129 | — | 132 |
| 0.688 | **137** | 94 | **141** | 97 |
| 0.438 | **134** | 59 | **138** | 60 |

改前同一条读数：`Group B` 在只剩 8/22u 时墨是 134，整行可见也是 137——差 3 个灰阶，
等于没差。

## 落地（逐条）

### 1. `headingRects`（`internal/launcher/launcher.go`，`App` 结构体）

- 位置：`rowRects/listRect` 旁边（`launcher.go:440` 附近）。
- 现状：没有这张表。
- 应改成：新增 `headingRects map[int]ui.Rect`，注释写死两件事——
  (a) 它**只喂 `foldAlpha`**，绝不喂 `viewportRange`/`numberedViewport`；
  (b) 与 `rowRects` 同一时序（`Bounds()` 是上一帧的矩形）。

### 2. `outputHeadingRects`（同文件，`launcher.go:452` 附近）

捕获输出页的同一张表。同样的排除规则（它喂 `foldAlpha`，不喂
`outputNumberedViewport`）。

### 3. 两张表的清空时机（与行盒同一个时机）

- `launcher.go:1403` `clear(a.rowRects)` 之后 → `clear(a.headingRects)`
- `output.go:122`（`showOutput`）与 `output.go:548`（`outputBody`）的
  `clear(a.outputRowRects)` 之后 → `clear(a.outputHeadingRects)`

理由：两张表都只描述**本帧**建出来的东西；一个离开列表的标签不能留下过期盒子。

### 4. `row` 的分组头臂（`launcher.go:2250-2270`）

- 现状：`ui.Row(c).FillWidth().Padding(…)` 直接建，无墨。
- 应改成：把元素存进 `heading` 变量 → 记盒子 → `if fold := foldAlpha(a.listRect, box); fold < 1 { heading.Opacity(float32(fold)) }` → 再 `Children`。
- **`Opacity` 必须在 `Children` 之前**：框架按建树那一刻的 opacity 画这一帧。

### 5. `outputRow` 的分组头臂（`output.go:611`）

同一个规则、同一个纯函数（`foldAlpha`），只是盒子记进 `outputHeadingRects`、
视口读 `outputListRect`。

### 行为契约零改动红线（全部不碰）

- 高度链（`rowUnits`/`headingPadding`/`Geometry` 的 band 计算）
- 键盘模型（编号家族、`clampSelection`、`digit chord`）
- 会话管理、设置写回、i18n 文案、命令接线

**关键**：`rowRects`/`outputRowRects` 的**键集合不变**——分组头进的是另外两张表，
所以 `viewportRange` 数出来的编号范围一个数字都不动（实测 `numbers` 仍是
`map[1:1 … 9:9]`，`rowRects` 里没有 heading 的键，`headingRects` 里没有行的键）。

## 门槛命令 + PASS 基线（实测当前值）

| 命令 | 基线（P13-w 收尾） | 本轮要求 |
|---|---|---|
| `gofmt -l cmd internal` | 空 | 空 |
| `go vet ./...` | 净 | 净 |
| `GOOS=windows go vet ./...` | 净 | 净 |
| `GOOS=darwin go vet ./...` | 净 | 净 |
| `go test -count=1 ./...` | 全绿 | 全绿 |
| `go test -count=1 -v ./... \| grep -c -- "--- PASS"` | 2988 | ≥ 2988（只许涨） |
| 测试函数数量 | 2207 | ≥ 2207 |
| SKIP | 80 | ≤ 80 | 
| 三平台 `go build ./...` | 过 | 过 |

## 新增 pin（`internal/launcher/headingfold_test.go`）

复用量 `outputfold_test.go` 已有的 `contrastIn`/`faceIn`/`visibleShare`（不另写一份），
加一个 `headingFoldEvidence` 助手直接提供 (盒, 折边, 图) 三元组：

1. `TestTheLauncherHeadingAtTheFoldRecedes`：同一个 `Recently launched`
   在 488 高帧（整行可见）与 300 高帧（推过折边）两帧的墨；浅色 + 深色两种外观。
2. `TestTheOutputGroupHeadingAtTheFoldRecedes`：同一个 `Group B` 在 440 高帧与 404
   高帧两帧的墨，浅色 + 深色。
3. `TestTheLauncherHeadingsInsideTheViewportKeepTheirInk`：整行可见的标签
   `contrast ≥ 100`（反向 pin：折边只退被切的那一个）。
4. `TestTheHeadingBoxesStayOutOfTheNumberedFamily`（**B 类结构 pin**）：
   `headingRects` 的键与 `rowRects` 的键不相交，且 `numbers` 的键集合与
   P13-v 之前一致（分组头没有占掉数字位）。这条是「不许把标签塞进 `rowRects`」的
   机器判据。
5. `TestDumpP13XHeadingFoldPNGs`（opt-in `FLOTER_PNG=1`）：深/浅两套出图。

## mutation 计划（≥2 条，实跑 + `md5sum -c` 还原）

| # | 注入 | 应让哪条 pin 变红 |
|---|---|---|
| M1 | 分组头臂不下发 `Opacity`（删掉那个 `if`） | 1、2 |
| M2 | `foldAlpha` 在两个分组头臂里恒返回 1（把参数写死成空视口） | 1、2 |
| M3 | 把标签盒子也塞进 `rowRects[i]`（重现 P13-w 警告的那个错误） | 4（编号家族被标签占位） |
| M4 | 整行可见的标签也淡（`fold < 1` 改成 `fold <= 1`，或直接 `Opacity(0.5)`） | 3 |
| M5 | 用 `rowRects`（而不是 `headingRects`）当分组头的盒子来源 | 1、2（盒子永远空 → 折边恒 1） |

## 出图

`/tmp/floter-P13-x-headingfold-{light,dark}.png`——同一页两帧拼在一起不是测试库的能力，
所以每套外观出一张「折边正好切着标签」的帧（启动器侧，`frameHeight` 减到标签被切），
以及一张捕获输出页的对应帧：`/tmp/floter-P13-x-outputheading-{light,dark}.png`。
