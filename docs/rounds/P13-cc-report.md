# P13-cc 报告：字段材质只有一处决定（三个绕开 `fieldFace` 的字段面收进来）

轮号 **P13-cc**｜规格 `docs/rounds/P13-cc-spec.md`｜合约新增 `docs/liquid-glass.md` **§6.46**

## 一句话意图

`fieldFace` 的注释从 P8-c 起就写着「它是字段材质唯一被决定的地方」，但三块字段面在自己拼
**同一行**材质：`aliasField`（外圈 `Hairline`，唯一合法差）、`sheetFieldSlot`、
`configMultiselectBox`（外圈 `ControlEdge`，与 `fieldFace` 逐字节相同的两行）。这一轮把
「谁决定」收成一处：**调用方只能选画框，不能选材质**。

**这轮为什么让它更轻、更有空间感**：字段是这一代唯一「面 + 一对受光/背光线」的语法。
材质散在三处 = 下一次调 `LensRim`/`LensEdge` 或加锁定臂时，必有一块留在旧材质上，
同一个面板里出现两种「能打字的槽」——那正是用户读到的「平、糊、旧」（层级不由面差说，
而由谁记得改谁说）。收口之后，「字段」这个概念在代码里只有一个坐标。

## 改法（结构；墨零改动）

| 文件 | 改前 | 改后 |
|---|---|---|
| `internal/settingsui/textinput.go:85` | `fieldFace` 内联 `Border(1, tokens.ControlEdge)` | `fieldFace` = `fieldFaceEdge(..., tokens.ControlEdge)` 的壳 |
| `internal/settingsui/textinput.go:100` | — | 新增 `fieldFaceEdge(c, e, lit, edge ui.Color)`：面 `Control`、外圈 `edge`、`lit` 选 `LitFace`/`DimFace` |
| `internal/settingsui/textinput.go:207` | `tokens.LitFace(e.Background(Control).Border(1, Hairline))` | `a.fieldFaceEdge(c, e, true, tokens.Hairline)` |
| `internal/settingsui/pluginsheet.go:359` | 自己 `Border(1, ControlEdge).Background(Control)` + `LitFace` | `a.fieldFace(c, slot, true)`（只剩几何） |
| `internal/settingsui/config.go:871` | 同上 + 行尾 `LitFace(box)` | `a.fieldFace(c, box, true)`（仍在 `Clip()` 之后，内阴影顺序不变） |

`lit` 的语义与 `DimFace` 的算术一个字节没动（P13-cb 的成果不重开）。别名框的
`MinHeight(Caption+2·padY)`、`Padding`、`Radius`、`FontSize` 一行没动。

## 实测

**像素零改动（这一轮的正向证据）**：`FLOTER_PNG=1 go test -run TestDumpP13MFacePNGs`
在改前（`git stash push` 三个源文件）与改后各出一对图，`md5sum` 逐字节相同：

```
改前 light cd41341c256a15bc79fadd8b9a18adfe   dark b7431784da0a0f948f0fc81bf7735954
改后 light cd41341c256a15bc79fadd8b9a18adfe   dark b7431784da0a0f948f0fc81bf7735954
```

三块面（别名框 / 多选箱 / 命令块）的每一行都还是那几行——改的是「谁决定」，不是「决定成什么」。

**唯一保留的合法差在像素上量得出来**（新 pin，探针地面 128，读外框那一列）：

| 外观 | 别名框外框（`Hairline` 0.09） | 兄弟字段外框（`ControlEdge`） | 差 |
|---|---|---|---|
| light | 108 | 112（0.06 黑） | **4** |
| dark | 151 | 149（0.07 白） | **2** |

把参数换成 `ControlEdge` → 两边逐像素相同（`112/112`、`149/149`），pin 红（见变异 M2）。

## V / B / S 台账

- **V 0**：没有一条视觉数值被重定基线（改前改后图逐字节相同）。
- **B 0**：`TestTheMultiselectBoxIsAFaceNotARing`、`TestTheAliasFieldIsAFaceNotARing`、
  `TestTheCommandPlateIsAFaceNotARing`、`TestEveryFaceWithAnEdgeWearsTheLitPair`、
  `lockedfield_test.go` / `lockedform_test.go` 全部**零改动**通过。
- **S 0**：没有结构断言需要改写；`fieldcensus_test.go` 是新增，不是替换。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 3232（基线 3228，+4）
                                                        SKIP 124（不变）｜测试函数 2386 → 2388
GOOS=linux/windows/darwin go build ./...               → 三平台过
```

+4 = `TestEveryFieldFaceIsDecidedInOnePlace`(1) + `TestTheAliasFieldKeepsItsOwnFrame` 的
parent/light/dark(3)。

## 变异（2 条实跑，各看到红，`cp` + `md5sum -c` 逐字节还原 + `touch` 复跑绿）

- **M1** `fieldFaceEdge` 的 `lit == false` 臂退回 `LitFace`：
  `TestTheLockedSelectGoesUnlit` / `TestTheLockedFieldFadesOnce` / `TestTheLockedFieldIsStillAGroove`
  三条红，原始报错串：
  `the locked select (light): the lit edge is {240 240 241 255 [0 0 0]} but the face is {238 238 239 255 [0 0 0]} (2 levels apart)`
  与 `(dark): the lit edge is {36 36 38 255 [0 0 0]} but the face is {26 26 28 255 [0 0 0]} (10 levels apart)`。
- **M2** `aliasField` 改传 `tokens.ControlEdge`（把合法差抹掉）：
  `light: alias frame {112 112 112 255 [0 0 0]}, sibling field frame {112 112 112 255 [0 0 0]}, gap 0`，
  深色同（`149/149`，gap 0），两臂都红。

还原后 `md5sum -c` 逐字节核对通过（`textinput.go` = `666c103cdc2f5e25f4109300f4385273`）。

## 出图

`/tmp/floter-P13-cc-fields-{light,dark}-{before,after}.png`（`FLOTER_PNG=1` opt-in，
来源是 `TestDumpP13MFacePNGs`，含别名框 / 多选箱 / 命令块三块面）。
**这张图上看到的一处变化**：没有——两对图的 md5 相同。这正是本轮要的证据：收口是
「把同一句话搬到一处」，图上不该有任何一笔挪动；有挪动就是我在收口时偷偷改了墨。
（`sheetFieldSlot` 不在那套 dump 里，它的等价证据是 `litpair_census`/`glassalign` 一族
零改动通过 + `TestEveryFaceWithAnEdgeWearsTheLitPair` 的扫描数仍 ≥ 15。）

## 偷了什么（`qiaomu-design` → `craft-loop.md` 工艺密度菜单）

- **第 12 项「材质细节：内边框高光 / glass 折射边」**：本轮把它落成
  「材质只有一个决定点，外框强度是**参数**」——`fieldFaceEdge(edge)`，别名框走
  `Hairline` 0.09、兄弟字段走 `ControlEdge` 0.06（浅）/ 0.07（深）。具体数值就是上面
  实测那 4 级 / 2 级。
- **第 6 项「编辑细节：hairline」**：别名框那条 `border: 1px solid var(--hairline)` 被
  保留成参数而不是第二套材质——它是参考钉过的编辑级细节，不是可以顺手统一的墨。
- **第 5 项「多层染色阴影，拒绝单层灰阴影」**：自绘版的对应物是 `LitFace` 的
  「顶受光边 + 底对侧暗线」两笔（`LensRim` + `LensEdge`），`DimFace` 只留后者——
  本轮保证这两笔**只在一处**被写。

## preflight 逐条

- **A 反 AI 套路**：过。没有新增渐变、没有新增模糊、没有新增「白边当高光」；
  本轮**减少**了三处重复材质写法（材质重复正是 AI 生成代码里最常见的「同一件事写三遍」）。
- **B 设计读取一致性**：过。读取仍是「近黑/近白玻璃面的桌面工具，轻便液态玻璃，
  E1 = 面 + 一对 1px 线」。三块字段面在改前改后图上逐字节相同，读取没有漂移。
- **C 动效抽查**：不适用（本轮零动效、零状态变化；`lit` 的状态臂由 M1 覆盖）。
- **D2 中文页追加项**：本轮没有改任何中文文案，也没有改字号/行高/字重。
  **沿用需拍板**：`tokens.Caption` 11px 与 D2「正文 ≥ 14px」的冲突（第 8 份证据）。

## 仍待（P13-cc）

1. **需拍板（沿用 + 第 9 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
2. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；路径截断保头弃尾；按下档要不要保留那一对线（`pointerFaceAllow` 两条）；
   终端外观抽屉行要不要面；终端弧 S1b / S1c / S2 尾。
3. **下一格（通用队列）**：字段族收口之后，`docs/liquid-glass.md` §6 里已经没有「材质决定点」
   类的悬案，下一轮回「空态满态」——具体先量 `internal/settingsui/empty.go` 之外的两处空态
   （启动器结果区、扩展页命令列表）与满态（长列表的滚动收口）是不是同一档空气。**先量再改。**
4. **抢救 stash 核对（本轮第五次）**：`git stash list` 顶部三条（`33e03d1f6a` / `afc38b247c` /
   `114b7d1585`）此前已判定与 HEAD 无未接续内容（P13-bw/bz/ca/cb 四次 + 本轮 `git diff --stat`
   复核：差异全部是**我方后续提交的内容**，方向相反）。**不要再试合。**
   注：本轮临时用的 `P13-cc temp` stash 已 `pop` 回来并删除。
