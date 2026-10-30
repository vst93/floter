# P13-u 报告 · 设置页剩下的四处两道淡：一道淡收尾

方向一句话：**同一张设置页上，「不可用」只有一个深度。**

## 这轮为什么让它更轻 / 更有空间感

P13-s / P13-t 把「禁用只有一个深度」这条规矩写进了 `docs/liquid-glass.md` §6.2b-6/-6b，
但同一页上还留着四处**产品自己那道 `Opacity`** 叠在框架给 `Disabled` 子树的 0.5 之上：

| # | 控件 | 改前有效淡度 | 算式 | 实测（浅色地面 247 / 深色 14） |
|---|---|---|---|---|
| 1 | `disabledSwitch`（开关） | **0.30** | `.Opacity(0.6)` × 框架 0.5 | 轨道面 (174,194,223) / (53,65,88) |
| 2 | `sessionIconButton` 禁用臂 | **0.20** | `.Opacity(0.4)` × 框架 0.5 | 图标墨只剩 **24 级**对比（静息 117） |
| 3 | `uninstallActionButton` 在飞臂 | **0.20** | `.Opacity(0.4)` × 框架 0.5 | 面 243（**−4**）/ 19（+5），底对侧 −3 / −1 |
| 4 | `barButton` 禁用臂 | **0.20** | `.Opacity(0.4)` × 框架 0.5 | 面 243（**−4**）/ 19（+5），底对侧 **0** / **0** |

这不是四种状态，是**四种深度**描述同一件事。同一页上禁用胶囊已经站在 −9 级（P13-s），
禁用开关却站在 −4 级、禁用会话小格干脆快没了——用户读到的「平、糊、没有空间感」，
在状态反馈这一侧就是这条算术：**面离地面的台阶没有统一的一档**。

改后四处落在同一条算术上（一道淡 = 静息与地面各一半）：

| 控件 | 浅色 改前 → 改后 | 深色 改前 → 改后 |
|---|---|---|
| 开关轨道面 | (174,194,223) → **(126,160,207)**（一道淡的目标 (127,161,207)） | (53,65,88) → **(79,99,136)**（目标 (79,99,136)） |
| 会话小格墨对比 | 71 → **177**（静息 351，一半 = 176） | 73 → **183**（静息 363，一半 = 182） |
| 在飞胶囊面 / 底 | 243（−4）/ 240（−3） → **238（−9）/ 231（−7）** | 19（+5）/ 18（−1） → **26（+12）/ 23（−3）** |
| 确认带胶囊面 / 底 | 243（−4）/ 243（**0**） → **238（−9）/ 231（−7）** | 19（+5）/ 19（**0**） → **26（+12）/ 23（−3）** |

## 设计读取（qiaomu-design，打磨模式）

> **读取为：桌面应用设置页里的一排「现在不能碰」的控件（开关 / 会话小格 / 在飞按钮 /
> 确认带），面向会改设置的普通用户，用「轻便液态玻璃」语言，倾向自绘材质 + 明确
> elevation 阶梯（E0/E1/E2）这一族。**

| 拨盘 | 值 | 人话 |
|---|---|---|
| `VARIANCE` | 2 | 不新增形状、不新增 token、不新增 helper，只把四处的墨收进同一条算术 |
| `MOTION` | 0 | 不动效（四处原有的 120ms 颜色过渡原样） |
| `DENSITY` | 6 | 设置页本来就密，本轮只补「一块禁用面该有的算术」 |

功能契约：用户要能一眼看出「这个控件现在不能碰」，同时**还要读得出它是什么**
（改前会话小格的图标墨只有 24 级对比）。走「已有方向，只要打磨」那条路径
（`craft-loop.md` + `preflight.md`），**没有**出四方向预览页、**没有**等点选。

## 偷了什么（工艺密度菜单）

- 从 `craft-loop.md` **第 12 项「材质细节」**（内边框高光 / glass 折射边）：落成
  「**受光边是「这块面可以被碰」的记号，禁用面把它熄掉、底对侧留着**」——两处胶囊
  （在飞 / 确认带）的材质从 `LitFace` 换 `DimFace`，深色档顶内线由 **23（面 19，+4 = 一条亮线）**
  变成 **26（面 26，= 面）**，底内线 18（−1）→ **23（−3）**。
- 从 **第 5 项「多层染色阴影」**：落成「**对侧不许一起熄**」——确认带胶囊改前的底内线
  与面**逐像素相同**（243 = 243，深色 19 = 19）：它不是「熄了灯的面」，是**一块平涂**。
  改后底内线 231（−7）/ 23（−3），与 P13-s 的胶囊同一对线。
- 从 **第 6 项「编辑细节 / hairline」**：落成「**一道淡，不是两道**」这条算术——
  0.4 × 0.5 = 0.2、0.6 × 0.5 = 0.3 都不是设计，是同一道淡被画了两次；落地值全部是
  框架那一道 0.5，槽（开关）与透明面（会话小格）**不叠材质**，面（两处胶囊）叠 `DimFace`。
- 没有取第 1/2/3/4/7/8/9/10/11 项（氛围层 / 选中色 / focus 环 / 签名交互 / 展示排版 /
  流体字号 / 滚动条 / 彩蛋 / 图形资产）：这是一排状态反馈，加任何一项都是噪点。

## 落地项

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `internal/settingsui/rows.go` `disabledSwitch` | `Disabled(true).Opacity(0.6).Cursor(default)` | `Disabled(true).Cursor(default)`——槽没有受光边可熄，整块退一半、内线各留一半（§6.2b-6b 的规矩）。`busySwitchControl` 走同一 helper，自动跟随 |
| 2 | `rows.go` `sessionIconButton` 的 `disabled` 臂 | `Opacity(0.4).Cursor(default).Disabled(true)` | `button = lockedControl(button)`——**透明面**没有面也没有边，只淡一道（不叠 `DimFace`） |
| 3 | `internal/settingsui/uninstall.go` `uninstallActionButton` 的 `busy` 臂 | `Disabled(true).Opacity(0.4).Cursor(default)`；材质恒 `LitFace` | `Disabled(true)` + 默认光标；材质按 `busy` 分支 `DimFace` / `LitFace` |
| 4 | `internal/settingsui/settingsui.go` `barButton` 的 `disabled` 臂 | `Disabled(true).Opacity(0.4).Cursor(default)`；材质恒 `LitFace` | 同上 |
| 5 | `config.go` `lockedControl` 的 doc | 只说配置表单 | 补一句：**面**再叠 `DimFace`，**槽**与**透明面**什么都不叠 |

**没有新增 token / helper**：`lockedControl`（P13-t）与 `theme.Tokens.DimFace`（P13-s）已经
就是这条规矩的两种读法；本轮只是把剩下的调用点接上去。

## 行为契约零改动红线

- 四处只改**墨与面**：`Disabled(true)` 的去留、点击守卫（`Clicked() && !disabled` /
  `&& !busy`）、光标、`busy` / `disabled` 参数的判定链一个字节不动。
- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：零改动。
- 滚动不受影响（`ui/input.go:515-536` 不查 `flagDisabled`）。
- `uninstall.go:427` 的 `card.Opacity(0.6)`（`part.Fixed` 的不可勾选卡片）**没动**：那一臂
  没有挂 `Disabled`，0.6 是**唯一**一道淡，且戴着完整的一对线——不是两道淡，不在本族里。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...    → 净
go test -count=1 ./...                                 → 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2981（基线 2977，+4）
                            ... | grep -c -- "--- FAIL" → 0
                            ... | grep -c -- "--- SKIP" → 78（基线 77，+1 = 本轮 dump）
测试函数数（`git grep -h "^func Test" -- '*_test.go' | wc -l`）
                                                       → 2195 → 2200（+5 = 4 pin + 1 dump）
GOOS=linux/windows/… go build ./...                    → 过
```

V 0 / B 0 / S 0：没有一条旧 pin 变红，旧测试一个字节没动。

## 变异（5 条，全部实跑变红并逐字节还原）

| # | 注入 | 红在哪（原始串） |
|---|---|---|
| M1 | `disabledSwitch` 加回 `.Opacity(0.6)` | `the settings switch (light): the locked face is {174 194 223 …}, but one fade of {6 74 166 …} over the ground {247 247 248 …} is {127 161 207 …} (off by 32 levels)` |
| M2 | `sessionIconButton` 禁用臂加回 `.Opacity(0.4)` | `the disabled session icon (light): the glyph is 23 levels off the ground, but one fade of the enabled 117 is 58 (off by 34)` |
| M3 | 在飞胶囊的材质分支退回恒 `LitFace` | `the in-flight uninstall pill (dark): the disabled pill kept a lit edge: top inner row {36 36 38 …} is 10 levels over the face {26 26 28 …}` |
| M4 | `barButton` 的 `DimFace` 换成 `LitFace` | `the discard band's disabled pill (dark): the disabled pill kept a lit edge: top inner row {36 36 38 …} is 10 levels over the face {26 26 28 …}` |
| M5（反向） | 给 `sessionIconButton` 的**透明面**补一条 `LitFace` | `the disabled session icon (dark): it wears a lit edge {25 25 27 …} over its transparent face (11 levels)` |

还原核对：`md5sum -c` 三份都对（`rows.go 8faf202f…`、`uninstall.go 5b4e821c…`、
`settingsui.go 6ca32b60…`），`touch` 后复跑一次仍绿。

**M5 的两个如实记录**：①抓它的是**本轮新增的 pin**，不是 P13-p 的 census 反向 pin——
那条 census 判的是「有面 + 有边却缺一对线」的函数（`faceMaterial` 只认
`Background(tokens.Control…)` 一族），**透明面根本不在它的账本里**；②浅色档**没红**
（`LensRim 0.09` 白压在 247 的地面上 = 248，+1 级，阈值 2）——浅色档的「受光边」在
透明面上本来就看不见，所以这条守卫只在深色档有效，pin 里也写明了这一点。

## 出图（自己复核过）

`/tmp/floter-P13-u-onefade-{light,dark}-{before,after}.png`（before 由
`FLOTER_PNG_TAG=before` 把四处旧值注回去实拍）。版面是「静息 / 禁用」成对的四组：
开关、会话小格、卸载胶囊、确认带胶囊。

- 差异 bbox：浅色 `(24,52)-(335,277)` 15749 px、深色同 bbox 15714 px——**四组里的
  四枚禁用枚全部在框内**，而四枚静息枚（第一、三、五、七行）一个像素没动
  （`(200,34)`、`(200,160)` 前后都是 247/230）。
- 这张图上看到的一处变化：确认带那枚禁用胶囊的**底边**（浅色 `(200,277)`）
  由 `(241,241,241)` → `(231,231,232)`，而它的面 `(200,260)` 由 `(243,243,244)` →
  `(238,238,239)`——改前面与底**几乎同色**（只差 2 级），改后第一次有 7 级的对侧线：
  那块胶囊从「一块平涂」变成「一块压在地面上的面」。深色档同一处 19 / 19 → 26 / 23。

## preflight（`references/preflight.md`）

- **A 反 AI 套路**：通过。没有渐变、没有 `#000000`、没有外发光、没有 italic、没有
  emoji 图标；本轮不新增文案。§1 三条硬禁令全部满足：没有 blur 冒充玻璃、没有
  glass 叠 glass（只改材质与透明度，没有叠层）、没有渐变白边。
- **B 设计读取一致性**：通过。四处禁用面现在走**两个**函数（`lockedControl` /
  `theme.Tokens.DimFace`），深浅两套外观同一套规则（数值差异来自地面与框架的 0.5，
  不是两套设计）。一句话记忆点：**禁用 = 一道淡；面熄灯，槽退一半，透明面只剩一半的墨**。
- **C 动效抽查**：通过（本轮不动效）。两处胶囊的 `Colors: true, 120ms` 原样；禁用没有
  入场动画；`ReduceMotion` 在测试渲染里开着，读数不受过渡影响。
- **D2 中文页**：通过。本轮没有新增中文 UI 文案（只有代码注释与文档）。
- 未验证项：真机（非测试渲染器）观感未拍；Windows / darwin 只做了 vet + build，没有出图
  （像素 pin 走同一套渲染器，三平台同码）。

## 仍待（P13-u，追加在 P13-t 之后）

1. **accent 主按钮的禁用臂仍是 `.Opacity(0.4)` / `.Opacity(0.6)`**
   （`config.go:341` 的 Save、`settingsui.go:3835` 的 `updateButton`）：accent 要淡的是
   **色相**（fill 本身），不是「面还在」——先定「禁用 accent = accent 与卡面按什么比例混」，
   另起一轮（§6.2b 9-25 一直挂着）。
2. **`rows.go:895` `sessionKillConfirm`（警告色胶囊）仍是 `.Opacity(0.4)`**：它戴
   `RaisedWarm` + `ShadowContact` + `LensRim`，`DimFace` 会把警告环洗淡（§6.2b 9-26 已判
   `故意不做`）。要做先拍板「警告面的禁用是熄灯还是换色」。
3. **透明面上的受光边只有深色档有像素守卫**（本轮 M5）：浅色档 +1 级，看不见也测不到。
   要不要给 census 补一条「透明面 ⇒ 不许有 `LitFace`/`Lens`」的源码级反向断言，
   需拍板（会新增一条跨包规矩，不只是改一个数值）。
4. P13-t 第 3 条（锁定列表框选项文字的 61 级对比）、P13-s 第 1/2 条、P13-r 第 1 条、
   P13-q 第 1 条、P13-p 第 1/2 条、P13-k 第 1/2 条、P13-c 四条、P13-h 第 9 条原样有效
   （均需拍板）。
