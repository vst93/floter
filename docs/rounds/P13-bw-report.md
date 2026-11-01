# P13-bw 报告：捕获输出页页头两枚 28u 方键的第三档

轮号 `P13-bw`｜分支 `mygo-rewrite`｜基线 HEAD `0432fc1a`（P13-bv）
规格 `docs/rounds/P13-bw-spec.md`｜合约 `docs/liquid-glass.md` §6.40 第 **40-3** 行

## 0. 一句话

差距表里**唯一一条 `未开始`**（40-3）：捕获输出页页头的「钉为独立窗口」「在窗口中打开」
两枚 28u 方键此前 `hover=2 / press=0`——按住不放的那一帧与悬停帧**逐像素相同**。
本轮把指针第三档补上，这一族在启动器侧收口。

**这轮为什么让它更轻、更有空间感**：一块能被碰的面必须回答「我压到它了吗」。
此前这两枚键按下时既不退、也不沉，用户读到的是「按坏了、没反应」——一块没有厚度的贴纸。
补上第三档之后，同一个 28u 盒子在指针下走完三步（无面 → 抬到 E1 → 沉回一半），
`docs/liquid-glass.md` §5 那句「按下是唯一的几何变化」在**整个应用**的指针族里第一次全成立。

## 1. 先量（改前，实测像素）

`internal/launcher/output.go` 的 `control` 闭包只有一条 `if button.Hovered()` 臂，
没有 `Pressed()` 分支。720 帧、真启动器地面，键面读点 `(box.X+3, box.Y+H/2)`，
条子面读点在键左 20px 同一行：

| 外观 | 条子面 | 静息 | 悬停 | 按下 |
|---|---|---|---|---|
| 浅 | `{253,253,254}` | `{253,253,254}` | `{226,226,227}` | **`{226,226,227}`**（与悬停同值） |
| 深 | `{23,23,25}` | `{23,23,25}` | `{56,56,57}` | **`{56,56,57}`**（与悬停同值） |

复现命令（改前会红，报错串见 §5 的 M1）：

```
go test -count=1 -run TestTheOutputHeaderKeysSinkWhenThePointerGoesDown -v ./internal/launcher/
```

## 2. 落地

`internal/launcher/output.go`（`outputBody` 内的 `control` 闭包）：

- 面写成变量 `plane`，`switch` 三臂：`Pressed` → `tokens.ControlPress` + `Scale(0.92, 0.92)`；
  `Hovered` → `tokens.ControlHover`；静息 → `ui.Transparent`。
- 一对线**提到臂外**：`if plane.A > 0 { tokens.LitFace(button) }`
  —— `launcher/power.go` / `settingsui/savealert.go` 的既有形状
  （`internal/ci/litarm_contract_test.go` 逃生口 ②：一份面，所有状态共用一对线）。
- 注释补第三档的存在与取舍：按下档**保留**一对线（同 P13-q / P13-bv：小格的面是
  「一块能被碰的板」，压下去仍在地面之上，不是沉进凹槽）。
- 高度链、`Padding`/`Gap`/`Size(7)`/`RadiusSM`/`Focusable`/`Label`/`Tip`、
  `PinText` / `DetachOutput` 的接线与 `copy.*` 文案：**一个字节没动**。

改后（同一探针）：

| 外观 | 条子面 | 悬停 | 按下（新） | 按下落在 |
|---|---|---|---|---|
| 浅 | 253 | 226 | **245** | 条子面 253 与悬停 226 之间 |
| 深 | 23 | 56 | **35** | 条子面 23 与悬停 56 之间 |

按下帧里受光边仍在（`Scale(0.92)` 后的顶边读 247 / 55，面 245 / 35），
对侧暗线在缩放的亚像素边缘上被抗锯齿化（浅 240 / 深 36）——**这是几何变换的必然结果，
不是这一族独有的取舍**，同 P13-q / P13-bv 的设置页小键一样。

## 3. 出图（深/浅 × 静息/悬停/按下，六张）

`FLOTER_PNG=1 go test -count=1 -run TestDumpP13BW -v ./internal/launcher/`

- `/tmp/floter-P13-bw-outputheader-light-{rest,hover,press}.png`
- `/tmp/floter-P13-bw-outputheader-dark-{rest,hover,press}.png`

**我自己在这张图上看到的一处变化**：`light-hover` 与 `light-press` 两张图，
像素差**恰好只落在 `x[648..675] y[60..87]`**（28×28 = 键自己的盒子），共 744 点；
盒子外**零像素差**（条子面、标题、状态行、正文全不变）。也就是说第三档不是「重排」，
而是同一块键在自己的盒子里换了一档面——这正是 §5 要求的「按下只改自己那一块」。
按下帧里键的顶边 247 比面 245 亮一分、条子面 253 在四周露出来一圈：读起来是
「键沉下去了一点、光还在上面」，而不是「键消失了」。

## 4. pin 与 V/B/S 台账

pin：`internal/launcher/outputheaderpress_test.go`，4 条判据 × 深/浅 + 1 条 opt-in dump：

1. 悬停真的画了面（悬停面 ≠ 条子面，差 ≥ 4）；
2. 按下真的画了**另一个**面（按下面 ≠ 悬停面，差 ≥ 3）——**旧实现这一条逐像素相同**；
3. 按下面在条子面与悬停面**之间**（第三档是往静息的回退，不是第三种颜色）；
4. 按下没重画条子面、也没搬键的布局盒（`Scale` 是变换，条子不许重排）。

**V 0 / B 0 / S 0**：旧 pin 一条没红（`launcher` / `ci` / `shell` / `settingsui` 四个包全绿），
本轮只新增文件，未改任何既有测试与断言。

## 5. 变异（实跑，全部变红后逐字节还原）

原始文件 md5 `34f874469b7bbf30e09529a01d2a2f6a`（`md5sum -c` 逐字节核对通过）。

**M1 —— 按下档画成悬停面**（= 本轮之前的行为）：`case button.Pressed(): plane = tokens.ControlHover`

```
--- FAIL: TestTheOutputHeaderKeysSinkWhenThePointerGoesDown
    dark=false: strip {253 253 254} rest {253 253 254} hover {226 226 227} press {226 226 227}
    the key under the pointer is {226 226 227} whether or not the button is down (strip {253 253 254}):
    the press rung paints no face of its own — one step back toward the strip is what
    `--glass-control-press` means (docs/liquid-glass.md §5, §6.40 第 40-3 行)
```

**M2 —— 删掉臂外共享的一对线**（`if plane.A > 0 { tokens.LitFace(button) }` 整块）：

```
--- FAIL: TestEveryPointerStateFaceNamesItsPair   (internal/ci)
    ../launcher/output.go:610: launcher.outputBody/Pressed paints a face in its pointer arm
      (plane = tokens.ControlPress / button.Scale(0.92, 0.92)) and never a pair
    ../launcher/output.go:613: launcher.outputBody/Hovered paints a face in its pointer arm
      (plane = tokens.ControlHover) and never a pair
```

两条都实跑变红，`cp` 还原后 `md5sum -c` 通过并 `touch` 复跑一次（避开测试缓存）。

**M3 未跑并说明原因**：把 `Scale(0.92)` 换成 `Scale(1)` 预期**不会**让本 pin 变红——
本 pin 只断言「盒子没被重排」，不钉缩放系数（系数属小格族几何，已由 P13-q / P13-bv 的
同族 pin 与 `docs/liquid-glass.md` §5 管）。不假装它有用，故不列入变异台账。

## 6. 门槛（实测）

```
gofmt -l cmd internal                 → 空
go vet ./...                          → 净
GOOS=windows go vet ./...             → 净
GOOS=darwin  go vet ./...             → 净
go test -count=1 ./...                → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 3217（基线 3216，+1）
                                   --- SKIP           → 121（基线 120，+1 = opt-in dump）
                                   测试函数            → 2376（基线 2374，+2）
GOOS=linux|windows|darwin go build ./...            → 全过
```

## 7. 用了设计 skill（qiaomu-design，走「已有方向，只要打磨」那条路）

读的参考：`SKILL.md` → `references/user-preferences.md`（偏好账本）→
`references/craft-loop.md`（工艺密度 / DNA 注入）→ `references/preflight.md`（A/B/C/D2）。
**没有**走 Phase 2 的四方向预览页：方向已由用户钉死（轻便液态玻璃），这一支不允许停下来等点选。

### 偷了什么（逐条对得上号）

- **craft-loop「一、DNA 注入」**：本轮的气质供体是 `design-systems/apple/DESIGN.md` 与
  `design-systems/linear.app/DESIGN.md`，取的是**三个具体值**，不是「像 Apple」这种话：
  1. `apple/DESIGN.md:136`「Active: **scale(0.9)**, background shifts」→ 本仓小格族的按下几何
     `Scale(0.92)`（比 Apple 更收敛一档：28u 的键按 0.9 会缩 2.8px，在 1px 发丝线族里读成抖动）；
  2. `apple/DESIGN.md:47` Button Active `#ededf2`（浅面上**往页面回退**而不是变深）→ 本轮按下档
     `ControlPress` 落在条子面与悬停面**之间**（浅 245 落在 253 与 226 之间），而不是第三种颜色；
  3. `linear.app/DESIGN.md:9` 的 `rgba(255,255,255,0.05–0.08)` 发丝线纪律 → 按下帧**不加粗**
     那一对线（`LitFace` 仍是 1px，不因按下变成 2px 的「按下的框」）。
- **craft-loop「二、工艺密度」菜单**：本轮取第 **12** 项「材质细节（内边框高光）」并把它从
  「悬停才有」扩到**三档都有**——落成具体数值：按下帧顶受光边 `247`（浅）/ `55`（深）
  对自身面 `245` / `35`，即「亮一分」；同一份面共用一对线，靠 `plane.A > 0` 一个判据。
  另取第 **4** 项「签名交互」的一半：本仓的签名动作是「按下是唯一的几何变化」，
  本轮把它的**触发面**从设置页扩到捕获输出页（`Scale(0.92)` + 面档）。
  菜单第 1/2/3/9/10/11 项**故意不取**：自绘桌面应用没有 `body::before`、没有 `::selection`
  品牌色（文本选中由 `inputtext` 自己的选区管）、没有滚动条、没有页脚彩蛋、不用 SVG 插画；
  第 6 项（章节编号/hairline）与第 8 项（流体字号）已在既有合约里定死，本轮不重开。
- **preflight 结论**：
  - **A（反 AI 套路）**：过。无渐变高光、无紫蓝、无发光；`Scale(0.92)` 不是 `scale(0)` 入场；
    文案零新增（`copy.HistoryPin` / `copy.OutputDetach` 未改）。
  - **B（设计读取一致性）**：过。设计读取 = 「桌面启动器的一个浮层页头，面向键盘用户，
    用低光泽轻液态玻璃语言，倾向 raycast/liquid-glass 家族」；VARIANCE 3 / MOTION 3 / DENSITY 6
    不变（本轮只补状态，不动结构）。记忆点：**按下去键沉、光还在上面**。
  - **C（动效抽查）**：过。无 `transition: all`（自绘，无 CSS）；按下有反馈（本轮的整件事）；
    只动 `transform`/面（`Scale` + 背景 alpha），不动 `width/height/padding`；
    高频键盘操作无动画；`ReduceMotion` 由 `ui` 层的既有开关兜底（本 pin 用真机帧验证，不依赖动画）。
  - **D2（中文页）**：过，但**沿用既有张力**——本页中文文案未改，`tokens.Caption` 11px
    与「正文 ≥ 14px」的冲突是**已记账的需拍板项**（`.pending-bugs.md` 第 4 份证据），
    本轮不擅自改字号。中西文空格、全角标点、字重 400/500/600 三档均未触碰。
- **两轮制**：第一轮实现 + 出图 → 四视角各挑一处：
  - 艺术总监：按下帧「缩了但边缘抗锯齿吃掉了对侧暗线」，读起来比悬停略糊 → 判定**接受**：
    这是 `Scale` 的几何必然，同族三个包一致；若要改就得改族规（需拍板），本轮不改。
  - 工程师：`Tip` 包在键外层，`Pressed()` 必须取 `Tip` 之后的元素——已按既有写法核对，无问题。
  - 甲方：三秒内能看出「我按到了」——从「看不出」变成「看得出」，契约达成。
  - 读者：键上只有一个箭头字形，按下帧字形跟着缩到 0.92，仍居中可读。
  第二轮只修了注释（把第三档的取舍写清），未再改像素。

## 8. 仍待（P13-bw）

1. **需拍板（沿用，第 4 份证据）**：按下档要不要保留那一对线。
   `internal/ci/litarm_contract_test.go` 的 `pointerFaceAllow` 里仍有
   `launcher.rowControl/Pressed`、`launcher.fieldSettingsButton/Pressed` 两条写着「不保留、待拍板」，
   而本轮与 P13-bv 在设置页/捕获输出页都选了「保留」——同一应用里两族两个说法，拍板后一起收。
2. **下一格候选（先量再改）**：① `launcher/power.go` 确认带两枚 chip（`hover=2 / press=0`，
   但暖面**没有按下档 token** → 需先定 token，属拍板）；② `launcher/filters.go:219` 过滤 chip
   （参考自己就没有面，R72：chip 是字不是药丸 → 需拍板）；③ 通用队列下一格：
   **禁用 / 只读面**（状态反馈的另一半）——先在设置页找「禁用了但看不出为什么」的面，量墨差。
3. **需拍板（沿用）**：`tokens.Caption` / `tokens.Body` 与 preflight D2「正文 ≥ 14px」；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
