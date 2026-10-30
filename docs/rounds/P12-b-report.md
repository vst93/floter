# P12-b 报告：启动器表面——E1 受光边落到这张卡的四个面上

**一句话**：这一轮没有新增任何功能，只是让「选中块 / 键帽 / 反馈带」从**一块平涂**
变成**一块站起来的 E1**——三个手段：面差、顶受光边、底对侧暗线。

## 为什么让它更轻、更有空间感

旧语言里这些块与底挤在**同一个 z 平面**上，只靠颜色差说话；用户读到的就是「平、糊、旧」。
现在每一块都有一条 1px 的受光顶边和一条对侧暗线，块与底的 z 关系是**读出来的**，不是猜出来的；
而整张卡仍然只有**一条**亮线（`LensRim` ≤ 0.09），没有第二层玻璃、没有渐变白边、没有模糊贴图
——「轻」的那一支没有变厚。

## 落地

新增 `theme.Tokens.LitFace`（`internal/theme/theme.go`）：顶边 `LensRim` 1px + 底边
`LensEdge` 1px，**不画投影**（§2 判据 2）。五处调用：

| 位置 | 文件 | 改动 |
|---|---|---|
| 结果行选中块 | `internal/launcher/launcher.go` `rowBody` 的 `i == a.Selected` 臂 | `row = tokens.LitFace(row)` |
| 页脚动作条选中块 | 同处 `isBar` 臂 | 同一个 `tokens.LitFace` |
| 键帽 | `internal/launcher/keycap.go` `keycap` | `key = tokens.LitFace(key)` |
| 捕获输出页选中行 | `internal/launcher/output.go` `outputRow` | `style = tokens.LitFace(style)` |
| 反馈行（静息 + 警告） | `internal/launcher/launcher.go` `feedbackRow` | `feedback = tokens.LitFace(feedback)`（此前只在警告档画顶边） |

**为什么是 `LitFace` 而不是现成的 `Lens`**：`Lens` 的第四层是四边 `ControlEdge` 环——
那正是这一支要退掉的「画框分层」，而且 `docs/raycast-design.md` §4.3 明写选中行
「no border, no ring, no glow」。E1 只用三件事站起来。

**为什么必须有底对侧**（这是本轮唯一一处「先量再改」）：浅色档的面是近白
（`Control` 0.07 黑压在 sunken 面板上），0.09 的白线压上去只抬 **2 级**
（实测 `(600,94)` 221→227）。让块在浅色上真正站起来的是对侧那条 `LensEdge`
（实测 `(600,129)` 220→203，**17 级**）。深色档顶边是 61→88（**27 级**）。
同一句话在 §3 里写着「顶亮一分、底暗一分，控件才站起来」，此前没人把它落到组件上。

## 图上看到的一处变化（自己复核）

`/tmp/floter-P12-b-list-dark-{before,after}.png`（720×580），x=600 那一列：

```
y=94   (受光顶边)  61,61,63  →  88,88,90     ← 这条线此前根本不存在
y=116  (块面)      61,61,63  →  61,61,63     ← 面没动（只加线，不加面）
y=129  (对侧暗线)  61,61,63  →  56,56,58     ← 底暗一分
y=160  (面板)      28,28,30  →  28,28,30     ← 底完全没动
```

浅色档同一帧：`y=94` 221→227（顶亮）、`y=129` 220→203（底暗）。
**这是「站起来」的全部内容**：面不动，上下各一条线。

反馈带（`/tmp/floter-P12-b-note-dark-{before,after}.png`）：
`y=543` 64,49,42 → **92,79,73**（静息档的顶受光边，此前只有警告档才有）。

## 偷了什么（qiaomu-design 的工艺密度菜单）

从 `craft-loop.md` 的工艺密度菜单里取了 **3 项**，逐条落成具体数值：

1. **第 12 项「材质细节：内边框高光」** → `LitFace` 的顶边 `LensRim` 0.09 白，
   底边 `LensEdge` 0.06 黑。**注意这一项我是按用户偏好账本 P-38 与合约 §0 硬禁令 3 的
   约束做的**：不是 CSS 那种渐变白边（那正是用户说的「类玻璃边」），而是一条 1px 实色线。
   这是本轮与 skill 的**唯一分歧点**，理由写在报告末尾的 preflight A 节。
2. **第 5 项「多层染色阴影」的反向用法** → 这一支**禁止** E1 画影（§2 判据 2），
   于是把「多层」改成「成对」：顶亮 + 底暗两条 1px 线，仍是一对而不是一层。
   落成：`InsetShadow(0, 1, 0, 0, LensRim)` + `InsetShadow(0, -1, 0, 0, LensEdge)`。
3. **第 7 项「展示排版动作」的微观版：让最小元素承担材质** → 键帽（20×20u）是这张卡上
   最小的面，也是唯一「被按下去的键」，所以它必须是最先受光的那个。落成：键帽的
   `LitFace` 与结果行同一个函数，不另起一套数值。

`craft-loop.md` 的「两轮制」也走了：第一轮落 `LitFace` → 读图发现浅色档顶边只有 2 级、
块读不出来（这正是「四视角偏执评审」里**艺术总监**挑出的那处）→ 第二轮补底对侧暗线
（工程师视角的那处）。两轮都发生在同一轮次内。

## preflight 逐条结论

- **A 反 AI 套路**：无渐变文字、无紫蓝渐变、无外发光、无三等分卡片、无 italic。
  **不过的一条**：A 节要求「字体不是系统默认」——本仓是原生桌面应用，字体跟随系统
  （`docs/raycast-design.md` §2 明写），**故意不做**，理由：桌面应用换掉系统字体是
  可访问性与一致性倒退。A 节的「无左侧竖线装饰」（P-38）**过**：`LitFace` 是顶/底
  两条横线，没有竖线。
- **A2 进攻三步**：DNA 注入（借 Apple HIG 的「材质只在需要分层处出现」这一条判据，
  落成「E1 不画影」）、工艺密度 3 项（见上）、两轮制（见上）。
  **A2 的「≥5 项」不过**：这是桌面应用的一个表面，不是一整页；把 5 项塞进一个 40u 的
  结果行只会变成噪点（`craft-loop.md` 自己的约束「密度不等于吵闹」）。
- **A4 功能契约**：过。这一轮没有动任何交互：Enter 仍运行选中的行，键帽仍是同一个 chord，
  反馈带仍是同一句文案。
- **B 设计读取一致性**：过。设计读取是「功能型桌面启动器 / 面向键盘重度用户 / 轻便液态玻璃」，
  拨盘 VARIANCE 3、MOTION 2、DENSITY 7（功能页的低冒险、低动效、高密度）——本轮
  没有任何动效，也没有任何冒险装饰，与读数一致。
- **C 动效抽查**：过。这一轮**没有新增任何动效**；已有的 120ms 行进入与 160ms 带进入
  未改。无 `transition: all`（本仓不是 CSS）、无 `scale(0)`、无纯 fade 入场。
- **D2 中文页**：本仓 UI 有中文文案，但这一轮**没有动任何文案**；新增的只有代码注释
  （中文注释不参与排版）。

## V/B/S 台账

| 类 | 条数 | 说明 |
|---|---|---|
| **V**（视觉数值重定基线） | **1** | `internal/shell/glass_test.go` `TestTheFrameSheenIsFixedAcrossStops`：采样带 `y=90..120` → `y=300..340`。旧带落在**行与键帽上**，而那条 pin 的断言是「**帧自己的脸**不随 stop 变」；控件透镜本来就该随 stop 分级（`--glass-lens-scale` 0.5/1/1.6，`glasscensus_test.go` 的普查）。受光边落到四个面之后，旧带的 stop 间色阶差从 **0.14 → 2.36**（深），超过 pin 的 `spread > 1`。新带只有面板自己的面，实测 stop 间差 **0.00（深）/ 0.02（浅）**，断言强度未松。 |
| **B**（行为/度量/接线） | 0 | 其余全绿，测试零改动 |
| **S**（结构） | 0 | 无结构变化 |

新增 pin **4 条函数 / +8 PASS**（每条都在深/浅两套外观上跑子测试）：
`TestTheChosenRowStandsOnOneLitEdge`、`TestTheKeycapIsTheSmallestLitFace`、
`TestTheFeedbackBandIsOneMaterialInBothStates`、`TestTheOutputSelectedRowWearsTheSameLitFace`。

## 门槛（全部实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                    → 全绿，0 FAIL
go test -count=1 -v ./... | grep -c "--- PASS"  → 2819（基线 2805，+14）
SKIP                                      → 60（基线 59，+1：本轮新增的 opt-in 截图 dump `TestDumpP12BLitEdgeFrames`）
测试函数                                   → 2091（基线 2086，+5：四条 pin + 一条 opt-in dump）
GOOS={linux,windows,darwin} go build ./... → 过
```

## mutation（4 条实跑变红 + `md5sum -c` 逐字节还原）

| # | 注入 | 实测报错（原串） |
|---|---|---|
| M1 | 结果行选中块删掉 `LitFace` | `litedge_test.go:106: the chosen row's top edge {238 238 239 255} is not brighter than its own face {238 238 239 255}: the lit edge is missing (E1 = face + one 1px LensRim, liquid-glass.md §2)` |
| M2 | 键帽删掉 `LitFace` | `litedge_test.go:153: the keycap has no lit edge: its face is {59 59 61 255} and none of the plate's top four rows is a neutral line brighter than it — the smallest face on the card lost its crown (liquid-glass.md §6.1 第 1-4 行)` |
| M3 | 反馈带删掉 `LitFace` | `litedge_test.go:233: the note band is a flat colour block: its face is {248 235 227 255} and none of its top four rows is a neutral line brighter than it — the band is not an E1 pane (liquid-glass.md §6.1 第 1-7 行)`（深浅两档都红） |
| M4 | 输出页选中行删掉 `LitFace` | `litedge_test.go:282: the chosen output row's crown {238 238 239 255} lifts its face {238 238 239 255} by only 0 levels` |

四条都用 `md5sum -c` 逐字节核对还原（输出「成功」），还原后复跑全绿。

## 仍待

1. **下一轮 = `docs/liquid-glass.md` §6.2 设置页**（9 行，其中 1 行 `故意不做`）。
2. **P12-a「仍待」第 2 条的 `Float` 档固定 +0.10** 本轮**没有**动：它属于 §6.6 弹层，
   量它的那一轮才该改（本轮只做启动器，不跨表面）。
3. **E1 的受光边在浅色档只有 2 级**（顶边），块站起来主要靠底对侧。这条已写进 §6.1 的
   注脚；如果后面几轮发现「浅色档的 E1 仍然读不出」，那就不是把线加亮（会破坏
   `LensRim ≤ 0.09` 的上限），而是**把面阶再抬一档**——需要拍板，写进 `.pending-bugs.md`。
