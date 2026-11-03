# P13-fe 报告 · 启动器最后两处「正文 ± N」，与把字阶账扩到这个包

**读取为**：桌面命令面板（启动器）与其捕获输出页，面向每天用键盘的人，用「近黑底 + 三级面 +
发丝线 + 紧凑行」的轻便液态玻璃语言，倾向 Raycast 系的设计家族。三拨盘本轮不动（这不是新方向，
是同一方向的一致性收尾）：`VARIANCE` 3、`MOTION` 2、`DENSITY` 7。
**功能契约**：查询 → 结果行（标题 + 副标题 + 右缘组）→ 回车执行；捕获输出页 → 结局句 + 图例。
本轮一个动作都没变，只改「这两个字的字号取自哪个名字」。

## 一句话

启动器里最后两处「正文 ± N」（结果行标题 `t.FontSize + 2`、输出页状态条 `t.FontSize - 1`）
收口到阶梯档，并让 `internal/ci` 的字阶账从设置页**扩到启动器**——从此这个包里再出现
`…FontSize ± …` 就红。

## 选题与证据

`docs/liquid-glass.md` §6.74（P13-de）把启动器的九处逐处量过，落地 4 处，其余 5 处判
`未开始（取景）`，并写死下一步：「**先收口取景，再搬这 5 处，再把字阶账从
`internal/settingsui` 扩到 `internal/launcher`**」。取景那一半已由 P13-ec / P13-ed / P13-ee
收口（§6.98 / §6.99 / §6.100），所以前置条件今天成立。

复核 5 行时发现表已过期两行：de-5 / de-11（助记提示的绘制与量算）早在 P13-eb（§6.86）就落地了，
表里还写着 `未开始（取景）`。本轮把它改成 `已落地（P13-eb）`，并落地剩下三行
（de-2 / de-3 是行盒测量，de-6 / de-9 是两处绘制）。

实测四个步长上的行盒（`internal/ci/fontladder_contract_test.go` 的表，参考字体）：

| 表达式 | tiny 0.8 | small 0.9 | default 1.0 | large 1.1 |
|---|---|---|---|---|
| `t.FontSize + 2` 行盒 | 18 | 19 | 22 | 23 |
| `tokens.Title` | **17** | 19 | 22 | 23 |
| `t.FontSize - 1` 行盒 | 13 | 15 | 17 | 19 |
| `tokens.Caption` | **14** | 15 | 17 | 19 |

出货档（0.9）上两者重合——**这正是它们活了这么久的理由**；用户把「界面大小」调到最小一格，
结果行标题与输出页状态条是这一页仅有的两件不跟步长走的东西。

## 逐条落地

| # | 位置 | 现状 → 应改成 | 状态 |
|---|---|---|---|
| 126-1 | `launcher.go` `View` 的行盒测量（标题行） | `MeasureText(… t.FontSize + rowTitleStep)` → `tk.Title` | `已落地` |
| 126-2 | 同上（副标题行） | `MeasureText(… t.FontSize)` → `tk.Body`（同值，只为名字一致） | `已落地` |
| 126-3 | `launcher.go` 结果行标题的绘制 | `FontSize(t.FontSize + 2)` → `FontSize(tokens.Title)` | `已落地` |
| 126-4 | `output.go` 捕获输出页状态条 | `FontSize(t.FontSize - 1)` → `FontSize(tokens.Caption)`（令牌在 `outputBody` 头部取一次） | `已落地` |
| 126-5 | `height.go` 的 `titleLine = font + rowTitleStep` 回退 | **不动**：它是「还没量过时的地板」，不是画出来的寄存器 | `故意不做（附理由）` |
| 126-6 | `internal/ci` 的字阶账 | 单包常量 → **按包的表**（`settingsui` 30 处下限 + 两条具名例外；`launcher` 10 处下限、**零例外**） | `已落地` |
| 126-7 | 墨 / 字重 / 高度链 / 键盘模型 / i18n / 命令接线 | 一字节不动 | `已落地（不变）` |

`§6.74` 的 de-2 / de-3 / de-6 / de-9 四行随之改 `已落地（P13-fe）`；de-5 / de-11 改
`已落地（P13-eb）`。

## 台账

**V 0 / B 0 / S 0** —— 一条 pin 都没改。`TestTheLauncherHeightsStayOnTheLadder`（出货尺六档
`15 / 17 / 18 / 19 / 25 / 25.2`）、`TestLongRowTextIsCutNotWrapped`、
`TestTheOutputListNeverScrollsIntoTheHeaderBand` 全绿：出货档上两个寄存器重合，所以这些
pin 量到的字一个像素都没动。

新增 3 条（1 条 opt-in 出图）：
`TestTheResultRowsTitleIsTheLaddersTitle`、`TestTheOutputStatusStripIsACaption`、
`TestDumpP13FELadderPNGs`。两条 pin 都在 **tiny 档**上量，期望值是「同一个字符串在同一主题下
按两个寄存器各画一次的盒」（`fontladder_test.go` 的 `p13deProbeBox`），并先断言两个寄存器
在这一档真的画得不一样——否则它看不见自己要钉的东西。

`internal/ci` 的 `TestTheSettingsPackagesTextSizesAreLadderRungs` 现在跑两个包（子测试
`../settingsui` / `../launcher`），判据 ①②③ 一字未改；`TestTheFontLadderExceptionsAreStillNamed`
同样按包走（启动器没有例外可开）。

## mutation（三条实跑）

| # | 注入 | 结果 |
|---|---|---|
| M1 | `output.go` 状态条退回 `t.FontSize - 1` | 红 ×2：`../launcher/output.go:703:67: the size is an expression (`t.FontSize - 1`), not a rung of the ladder…`；`the status strip's line box = 13, want the caption rung's 14 (body-1 draws 13)` |
| M2 | 结果行标题退回 `t.FontSize + 2` | 红 ×2：`../launcher/launcher.go:2669:46: the size is an expression (`t.FontSize + 2`)…`；`the row title's line box = 18, want the title rung's 17 (body+2 draws 18)` |
| M3 | 等价写法 `tokens.Caption` → `a.tokens(c).Caption` | **全绿**（证明 pin 钉的是「档」，不是字面量） |

每条都 `md5sum -c` 逐字节核对还原（`internal/launcher/output.go`、`internal/launcher/launcher.go`
两枚 md5 成功），并 `touch` 后复跑一次避免测试缓存复用。

## 门槛（实测，HEAD 前 `33775080` → 本轮）

```
gofmt -l cmd internal                              空
go vet ./...                                       净
GOOS=windows go vet ./...                          净
GOOS=darwin  go vet ./...                          净
go test -count=1 ./...                             0 FAIL
PASS 计数                                           3396 → 3400
SKIP                                               187 → 188（新增的 opt-in 出图）
测试函数                                            2563 → 2566
三平台 build                                        linux / windows / darwin 全过
```

## 出图（tiny 档，深浅两套）

`before` 用 `git worktree` 在 HEAD `33775080` 渲（把本轮的 dump 测试拷进去，生产代码是 HEAD 的），
`after` 是工作区。

| 帧 | 浅 | 深 | bbox |
|---|---|---|---|
| `rows`（启动器十行） | **24081 px** | **24189 px** | `(6,75)-(713,400)` |
| `strip`（输出页状态条） | **6531 px** | **6539 px** | `(6,65)-(713,276)` |

文件：`/tmp/p13fe/{before,after}/floter-P13-fe-{rows,strip}-{before,after}-{light,dark}.png`。

**这张图上量到的一处变化**：`rows` 那一帧的 bbox 从 y=75 起——那是第一行的标题行；改动后
十行的标题**整行墨都在同一个位置**（行盒没动），变化的是标题字形的宽度（0.2 点的字号差遍布
十行），所以 24k 个像素的差全是**同一批字的抗锯齿边**，没有一行移动。`strip` 的差只有 6.5k，
因为那一行是整页唯一的 caption 句，字更小、笔画更少。

## 这轮为什么让它更轻、更有空间感

同一张卡里的字从此共享一条**随界面步长缩放的阶梯**：用户把「界面大小」调到最小一格时，
结果行的标题、输出页的结局句会和副标题、图例一起呼吸，而不是单独两件留在原地——
层级关系在**任何一档**都成立，而不是只在出货档成立。这不是「更好看」，是「同一套比例在
四个档上都不散架」。

## 偷了什么（qiaomu-design · `craft-loop.md` 工艺密度菜单）

- **第 9 项（排版细节：一条比例尺，而不是一堆偏移量）** → 落成**四个步长上的一张表**
  （`internal/ci` 的 18/17、13/14）：把「比正文大一点」这个说法换成阶梯上的**名字**，
  并让契约测试替这张表站岗（`fontLadderAccounts`）。取的是「比例尺」这一项，落成的具体数值
  是 `tokens.Title` = 12.6 与 `tokens.Caption` = 9.9（出货档），tiny 档 17 / 14 的行盒。
- **第 12 项（材质细节：接缝只许一个值）** → 落成**量的与画的是同一个名字**：行盒测量从
  `t.FontSize + rowTitleStep` 改成 `tk.Title`，与 `row` 里画的那个档逐字相同（de-2/de-3）。
  这条是「接缝」在**排版**上的等价物——同一个盒子在计费与绘制两处不许有两个来源。
- 第 5 项（多层染色阴影）**不适用**：本轮不动任何面与影。
- 两轮制：第一轮先量（复核 §6.74 的五行、发现两行已过期）→ 四视角里「工程师」（量的与画的
  两个来源）与「读者」（把界面调小一格时这两处不跟）各挑出一处 → 第二轮只改这两处 + 扩账。

## preflight 逐条（`preflight.md`）

- **A 反 AI 套路**：零新增视觉语汇（不动色、不动面、不加影/发光/渐变/斜体），A 节无一项被触发。
- **B 设计读取一致性**：读取仍是「近黑/近白底 + 三级面 + 发丝线 + 紧凑行」的轻便液态玻璃；
  本轮只把两个字号的**来源**换成阶梯名字，读取不变，记忆点不变。
- **C 动效抽查**：本轮不动动效（`Transition` / `ContentTransition` / 呼吸全部原样）。
- **D2 中文页**：zh 臂与 en 臂走同一条代码路径，字号未改（出货档逐值相同，tiny 档同样逐值相同）；
  **未过项**：正文 10.8px / caption 9.9px 对 D2「正文 ≥ 14px」仍是沿用中的**需拍板**项
  （第 73 份证据），本轮不擅自改字阶——本轮恰恰是在**不改任何一档字号**的前提下做的。

## 仍待

1. **下一格**：通用队列 · **状态反馈**的另一半（禁用 / 只读面已在 P13-eu / P13-s / P13-t / P13-aq
   收口，剩下的只有「忙 / 进行中」这一类：终端会话连接中、扩展安装中），**先量再改**。
2. **需拍板（沿用，未动）**：见 `.pending-bugs.md` 顶部「仍待」第 2 条。
