# P13-et 报告 · 动作栏那一行的 42u 地板从来没落到盒子上

**方向一句话**：这一轮把「动作栏行 = 42u 地板 + 3u 呼吸」这句**看起来对、实际没生效**的
账拆成真的三段（呼吸 2.7 + 行 36 + 卡 tail 1.8 = 带子 40.5），并删掉那句会诱导下一次
「修好」它的死码——那一次「修好」会让选中的胶囊贴住窗口最下沿。

**这轮为什么让它更有空间感**：动作栏的胶囊下沿与窗口沿之间那 **1.8px** 的呼吸，是
§2 elevation 阶梯在卡片底部的最后一级；此前它只是**碰巧**存在（靠带子比行高多出来的
余量），现在它被写进账本并被两条 pin 守住——空间感不再依赖巧合。

- 规格：`docs/rounds/P13-et-spec.md`；合约新增 **`docs/liquid-glass.md` §6.115**。
- 抢救 stash：本轮**接续了** `stash@{0}`（`704b7643ba`「floter orphan 2026-11-03 10:46
  [stale-kill]」，parent = `423c1606` = P13-es）。它带来的正是这件事的代码与注释，
  但**没有测试、没有报告、注释里的两个绝对坐标是另一张帧的读数**（437.2 / 439）——
  本轮补齐 pin、重写数值、出图并提交。**不是静默丢弃，是接上并完成。**

## 量到的病（出货尺 3.6px/`Space(1)`，满态 12 匹配 + 动作栏，720×478）

| # | 量的是什么 | 值 |
|---|---|---|
| 1 | 带子 `Space(actionBarUnits)` | **40.5** |
| 2 | 上呼吸 `Space(actionBarBreathUnits)` | **2.7** |
| 3 | 行本身（`rowRects[bar].H`） | **36.0** = `Space(rowUnits)` |
| 4 | 卡 tail `Space(TailUnits)` | **1.8** |
| 5 | 2 + 3 + 4 | **40.5** = 1（逐位相等） |
| 6 | 动作栏行盒 | `Y=440.2 H=36 bottom=476.2`（窗口下沿 478） |
| 7 | 行底 → 窗口下沿 | **1.8** = 4 |
| 8 | 陷阱成真（地板真的 42u）时：行高 / 行底 → 窗口下沿 | **37.8** / **0** |

两种账**都算出 45u**（3 + 42 与 3 + 40 + 2），所以总量看不出错；错的是行自己的盒子。

## 落地

| # | 位置 | 改动 |
|---|---|---|
| 1 | `internal/launcher/launcher.go:2471`（旧行号） | 删掉 `floor = t.Space(10.5)`（赋值在 `MinHeight(floor)` 之后，从来没生效），原地留注释写清 37.8 / 0 的陷阱 |
| 2 | `internal/launcher/launcher.go:2472` | `t.Space(0.75)` → `t.Space(actionBarBreathUnits)` |
| 3 | `internal/launcher/height.go:46-58` | `actionBarUnits` 注释从「42u 地板 + 3u 呼吸」改成三段账，并指向 shell 的 `padBottom` |
| 4 | `internal/launcher/height.go` | 新增 `actionBarBreathUnits = 0.75`（3u） |
| 5 | `internal/launcher/actionbarbreath_test.go`（新） | `TestTheActionBarBandIsBreathRowAndTail`（三段之和 == 带子）+ `TestTheActionBarKeepsTheCardsTailUnderIt`（深浅两臂：行高 / 行顶 / 行底 → 窗口下沿）+ opt-in `TestDumpP13ETBarPNGs` |

## 台账（V / B / S）

- **V 0 / B 0 / S 0**：本轮**没有任何一条既有 pin 变红**。它删的是死码，改的是同值常量；
  既有 `TestTheFooterStaysPinnedOverAFullList`、`TestTheActionBarClosesTheListNotHeadsIt`
  等 20 余条动作栏 pin **零改动全绿**。
- 新增测试函数 **3**（2 pin + 1 opt-in dump）。

## 门槛（实测）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c "--- PASS"   # 3361（HEAD 基线 3357）
grep -c "--- FAIL"                        # 0
grep -c "--- SKIP"                        # 177（HEAD 基线 176）
测试函数（internal + cmd）                  # 2532（HEAD 2529）
GOOS=linux|windows|darwin go build ./...  # 过
```

`HEAD` 基线由「本轮新增 4 PASS / 1 SKIP / 3 函数」反推并核对：P13-es 收尾 3356 / 175 / 2527
→ P13-es 自己 +1 PASS +1 SKIP +2 函数 = **3357 / 176 / 2529**（`git grep -h "^func Test" HEAD
-- 'internal/*' 'cmd/*'` 实测 2529）。

## mutation（3 条，全部实跑，`md5sum -c` 逐字节还原后复跑门槛）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 在 `isBar` 分支加 `row.MinHeight(t.Space(10.5))`（= 把那句死码「修好」） | **红**，深浅同两条：`the action bar's row is 37.80 px tall, not the result row's 36.00 px … (P13-et)` / `the action bar's row ends 0.00 px above the window's bottom edge, not the card's tail of 1.80 px: the selected capsule has lost its last breath and sits on the rim (P13-et)` |
| M2 | `actionBarBreathUnits` 0.75 → 1.5 | **红**：`the action bar's band is 40.50 px but its parts add to 43.20 px: breath 5.40 + the row's own 36.00 + the card's tail 1.80 …`；渲染那条也红（`ends -0.90 px above the window's bottom edge`） |
| M3（对照） | `t.Space(actionBarBreathUnits)` → `t.Space(0.75)` 字面量 | **全绿**（证明第 2 项改动是纯可读性，不动几何） |

M2 第一次跑时我忘了先还原 M1 的 `height.go`，两条变异叠在一起，读数无效——**已作废重跑**
（先 `md5sum -c` 逐字节还原，再单独注入 M3），报告里的数字是重跑值。

## 出图（自己复核）

`/tmp/floter-P13-et-bar-{light,dark}-{before,after,mutant}.png`

- **before / after**：`cmp` **无输出**（深浅各一张）——本轮**零像素改动**，这是刻意的：
  交付物是「删掉一句会误导下一次改动的死码 + 钉住它守的那条呼吸」。`before` 是把
  `internal/launcher/{launcher,height}.go` 退回 `HEAD` 后用同一支探针渲的，不是假设。
- **mutant**（M1）：与出货帧 diff **1385 px（light）/ 1454 px（dark）**，bbox
  `(14,446)-(705,472)` —— 眼睛复核：整条胶囊的下半截变高，底边正好压在窗口沿上，
  胶囊下面那条 1.8px 的缝没了。这就是那句死码「被修好」的样子。

## 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

- **第 6 项「编辑细节（章节编号 / 页边注 / hairline / 留白节奏）」** → 落成：带子的
  三段账 **2.7 + 36 + 1.8 = 40.5**，并把「行底 → 窗口下沿 = `Space(TailUnits)` = 1.8」
  写成 pin 的判据（不是容差，是等式）。
- **第 5 项「多层染色阴影」** → **不适用**（记下来而不是跳过）：动作栏是**卡片内部的一行**，
  §3 的阴影只给浮层（E2 档），卡内元素不许加阴影。
- **第 12 项「材质细节（内边框高光 / 受光边）」** → **不适用**：同上，§6.48 明确卡片上的
  滚动体与行不许有受光边（那是浮层的记号）。
- **DNA 供体**：本仓自己的 `docs/liquid-glass.md`（单一来源）。这是「已有方向、只要打磨」
  那条车道——方向已由用户给死（轻便液态玻璃），不另起风格，也不出 HTML 预览页。

## preflight 逐条

- **A 反 AI 套路**：通过。diff 只有一处删除、一处常量替换、三处注释；没有新增颜色、渐变、
  发光、斜体、图标、文案或感叹号。
- **B 设计读取一致性**：通过。两个 pin 的比较值全部从**这张面自己的主题**读
  （`a.Spacing` × 常量、`charged` 从 `Actions.ResizeTo` 实测），没有库存字面量。
- **C 动效抽查**：通过 / 不适用。本轮没有动效，`before` 帧与 `after` 帧逐字节相同，
  没有新增帧请求。
- **D2 中文页 / 字号**：**仍未过**（第 69 份证据）。`tokens.Caption` 9.9px、正文 10.8px 对
  「正文 ≥ 14px」——这是全仓字阶，需拍板，不在这一轮里动。

## 仍待（下一格）

1. **需拍板（沿用，未动）**：`tokens.Caption` 9.9px / 正文 10.8px 对 preflight D2；
   启动器两行结果行 36px 盒装 27 行墨；§6.111 浮层动作失败句的宽度上限；§6.113 的 113-5
   （设置体自身的顶部内边距 = 软边高）；§6.114 的 114-4（列表分支下沿随分页差 19.8px）；
   浮层页头字号上阶梯（`104-2`）/ 便条行 rank 1.28 / 终端会话抽屉失败只加字不换墨 /
   扩展页空态只有一行结论没有出路 / 会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 /
   `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 /
   终端弧 S1b / S1c / S2 尾。
2. **下一格（通用队列 · 状态反馈的「禁用/只读面」）**：先量再改——`Lens` 一族已结案
   （只剩 `control.SegmentedIndex` 一处 `故意不做`），禁用面这一族**本轮未量**。
3. 抢救 stash（第七十三次）：`stash@{0}`（`704b7643ba`）**本轮已接续并完成**，其余
   （`33e03d1f6a` / `afc38b247c` / `114b7d1585` / `567cf0a916` / `df5647edab`）仍判定与 HEAD
   无未接续内容，未试合、未静默丢弃。
