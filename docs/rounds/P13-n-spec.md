# P13-n · E1 是**一对**线：先量受光边，再改真正缺的那一半（深色档的对侧）

## 意图（一句话）

任务书点名要量「`LensRim` 0.09 在浅色面上是否真看得见」。量下来的答案是**看不见**
（面 230 上只抬 +2 级），但**改它会把 accent 面洗成类玻璃边**——而浅色档并不缺这半条线
（它的对侧降 14 级）。真正只剩一半的是**深色**档：面 39 上，0.06 的对侧暗线只降 **2 级**。
这一轮改的是它：深色 `LensEdge` **0.06 → 0.20**。

## 为什么现在做（证据）

`docs/liquid-glass.md` §6 没有 `未开始` 行 → 转通用队列。队列第一条就是任务书点名
「先量再改」的两个数里的第二个（浅色受光边）；量完它，同一次测量把「另一半」也量出来了。

量法：渲染**真的设置页**（`PageGeneral`，860×900），取页面自己的地面像素（light 247 /
dark 14），再把 `theme.Tokens.LitFace` 画在同一块地面上，扫「顶 3 行最亮 / 底 3 行最暗 /
中间行是面」。默认玻璃档（`LensScale = 1`）：

| 外观 | 地面 | 面 | 冠线（顶） | 抬高 | 对侧（底） | 压暗 |
|---|---|---|---|---|---|---|
| light | 247 | 230 | 232 | **+2** | 216 | **−14** |
| dark | 14 | 39 | 58 | **+19** | 31（旧 37） | **−8**（旧 −2） |

同一批测量还回答了一个「改哪一半」的问题（候选白线的合成表，`liteMeasure` 实测）：

| 面 | rim 0.09 | 0.45 | 0.55 | 0.92（= E2 浅色冠线） |
|---|---|---|---|---|
| 221 | +3 | +15 | ~+18 | +31 |
| 244 | **+1** | +5 | +6 | +10 |

**为什么浅色不改**（实测，不是判断）：把浅色 `LensRim` 抬到 0.55 之后，
`TestThePrimaryCTAIsAnAccentButtonAtRadiusSM` 立刻红——它靠「强调色 CTA 的**顶行**里能找到
强调色」来量圆角（radius 8 vs stadium），而 0.55 的白把那一整行洗成浅白，强调色在顶行
**不存在**了。这正是 §1 禁令 3 点名的「渐变白边模拟高光 / 类玻璃边」。浅色档的块本来就靠
对侧站起来（−14 级），不缺那半条线。

## 逐条落地项

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `internal/theme/theme.go` 深色调色板 `lensEdge`（现 `:823`） | `ui.RGBA(0,0,0,0.06)`：面 39 上只降 2 级 | `ui.RGBA(0,0,0,0.20)`；注释写旧值→新值与实测（−2 → −8） |
| 2 | 同处浅色调色板 `lensEdge` / `lensRim`（两套） | — | **不动**（理由见上） |
| 3 | `internal/launcher/raycast_test.go` 的 `LensEdge.A > 16` | 一律 ≤ 0.06 | 按外观分档：浅 ≤ 0.06 / 深 ≤ 0.20（V 重定基线） |
| 4 | `docs/liquid-glass.md` §1 光泽上限表 + 「新上限一句话」 | `LensEdge` 一列一个值 `0.06` | 深 `0.20` / 浅 `0.06`，附旧值→新值与理由 |
| 5 | `docs/liquid-glass.md` §3 受光边表 | 没有 `LensEdge` 行 | 新增一行（深 0.20 / 浅 0.06） |
| 6 | `docs/liquid-glass.md` §6.0 | 七行 | 新增 **0-8**（`已落地`）；§6.2 的 2-5 补一句「凹槽的顶内线随之加深 −8 级」 |
| 7 | 新增 `internal/settingsui/counterside_test.go` | 无 | 一条像素 pin（深/浅两套外观，量**真的** `LitFace`：对侧必须降 ≥ 6 级，且两半里至少一半 ≥ 10 级）+ 一条 opt-in dump |

## 行为契约零改动红线

只改一个墨值（一个 token 的 alpha）。高度链、键盘模型、会话管理、设置写回、i18n 文案、
命令接线、几何全部零改动；层数不变（`LitFace` 仍是顶亮 + 底暗两层，无投影，§2 判据 2）。

## 门槛命令 + PASS 基线

```
gofmt -l cmd internal                                  # 空
go vet ./... + GOOS=windows/darwin go vet ./...        # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2941 → 实测 2944
测试函数                                                # 基线 2165 → 实测 2167
三平台 go build ./...                                   # 过
```

## mutation（实跑）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 深色 `lensEdge` 回 `0.06` | 红：`[dark] the counter-side drops its face by only 2 levels (37 under 39 …)` |
| M2 | 深色 `lensEdge` 改 `0.60` | 红：`LensEdge is alpha 153, above this appearance's counter-side cap 51` |
| M3 | `LitFace` 的底边换成 `LensRim` | 红（新 pin，深/浅两条）：`the counter-side (230) is not darker than the face (230): E1 is a pair` |
| M4 | `LitFace` 的顶边换成 `ControlEdge` | 红（既有族，不是新 pin）：`the action pill: the rows inside its top frame are 115, not brighter than its face 119`、`the chosen row's top edge 224 is not brighter than its own face 238` |
