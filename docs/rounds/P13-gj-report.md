# P13-gj · 浮层的顶边是三段，而「谁在分离」随地面换人（报告）

**一句话**：E2 的顶边不是 E1 那种「分离 + 冠」两条线——它是**三行**，浅色档由**影**
（30 级）一个人把浮层从纸面上拿开、描边只高 5 级、冠线被纯白封顶（+4）；深色档反过来，
影只拿得走 **4** 级，边界由那 1px 描边（+24）扛，冠线 +56。

- 规格：`docs/rounds/P13-gj-spec.md`
- 合约：`docs/liquid-glass.md` **§6.155**（新增）+ §6.154 的 154-1 单元格修正
- 台账：**V 0 / B 0 / S 0**（产品代码字节零改动）
- 出图：`/tmp/floter-P13-gj-floatedge-{light,dark}.png`（720×520 帧，3× = 2160×1560）

## 1. 量（真 toast，settings 面 720×520，默认 47% 框 alpha，straight luma）

| 外观 | 地面（无浮层那一帧） | 影（紧贴盒上沿） | 描边行 | 冠线 | 面 |
|---|---|---|---|---|---|
| 浅 | 248 | **218** | 223 | **255** | 251 |
| 深 | 32 | **28** | 52 | **82** | 26 |

| 外观 | 影拿走 | 描边 − 影 | 冠 − 面 | 面 − 地面 | 盒子的分离靠谁 |
|---|---|---|---|---|---|
| 浅 | **30** | +5 | **+4**（255 = 纯白，已封顶） | **+3** | **影** |
| 深 | **4** | **+24** | **+56** | **−6**（面比纸更暗） | **描边行** |

**量法上的两个坑**（都写进了测试注释）：① 框是 47% 半透明的，tester 的缓冲是**预乘**的，
alpha 不同的两行不能直接比亮度，必须先把框自己的 alpha 除回去；② 盒子的边界只能靠
**alpha 台阶**定位（196 vs 127），不能靠亮度——浅色档盒沿只比纸面高 3 级，靠亮度会认错行
（第一版探针就先把 ✕ 自己的冠线当成了盒沿）。

## 2. 三句话（写进 §6.155）

1. **纸面上的层级全在影上。** 浅色浮层的面是 251、纸面 248：面差 3 级。冠线压在纯白上，
   只剩 4 级。把这张纸从桌面上拿开的是影的 **30 级**——浅色档的「空间感」只有一个来源。
2. **夜里影干不了这件事。** 深色地面本身只有 32，影子只拿得走 4 级（再压就是黑带，不是深度），
   边界交给 1px 描边（+24），冠线照旧点亮自己（+56）。E2 的描边在夜里不是画框，是唯一的分离。
3. **冠线与描边永远不许并成一条**（相差 32 / 30 级）。把描边的墨换成 `EdgeHighlight`
   就是 §1 禁令 3 的白边高光——M3 实测：纸面 255 = 255 两行白带。

**两条被实测挡掉的错路**：① 「浅色浮层不够亮，提亮面」——面 251 离纯白只剩 4 级，提亮只会
把冠线整条吃掉；② 「夜里影子太淡，加厚影」——地面 32 上再压黑就是黑带。要更多分离只能动
面（材质档，需拍板），不许动这两条线。

## 3. 落地

| # | 位置 | 内容 | 状态 |
|---|---|---|---|
| 155-1 | `internal/shell/floatedge_test.go`（新） | 五行字面量 `{248, 218, 223, 255, 251}` / `{32, 28, 52, 82, 26}` | 已落地（pin） |
| 155-2 | 同上 | `冠 − 描边 ≥ 16`（32 / 30） | 已落地（pin） |
| 155-3 | 同上 | `冠 − 面 ≥ 3`（+4 / +56，两种外观同号） | 已落地（pin） |
| 155-4 | 同上 | 浅：影 ≥ 24 **且** 影 > 描边；深：影 ≤ 8 **且** 描边 − 影 ≥ 16 | 已落地（pin） |
| 155-5 | 同上 | 浅 `面 − 地面 ∈ [0, +6]`；深 `面 < 地面` | 已落地（pin） |
| 155-6 | 同文件 opt-in dump | `FLOTER_P13GJ_DUMP=1` → 两张 3× PNG | 已落地（证据） |
| 155-7 | `docs/liquid-glass.md` | §6.155 新增；§6.154 的 `\|边 − 地面\|` 改成 `abs(边 − 地面)` | 已落地 |

**产品代码零像素改动**：`internal/shell/notices.go:370`（`Border`）、`:398`/`:401`
（`InsetShadow`）、`:400`（`ElevThrow`）一个字节没动，`git status` 只有测试与文档。

**这张图上看到的一处事实**（3× dump，浅色那张）：浮层顶边上方是一条**很宽的暗带**
（不是一条线），暗带最深处比纸面低 30 级；紧接着一行近白发丝，然后是几乎和纸面同亮的
面——「这张纸是浮起来的」全部由那条暗带说出来。深色那张同一位置，暗带几乎没有（4 级），
改成两行由暗到亮的台阶。两张图并排看，就是本轮的结论。

## 4. 门槛（全实测）

- `gofmt -l cmd internal` **空**；`go vet ./...` **净**；`GOOS=windows go vet ./...` **净**；
  `GOOS=darwin go vet ./...` **净**；`go build ./...` 三平台 **过**。
- `go test -count=1 ./...` **0 FAIL**；`--- PASS` **3483**（基线 3482，+1 = 本轮 pin；
  dump 是 opt-in `t.Skip`，不计）；测试函数 **2646**（+2：pin + dump）。
- **开工时修掉一处 HEAD 上的红**：`internal/ci` 的
  `TestTheLiquidGlassContractSupersedesTheRaycastOne` 在 `d46e80ae` 上就是红的——
  §6.154 的 154-1 单元格里 `|边 − 地面| ≥ 24` 的竖线把表格劈开，状态列读到「边 − 地面」。
  改文档（`abs(...)`），断言一个字没动。

## 5. mutation（3 条实跑，全红，`md5sum -c` 还原 + `touch` 复跑）

`internal/shell/notices.go` 原件 md5 `021aa9cf163b442fafa6035e404e6597`，三条都还原成功。

- **M1**（`:401` 冠线 `EdgeHighlight` → `InputStroke`）→ 红：
  ```
  light: the top edge reads {ground:248 shadow:218 border:223 crown:223 face:251}, want {ground:248 shadow:218 border:223 crown:255 face:251}
  light: the crown sits +0 levels over the border row (223 vs 223), want ≥ 16
  light: the crown sits -28 levels over the face (223 vs 251), want ≥ +3
  dark:  the crown sits +0 levels over the border row (52 vs 52), want ≥ 16
  ```
- **M2**（`:400` `ElevThrow` → `ElevContact`，只留接触影）→ 红：
  ```
  light: the top edge reads {ground:248 shadow:244 border:223 crown:255 face:251}, want {ground:248 shadow:218 …}
  light: the cast shadow takes only 4 levels off the page (248 → 244), want ≥ 24
  ```
- **M3**（`:370` 描边的墨换成 `EdgeHighlight`，= 140-5 的候选搬到 E2）→ 红：
  ```
  light: the top edge reads {ground:248 shadow:218 border:255 crown:255 face:251}
  light: the crown sits +0 levels over the border row (255 vs 255), want ≥ 16
  light: the border row separates 37 levels against the shadow's 30 — on paper the hairline is the weaker reading and must stay so
  dark:  {… border:82 crown:82 …} the crown sits +0 levels over the border row (82 vs 82)
  ```

## 6. 偷了什么（qiaomu-design，逐条）

1. `craft-loop.md` 工艺密度菜单**第 5 项「多层染色阴影」**→ 落成「影必须**两层**在场」的
   可数判据：纸面上 env+contact 一起拿走 **30** 级（`地面 − 影 ≥ 24`），夜里只许拿走
   **≤ 8** 级（实测 4）。M2 把 ambient 删掉后纸面掉到 4 级，这条判据当场变红。
2. `craft-loop.md` 菜单**第 3 项「边界与描边」**→ 落成「描边和冠线是两行、两句话」：
   `冠 − 描边 ≥ 16`（32 / 30），并把它作为 140-5 那条禁令在 E2 上的等价物。
3. `craft-loop.md` 菜单**第 12 项「材质细节：内边框高光」**→ 落成「冠线是**面**上的线，
   不是盒子上的线」：`冠 − 面 ≥ 3`，且浅色档明确记下它在 255 **封顶**（头寸只剩 4 级）。
4. `craft-loop.md` 第三节「**两档看图**」→ 用「有浮层 / 没浮层」两帧在同一行同一列各读一次
   代替凭感觉争「影子够不够重」：影的深度成了同一像素的两次读数。
5. `apple-fluid-interfaces.md` 的「材质回答层级」→ E2 的 elevation 在纸面上由**影独占**
   （面差 3 级），这条写成判据 `面 − 地面 ∈ [0, +6]`，堵住「靠提亮面冒充浮起」那条路。
6. `preflight.md` **A（反 AI 套路）**：本轮拒绝的正是「给每条边都加一条亮线」——
   M3 实测浅色上 255 = 255 两行白带，深色上描边反超影。
7. `preflight.md` **B（设计读取一致性）**：数值全部来自既有 token
   （`Float` / `InputStroke` / `EdgeHighlight` / `ElevThrow`），没有新开一套。

## 7. preflight 逐条结论

| 项 | 结论 |
| --- | --- |
| A 反 AI 套路 | **过**：M3 把「亮描边」实测判掉（两行白带 + 描边反超影），无新增 glow / 无白边高光 |
| B 设计读取一致性 | **过**：§6.155 只读 §2/§3 既有 token 与既有 `ElevThrow` 几何，结论写回单一来源 |
| C 动效抽查 | **不适用**：不碰动效。入场弹簧只被当成「settle 之后才量」的前置条件（`toastEnter` 读两次），未改一帧 |
| D2 中文页 | **不能过、沿用既有拍板**：`tokens.Caption` 9.9px / 正文 10.8px 仍低于 14px 判据，属「需拍板」字号账，本轮不碰 |

## 8. 仍待

1. **需拍板（沿用）**：138-3、138-4、§6.111 / 113-5 / 114-4 / 129-5 / 151-4、字号账、
   36px 盒、终端弧 S1b / S1c / S2 尾。
2. **本轮销账 1 条**：P13-gi「仍待」第 4 条（E2 顶边从未读进帧）→ §6.155。
3. **本轮新增需拍板：无**（两条错路都已被实测挡掉，不需要用户选）。
4. **下一格（首选）**：同一把尺子量 **E2 的下沿**——本轮只读了顶边，而 E2 的底边是
   `EdgeShadow`（暗线）+ 接触影，它与顶边**不是同一套**（§3 的表里 `EdgeShadow` 从未被
   帧内读过）；其次是 `uninstall` 面板（浮在设置页上的 E2，`settingsui/uninstall.go:174`）
   与 toast 是否同一套读数。
