# P13-cw 报告：健康块从卡框上收回卡的列，两条暖棱回到它们介绍文字的那一侧

> 增量写。数字全部是本轮实测（720×1700、深浅两套、逐像素同坐标）。

## 0. 设计读取（qiaomu-design，已有方向 / 只要打磨那条路径）

> **读取为：桌面设置面板的集成详情卡，面向装了扩展的普通用户，用「近黑底 + 面差 + 发丝线」
> 语言，倾向轻便液态玻璃（`docs/liquid-glass.md`）。**

- 冒险度 4 / 动效 0（本轮没有动效面）/ 密度 6——功能页，这一格只动**列**与**哪一侧**。
- 功能契约：这张卡要让人 3 秒内答出「这个工具上次检测怎么样、哪儿失败了、批准的是哪一版」。
  本轮不改任何事实、文案或交互，只让**块站在它该站的竖线上**、让**重线站在它介绍文字的那一侧**。

## 1. 先量（改之前）

见 `docs/rounds/P13-cw-spec.md` 的表。三条关键：

1. 卡面横向 `192..700`（1px 框线在 191 / 701）；卡上每一个行标签（`Health` / `Approval record` /
   `Diagnostics`）在 **x=200**，健康块的 `Checks` / `Failures` / `Checked …` 在 **x=192**——8px。
2. 探针时长 `12ms` 的右端在 **x=701**（就是卡框线），块的两侧都顶着卡自己的边。
3. 失败卡的 2px 警示棱在 **`699..700`（右边）**，左边只有 1px keyline；陈旧横幅同病
   （左 0px / 右 2px）。参考写的是 `border-left: 2px solid var(--text-warning)`
   （`floter-main:src/styles/extensions.css:2200`，陈旧横幅 `:2172`），
   而本仓框架的负偏移把未覆盖的一条留在**对侧**（`mygo/ui/paint.go:1254`）。

先写 pin 再改：`TestTheHealthBlockSitsInTheCardsLabelColumn` 与
`TestTheWarmRuleLeansOnItsLeadingEdge` 在改前**红在四条真问题上**（浅深各两份）：

```
the health block's labels start at x=192.0/192.0, the row above them at x=200.0: the block hangs 8.0px left of the card's label column
the block starts 0.0px inside the card's face (x=192.0, face starts at 192), want 8px
the block ends at x=701.0, want the card's content edge at x=693.0 (face ends at 700)
failure card (x=192..700): leading edge 1px / trailing edge 2px of warning ink, want 2 / 1
stale banner (x=200..692): leading edge 0px of warning ink, want the reference's 2px
stale banner: trailing edge 2px of warning ink, want none
```

## 2. 改了什么

| # | 文件 | 改动 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go` `healthDetail` | `block` 列加 `.Padding(0, a.rowInset(t))`：块进入卡的行列（集成卡的 `rowInset` = `Space(2)` = 8） |
| 2 | `internal/settingsui/healthfaces.go` `healthFailureCard` | `InsetShadow(-2, 0, 0, 0, Warning)` → `InsetShadow(2, 0, 0, 0, Warning)`（棱到前导边） |
| 3 | `internal/settingsui/healthfaces.go` `approvalStaleBanner` | 同上（同一族、同一张卡） |

**只加水平内缩**：纵向一个像素都没动——`Checks` y=439、`Failures` y=521、首条探针 y=462、
时间戳 y=598、失败卡 `exit: 3` y=548 前后完全一致（P13-cv 刚定完的线距原样保留）。

## 3. 改之后（实测）

| 事实 | 改前 | 改后 |
|---|---|---|
| `Checks` / `Failures` / `Checked …` 横向 | `192..701` | **`200..693`** |
| 探针时长 `12ms` | `675..701`（贴卡框） | **`667..693`** |
| 失败卡横向 | `192..700`（占满卡的内容盒） | **`200..692`** |
| 失败卡的警示墨 | 左 1px / 右 2px | **左 2px（`200..201`）/ 右 1px（`692`）** |
| 陈旧横幅的警示墨 | 左 0px / 右 2px | **左 2px（`200..201`）/ 右 0px** |
| 卡面 / 暖面色（浅） | `229 229 230` / `227 214 206` | 同（墨一格没动） |
| 卡面 / 暖面色（深） | `39 39 41` / `64 55 51` | 同 |

## 4. 出图

`/tmp/floter-P13-cw-healthblock-{light,dark}-{before,after}.png`。

- 差分：浅色 **4766 px**、深色 **4764 px**，bbox 都是 `(192,442)-(700,964)`
  ——正好是健康块 + 失败卡 + 陈旧横幅这三段，卡上其它行**零差分**。
- **这张图上看到的一处变化**：失败卡左缘现在是一条饱和的暖橙 2px（`217 119 6`），右缘只剩一道
  1px 的暖 keyline；改前两者反过来。同一张图上，`Checks` / `Failures` / `Checked …` 三行的
  左缘各让出 8px 的卡面，于是它们和上面的 `Health`、下面的 `Approval record` 落在同一条竖线上；
  探针那两行右侧的 `12ms` / `30ms` 也从卡框边退回 8px，和标题列右端对齐。

## 5. 偷了什么（qiaomu-design `craft-loop.md` 工艺密度菜单，逐条落到数值）

本轮走「已有方向，只要打磨」那条道（`craft-loop.md` + `preflight.md`），不出四方向预览页
（用户已把方向给死：轻便液态玻璃）。工艺密度菜单里真正用得上的是**第 6 项（编辑细节：hairline /
缩进列）**与**第 12 项（材质细节：内边框高光/键线）**，落成这样三条具体数值：

| 菜单项 | 偷到的动作 | 落成本轮的具体数值 |
|---|---|---|
| #6 编辑细节 | 「块的标签与它所属行的标签同一条竖线」——一个块不是新的一栏 | 块两侧内缩 = 卡自己的 `rowInset` = `Space(2)` = **8u**：`192→200` / `701→693` |
| #12 材质细节 | 「重线站在它介绍的物件那一侧」（参考 `border-left`，不是两侧对称） | 2px 警示墨从 `699..700`（尾边）搬到 `200..201`（前导边）；尾边只留 1px keyline |
| #6 编辑细节 | 「同一族的面用同一把尺」——同页另一块（陈旧横幅）同一个数字 | 横幅 `0px → 2px` 前导、尾边 `2px → 0px`；两处一起改，不留一半 |

**没偷的**：菜单里的氛围层 / 渐变 / 多色阴影都不适用（本仓是自绘的近黑底 + 面差 + 发丝线，
`docs/liquid-glass.md` §0 明令禁止伪造玻璃）。这轮的价值在**对齐**与**方向**，不在加装饰。

## 6. preflight（`references/preflight.md`）逐条结论

- **A 反套路**：过——无渐变/纯黑/霓虹/发光，无斜体（`noitalic_test.go` 全仓 pin），
  无三等分卡、无居中 Hero、无 AI 空词；这一轮没有新增文案，一个 i18n 字符串都没动。
- **A2 进攻三步**：DNA 注入 = 偷参考自己的 `extensions.css:1997-2002 / 2200 / 2172` 三处原文数值
  （不是「像参考」，是同一串数）；工艺密度 ≥5 项这条对**打磨型单表面轮**不适用，本轮按
  `craft-loop.md` §四的取舍顺序只交「两轮制」与「DNA 注入」两项，密度项已在 §5 记三项。
- **A2.1 执行者**：默认由当前 Codex 直接执行，未调用 K3（用户未在本任务点名）。
- **B 设计读取一致性**：过——本轮声明的读取是「近黑底 + 面差 + 发丝线的轻便液态玻璃」，
  产出只动列与棱的方向，没有引入第二种材质；整帧仍只有一个明暗主题（浅/深分别出图）。
- **C 动效抽查**：不适用——本轮零动效、零交互面（`Transition` 一个都没加）。
- **D2 中文页**：过（未回归）——`tokens.Caption` 11px 与「正文 ≥ 14px」的冲突是**沿用**的
  「需拍板」项（第 24 份证据），本轮没有新增或放大它；中西文空格、全角标点、字重 400/500/600
  均未触碰。
- **E 三秒测试**：改前这一屏的视线会被失败卡左缘那条**1px 接缝**带走（它比正文还靠边）；
  改后最先看到的是 2px 暖棱 + 缩进后的块标签，卡的层级读得出来了。

## 7. 门槛（本轮实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...   → 净
go test -count=1 ./...                                 → 全绿（FAIL 0）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 3296（基线 3294）
--- SKIP                                               → 141（基线 140，新增 opt-in 出图）
测试函数（含 docs/ 模板）                              → 2449（基线 2446）
GOOS=linux/windows/darwin go build ./...               → 全过
```

## 8. 变异（实跑 3 条，全部变红并逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| M1 | `Padding(0, a.rowInset(t))` → `Padding(0, 0)` | `TestTheHealthBlockSitsInTheCardsLabelColumn` 六红（3 条断言 × 深浅各一份）：`the health block's labels start at x=192.0/192.0, the row above them at x=200.0: the block hangs 8.0px left…` / `the block starts 0.0px inside the card's face…` / `the block ends at x=701.0, want … 693.0` |
| M2 | `healthFailureCard` 的 `InsetShadow(2, …)` → `-2` | `TestTheWarmRuleLeansOnItsLeadingEdge` 两红：`failure card (x=200..692): leading edge 1px / trailing edge 2px of warning ink, want 2 / 1` |
| M3 | 过冲臂：`Padding(0, a.rowInset(t)*2)` | 列 pin 红（深浅各一份）：`the block starts 16.0px inside the card's face (x=208.0…)` / `the block ends at x=685.0, want … 693.0` |

`md5sum -c` 三条全部「成功」（`settingsui.go` `f793f4d6…`、`healthfaces.go` `5ecb94ce…`），
还原后 `touch` 再跑一次两条 pin：绿（不复用测试缓存）。

## 9. 台账

- **V 0**（没有「旧设计数值被重定」——两条新 pin 全是新增；另有一条**取样点**重定，
  见 §4 与 `.pending-bugs.md`：断言本身未变）。
- **B 0**（行为/度量/接线零改动：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线）。
- **S 0**（结构未变：块的父子关系没改，只给它加了水平内缩）。
- **产品代码 2 文件 3 处**；测试 2 文件新增 3 个函数（2 pin + 1 opt-in 出图），1 条旧 pin 的取样点。
