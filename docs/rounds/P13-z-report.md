# P13-z 报告：toast 的 ✕ 戴上 E1 的一对面（并拆掉 P13-y 留下的一条假证）

规格：`docs/rounds/P13-z-spec.md`。方向：**细节打磨 / 状态反馈**，表面 = toast（弹层）。

## 这轮为什么让它更轻、更有空间感

toast 的 ✕ 此前是**一块平涂**：悬停只抬一层 `ControlHover` 洗色，没有受光边、也没有对侧，
所以它读起来「一片颜色」，不是「一块能被碰的面」。现在它戴的是同一个 toast 里动作 chip
早就戴过的那一对线（P13-j）：顶行亮一分、底行暗一分。**同一个父元素下两块面终于用同一套
层级语言说话**——面板内的一切都是 E1 面（§2 判据 1），层级靠面差 + 一对线，不靠画框。

## 落地

1. `internal/shell/notices.go:486-499`（HEAD 行号）：`dismissInk` 臂的
   `button.Background(tokens.ControlHover)` → `button = tokens.LitFace(button.Background(tokens.ControlHover))`，
   注释写明 E2 里的 E1 面、与 `barButton` / 动作 chip 同源。
2. `internal/shell/pluginwindowbar_test.go` `barAssertE1`：**绝对距离 → 对数比例**判据
   （见下节「假证」）。
3. 新增 `TestTheToastDismissKeyIsAnE1Face`（深/浅）：横向读法——同一行「键内 vs 键外 3px 的
   面板面」，把 toast 面板自身的**竖向 alpha 斜坡**约掉；顶行必须亮于键面、底行必须暗于键面。
4. 新增 opt-in dump `TestDumpP13ZToastPNGs`（`FLOTER_PNG=1`）。

## 假证（P13-y 仍待第 2 条）：根因已查明

把 `theme.go:1142` 的 `t.LensRim` 换成 `t.EdgeHighlight`，`TestThePluginWindowBarKeysAreE1Faces`
**仍然绿**（P13-y 的 M4 记录）。本轮实测到两条读数：

- 冠线从 221 → **233**，而 `barLineOver` 给的两条候选线是 `LensRim` **223** / `EdgeHighlight`
  **252**（浅色）。绝对距离：`|233−223| = 10` < `|233−252| = 19` → 「更接近 rim」，假绿。
- 根因是**覆盖倍数**：像素网格给冠线的强度是 token 的**分数或整数倍**——文字键的顶行只拿到
  LensRim 的 1/3（1 级 / 3 级），图标键的顶行拿到 **2 倍**（6 级 / 3 级，`barKeyColumn`
  落在的那一列上有两笔线）。一个乘数在绝对距离里会把 E2 的线「拉近」rim。

修法：比**对数比例** `|log(冠线抬升 / 候选抬升)|`，乘数被约掉。
实测：文字键 `log(1/3) = −1.10` vs `log(1/32) = −3.47` → rim；图标键 `log(6/3) = +0.69` vs
`log(6/32) = −1.67` → rim；变异（E2 冠线）`log(13/3) = +1.47` vs `log(13/32) = −0.90`
→ **highlight，红**。守卫保留：两条候选线的抬升必须 > 0 且 `EdgeHighlight` 必须更高。

## 门槛（实测）

| 项 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 |
| `go test -count=1 ./...` | 全绿，FAIL 0 |
| `--- PASS` 计数 | 3003 → **3004**（+1） |
| `--- SKIP` | 82 → **83**（+1，opt-in dump） |
| 测试函数 | 2216 → **2218**（+2） |
| 三平台 `go build` | 过 |

## mutation（4 条实跑，全部变红并 `md5sum -c` 逐字节还原）

- **M1** `notices.go` 撤掉 `LitFace`（回到平涂）→
  `light: the hovered ✕ has no counter-side line — its bottom row is -21 levels against the pane, its face -25`；
  `dark: the hovered ✕ has no lit edge — its top row is +33 levels against the pane, its face +33`。
- **M2** `theme.go:1142` `LensRim` → `EdgeHighlight`（E2 档，**P13-y 那条没红的变异**）→
  `light/detached re-run: the lit edge {233 233 234 254} is 13 levels over the face — 4.33 of LensRim's 3, 0.41 of EdgeHighlight's 32: the float rung, not the component's — a key on a bar is E1, not E2`。
- **M3** 只戴线不给面（`LitFace(button)`）→ 两条新断言同时红（浅 `-21 / -25`、深 `+33 / +33`）。
- **M4** `theme.go:1143` `LensEdge` → `LensRim`（上下都亮）→
  `light/detached re-run: the hovered key has no counter-side line — bottom {221 221 222 254} is not darker than face {220 220 221 254}`。
- 还原核对：`notices.go 0f1a4adb…`、`theme.go 57d0aca1…`、`pluginwindowbar_test.go ec7ba1b0…` 三行 `成功`。

## 出图

`/tmp/floter-P13-z-toast-light.png`、`/tmp/floter-P13-z-toast-dark.png`（悬停帧）。
图上读到的一处变化：深色帧 x=683 那一列，冠行 y=66 = **77**、键面 y=74 = **64**、底行 y=86 = **49**
（面板面 23）——1px 的亮线 + 1px 的暗线，中间是 `ControlHover` 的面。
浅色帧同列 y=66 = **226**、y=74 = **217**、y=86 = **202**。

## 台账

- **V 0**：没有一条既有断言钉着平涂的旧值（`TestTheToastFloatsOnTheElevationLadder` 只钉影）。
- **B 0**：关闭语义（`c.CloseToast`）、`Label(DismissNotice)`、命中盒、i18n 文案全不动；
  `TestTheToastIsDrawnByTheAppsOwnViewport` / `TestTheToastActionRunsAndClosesTheToast` 仍绿。
- **S 0**：结构没变（一块面多两条内线）。
- **pin 修复 1 处**（`barAssertE1` 的档位判据）：不属于 V/B/S——它把一条**从没红过**的断言
  换成能红的判据，旧断言既没被删也没被放宽，是**收紧**。

## 用了 `qiaomu-design`（打磨车道：`craft-loop.md` + `preflight.md`）

### 偷了什么（菜单第 N 项 → 落到哪个数值）

| 菜单 | 取了什么 | 落成的具体数值 |
|---|---|---|
| **#12 材质细节** | 「受光边是一条 1px 实色线，不是渐变」 | ✕ 的顶行 = `LensRim`：深色实测 y=66 **77**（面 64） |
| **#12 对侧** | 「顶亮一分必须配底暗一分」 | 底行 = `LensEdge`：深色 y=86 **49**（面 64） |
| **#4 签名交互** | 面**只在被碰时存在**，静息零面 | 静息 ✕ 透明（面板面 23/202），悬停才抬 `ControlHover` + 一对线 |
| **#10 收尾彩蛋** | 「hover 才出现的细节」 | 那条亮线只属于可碰状态；`Label` 与 glyph 墨同帧转到 `TextStrong` |
| **#6 编辑细节** | 「hairline 不许承担分层」 | toast 的边框与面板影一像素未动，分层由面差 + 一对线承担 |
| #1 氛围层 | **拒绝** | §0 硬禁令：自绘无 blur，纹理冒充玻璃 |
| #5 多层染色阴影 | **拒绝** | §2 判据 2：E1 不许外投影，键长在 E2 面里 |
| #11 图形资产 | **拒绝** | 记号仍是既有 `close` glyph，不引入新资产 |

### preflight 逐条

- **A 反套路**：零渐变、零发光、零斜体、零三等分卡片、零左侧竖线；新增 2 条 1px 内线 + 1 处
  判据收紧。**过**。
- **A2 进攻三步**：DNA 注入见上表（3 条落到数值）；工艺密度 5 项可指认；两轮制——第一轮加
  `LitFace`（截图 + 量数），四视角挑刺：艺术总监「✕ 和动作 chip 在同一个 toast 里居然不是同一
  种面」、工程师「这条 pin 的档位判据从来不会红」、甲方「鼠标落在 ✕ 上没有『可以按』的反馈」、
  读者「toast 右侧那块颜色说不清是按钮还是底色」；第二轮只修两处（pin 判据 + 上表数值复量）。
  **过**。
- **B 设计读取一致性**：深/浅各一套主题；记忆点一句话——「toast 的 ✕ 被碰时才亮起一条边」。
  **过**。
- **C 动效抽查**：本轮**不加**动效，也不动既有入场（`toastEnter` 220ms 弹簧一像素未改）。
  ✕ 的换面仍是即时的——与 §5「颜色过渡 90ms」不一致，**既有**状态，理由同 P13-y（`ui.Tester`
  的时钟是真实时间，过渡会让 pin 变成时间依赖）。**不算过，是已知未落地，记在仍待第 3 条**。
- **D2 中文页**：本轮零文案、零字阶改动（i18n 一个字没动）。**过**。

## 抢救 stash 处置（本轮核对，无丢弃）

`stash@{0}` = `df5647edab`（02:42）、`stash@{1}` = `567cf0a916`（01:42）：两条 tree 都是
`9e30288`（**逐字节相同**），parent = `a95274b7`（P13-x 之前的 HEAD）；`git diff HEAD stash@{0}`
只剩 `internal/shell/detach.go` / `pinned.go` 的 P13-y 改动（HEAD 有、stash 是旧版）与 P13-x/P13-y
的文档与测试（HEAD 有、stash 没有）——即内容已被 `4783c62d` + `fec8b8b9` 完整覆盖，
**无可接续内容**。结论：**已回收 —— P13-z**（本体保留不删，同 P11-ae / P12-a / P13-y）。

## 仍待（P13-z，追加在 P13-y 之后）

1. **浅色档 ✕ 的冠线读不出独立信号**：浅色帧 y=66 = 226 而键面 217（+9），但这 9 级里混着
   「键盒起始在 65.8 这种分数行」造成的覆盖率差异（同一行里 20%/80% 混合面板色）。深色帧干净
   （77 / 64 / 49），浅色帧的「亮线」与「覆盖率」**分不开**——需要一张整数对齐的 fixture
   （把 toast 放在整像素 y 上）才能把浅色的冠线单独钉住。
2. **图标键的冠线是 token 的 2 倍**（6 级 / 3 级）：`barKeyColumn` 落在的那一列上有两笔线
   （键自己的顶行 + 相邻元素/bar 的同色线）。对数比例判据已能容它，但**为什么是两笔**没查清，
   下一轮可查（会影响任何想按绝对值得出结论的 pin）。
3. **P13-y 仍待第 3 条原样有效**：换面无过渡、§5 的 E2 入场 160ms 一处未落地，**需拍板**
   （要先解决 `ui.Tester` 的时钟）。
4. **P13-y 仍待第 4 条原样有效**：浅色 `LensEdge` 0.06 → 0.08 **需拍板**（会同时动设置页/启动器
   所有 E1 面）。
5. P13-x 第 1-3 条、P13-w 第 2/4 条、P13-u 第 1/2/3 条、P13-t 第 3 条、P13-s 第 1/2 条、
   P13-r 第 1 条、P13-q 第 1 条、P13-p 第 1/2 条、P13-k 第 1/2 条、P13-c 四条、P13-h 第 9 条
   原样有效（均需拍板）。
