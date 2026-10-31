# P13-ay 报告：启动器的七处提示戴上浮面（§6.20 的 20-2），顺带补回被吃掉的那 1.5u

规格见 `docs/rounds/P13-ay-spec.md`。合约 `docs/liquid-glass.md` **§6.20 的 20-2 由
`未开始` 转 `已落地`**（20-3 设置页 10 处仍是 `未开始`）。

## 这轮为什么让它更轻、更有空间感

启动器是**同一扇窗里唯一还在用框架系统提示的表面**：七处提示是 `inverse()` 的反色平涂
（本仓没设 `ui.Theme.Inverse`，退回 `Text`），深色档实测就是一枚近白片子（level 244）贴在
深玻璃上；而终端页（P13-ax）已经是 `Float` 面 + 受光边 + 两层影的浮层。现在一个产品里只有
**一种**提示，而且它和抽屉/对话框/便条说的是同一套 z 语言——再加上这一轮补回的 1.5u 缝，
它读成「浮在标注物上方的一张纸」，不是「贴在标注物上的一块牌子」。

## 落地

1. **七处全迁 `theme.Tokens.Tip`**（E2：`Float` 面 + `RadiusSM` 8 + `InputStroke` 发丝 +
   顶边 `EdgeHighlight` 冠线 + 两层影）：
   `launcher.go` 的 `rowControl`（行尾 20u 控制格）、冲突警示点、空格键帽、来源词、历史行星标、
   `fieldSettingsButton`（字段行尾三枚 28u 键），`output.go` 的捕获输出页头两枚方键。
   链式写法拆成「先建、再 `tokens.Tip(c, e, hint)`」，`Label`/`Clicked`/`Size`/命中框一字未动。
2. **补回锚点间距**：`TooltipBase` 的默认 attach 是**贴住**锚点的（`ui/tooltip.go:105-107`），
   那 1.5u 是框架 `Tooltip` 自己写的（`ui/tooltip.go:132`）。P13-ax 只搬了材质、漏了这行，
   所以**它落地的三处终端页提示一直是贴着锚点的**。这轮在 `tokens.Tip` 里补上
   `AttachTo(anchor, AnchorTop, AnchorBottom).Margin(0, 0, Space(1.5), 0)`，八处一起修好。
3. **行为零改动**：0.6s 静息、0.4s 热接力、Esc、`Description`、80u 上限、锚点方向全部沿用
   框架；i18n 零改动。

## 读数（实测）

| 项 | 深色 | 浅色 |
|---|---|---|
| 改前提示面（框架反色片子） | **244/244/245**（level 244，近白压在深玻璃上） | **24/24/27**（level 25，近黑） |
| 改后提示面（`Float` 压在内容上） | **33/33/35**（level 33） | **246/246/247**（level 246） |
| 改后提示面（警示点那条，视口 107×17 / 121×30） | 33/33/35 | 246/246/247 |
| 冠线（深色，图上读到） | 顶行 **58/58/60** 压面 24/24/27 | 不断言（0.92 近白压 252 近白面 = 白压白，同 §6.20） |
| 与锚点的缝（平地 harness，1.5u = 6.0px） | **5.0px** | **6.0px** |
| 缝（改前 `Tip`，M3） | **2.0px** | **2.0px** |
| 框架 `.Tooltip(` 残留（launcher 非测试文件） | **0** | — |
| `tokens.Tip(c, …)` 处数 | **7**（§6.20 20-2 逐处点名） | — |

出图：`/tmp/floter-P13-ay-{direct-keycap,command-conflict}-{dark,light}.png`（`FLOTER_P13AY_DUMP=1`）。
图上能看到的一处变化（深色 `direct-keycap`，x=460 那一列）：y=39–41 是冠线 **58/58/60**，
紧接着 y=42–55 是面 **24/24/27**，y=56 起是压在下一行上的影（32→43 渐深）——
改前这一段是一枚 level 244 的白片子，没有冠线，也没有把下面压暗的影。

**为什么缝要在平地上量**：启动器的提示是半透明面，它压着的内容逐行不同，面色的像素框会跟着
内容漂（同一枚 1.5u 缝在 launcher 上量到 6px 与 4px 两种读数）。所以材质 pin 在 launcher 上量
（读「改前/改后最大的那块单色」），缝的 pin 在 `ui.Tester` 的平地 harness 上量（面色
= `FaceOver(ground, Float)`，框是确定的）。

## V / B / S 台账

- **V 0**：这七处此前没有任何像素 pin（只有「提示出现了」的文本断言）。
- **B 0**：`view_test.go` 的五条行为 pin 零改动全绿——`TestDirectOutputRowCarriesTheSpaceKeycapTooltip`、
  `TestCommandRowWarningCarriesItsSentence`、`TestHistoryRowMarksCarryTheirSentences`、
  `TestCommandRowSourceCarriesItsWholeWord`、`TestOutputViewControlsCarryTheirHints`（还有
  `filters_test.go` 的两条）。出现条件、锚点、无障碍描述都是框架行为。
- **S 0**：没有删断言、没有加容差、没有 `t.Skip`（出图靠 env）。

## 门槛（实测）

```
gofmt -l cmd internal                       # 空
go vet ./... / GOOS=windows,darwin go vet   # 净
go test -count=1 ./...                      # 全绿
PASS 3103 → 3112 (+9) | FAIL 0 | SKIP 102（不变）| 测试函数 2291 → 2294 (+3)
GOOS=linux/windows/darwin go build ./...    # 过
```

## 变异（4 条实跑，4 红，逐字节还原）

1. `tokens.Tip` 的面退回 `t.Theme.Text`（框架的反色片子）：
   `tipglass_test.go:100: the tip's face is {237 237 239} (level 237): that is the inverse chip, not the float face`
   （浅色档同一行 `:104` 报 `{27 27 29} (level 27)`）。
2. 删掉 `tip.InsetShadow(0, 1, 0, 0, t.EdgeHighlight)`：
   `tipglass_test.go:184: the tip's top edge is {19 20 23} against the face {23 23 26} (-4 levels): no lit edge`
3. 删掉 `tip.AttachTo(...).Margin(0, 0, Space(1.5), 0)`：
   `tipglass_test.go:320: the tip's face ends 2.0px above its anchor, not the framework's 1.5u (6.0)`
4. 空格键帽退回 `key.Tooltip(copy.DirectOutputHint)`：
   `tipglass_test.go:248: launcher.go:2632 still wears the framework's opaque chip` +
   `:254: the launcher carries 6 tips on the liquid-glass material, want the seven §6.20 20-2 names`

四条都 `cp` 还原、`md5sum -c /tmp/p13ay.md5` 通过，`touch` 后复跑绿。

## 偷了什么（craft-loop 工艺密度菜单，逐条落成数值）

- **第 5 项「多层染色阴影」** → 七处各戴两层：环境 `0 6px 18px rgba(0,0,0,0.30)` +
  接触 `0 1px 2px rgba(0,0,0,0.20)`（取代框架那一层 `0 2px 8px 0.2`）。浅色图上量到提示下方
  180 → 238 的渐变，就是那层环境影把下一行压暗。
- **第 12 项「材质细节：内边框高光」** → 顶边 1px `EdgeHighlight`（深色图上 58/58/60 压面
  24/24/27）+ `InputStroke` 一圈发丝；圆角从键帽的 5 抬到浮层的 8（`RadiusSM`）。
- **第 9 项「自定义滚动条」/ 第 1 项「氛围层」/ 第 2 项 `::selection`** → 本仓前几轮已落地或按
  §1 禁令不做（玻璃面不许自造透明、不许叠玻璃），这轮不动。写在这里是为了不假装偷了 5 项：
  这一轮真正的新工艺只有一条——**浮层与标注物之间的 1.5u 呼吸**（不是密度菜单里的项，是
  空间刻度的账，落在 §4）。

## preflight

- **A 反 AI 套路**：无新文案（i18n 零改动）、无 emoji、无渐变白边（冠线是 1px 实色不是渐变）、
  无把手/圆点装饰。**过**。
- **B 设计读取一致性**：读取仍是「克制的功能型轻玻璃」，这轮只把七处提示拉回 z 阶梯，
  密度 6 未变；记忆点 = 提示只有顶边一条亮线。**过**。
- **C 动效抽查**：这轮**不动效**——提示的时钟是框架的 0.6s/0.4s，没有新增 transition，
  `prefers-reduced-motion` 不适用（无动画可关）。**不适用，已写明理由**。
- **D2 中文页**：无新增文案；`tokens.Body` 10.8px 与「正文 ≥ 14px」的张力**仍然不过**，
  已在 `.pending-bugs.md` 记为需拍板（全局字阶），本轮不擅自改。

## 仍待（下一轮候选）

1. **§6.20 的 20-3**：设置页 10 处（`rows.go` 3、`settingsui.go` 6、`config.go` 1）仍是框架平涂，
   机械迁到 `tokens.Tip`，**不需要拍板**。
2. **需拍板（沿用）**：`tokens.Body` 10.8px / `tokens.Caption` 11px 与 D2 的张力；`cardRow`
   控制槽 `Shrink(0)`；多选箱宽度；终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、
   S2 尾（客户端不告而别）。
