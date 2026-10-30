# P13-r 报告 —— 凹槽里的那一格：浅色档的「抬起」方向反了

## 1 意图

设置页与终端抽屉共用的分段控件（`control.SegmentedIndex`）里，**选中那一格**在浅色档
比它自己的凹槽还暗 23 级——同一枚控件在深色档说「这是抬起来的」，在浅色档说「这是挖
下去的」。这一轮给它一个自己的 token（`theme.Tokens.Thumb`），浅色档改成**纸自己的
颜色**，深色档一个字节不动。

## 2 为什么现在做（证据）

`docs/liquid-glass.md` §6 没有 `未开始` 行；`Lens` 只剩 1 个调用点（`control.SegmentedIndex`），
且 §6.2b 9-11 已判 `故意不做`——但那一行判的是**透镜**（`Lens` ↔ `LitFace`），
判不到**填充**。按纪律转通用队列的「层级 / 空间感」格，先量再改。

量在真 `SegmentedIndex` 上（画在设置页的分组卡 `Control` 面上，不是裸页面）：

| 外观 | 页面 | 卡面 | 槽面 | 选中格（旧） | 抬高量 |
|---|---|---|---|---|---|
| 浅色 | 247 | 230 | 216 | **193** | **−23** |
| 深色 | 39 | 61 | 78 | 103 | +25 |

根因不是 accent 环，是填充借了 `tokens.ControlHover`：浅色档的「抬起」是黑洗
（`0,0,0,0.105`），压在凹槽面 216 上得到 193；深色档的「抬起」是白洗
（`255,255,255,0.14`），压在 78 上得到 103。**同一个 token 在两种底上指向相反的 z 方向。**

而「凹槽里更深的一格」正是用户 2026-10-30 点名的 macOS 10.x 读法（「有点偏 macOS 旧
版本，空间感不够」）；现代 macOS/iOS 的分段滑块是**纸色抬起**。

## 3 落地

| # | 位置 | 改成 |
|---|---|---|
| 1 | `internal/theme/theme.go` | 新 token `Thumb`：浅 `rgba(255,255,255,0.85)`（在卡面的凹槽 216 上复合到 **249**，页面 247 ±2 —— 抬起的面就是那张纸穿过了凹槽）、深 `rgba(255,255,255,0.14)`（= 现值，外观不变）。`palette` 加 `thumb`，`Tokens` 加 `Thumb`，两套外观各写一条带理由的注释 |
| 2 | `internal/control/control.go` 选中臂 | `Background(tokens.ControlHover)` → `Background(tokens.Thumb)`。`Lens` 四层与 `AccentEdge` 环**不动**（9-11 的理由照旧：环是这一格的身份线，且它盖住 rim/edge） |
| 3 | `internal/control/thumb_test.go`（新，4 条） | ①两套外观下选中格比槽面亮 ≥10 级；②浅色档选中格落在纸色 ±3 且高于卡面；③accent 环相对选中格面仍然「是一条蓝线」（蓝−红 差 ≥25）；④`FLOTER_PNG=1` 的 dump |
| 4 | `docs/liquid-glass.md` §6.2b 9-11 + 新 §6.2b-5 | 9-11 拆成「透镜：故意不做 / 填充：已落地」，§6.2b-5 写清根因、新旧值、实测表与 pin |

**零改动红线**：只改一个 token 的墨与一个 `Background` 调用。高度链（段高 30u）、水平
度量（`inset` 9u/6u）、`Lens` 四层、`AccentEdge` 环、`Padding`/`MinHeight`/`Radius`、
标签墨（`TextStrong` / 580）、点击回调、键盘模型、i18n 文案：**一个字节没动**。
深色档的渲染前后 **diff 0 像素**（下面第 5 节）。

## 4 门槛（实测）

```
gofmt -l cmd internal                空
go vet ./... / GOOS=windows|darwin    净
go test -count=1 ./...                全绿（0 FAIL）
PASS 2963 → 2970（+7）  SKIP 74 → 75（+1 dump）  测试函数 2180 → 2184（+4）
GOOS=linux|windows|darwin go build    过
```

**V 0 / B 0 / S 0**：选中格的填充此前没有任何像素 pin；深色档值逐字节不变，
所以没有一条既有断言需要改。`control.TestTheSegmentedControlCarriesTheStopLens`
（透镜随 stop 单调上升）在本轮之后仍然全绿——`Thumb` 不是透镜，它不参与
`--glass-lens-scale` 那条梯子。

## 5 出图（自己复核过）

`/tmp/floter-P13-r-segment-{before,after}-{light,dark}.png`（420×140，真 `SegmentedIndex`
画在卡面上，中间格选中；before 由注入态实拍）。

- **这张图上看到的一处变化**：浅色档选中那一格从「槽里更深的一块」变成「一张白纸穿过
  了凹槽」——它现在比槽面亮 33 级、比卡面亮 19 级，而页面是 247，于是这一格读起来
  是**纸本身被抬起来**，accent 环仍旧框住它。
- 差异 bbox：浅色 `(150,29)-(269,55)`，**恰好是中间那一格自己的盒子**
  （段宽 122.1，中间段 148.8~271.2），控件之外一个像素没动。
- 深色档 diff = **0 像素**（外观按设计不变）。

## 6 mutation（4 条实跑，全部变红并逐字节还原）

- **M1** 选中臂退回 `tokens.ControlHover` → `TestTheChosenSegmentRisesOutOfItsGroove/light` 红：
  `[light] the chosen slot is -23 levels above its groove ({193 …} over {216 …})`
  （同时 `TestTheLightThumbIsTheSheetColour` 红：`54 levels off the page's own colour`）。
- **M2** 浅色 `thumb` 改成 `rgba(0,0,0,0.10)`（黑洗）→ 同两条红：`-22 levels`、`53 levels off`。
- **M3** 删掉 `InsetShadow(0,0,0,1, tokens.AccentEdge)` → `TestTheChosenThumbKeepsItsAccentRing/light` 红：
  `the top accent ring is only 0 levels bluer than the chosen slot's face ({230 …} against {249 …})`。
- **M4** 浅色 `thumb` 抬到 `rgba(255,255,255,0.99)` → `TestTheLightThumbIsTheSheetColour` 红：
  `the chosen slot is 8 levels off the page's own colour ({255 …} against {247 …})`。
  这一条就是本轮的第二轮：第一轮更白的档把纸色顶成 255（比纸高 8 级 = §1 禁令 3 的
  「类玻璃边」），于是退回 0.85（249，±2）。

四条都 `cp` 备份 → 注入 → 看到红 → 还原 → `md5sum` 逐字节相同。

## 7 用了 qiaomu-design（偷了什么）

读：`SKILL.md` → `references/user-preferences.md`（P-5 可读性优先、P-1 禁斜体、
P-14 不常驻系统自言自语）→ `craft-loop.md` + `preflight.md`。本轮表面是自绘桌面 UI、
方向已定死（轻便液态玻璃），走「已有方向，只要打磨」那条车道：**不出四方向预览页、
不等点选**。

- **craft-loop 工艺密度第 12 项（材质细节：内边框高光 / glass 折射边）**：选中格不是
  「换个颜色」，而是**换一档材质**——`Thumb` 是「纸穿过凹槽」，落成具体数值
  `rgba(255,255,255,0.85)`，在卡面的凹槽（`Control` 230 → `Track` 216）上复合到 249，
  与页面 247 相差 ±2。深色档同一条规则解出另一个值（0.14，因为深色没有更亮的纸可借）。
- **craft-loop 工艺密度第 3 项（品牌 focus 环）**：这一格的身份线是 `AccentEdge` 环，
  本轮**保留并加了反向 pin**——浅色档把面洗到 249 之后，环必须仍然是「一条蓝线」
  （蓝−红 差 ≥25；实测浅色 58、深色 31）。这一条是 P-5 可读性优先的直接落地：
  身份线不能被更亮的面吃掉。
- **craft-loop 第三节「两轮制」**：第一轮落地后按 M4 试了更白的档（0.99 → 255），
  评审判「比纸还亮 8 级 = 发光面」→ 第二轮退回 0.85（249）。两轮都留了实测数字。
- **没偷的**：本轮**没有**从 58 站 DESIGN.md 取 DNA 供体（A2 第一条）。理由写在这里：
  这一轮改的是一枚 30u 原生控件里的一格填充，几何、字体、布局、阴影全都不许动
  （行为契约红线），可偷的只有「分段滑块用纸色」这一条平台约定——它来自 iOS/macOS
  的控件本体，不是某个站点的设计语言。**这是本轮明确没做到的一条**，不算过。

## 8 preflight（逐条）

- **A 反套路**：无渐变白边（`Thumb` 是一条实色 alpha 面，不是 gradient）、无外发光、
  无纯黑、无斜体（P-1）；没有新增文案（P-14 不受影响）；没有用左侧竖线表达选中（P-38）。**过。**
- **B 设计读取一致性**：一页一个主题（深浅各跑一遍 pin）；记忆点 = 「选中的那格是纸」；
  材质与动效气质一致（只有颜色，无新增动效）。**过。**
- **C 动效抽查**：本轮不新增动效（`Thumb` 只参与既有的颜色过渡，90ms 只动颜色）；
  无 `transition: all`；按下/悬停两档（P13-q）不受影响。**过。**
- **D2 中文页**：没有改任何中文文案（红线），中西文空格、全角标点、行高、字号一个
  字节没动。**过（不适用项已注明）。**
- **A4 功能契约**：分段控件的契约是「选中的那个就是选中的」——此前浅色档靠一条环
  说这句话，面在唱反调；现在面与环说同一句话。**过。**
- **A2 进攻三步**：DNA 注入**未做**（见第 7 节，已写明理由）；工艺密度 2 项（第 3、12 项，
  逐项指认了位置与数值）；两轮制**做了**（M4 → 0.85）。**部分过，缺口已注明。**

## 9 仍待（追加在 P13-q 之后）

1. **`Thumb` 在浅色档的 ±2 是「在卡面上」的读数**：分段控件在终端抽屉里画在浮层
   （`Float`）上，那里的槽面比 216 亮，选中格会跟着上移到 ~250 以上（比纸高）。
   要不要给浮层里的 `Thumb` 另算一档，属于**改 token 的取值规则**，需拍板（本轮不改，
   pin 只钉设置页的卡面）。
2. **P13-q 第 1 条原样有效**：浅色 `ControlPress`（黑 0.032）比 `Control`（黑 0.018）更亮，
   要改就是改 token 基线，**需拍板**。
3. **P13-p 第 1/2 条、P13-k 第 1/2 条、P13-c 四条、P13-h 第 9 条**原样有效（均需拍板）。
