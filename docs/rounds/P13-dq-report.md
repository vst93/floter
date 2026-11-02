# P13-dq 报告：帧高的两条腿各有名字（库存尺 / 出货尺）

**轮号**：P13-dq ｜ 规格 `docs/rounds/P13-dq-spec.md` ｜ 合约 `docs/liquid-glass.md` §6.86
**改动面**：`internal/launcher/view_test.go` + 新增 `internal/launcher/frameruler_test.go`；
**产品代码零改动**（`git status` 可证）。

---

## 1. 这轮做了什么

`§6.76` 把启动器的取景收口记成「21 条红」，任务书把它排成 `P13-dg-l`。本轮先把它量准，
结论与那句记账不同：

- **21 个测试函数、118 条断言、17 个文件**（不是 21 条断言）；
- 其中 **1 条是产品的墨**（`outputsubtitle`：同一张卡的两条列表给选中行副标题差 **6** 级，
  119 vs 125，容差 ±4），其余 20 条是取景/单位/取整/前提；
- 一次把 `render` 换成出货尺会让这 21 个函数**同时**红 —— 那正是任务书禁止的
  「下轮清空」式交付。

所以本轮落地的是**尺子**：`frameHeight`（库存尺）与 `themedFrameHeight`（出货尺）
分开命名，`designMetrics` 按 `App.View` 的量法实测四个数。`P13-dg-l` 从「一格 21 条」
拆成**逐表面交割**，账写在 §6.86 的表里。

## 2. 实测（本轮自己跑出来的）

| 量 | 库存尺（夹具此前） | 出货尺（生产） |
|---|---|---|
| `Spacing` | 4.0 | **3.6** |
| `FontSize` | 14.0 | **10.8** |
| 行盒 `Ag` @ 标题（12.6） | 16（`font + rowTitleStep` 兜底） | **19** |
| 行盒 `Ag` @ 正文（10.8） | 14（`font` 兜底） | **17** |
| 十行帧高 | **488** | **439** |

**关键发现（补上 §6.76 的最后一块）**：出货尺下 `rowUnits × 3.6 = 36` 的行地板仍高于
19 的行盒，所以**十个结果行的帧高差全在 chrome 与 page-header 上**；把两个行盒喂成 0
（变异 M3）十行帧高**一个数都不变**。这条 pin 因此在行上天生是盲的 —— 它钉的是
**尺子**（3.6 / 10.8 / 19 / 17）与**两帧的差**（488 − 439 = 49），不是任何一行的盒子。

## 3. 台账

- **V = 0 / B = 0 / S = 0**：本轮**没有改任何既有断言**，只新增 2 条 pin
  （`TestTheTwoRulersAreNamedAndMeasured`、`TestTheProbeResolvesTheDefaultStore`）。
- 测试函数 2489（本轮 +2；同一口径下基线 2487 —— P13-dp 报告的 2485 是另一口径，
  本轮以 `grep -rn "^func Test" --include=*_test.go | wc -l` 为准）。
- 产品代码：**0 改动**。

## 4. mutation（实跑，全部还原并逐字节核对）

| # | 注入 | 实测 |
|---|---|---|
| M1 | `designMetrics` 标题行盒 `FontSize + rowTitleStep` → `FontSize` | **红**：`frameruler_test.go:34: the shipped line boxes are title 17.00 / subtitle 17.00, want 19 / 17 (measured 'Ag' at 12.6 and 10.8 — §6.76's 22.0 → 17.0 at the body size)` |
| M2 | `designMetrics` 把上下文装成 `ui.LightTheme()` 再量 | **红**：`frameruler_test.go:34: the shipped line boxes are title 22.00 / subtitle 22.00, want 19 / 17` |
| M3 | `themedFrameHeight` 喂 `TitleLine: 0, SubtitleLine: 0` | **静默**（预期）：十行帧高 439 不变 —— 36u 行地板赢。**照实登记为本 pin 的盲点**，不假装它抓到 |

还原：M1/M2/M3 都 `cp` 回原件，`md5sum -c` 逐次 **成功**；M3 之后 `touch internal/launcher/*_test.go`
再全包复跑 → `ok`（避免测试缓存复用）。

## 5. 门槛（实测）

```
gofmt -l cmd internal                → 空
go vet ./...                         → 净
GOOS=windows go vet ./...            → 净
GOOS=darwin  go vet ./...            → 净
go test -count=1 ./...               → 全绿（FAIL 包 0）
go test -count=1 -v ./... | grep -c -- "--- PASS" → 3324（基线 3322）
GOOS=linux/windows/darwin go build ./... → 过
测试函数 → 2489（基线同口径 2487；+2）
SKIP 不变（153；本轮没有新增 t.Skip）
```

## 6. 图

本轮**无像素改动**（产品代码零改动），所以不出新图；上一轮的设计尺证据仍在
`/tmp/floter-P11-au-drawer-end-{light,dark}.png`（P13-dp）。这一轮的可验证产物是
**两个数字**（488 / 439）与**四个尺子值**（3.6 / 10.8 / 19 / 17），它们由新 pin 逐个钉住。

## 7. skill 复核（`qiaomu-design`）

读：`SKILL.md` → `references/user-preferences.md` → `craft-loop.md` → `preflight.md`
（本轮只需这四份；没通读全库）。走「已有方向，只要打磨」车道，不出 HTML 预览、不等点选。

**偷了什么**（逐条）：

1. `craft-loop.md` 工艺密度第 3 项（**尺子/单位先于像素**）：落成「库存尺 / 出货尺」两个
   命名函数，并把四个尺子值（3.6 / 10.8 / 19 / 17）写成断言 —— 下一格起「换一个表面的取景」
   是换一个函数调用，不是散点改小数。
2. 同菜单第 6 项（**一个判据只许有一个来源**）：落成 `themedFrameHeight == Geometry.Height()+2`
   这条自检 —— 帧高不许出现「helper 里另有一个常数」。
3. 同菜单第 9 项（**换窗口不是换断言**）：落成 §6.86 表里那 5 条 S 的手续（`recentPage` 行数 /
   帧高 / 滚动位置 / 呼吸带从裁剪上沿推导），写死给下一格。
4. `preflight.md` 的 B（设计读取一致性）：本轮把「不一致」量成 118 条并分类，判据保留在合约里。

**preflight 逐项**：A（反 AI 套路）过 —— 本轮无新增文案、无占位符、无「赋能/无缝」类词；
B（设计读取一致性）**未过** —— 这正是本轮量化的工作，处置 = §6.86 的逐表面交割表；
C（动效抽查）**N/A** —— 产品零改动，无动效可抽查；D2（中文页正文 ≥ 14px）**仍未过**，
沿用第 42 份证据（`tokens.Caption` 9.9px / 正文 10.8px），**需拍板**。

## 8. 仍待（P13-dq）

1. **下一格（首选）：`P13-dg-l-1` = 10 条单位算术 V。** 把「参考单位 ×4」改成 `× designUnit`
   （`designUnit = 3.6/4 = 0.9`）：`x=54 → 48.6`、`pin 20u → 18`、`floor 55 → 49`、
   `status band 30u → 27`。同批带 2 条取整（`int()` → `math.Round`；盒子宽度按 painter 的
   `round(x+w) − round(x)` 读）。**一次只改一条表面**，改完立刻把该表面的取景换到
   `themedFrameHeight`。
2. **`P13-dg-l-2` = 5 条前提破 S**：编号族 9 → 10（`recentPage` 行数）、`outputedges` 两处
   呼吸带改从裁剪上沿推导（不再读 118）、`outputfold` 首行 share、`outputedges` 视口下沿。
3. **`P13-dg-l-3` = 2 条混排 B**： `textcolumn` / `outputheading` 的 `ui.NewTester` 装上
   这一面的调色板。
4. **`P13-dg-l-4`（产品墨，独立一轮）**：`outputsubtitle` 的 6 级 —— 两支副标题要收到同一条公式。
5. **沿用需拍板**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」（第 42 份证据）；
   终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态说错对象 /
   `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 /
   路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
6. **抢救 stash 核对（第四十四次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
