# P13-cs 报告 · 分隔点：全应用唯一一枚在阶梯之外的墨

**这轮为什么让它更轻、更有空间感**：`·` 是这一支里最后一个「不是阶梯给的」墨——它画成
`TextTertiary.Alpha(0.7)`，也就是「最暗的那一格再乘一把 0.7」。半透明 ink 的合成是乘在
**地**上的，于是同一个组件在四张面上是四个高度（地面 253 上 70 级、面板面 229 上 48 级、
深色 39 上 40 级），越低的面越淡；深色卡片面上它只有 40 级，与 `HairlineSoft` 同级——
一个 2px 的灰点，读起来像屏幕上的脏，而不是标点。交给阶梯之后，四个调用点第一次是同一个
高度，而「比它引入的事实更轻」由**档位**说（`TextTertiary` 是第四档），不再由一次额外的
乘法说。

## 1. 证据（先量，产品代码未动）

`ui.Color.Alpha` 是**乘**：参考的 `.7`（`extensions.css:979-984`、`:2096-2103` 的
`opacity: .7`）是叠在一条**不透明**墨上的 CSS 乘法；本仓把同一个数接在**半透明**的
token 上，一次乘法变成两次（token 的 alpha × 0.7），并且合成结果跟着地面走。

实测（`ui.Tester` 真页面，读数取该 `·` 自身的最深像素；括号内是它离地面的级数）：

| 表面（调用点） | 外观 | 地面 | `·` | 档位 `TextTertiary` 的实测值 | `·` 占档位 |
|---|---|---|---|---|---|
| 捕获输出页条子（`launcher/output.go:794`） | 浅 | 253 | `183,183,187`（70） | `142,142,148`（111） | 0.63 |
| 同上 | 深 | 23 | `78,78,83`（56） | `110,110,118`（89） | 0.63 |
| 设置页事实点（`settingsui.go:2275`） | 浅 | 229 | `183,183,186`（45） | 事实 85（`TextTertiary`） | 0.53 |
| 同上 | 深 | 39 | `77,77,82`（39） | 事实 73 | 0.53 |
| 设置页状态字前导点（`settingsui.go:2162`） | 浅 / 深 | 229 / 39 | 47 / 40 | 状态字 `TextStrong` 196 / 193 | — |
| 浮窗口条（`shell/detach.go:274`） | 浅 / 深 | 246 / 20 | 55 / 49 | 同条带的事实 | 0.63 |

两件事读出来：① **越低的地面上越淡**（253 → 229，70 → 45），也就是「分隔」的强度跟着面漂；
② 深色档最惨：39 级 = `HairlineSoft` 同级。而且这四处**一个像素 pin 都没有**——
所以这枚墨从 P13-af 落地至今没有人量过。

## 2. 改法（只换墨，不换字阶/字重/节点）

`internal/launcher/output.go`、`internal/settingsui/settingsui.go`（两处）、
`internal/shell/detach.go`：`tokens.TextTertiary.Alpha(0.7)` → `tokens.TextTertiary`。
`TextTertiary` 是阶梯第四档（`Text > TextSecondary > TextMuted > TextTertiary`）。

- **为什么不删成更重的档**：`TextMuted` 比 `TextTertiary` **重**（浅 128 对 142、深 133 对
  110）——分隔点会与事实抢读。变异 M3 就是这个方向，三条都红。
- **为什么不留在原地**：它是全应用唯一一枚低于最后一档的墨，且四个表面四个高度；
  契约 §4.2 的「比事实更轻」仍然成立（相对关系不变，只是现在由档位表达）。
- 行盒 / 字阶（`tokens.Caption`，捕获输出页是 `FontSize - 1`）/ 字重（事实 500、状态字 600）/
  `RichText` 节点划分（§4.2 的折行行为）/ 8u+2u 的 gap / i18n 文案 **一个字节没动**。

## 3. 台账

**V 0 / B 0 / S 0**（这四处此前都没有像素 pin，本轮是**新增**三条；`metaline_test.go` 五条
量的是节点划分与折行，零改动通过）。

- 合约：`docs/liquid-glass.md` §4.2 的墨列重定基线（旧值→新值写在表下）+ 新增 **§6.62**。
- pin：
  - `internal/launcher/outputdot_test.go`：`TestTheStripDrawsItsDotFromTheLegendRung`
    （深+浅 × 三种结局）。它先在**同一张地面、同一字号**上画两个候选（阶梯第四档 / 那一档
    乘 0.7），再要求页面上那枚点落在档位 ±15 级以内、且比「乘 0.7」高出 ≥15 级。
    **±15 是字形自身的亚像素原点差**（页面里的 `·` 落在小数 x 上，比整数原点的同一枚字形
    重 12 级），不是给墨留的余量——写进注释里；正因此这条 pin **不用**「上一档」候选
    （它与档位的间距比这个偏移还小），那一半由另外两条 pin 承担。
  - `internal/settingsui/metadot_test.go`：`TestTheMetaDotIsTheFactRung`（事实点 + 状态字点，
    深+浅）。三候选（乘 0.7 / 档位 / 上一档 `TextMuted`）取最近者，必须命中**档位**。
  - `internal/shell/detachdot_test.go`：`TestTheDetachedBarDotIsTheFactRung`（深+浅），同三候选。
- 出图（opt-in `FLOTER_PNG=1`）：`TestTheStripDotDump` / `TestTheMetaDotDump`，
  `/tmp/floter-P13-cs-{strip,meta}-{before,after}-{light,dark}.png`。
  `before` 由「回退三处生产代码 → 跑 dump → `git apply` 还原 → `md5sum -c` 逐字节核对」取得。

## 4. 门槛（实测）

```
gofmt -l cmd internal                     空
go vet ./...                              净
GOOS=windows go vet ./...                 净
GOOS=darwin  go vet ./...                 净
go test -count=1 ./...                    全绿（0 FAIL）
PASS 3289（基线 3286）· SKIP 138（基线 136，新增两条 opt-in 出图）
测试函数 2437（基线 2432，+3 pin +2 dump）
GOOS=linux/windows/darwin go build ./...   全过
```

## 5. 变异（三条实跑，全部变红并逐字节还原）

| # | 注入 | 结果（原始报错串，节选） |
|---|---|---|
| M1 | 三处一起回退成 `TextTertiary.Alpha(0.7)`（= 全部改动的撤销） | 12 条断言红：`the `·` reads 69 levels off the ground, want the TextTertiary rung (… , 86)…` / `…is within 15 levels of the removed ink (…, 60): the separator is still below the ladder`；设置页 `…nearest is the removed 0.7, want the rung`；浮窗口条同 |
| M2 | 只把 `launcher/output.go` 改回 alpha | `the `·` reads 69 levels off the ground, want the TextTertiary rung ({166 166 171}, 86) within the glyph's own 15-level subpixel offset` + `the `·` (69) is within 15 levels of the removed ink ({192 192 196}, 60)` |
| M3 | 把 `settingsui.go` 事实点抬一档到 `TextMuted` | `light "· Managed": the `·` reads 75 levels off the panel (…) ; … one rung above 75 (…) — nearest is the rung above, want the rung`（深色同） |

每条都：注入 → 看到红 → `cp` 备份还原 → `md5sum -c` 成功 → `touch` 后复跑绿。

## 6. 出图复核（我自己在图上读到的一处变化）

`/tmp/floter-P13-cs-strip-{light,dark}-{before,after}.png` 与
`/tmp/floter-P13-cs-meta-{light,dark}-{before,after}.png` 逐像素差分：

```
strip  light  diff 6 px  bbox (75,98)-(77,99)
strip  dark   diff 6 px  bbox (75,98)-(77,99)
meta   light  diff 12 px bbox (371,329)-(540,330)
meta   dark   diff 12 px bbox (371,329)-(540,330)
```

条子上只有那枚 `·` 自己（3×2 px）变了，一个字母都没动；设置页那 12 px 是同一行上
**六个分隔点各 2 px**（`· vst` / `· 1.2.3` / `· 工具` / `· Managed` / `· Running` …），
从 `(371,329)` 到 `(540,330)` 一条横线上——正是「一页上的所有分隔点都是一个高度」这件事。
在浅色图上看得到的变化：那枚点在卡片面上从一块极淡的灰（`183`）变成与它后面那个事实同一
支墨（`163`），于是「· Managed」第一次读成一个整体，而不是一个点加一个词。

## 7. 设计读取（qiaomu-design · 打磨模式）

> **读取为**：桌面命令面板 + 设置面板的**标点层**，面向每天用键盘的人，用「安静、分层、
> 不喧哗」的语言，倾向 **Light Liquid Glass + 阶梯墨**（本仓 `docs/liquid-glass.md`）。
> `VARIANCE` 3（功能面，不加艺术冒险）· `MOTION` 2（本轮无动效）· `DENSITY` 6。

**偷了什么**（逐条落到数值）：

- 从 `craft-loop.md` 工艺密度菜单取 **第 6 项「编辑细节」（hairline / 大写字距标签这一档
  印刷级小件）** → 落成「分隔点用阶梯第四档 `TextTertiary`（浅 142 / 深 110），与它引入的
  事实同档」。这条动作的全部内容是「让一个小件归属某一格」，不新增任何装饰。
- 同菜单 **第 12 项「材质细节」（内边、受光边这一档）** → 落成「同一枚点不许叠第二层 alpha」：
  §1 的「一个面只说一句话」落到墨上是「一格墨只乘一次」；实测删掉那次乘法后四处点
  分别是 96 / 78 / 67 / 56 级，与各自档位差 ≤ 6 级。
- 未取：第 5 项（多层染色阴影）、第 8 项（流体字号）、第 1 项（氛围层）——本轮是墨的一格，
  不是新面；加了就是 §1 禁令 1/3。

## 8. preflight（A / B / C / D2 逐条）

- **A（反 AI 套路）**：过。没有渐变白边、没有 glow、没有模糊贴图、没有新阴影；本轮删掉的是
  一次 alpha 乘法，图案与几何零变化。
- **B（设计读取一致性）**：过。四处调用点与 `docs/liquid-glass.md` §4.2 的墨列一致，
  合约已同步（旧值→新值）；颜色仍只从 `theme.Tokens` 取，没有字面色值。
- **C（动效抽查）**：过（不适用）。本轮没有动效改动；`·` 不参与任何 transition。
- **D2（中文页）**：过（不适用）。没有改字号/行高/字重/i18n 文案；`tokens.Caption` 11px 与
  「正文 ≥ 14px」的冲突仍是**需拍板**项（第 20 份证据）。
- **E（三秒测试）**：过。这一行视线里仍然只有一个焦点（结局），分隔点退在后面；本轮只是让
  它退得**有刻度**，而不是退到阶梯之外。

## 9. 仍待（P13-cs）

1. **下一格（不需拍板，先量）**：`ui/tooltip.go:133` 的禁用元素不弹提示（用户实测里点过两次）
   ——属行为，先量出「禁用面」在三处入口的读数，再决定是否进拍板。
2. **需拍板（沿用 + 第 20 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：扩展页空态只有一行结论、没有出路（`settingsui.go:1838`）；会话页空态
   说错了**对象**（「持久化会话」vs 进程内会话）；终端页会话抽屉/标签对失败只加字不换墨；
   终端外观抽屉行要不要面；`launcher/filters.go` 过滤 chip 与 `launcher/power.go` 确认带
   chip 的按下档；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；路径截断保头弃尾；
   终端弧 S1b / S1c / S2 尾。
4. **需拍板（第 5 份证据，§6.32-4）**：同一张卡上的两种「这一层的名字」。
5. **本轮记下的边界**：捕获输出页那条 pin 用「±15 级亚像素原点」而不是第三候选，因为页面里
   的 `·` 落在小数 x 上；如果哪天真要连「抬一档」也钉在启动器这页，得先让渲染器把这条
   行按整数原点布局（那是布局改动，不是墨）。
6. **抢救 stash 核对（第二十一次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
