# P12-c 报告：设置页表面——E1 的受光边与 E2 的冠线落到这一张表面

**一句话**：这一轮没有新增任何功能，只是把设置页的四类面（侧栏选中页、分组卡、字段、
弹层字段槽）从**一块平涂**变成**一块站起来的 E1**，并把两张 `Float` 面（卸载对话框、
保存失败告警）的冠线从 E1 的 `LensRim` 换成 E2 的 `EdgeHighlight`。

## 为什么让它更轻、更有空间感

旧语言里设置页的每一块面都只有**一个填充**和**一圈 1px 描边**：填充是平涂，描边是画框。
用户读到的是「所有东西挤在同一个 z 平面上，靠 1px 描边分家」——这正是任务书里
「平、糊、旧」那句话在设置页上的具体样子。现在每一块 E1 都是「面 + 顶受光边 + 底对侧暗线」
三件事，块与底的 z 关系是**读出来的**；而两张浮层的冠线换了档之后，
「贴着的块」与「浮起来的纸」第一次有了不同的边——**同一条规则在两层上给出两个值**，
空间感就是这么来的，不是靠加层数。整张表面仍然只有一条亮线（`LensRim ≤ 0.09`），
没有第二层玻璃、没有渐变白边、没有模糊贴图。

## 落地

`theme.Tokens.LitFace`（P12-b 新增：顶 `LensRim` + 底 `LensEdge`，无投影）这一轮落到
设置页的四个位置；两张 `Float` 面换成 E2 的冠线。

| # | 位置 | 文件 | 改动 |
|---|---|---|---|
| 2-1 | 内容页 `Sunken` | `settingsui.go` | **不动**：它是底（E0），底不画面、不画线 |
| 2-2 | 侧栏选中页 | `settingsui.go` `sidebarRow` 的 `selected` 臂 | `row = tokens.LitFace(row)` |
| 2-3 | 分组卡 | `rows.go` `cardGeometry` | `card = tokens.LitFace(card)`（外圈仍是 `HairlineSoft`） |
| 2-4 | 设置页字段 | `textinput.go` `fieldFace` | `tokens.LitFace(e.Background(Control).Border(1, ControlEdge))` |
| 2-4 | 弹层字段槽 | `pluginsheet.go` `sheetFieldSlot` | 同一个 `tokens.LitFace`，不另起数值 |
| 2-5 | 开关 / 分段控件凹槽 | `rows.go` `switchControl` | **不改代码**；`LensEdge` 已随 P12-a 到 0.06，本轮加**反面 pin** |
| 2-6 | 主 CTA 胶囊 | `rows.go` `extensionPrimaryButton` | **不改**：它本来就戴着 `LensRim`，且 pin 现在钉住「是 E1 的线、不是 E2 的」 |
| 2-7 | 卸载对话框 | `uninstall.go` `uninstallDialog` | `LensRim` → **`EdgeHighlight`**（两层影 `ElevThrow` 已在场） |
| 2-8 | 保存失败告警 | `savealert.go` `saveAlert` | 补 `EdgeHighlight` 冠线；**不加影**（见下） |
| 2-9 | 状态 chip / 权限药丸 | — | `故意不做`（4px 高，加一条线就成噪点，P12-a 已定） |

**2-7 为什么必须换**：那张面板的面是 `Float`、带着 `ElevThrow` 的两层影、还能按 Esc 关掉
——§2 三条判据里的第三条全部成立，它就是 E2。E2 的冠线是 `EdgeHighlight`
（深 0.20 / 浅 0.92），E1 的是 `LensRim`（0.09）。此前它戴的是**组件级**那条线，
于是「浮起来的纸」和「贴着的块」是同一个边——这正是用户说的「空间感不够」。

**2-8 为什么只加冠线不加影**：这条带画在页面列里、随内容一起滚（`body()` 的 `Scroll`
内容），不是固定在窗口上的浮层。`ElevThrow` 的 6.5u 宽投影会在滚动视口与列的内边距上
被裁成一条硬边——那就是 §0 硬禁令 1 的同类错误：用一个画不完整的影冒充「浮起来」。
把它做成真浮层是**布局与交互改动**（不在这支的红线内），已写进 `.pending-bugs.md`
的「需用户拍板」。

## 图上看到的一处变化（自己复核）

`/tmp/floter-P12-c-general-dark-{before,after}.png`（720×900），侧栏选中页那一列 `x=130`：

```
y=69   (顶边上一行)  27,27,29  →  32,32,34
y=70                  35,35,37  →  47,47,49
y=71   (受光顶边)    37,37,39  →  51,51,53    ← +14 级
y=101  (对侧暗线)    35,35,37  →  34,34,36    ← 底暗一分
y=102                35,35,37  →  34,34,36
```

浅色档同一列：`y=69` 244→245、`y=99` 244→**235**、`y=102` 250→247——顶边只抬 1 级
（白压白），站起来的是**底对侧**那 9~15 级。这是 P12-b 已经量到的同一条结论，
在设置页上又一次成立。

分组卡（同一帧 `y=178`，卡的第一行）：深 55→**61**、浅 226→227（顶冠）；
卡的最后一行 `y=516`：深 55→61 之前是 226→**222**（浅，底暗）。卸载对话框
`/tmp/floter-P12-c-dialog-dark-*.png` `y=21`：`101,102,104` → `108,109,110`
（E1 的 0.09 换成 E2 的 0.20，同一条线亮一档）。保存告警
`/tmp/floter-P12-c-banner-dark-*.png` `y=20`：`24,24,27` → `34,34,37`，`y=21`
→ `51,51,55`（冠线此前根本不存在）。

## 偷了什么（qiaomu-design 的工艺密度菜单）

从 `craft-loop.md` 的工艺密度菜单取了 **3 项**，逐条落成具体数值：

1. **第 12 项「材质细节：内边框高光」** → `LitFace` 的顶边 `LensRim` 0.09 白、
   底边 `LensEdge` 0.06 黑。按用户偏好账本 P-38 与合约 §0 硬禁令 3 的约束，
   **不是 CSS 那种渐变白边**（那正是用户说的「类玻璃边」），而是一条 1px 实色线。
2. **第 5 项「多层染色阴影」的反向用法** → 这一支**禁止** E1 画影（§2 判据 2），
   于是把「多层」改成「成对」：顶亮 + 底暗两条 1px 线，仍是一对而不是一层。
   落成：`InsetShadow(0, 1, 0, 0, LensRim)` + `InsetShadow(0, -1, 0, 0, LensEdge)`。
3. **第 3 项「品牌 focus 环」的降级用法** → 焦点环在本仓是 `EdgeHighlightActive`
   （已在 `shell/cardedge.go`），而这一轮的贡献是**给「层级」也定了一条边**：
   E1 用 `LensRim`（0.09）、E2 用 `EdgeHighlight`（0.20）。同一条规则两个值，
   与焦点环的「静息/聚焦」是同一个思路——**用亮度回答「在哪一层」**，
   而不是用第二条描边。

`craft-loop.md` 的「两轮制」也走了：第一轮按 `LitFace` 直接落四个面 → 读图发现
浅色档的顶边只有 1 级、块读不出来（艺术总监视角那处）→ 第二轮把断言改成
「拿同一个面画两条候选线当参照、看被量到的线离哪条更近」，并把「底对侧」写成
与顶边同等的断言（工程师视角那处）。两轮都发生在这一轮次内。

## preflight 逐条结论

- **A 反 AI 套路**：无渐变文字、无紫蓝渐变、无外发光、无三等分卡片、无 italic、
  无左侧竖线装饰（P-38）。**不过的一条**：A 节要求「字体不是系统默认」——本仓是
  原生桌面应用，字体跟随系统（`docs/raycast-design.md` §2 明写），**故意不做**，
  理由：桌面应用换掉系统字体是可访问性与一致性倒退。
- **A2 进攻三步**：DNA 注入（借 Apple HIG「材质只在需要分层处出现」，落成「E1 不画影、
  E2 才画两层」）、工艺密度 3 项（见上）、两轮制（见上）。**A2 的「≥5 项」不过**：
  这是桌面应用的一个表面，不是一整页；把 5 项塞进一个 44u 的卡只会变成噪点
  （`craft-loop.md` 自己的约束「密度不等于吵闹」）。
- **A4 功能契约**：过。没有动任何交互：点侧栏仍切页，卡里的行仍可点，字段仍写回同一个
  store，卸载对话框仍按 Esc 关。
- **B 设计读取一致性**：过。读取是「功能型桌面应用设置页 / 面向键盘重度用户 /
  轻便液态玻璃」，拨盘 VARIANCE 3、MOTION 2、DENSITY 7——本轮没有任何动效，
  也没有任何冒险装饰，与读数一致。
- **C 动效抽查**：过。这一轮**没有新增任何动效**；已有的 120ms 行颜色过渡与
  保存告警的 160ms 入场未改。无 `transition: all`（本仓不是 CSS）、无 `scale(0)`、
  无纯 fade 入场。
- **D2 中文页**：本仓 UI 有中文文案，但这一轮**没有动任何文案**；新增的只有代码注释。

## V/B/S 台账

| 类 | 条数 | 说明 |
|---|---|---|
| **V**（视觉数值重定基线） | **0** | 没有一条旧 pin 变红。设置页原有两条「无透镜」pin（`TestTheSidebarSelectionIsTheNeutralRowSelected` 的「top rim 就是 fill」与 `TestTheGroupedCardWearsTheContractsFaceEdgeAndCorner` 的「top inner row 就是 face」）**本来就是反面断言**，它们钉的是「不许戴 `Lens` 的四层透镜」。`LitFace` 是**两**层内线，且两条 pin 的采样点（`Y+2` 与 `edgeY+2`）落在冠线**下面一行**，因此两条都仍然绿——这正是这一支的设计：`Lens` 与 `LitFace` 不是一回事，旧 pin 守的是前者。 |
| **B**（行为/度量/接线） | **0** | 其余全绿，测试零改动 |
| **S**（结构） | **0** | 无结构变化 |

新增 pin **5 条函数 / +17 PASS**（`internal/settingsui/liteedge_test.go`，每条在深/浅两套
外观上跑子测试）：`TestTheChosenPageStandsOnOneLitEdge`、`TestTheGroupedCardIsAnE1Pane`、
`TestTheSettingsFieldIsOneLitSlot`、`TestTheRecessedGrooveHasNoCrown`（反面）、
`TestTheFloatingPanesWearTheE2Crown`（对话框 + 告警两条子测试）。

**为什么 pin 不写死合成后的像素值**：`liteMeasure` 把同一个面、同一个底色、两条候选线
各画一遍当参照，然后要求被量到的冠线离它「应该」那条更近（`liteNearest` 还带
`discriminating` 式的 canary：两条候选线合成到同一个像素时直接 `Fatal`，说这条 pin
已经没有主体了）。这样一条 0.09 的线与一条 0.20 的线在两套外观上都分得开，
而 pin 里没有一处写死渲染器算出来的值——写死的那些会在下一次渲染器微调时变成噪声。

## 门槛（全部实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                    → 全绿，0 FAIL
go test -count=1 -v ./... | grep -c "--- PASS"  → 2836（基线 2819，+17）
SKIP                                      → 61（基线 60，+1：本轮新增的 opt-in 截图 dump）
测试函数                                   → 2095（基线 2091，+4：五条 pin 里两条是子测试，
                                            再加一条 opt-in dump）
GOOS={linux,windows,darwin} go build ./... → 过
```

## mutation（5 条实跑变红 + `md5sum -c` 逐字节还原）

| # | 注入 | 实测报错（原串） |
|---|---|---|
| M1 | 侧栏选中页删掉 `LitFace` | `liteedge_test.go:212: [light] the chosen page's crown {232 232 233 255} is not brighter than its own face {232 232 233 255}: the lit edge is missing (E1 = face + one 1px LensRim, liquid-glass.md §2)`（深浅两档都红） |
| M2 | 分组卡删掉 `LitFace` | `liteedge_test.go:299: [light] the card's crown {237 237 237 255} is not brighter than its face {237 237 237 255}: the E1 crown is missing (liquid-glass.md §6.2 第 2-3 行)` + `[dark] the card has no counter-side: its bottom inner row {26 26 26 255} is not darker than its face {26 26 26 255}` |
| M3 | 卸载对话框 `EdgeHighlight` 换回 `LensRim` | `liteedge_test.go:519: [light] the dialog's crown {216 216 217 255} is nearer the E1 `LensRim` ({219 219 220 255}) than the E2 `EdgeHighlight` ({252 252 252 255}): a `Float` pane wears the floating rim`（深浅两档都红） |
| M4 | 保存告警删掉冠线 | `liteedge_test.go:578: [light] the save banner has no crown: its crown {198 198 199 255} is not brighter than its face {198 198 199 255}`（深浅两档都红） |
| M5 | 开关的 `LensInset` 换成 `LitFace` | `liteedge_test.go:451: [light] the switch's groove has a lit crown ({14 80 169 255} over its own track {6 74 166 255}): a recess is not a raised face, `LensInset` lights the bottom` |

五条都用 `md5sum -c` 逐字节核对还原（六行全部「成功」），`touch` 后复跑全绿。

## 仍待

1. **下一轮 = `docs/liquid-glass.md` §6.3 终端页**（6 行，其中 2 行 `故意不做`）。
2. **2-8（保存告警）是「半个 E2」**：冠线到位、两层影没有。做成真浮层要动布局
   （它现在在 `body()` 的滚动列里），已写进 `.pending-bugs.md` 的「需用户拍板」。
3. **§6.2 第 2-6 行（主 CTA 胶囊）本轮只加了 pin、没改代码**：它本来就戴
   `LensRim`，`Lens` 的四层透镜里第一层就是它。E1 的判据是「面 + 一条冠线 + 一条
   对侧暗线」，而主 CTA 有 accent 填充，对侧暗线在 accent 上读不出来（会被色相吃掉）
   ——**这一行如果要补对侧，需要拍板**，因为它是 accent 面上第一条非 accent 的线。
4. **P12-a「仍待」第 2 条的 `Float` 档固定 +0.10** 仍未动：它属于 §6.6 弹层。
