# P13-ch 报告：参数编辑器的空槽说的是一句结论，却和它上面的提示一样淡

规格 `docs/rounds/P13-ch-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.51**｜
起点 `cae6e5a2`（P13-cg）

## 这轮为什么让它更轻、更有空间感

空态是「一句话」——§4.7 把它钉成**陈述**（压过提示），因为一块空区域要靠**这句话**成立，
而不是靠画框。参数编辑器的这个槽是全仓五处空态里唯一还按参考 CSS 的旧值画的：句子与它
上面那行**提示**逐像素同值（浅 100/100、深 96/96），于是虚线槽里那句话读起来只是「第二行
灰字」，槽本身（1px 虚线 + 8u padding）反而成了唯一在说话的东西。抬到陈述档之后，
槽是「待填的格子」，句子里那句话是「这一段不用填，这没问题」——层级回来了，槽的面一像素没动。

## 先量再改：量到的、和没量到的

**成立（改）**：卡片的墨阶梯实测为 标题 128 / 提示 100 / 空态句 100（浅）、
147 / 96 / 96（深）。最淡的两行是「说明」和「结论」，结论与说明同值。

**不成立（不改，写进 §6.51）**：
- 上一轮猜「虚线槽的边可能看不见」——**量到 22 级**（面 246 / 虚线 224，深 15 / 19），
  与实线发丝线同一条；3 on / 3 off，四条边都在，盒子 `1025..1057` = 33px、左右 padding 8u。
  这条猜想的根源是把「虚线像素少」当成了「虚线淡」。
- 上一轮猜「便条行的图标板叠 `DimFace` + `Opacity(0.62)` 两道会淡到没有」——**本轮没量**，
  不写结论，留在「仍待」。

## 改了什么

| 文件 | 改动 |
|---|---|
| `internal/settingsui/parameditor.go:81-99` | 空态句 `Caption` + `TextTertiary` + 400 → `Caption` + **`TextStrong`** + **580**；注释改写（参考旧值 → P8-b 之后的陈述档，并写明为什么尺寸不跟着抬） |
| `internal/settingsui/paramempty_test.go` | 新 pin 三条 + 一条 opt-in 出图 |
| `docs/liquid-glass.md` | 新增 §6.51（含「只钉最暗像素的 pin 抓不到字重」与「虚线要读发丝带」两条量法） |

**产品代码只改这一处墨与字重**：高度链、键盘模型、会话管理、设置写回、i18n 文案
（`ParamsEmpty` 一个字不改）、命令接线全部不动；槽的几何（1px 虚线 / 8u padding /
28u 下限 / `RadiusSM`）不动。

## 实测

| | 浅 | 深 |
|---|---|---|
| 卡片标题（`TextMuted` 600） | 128 | 147 |
| 提示（`TextTertiary`） | 100 | 96 |
| 空态句 改前 | **100** | **96** |
| 空态句 改后 | **217** | **220** |
| 墨量（`Δ ≥ 60` 的像素）400 → 580 | 512 → **611** | 512 → **619** |
| 槽的盒子 / 上下 padding | `1025..1057` = 33 / 8u + 8u | 同值 |
| 槽的虚线级差 | 22 | 22（面 15 / 虚线 19） |

图上看到的一处变化（`/tmp/floter-P13-ch-paramempty-{light,dark}.png`，卡片带裁切）：
虚线槽里那句话现在是这张卡上**墨最重**的一行（217/220，压过标题的 128/147），而它**仍然
是 11px 的同一行、同一个 15u 行盒、槽仍然是 33px 高**——变的只有墨和笔画量，格子和排版
一个像素没动。

## V / B / S 台账

- **V 0**：没有旧断言钉着「空态句 = TextTertiary」的数值；`params_test.go` 只查文本存在，
  零改动。
- **B 0**：`TestTheParamEditorDrawsTheReferencePlates` 等既有 pin 全绿、零改动。
- **S 0**：无结构变化。

## 门槛（实测）

```
gofmt -l cmd internal                     → 空
go vet ./...                              → 净
GOOS=windows go vet ./...                 → 净
GOOS=darwin  go vet ./...                 → 净
go test -count=1 ./...                    → 全绿
PASS 计数                                 → 3248 → 3251（+3）
FAIL / SKIP / 测试函数                     → 0 / 128 → 129 / 2404 → 2408
GOOS=linux|windows|darwin go build ./...  → 过
```

## 变异（3 条实跑，全部变红并逐字节还原）

- **M1**：`TextStrong` → `TextTertiary`（字重留着）→
  `dark=false: the empty state is 100 levels off the face, its hint above it is 100 — a conclusion
  is not a second hint (§4.7)` + `the conclusion (100) is lighter than the heading that names the
  section (128)`；深色同样双红。
- **M2**：删 `FontWeight(580)` → `dark=false: the conclusion puts down 512 ink pixels, want the
  580 weight's 560+ (400 was 512)`（浅/深双红）。**这条第一次没红**——只钉最暗像素的 pin
  看不见字重，于是补了墨量臂（规格里预告过这个分支）。
- **M3**：`Padding(Space(2))` → `Space(1)` →
  `no rule 9px above the statement (the slot's top edge)` + `the slot's bottom edge is not a
  1px rule (neighbour rows are not face)`（浅/深双红）。
- 每次还原后 `md5sum -c` 逐字节核对 + `touch` 复跑绿。

## 偷了什么（`qiaomu-design` 工艺密度菜单 → 具体数值）

- 从 `craft-loop.md` 的**「层级靠墨与字重，不靠画框」**这一项（工艺密度菜单第 3 项）取了
  「同一块面里两行字必须差一个可读级差」→ 落成**陈述 ≥ 提示 + 60 级**这条门槛，
  以及「结论不弱于标题」的第二条断言。
- 从 `craft-loop.md` 的**「字重是墨的一半」**（第 5 项）取了「同一 token 下靠笔画量分档」
  → 落成 `FontWeight(580)` + **墨量 ≥ 560**（400 实测 512）。这条直接来自 M2 第一次没红
  的教训：只读最暗像素的验收是假验收。
- 从 `preflight.md` 的 **D2（中文/小字可读性）** 取了「小字不许既小又淡」→ 这一轮的修法
  正是它的反面：**尺寸留在 11px，墨抬到陈述档**（而不是把字放大，那会破坏槽的 28u 格子）。

## preflight 逐条

- **A（反 AI 套路）**：过——没有加渐变、没有加光晕、没有新画框；改的是两行代码里的两个
  token，视觉上是「同一句话变清楚」，不是「多了一个装饰」。
- **B（设计读取一致性）**：过——§4.7 的陈述档（`TextStrong` + 580）在本仓已有四个同族
  调用点，这一处是第五个；尺寸的分歧写进 §6.51 判据 1 并给了理由（槽不是一页）。
- **C（动效抽查）**：不适用——这一处没有过渡、没有状态切换动效（空态句是静态文本）。
- **D2（中文页）**：**过一半**——`ParamsEmpty` 的中文是「脚本不带参数也能正常运行。」，
  抬墨对中文同样成立（11px + `TextStrong` 在浅色面上 217 级差）；但 `tokens.Caption` 11px
  与 D2「正文 ≥ 14px」的冲突仍在，**这是第 12 份证据**，继续挂在「需用户拍板」。

## 仍待（P13-ch）

1. **下一格**：上一轮列的第 2 条候选——便条行的图标板叠 `DimFace` + `Opacity(0.62)` 两道
   （`output.go:907-910`、`launcher/glyphs.go:286-309`），**先量**图标在淡了两道之后还有
   多少级差，再决定要不要收成一道。
2. **需拍板（沿用 + 第 12 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：启动器 `breathUnits`（4px）对捕获输出页 `Space(4)`（16px）；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；按下档那一对线；终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
4. **抢救 stash 核对（本轮第十次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
