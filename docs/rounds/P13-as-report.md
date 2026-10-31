# P13-as 报告：满态的下沿不是一刀切（设置页底边补上顶边那道软边）

## 一、这轮为什么让它更轻 / 更有空间感

设置页滚起来之后，**下沿是一刀刃**：一页比窗口长的内容在窗口最后一行像素上被水平
切断，字的墨一点没退（实测浅色档 y=579 那一行 `lo=27 hi=246`，展开 **219**，与软边上方的
219 一样满）。而它的上沿早有一道 `glass.ScrollEdge` 软边。同一块面板，一头是软的、
一头是刀口——**空间感恰恰死在刀口上**：读的人看到的不是「这页还有，往下滚」，而是
「这页被裁掉了」。

启动器的满态早就有折边（`liquid-glass.md` §6.1 表 1-10：行被视口切一半时，
墨 = 可见高度 / 行高）；设置页这块**自由滚动**的面板是同一族里最后一个刀口。

一句话：**满态不再是一张被裁掉的纸，而是一页在底边继续往下读的内容。**

## 二、落地

- `internal/settingsui/settingsui.go:1515-1516`（`body` 的滚动边）：补一道
  `ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(edge).PassThrough().
  Material(glass.ScrollEdge{Bottom: true, Background: t.Background})`，
  与顶边那条**逐字段相同**（同一个 `edge = t.Space(4)`、同一个 `Background`、
  同样 `PassThrough()` 不吃指针）。
- **盒子零改动**：底部 padding 本来就是 `t.Space(6)`（24px）> 边高 16px，
  滚到底时这道边落在空白上（没有内容可淡），所以 `MaxY`、内容位置、滚动链逐像素不变
  （`wheel_test.go` 的三条 B 类 pin 零改动全绿）。
- `internal/settingsui/bottomedge_test.go`（新）：两条 pin × 深浅两套。
- `docs/liquid-glass.md` §6.16；`.pending-bugs.md` 顶部本轮段。

## 三、量（同一把尺：内容列 x∈[240,696) 的逐行墨展开度 `max−min`）

19 档滚动位置（`Body.Y = MaxY·k/20`）上的**最坏值**：

| | 窗口最后一行 | 软边上方那一带 | 比值 |
|---|---|---|---|
| 改前（M1 无底边） | **222** | 202 | **110%** |
| 改后 · 浅色 | **39** | 202 | **19%** |
| 改后 · 深色 | **40** | 198 | **20%** |

自门控（滚到底、下面只剩 padding）：带内均值 **246 / 14**，带上方的行 **246 / 15**——
差 0 / 1 级，等于「没有内容滚过时这道边看不见」。

pn 判据：① 最后一行 ≤ 边上方最坏值的 45%（改前 110% 红）；② 滚到底时带内均值与上方
相差 ≤ 6 级（自门控，保证它不会在页脚空白上留下一道灰带）；③ 边上方的墨必须 ≥ 100
级，否则 ① 会在空页面上假绿。

## 四、V / B / S 台账

**V 0 / B 0 / S 0**（新增 2 条 pin，无既有断言被改）。

## 五、门槛（实测）

```
gofmt -l cmd internal                                # 空
go vet ./... / GOOS=windows / GOOS=darwin            # 净
go test -count=1 ./...                               # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"    # 3062 → 3065
go test -count=1 -v ./... | grep -c -- "--- FAIL"    # 0
go test -count=1 -v ./... | grep -c -- "--- SKIP"    # 99（不变；出图在 pin 里按 env 走，不新增 Skip）
测试函数（同一把尺：只数 cmd/ + internal/）          # 2271 → 2272
GOOS=linux|windows|darwin go build ./...             # 三个都过
```

## 六、变异（2 条实跑，都变红，`md5sum -c` 还原成功）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 底边高度 4u → 0（等于把这道边删掉） | **红**：`the last row of the window reads at 222 while the rows above the edge read at 202 (110% of it)`（浅）/ 深同 |
| M2 | 底边离开下沿（`Bottom(0)` → `Bottom(6u)`，悬在 padding 上） | **红**：`222 … 202 (110%)`（浅）/ `222 … 198 (112%)`（深） |
| — | 另试**底色换成墨色**（`Background: t.Background` → `t.Text`） | **如实记：不红**。滚到底那一带的均值仍是 246 / 14，说明这条 pin 量的是「带内 vs 带上方的**均值**」，而在这个位置上换底色没能把它推开；记在这里，不假装它红 |
| M3 | 出图路径按 `FLOTER_P13AS_TAG` 走，不参与判据 | — |

两条还原后 `md5sum -c /tmp/p13as.md5` → `internal/settingsui/settingsui.go: 成功`，
并 `touch` 后复跑一次（避开测试缓存），全绿。

## 七、偷了什么（qiaomu-design）

本轮读的是 `SKILL.md` → `references/user-preferences.md`（偏好账本）→ `craft-loop.md`
（工艺密度 + 两轮制）→ `preflight.md`。走「已有方向，只要打磨」那条车道，**没有** Phase 2
的四方向预览（自动驾驶道上不许出 HTML 预览页、不许停下来等点选）。

- **`craft-loop.md` 第二节「工艺密度」菜单第 6 项（边与边的收口：屏幕边缘要有一条退让带）**
  → 落成具体数值：底边带 `height = t.Space(4)` = 16px，与顶边同值；底 padding 6u(24px) 不动
  —— 退让带比 padding 短，滚到底时它落在空白上，于是「有内容时淡、没内容时看不见」。
- **`craft-loop.md` 第三节「两轮制」（Render → Critique → Refine）** → 第一轮**只量不改**：
  先量出「最后一行 219 vs 边上方 219 = 110%」这组读数，才敢确认这不是错觉；
  第二轮才动代码，并当场用同一批档位复量（39/202 = 19%）。
- **`craft-loop.md` 第二节第 9 项（可滚动区域的两端要成对处理）** → 落成「一处 commit 两条边」：
  顶边的 `ScrollEdge` 与底边的 `ScrollEdge{Bottom:true}` 逐字段相同，不留「一头软一头硬」。
- **`preflight.md` A2（不用「裁切」当布局）** → 落成判据 ① 的写法：**不**接受「被切一半的
  行也算一种状态」，要求最后一行必须退后到 ≤45%。

## 八、preflight 逐条结论

- **A（反 AI 套路）**：过。只加一道渐变边，零新增阴影/发光/斜体/侧竖线/emoji。
- **B（设计读取一致性）**：过。读取 = 「轻便液态玻璃 + 明确的 elevation」：滚动区两端
  用同一道软边，墨退向**面板自己的底色**（`t.Background`），不引入第二种材质。
- **C（动效抽查）**：过。零动效变更（`ScrollEdge` 是静态材质，没有 duration）。
- **D2（中文页）**：**部分不过，如实记录**：本轮没有碰任何文案与字号，`tokens.Caption`
  11px 与「正文 ≥ 14px」的张力仍是 P13-ak 起记着的「需拍板」，本轮没有扩大它的使用面。
- **E（三秒测试）**：过。眯眼看：滚动区两端现在是同一种收口，视线在页面上不再被
  窗口边缘的一条硬线打断。

## 九、仍待（P13-as）

1. **同族（下一轮候选）**：其余三条滚动容器仍是「一头软一头硬」或两头都硬——
   `settingsui.go:876`（配置表单）、`pluginsheet.go:64`（插件配置弹层）、
   `settingsui.go:6800`（运行输出）。它们是三个不同表面，按纪律各占一轮。
2. **需核实（本轮没敢动）**：顶边那道 `ScrollEdge` 的 `Background` 是 `t.Background`，
   而它脚下是 `Sunken` 面板（实测浅色 229 / 深色 39，而 `Background` 是 247 / 14）。
   静止时它在预留 padding 上画 0.85 的底，理论上有 **~15 级的淡痕**；本轮只补对称的底边，
   没有改它——要改得先量出静止帧那一条 16px 的读数，再决定换 `Sunken` 的合成色。
3. **需拍板（沿用）**：`cardRow` 控制槽 `Shrink(0)`；`tokens.Caption` 11px 与 D2；
   终端弧 S1b / S1c / S2 尾。
4. 抢救 stash：`stash@{0}`（`afc38b247c`）与 `stash@{1}`（`114b7d1585`）在 P13-ap 已核过
   （内容已在 HEAD / 只改 `.pending-bugs.md`），本轮未再试合，也未静默丢弃。
