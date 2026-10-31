# P13-am 报告：终端浮条的五枚键补上第三档

**一句话**：终端页的浮条上的五个 28u 键，从此按下去和悬停不是同一帧——面往浮条自己的面
退一格（`LensRim` 的冠线仍在），第三档从命名调色板自己的墨里派生，不是借窗口灰。

**这轮为什么让它更轻 / 更有空间感**：这五个键在静息时不画任何面（浮在画布上的一枚透明
28u 方块），悬停时贴上一块薄面（`LitFace` 的一对面）。此前**按下**那一帧与悬停帧逐像素相同
（实测浅 221↔221、深 48↔48、`ink` 画布上 33,36,38↔33,36,38、`paper` 上 223,219,209↔223,219,209），
于是三层 z 深度只有两层：指针落下去，键不动。现在同一个键有三个深度——静息（无面）→
悬停（0.10 的洗色 + 受光冠线）→ 按下（0.035 的洗色，退回画布方向一档，冠线仍在）。这一档
没有加任何新材质、没有新颜色：只是让**已经存在的阶梯**在画布这一类面上也成立。

## 落地

- `internal/terminalui/terminalui.go`
  - `barInks` 第六档 `Press`（`docs/liquid-glass.md` §5 的第三档在哪一页都得有落点）。
  - `windowBarInks`：`Press = f.SurfacePressed`（浮在窗口画布上时，浮条的两块面就是控制阶梯的两档）。
  - 命名调色板分支：`Wash = ink.Alpha(0.10)` / `Press = ink.Alpha(0.035)`；`0.035` 来自阶梯自己的
    比例（`0.032/0.105 = 0.305` 浅、`0.05/0.14 = 0.357` 深）取上沿，理由：浮条的洗色本来就比
    设置页薄，0.035 是调色板墨在最暗画布上还能画出 ~8 级的那一档。
  - `barButton`：`switch` 里 `Pressed()` 臂在前，`LitFace(Background(inks.Press))`。
- `internal/terminalui/pressrung_test.go`（新）：两条 pin + 一条 opt-in 出图，四套墨源
  （`inherit` 浅/深、`ink` 深调色板在浅窗口、`paper` 浅调色板在深窗口）× 五枚键。
- `docs/liquid-glass.md` §6.7b：第 7-9 行 `故意不做` → `已落地`；新增 7-10 记几何那一档「需拍板」。

## 实测（四套墨源 × 五枚键，深浅两套外观）

| 墨源 | 浮条面 | 悬停面 | 按下面（新） | 差 |
|---|---|---|---|---|
| inherit-light | 247,247,248 | 221,221,222 | **239,239,240** | 距悬停 18、距静息 8 |
| inherit-dark | 14,14,16 | 48,48,50 | **26,26,28** | 距悬停 22、距静息 12 |
| ink-lightwindow | 11,13,16 | 33,36,38 | **19,21,24** | 距悬停 14、距静息 8 |
| paper-darkwindow | 244,239,228 | 223,219,209 | **237,232,221** | 距悬停 14、距静息 7 |

按下的冠线与对侧：`ink` 画布上按下键的一列（x=873）读作
`画布 11,13,16 → 冠 75,77,79 → 面 19,21,24 … → 18,20,23（对侧）→ 画布`，
即按下这一档**仍然是一块有受光边的面**，不是一块平涂。

## 取舍：几何收档留给框架（**需拍板**）

族里的按下是「面沉 + `Scale(0.92)`」。本轮实测把它带进来会丢一个点击：浮条上的键在抽屉
滑入的那两帧里被裁到 **2 DIP** 宽（实测盒子 `{858 314 2 28}`），`Scale` 绕中心缩后左边缘
往里让 0.08 DIP，正好把落在中心线上的点击点挤出命中框，释放时 `rt.contains` 为假 ——
`TestTerminalDrawerClosesFromItself` / `TestTerminalDrawerGearToggles` 两条 B pin 变红
（原始报错：`the drawer's own close did not close it`、`the flipped gear did not close the drawer`）。
按纪律改产品不改测试，所以这一页只收面；几何写成 `liquid-glass.md` §6.7b **7-10**。
（附带观察到的事实，写给下一轮：真机上「按下 + 至少一帧再松开」是好的——实测
press → 2 frame → release 能把抽屉开关一次；零时长点击只在盒宽被裁到 2 DIP 时才丢。
这是框架里「盒宽收缩时命中框跟着缩」的账。）

## V / B / S 台账

- **V 2**：`pressrung_test.go` 的两条新 pin（面 / 局部性）。没有旧 pin 被改写。
- **B 0**：零改动；`drawer_test.go`、`pointerreach_test.go` 等全绿（本轮中途曾因 `Scale`
  变红 2 条，按 B 的判据退回产品侧，见上）。
- **S 0**。

## 门槛（实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → **全净**
- `go test -count=1 ./...` → **0 FAIL**
- PASS **3047 → 3049**（基线 3047）| PASS 计数只涨 | SKIP 93 → **94**（+1 opt-in 出图）| 测试函数 2252 → **2255**（+3）
- `GOOS=linux|windows|darwin go build ./...` → **全过**

## mutation（3 条实跑，全部变红，md5 逐字节还原）

1. **M1** 按下臂 `Background(inks.Press)` → `inks.Wash`：
   `inherit-light/Terminal settings: the key under the pointer is {221 221 222} whether or not the
   button is down (bar {247 247 248}): the press rung paints no face of its own…`
2. **M2** `barPressAlpha 0.035 → 0.2`：
   `ink-lightwindow/Terminal settings: the pressed face {55 57 61} is not between the bar's own face
   {11 13 16} and the hovered {33 36 38} — the third rung is a step back toward rest, not a third colour`
3. **M3** `windowBarInks` 的 `Press: f.SurfacePressed → f.SurfaceHover`：
   `inherit-light/...: the key under the pointer is {221 221 222} whether or not the button is down…`

每条跑完 `cp /tmp/tui.orig` 还原并 `md5sum -c` 通过（`4f26ce3b…`），`touch` 后复跑一次全绿。

## 出图（8 帧，自己复核过）

`/tmp/floter-P13-am-<inherit-light|inherit-dark|ink-lightwindow|paper-darkwindow>-<hover|press>.png`

- 图上看到的一处变化：`inherit-light` 的 hover 帧里关闭键是一块 221 的整块面（顶边 229 受光），
  press 帧里同一枚键退到 **239**、顶边仍是 243 的受光冠线、键还在原来那 28 DIP 的盒子里
  （布局盒 `{820 0 28 28}` 两帧相同）。
- 图上看到的第二处：整帧差异只有 **585 像素**，bbox `25×25 @ (864,0)`——正好是那枚被按住的键；
  左边四个键、标题、浮条本身一个像素没动（这也是 pin ④ 的数字）。

## 偷了什么（`qiaomu-design` / `craft-loop.md` 工艺密度菜单）

- 菜单 **#12 材质细节**（内边框高光）：不新造边，按下这一档沿用 `theme.Tokens.LitFace` 的
  一对 1px 内线（冠 `LensRim` / 对侧 `LensEdge`），实测按下帧顶端读到 75/77/79 与 243/243/244
  的冠线——「按下仍然是一块有受光边的实体」，而不是一块贴纸。
- 菜单 **#4 签名交互 / hover 揭示**：三档都只作用在键自己的盒子里（pin ④ 把「只改自己那块」
  钉成像素账：585 像素、bbox 就是键盒），不会带动邻居——工艺密度不等于全局重排。
- 菜单 **#5 多层染色阴影（这里用它的反面）**：§2 判据 2「E1 不许外投影」，所以按下这一档
  不叠加阴影，只换面 + 保留一对内线；阴影只留给浮条自己（`ShadowAmbient` + `ShadowContact`，P13-d 已落地）。

## preflight（`qiaomu-design/references/preflight.md`）

- **A 反 AI 套路**：过。没有渐变白边冒充高光、没有玻璃叠玻璃、没有装饰性 glow；第三档是
  同一个 ink 的 alpha 阶梯（0.10 → 0.035），不是第三种颜色（M2 把这条钉成红）。
- **B 设计读取一致性**：过。`docs/liquid-glass.md` §5 是单一来源；两套墨源（窗口阶梯 / 调色板
  派生）各自成立，同一页里不会混用（`canvasInks` 只回答一次）。
- **C 动效抽查**：过（无动效新增）。这三档是状态而不是过渡：`reduce-motion` 下语义不变。
- **D2 中文页字号**：不适用（本轮只改材质，没有新文案；终端页文案零改动）。

## 仍待

1. **需拍板（7-10）**：按下这一档要不要带几何收档，全仓现在不一致——`shell` / `settingsui`
   有 `Scale`，这一页没有（理由：2 DIP 宽的裁切键会丢点击，属框架命中账）。
2. `terminalui` 的另外两处键（常驻便条的 ✕、失败提示无键）没有第三档问题：前者只有一档。
3. 下一个候选：`ContentAlpha()` 随透明度滑块的比例（任务书「先量再改的两个数」之一）或
   `terminalui.statusDot` 的墨在两套外观下的级数。
