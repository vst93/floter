# P13-az 报告：设置页 10 处提示戴上浮面（§6.20 收官）

**方向一句话**：`internal/settingsui` 里最后 10 处**框架反色片子**换成 `theme.Tokens.Tip`
的浮面 —— 到这一轮，全仓（终端页 / 启动器 / 设置页）的提示是**同一个材质、同一个 z 档**。

**这轮为什么让它更轻、更有空间感**：提示此前是设置页里唯一一枚**不在 z 阶梯上**的面：
它按构造与脚下的地面**反极性**（深色档一枚 237 级的近白片、浅色档一枚 27 级的近黑片），
既不透明、也没有受光边，读起来是「贴在玻璃上的一张贴纸」；现在是半透明的 `Float` 面
（深 20/20/23、浅 250/250/251）、顶边一道 `EdgeHighlight` 冠线（深 67、浅 255）、
一圈 `InputStroke` 发丝 + 两层影 —— 同一张纸上浮起来的另一张纸。

## 一、先量：改前 / 改后（全部实测，深/浅两套）

在设置页的真实地面（`realPageGround`）上渲染一个锚点，指针歇 700ms，取提示矩形里
**出现次数最多的颜色**（= 面，字形与 1px 线都是少数）：

| 外观 | 地面 | 改前面（框架 `inverse()` → `Text`） | 改后面（`FaceOver(地面, Float)`） | 冠线那一行 |
|---|---|---|---|---|
| 浅色 | 247/247/248 | **27/27/29**（近黑片，反极性） | **250/250/251**（`FaceOver` 精确相等） | 255（**+4**） |
| 深色 | 14/14/16 | **237/237/239**（近白片，反极性） | **20/20/23**（`FaceOver` 精确相等） | 67（**+46**） |

改前的两个数不是估的：`docs/rounds/P13-az-spec.md` 的 M2 变异（把 `Tip` 的面退回
`Theme.Text`）就是框架那枚片子的复现，它的报错串逐字给出了 `27/27/29` 与 `237/237/239`。

**这一轮顺带量清了 §6.20 的一条旧结论**：P13-ax 说「浅色冠线 −13 级、所以浅色不假装有」。
本轮在同一枚 `Tokens.Tip` 上量到 **+4**（250 → 255，0.92 近白压 250 就饱和）。同一次测量里
把冠线删掉（M3），那一行退成 `InputStroke` 发丝 = **227（−23）** —— 所以冠线本身是把那一
行**提亮**的，p13-ax 量到的 −13 是被亚像素劈开的**发丝行**。合约已按这条重写，pin 也从
「只断言深色」改成「两个外观都断言读得出（≥ +1 级）」。

`gofmt`/`vet` 门槛之外，三条 pin 的读数：面、冠线、可见框 `{0 43 153×41}`（浅）/
`{3 53 137×29}`（深）。

## 二、落地：10 处，一处分片不落

| # | 位置（改后行号会漂，按函数名找） | 锚点 |
|---|---|---|
| 1 | `rows.go` `sessionIconButton` | 28u 图标键（Play / Trash） |
| 2 | `rows.go` `sessionKillConfirm` | 28u 危险药丸 |
| 3 | `rows.go` `rowTitle` | **截断名**（文本锚点） |
| 4 | `settingsui.go` `integrationRow` | **包名**（文本锚点） |
| 5 | `settingsui.go` `integrationRow` 的 Run 键 | 禁用理由 |
| 6 | `settingsui.go` `integrationRow` 的 Homepage `rowAction` | 行忙时禁用 |
| 7 | `settingsui.go` `repairAction`（进行中面） | `busyActionButton` |
| 8 | `settingsui.go` `repairAction`（静息面） | 重连 / 重查 |
| 9 | `settingsui.go` `freshnessDot` | 8×8 非交互状态点 |
| 10 | `config.go` 字段行 | 必填 `*` 标记（文本锚点） |

**读数**：`internal/settingsui` 的 `.Tooltip(` **10 → 0**，`tokens.Tip(c, ` **0 → 10**
（`rows.go` 3 / `settingsui.go` 6 / `config.go` 1，普查 pin 逐条点名）。

**行为零改动**：0.6s 停留、0.4s 热接力、Esc 关掉、锚点方向、80u 上限、`Description`
的无障碍句子、每处的 `Label`/`Clicked`/`Disabled`/`Cursor` 全部沿用。既有 B 类 pin
（`TestATruncatedSessionNameStillReadsInFull` 等）零改动全绿；新加的
`TestATruncatedIntegrationNameStillReadsInFull` 把第 4 处也钉上（文本锚点仍是读全名的出口）。

## 三、顺手量到的一条框架事实（**没改，记入拍板**）

`ui/tooltip.go:133` 是 `s != nil && s.tip && s.flags&flagDisabled == 0` ——
**禁用元素不弹提示**。所以第 5 处（「Run 为什么不能按」）与第 7 处（repair 进行中）
的提示在指针下**根本不会出现**；第 1/2/6/8 处在禁用/忙碌态同理。这一轮迁材质只是让
「可交互的那一档」变对。要真让禁用态说出理由，得改框架的悬停语义（或改成 toggletip），
属于「必须改交互才能做好」，本轮不动，写进 `.pending-bugs.md` 的「需用户拍板」。

## 四、偷了什么（`qiaomu-design` · 本轮真用了的部分）

读的：`SKILL.md` → `references/user-preferences.md`（全量）→ `craft-loop.md` +
`preflight.md`（按本轮表面挑的两个 reference，`apple-fluid-interfaces.md` 与中文排版本轮
不适用：没有手势、没有中文字阶改动）。走的是「已有方向 → 打磨模式」，**没有**出四方向
预览页（用户已把方向钉死，这条车道不许停下来等点选）。

**`craft-loop.md` 工艺密度菜单，逐项对号（写不出数值的不算落地）**：

1. **第 12 项「材质细节 / 内边框高光」**：`InsetShadow(0, 1, 0, 0, EdgeHighlight)`，
   实测那行 **+4（浅）/ +46（深）** —— 不是渐变白边，是 1px 的受光边，且只给浮层。
2. **第 5 项「多层染色阴影（2-3 层、拒绝单层灰）」**：`0 6px 18px 0.30` +
   `0 1px 2px 0.20` 两层，正好替换框架那**一层** `0 2px 8px 0.2`（既不是环境影也不是
   接触影）。数值取自 `--elev-3-compact` 在 scale 1 上的字面值，**不走** `ElevThrow`
   （那个跟的是窗框的 `main_opacity`，提示不在框里）。
3. **第 6 项「编辑细节 / hairline」**：外圈 1px `InputStroke` —— 浅色档冠线虽在，
   但「盒子在哪里结束」仍然靠这圈发丝，两条一起才读得出边。
4. **第 2 项（`::selection`）与第 4 项（签名交互）本轮不适用**：自绘 UI 没有选区，
   提示的签名交互已经是框架的 0.6s/0.4s 节拍（不许为了密度改它）。
5. **第 11 项「图形资产」不适用**：这一轮不新增图标。

**用户偏好账本**：P-5（可读性优先于气质）—— 提示的字号与对比度一个字节没动，只换材质；
P-15（顶部 bar 与内容共享层级）的同类推理用在浮层上：提示与抽屉/对话框/便条同材质；
P-14（不常驻占版面的系统自言自语）—— 提示仍然是「需要才出现」，没有加任何常驻说明。

## 五、`preflight.md` 逐条（A / B / C / D2）

- **A 反 AI 套路**：无渐变文字、无紫蓝光晕、无纯黑、无斜体、无左侧竖线、无三等分卡片
  —— 本轮只改一个浮层的墨，全部通过。**过。**
- **A2 进攻三步**：DNA 注入见上（数值可指认）；工艺密度 3 项落地 / 2 项判不适用；
  两轮制：第一轮落地 → 四视角挑刺 → 第二轮只修两处（见下）。**过。**
- **B 设计读取一致性**：方向是用户钉死的「轻便液态玻璃」，本轮的判据是 §2 判据 3
  （两层影同时在场才读得出浮起一张纸）+ §3（受光边只在浮层）——10 处全部满足，
  面板内元素一个都没加影。**过。**
- **C 动效抽查**：0.6s 停留 / 0.4s 热接力未改；pin 在 `ReduceMotion: true` 下仍拿到提示，
  说明减弱动效没有把提示一起关掉。**过。**
- **D2 中文页**：提示的文案全部来自 i18n（本轮没写新句子），但**字号不达标**：
  `FontSize - 1` ≈ 9.8px，低于 preflight 的「正文 ≥ 14px」。**不能过** —— 理由：这是
  `tokens.Body` 10.8px / `tokens.Caption` 11px 那一族的**全局字阶**问题，已挂在
  `.pending-bugs.md` 的「需用户拍板」；本轮只统一材质，不擅自改字号。

**四视角（各挑一处，共四处）**：

- **艺术总监**：改前设置页的提示是全仓**第七种**说法（5px 角 + 反色 + 一层影），
  与抽屉/对话框/便条/终端页/启动器都不同年 —— 这轮之后是同一句话。修：迁移本身。
- **工程师**：`repairAction` 里没有 `tokens` 变量，机械迁移会撞作用域；补了
  `tokens := a.tokens(c)` 并把两处都收在该变量下。修：一次性。
- **甲方**：用户问「这个按钮为什么不能按」——提示**不会弹**（框架 gate）。修不了，
  记入拍板（第三节）。
- **读者**：眯眼看，改前深色档提示是一块比正文还亮的白片，抢视觉焦点；改后它退回到
  「浮起来的一张纸」。修：材质。

## 六、V / B / S 台账

- **V 0**：不动任何已 pin 的视觉数值；`Tokens.Tip` 的数值是 P13-ax 定的，本轮只多用它。
- **B 0**：既有测试**零改动**，全绿（含 `TestATruncatedSessionNameStillReadsInFull`）。
- **S 0**：没有结构变化（`Tip` 返回锚点，链式写法拆成「先建、再 Tip」是局部改写）。

## 七、变异（3 条实跑，全部变红并逐字节还原）

1. **M1** `repairAction` 的静息面退回 `.Tooltip(tooltip)` →
   `settingsui.go still rides the framework's chip in 1 place(s)` /
   `carries 5 tokens.Tip calls, want 6` / `carries 9 tips, want 10`。
   还原 `md5sum -c` 通过（`b26dd6a5…`）。
2. **M2** `Tokens.Tip` 的面退回 `Theme.Text` →
   `the tip's face is (27,27,29 a255); the pane on this ground is FaceOver((247,247,248 a255), Float) = (250,250,251 a255)`（浅）与
   `face is (237,237,239 a255) … = (20,20,23 a255)`（深）。还原 `md5sum -c` 通过（`4789e242…`）。
3. **M3** 删掉 `Tip` 的 `InsetShadow(0, 1, 0, 0, EdgeHighlight)` →
   `the light crown is -23 levels against the face ((227,227,228 a255) → (250,250,251 a255))`。
   还原 `md5sum -c` 通过（`4789e242…`）。

**没有做的第 4 条**：把 `RadiusSM`（8）退回 5 的方角变异 —— 8px 与 5px 在这一枚
137×29 的盒子上**没有任何现有 pin 读得出差**（面仍然是同一个面），硬造一条「角上像素差」
的断言只会变成脆弱 pin。诚实记录：本轮没有为「圆角 8」新增举证。

## 八、门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 全绿（FAIL 0）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 3117（基线 3112，+5）
SKIP 计数                                              → 102（基线 102，不变）
测试函数（grep -c "^func Test"）                        → 2297（基线 2294，+3）
GOOS=linux/windows/darwin go build ./...               → 全过
```

## 九、出图（opt-in，`FLOTER_P13AZ_DUMP=1`）

`/tmp/floter-P13-az-settings-tip-dark.png`、`/tmp/floter-P13-az-settings-tip-light.png`
（360×200，设置页真实地面 + 锚点 + 提示）。

**图上看到的一处变化**（深色那张）：提示是一块比地面亮 6 级的深灰纸（20/20/23 vs
14/14/16），顶边有一道明显更亮的 1px（67），下面还有一圈渐隐的影 —— 改前这里是
一块 237 级的近白片，比它下方的锚点文字还亮。浅色那张相反：提示是一块比地面亮 3 级的
近白纸（250 vs 247）+ 一道 255 的顶边，改前是 27 级的近黑片。

## 十、抢救 stash 核对（开工时 `git stash list`）

- `stash@{0}` `afc38b247c`（2026-10-31 10:06，parent `91bcf055`）的补丁是
  `metaSeparator(fact) string` + 把三个 `·` 拼进事实串 —— **已被 P13-af 覆盖**：
  HEAD 用的是 `a.metaFact(c, tokens, fact, lead)`（`settingsui.go:2142-2162`），
  同一语义（点随着它引入的事实走）用方法而不是字符串拼接实现，且带完整注释。
  合并它会把已落地的 `metaFact` 退回字符串拼接，**不接**。
- `stash@{1}` `114b7d1585`（09:16）只改 `.pending-bugs.md`，那几段文字已在文件里
  （P13-ad/P13-ac 的账）。**不接**。
- 更早的 `stash@{2}`… 已在 P12-a/P13-ac 的账里核过，见本文件前几节的历史条目。

## 十一、仍待（P13-az）

1. **需拍板（本轮新记）**：`ui/tooltip.go:133` 的 `flags&flagDisabled == 0` —— 禁用元素
   不弹提示。设置页有 3 处（Run 的禁用理由、repair 进行中面、行忙时的三个键）的提示
   因此**永远不出现**。要改就得动框架的悬停语义或换成 toggletip。
2. **需拍板（沿用）**：`tokens.Body` 10.8px / `tokens.Caption` 11px 与 preflight D2
   「正文 ≥ 14px」的全局字阶张力；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度。
3. **需拍板（沿用，终端弧）**：S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、
   S2 尾（客户端不告而别）。
4. **下一格**：`§6` 表已无 `未开始` 行 → 回通用队列的「状态反馈」另一半（禁用/只读面）
   或空态满态，**先量再改**。`Lens` 那一族已结案（只剩 1 个调用点且是 `故意不做`）。
