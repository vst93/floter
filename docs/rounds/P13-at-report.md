# P13-at 报告：一块凹槽的两端——内容不该在两条边上同时被刀切

**方向一句话**：扩展页「最近一次输出」那块 180px 凹槽的两个端口原来是**平刀切**（第一行
读到最内的 96%），本轮给它两道**按滚动位置自门控**的软边，并把「这道边该是什么颜色」
从一个猜值变成一条可算的链（`theme.FaceOver`）——凹槽从此有「面 / 浮」的先后，而不是
一块被画框圈住的玻璃窗。

- 规格：`docs/rounds/P13-at-spec.md`
- 合约：`docs/liquid-glass.md` **§6.17**（并回填了 §6.16 的「仍未量」——那把尺造出来了）
- 选题依据：§6 差距表**没有 `未开始` 行**，按纪律回通用队列；P13-as 报告「仍待 1」点名的
  三处同族容器里，这处是**唯一两端都硬**的（另两处：`config.go:876` 的 66px 多选箱、
  `pluginsheet.go:64` 的浮层，各占一轮）。

## 一、这轮为什么让它更轻、更有空间感

凹槽的两端原来是两条**边界**：内容在最外一行像素上被切断，读起来像一块被玻璃贴住的
硬窗。改成软边之后，两头变成**退让带**——内容不是被挡住，是「继续往前但轻下去」，
这正是轻便液态玻璃里「面差」那一条在不新增任何阴影/发光的前提下能做的事：同一块面上
多了两个方向（上沿、下沿），而不多一块料。

## 二、实测（改前 / 改后，两套外观都跑）

装置：`runOutputScrollApp(t)`（60 行 `line NN`）+ `integrationPage(t, a, 720, 2400)`，
`scroll.Y = scroll.MaxY/2`。视口 `(209,564) 475×180`，`MaxY = 735`。

| 量（19 档滚动位置的最坏值） | 改前浅 / 深 | 改后浅 / 深 |
|---|---|---|
| 视口第一 4 行的墨展开 vs 最内 | 202/202（**100%**）· 188/188（100%） | **80/202（40%）· 75/188（40%）** |
| 视口最后 4 行 vs 最内 | 202/202（100%）· 188/188（100%） | **77/202（38%）· 72/188（38%）** |
| `Y = 0` 时第一 8 行 vs 最内 | 194/194 · 181/181 | **194/194（100%）· 181/181（100%）**——门关着 |
| `Y = MaxY` 时最后 8 行 vs 最内 | 194/202 · 181/188 | **194/202（96%）· 181/188（96%）**——门关着 |
| 边带内（无字竖带）的均值 vs 凹槽自己的面 | — | **236 / 236 · 42 / 42（0 级差）** |
| 视口矩形 / `MaxY` | 475×180 / 735 | **475×180 / 735（逐像素不变）** |

「改前」那一列不是估的：单帧（`Y = MaxY/2`）实测就是 193/189/179/202 对 202（第一 4 行
的最深一行 = 最内），而 M1（把边高置 0，等于回到改前）在**19 档位置**上的最坏值仍是
202/202、188/188——两个来源在 100% 上重合。

出图（`FLOTER_P13AT_DUMP=1`，`FLOTER_P13AT_TAG=before|after`）：
`/tmp/floter-P13-at-runoutput-{light,dark}-{before,after}.png`。

**图上看到的一处变化**：`before` 那张里，凹槽上沿的第一行像素就是一条满对比的字
（同样 720 宽裁切下 top4 展开 **220**、与中段 **220** 一样满），`after` 那张同一位置
降到 **98**（45%）——同一帧里中段仍是 **220**，说明淡的是「边」，不是「墨变浅了」；
下沿同样 212 → **86**。深浅两套出图都是这个形状（深色 206 → 93、200 → 81）。

## 三、落地（文件 + 行为）

| 位置 | 改动 |
|---|---|
| `internal/settingsui/settingsui.go` `runOutput` | 滚动视口包进一层 `FillWidth` 的 `Column`；同层两块 `Absolute().Top/Bottom(0).Left/Right(0).Height(t.Space(3)).PassThrough()` 的 `glass.ScrollEdge`；门开在 `scrolled.Y > 0` / `scrolled.Y < scrolled.MaxY` |
| 同上，新 helper `runOutputEdgeFace` | 把「窗口地 → 设置体面板 `Sunken` → 扩展卡 `Control` → 凹槽 `Sunken`」压成不透明的一个面（见 §四） |
| `internal/theme/theme.go`，新导出 `FaceOver` | `blendOpaque` 的公开入口：`FaceOver(base, face)` = face 合成到 base 上的不透明结果。**新增 API 只有一个函数、零调用点变更**（`blendOpaque` 原仅 1 处内部调用） |

**行为契约零改动**：高度链（`MaxHeight(45u)`、padding/gap、`runOutputScroll(id)` 的
`*ui.ScrollState`）、滚轮路径、重绘保位、键盘模型、设置写回、i18n、命令接线——一个都没
碰；两条新盒是 `PassThrough()` 的纯 paint，不吃指针事件。`runoutputwheel_test.go` 三条
pin **零改动全绿**。

## 四、三条反直觉（都写进合约 §6.17 了）

1. **`ScrollEdge` 的 `Background` 必须是本来就「不透明」的面。** `ui.Color.Alpha(a)`
   （`mygo@v0.4.8/ui/color.go:93`）是**乘**：把 `Sunken`（浅色 0.169 / 深色 0.169 的薄罩）
   直接交给 `ScrollEdge`，那道 0.85 被乘成 0.14 —— 实测第一行仍读到最内的 **90%**，
   跟不改几乎一样（这就是 M4 那条变异）。变红的是 pin ①，不是人眼。
2. **这块凹槽的面不是一个字面值，是一条链。** 只把 `Control`（一层 0.07 的黑罩）与
   `Sunken` 混在一起得到近黑（实测 **41**），只混 `Background` 与 `Sunken` 又太白
   （**246**）。按「地→面板→卡→块」压完整条链，实测**正好**是 236 / 42 两个值本身，
   边落在里面与凹槽自己的面**一级不差**。
3. **§6.16 的「自门控」在这里不成立，得靠 `ScrollState.Y`。** 那处靠 padding（边落在
   空白上）；这块凹槽的 padding 不许长（`MaxHeight` 固定，长了就是从视口里偷两行字），
   所以门只能是条件判断——而它必须**同时**关两头：停在开头时第一行、滚到底时最后一行，
   都是用户正在读的那一行。

## 五、门槛（全跑，实测）

```
gofmt -l cmd internal                                 # 空
go vet ./...                                          # 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...   # 净
go test -count=1 ./...                                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"     # 3065 → 3076（+11）
grep -c -- "--- FAIL"                                 # 0
grep -c -- "--- SKIP"                                 # 99 → 100（+1 = opt-in 出图）
测试函数（同一把尺：只数 cmd/ + internal/）            # 2272 → 2278（+6）
GOOS=linux|windows|darwin go build ./...              # 三个都过
```

**V / B / S 台账：V 0 / B 0 / S 0** —— 没有一条旧 pin 变红：这块凹槽此前没有任何像素 pin
（这正是本轮新增四条的理由），`internal/theme` 的既有 token pin 一个数没动
（`FaceOver` 是**新增**函数，不改任何 token 的值）。

## 六、变异（4 条实跑，全红，`md5sum -c` 逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 边高 `t.Space(3)` → `t.Space(0)`（= 把两道边删掉） | **红**：`the block's first 4 rows read at 202 against the inside's 202 (100% of it)` / `last 4 rows … 202 (100%)`（浅）；深 `188 … 188 (100%)` |
| M2 | 去掉门（`if scrolled.Y > 0` / `if scrolled.Y < scrolled.MaxY` → `if true`） | **红**：`at the start of the scroll the block's first 8 rows read at 132 against the inside's 194 (68% of it)`；`at the end … last 8 rows read at 132 against the inside's 202 (65% of it)`（浅）；深 123/181（68%）、123/188（65%） |
| M3 | 底色换成窗口地（`face := t.Background`，即 P13-as 仍待 2 的那个错） | **红**：`inside the top edge the empty column reads 244 while the block's own face reads 236 (+8 levels)`（浅）+ 同一条底部；深色方向相反（暗面被一道亮带盖住） |
| M4 | 底色直接用薄罩（`face := runOutputFace(tokens)`，即不合成） | **红**：`the block's first 4 rows read at 182 against the inside's 202 (90% of it)` / `last 4 rows … 179 (89%)`（浅） |

四条还原后 `md5sum -c /tmp/p13at.md5` → `internal/settingsui/settingsui.go: 成功`，
并 `touch` 后复跑一次（避开测试缓存），全绿。

**一条如实记**：M2 第一次跑时门宽取 16 行，红得只有 **85% / 86%**（离 90% 的门槛只有
4~5 点）——那是「带子比边长、深处已经没被淡」稀释出来的。把这条 pin 的带子收到 **8 行**
（= 边高 12px 之内）之后，同一个变异红到 **68% / 65%**，并把「停在开头时第一行不许被淡」
这句话真的钉住了（关着门实测 100% / 96%）。

## 七、偷了什么（qiaomu-design）

本轮读的是 `SKILL.md` → `references/user-preferences.md`（偏好账本，最高优先级）
→ `craft-loop.md`（工艺密度 + 两轮制）→ `preflight.md`。走「已有方向，只要打磨」那条
车道，**没有** Phase 2 的四方向预览（自动驾驶道上不许出 HTML 预览页、不许停下来等点选）。

- **`craft-loop.md` 第二节「工艺密度」菜单第 12 项（材质细节）** → 落成具体数值：
  边的底色不是挑一个 token，而是**算出来**的——`theme.FaceOver` × 3 层，
  `Control` 0.07 的黑罩 + `Sunken` 0.169 的半透明面 + 窗口地，实测压出 236 / 42，
  与凹槽自己的面 0 级差。材质细节的意思是「这块面的每一层都能说出来」，不是加一层高光。
- **`craft-loop.md` 第三节「两轮制」（Render → Critique → Refine）** → 第一轮**只量不改**：
  先把改前的 96% / 96% 量出来，才敢确认「两头都在刀切」不是错觉；第二轮才动代码，
  动了之后**用同一批档位复量**，并且第一次（薄罩直接上）量到 90% 时**没有**放宽门槛
  混过去，而是回头去找「为什么 0.85 没生效」——答案就是 `Alpha` 是乘。
- **`craft-loop.md` 第二节第 9 项（可滚动区域的两端要成对处理）** → 落成「一处 commit
  两条边」：顶/底两块盒的 `Height`、`Background`、`PassThrough()` 逐字段相同，
  只差 `Bottom: true`；并且两头的**门**是同一条规则的两次书写（`Y > 0` / `Y < MaxY`）。
- **`user-preferences.md` P-1（界面排版禁用斜体）** → 这条凹槽里的空态是 `Italic()`
  （`.extension-row__output-empty` 的既有契约），本轮**不动**它，也没有把斜体带去
  新加的面上——两条边只画底色，不带字。

## 八、preflight 逐条结论

- **A（反 AI 套路）**：过。只加两道渐变边，零新增阴影、零发光、零渐变白边（§1 禁令 3：
  受光边是「面的一侧」，不是描一圈高光）、零新图标、零新文案。
- **B（设计读取一致性）**：过。读取 =「轻便液态玻璃 + 明确的 elevation」：凹槽是 E0 的
  凹面，它的两端用**它自己的面**退让，不引入第二种材质；边高 3u 在 §4 的刻度上，
  与 §6.16 的 4u 差一档，理由（行盒 16px vs 边高）写在代码注释与合约里。
- **C（动效抽查）**：过。零动效变更：两道边是静态材质，没有 duration、没有过渡；
  门的开关跟随 `ScrollState.Y`，不引入动画。
- **D2（中文页）**：**部分不过，如实记录**：本轮没有碰任何文案与字号，`tokens.Caption`
  11px 与「正文 ≥ 14px」的张力仍是 P13-ak 起记着的「需拍板」；本轮反而**用到**了这档
  （边高按 caption 的行盒定），所以把它从「待拍板」第 2 条再点一次名。
- **E（三秒测试）**：过。眯眼看：这块凹槽现在有「两端」了——上下各一条退让带把它关在
  卡里，视线不再撞在两条硬边线上。

## 九、仍待（P13-at）

1. **同族（下一轮候选）**：`config.go:876` 的多选箱（视口 66px，边高要另定——3u=12px
   会吃掉它 1/5 的高度，可能得 2u）与 `pluginsheet.go:64` 的插件配置弹层（浮层，
   §6.2 管辖，边要落在浮层自己的面上）。两者各占一轮。
2. **需核实（P13-as 仍待 2 的收尾）**：设置页滚动体那两道边的 `Background` 仍是
   `t.Background`，脚下是 `Sunken`；本轮造出的 `theme.FaceOver` 正是它需要的尺
   （把「地→面板」压成不透明面）。但那是另一个表面，按纪律另起一轮，本轮只量了
   这块凹槽（0 级差）。
3. **需拍板（沿用）**：`cardRow` 控制槽 `Shrink(0)`（长值先吃标签列）；
   `tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」；终端弧 S1b / S1c / S2 尾。
4. 抢救 stash：`stash@{0}`（`afc38b247c`，P13-af 的 `metaSeparator`，已在 HEAD）与
   `stash@{1}`（`114b7d1585`，只改 `.pending-bugs.md`，文字已在文件里）本轮开工时复核过
   （`git stash show -p | md5sum` + `git diff` 逐条对照），**未再试合，也未静默丢弃**。
