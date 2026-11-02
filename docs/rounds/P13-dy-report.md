# P13-dy 报告：启动器的行高阶梯在出货尺上是六档，其中两档只差 0.2

**轮号**：P13-dy ｜ **表面**：`internal/launcher` 的**夹具尺子**（产品代码零改动）
**合约**：`docs/liquid-glass.md` §6.94 ｜ **规格**：`docs/rounds/P13-dy-spec.md`
**选题**：`P13-dx`「仍待」第 1 条 = §6.86 表里「阶梯（V）1 条」，`dg-l` 的最后一格。

## 1. 先量（证据）

`renderShipped`（出货尺，`u = 3.6/4 = 0.9`）+ `frameaudit.Heights`，en/zh × 深/浅
**四组合逐字节相同**：

| 臂 | 实画 | 逐 run 归属（`frameaudit.Read`） |
|---|---|---|
| 空查询页 | `[17 25 25.2]` | `Type to search` 17 ｜ `Search` 25 ｜ `Settings` 25.2 |
| 命中页 | `[15 17 19 25 25.2]` | `Ctrl`/`1`/`↩`/`K` 15 ｜ `git` 副标题 + `PATH` + `Run in shell` 17 ｜ `git` 标题 19 ｜ `Search` 25 ｜ `Settings`+`Clear` 25.2 |
| 无结果页 | `[18 25 25.2]` | `No results` 18（`launcher.go:885` `quietStatement`，`tokens.Emphasis` 11.7）｜ `Search` 25 ｜ 两个图标键 25.2 |

库存尺（pin 此前量的那一帧）是 `[15 18 22 25 28]`：`18 / 22 / 28` 与出货尺一个都对不上，
而 `±0.5` 的容差带在库存尺下永远绿。

## 2. 落地（只改夹具）

1. `render` + `tt.SetDark` → `renderShipped`（两半同尺）。
2. 阶梯 `[15 18 22 24 25 28]` → **`[15 17 18 19 25 25.2]`**，具名常量 + 每档的出处注释。
3. 三条 `must` → 实测值（见上表）。
4. 容差带 `0.5` → **`0`**（`Heights` 已按十分位取整）。
5. 新增 `TestTheLauncherIconBoxIsSevenUnitsTall`：`7 × Spacing = 25.2`、`25.2 − 25 = 2 个十分位`、
   真帧上两档都在。
6. 新增 opt-in 出图 `FLOTER_P13DY_DUMP=1` → `/tmp/floter-P13-dy-ladder-{light,dark}-{shipped,stock}.png`。

**行为契约零改动**：高度链、键盘模型、会话管理、设置写回、i18n、命令接线一个字节不动。

## 3. 台账与门槛（实测）

- V 1 / B 0 / S 0；产品改动 **0 行**。
- 测试函数 2492 → **2493**；PASS **3325 → 3326**；FAIL 0；SKIP 159 不变。
- `gofmt -l cmd internal` 空 ｜ `go vet ./...` / `GOOS=windows` / `GOOS=darwin` 净 ｜
  `go test -count=1 ./...` 全绿 ｜ 三平台 `go build ./...` 过。

## 4. 变异（3 条实跑变红并还原，`md5sum -c` 成功 + `touch` 复跑）

| # | 注入 | 红串 |
|---|---|---|
| M1 | `renderShipped` 退回 `render` | `height 22.0 is not on the ladder [15 17 18 19 25 25.2] (±0.00)` + `no run of height 17.0 was painted (heights [22 25 28])` |
| M2 | `rungIconBox 25.2 → 25` | `height 25.2 is not on the ladder [15 17 18 19 25 25] (±0.00)`（三臂 × 四组合） |
| M3 | 产品 `launcher.go:890` `tk.Emphasis → tk.Body` | `no run of height 18.0 was painted (heights [17 25 25.2])`（四组合） |

**M3 的第一次尝试是静默的**：`height.go:62` 的 `rowTitleStep 2 → 0` 全绿 ——
行标题画出来的 19 **不来自**这个常量（它只喂帧高与 `designMetrics` 的标题行盒）。
照实记进 §6.94，不留成「以后再说」。

## 5. 出图（肉眼复核）

`FLOTER_P13DY_DUMP=1` → 四张，两把尺子的帧高正是 §6.86 的两个数：
库存 **720×488**、出货 **720×439**（`file` 逐张读的）。
**这张图上看到的一处变化**：命中页那一行在出货尺上是 19 / 17（库存 24 / 22），
整张图因此矮 49px —— 同一份内容，两把尺子给出两种呼吸。

## 6. 偷了什么（`qiaomu-design`，走「已有方向，只要打磨」那条路径）

本轮读的是：`SKILL.md`（路径表 + 路由器）→ `references/user-preferences.md`（头部规则）
→ `references/craft-loop.md`（工艺密度菜单）。**没读** `preflight.md` 全文（预算），
A/B/C/D2 按合约既有结论对照，见 §7。

| 从菜单取的 | 落成哪个具体数值 |
|---|---|
| **第 6 项「编辑细节」**（hairline / 大写字距标签那一类「每屏有意为之的细节数量」） | 阶梯从「六个数字」变成**六个有名字的档**：15 键帽图例 / 17 body 行 / 18 陈述句 / 19 行标题 / 25 字段行 / 25.2 图标键；pin 从「落在某个数附近」收紧成**精确相等** |
| **第 8 项「流体字号」**（字号随视口呼吸） | 六档全部改成 `× Spacing` 的读数：`25.2 = 7 × 3.6`（`28u`），`u = Spacing/4 = 0.9`；用户把界面大小调一档，六档一起呼吸，没有一档钉死 |
| **第 12 项「材质细节」**（内边框高光那一类） | 只取了「网格是材料的刻度」这一半：`25.2` 与 `25` 的 0.2 差**写进账**（六档）而**不写进眼**（`round` 之后同高 = 五档），不假装那是两档可见的空间 |

**没凑满 5 项，照实说**：第 1/2/4/7/9/10/11 项是 web 产物的动作（`::before` 氛围层、
`::selection`、滚动条、SVG 资产、彩蛋），本仓是 Go 自绘、没有 CSS，硬套就是「假装有」——
正是 `liquid-glass.md` §0 禁令 1 点名的行为。

## 7. preflight 四条（逐条结论）

- **A 反 AI 套路**：过。产品像素零改动；没有渐变白边、没有伪造 blur、没有 glass 叠 glass。
- **B 设计读取一致性**：过，而且是本轮的主收获 —— pin 此前读的是框架库存尺（`Spacing 4.0` /
  `FontSize 14`），现在读的是 app 出货尺（`3.6` / `10.8`）。
- **C 动效抽查**：不适用（没有动效改动；`theme.ColorStep` 一个字节没动）。
- **D2 中文页（正文 ≥ 14px）**：**仍不过**。`tokens.Caption` 9.9 / 正文 10.8。
  本轮新增第 **50** 份证据：18.0 那一档（emphasis 11.7）与 17.0（body 10.8）**只差 1px**，
  空态陈述句与普通正文在 1x 屏上几乎同高 —— 字号这一档要动，是全局设计决定，
  仍挂在 `.pending-bugs.md` 的「需拍板」。

## 8. 仍待

1. `dg-l` 结案；下一个画面包 `P13-dg-t`（`internal/terminalui` 的夹具取景）→ `P13-dg-s`。
2. §6.86 表里剩下的 `未开始（取景）` 行（`de-2`/`de-3`/`de-5`/`de-6`/`de-9`/`de-11` 与
   `73-5`/`73-6`）现在只被「`render()` 取景 + 会话行地板」挡着，与本轮同族。
3. **需拍板**：`tokens.Caption` 9.9 / 正文 10.8 对 D2「≥ 14px」（第 50 份证据）。
