# P13-ez 报告：透明度组的那句「答案」比它回答的数字小一档

**方向一句话**：一张卡里「这一组说的是什么数」只有一个字号档——答案与它回答的数字同档，
层级由墨分开。

- 规格：`docs/rounds/P13-ez-spec.md`
- 合约：`docs/liquid-glass.md` **§6.121**（新增）
- 改动文件：`internal/settingsui/settingsui.go`（`a.effectiveOpacity` 两行的字号档）、
  `internal/settingsui/readoutstep_test.go`（新增，两条 pin + opt-in 出图）

## 病（实测，860×2400 真页面，深浅两套逐值相同）

| 行 | 字号档 | 盒高 | 墨 |
|---|---|---|---|
| `Main opacity`（行名） | `tokens.Emphasis` | 18.0 | `TextStrong` 580 |
| `47%`（滑块的实时读数） | `t.FontSize` = 正文档 | **17.0** | `TextStrong` |
| `Transparent` / `Solid`（轴词） | `tokens.Caption` | 15.0 | `t.TextMuted` 500 |
| `Effective opacity`（答案的名字） | `tokens.Caption` | **15.0** | `TextTertiary` 500 |
| `47%`（答案本身） | `tokens.Caption` | **15.0** | `TextSecondary` 560 |

两处病，都是**结构**：

1. **同一角色两个档**：滑块读数是「值」，走正文档（17）；这一组的总读数也是「值」，走
   caption（15）。同一张卡里两个同类数字，眼睛要在第二处重新校准一次字号。
2. **答案比问题小**：读数是这一组的结论（§6.118 原话「这一组的答案，不是又一行」），却是
   全卡唯一一个比它回答的数字低一档的东西，而且与**轴词**（卡里最不要紧的两个词）同级。

## 落地

- `settingsui.go:4880`（答案的名字）：`tokens.Caption` → `tokens.Body`；`FontWeight(500)` /
  `TextTertiary` **不动**。
- `settingsui.go:4881`（答案的数值）：`tokens.Caption` → `tokens.Body`；`FontWeight(560)` /
  `TextSecondary` / `ContentTransition{Numeric}` **不动**。
- 几何：行盒 15 → 17，是**唯一**被动过的几何。`Padding(Space(1), inset, Space(2.25), inset)`
  （§6.118 定的）不动，所以卡只长 2px；读数行**顶端仍在 Y=1589.7**（一个像素没动），
  卡下方的 `TransparencyHint` 从 Y=1662.2 → 1664.2（+2，整段随卡下沿走）。
- 墨：一个字节没动。三处同墨会让层级只剩字号在说，而字号刚被拉平（§6.120 的 120-3）。
- 轴词仍是 caption 15：它是**刻度标签**，不是值，本轮不许把它一起拉上去（pin 钉着）。

## V / B / S 台账

**V 0 / B 0 / S 0**。

- V 0：`transparencyreadout_test.go` 只钉文本内容（`copy.EffectiveOpacityValue(...)`），
  不钉字号/盒高；`slidergap_test.go` 只用这行当锚点量**缝**（§6.118），字号变化不影响它
  （盒高变 2px，缝的判据是墨带之间的距离，实测未变）。
- B 0：无行为断言依赖这行的盒高。设置页是自由滚动页，卡高 +2px 不参与窗口/终端高度链。
- S 0：结构未变（仍是一行两段 `SpaceBetween`）。

## 门槛（全部实测）

```
gofmt -l cmd internal                      # 空
go vet ./...                               # 净
GOOS=windows go vet ./...                  # 净
GOOS=darwin  go vet ./...                  # 净
go test -count=1 ./...                     # 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c "--- PASS"   # 3383（基线 3377）
                                        SKIP     # 183（基线 182，+1 = opt-in 出图）
GOOS=linux/windows/darwin go build ./...   # 三平台过
```

测试函数：**2549 → 2552**（+3：两条 pin + 一条 opt-in 出图）。

## 变异（4 条实跑，`md5sum -c` 逐字节还原 + `touch` 复跑 `ok`）

- **M1** 数值那一行改回 `tokens.Caption` → 红：
  ```
  the readout's name is 17.0 tall and its number 15.0: one line, one step (§6.121)
  the readout is 15.0 tall but the slider value it answers is 17.0: the answer wears a smaller step than the ask
  the readout is 15.0 tall: the caption step's 15 is back (§6.121)
  ```
- **M2** 名字那一行改回 `tokens.Caption` → 红：
  `the readout's name is 15.0 tall and its number 17.0: one line, one step (§6.121)`（深浅两臂）
- **M3**（等价写法）`tokens.Body` → `t.FontSize` → **全绿**。记成**已知覆盖边界**：出货尺上
  两者是同一个数 10.8，pin 量不出来；`tokens.Body` 是这页的语义档，M3 只证明「改成同一个数
  不会红」，不证明等价可以互换。
- **M4** 数值的墨改成 `tokens.TextStrong`（与行名同墨）→ 红：
  ```
  the readout's number reads 196 and the slider value it answers reads 192:
  the answer must stay quieter than the ask (§6.121 — 层级由墨说，字号已被拉平)
  ```
  **M4 第一次跑是绿的**——pin 2 原版拿数值与**行名**比（`rowWeight` 196 对 202），因为行名是
  `Emphasis`（更大的字号 → 更重的墨峰），把数值抬到 `TextStrong` 也仍然「比行名轻」。
  这不是断言写松了，是**比错了对象**：只有拿数值与**滑块实时读数**比（两者现在同为正文档，
  苹果对苹果）才是这条判据。改成这个对象后 M4 两臂都红。这一条是本轮唯一一次「pin 先假绿」。

## 出图（`/tmp/p13ez/{before,after}/floter-P13-ez-readout-{light,dark}.png`）

- `before` 用 `git worktree` 在 HEAD `f19f79b2` 渲（同一份 `readoutstep_test.go` 拷进去跑），
  `after` 在改完的工作区渲。裁图 = 透明度卡（560×218）。
- 像素 diff：**6187 px（浅）/ 6179 px（深）**，bbox `(11,174)-(559,217)`
  —— 上沿正好是读数行、下沿是卡的下沿。
- **这张图上看到的一处变化**：读数那一行在裁图里向右多出约 7 列（名字 79 → 84px、数值
  34 → 36px），卡的下沿与它下面的空白整体下移 2px；读数行**自己的上沿一个像素没动**。
- 诚实的负结果：**字形墨高没变**（9.9 与 10.8 在 1x 上栅格出同样的 12 行 cap 高，逐行 ASCII
  比对可见），所以这是一次**安静**的改动——量得出来的是盒高、行宽和卡高，不是「字突然变大」。

## 用 skill 的账（`qiaomu-design`）

开工前读了 `SKILL.md` → `references/user-preferences.md`（偏好账本）→ `craft-loop.md` →
`preflight.md`；按本轮表面（一行西文标签 + 一个数字，属于**已有方向、只要打磨**那条路径）
只读这三份 + 账本，没有全库通读，也没有走 Phase 2 的四方向预览（用户已把方向给死）。

**偷了什么**（逐条落成具体数值）：

1. **`craft-loop.md` 二·工艺密度菜单第 6 项「编辑细节」的原则——同一角色的元素必须共用一个
   刻度**：落成读数两段的 `tokens.Body`，盒高 **15 → 17**，与它回答的滑块读数逐像素同档。
2. **`craft-loop.md` 二·约束「密度不等于吵闹」**：落成「只抬字号、不抬墨」——三处墨重实测
   87 / 118 / 192（浅）与 71 / 111 / 188（深）保持三档。把墨也统一是最省事的写法，但那会让
   一张卡上出现三个同样响的数字，正是「吵闹」。
3. **`craft-loop.md` 三·两轮制**：第一轮改完出 4 帧（深浅 × 前后）+ 四视角评审（见下），
   第二轮只做一件事——确认**卡内其它行没被拖动**（读数行顶端 Y=1589.7 不变，只有卡下方 +2px）。

**四视角评审**（各挑一处）：

- **艺术总监**：这张卡此前是「两个大数字 + 一行小字」，那行小字其实是这张卡的**结论**；
  它现在与它回答的数字同档，卡片读成一段话而不是「正文 + 脚注」。
- **工程师**：改动只有两个 token，且墨、字重、内边距、`ContentTransition` 全不动——
  回归面被压到「盒高 +2」这一件事上，pin 也只钉这一件事。
- **甲方**：透明度这一组现在从上到下是「名字 / 数字 / 轨道 / 刻度 / 答案」，答案不再比刻度小。
- **读者**：眯眼看，读数行不再是卡里最轻的一段；眼睛滑到卡底时不用再降一次字号。

## preflight 逐条结论

- **A 反套路**：无渐变白边、无外发光、无模糊贴图冒充玻璃；本轮只动一个字号 token。**过**。
- **A2 进攻三步**：DNA/工艺密度见上 3 条；两轮制已走（含一次 pin 假绿的复盘）。**过**。
- **B 设计读取一致性**：一句话记忆点 =「答案和它的前提同一个字号，层级由墨说」；明暗两套
  各跑一遍，`tokens.Body` / `TextTertiary` / `TextSecondary` 都读当前主题，没有混主题。**过**。
- **C 动效抽查**：本轮零新动效；`ContentTransition{Numeric: true}` 原样保留（它只在数值变化时
  滚动，不是入场动画，且 `ReduceMotion` 在测试里开着跑过）。**不适用**（不是静默跳过）。
- **D2 中文页**：本轮不新增任何可见文案；中文 `EffectiveOpacity` 串随字号 10.8 变大，行盒
  17 → 17/10.8 ≈ **1.57 倍**（≥1.5 过）；字重仍只用 500 / 560。**字号 ≥ 14px 仍不过**——
  沿用需拍板项（`tokens.Caption` 9.9 / 正文 10.8），本轮不动，也不假装过。

## 仍待 / 下一格

1. **下一格（通用队列 · 边界情况）**：本轮动过的这行是 `SpaceBetween` 两段——名字与数值
   在**窄窗 + 中文长串**（`有效不透明度` + `有效值`）下会不会相撞/回绕。先量（720 / 640 /
   最小窗宽 × 中英两套），再决定是加 `SingleLine()` 还是让名字先让位。**先量再改。**
2. **需拍板（沿用，未动）**：`tokens.Caption` 9.9 / 正文 10.8 对 preflight D2（第 72 份证据）/
   启动器两行结果行 36px 盒装 27 行墨 / §6.111 / §6.113-5 / §6.114-4 / `104-2` / 便条行
   rank 1.28 / 终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 / 会话页空态
   说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` / 多选箱宽度 /
   路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
3. **本轮证伪的一条**：没有顺手把轴词（`Transparent` / `Solid`）也抬到正文档。它是**刻度
   标签**不是值；抬上去会让「值」这一档失去意义，而且卡会再长 2px 却没有换来任何层级。
   合约 §6.121 把「值走正文档、标签走 caption 档」写成规矩，而不是留一个待办。
