# P13-bp 报告 · 插件窗口列表臂的行盘，这一族最后一块没戴 E1 一对线的面

- 规格：`docs/rounds/P13-bp-spec.md`
- 合约：`docs/liquid-glass.md` **§6.34**（4 行：3 `已落地` / 1 `已落地（复核）`）
- 表面：**插件窗口（输出列表臂）**（`internal/shell/detach.go`）
- 台账：**V 0 / B 0 / S 0**
- 设计读取（skill 第 0 步）：**插件窗口的结果列表，面向刚跑完一条命令的用户，
  用「轻便液态玻璃 / 安静事实表」的语言，倾向 E1 面 + 一条受光边的材质家族。**
  三拨盘（中文）：视觉冒险度 **4**（功能型只读列表，越高越像装饰）、动效强度 **2**
  （列表本身不动效；本仓只做颜色过渡）、信息密度 **6**（一行标题 + 一行灰线）。
  功能契约：**这页存在的理由是「那条命令吐出了什么」；用户 3 秒内要看清每一条的标题
  与副标题；必须能选中复制、能翻页。** 本轮只改墨与面，三条契约一个字没动。

## 一、这轮为什么让它更轻、更有空间感

不是「加一条白线」，是**同一份数据在两个表面上是两种东西**。启动器列表里的结果行是
E1 的一块面（§6.1 第 1-2 行，P12-b 落地）；同一份 emit 出来的 `Row` 在插件窗口里画成
了一块**平涂**——四条边一条线都没有。用户在两处看到同一行数据，一处「站得住」，
一处「是一块颜色」。补上一对线之后，这块盘第一次**指得出方向**：顶边受光、底边压暗，
读起来是「一张贴在窗口上的纸」，而不是「一块浅一点的矩形」。层级不是靠加框、加底、
加阴影做的——`LitFace` 只加两条 1px 内线，几何一个字节不动。这正是轻便液态玻璃这一支
的做法（`docs/liquid-glass.md` §2 判据 2：E1 不许有外投影）。

## 二、先量（改前，`ui.Tester`，720×460，窗口自己的地面 `tokens.Opaque`）

`plugindowListProbe` 从帧里**读**出盘子的矩形（从标题盒向外走到面「回到地面」的地方，
不写死 padding 常量），再读五个像素：

| 外观 | 地面 | 冠线 | 面 | 底内线 | 左列 | 右列 |
|---|---|---|---|---|---|---|
| 浅 | 246 | **229** | **229** | **229** | 229 | 229 |
| 深 | 20 | **44** | **44** | **20** | 44 | 44 |

三个结论：

1. **这台盘的冠线、面、底内线三读同值**（浅 229 / 深 44）——`LitFace` 的两条 1px 内线
   一条都不存在，这个面上**只有一块底**。
2. 浅色档它比窗口地面**暗 17 级**（一行灰块）；深色档面 44 对地面 20 差 **+24**，
   但四条边仍然一条线都没有。
3. 这块盘**两侧也没有线**（浅 229 / 深 44 = 面本身）——所以它不是「画框分层」的
   受害者，它是**完全没有层**。

改后（同一探针，同一帧）：

| 外观 | 地面 | 冠线 | 面 | 底内线 | 左/右列 |
|---|---|---|---|---|---|
| 浅 | 246 | **235（+6）** | 229 | **216（−13）** | 229 = 面 |
| 深 | 20 | **63（+19）** | 44 | **45** | 44 = 面 |

**盒子的位置与尺寸逐字节相同**：`13 52 694 46}` 两轮一字不差。

## 三、落地

| # | 位置 | 旧 | 新 |
|---|---|---|---|
| bp-1 | `shell/detach.go` `detachedList` 的行盘 | `Background(tokens.Control)` 后直接 `Children` | `plate = tokens.LitFace(plate)`：顶 `LensRim` + 底 `LensEdge` |
| bp-2 | 同处的几何（`Space(0.5)` gap / `Space(2)` / `Space(2.5)` padding / `RadiusSM` / `FillWidth`） | — | **一个像素不动** |
| bp-3 | `internal/ci/flatface_contract_test.go`（新） | 无 | 面级 census：`Background(<面 token>)` + `Padding(` + `Radius(` ⇒ 必须有一条线 |
| bp-4 | `internal/shell/plugindowlistface_test.go`（新） | 无 | 5 条像素判据 × 深浅两套 + 1 条 opt-in 出图 |

### 这一族的 census 为什么必须换判据（bp-3 的理由）

这块盘此前对**两张**材质账都不可见，因为它们判的都是「面 **+ 边**」：

- `internal/settingsui/facecensus_test.go`（P13-p）要求函数体里同时有 `Border(1` 或
  `InsetShadow(`
- `internal/ci/litarm_contract_test.go`（P13-ab）只扫 `Hovered()` / `Pressed()` 臂

而这块行盘**既没有边框、也没有指针臂**（列表不可选）——它是两张账本的共同盲区。
P13-aa 的「仍待」已经把这条写明：「census 还缺一条**反向**判据……这一族现在只能靠
人肉找。」`flatface_contract_test.go` 就是那条反向判据，按**面的形状**（有面 + 有
padding + 有圆角）而不是指针状态来判。**回测**：把 bp-1 撤掉，它点名
`shell.detach.go:677` 变红（见「四、mutation」的 M4）。

## 四、门槛（全部实跑）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 3205
GOOS=linux/windows/darwin go build ./... → 三平台过
```

| 项 | 基线（本轮前） | 本轮 | 差 |
|---|---|---|---|
| PASS | 3203 | **3205** | **+2** |
| FAIL | 0 | **0** | 0 |
| SKIP | 114 | **115** | +1（opt-in 出图） |
| 测试函数 | 2355 | **2358** | +3 |

新增 pin 文件 2 个：`internal/shell/plugindowlistface_test.go`（1 条像素 pin + 1 条
opt-in 出图）、`internal/ci/flatface_contract_test.go`（1 条 census）。

## 五、mutation（4 条实跑，全部变红并逐字节还原）

`internal/shell/detach.go` 的 md5 基线 `bd7871266758e7b9991a8df3dfecaa6b`，
每条跑完都 `md5sum -c` 核对 + `touch` 后复跑绿。

1. **M1 · 撤掉那一对线**（回到改前）：`TestThePluginWindowListRowWearsTheE1Pair` 红，
   原始报错串：
   ```
   light: the plate's top inner row is not lighter than the face: {229 229 230 254} vs face {229 229 230 254} (want the 0.09 rim)
   light: the plate's bottom inner row is only 0 levels under the face ({229 229 230 254} vs {229 229 230 254}); want the counter-side shade
   ```
   （深色档同样红；「面仍离地面 ≥6 级」与两侧判据保持不变绿。）
2. **M2 · `LitFace` → `Lens`**（四层透镜，环画在最后、压住 rim）：红，报错串：
   ```
   light: the plate's top inner row is not lighter than the face: {229 229 230 254} vs face {229 229 230 254}
   light: the plate's left column carries a line ({216 216 216 254} vs face {229 229 230 254})
   light: the plate's right column carries a line ({216 216 216 254} vs face {229 229 230 254})
   dark: the plate's left column carries a line ({60 60 62 252} vs face {45 45 47 252})
   dark: the plate's right column carries a line ({60 60 62 252} vs face {45 45 47 252})
   ```
   这一条正是 §6.2b-2 判过的「环与 rim 落在同一行、把受光边压住」的复现。
3. **M3 · `LitFace` → 裸 `InsetShadow(0, 1, 0, 0, LensRim)`**（只有顶边的「半对」，
   P13-aa 在电源 execute 键上判过的形状）：红，报错串：
   ```
   light: the plate's bottom inner row is only 0 levels under the face ({229 229 230 254} vs {229 229 230 254}); want the counter-side shade
   ```
4. **M4 · census 的回测**：把 bp-1 从 `detachedList` 里删掉后跑
   `go test -count=1 -run TestNoPaddedPaneIsFlat ./internal/ci/` → 红，报错串：
   ```
   shell.detachedList:shell/detach.go:677: this pane paints a face with padding and a corner
   but no edge at all — it is a flat block of colour, not a surface on the z ladder
   (docs/liquid-glass.md §2: a face wears a lit edge on top and a counter-side line below,
   theme.Tokens.LitFace). Paint the pair, or add the entry to flatFaceAllow with the contract line.
   ```

## 六、出图与眼睛复核

`FLOTER_P13BP_DUMP=1`（`before` 由 M1 注入取得）：

```
/tmp/floter-P13-bp-windowlist-light-before.png   /tmp/floter-P13-bp-windowlist-light-after.png
/tmp/floter-P13-bp-windowlist-dark-before.png    /tmp/floter-P13-bp-windowlist-dark-after.png
```

**这张图上看到的一处变化**：`dark-before` 里 `y=52..97` 那块行盘（`x=13..707`）是一条
与窗口底几乎同色的暗块（面 44 对地面 20），四条边看不清；`dark-after` 同一块盘的
**顶边**出现一条明显更亮的线（`y=52` 处 **63**，比面亮 19 级），底边 `y=97` 回到地面
（45 → 20）。`light-after` 里同一块盘的顶边从 229 抬到 **235**、底边压到 **216**——
一块灰块第一次分成上亮下暗两半。盘的包围盒 `13 52 694 46}` 两轮相同。

## 七、skill（qiaomu-design）用法

本轮走的是**「已有方向，只要打磨」车道**（`craft-loop.md` + `preflight.md`），
**不走** Phase 2 的四方向预览（用户已把方向给死为「轻便液态玻璃」，自动车道没人能点选），
也没有出 HTML 预览页。

读的参考（只读本轮用得上的，不通读全库）：`SKILL.md` → `references/user-preferences.md`
（偏好账本，最高优先级）→ `craft-loop.md`（工艺密度菜单 + 两轮制）→ `preflight.md`
（A / B / C / D2 逐条）。

### 偷了什么（`craft-loop.md` 的工艺密度菜单，逐项落成具体数值）

| 菜单项 | 偷的动作 | 落成哪个具体数值 |
|---|---|---|
| **#12 材质细节** | 「内边框高光」——受光边是顶边一条 1px 亮线，不是一层渐变/发光 | `theme.Tokens.LitFace` 的 `InsetShadow(0, 1, 0, 0, LensRim)`：实测浅色冠线 **235 对面 229（+6 级）**、深色 **63 对面 44（+19 级）** |
| **#5 多层染色阴影** | 「拒绝单层灰阴影」——一对线是**两条方向相反**的线，不是一条 | 底对侧 `InsetShadow(0, -1, 0, 0, LensEdge)`：实测浅色 **216（−13 级）**；深色 `LensEdge` 已是 0.20（P13-n 重定基线），实测 45 |
| **#12 材质细节（反面）** | 「玻璃折射边」在此**不许**用：`Lens` 的四边环是「画框分层」，正是 §1 禁令 | pin 的「左/右列 = 面」两条断言守的就是这个：M2（换 `Lens`）在左右两列各读到一条线（浅 216 / 深 60），红 |
| **#6 编辑细节** | 「hairline」——一条线只在需要分层的地方出现，其余保持平面 | `flatFaceAllow` 现在是**空表**：这条判据刚落地就零豁免，说明它是真约束不是橡皮图章 |
| **#10 收尾彩蛋** | 「hover 才出现的细节」在此**故意不做** | 这块行盘是不可选的只读列表（`Selectable()` 只给复制，不给 hover 面）——加 hover 会撞 §6.24 24-5 判过的「从属行不该跟结论平起平坐」 |

**两轮制**：第一轮写 pin 并量到改前的三读同值（229/229/229）→ 渲染 `before` 图 →
按「四视角偏执评审」走了一遍（见下）→ 第二轮只修这一处、补齐 census → 重出 `after` 图。

### preflight 逐条结论

- **A（反 AI 套路）**：**过**。本轮不加渐变、不加外发光、不加纯黑（面仍是 `Control`
  0.07 / 0.10）、无斜体（本仓 `noitalic_test.go` 全仓普查守着）、无三等分卡片、
  无假数据（列表文案是插件自己 emit 的）、无装饰性竖线（§6 的 `Lens` 环正是被
  pin 的「左右列 = 面」挡住的）。
- **A2（进攻三步）**：**过**。DNA 注入 = 工艺密度表 5 项（见上，逐项有数值）；
  工艺密度 ≥ 5 项可指认；两轮制已发生（改了两次：先 pin、后修 + census）。
- **A4（功能契约）**：**过**。三条契约（看清每条标题/副标题、可选中复制、可翻页）
  在本轮之后逐条复核：标题与副标题的墨（`TextStrong` / `TextMuted`）一个字节没动；
  `Selectable()` 与 copy-on-select 的调用点没动；分页触发读的是 `detached.scroll.MaxY`，
  不受内线影响。
- **B（设计读取一致性）**：**过**。声明的读取是「E1 面 + 一条受光边」，产出是
  `LitFace`（顶受光边 + 底对侧）——一致。全页锁定单一外观（深浅各自一整套 token，
  由 `barApp` 的 `s.Theme` 决定）。记忆点一句话：**「同一份数据，在两个窗口里都是
  同一块面」**。动效：本轮零动效（列表是只读的），与「安静的只读列表」气质一致。
- **C（动效抽查）**：**不适用（本轮零动效）**。这块盘没有 `Hovered()`/`Pressed()` 臂、
  没有 `Transition`，pin 里也没有动的量。本仓的 `prefers-reduced-motion` 由
  `tt.SetPreferences(ui.PreferencesReduceMotion: true})` 在测试侧统一处理（`barApp` 走的是
  `Resolve`，不引入新的动效常量）。
- **D2（中文页）**：**过**。本轮**不新增任何中文文案**（窗口里的字全部来自 i18n 或插件
  emit），也就没有「中西文之间空格 / 全角标点 / 正文 ≥14px」这三条的**新增**风险。
  任务书挂着的**需拍板**项（`tokens.Caption` 9.90px vs「正文 ≥ 14px」）**本轮不碰**：
  它是一条**全仓既有账**，改它要动字号 = 动行盒 = 撞高度链红线，仍然留在
  `.pending-bugs.md` 等拍板。

## 八、仍待（P13-bp）

1. **`flatFaceAllow` 现在是空表，但口径要盯住**：这条判据的触发形状是「面 + padding +
   圆角」。如果哪一轮出现一块**故意**没有线的盘子（例如纯色占位块），它必须进表并写明
   契约行；反过来，表里出现「已经满足」的条目会变红（同 `litarm` 约定）。这是本轮
   新开的一条账，下一轮如果它报红，先看是产品真漏了还是判据太宽。
2. **同族的下一个盲区（未做，另开一轮）**：P13-aa 明写的反向判据本轮补上了「有面的
   盘子」，但**没有**补「『面』出现在 `Hovered()` / `Pressed()` 臂里」的那半边——
   `litarm_contract_test.go` 的 `pointerFaceAllow` 里仍留着两条「需拍板」
   （`launcher.rowControl/Pressed` / `launcher.fieldSettingsButton/Pressed`）。
   那是一条拍板项，不许擅自改。
3. **需拍板（沿用）**：`tokens.Caption` 9.90px / `tokens.Body` 10.80px 与 preflight D2；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾（P13-bm 仍待 1）；终端弧 S1b / S1c / S2 尾；
   `pointerFaceAllow` 的两条按下档。
4. **抢救 stash 复核**：`stash@{0}`=`afc38b247c`（`metaSeparator`，HEAD 里出现 0 次，
   已被 P13-af 的 `metaFact` 取代）；`stash@{1}`=`114b7d1585`（只改 `.pending-bugs.md`，
   文字已在文件里）。**两条无需再试合。**
