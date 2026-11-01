# P13-dc 报告 · 插件配置浮层：三条 pin 一直在量一个不存在的界面

规格 `docs/rounds/P13-dc-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.72**（§6.19 顶部加了一条重量注）｜起点 `84984977`（P13-db）。

**这轮为什么让它更有空间感（一句话）**：同一张浮层上的四级字（名字 / 分组 / 标签 / 说明）现在
**全部**落在同一条阶梯上——分组标题原来钉在 `body + 2` 的**偏移量**上，用户把「界面大小」调一步，
它就离开阶梯一点；换成 `tokens.Title` 之后，四级字一起随用户的放大缩放。

## 1. 先量：尺子本身是歪的

`sheetcutedge_test.go` 的 `pluginSheetFrame` 从 P13-aw 起只取了 `tokens.Theme.Background` 画底，
**没有 `c.SetTheme(tokens.Theme)`**；而 `PluginSheet` 的间距与字号读的是 `c.Theme()`
（`pluginsheet.go:62-63`）。于是：

| 量 | 取景（改前） | 生产（设计） |
|---|---|---|
| `t.FontSize`（body 档） | 14（框架库存） | 10.8 |
| `t.Space(1)` | 4 | 3.6 |
| 标题盒 | `(20,63) 90×22` | `(18.4, 63.7) 70×17` |
| `MaxY` | 500 | **316.4** |

三条 pin 开着绿灯，量的却是一个不存在的界面。P13-cm 已经记过同一条口径教训（pin 必须
`c.SetTheme(a.tokens(c).Theme)` 取景），这是**第三次**现形。

## 2. 改了什么

1. **取景**（测试）：`pluginSheetFrame` 每帧先 `c.SetTheme(tokens.Theme)`，再画底。
2. **重钉三条既有 pin**（V）：
   - `TestThePluginSheetEdgesAddNoGeometry`：`MaxY` 500 → **316.4**（float 用 ±0.05 的窗，
     量到的是 316.4001）、标题盒 `y=63 h=22` → `y≈63.7 h=17`。
   - `TestThePluginSheetEdgesSitOnBlankAtBothEnds`：窗口从凭手感的 **12 行**改成
     `ceil(3u)` = **11 行**（= 边高），并加了一条前置断言「窗口之外必须真有墨」——
     否则这条 pin 会在一条空白带上轻松通过。实测：最上 11 行 **0**、第 12 行 **62**；
     最下 11 行 **0**、往外一行 **30 / 36**。
   - `TestThePluginSheetFadesAtBothEdges`：零改动，读数从 91/88 变成 **92 / 96**（内侧 228）、
     深 **101 / 101**（内侧 240），仍 ≤ 45%。
3. **字阶**（产品）：`pluginsheet.go:153` `FontSize(t.FontSize+2)` → **`FontSize(tokens.Title)`**。
4. **新 pin**：`internal/ci/fontladder_contract_test.go`
   `TestThePluginSheetsTextSizesAreLadderRungs` —— AST 账，规则三条：
   ① 不许 `FontSize(<算式>)`（`t.FontSize±N` 这一族）；② 参数只许 `tokens.<档>` 或
   `t.FontSize`（浮层**自己的名字**，§6.71 的「故意不做」）；③ 账不能空转（至少三条 `FontSize`）。
5. **出图**：`TestDumpP13DCPluginSheetLadder`（opt-in `FLOTER_P13DC_DUMP=1`）。

**量过、不改（写进报告不进代码）**：几何自门控仍然成立，但余量只有 **0.2px**（边高 10.8 对
空白 11 行）。所以这一轮**没有**把门改成 `ScrollState.Y`——测量证明几何还成立，改机制没有理由；
但把「余量几乎为零」写进了 §6.72 与 `pluginsheet.go` 的注释：标题行一旦变高就该换门。

## 3. V / B / S 台账

- **V 3**：三条浮层 pin 的基线被重钉（原因：取景此前没装设计，量的是框架库存的字）。
  旧值 → 新值写在断言注释里。
- **B 0**：`go test -count=1 ./...` 全绿；行为 pin 零改动。
- **S 1**：`pluginSheetFrame` 这个**夹具**换了取景方式（测试结构），产品结构不变。

## 4. 门槛（实测）

| 项 | 实测 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 / 净 / 净 |
| `go test -count=1 ./...` | exit 0，全绿 |
| PASS | **3308**（基线 3307，+1 = 新 contract test） |
| FAIL | 0 |
| SKIP | **145**（基线 144，+1 = 本轮 opt-in 出图） |
| 顶层测试函数（`^=== RUN   Test…$` 口径） | **2454**（基线 2452，+2） |
| `GOOS=linux/windows/darwin go build ./...` | 过 / 过 / 过 |

## 5. mutation（3 条，实跑变红 + 逐字节还原）

**M1**（取景退回 HEAD：删 `c.SetTheme(tokens.Theme)`）：

```
--- FAIL: TestThePluginSheetEdgesAddNoGeometry/light
    sheetcutedge_test.go:214: the sheet's scroll range moved: MaxY 490, want 316.4 (the edges are paint, not geometry)
    sheetcutedge_test.go:218: the sheet's title band moved: {X:20 Y:63 W:90 H:22}, want y≈63.7 h=17 (...)
```

**M2**（分组标题退回 `t.FontSize+2`）：

```
--- FAIL: TestThePluginSheetsTextSizesAreLadderRungs
    fontladder_contract_test.go:55: ../settingsui/pluginsheet.go:153:4: the size is an expression
    (`t.FontSize + 2`), not a rung of the ladder: ... — use `tokens.<rung>`
```

**M3**（给浮层**标题**也加一个偏移 `t.FontSize-1`）：

```
--- FAIL: TestThePluginSheetsTextSizesAreLadderRungs
    fontladder_contract_test.go:55: ../settingsui/pluginsheet.go:83:6: the size is an expression
    (`t.FontSize - 1`), not a rung of the ladder: ...
```

三条都用 `cp` 还原 → `md5sum -c` 逐字节核对（两条来源各一次「成功」），再 `touch` 测试文件复跑
→ `ok floter/internal/settingsui 0.838s` / `ok floter/internal/ci 0.005s`（不吃缓存）。

## 6. 出图

```
FLOTER_P13DC_DUMP=1 go test -run TestDumpP13DCPluginSheetLadder ./internal/settingsui/
FLOTER_P13DC_DUMP=1 FLOTER_P13DC_TAG=before go test ...   # 在 git stash 掉 pluginsheet.go 的树上跑
```

```
/tmp/floter-P13-dc-pluginsheet-light-before.png  ←→ -light-after.png
/tmp/floter-P13-dc-pluginsheet-dark-before.png   ←→ -dark-after.png
```

逐像素差分（720×600）：**浅 908 px / 深 909 px，bbox `(31,100)-(125,520)`**——只有两个分组标题
（`First section` / `Second section`）的墨，字段行一个像素没动。P13-db 记的「零像素差」是**库存字
取景**里的读数，那条结论在本轮作废（12.8 对 12.6 在设计的取景里差出 ~900 px）。

**这张图上看到的一处变化**：`First section` 这一行比改前**窄了一点**——右端收进去 2px 左右，
但行盒、行高、与下面那张卡的距离一格没动（`MaxY` 316.4 与改前相同）。

## 7. 用了设计 skill（用户 2026-10-30 要求）

读的路径：`SKILL.md`（Router Rules 的「已有方向，只要打磨」车道）→ `references/user-preferences.md`
（P-5 可读性优先、P-1 斜体硬禁令）→ `references/craft-loop.md`（§二工艺密度菜单 + §三两轮制四视角）
→ `references/preflight.md`（A / B / C / D2 逐条）。**没有**走 Phase 2 的四方向预览车道，没有出
HTML 预览页。

### 偷了什么（逐条落到具体数值）

1. 从 `craft-loop.md` §二菜单取 **第 6 项「编辑细节」**（菜单原例：hairline、大写字距标签）：
   落到「浮层里的字号只有阶梯上的档」——`tokens.Title` 12.6 取代 `body + 2` 的 12.8，
   并且由 AST 账钉住（`fontladder_contract_test.go`），不是靠人记。
2. 从 §二菜单取 **第 12 项「材质细节」**（菜单原例：内边框高光、玻璃折射边）：落到
   §6.19 那道滚动边**余量 0.2px** 的重新量取——10.8px 的边对 11 行空白，读数是 0 对 62。
3. 从 §三「两轮制」的**四视角评审·工程师**那一条（「哪里对不齐 / 状态缺失 / 间距不一致」）：
   取景与生产不一致本身就是这一类缺陷，本轮把它当阻断项修掉（pin 量的是库存字）。

### preflight 逐条结论

- **A 反 AI 套路**：过。本轮没有新增视觉母题（无渐变、无发光、无左侧竖线 P-38），只换了
  一个字阶档与一把尺子。
- **B 设计读取一致性**：过。设计读取是「近黑底 + 发丝线 + 阶梯字阶 + 一条受光边」，本轮改的
  正是「字阶必须来自阶梯」这一条；主题仍单套（深浅两帧各自一致）。
- **C 动效抽查**：不适用 / 过。本轮零动效改动（浮层的滚动边按位置**开关**，不是动画）。
- **D2 中文页追加项**：**不过**，沿用第 28 份证据——`tokens.Caption` 9.9px 对 D2「正文 ≥ 14px」。
  这是设计源本体的取舍（参考的 `--text-caption` 与我仓的 11u 阶梯），不是这一轮引入的；
  已记在 `.pending-bugs.md` 的「需用户拍板」里，不擅自改。
