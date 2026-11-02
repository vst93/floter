# P13-dr 报告：字段行一族的取景换成出货尺（`54u → 48.6`、`56u-1 → 49`）

**轮号**：P13-dr ｜ 规格 `docs/rounds/P13-dr-spec.md` ｜ 合约 `docs/liquid-glass.md` §6.87
**改动面**：`internal/launcher` 五个 `_test.go`；**产品代码零改动**（`git diff --stat` 可证）。

---

## 1. 这轮做了什么

`P13-dq` 把 §6.76 的「21 条红」摊平后排出下一格 `P13-dg-l-1`＝「10 条单位算术 V」。本轮只做
**其中同一个表面**的那一族：**字段行（54u 的文字列）与字段块的下沿（56u − 1）**。

夹具此前用库存尺（`ui.LightTheme().Spacing = 4.0`）画帧、却钉参考的 `u` 数。参考的 `u` 在
出货步子上是 `Spacing/4 = 0.9`，于是同一句话在两个坐标系里：`54u` 是 48.6 还是 54，
`56u − 1` 是 49 还是 55，取决于夹具装的是哪张调色板。

## 2. 实测（本轮自己跑出来的）

| 量 | 库存尺（此前钉的） | 出货尺（app 真正画的） |
|---|---|---|
| 字段行文字列 `16u+28u+10u` | `x = 54.0` | **`x = 48.6`** |
| 字段块高 `56u` | `56`（`14 × 4.0`） | **`50.4`**（`14 × 3.6`，落行 50） |
| 字段块底（块的最后一个像素） | `y = 55` | **`y = 49`** |
| 列表裁剪上沿 `56u + 4u` | `60` | **`54`** |
| 十行帧高 | `488` | **`439`** |

图 `/tmp/floter-P13-dr-field-{light,dark}.png`（720×439）上用眼睛复核到的一处变化：
**闭合字段块的那条发丝线从 y=55 上移到 y=49**，且两套外观一致 —— 浅色 `y=49` 读到
`(232,232,234)`、邻居 `(246,246,248)`；深色读到 `(28,28,29)`、邻居 `(15,15,16)`。
帧高也从 488 变成 439（这一半是 P13-dq 的尺子第一次真正用在字段这一面上）。

## 3. 落地

- `view_test.go`：`designUnit()`（`Spacing/4` = 0.9，参考的 `u` 唯一定义点）、
  `renderShipped(t, a, dark)`（装 `a.tokens(c).Theme` + `themedFrameHeight`）、
  `tenths(v)`（把落点读成十分之一像素的整数：float32 的落点与 float64 的乘积比**同一个数**，
  不是「差不多」—— 因此没有容差带）。
- 四个 pin 换尺：`TestTheFieldAndTheRowsShareTheReferenceTextColumn`（caret + 首行标题）、
  `TestTheFieldBandIsThePanelsOwnSurface`（块底 + `band == panel`）、
  `TestNoRowScrollsUnderTheFieldBand`（列表沿）、`TestTheFieldBlockClosesOnOneWeakHairline`
  （像素级块底，浅/深两条子测）。
- 新锚点 `frameruler_test.go:TestTheReferenceUnitIsNineTenths`：钉 `0.9 / 48.6 / 50.4 / 49`。
  谁把 0.9 写回 1.0、或把 48.6 写回 54，这里先红，不必等三个像素断言在三个文件里一起漂。

## 4. 台账

- **V = 4**（四条既有 pin 重定基线，全在 §2 的表里，注释写「P13-dr 重定基线：旧值 → 新值」）；
  **B = 0**；**S = 1**（`TestNoRowScrollsUnderTheFieldBand` 的列表沿 60 → 54：这是前提破了 ——
  夹具换尺后「边沿 = 库存字面量 60」不再成立，改成 `int((fieldRowUnits+breathUnits) × spacing)`，
  断言强度不降）。
- 新增 2 条测试函数（锚点 + opt-in dump），测试函数 2489 → **2491**。
- 产品代码 **0 改动**。

## 5. mutation（实跑，全部还原并逐字节核对）

| # | 注入 | 实测 |
|---|---|---|
| M1 | `designUnit()` 的 `return spacing / 4` → `return spacing`（「1u = 一个 Space 单位」这个经典单位错） | **红 2 个函数**：`the reference unit at the shipped step is 3.600px, want 0.90`、`the field row's text column is 194.4, want 48.6 (54u × 0.9)`、`the field's caret starts at x=48.6, want the rows' title column 194.4` |
| M2 | `renderShipped` 删掉 `c.SetTheme(a.tokens(c).Theme)`（夹具回到库存尺） | **红 3 个函数、以旧值报错**：`the field's caret starts at x=54.0, want … 48.6`、`the field block's floor is at y=55, want y=49 (… fieldRow 50)`（浅/深）、`the field block's floor at y=49 is {253 253 254 255}, the same as its neighbours`（浅/深） |

还原：两次都 `cp` 回原件，`md5sum -c /tmp/P13dr-view.md5` → **成功**；之后 `touch view_test.go`
再复跑（避免测试缓存复用）→ `ok`。

**M1 的盲点照实登记**：`TestTheFieldBlockClosesOnOneWeakHairline` 在 M1 下**不红** —— 它读的是
`designMetrics()` 的 `spacing` 而不是 `designUnit()`。这不是漏洞而是两条独立的式子（块高是
`Space(14)`，文字列是 `54u`），但本 pin 因此只钉「块」，不钉「单位」；「单位」由锚点那条钉。

## 6. 门槛（实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 全绿（FAIL 包 0）
go test -count=1 -v ./... | grep -c -- "--- PASS" → 3325（基线 3324）
GOOS=linux/windows/darwin go build ./... → 过
测试函数 → 2491（基线 2489，+2）
SKIP → 156（基线 153；本轮新增 1 条 opt-in dump `TestDumpP13DRFieldRowPNGs`，
       其余 2 条是环境条件 skip，本轮没有新增任何 `t.Skip` 语句 —— `git diff` 可证）
```

## 7. skill 复核（`qiaomu-design`）

读：`SKILL.md` → `references/user-preferences.md` → `craft-loop.md` → `preflight.md`
（本轮只需这四份，没通读全库）。走「已有方向，只要打磨」车道，不出 HTML 预览、不等点选。

**偷了什么**（逐条）：

1. `craft-loop.md` 工艺密度第 3 项（**尺子/单位先于像素**）：落成 `designUnit()` —— 参考的 `u`
   从今往后只有 **一个** 定义点（`Spacing/4`），四个 pin 全部乘它，不再各自写像素。
2. 同菜单第 6 项（**一个判据只许有一个来源**）：落成 `renderShipped` —— 「出货尺的帧」+「app 自己的
   调色板」是同一件事，夹具里不再出现「装了一种主题、按另一种刻度算帧高」的组合。
3. 同菜单第 8 项（**读数要读同一个数量级**）：落成 `tenths()`：落点与期望值都读成十分之一像素，
   于是「同一列」是一个整数比较，不需要 ±0.5 的容差带（preflight 的「不许加容差」也由此满足）。
4. `preflight.md` 的 B（设计读取一致性）：本轮把「不一致」从一句话变成一张四行表（§2），
   并把剩下 6 条单位算术 pin 归属到具体表面写进 §6.87 的队列表。

**preflight 逐项**：A（反 AI 套路）过 —— 本轮无新增用户可见文案、无占位符；
B（设计读取一致性）**过（本表面）** —— 字段行的四条账现在读同一把尺子；
C（动效抽查）**N/A** —— 产品零改动，无动效可抽查；
D2（中文页正文 ≥ 14px）**仍未过** —— `tokens.Caption` 9.9px / 正文 10.8px 与「≥ 14px」的冲突
照旧（第 43 份证据），**需拍板**。

## 8. 仍待（P13-dr）

1. **下一格（首选）：`P13-dg-l-1b` = 剩下 6 条单位算术 V**，按表面分组做：
   - 历史行的 pin 控件 `20u → 18`（`powerfaces_test.go:TestTheRowPinSinksLikeTheSharedFieldControl`）；
   - 状态带 `30u → 27`（`numberband_test.go:TestHoldingTheModifierKeepsTheTerminalHint`，
     同文件的 viewport Y 也读库存 `Space`）；
   - 空页带顶 `60 → 54`（`emptystate_test.go:TestTheEmptyPageSaysSoUnderTheField`）；
   - 过滤条带（`filterband_test.go`）。
   每个都要先把它自己的夹具换到 `renderShipped`（本轮已验证这条路可行、且**不需要**改产品代码）。
2. **`P13-dg-l-2` = 5 条前提破 S**；**`-l-3` = 2 条混排 B**；**`-l-4`（产品墨）**：
   `outputsubtitle` 的 6 级。
3. **需拍板（沿用 + 第 43 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **抢救 stash 核对（第四十五次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   仍判定与 HEAD 无未接续内容，本轮未试合、未静默丢弃。
