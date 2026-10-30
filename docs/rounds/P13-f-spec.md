# P13-f 规格：文件页表面（`docs/liquid-glass.md` §6.5）——E1 的证据与图标列说错的那句话

**轮号**：P13-f（视觉队列第 5 片；§6.1 = P12-b、§6.2 = P12-c、§6.3 = P13-d、§6.4 = P13-e 已落地）
**表面**：文件页（`internal/launcher/files.go` 的 drop 页）**一个表面，一轮**。
**为什么现在做**：`docs/liquid-glass.md` §6 是唯一权威选题队列，§6.5 是表里最后一个
`未开始` 的表面。终端弧剩下的 S1b（`CloseAll` 改 detach）**改会话管理语义 = 需用户拍板**，
所以按纪律回到视觉队列。

**这轮为什么让它更轻 / 更有空间感**（每个视觉轮必答的一句话）：
这一页的 E1 在 P12-b 之后**已经继承到了**（rows 都走 `rowBody`，选中块就是
`tokens.LitFace`），但这一页**从来没有一条像素证据**——「继承到了」和「量到了」是两件事。
更重要的是：这一页每行第一眼落在那块 28u 的图标板上，而**十五个板里画的是十五个历史
时钟**（`rowGlyph` 的尾巴 `return glyphClock, true`）——一个把「打开 / 进终端 / 拷路径」
三件不同的事都画成「最近用过」的图标列，会把这一页唯一的层级信号（面 + 受光边）
淹掉：读起来是「一堆一样的东西」，这正是用户说的「平、糊」。让三个动作各说各的，
面与线的层级才读得出来。

---

## 落地项（位置 → 现状 → 应改成）

行号是本轮 `grep -n` 实测（动手前重核），不是旧报告里的漂移值。

### 5-1 文件列表行（`internal/launcher/launcher.go:2347`、`:2359`、`:2370`）——**复核 + 像素 pin**

- 现状：文件页的行不是另一套绘制，它们就是 `rowBody` 的普通结果行
  （`files.go:144` 的 id 是 `drop:<action>:<path>`，不带 `bar:` 前缀 → 走 `i == a.Selected`
  那条臂 `launcher.go:2363`，其中 `row = tokens.LitFace(row)` 在 `:2370`）。
  所以 E1 的**代码**这一页早就有了，缺的是**证据**。
- 应改成：**产品代码零改动**。补一条像素 pin（深/浅两套）：文件页选中块的
  ①面阶 ≥ `litedgeFaceFloor`（8 级）②顶边严格亮于面 ③块外没有投影。
  这一条正是 §6.5 原行写的「与 1-2 同一条规则」；没有像素证据的「同一条规则」不算落地。

### 5-2 **新增**｜图标列（`internal/launcher/glyphs.go:196-229`）——十五个时钟

- 现状：`rowGlyph` 的 `switch item.ID` 里没有任何 `drop:` 分支，尾巴是
  `glyphs.go:228` 的 `return glyphClock, true`。于是文件页每一条行——`drop:open:` /
  `drop:cd:` / `drop:copy:`——都戴着**最近启动的时钟**。一屏五份文件就是十五个时钟。
- 应改成：**与动作条同一语汇**。这三件事在动作条上已经有一套钉死的标记
  （`glyphs.go:201-204`）：`bar:path-open` → `glyphFolder`、`bar:path-cd` → `glyphTerminal`、
  `bar:path-copy` → `glyphFile`。文件页的三行**就是**同一个路径的同一组动作，
  所以它们必须戴同一组标记（id 里带着路径，因此按前缀判，不用等值 switch）。

### 5-3 **新增**｜折叠行（`internal/launcher/files.go:112` `drop:more`）——时钟 → 展开符

- 现状：`…and N more files` 这一行同样落进 `glyphClock`。它做的事是**展开**列表
  （`Run` 只置 `dropsExpanded`），不是「最近」。
- 应改成：`glyphChevronRight`（`glyphs.go:66`）——客户端的「还有更多，往这边」，
  与它自己那一行的语义一致。

### 故意不做｜图标板（`internal/launcher/glyphs.go:257` `iconPlate`）

- 它还是 `tokens.Lens(plate)`（四层，第四层是四边 `ControlEdge` 的环）。**不动**：
  `iconPlate` 是启动器**所有页**共用的一块板（结果行、剪贴板、计算器、浏览器都戴它），
  改它等于跨表面——违反「一个表面一轮」。留一条观察给启动器表面那一轮。

---

## 行为契约零改动红线

- **id 不动**（`drop:<action>:<path>` / `drop:more` / `section:dropped`）：`view_test.go` 里
  有按 id 数行数的 pin（`filesItems()` 的 8×3+1），行数、顺序、`Run` 一个都不碰。
- **高度链不动**：行还是 40u（`rowUnits`），图标板还是 28u（`Space(7)`），标题列不移动。
  换的只是板里那 16px 的墨。
- **键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线**：零改动。
- i18n：`FilesSection` / `FilesMore` / `FilesMoreHint` 一个字都不动。

## 门槛命令 + PASS 基线（本轮实测）

基线（P13-e，HEAD `1c7e9953`）：**PASS 2887 / FAIL 0 / SKIP 63 / 测试函数 2137**。

```
gofmt -l cmd internal
go vet ./...
GOOS=windows go vet ./...
GOOS=darwin  go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## mutation 计划（≥2 条，说明每条会让哪个 pin 变红）

| # | 注入的缺陷 | 应该变红的 pin |
|---|---|---|
| M1 | `glyphs.go` 删掉三条 `drop:` 前缀分支（回到尾巴的时钟） | `TestTheFilesPageLeadsWithItsOwnActionMarks`（三条行各报「expected folder/terminal/file, got clock」） |
| M2 | `drop:more` 改回 `glyphClock` | 同文件里的折叠行断言 |
| M3 | `launcher.go:2370` 的 `LitFace` 换成 `Background` 之外的空操作（只留平涂） | `TestTheFilesPageChosenRowStandsOnOneLitEdge`（顶边不亮于面 / crown 缺失） |
| M4 | 给文件行补一条 `Shadow(...)`（E1 冒充 E2） | 同一条 pin 的第 ③ 段（块外不是面板色） |

## 出图

`/tmp/floter-P13-f-files-{dark,light}-{before,after}.png`（`FLOTER_PNG=1` +
`FLOTER_PNG_SUFFIX=-before|-after`，先跑一次生产改动前的代码，再跑一次改动后的）。
