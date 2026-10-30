# P13-h 报告 · 启动器的小面：28u 图标板与两处悬停小格从「四层环」退成 E1 的一对面

规格 `docs/rounds/P13-h-spec.md`。改动落在 **3 个生产 draw point + 1 张 census + 1 个新 pin 文件**。

## 1. 选题（为什么是这一条）

`docs/liquid-glass.md` §6 的表在 P13-g 之后**没有 `未开始` 行**，所以按任务书的优先级转回
「细节打磨通用队列」。队列里现成的第一条来自 P13-f「仍待」第 3 条与 P13-g「仍待」第 1 条：
`iconPlate` 的四层 `Lens`（第四层是四边 `ControlEdge` 的环）是启动器**所有页共用**的板，
收它属于「启动器表面那一轮」——就是这一轮。

它不是「再美化一点」，而是**升代留下的一处自相矛盾**：

- `internal/launcher/glyphs.go` 的注释原本写着「Raycast's plate is flat: the control lens is
  capped at 0.04 … no longer reads as a lit pane of glass」；
- P12-a 把 `LensRim` 从 `0.04` 抬到 `0.09`、`LensEdge` 抬到 `0.06`（§1 的重定基线表），
  **注释当场变成假话**，而代码还是 `tokens.Lens(plate)`；
- 这块板在启动器的每一行都出现（结果行 / 剪贴板 / 计算器 / 浏览器 / 文件页 / 捕获输出页），
  是全卡**重复次数最多**的元素。一块每行都戴四边环的小方块，正是用户说的
  「偏 macOS 旧版本」那种工具条按钮的读法。

**这轮为什么让它更轻 / 更有空间感（一句话）**：这块板此前**根本没有受光边**——
`Lens` 的第四层（`ControlEdge` 环）是**最后画上去的**，它压在第一条线（`LensRim`）之上，
于是同一行 1px 里叠了两条线：深色档是 0.09 白 + 0.07 白（读到 97，比面亮 32 级，是「一圈白框」），
浅色档是 0.09 白 **再被 0.06 黑压掉**（读到 212，**比面 221 还暗 9 级**——浅色下这块板的「顶边」
是一条暗线）。退掉环之后它第一次真的是 E1：顶 224（亮 3 级）、底 208（暗 13 级）、两侧回到面本身。

## 2. 落地（逐条）

| # | 位置 | 改成 |
|---|---|---|
| H-1 | `internal/launcher/glyphs.go:296`（`iconPlate` 的尾巴） | `tokens.Lens(plate)` → `tokens.LitFace(plate)`；注释重写成 E1 的一对面，并写明 P12-a 把旧注释变成了假话 |
| H-2 | `internal/launcher/launcher.go:1838`（`rowControl` 的 `Hovered()` 臂） | `tokens.Lens(...)` → `tokens.LitFace(...)`（20u 悬停小格） |
| H-3 | `internal/launcher/launcher.go:3373`（`fieldSettingsButton` 的 `Hovered()` 臂） | 同上（28u 悬停小格） |
| H-4 | `internal/shell/glasscensus_test.go`（`.launcher-result__icon` / `.launcher-action-bar__icon`） | 两条从「`tokens.Lens(plate)` 必须还在」翻成 `dropped`，`test:` 指向本轮的新 pin（P13-d 的既有机制，不是新豁免） |
| H-5 | 新增 `internal/launcher/plateface_test.go` | 3 条 pin（板 / 字段行悬停格 / 20u 悬停格）+ 1 个 opt-in dump，全部深/浅两套外观 |

**几何零改动**：28u 的盒、`RadiusXS`、行内 padding、图标列 28u、文字列 54u、高度链、
悬停面阶 `ControlHover`、按下 `ControlPress` + `Scale(0.92)`、浏览器行的裸格——一个都没动。

## 3. 像素证据（实测，同一帧同一列）

`x=30` 穿过图标板中心，第一块板 y=101..128；面 = 板内像素。**前 = HEAD 代码 + 本轮 dump，
后 = 本轮代码**，同一 fixture、同一尺寸 720×488。

| 读数 | 浅色 前 → 后 | 深色 前 → 后 |
|---|---|---|
| 面（板内） | 221 → 221 | 65 → 65 |
| 顶内线（`y=101`） | **212（比面暗 9）** → **224（比面亮 3）** | **97** → **82** |
| 底内线（`y=128`） | 196 → 208 | 75 → **61（比面暗 4）** |
| 左列（`x=16`，行中高） | 225 → 231（环的暗内缘消失） | 68 → 61 |
| 右侧列（`x=42..43`） | 216 → 227 | 79 → 68 |

**算术对得上，环确实是最后画的那一层**（`InsetShadow` 的层序：先加的在下面）：
浅色面 221 → 叠 0.09 白 = 224 → 再叠 0.06 黑 = **210.5 ≈ 212**（前）；
深色面 65 → 叠 0.09 白 = 82 → 再叠 0.07 白 = **94 ≈ 97**（前）。后 = 只剩第一条，即 224 / 82。
**浅色档的「顶边」此前是一条暗线**，这是这轮最值得记的一条：P12-a 把受光边抬到 0.09
之后，这块板一次都没真正亮过。

整页差异：浅色 **380 px** / 深色 **970 px**，bbox 两边都是 `(16,101)-(44,221)`
——正好是四块 28×28 图标板那一列，行盒、文字列、选中块的两条线、键帽一个像素没动。

## 4. V / B / S 台账

- **V（视觉数值）2 条**：`internal/shell/glasscensus_test.go` 的 `.launcher-result__icon` 与
  `.launcher-action-bar__icon`。旧断言钉的是「四层透镜那个调用必须还在」，按 §2 的判据翻成
  `dropped`（「那个调用不许回来」），新面指给 `launcher.TestTheIconPlateIsAFaceNotARing`。
  与 P13-d 的 `.toolbar-button:hover` 同一种处置、同一张 census。
- **B 0 条**：其余 2140 条测试函数**零改动**全绿——`TestTheResultPlateCarriesTheStopLens`
  （板随玻璃档升降）**原样通过**：`LitFace` 的两条线正是 `--glass-lens-scale` 分级的那一族，
  退掉宽 sheen 与环之后 regular→liquid 的差值仍在它自己的可见门槛之上。
- **S 0 条**。

## 5. 门槛（全部实测）

```
gofmt -l cmd internal                 → 空
go vet ./...                          → 净
GOOS=windows go vet ./...             → 净
GOOS=darwin  go vet ./...             → 净
go test -count=1 ./...                → 全绿
PASS 2907（基线 2898，+9）| FAIL 0 | SKIP 66（基线 65，+1 dump）| 函数 2148（基线 2144，+4）
GOOS=linux/windows/darwin go build ./... → 过
```

`+9` = 三条 pin × 深/浅两个子测试 + 三条父测试（dump 在常规跑里是 SKIP）。

## 6. 变异（7 条，全部实跑变红并逐字节还原）

`md5sum` 基线：`glyphs.go d8cb0e38…`、`launcher.go 190023a1…`；每条还原后 `md5sum -c` 成功、
`touch` 后复跑绿。

| # | 注入 | 红在哪（原始串节选） |
|---|---|---|
| M1 | `iconPlate` 换回 `tokens.Lens(plate)` | `the plate's left column at mid height is {216 216 216 255} but its face is {229 229 230 255} — a 14-level ring is drawn around the whole box` |
| M2 | 只留顶边（`InsetShadow(0,1,0,0,LensRim)`） | `the plate's bottom inner line {229 229 230 255} is not darker than its own face` |
| M3 | 给板加 `Shadow(0,1u,4u,0,ShadowAmbient)` | `the plate paints left of its own 28u box ({223 223 225 255} against the ground {246 246 248 255})`（六条边全红） |
| M4 | `fieldSettingsButton` 悬停换回 `Lens` | `the hovered control's left column at mid height is {213 213 214 255} but its face is {226 226 227 255}` |
| M4b | `rowControl` 悬停换回 `Lens` | `the hovered 20u control's left column at mid height is {207 207 209 255} but its face is {220 220 222 255}` |
| M5 | 板彻底不带线（`return plate`） | `the plate's top inner line {229 229 230 255} is not brighter than its own face` |
| M6 | 板的冠线换 E2 的 `EdgeHighlight` | `the plate's crown {253 253 253 255} is nearer the floating EdgeHighlight ({253 253 253 255}) than the component LensRim ({231 231 232 255})` |

**M3 抓到我第一版 pin 的一个真缺陷**：原来的盒是「从中心向外扫」出来的，一加影，扫出来的盒
**跟着影一起长大**，于是「影不许溢出」这条永远成立。现在盒取自主题自己的图标列
（`Space(7)` = 28u），影只能落在盒外那几个像素上——M3 就是这么红的。

## 7. 出图

`/tmp/floter-P13-h-launcher-{before,after}-{light,dark}.png`（720×488）。
**这张图上看到的一处变化（浅色 before）**：每行的图标板四周有一圈**暗框**，尤其顶边
比板面还暗，读起来像 2010 年代的工具条按钮；after 里那圈框没了，板上只剩顶边一条几乎
看不见的亮线 + 底边一条淡影——板从「被框住的方块」变成「贴着卡的一片」。

## 8. 偷了什么（qiaomu-design，走「已有方向，只要打磨」车道）

读了 `SKILL.md` → `references/user-preferences.md`（P-1 禁斜体 / P-5 可读性优先 / P-2 先看后选
——用户已把方向给死，走自动车道，不出预览页）→ `references/craft-loop.md` +
`references/preflight.md` → `references/apple-fluid-interfaces.md` §8「透明材质与深度」
（半透明只给浮动功能层）。DNA 供体仍是 `design-systems/linear.app/DESIGN.md`（§6 Depth & Elevation）。

1. **craft-loop 工艺密度菜单第 12 项「材质细节：内边框高光」** → 落成板顶那一行
   `InsetShadow(0,1,0,0, LensRim)`：**1px 实色线**，不是渐变（§0 硬禁令 3）。
2. **菜单第 6 项「编辑细节：hairline」** → 受光边与对侧暗线都锁在 1px（`InsetShadow` 的
   offset 1 / -1、blur 0），整块板只有两行像素参与，其余 26 行是面本身。
3. **菜单第 5 项「多层染色阴影」的反面用法** → 本轮**减**掉一层：`Lens` 的四层里退掉宽 sheen
   （`0,6,10,-6`）与四边环，只留「顶亮 / 底暗」两层。Linear 的判据是
   「elevation 由面阶与光说话，不由投影说话」，落到本仓就是 §2 的 E1 判据 2「E1 不许有外投影」。
4. **从 apple-fluid-interfaces §8 偷的判据**（不是菜单项）：「只在**浮动功能层**使用半透明材质」
   ——图标板是长在卡上的一个面，不是浮动层，所以它不该有玻璃栈；这条同时解释了为什么
   悬停小格也不该戴四层透镜。
5. **没偷的两项（如实说明）**：菜单第 1 项「氛围层」与第 9 项「自定义滚动条」在本仓没有对应物
   （Go 自绘没有 DOM / 滚动条 CSS），硬造等于给 §0 的「不许假装有 blur」开后门。

## 9. preflight 逐条结论

- **A 反 AI 套路**：过。无渐变白边（冠线是 1px 实色）、无外发光、无玻璃叠玻璃（板内是面，
  `LitFace` 只加两条内线）、无斜体、本轮没有文案进出。
- **A2 进攻三步**：DNA 注入 ✓（Linear + apple-fluid-interfaces 共 4 条，见 §8）；
  工艺密度 3 项落地 + 2 项写明不适用；两轮制 ✓——第一轮改完出图后，用 before/after 四张图做
  两视角复核（艺术总监：浅色 before 那圈**暗框**比「亮框」更刺眼，顺藤查到环画在最后；
  工程师：pin 的盒是扫出来的，影会让它长大 → 第二轮只改这两处，未推倒重做）。
- **B 设计读取一致性**：过。本轮的设计读取是「桌面命令面板里的一个小图标位，深/浅双外观，
  轻玻璃气质，E1 面」；产出只动了 E1 那一档，E2（toast / 抽屉 / 弹层）一个像素没碰。
- **C 动效抽查**：过（且未改）。悬停仍是 120ms 颜色过渡 + `ControlPress` 的 `Scale(0.92)`；
  本轮没有新增动画，也没有动 `width/height/margin`。
- **D2 中文页**：不适用——本轮零文案、零 i18n 字符串改动（界面文案是英文 i18n 表里的既有条目）。
- **E 三秒测试**：过。眯眼看 after：视线落在行标题上，不再被每行四个小方框抢走。

## 10. 仍待

1. **P13-c 的四条原样有效**：S1b（`CloseAll` 改 detach，等拍板）、S1c（Windows ConPTY）、
   客户端不告而别的检测、capture 通道迁移。
2. **`t.Space()` 被当成 px 用的同源族**（P13-g 观察）：本轮**没有**扫全仓，仍是独立一轮的候选。
3. **本轮观察（新）**：`Lens` 还剩 **12 个调用点**（`settingsui` 10 处：`parameditor` 1 /
   `rows` 2 / `savealert` 1 / `settingsui` 2 / `uninstall` 4；`shell/notices.go` 的 toast 动作 1；
   `control.SegmentedIndex` 的抬起槽 1）。这些是「控件」而不是「面」，是否也退成 `LitFace`
   需按 §2 逐个判——不是一轮能扫完的，且其中开关/分段槽是**凹槽**（`LensInset`），不能混。
   留给下一轮按表面走（下一站：`settingsui`）。
4. **本轮观察（新，需拍板）**：`iconPlate` 的**暖面**（电源动作的 `IconSurface`）现在也戴
   `LitFace`。暖面上一条 0.09 的白线在深色档几乎读不出来（色相吃掉亮度差）——这与 P12-c 的
   2-6（主 CTA 胶囊的对侧暗线）是同一类问题，要不要给暖面单独一条线，需拍板。
