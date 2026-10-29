# P11-cl 报告：点一下行的那一帧，目录也只走一遍

轮号 `P11-cl`｜分支 `mygo-rewrite`｜规格 `docs/rounds/P11-cl-spec.md`

## 1. 选题理由

选题顺序走到第 4 档（用户实测反馈 F1–F4 已由 P11-y/z/aa/bb/bj/bk/bo 收完 → 合约 §6 全
`已落地`，只剩拍板项 → P11-ck「仍待」第 1 条整张是 15 条拍板项、第 2 条本轮已关闭、
第 3/4 条是记账与尺子盲点）→ 按用户定的「界面质感与交互（含用户报的 bug）」自选。

挑中的是 `internal/launcher` 的**绘制期指针边**：它不是新功能、不是拍板项，是同一份代码里
已经写明的规则漏了一个调用点（键盘那一支早改过，指针那一支没改）。

## 2. 现象与根因（全部本轮实测）

`internal/launcher/launcher.go:2656`（改前）：

```go
if row.Clicked() && !removed {
    a.Selected = i
    a.pointerChose = true
    a.activate(a.Results())   // ← 绘制期的第二次全量匹配
}
```

`a.Results()` 不是缓存读，它每次调用都 `a.searchRuns++` 并**完整重建**目录
（`search.go:811-816`：`Catalog()` → 每条打分 → 拼音）。而 `View` 在 `:1191` 已经为这一帧
建好了 `results` 并把它交给列表建行。所以：

- **一次点击的帧里有一遍多出来的全量匹配**，而且这一遍发生在**绘制中**（用户已经点下去、
  窗口正在画这一帧），没有任何按键可以解释它。
- 大目录（Start Menu 上千条 + 拼音打分）下，这一遍就是「点一下要等一下」。
- 它与 `nextSelectableIn` 的注释里那句规则是同一个形状：
  「the draw clamps its choice against the slice it built, and asking for a fresh catalog there
  was the draw's second full match」——键盘那支已经照它改了，指针那支漏了。

**实测**（720×488，五条最近启动，`internal/launcher/zz_scratch_p11cl2_test.go` 探针）：

```
click a row          searches per draw: [1 1 1 … 1 2 1 … 1]   ← 第 22 个 draw 是 2
idle frame           searches per draw: [1]
click the footer bar searches per draw: [1]
```

同一个探针还量到：`Esc` 从有查询的页面回到空页时，落点是 `app:/Applications/Recent00.app`
（`Selected=1`，跳过 `section:recent` 那条分组头），空页 `Up` 回绕到 `Recent04`——这两条是
**既有正确行为**，本轮未动，记在这里是为了不让人当缺陷重领。

## 3. 落地

### 3.1 产品代码（`internal/launcher/launcher.go`，三处，共 +9 行含注释）

1. `App` 新增字段 `drawn []Item`，注释写清它为什么存在（与 `nextSelectableIn` 同规则）。
2. `View` 在 `clampSelection` 之后、`assignNumbers` 之前写 `a.drawn = results`——**就是列表
   建行用的那一份**，不是新查一遍。
3. `row` 的 `Clicked` 分支 `a.activate(a.Results())` → `a.activate(a.drawn)`。

**行为契约零改动**：选择/指针交接/行几何/动作执行/编号/高度链/键盘模型/i18n/命令接线
一个字节没动；`a.Selected = i` 与 `a.pointerChose = true` 两行原样保留。**零像素改动**。

### 3.2 测试（两处）

- `internal/launcher/searchruns_test.go`：`TestADrawWalksTheCatalogOnce` 的步骤表新增
  `{"a click on a row", 0, …}` 一臂（夹具用 `recentPage(a,5)`，点 `Recent 02`）。
  这是**表里的一步**，不是新子测试——所以它涨覆盖面不涨 PASS 计数。
- `internal/launcher/clickdraw_test.go`：新 pin `TestAClickRunsTheRowTheDrawBuilt`。
  两半都读：点击帧每遍搜索数 == 1，**且**跑起来的那一行就是指针落到的那一行
  （先 `Type("Recent 02")` 把列表收窄，再点，证明读的是**这一帧**的切片而不是上一帧的）。

## 4. 台账（V / B / S）

- **V = 0**：没有旧 pin 钉过这次查询的来源；无像素改动。
- **B = 0**：没有既有行为/度量 pin 需要改；`searchruns_test.go` 是**加**一臂，不改旧断言。
- **S = 0**：没有结构改名/搬迁。

## 5. mutation（2 条，实跑变红 + 逐字节还原）

**M1** — `row` 的 `Clicked` 分支改回 `a.activate(a.Results())`：

```
--- FAIL: TestADrawWalksTheCatalogOnce (0.47s)
    searchruns_test.go:111: the a click on a row frame searched twice on 1 draws, but injected 0 keystrokes
    searchruns_test.go:115: the a click on a row frame searches twice on 1 draws with no keystroke to explain it
```

**M2** — 删掉 `a.drawn = results`（`drawn` 恒为 nil）：

```
--- FAIL: TestAClickRunsTheRowTheDrawBuilt (0.53s)
    clickdraw_test.go:51: the click ran [], want [Recent 02]
```

**M2 是这一轮最重要的实测**：`TestADrawWalksTheCatalogOnce` 在 M2 下**仍然绿**（`drawn` 为空
时那一次点击根本不去查目录，搜索数还是 1）。只数次数守不住「跑的是哪一行」，所以新 pin 必须
两半都读——这正是本仓「断言要钉行为、不要钉形态」的既有教训。

两条变异都 `md5sum -c` 逐字节还原（`internal/launcher/launcher.go: 成功`），再 `touch` 复跑绿。

## 6. 门槛（本轮实测）

```
gofmt -l cmd internal                    空
go vet ./...                             净
GOOS=windows go vet ./...                净
GOOS=darwin  go vet ./...                净
go test -count=1 ./...                   全绿（FAIL 0）
go test -count=1 -v ./... PASS 计数      2763 → 2764（+1）
go test -count=1 -v ./... SKIP 计数      55（不变）
go test 函数数                            2052 → 2053（+1）
GOOS=linux/windows/darwin go build ./... 全过
```

## 7. 出图

**不出图**：本轮一个像素都没改（证据是「哪份切片被读」，不是墨）。按纪律，视觉轮才必须出图；
这一轮不是视觉轮，且出图也看不出差别（同一行、同一墨、同一几何）。

## 8. 仍待（P11-cl）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行 /
   P11-cc 带子里要不要显示全文 / P11-cd 空页要不要一句引导）。
2. **`a.Results()` 的其余调用点本轮逐条核过**（`launcher.go:1087,1499,1874,2656,2848,2878,3203`、
   `calculator.go:308`、`fieldkeys.go:328,349`）：
   `:2656` 是唯一一处**绘制期**的调用（其余全在按键处理里，一帧一次是它们的价格）。
   本轮只动了它，别把其它几处当「同一族漏网」重领——它们不是绘制期。
3. **`App.drawn` 的生命周期是「一帧」**：它不重置，只被下一帧覆盖。若将来有人在**非绘制期**
   读它（比如按键处理里），读到的是上一帧的列表——那是错的。要读列表就 `a.Results()`。
   这条写在这里是为了防止误用，不是缺陷。
4. 尺子老盲点不变（只量字盒 / `Tester.Find` 按字符串去重 / 滚动容器裁掉的行不进 `Texts()` /
   PTY 自己画的行不在高度尺子上）。
