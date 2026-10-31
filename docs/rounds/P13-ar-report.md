# P13-ar 报告：一张整卡不可用的卡片——淡只有一道，灯是熄的

方向：**细节打磨 / 状态反馈的另一半（禁用面）的最后一处尾巴**（`docs/liquid-glass.md` §6 无
`未开始` 行 → 通用队列）。规格：`docs/rounds/P13-ar-spec.md`。合约条目：§6.2b-6 收口 +
新增 §6.15。对象是 P13-aq「仍待 1」点名的 `uninstall.go:442`。

## 一、这轮为什么让它更轻、更有空间感

卸载对话框里那一行「程序本体」是全仓唯一一张**整卡**不可用的卡片，而它同时做了三件互相
抵消的事：卡自己 `.Opacity(0.6)` 把整棵子树洗淡（一个 0.6 的档，不在框架那档 0.5 上）、
里面那个勾选框又自己挂了一粒 `Disabled`（框架再乘 0.5，勾选框有效 **0.30**）、而卡上那条
`LitFace` 冠线写在 `Fixed` 臂**里**——于是一块按不动的面仍然点着灯。用户读到的不是
「不可用」，是「看起来能碰、碰了没反应」，而卡的墨比同页其它禁用面更淡、台阶更浅，
整块面积浮不起来也沉不下去。这一轮把它收成和全仓同一个语法：**一道淡（框架的 0.5，
落在卡的根上）+ 灯熄（`DimFace`）**，勾选框只继承、不再自带 flag。三件东西第一次说同一句话。

## 二、落地

| # | 文件 | 改动 |
|---|---|---|
| 1 | `internal/settingsui/uninstall.go:440`（`case part.Fixed`） | `card.Opacity(0.6)` + `LitFace(card)` → `card.Disabled(true).Cursor(NotAllowed)` + `tokens.DimFace(card)` |
| 2 | 同文件 `:487`（固定行的勾选框） | 删 `box.Disabled(true)`（从卡根继承），保留 `Cursor(NotAllowed)` |
| 3 | `internal/settingsui/fixedpartdim_test.go`（新） | 两条 pin × 深浅两套：卡（灯熄 / 面台阶 / 字只淡一道）+ 卡里的勾选框（只淡一道） |
| 4 | `internal/settingsui/fixedpartdim_dump_test.go`（新） | opt-in 出图（四行组件卡 × 深浅，`FLOTER_P13AR_TAG=before` 拍改前） |
| 5 | `docs/liquid-glass.md` §6.15 | 18-1…18-9 逐行 + 两条反直觉 |

**没有动的东西**：`Cursor(NotAllowed)` 的语义（卡与框都还在）、几何（`Padding(3u)` /
`Gap(3u)` / `Radius(2u)` / `Border(1, ControlEdge)`）、`Value != nil` 才写回草稿、i18n 文案、
键盘模型、高度链、会话管理。

## 三、实测（同一把尺：`dimProbe` 的读数法，litSum/3 = 亮度级）

| 位置 | 量 | 旧 | 新 |
|---|---|---|---|
| 固定卡的冠线 | 顶内线 − 面（浅 / 深） | **+3 / +12** | **0 / 0** |
| 固定卡的面 | 面离地面（浅 / 深） | 11 / 17 | **9 / 12**（= §6.2b-6 那族禁用面的同一档：浅 238 vs 247、深 27 vs 15） |
| 固定卡的字 | 字/面对比（浅 / 深，可用态 202 / 198） | 124 / 124（0.61 / 0.63） | **105 / 105**（0.52 / 0.53） |
| 卡里的勾选框 | 墨/面对比（浅 / 深，可用态 148 / 154） | **46 / 49（0.32 / 0.32）** | **78 / 84（0.53 / 0.55）** |

两条要写下来的反直觉（§6.15 里也记了，免得下轮重猜）：

1. **整卡的 `.Opacity(0.6)` 不是「两次淡」，是「另一档淡」。** 它只作用一次，所以卡上的字
   旧值是 0.61、新值反而降到 0.52——单个 0.6 比框架的 0.5 更亮。真正被淡两遍的，只有
   **自发挂 `Disabled` 的子件**（这里就是勾选框，0.32）。所以本轮的判据不能写成「字更亮了」，
   只能写「勾选框回到一道淡」。
2. **把 flag 从子件挪到根上，语义没有变、像素变了。** `node.IsDisabled()` 沿父链上溯
   （`mygo@v0.7.3/ui/element.go:1348-1355`），所以卡根上一粒 `Disabled` 已经让里面的勾选框
   不可用（命中、游标、可聚焦性都读到它），而画笔只在**自带 flag 的那一层**乘 0.5
   （`paint.go:257-259`）——一道淡。`TestComponentizedUninstallCardAnswersThePointer`
   零改动仍绿，就是这条的旁证。

## 四、出图与整帧 diff

`FLOTER_P13AR_DUMP=1 go test -run TestDumpP13ARFixedPart ./internal/settingsui/` 写出
`/tmp/floter-P13-ar-fixedpart-{light,dark}-{before,after}.png`（四行组件卡；before 由把
旧写法整条回灌后拍，即变异 M1）。PIL 逐像素比：

```
light (460, 360) diff px 22766 bbox (20, 20, 400, 80)  changed rows 20..79
dark  (460, 360) diff px 22768 bbox (20, 20, 400, 80)  changed rows 20..79
```

**这张图上看到的一处变化**：diff 的 `y ≤ 80` 恰好是**第一张卡**（程序本体那一行）的高度，
下面三张可选的卡零位移——杀的只是这一张。变化里最先跳出来的是那张卡的顶边白线消失
（浅色 +3、深色 +12 的那条冠线），其次是勾选框里的对勾从「几乎看不见」回到能读出来，
而卡的面本身几乎没动（台阶 11 → 9，与同页其它禁用面齐平）。

## 五、V / B / S 台账

- **V 0**：没有一条钉旧数值的断言。新加的两条 pin 是本轮新账。
- **B 0**：`go test -count=1 ./...` 全绿，`TestComponentizedUninstallCardAnswersThePointer`
  （游标）与 `TestComponentizedUninstallCardDropsItsShadowOnPress`（按下退成平涂）零改动。
- **S 0**：没有接口/结构变化；`uninstallPartRow` 的签名、返回的盒子、`part.Fixed` 的语义都不变。

## 六、门槛（实测，HEAD `666a5c9f` → 本轮）

```
gofmt -l cmd internal                                # 空
go vet ./...                                         # 净
GOOS=windows go vet ./...                            # 净
GOOS=darwin  go vet ./...                            # 净
go test -count=1 ./...                               # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"    # 3060 → 3062
go test -count=1 -v ./... | grep -c -- "--- SKIP"    # 98 → 99（+1 opt-in 出图）
go test -count=1 -v ./... | grep -c -- "--- FAIL"    # 0
测试函数（同一把尺：只数 cmd/ + internal/）          # 2268 → 2271
GOOS=linux|windows|darwin go build ./...             # 三个都过
```

## 七、变异（3 条实跑，全部变红，`md5sum -c` 还原成功）

| # | 注入 | 结果 |
|---|---|---|
| M1 | 整条旧写法回灌（`Opacity(0.6)` + `LitFace` + `box.Disabled(true)`） | **两条 pin 全红**：`the fixed component card (dark): top inner row {41 41 43 255} is 12 levels over the face {29 29 31 255} — a row that cannot be touched must not wear the mark of one that can` / `the fixed card's checkbox (light): 46 levels off the face, 0.32 of the enabled 148 — the box is faded on top of the card's own fade; want one fade (≥0.40)` |
| M2 | 只把 `box.Disabled(true)` 挂回子件（其余保持新写法） | **勾选框那条红、冠线那条绿**：`light: box inside the fixed card reads 39 levels off the face, the enabled twin's 148` / `0.26 of the enabled 148` |
| M3 | 只把 `DimFace` 换回 `LitFace`（其余保持新写法） | **冠线那条红、勾选框那条绿**：`the fixed component card (light): top inner row {240 240 241 255} is 2 levels over the face {238 238 239 255} — a row that cannot be touched must not wear the mark of one that can` |

三条还原后 `md5sum -c /tmp/p13ar-uninstall.md5` → `internal/settingsui/uninstall.go: 成功`，
并 `touch` 后复跑一次（避开测试缓存）。

## 八、偷了什么（qiaomu-design）

本轮读的是 `user-preferences.md`（偏好账本，最高优先级）+ `craft-loop.md` + `preflight.md`，
走「已有方向，只要打磨」那条车道，**没有** Phase 2 的四方向预览（任务书禁止在自动驾驶道上
出 HTML 预览页，也不许停下来等点选）。

- **`craft-loop.md` 第二节「工艺密度」菜单第 12 项（材质细节：内边框高光 / glass 折射边）**
  → 落成具体数值：这块面的材质**只保留内侧的对侧阴影 `LensEdge −1px`**（`DimFace`），
  冠线 `LensRim +1px` **归零**。判据不是「看起来灰一点」，而是「顶内线 − 面 = 0（浅 / 深）」。
- **`craft-loop.md` 第二节第 5 项（多层阴染 → 禁用于面）** → 落成 `DimFace` 不投影、
  `LitFace` 也不投影：禁用面靠**一道淡 + 灯熄**分家，不靠阴影；`§2 判据 2` 本来就是
  「E1 不投影」。
- **`craft-loop.md` 第三节「两轮制」（Render → Critique → Refine）** → 第一轮**只量不改**
  （新 pin 先在旧代码上跑，得到 0.32 与 +3/+12 两个读数），第二轮才动代码；写错的判据
  （一开始想把「字/面对比」当门槛）就是被第一轮的数否掉的——那 0.61 → 0.52 是**变淡**，
  拿它当判据会做出反方向的修改。
- **`preflight.md` A4.1「disabled / read-only / hidden 语义不许混淆」** → 落成「flag 挂在
  卡的根上，子件继承」：一个不可用**单位**只有一个 flag。

## 九、preflight 逐条结论

- **A（反 AI 套路）**：过。本轮只删两样东西（一道淡、一圈灯），没有新增渐变/发光/斜体/侧竖线。
- **B（设计读取一致性）**：过。读取 = 「轻便液态玻璃，禁用 = 面在、光熄」，与 §6.2b-6 / §6.15
  写下的判据一一对应，深浅两套同值同规则；记忆点没变（禁用面是「灰面 + 熄灯」，不是「隐形」）。
- **C（动效抽查）**：过。零动效变更；`Transition{Colors: true, Duration: theme.ColorStep}`
  一像素没动。
- **D2（中文页）**：**部分不过，如实记录**：本轮没有碰任何 11px 的 `tokens.Caption` 文案，
  卡片里的中文标签仍在 `tokens.Emphasis` 一档；`tokens.Caption` 11px 与「正文 ≥ 14px」的
  张力仍是 P13-ak 起就记着的「需拍板」，本轮没有扩大它的使用面。
- **E（三秒测试）**：过。眯眼看这张卡：它与下面三张可选卡**同一种面、不同的光**——
  「这一张不能选」这件事靠灯而不是靠模糊表达；页面上争夺注意力的元素仍是 1 个（主按钮）。

## 十、仍待（P13-ar）

1. **需拍板（沿用）**：`cardRow` 控制槽 `Shrink(0)`（长值先吃标签列）；`tokens.Caption` 11px
   与 preflight D2 的张力；终端弧 S1b / S1c / S2 尾。
2. **同族复核**：`internal/settingsui/commandlist_test.go:19` 记着参考的
   `.extension-command-list__row--off { opacity: 0.62 }`——那是**行**不是控件，且它是一道淡，
   不属于「两次淡」那族；留给下一轮判断要不要把它也归到「灯熄」语法。
3. 下一轮候选：① 空态的**满态**那一半（多条会话时的列表与页脚关系 / 剪贴板页满屏时最后一行）；
   ② `ContentAlpha()` 沿透明度滑块两端（任务书「先量再改的两个数」之一，**仍未量**）。
4. 抢救 stash：`stash@{0}`（`afc38b247c`）与 `stash@{1}`（`114b7d1585`）在 P13-ap 已核过
   （内容已在 HEAD / 只改 `.pending-bugs.md`，文字已在文件里），本轮未再试合、也未静默丢弃。
