# P13-aa 报告 · 启动器两条「带」上的四个小面：各缺半套 E1

规格 `docs/rounds/P13-aa-spec.md`。`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律走
通用队列的「状态反馈」。这一轮的对象是**同一族最后四块**「能被指针碰、但只说了半句话」的面。

**这轮为什么让它更轻 / 更有空间感**：一块面「能被碰」在这套语言里只有一句话——顶亮一分、
底暗一分。这四块此前要么一条线都没有（悬停只有一层洗色），要么只亮半边（只有顶边），
于是同一个面板里的可碰面说了两种话。补齐之后，**可碰 = 一对线、不可碰（透明面 / 图标墨）
= 一条线都不戴**，用户不必去猜哪块能按。

## 落地（三处，四个面）

| # | 位置 | 改前 | 改后 |
|---|---|---|---|
| 1 | `internal/launcher/output.go:517` `control` 闭包（捕获输出页页头两个 28u 方键 `↗` / `⧉`） | `button.Background(tokens.ControlHover)`：一层洗色，**一条线都没有** | `button = tokens.LitFace(button.Background(tokens.ControlHover))`，**不补 `Border`**（面差就是边界，P13-j 同判据） |
| 2 | `internal/launcher/power.go:191` cancel 键悬停臂 | `plane, edge = ControlHover, ControlEdge` + `Border(1, edge)`：一块被画框圈住的平涂 | `if plane.A > 0 { button = tokens.LitFace(button) }` |
| 3 | `internal/launcher/power.go:200` execute 键（暖色抬升面） | `Shadow(ShadowContact)` + `InsetShadow(0,1,0,0, LensRim)`：**半对**（只有顶边） | `Shadow` 保留，顶/底由 `tokens.LitFace` 给 |
| 4 | 同处的**静息** cancel（`plane == ui.Transparent`） | 透明面 | **仍然不戴**：`plane.A > 0` 的判据把它挡在外面（§1 禁令 3：透明面上的白线就是「类玻璃边」；同 §6.2b-4 的 9-20b），反面 pin 守着 |

**为什么这四块此前三轮 census 都照不到**：P13-p 换成的材质 census 判据是「面 **+ 边** ⇒
必须有一对线」。第 1 块没有任何边（所以「面 + 边」这条前提不成立），第 2、3 块的边写在
变量 `edge` / 走 `InsetShadow`，单行 grep 与「`Border(1,` 同不同行」都数不到它们。这是
census 的第二个盲区：**没有边框的面不在视野里**，本轮把它记在仍待第 1 条。

## 实测（同一帧、同一像素列，深/浅两套外观）

| 面 | 外观 | 面 | 顶内线 | 底内线 | 两侧列 |
|---|---|---|---|---|---|
| 输出页 `↗` 键（悬停） | 浅 | 226 | **229（+3）** | **213（−13）** | 226 = 面 |
| 同上 | 深 | 56 | **74（+18）** | **45（−11）** | 56 = 面 |
| 电源 execute（静息） | 浅 | 244,220,204 | 245,223,209（不动） | **230,207,192（−14）** | = 面 |
| 同上 | 深 | 72,55,48 | 89,73,67（不动） | **58,44,38（−14）** | = 面 |
| 电源 cancel（悬停） | 浅 | 222 | **225（+3）** | **209（−13）** | = 面 |
| 同上 | 深 | 78 | **94（+16）** | **62（−16）** | = 面 |
| 电源 cancel（静息，反面） | 浅 / 深 | 248,235,227 / 49,40,37 | 同面（**不亮**） | 同面 | — |

改前这六行里的「顶 / 底 / 两侧」全部**逐像素等于面**（226 / 56 / 222 / 78 / 244 / 72），
也就是说这些面在此之前没有任何一条线。

出图（`FLOTER_PNG=1`）：`/tmp/floter-P13-aa-strip-{before,after}-{light,dark}.png`、
`/tmp/floter-P13-aa-power-{before,after}-{light,dark}.png`。像素 diff 的 bbox：

- strip：`(648,60)-(675,87)` —— **恰好是那个 28u 键自己的盒子**（28×28）；
- power：`(563,417)-(702,438)` —— 恰好是两枚 chip 的矩形（高 22 = chip 24 去掉抗锯齿边）。

**图上看到的一处变化**：`power-after-dark.png` 里 cancel chip 的下沿从「与带子同色」
变成一条暗线（62,56,54 对 78,70,68 的面），同一帧里 execute chip 的下沿也第一次有了
一条对侧线——两块 chip 从「印在带子上」变成「立在带子上」。

## 门槛（全部实测）

- `gofmt -l cmd internal` 空 | `go vet ./...` 净 | `GOOS=windows/darwin go vet ./...` 净 |
  三平台 `go build ./...` 过
- `go test -count=1 ./...` 全绿；PASS **3004 → 3010（+6）**、FAIL **0**、
  SKIP **83 → 84（+1 opt-in dump）**、测试函数 **2218 → 2221（+3）**
- **V 0 / B 0 / S 0**：没有一条既有 pin 需要改断言（四处此前都没有像素 pin）

## 变异（4 条，全部实跑变红，`md5sum -c` 逐字节还原，`touch` 后复跑绿）

- **M1** 撤掉 `output.go` 的 `LitFace` → 报
  `the pin key's top inner line {226 226 227 255} is not brighter than its own face {226 226 227 255}`
  （深色同一条，`{56 56 57 255}`）。
- **M2** 只让 execute 戴线（`if execute && plane.A > 0`）→
  `the hovered cancel chip's top inner line {222 210 203 255} is not brighter than its own face`
  （深 `{78 70 68 255}`）。
- **M3** execute 退回半对（顶边保留、撤对侧）→
  `the execute chip's bottom inner line {244 220 204 255} is not darker than its own face`（深 `{72 55 48 255}`）。
- **M4** 让透明静息面也戴线（`plane.A >= 0`）→ 反面 pin 报
  `the cancel chip wears a lit edge at rest ({249 237 230 255} against the band's {248 235 227 255})`（深 `{68 59 57 255}` 对 `{49 40 37 255}`）。

## 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

- **第 12 项「材质细节 · 内边框高光 / glass 折射边」** → 落成顶内线 1px `LensRim`
  （`tokens.LitFace` 的 `InsetShadow(0, 1, 0, 0)`）：实测浅色 226 → **229**、深色 56 → **74**。
  它是**实色 1px 线**，不是渐变白边（§1 禁令 3）。
- **第 12 项的对侧（菜单没写，但「材质细节」的完整写法要有对侧）** → 底内线 `LensEdge`
  （浅 0.06 / 深 0.20）：浅 226 → **213（−13）**、深 56 → **45（−11）**。
- **第 5 项「多层染色阴影」** → execute 键保留 `ShadowContact` 那一层接触影（参考的
  `--glass-raised-shadow`），**不新增第二层**：§2 判据 2 说面板内 E1 不画外投影，
  而这块是暖色抬升面，影是它身份的**唯一**来源，多一层就是「浮层」。
- **第 6 项「编辑细节 · hairline」** → 反过来用：cancel 键静息是透明面，**不加** hairline，
  由反面 pin 钉住（`wears a lit edge at rest` 会红）。
- **第 3 项「品牌 focus 环」**：本轮**不适用**（没碰 focus 链），如实记录。

## preflight.md 逐条

- **A 反 AI 套路**：过。受光边是实色 1px（不是渐变白边）；无 glow、无紫蓝渐变；
  无斜体（用户偏好 P-1）；没有新增装饰性卡片。
- **B 设计读取一致性**：过。四处只读 `theme.Tokens`，一个字面色值都没写；
  `LitFace` 仍是唯一的一对面函数（三处调用同一个，没有就地手写 `InsetShadow`）。
- **C 动效抽查**：过。本轮不加动效；cancel 键原有的 120ms 颜色过渡保留，pin 在
  `Advance(300ms)` 之后读数（settled），所以量到的是材质不是过渡中间帧。
- **D2 中文页**：过。文案零改动（`PowerCancel` / `PowerRestartAction` / `HistoryPin` /
  `OutputDetach` 四个标签一个字没碰），中文排版零变化。

## 仍待（P13-aa，追加在 P13-z 之后）

1. **census 的第二个盲区：没有边框的面**。`internal/settingsui/facecensus_test.go` 的判据是
   「面 + 边 ⇒ 必须有一对线」，所以**没有边**的面永远不进视野（本轮四处都是这样被漏掉的）。
   要补的是一条反向判据：**「面」出现在 `Hovered()` / `Pressed()` 臂里 ⇒ 也必须有一对线**，
   允许表写清例外（行悬停 `RowHover` 是参考自己的平涂、星标是墨不是面）。下一轮先量再改。
2. **`internal/shell` 与 `internal/launcher` 的 `Pressed()` 臂还没进过同一张账**：
   本轮只扫了悬停。`rowControl` / `iconPlate` 的按下臂是 `ControlPress + Scale(0.92)`（只有面、
   没有线），要不要在按下时也保留一对线**需拍板**（按下是「沉进玻璃」，参考自己也去掉了影）。
3. P13-z 第 1/2 条（浅色 ✕ 冠线的整数对齐 fixture、图标键冠线 2 倍的来源）、
   P13-y 第 3/4 条、P13-x 第 1-3 条、P13-w 第 2/4 条、P13-u 第 1/2/3 条、P13-t 第 3 条、
   P13-s 第 1/2 条、P13-r 第 1 条、P13-q 第 1 条、P13-p 第 1/2 条、P13-k 第 1/2 条、
   P13-c 四条、P13-h 第 9 条原样有效（均**需拍板**）。
