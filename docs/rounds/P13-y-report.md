# P13-y 报告 · 插件窗口 bar 的三个键：悬停戴 E1，禁用只淡一道

规格：`docs/rounds/P13-y-spec.md`。表面：`.plugin-window__bar`（输出窗口 `detach.go` +
便条窗口 `pinned.go`，同一条 bar）。

## 这轮为什么让它更轻 / 更有空间感

三个键此前是**一块平涂**：既不是底也不是面——没有边、没有受光边，指针落在上面时界面
不说「这里可以按」，它只是把一小块颜色换掉。现在它们戴上 E1 的一对面（顶亮一分、底暗一分），
被禁用的那个键从**两次淡**（0.25）回到**一次淡**（0.5）。一句话：
**层级靠面差说话，可碰性靠受光边说话，不可碰靠熄灯说话**——同一条算术在设置页（P13-s/t/u）
与启动器（P13-h）已经成立，这一轮把它接到 shell 的两个插件窗口上。

## 落地

| # | 位置 | 改前 | 改后 |
|---|---|---|---|
| 1 | `internal/shell/detach.go` `barButton` 的 `Hovered()` 臂 | `button.Background(tokens.ControlHover)`（平涂） | `button = tokens.LitFace(button.Background(tokens.ControlHover))` |
| 2 | 同处 `!enabled` 臂 | `Disabled(true)` + `Opacity(0.5)`（有效 **0.25**） | 只留 `Disabled(true)`（框架那一道 0.5） |
| 3 | `internal/shell/pinned.go` 便条窗口的关闭键 | `close.Background(tokens.ControlHover)`（平涂） | `close = tokens.LitFace(close.Background(tokens.ControlHover))` |

两个窗口是**同一条** bar（同一个 8u 圆角、同一条 `Hairline` 底线、同一对 padding 常量），
所以「一个表面一轮」成立；`detach.go` 的关闭键与重跑键还共用同一个 `barButton` 闭包，
因此两个代码路径（闭包 / 便条自己的键）各有一条 pin 量着。

## 实测（`internal/shell/pluginwindowbar_test.go`，720×460 帧）

键自己的那一列（冠线 / 面 / 对侧），指针停在键上：

| 外观 | 键 | 冠线 | 面 | 对侧 |
|---|---|---|---|---|
| 浅 | 输出窗口 重跑键 | **221**（改前 220） | 220 | **215**（改前 231） |
| 浅 | 输出窗口 关闭键 | **226**（改前 220） | 220 | **218**（改前 224） |
| 浅 | 便条 关闭键 | **226**（改前 220） | 220 | **218**（改前 224） |
| 深 | 输出窗口 重跑键 | **60**（改前 40） | 53 | **49** |
| 深 | 输出窗口 关闭键 | **56**（改前 53） | 53 | **51** |
| 深 | 便条 关闭键 | **56**（改前 53） | 53 | **51** |

禁用（重跑键在飞时）的墨对比，比值开平方即墨自己的 alpha：

| 外观 | 静息对比 | 在飞对比 | 比值 | 改前比值 |
|---|---|---|---|---|
| 浅 | 30217 | 7603 | **0.50** | 0.25（1928） |
| 深 | 24678 | 6081 | **0.50** | 0.25（1544） |

**浅色重跑键的冠线只有 +1 级**（220→221）：它是 §3 那条「浅色的受光边只能是近白」的极限，
真正让它站起来的是对侧那一笔（−5）。这不是本轮新引入的问题，而是把既有事实量出来——
记进「仍待」。

出图：`/tmp/floter-P13-y-bar-{light,dark}-{rest,hover}.png`。
图上读到的一处变化：浅色 hover 帧里重跑键**底行** `(632,30)` 从 231 掉到 **215**，
同一帧的 bar 面 `(700,20)` 仍是 246——那条暗线只属于键自己，没有漏到 bar 上。

## 门槛（实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → **净**
- `go test -count=1 ./...` → **全绿**
- PASS **3001 → 3003**（+2；第三条新函数是 opt-in 的截图 dump，计入 SKIP）
- SKIP 81 → **82**（同一条 dump）｜测试函数 2213 → **2216**（+3）
- `GOOS=linux/windows/darwin go build ./...` → **三平台过**
- **V 0 / B 0 / S 0**：既有断言一条都没改（`TestTheRerunKeyDimsWhileARunIsInFlight`
  只要求「变淡」，0.5 仍然满足它）。

## 变异（4 条实跑，全部变红并逐字节还原）

- **M1** `detach.go` 的 `LitFace` 撤回平涂 → `light/detached re-run: no counter-side line —
  bottom {231 231 231} is not darker than face {220 220 221}`、`dark/detached re-run: no lit
  edge — crown {40 40 42} is not brighter than face {53 53 55}`。
- **M2** `pinned.go` 的 `LitFace` 撤回平涂 → 只有 `light/pinned close` 与 `dark/pinned close`
  红（证明两个窗口各钉各的，一条 pin 不能替另一条作证）。
- **M3** 把 `Opacity(0.5)` 加回禁用臂 → `the disabled re-run key's ink is 0.25 of the live
  one ... (idle contrast 30217, running 1928)`（浅）/ `(24678, 1544)`（深）。
- **M4** 把 `LitFace` 换成 E2 的 `EdgeHighlight` 单线（调用点）→ `no counter-side line` 红。
  **如实记录**：同一变异打到 `theme.Tokens.LitFace` 的函数体里（三条 `LensRim` 全换
  `EdgeHighlight`）**没有变红**，pin 报 `ok`。也就是说 pin 的「更接近 `LensRim`」那一半
  目前**没有被这条变异证明**——它只被「有没有一对面」那一半证明了。这条留在「仍待」第 2 条，
  下一轮第一件事是把原因查清（怀疑 `Tokens.LitFace` 不是这根键的线来源，但 M4 又证明调用点
  是），不许当成已解决。
- 还原后 `md5sum -c` 逐字节核对通过（`detach.go` `ea78a5c9…`、`pinned.go` `b647d210…`、
  `theme.go` `57d0aca1…`）。

## 抢救 stash 处置（本轮核对，无丢弃）

`git stash list` 里两条新记录 `stash@{0}` = `df5647edab`（02:42）、`stash@{1}` = `567cf0a916`
（01:42）：`git stash show -p` 两条**逐字节相同**（各 +61/−11，两个文件
`internal/launcher/{launcher,output}.go`），parent 都是 `a95274b7` = P13-x 之前的 HEAD，
而 `git diff stash@{0} 4783c62d -- internal/` 只剩一个 `headingfold_test.go`（**HEAD 有、
stash 没有**）。即：两条 stash 的内容正是 P13-x 那一轮的工作，已被 `4783c62d` 完整提交，
**无可接续内容**。结论：**已回收 —— P13-y**（本体保留不删，与 P11-ae / P12-a 同样处理）。

## 用了 `qiaomu-design`（打磨车道：`craft-loop.md` + `preflight.md`）

### 偷了什么（逐条：菜单第 N 项 → 落到哪个数值）

| 菜单 | 取了什么 | 落成的具体数值 |
|---|---|---|
| **#12 材质细节（内边框高光 / glass 折射边）** | 「受光边是一条 1px 实色线，不是渐变」 | 键的顶行 = `LensRim` `rgba(255,255,255,0.09)`：实测浅 220→**221**、深 53→**60** |
| **#12 的对侧（同一菜单项的第二半）** | 「顶亮一分必须配底暗一分，面才站起来」 | 键的底行 = `LensEdge`（深 0.20 / 浅 0.06）：实测浅 220→**215**、深 53→**49** |
| **#4 签名交互（hover 揭示）** | 面**只在被碰时存在**，静息零面 | 三个键静息 = 透明（bar 面 246/20），hover 才抬 `ControlHover`（220/53） |
| **#10 收尾彩蛋（hover 才出现的细节）** | 那条边只属于「可碰」这一状态 | 禁用臂把它**熄掉**（`Disabled(true)`，无 `LitFace`），墨只剩一半：0.50 |
| **#6 编辑细节（hairline）** | 「发丝线不许承担分层」 | bar 的 `Hairline` 底线保留不动；分层改由面差 + 一对线承担（本轮没有新增任何一条线做分组） |
| #5 多层染色阴影 | **拒绝** | §2 判据 2「E1 不许有外投影」：键长在 bar 上，加影就是把它抬成 E2 |
| #1 氛围层（噪点/纸纹） | **拒绝** | §0 硬禁令：自绘没有 blur，拿纹理冒充玻璃 |
| #11 图形资产 | **拒绝** | 键的记号是既有 glyph 集（`close` / 文本），本轮不引入新资产 |

### preflight 逐条

- **A 反套路**：本轮零渐变、零发光、零斜体、零「三等分卡片」、零左侧竖线（P-38）；
  新增的只有 3 条 1px 内线与 1 处删除的 `Opacity`。**过**。
- **A2 进攻三步**：DNA 注入见上表（3 条落到数值）；工艺密度 5 项可指认（见上表前五行）；
  两轮制——第一轮把平涂换成 E1（截图 + 量数），四视角各挑一处：艺术总监「浅色冠线只有
  +1 级，光几乎看不见」、工程师「重跑键的对侧 231 比面还亮，等于没有对侧」、甲方「三个键
  里有一个（禁用）看起来比可用的还淡两倍」、读者「浅色档三处键的边界要靠墨才认得出」；
  第二轮只修最关键的 2 处（禁用 0.25→0.5；便条键补同一条面），再复量。**过**。
- **B 设计读取一致性**：深/浅各一套主题、不混；记忆点一句话——「窗口 bar 的键在被碰时才亮起
  一条边」。**过**。
- **C 动效抽查**：本轮**不加**动效，也不动既有动效。三个键的换面是**即时**的（没有
  `ElementTransition`），与 §5 的「颜色过渡 90ms」不一致——这是**既有**状态，本轮**故意不补**：
  `ui.Tester` 的时钟是真实时间（`ui/runtime.go:506-511`，`clock` 为 nil），90~160ms 的过渡在
  `settle()` 的 20 帧里不一定走完，pin 会变成时间依赖。同理 §5 的「E2 浮层入场
  （160ms translateY + scale）」也**没有**落在这里（本轮查过 `internal/shell` 全包没有一处
  `ElementTransition`），需要先解决测试时钟再谈。记进「仍待」第 3 条，**需拍板**。
- **D2 中文页**：本轮零文案改动（i18n 一个字没动）、零字阶改动；行高/字重/中西文空格不适用。**过**。

## 台账

- **V 0**（没有一条既有断言钉着旧的平涂或 0.25）
- **B 0**（键盘模型、命中区域、`Label`、`Clicked()` 全不动；`TestTheRerunKeyDimsWhileARunIsInFlight` 仍绿）
- **S 0**（结构没变，只多/少内线与一处 opacity）
