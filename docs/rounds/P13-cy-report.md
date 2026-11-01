# P13-cy 报告 · meta 行的收尾词回到它自己那一档

方向一句话：**集成卡 meta 行上的四个词现在是一个字号——行的高度由 caption 决定，不由最后
那个词决定**（标题行白长的 5px 还回去）。

## 1. 这一轮「为什么让它更轻 / 更有空间感」

同一行里混着 11px 与 12px 两种字号，行盒就归大的那个管：那条 meta 行实测 20 高（三个事实
只有 15），标题行跟着长高 5px，卡里**每一行都往下挪 5px**。把它收成一行一档之后，
`Health` 行标签 `429 → 424`、`Checks` 块 `464 → 459`，卡内的纵向节奏立刻回到「每行只占
自己那一行」——这是纯粹的空间账，不是配色或装饰。

## 2. 先量（改前真实数字，深浅两套逐像素同坐标，同一帧）

| 事实 | 改前 | 改后 |
|---|---|---|
| 卡面横向 / 1px 框线 | `192..700` / `191`,`701` | 不变 |
| 卡上行标签 `Health` | `x=200` | 不变 |
| 前导 `packageChip` | `200..230`（左沿 = 行标签列） | 不变 |
| 标题 / 描述 / meta 三行文字列 | `x=242`（方格右沿 230 + `Space(3)`） | 不变 |
| 三个事实（identity / kind / source） | `y=392.5`，`h=15` | 三点同 `y=390`，同 `h=15` |
| 收尾状态字 `· Running` | **`y=390`，`h=20`** | `y=390`，`h=15` |
| meta 行横向尺寸 | **20** | **15** |
| `Health` 行标签 | `y=429` | **`y=424`** |
| `Checks` 块 | `y=464` | **`y=459`** |
| `Tool unavailable` 暖胶囊 | 字 `h=19`，胶囊 `584..688` | 字 `h=15`，胶囊 `607..688` |

P13-cx「仍待」第 1 条点名的两个候选（`integrationMeta` / `packageChip`）**量完是列对的、
不改**：`packageChip` 左沿 = 行标签列 200，标题 / 描述 / meta 同一条 242 文字列（与参考
`.extension-row__icon` 30px + 12px gap 一致），meta 行右端停在卡内容右端 693 不顶卡框。
偏差在**行里面**，不在列上。

## 3. 改法

`settingsui.go`：`stateMark` 的三条臂 + `runtimeUnavailableBadge` 的字号
`t.FontSize-1`（12）→ `tokens.Caption`（11）。依据是参考自己：`.extension-row__meta`
声明 `font-size: var(--text-caption)`（`main:src/styles/extensions.css:960-970`），
`.extension-status` **没有**声明字号（`:1007-1015` 只有 `padding` / `line-height` /
`border-radius`）——它继承 meta 行的 caption。

零改动：字重 600、墨（`TextStrong` / `TextSecondary` / `Warning`）、那枚 `·`、
`Margin(0,0,0,Space(0.5))`、胶囊的 `Padding(Space(0.5), Space(1.5))` / `RaisedWarm` /
`ControlEdge` 内环、行的 `gap` 与 `AlignItems(Center)`。

## 4. V / B / S 台账

- **V 0**：没有一条测试钉着这四处字号（`grep FontSize` 与七个相关测试文件逐个看过）。
- **B 0**：全量 `go test -count=1 ./...` 全绿，零测试改动。
- **S 0**：结构没变。

## 5. 门槛（实测）

| 项 | 结果 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 / 净 / 净 |
| `go test -count=1 ./...` | 全绿 |
| PASS 计数 | **3300**（基线 3298） |
| 测试函数 | **2453**（基线 2450） |
| SKIP | **143**（基线 142；本轮新增 1 条 opt-in 出图） |
| 三平台 build | 过 / 过 / 过 |

## 6. mutation（3 条实跑，全部变红并逐字节还原）

- **M1** `stateMark` 两条透明臂回 `t.FontSize-1` →
  `p13cy_test.go:58: light: the closing state word is 20 tall, the facts it closes are 15: the last word is a step larger than the line it closes`
  + `:62: light: the state word starts at y=390, the facts at y=392.5: the line is taller than its own text`
  （深色同样两条，共 4 红）。
- **M2** `runtimeUnavailableBadge` 回 `t.FontSize-1` →
  `:73: light: the unavailable badge's label is 19 tall, the meta facts 15: the two state marks of one row speak two sizes`
  （深色同样，共 2 红）。
- **M3** 破损胶囊的文字回 `t.FontSize-1` →
  `:100: light: the broken chip's label is 19 tall, a meta fact 15: the chip's height comes from its padding, not from a larger font`
  （深色同样，共 2 红）。

每条都用 `cp` 还原后 `md5sum -c` 逐字节核对，再 `touch` 复跑一次确认绿（避免测试缓存复用）。

## 7. 出图

`/tmp/floter-P13-cy-meta-{light,dark}.png`（改后）与 `-before.png`（同一 dump 在 M1 状态下渲的
改前帧）。逐像素差分：浅色 **12658 px**、深色 **12646 px**，bbox `(191,356)-(701,525)`——
动的正是 meta 行以下那一段（每行上移 5px）。

**我在图上看到的一处变化**：浅色帧里 `1.2.3 · dev.floter.probed` 这一行与它右边
`· Running` 现在顶在同一条上，`Running` 不再是那一行里「略大、略微浮起」的词；同一帧里
`Tool unavailable` 暖胶囊窄了 17px（`584..688 → 607..688`），胶囊本身仍比正文高 4px——
那 4px 是它的 padding，是参考自己的几何，读起来仍然像一枚章。

## 8. 用了设计 skill（用户 2026-10-30 要求）

读的路径：`SKILL.md` → `references/user-preferences.md` → 本轮表面按
「已有方向，只要打磨」走 `craft-loop.md` + `preflight.md`。**没有**走 Phase 2 四方向预览，
没有出 HTML 预览页（方向已由用户钉死为轻便液态玻璃）。

### 偷了什么（逐条，写不出就是没偷）

1. 从 `craft-loop.md` §一「参考 DNA 注入（先偷再做）」偷的是**参考自己声明的字号继承**：
   把「`.extension-status` 不声明 `font-size`」这个事实翻成「状态字 = caption」，
   落成 `tokens.Caption` = **11px**（四个节点）。
2. 从 `craft-loop.md` §二工艺密度菜单取 **第 7 项「编辑细节」**（菜单原例：hairline / 大写
   字距标签）→ 本仓的对应物是**一行一个字号**：落成 meta 行四个节点同为 11px、行盒同为
   **15px**、行距仍为 **4px**（`Space(1)`）。
3. 同菜单取 **第 12 项「材质细节」**的边界：状态字与暖胶囊的**材质不动**（`RaisedWarm` +
   `ControlEdge` 内环 + `RadiusSM`），只收字号——因为 `docs/liquid-glass.md` §3 已经钉死
   受光边与阴影的形状，本轮不得顺手改材质。
4. 诚实记账：菜单 12 项本轮只落了 2 项，**低于 ≥5 的密度要求**。理由写清：这是「一个表面
   一行」的收口轮（标题行里的一行字），不是新表面；新增 5 项签名动作会越界到材质与动效，
   而这一支的纪律是「一轮一件事」。
5. 两轮制：本轮实际发生了「量（第一轮）→ 四视角挑刺 → 只修这一处（第二轮）」，见下一节。

### preflight 逐条结论

- **A 反套路**：无渐变文字 / 无纯黑 / 无外发光 / 无斜体 / 无三等分卡片 / 无「革命性」式文案 /
  无左侧竖线装饰——本轮只改字号，这些项全部照旧成立。**结论：过**。
- **A2 进攻三步**：DNA 注入（参考 CSS 字号继承）✔；工艺密度 **2/5**（现况如实记在上一节，
  不达标并说明原因）✖；两轮制 ✔。**结论：密度一项不过，理由已写**。
- **B 设计读取一致性**：一页一个明暗主题（深/浅两帧分别出图）、记忆点是「一行一档字阶」的
  收紧、无动效所以无 cohesion 问题。**结论：过**。
- **C 动效抽查**：本轮零动效改动（没有 `transition`、没有入场、没有 `prefers-reduced-motion`
  相关面）。**结论：不适用（改动面不含动效）**。
- **D2 中文页**：字号仍是 `tokens.Caption`（11px），**低于 preflight 的「正文 ≥ 14px」**。
  这是本仓既有口径，不是本轮引入——已在 `.pending-bugs.md`「需用户拍板」挂了第 26 份证据
  （`tokens.Caption` 11px vs D2 14px），**不许擅自改**。字重仍只用 500/600；中英混排空格与
  全角标点未触碰。**结论：字号一项不过，原因是需拍板项，已挂账**。

## 9. 仍待

1. **下一格（先量）**：终端页的会话抽屉 / 标签对失败只加字不换墨（`HeaderExited(code)`）——
   与 §6.56 刚落地的会话列表不是同一个说法，先量再定。
2. **需拍板**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」（沿用，第 26 份证据）。
3. **需拍板（沿用）**：扩展页空态只有一行结论没有出路；会话页空态说错对象；终端外观抽屉行
   要不要面；`launcher/filters.go` 过滤 chip 与 `launcher/power.go` 确认带 chip 的按下档；
   `ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；
   路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **记一笔（不改）**：`rowFace.Muted`（`.settings-row--muted` 的 0.62）当前零调用点。
5. **抢救 stash 核对（第二十七次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，不要再试合。
