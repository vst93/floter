# P13-cq 报告 · 文本动作的三个状态：一盏灯，不是两盏

**这轮为什么让它更轻、更有空间感**：这条动作**没有面**（`.settings-reset` 透明底，P7-gk 专门
把底带删掉了），它的状态只能靠墨与几何说。此前它一个状态挂了**两个方向相反的旋钮**——hover
把墨抬一档又乘 0.78（抬起来的 80 级只剩 31 级到屏幕上），press 把墨**退回**静息档再乘 0.6。
于是「按住不放」这一帧，在整页里读起来是最淡的一处：**用户按下最想要的那个动作，得到的是
「这条不可用」的读数**。现在三档各只有一盏灯：静息 `TextSecondary` → hover `Text` →
press `Text` + 下沉 `Scale(0.986)`（§5 的第三档），三档单调，且**没有一格面**被画出来——
空间感来自「同一块平面上的墨与一个下沉动作」，不是来自又一层底板。

## 1. 证据（先量后改）

pin 在改产品代码之前跑出来的原始串（HEAD `e6bed3b7`，产品代码未动）：

```
--- FAIL: TestTheTextActionTakesOneRungPerState/light
    actionrung_test.go:71: the hovered ink is {77 77 79 255}, 50 levels off the Text token {27 27 29 255}: the ink lift is being faded back — one state, one knob
    actionrung_test.go:77: the pressed ink is {164 164 167 255}, 138 levels off the Text token {27 27 29 255}: press must not step back down the ink ladder
    actionrung_test.go:82: the pressed ink stands 20009 levels off its plane {246 246 248 255} and the resting ink only 55494 off {246 246 248 255}: the commit frame is the quietest of the three
--- FAIL: TestTheTextActionTakesOneRungPerState/dark
    actionrung_test.go:71: the hovered ink is {186 186 188 255}, 51 levels off the Text token {237 237 239 255}
    actionrung_test.go:77: the pressed ink is {97 97 102 255}, 140 levels off the Text token {237 237 239 255}
    actionrung_test.go:82: the pressed ink stands 20673 levels off its plane {15 15 17 255} and the resting ink only 57419 off {15 15 17 255}
```

（那时 pin 的 ③ 还用平方和报「levels」，红串里那 20009 / 55494 是平方和；定稿的 pin 改成
逐通道最大差，见下表。）

三档墨，浅色页 `{246,246,248}` / 深色页 `{15,15,17}`：

| 外观 | 状态 | 改前墨 | 改前离面 | 改后墨 | 改后离面 | 改后 rung |
|---|---|---|---|---|---|---|
| 浅 | 静息 | `{109,109,114}` | **137** | `{109,109,114}` | **137** | 0.00 |
| 浅 | hover | `{77,77,79}` | 169 | `{30,30,32}` | **216** | 0.96 |
| 浅 | press | `{164,164,167}` | **86** | `{40,40,42}` | **206** | 0.84 |
| 深 | 静息 | `{152,152,158}` | **141** | `{152,152,158}` | **141** | 0.00 |
| 深 | hover | `{186,186,188}` | 171 | `{234,234,236}` | **219** | 0.96 |
| 深 | press | `{97,97,102}` | **92** | `{224,224,226}` | **209** | 0.84 |

读出来的三件事：① 改前 **press 离面 86 / 92，低于静息的 137 / 141**——按下比不按还淡；
② 改前 hover 只走到两档之间的 **0.40 / 0.39**，抬墨被自己的不透明度吃掉一半；③ 改后
hover / press 都在 `Text` 那一档上（0.96 / 0.84；press 差的 0.16 是 0.986 缩放的抗锯齿损失，
字形没有任何一个像素被完整覆盖——不是墨阶退了一档）。

## 2. 改法（只改一条动作的两个状态臂）

`internal/settingsui/rows.go` `actionWithGlyph`：

```
-	switch {
-	case row.Pressed():
-		row.Opacity(0.6) // `.settings-reset:active { opacity: .6 }`
-	case row.Hovered():
-		row.Opacity(0.78) // `.settings-reset:not(:disabled):hover { opacity: .78 }`
-		ink = t.Text
-	}
+	switch {
+	case row.Pressed():
+		ink = t.Text
+		row.Scale(0.986, 0.986)
+	case row.Hovered():
+		ink = t.Text
+	}
```

- **为什么删的是两个 `Opacity`**：参考 CSS 只有一个旋钮（`opacity`），P7-gk 抄了数，P8-d 又
  加了「抬墨」；两个旋钮同时留在源码里，抬墨就被自己的不透明度按回去（§6.59 判据 1）。
- **为什么 press 的第三档是几何不是墨**：墨阶梯到 `Text` 就到底了（`TextStrong` 与 `Text`
  在本仓是同一个值：`#1B1B1D` / `#EDEDEF`），再往下没有一格；而这条动作没有面
  （P7-gk 专门删掉了底带），所以第三档只能是 §5 的 `Scale(0.986)`——与启动器结果行
  `launcher.go:2511` 同一句话，也与 `sidebarRow` 的「第三档换的不是墨」同一句
  （`settingsui.go:1419`）。**布局盒不动**（`Scale` 是渲染变换），命中框、盒子度量、
  既有 `actionmetrics_test.go` 的三条几何断言零改动。
- **`Focusable()` 没动**：键盘模型是红线。

## 3. 台账

**V 1 / B 0 / S 0**

- **V 1**：`internal/settingsui/raycast_test.go` `TestTheResetActionIsNeutralAtRestAndOnHover`
  原本钉 `panelOver(tokens.Theme.Text.Alpha(0.78), page)`（hover 的墨 = primary 档再乘 0.78）。
  注释写明 `P13-cq 重定基线（V，旧值 → 新值）`：旧 `{77,77,79}` / `{189,189,191}` →
  新 `{28,28,30}` / `{236,236,238}`（那一档墨本身）。同一函数里 `page := subPage(dark)` 随之
  变成死变量，删掉。
- **B 0**：`actionmetrics_test.go` 三条（24u 盒 / 2u 内边距 / 底部对齐）、
  `TestTheTextActionHoverLiftsTheInkOnly`（hover 抬墨、不画面、不带色相）全部**零改动**通过。
- **S 0**：结构没变（同一个函数、同一个 `switch`）。

## 4. 门槛（实测）

```
gofmt -l cmd internal                     空
go vet ./...                              净
GOOS=windows go vet ./...                 净
GOOS=darwin  go vet ./...                 净
go test -count=1 ./...                    全绿（0 FAIL）
PASS 3283（基线 3280）· FAIL 0 · SKIP 135（基线 134，新增一条 opt-in 出图）
测试函数 2430（基线 2428，+2：一条 pin 一条 dump）
GOOS=linux/windows/darwin go build ./...   全过
```

## 5. 变异（三条实跑，全部变红并逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| M1 | hover 臂把 `row.Opacity(0.78)` 放回去 | 红：`the hovered ink {77 77 79} sits at 0.40 of the way from TextSecondary {107 107 112} to Text {27 27 29}`（深 0.39） |
| M2 | press 臂墨退回 `TextSecondary`（保留 `Scale`） | 两红：`the pressed ink {115 115 120} sits at -0.10 …`、`the pressed ink stands 131 levels off its plane … and the resting ink only 137` |
| M3 | 删掉 `row.Scale(0.986, 0.986)` | 红：`the pressed frame is pixel-identical to the hovered one: holding the action down says nothing`（浅 `{30,30,32}` / `{30,30,32}`，深 `{234,234,236}` / `{234,234,236}`） |

每条之后 `cp` 回原文件 + `md5sum -c`（`c6540ca0…` / `92209e8a…`）逐字节核对，再 `touch` 复跑绿。

## 6. 出图（深 / 浅 × 三档）

`/tmp/floter-P13-cq-textaction-<light|dark>-<rest|hover|press>-<before|after>.png`（720×800，
真设置页的 Shortcuts 页）。改前/改后逐像素差分：

| 外观 | 状态 | 差分 px | bbox |
|---|---|---|---|
| 浅 | rest | **0** | —（静息一帧一个字节都没动） |
| 浅 | hover | 275 | `(619,136)-(700,145)` |
| 浅 | press | 425 | `(619,136)-(700,145)` |
| 深 | rest | **0** | — |
| 深 | hover | 274 | `(619,136)-(700,145)` |
| 深 | press | 423 | `(619,136)-(700,145)` |

**眼睛复核的一处**：`...-light-press-after.png` 上 `Restore defaults` 这行字比 `-light-press-before.png`
**重了整整一档**（改前它比同一行左边的 `Reset to defaults` 提示还淡，改后与 hover 帧几乎同墨，
只是整串字往里收了不到 1px）——按下那一刻，动作是「沉下去」，不是「淡出去」。

## 7. 偷了什么（qiaomu-design / craft-loop.md）

本轮读了 `SKILL.md` → `references/user-preferences.md` → `references/craft-loop.md` →
`references/preflight.md`（`apple-fluid-interfaces.md` / `motion-craft.md` 没读：本轮没有手势、
没有动效改动，读它们是凑数）。逐条落成：

1. **craft-loop §一（DNA 注入）**：供体不是外部站点，而是**参考实现自己的 CSS**
   （`settings.css:682-730` 的 `.settings-reset`）。偷的不是数值，是它的**纪律**：
   「一个状态只改一个属性」。落成：hover 只剩 `ink = t.Text`（删 `.78`），press 只剩
   `Scale(0.986)`（删 `.6`）。
2. **工艺密度菜单 #4（签名交互）**：这个动作的第三档从「不透明度」换成「下沉」
   ——`Scale(0.986)`，实测 press↔hover 的差分 **437 px**，全部落在 `81×9` 的字带里
   （`{619,136,81,9}`），没有一个像素跑到动作盒子外面。
3. **菜单 #6（编辑细节：hairline / 字距）与 #12（材质细节：内边框高光）故意不做**：
   这条动作没有面，§1 禁令 3 明说「透明面上的白线就是类玻璃边」——给它补一条受光边正是
   用户点名的那个毛病。**记进「故意不做」，不是漏掉。**
4. **craft-loop §三（两轮制）**：第一轮改完读三档（表 §1），第二轮只修一处——
   pin 的 ③ 从「平方和」换成「逐通道最大差」，因为第一轮的红串把 55494 读成了「levels」，
   那是量纲错，不是产品错。

## 8. preflight 逐条

- **A（反 AI 套路）**：过。无渐变字/紫蓝光晕/纯黑底（深色是 `#0E0E10`）、无斜体、无
  emoji 图标（lucide 字形）、无三等分卡片、无左侧竖线装饰（P-38）、文案无空词。
  **不适用两条并写明**：① 「字体不是 Inter/Roboto/系统默认」——本仓字体栈是
  `docs/raycast-design.md` §4 钉死的行为契约，换字体是另一个切片；② 眉批/Hero 规则属于
  网页，本仓是桌面自绘窗口。
- **A2.1（K3 门）**：过（不适用）——当前任务没有点名 K3，按 P-46 由本代理直接执行。
- **A2.2（创意提示词门）**：不适用（不是开放方向）。
- **B（设计读取一致性）**：过。一页一个主题；本轮改动只在这条动作的三档，与 §5 的状态阶梯
  同一句话；记忆点一句话：**「按下去，动作沉下去；不是淡出去。」**
- **C（动效抽查）**：过（部分不适用）。按压有 `:active` 反馈（`Scale(0.986)`）；无
  `transition: all`；无 `scale(0)` 入场；无 `ease-in`。**发现一处 gap 并记进「仍待」**：
  这条动作**没有任何颜色过渡**（`actionWithGlyph` 建行时没传 `Transition{Colors: true,
  Duration: theme.ColorStep}`，而 `sidebarRow` / `cardRow` / `headerButton` 都有）——hover
  的墨是硬切。补它要连带改两条既有 pin 的取帧时机，属另一片。
- **D2（中文页）**：过（不适用）。本轮没有改任何字号/行高/字重/i18n 文案；页面文案仍是
  英文（`i18n.For("en")` 那套），`tokens.Caption` 11px 与「正文 ≥ 14px」的冲突仍是
  **需拍板**项（第 18 份证据）。
- **E（三秒测试）**：过。这一页视线里仍然只有一个焦点（当前分组的卡），本轮没有新增
  任何争夺注意力的元素——删掉的是两个旋钮，不是加了一个。

## 9. 仍待（P13-cq）

1. **下一格（不需拍板，先量）**：`actionWithGlyph` 的 hover 墨是**硬切**，没有
   `Transition{Colors: true, Duration: theme.ColorStep}`（§5 的 120ms）。同一页的
   `sidebarRow` / `cardRow` / `headerButton` 都有。补它要连带改 `actionrung_test.go` 与
   `TestTheTextActionHoverLiftsTheInkOnly` 的取帧时机（先 `Advance` 再读），先量再定。
2. **需拍板（沿用 + 第 18 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：扩展页空态只有一行结论、没有出路（`settingsui.go:1838`）；
   会话页空态说错了**对象**（「持久化会话」vs 进程内会话）；终端页会话抽屉 / 标签对失败
   只加字不换墨；终端外观抽屉行要不要面；`launcher/filters.go` 过滤 chip 与
   `launcher/power.go` 确认带 chip 的按下档；`ui/tooltip.go:133` 禁用元素不弹提示；
   `cardRow` 控制槽 `Shrink(0)`；多选箱宽度；路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **需拍板（第 4 份证据，§6.32-4）**：同一张卡上的两种「这一层的名字」。
5. **另一片（不碰）**：`uninstall.go:442` 的 `card.Opacity(0.6)`（§6.58 尾注留下的那一格）。
6. **抢救 stash 核对（第十九次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
