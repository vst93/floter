# P13-ai 报告 · 框架的灰盘按钮：全仓最后四枚没有「光」的控件

**方向一句话**：`internal/` 与 `cmd/` 的**四枚** `ui.Button` 是应用里唯一还靠「四边一圈同值的
画框」分层的控件；这一轮把它们收成 E1，并把这个零写成一条 CI 账。

**这轮为什么让它更轻 / 更有空间感**：空间感来自「一块面朝哪边亮」。框架的按钮面**四条边一个值**
（浅色 108 / 深色 151），读起来是一圈等宽的画框——它说了「这里有边界」，但没说「这块比地面高」。
E1 的一对面说了后者：冠线行亮于面（浅 **+33** / 深 **+10**）、对侧行暗于面（浅 **−7** / 深 **−28**），
一个按钮第一次在这页上说「我在 z 上」。

---

## 一、本轮做了什么

| # | 位置 | 改动 |
|---|---|---|
| 11-1 | `internal/settingsui/rows.go`（新增 `a.pageButton`） | `ui.ButtonBase` + 框架几何逐字段照抄（`MinHeight` 30u / `Padding` 6u·14u / `Radius` 9u）+ 面 `Control`→`ControlHover`→`ControlPress` + 边 `ControlEdge` + `tokens.LitFace`（顶 `LensRim` / 底 `LensEdge`）+ 墨 `TextStrong`(600)→`Text`(600) + `theme.ColorStep` 120ms + 按下 `Scale(0.97)` |
| 11-2 | `settingsui.go:1529` / `:1631` / `:4745`、`custom.go:269` | 四处 `ui.Button(c, …).Clicked()` → `a.pageButton(c, …).Clicked()`；**回调体、i18n 字串、键盘路径一字不动** |
| 11-3 | `internal/ci/stockbutton_contract_test.go`（新） | 反向 census：非测试源码里任何 `ui.Button(` / `ui.PrimaryButton(` 都红并点名 file:line；豁免表**空**（行尾 `// stock-button: <理由>` 才放过） |
| 11-4 | `internal/settingsui/pagebutton_test.go`（新） | 两条像素账 × 深浅两套外观（见 §三）+ 一条 opt-in 出图 |
| 11-5 | `docs/liquid-glass.md` §6.8 | 新增小节，11-1…11-4 逐行落成值 |

## 二、先量再改（证据）

框架的样式写在 `mygo@v0.7.3/ui/widgets.go:152-186`：`Padding(1.5,3.5)`、`Radius(t.Radius)`、
`Background(t.Surface)`、`Border(1, t.Border)`、hover → `SurfaceHover`、press → `SurfacePressed`，
**没有 `InsetShadow`、没有 `FontWeight`**。改前实测（`lensProbe`，中灰 128 地面）：

| 外观 | 面（盒内中线行） | 顶边 | 底边 | 左边 | 右边 |
|---|---|---|---|---|---|
| 浅色 | 119 | 108 | 108 | 108 | 108 |
| 深色 | 141 | 151 | 151 | 151 | 151 |

改后（同一探针、同一读法；冠线行是 `LitFace` 的第一条内线，在边框内一行）：

| 外观 | 面 | 冠线行 | 对侧行 | 侧边线 |
|---|---|---|---|---|
| 浅色 | 119 | **152（+33）** | **112（−7）** | 112 |
| 深色 | 141 | **151（+10）** | **113（−28）** | 149 |

浅色对侧只有 −7 级，是 `LensEdge` 在浅色面上的**上限**（§1：抬它会把 accent 面洗成类玻璃边），
不是缺一步。**几何零改动**：盒子仍是 `y=20..49`、宽 **93u**。

## 三、V / B / S 台账

- **V 1**：`internal/ci/colorstep_contract_test.go` 的位置表 `internal/settingsui/rows.go` **3 → 4**
  （这一族新增了一条 `Colors: true` 的过渡，那张表的规矩就是「新增即红、改表才绿」——
  第一次全量跑正是这条把我自己拦下的，报错原文见 §五）。
- **B 0**：行为、高度链、键盘、i18n 一个字节没改。`ui.ButtonBase` 与 `ui.Button` 同一个基类
  （`coreButtonBase`：`Row().Center().Focusable().Shrink(0)`），Tab/Enter/Space 路径不变。
- **S 1**：新增 `internal/ci/stockbutton_contract_test.go`（一条新的结构账，豁免表为空）。

## 四、门槛（全部实测）

```
gofmt -l cmd internal                     → 空
go vet ./... / GOOS=windows / GOOS=darwin → 净（VET_OK）
GOOS=linux|windows|darwin go build ./...  → 过（BUILD_OK）
go test -count=1 ./...                    → 全绿
PASS 3036 → 3043（+7）| FAIL 0 | SKIP 89 → 90（+1 opt-in 出图）
测试函数 2241 → 2245（+4：ci 1 / pagebutton 2 / dump 1）
```

## 五、变异（实跑）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 删掉 `pageButton` 里的 `tokens.LitFace(button)` | **红**：`the lit row is only 0 levels off the face ({119 119 119} vs {119 119 119}): the face's lit line is gone` + `the counter row … is not darker than the face`（深浅两套都红） |
| M2 | `LitFace` 换成四边 `InsetShadow(…, ControlEdge)`（恢复「画框」） | **红**：浅 `lit=112 / counter=112 / left=112 / right=112`（四边同值，正是改前的状态）；深 `the counter row {149} is not darker than the face {141}` |
| M3 | 字重 600 删掉（退回框架默认字重） | **不红**——如实记录：字重没有像素 pin，四处的 label 都是短词，像素账读的是面不是字形。要钉它得先有「同一句话在 400/600 下的墨量差」这条账，本轮没开 |
| M4 | `custom.go:269` 退回 `ui.Button(c, copy.ShortcutsBind)` | **红**（CI 账点名）：`these controls wear it: internal/settingsui/custom.go:269` |

每条变异后 `md5sum -c` 逐字节核对还原并 `touch` 复跑（避免缓存复用），复跑全绿。

**另有一次「非计划红」也照实记**：新增 `pageButton` 后第一次全量跑，
`TestEveryColorTransitionRidesOneStep` 红在 `internal/settingsui/rows.go`（位置表 3 ≠ 实测 4）——
这不是产品回归，是 P13-ad 那张表的设计意图（新增颜色过渡必须改表），已按 **V** 处置。

## 六、出图

`/tmp/floter-P13-ai-sessions-{light,dark}-{before,after}.png`（`FLOTER_PNG=1` 的 opt-in dump，
会话页空态——这四枚里干活最重的一枚就在那儿：它是 160u 空白里唯一的控件）。

像素 diff：**浅 867 px / 深 865 px**，bbox **`(399,228)-(491,257)`** = 93×30，
**恰好是按钮自己的盒子**，峰值 202（字重 400 → 600 重排的字形抗锯齿）。页面上其它一切
（标题、空态图标、hint、右侧栏）**一个像素都没动**。

**我在图上看到的一处变化**：那枚「新会话」的**上沿多出一条比面亮的线、下沿多出一条比面暗的线**，
左右两侧没有——同一个盒子上第一次有「光从上面来」；同时盒内的字从细变成了 600（浅色图里
`New session` 的笔画明显更实）。

## 七、偷了什么（qiaomu-design / craft-loop 工艺密度菜单）

- **第 12 项「材质细节」**（内边框高光 / glass 折射边）→ 落成 `tokens.LitFace` 的两条 1px 内线：
  冠线行 **浅 152 / 深 151**，对侧行 **浅 112 / 深 113**。这是本轮唯一的「材质」，没有渐变、
  没有模糊（§0 硬禁令 1/3）。
- **第 5 项「多层染色阴影」**→ 这一轮**只用了它的反面**：四枚按钮**不**加外投影（§2 判据 2：
  E1 不许有影），边界靠面差 + 一对内线——把「层」留给真正浮起来的纸。
- **第 6 项「编辑细节」**（hairline）→ 边墨从框架的 `stroke`（0.09/0.09）换成这一族自己的
  `ControlEdge`（0.06/0.07），与同页其它胶囊同一条线。
- 反目标（没偷）：第 3 项品牌 focus 环、第 4 项签名交互、第 8 项流体字号——功能页、高频点击，
  加这些会把「轻」变成「闹」（preflight C：高频/键盘触发操作不加动画）。

## 八、preflight 逐条（A / B / C / D2）

- **A 反套路**：无渐变白边（§1 禁令 3 明确禁的「类玻璃边」正是我们**没有**做的）、无紫蓝渐变、
  无纯黑、无外发光、无斜体、无三等分卡片、无假数据文案。**过**。
- **B 设计读取一致性**：一页一个主题（探针深浅各一套，不混）；记忆点 = 「这页上每一块面都朝上亮」；
  动效气质 = 120ms 颜色 + 0.97 收档，与同页其它胶囊同一句。**过**。
- **C 动效抽查**：颜色过渡走 `theme.ColorStep`（120ms ≤ 300ms，无 `transition: all`、无 `ease-in`、
  无 `scale(0)`）；按压有 `:active` 档（`ControlPress` + `Scale(0.97)`）；高频鼠标操作用 transition
  不用 keyframes；`ReduceMotion` 由框架的 `ui.Tester`/主题兜底。**过**（唯一不适用的条目是
  「hover 包在 `@media (hover:hover)`」——自绘没有 CSS 媒体查询，指针态由 `Hovered()` 表达）。
- **D2 中文页**：本轮不新增文案（四处 label 全来自既有 `i18n.Settings`），中文行高/字重档
  （600 在 400/500/600 允许集内）/ 中西文间距均未改动。**过**。

## 九、仍待（写进 `.pending-bugs.md`）

1. **需拍板**：`SettingsNew` 这类「页级动作」要不要抬成 accent（原版更新按钮那条路），
   还是保持现在的安静胶囊？现在按 §2 的 E1 处理（安静），因为一页可以有多个页级动作。
2. M3 的空白：字重没有像素账。要钉得先决定「同一句话 400 vs 600 的墨量差」这条断言该量哪一列。
3. 框架的 `ui.Button` 仍被允许在**测试**里用（census 只扫非测试源码）——这是有意的：
   dump/probe 有时要一个「随便的按钮」对照。
