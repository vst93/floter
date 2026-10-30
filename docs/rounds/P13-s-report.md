# P13-s 报告 · 禁用面：面还在，光熄了

方向一句话：**禁用态不是「把整棵子树调淡」，是「面留在那儿，灯熄了」。**

## 这轮为什么让它更轻 / 更有空间感

禁用同时被淡了**两次**：框架的画笔给 `Disabled` 的子树乘 0.5（`ui/paint.go:257-259`），
产品又在外面叠 `.Opacity(0.4)`，有效 **0.2**。于是禁用胶囊：

| 外观 | 地面 | 静息面 | 禁用面（改前） | 禁用顶 / 底内线（改前） |
|---|---|---|---|---|
| 浅色 | 247 | 230（−17） | **243（−4）** | 243（= 面）/ 240（−3） |
| 深色 | 15 | 39（+25） | **19（+4）** | 19（= 面）/ 18（−1） |

面离地面只剩 4 级、底对侧 1~3 级——E1 的一对线双双掉进噪声。用户说的「平、糊、旧」
「所有东西挤在同一个 z 平面上」，在状态反馈这一侧的算术来源就是这张表。

改后：面 **238（−9）/ 27（+12）**，底对侧 **−7 / −3**，顶内线 **= 面**（受光边熄了）。

## 设计读取（qiaomu-design，打磨模式）

> **读取为：桌面应用的设置页控件，面向会用扩展/配置的用户，用「轻便液态玻璃」语言，
> 倾向自绘材质 + 明确 elevation 阶梯（E0/E1/E2）这一族。**

| 拨盘 | 值 | 人话 |
|---|---|---|
| 视觉冒险度 `VARIANCE` | 4 | 只改状态反馈的墨，不动布局与形状 |
| 动效强度 `MOTION` | 3 | 本轮不加任何新动效（禁用面没有入场） |
| 信息密度 `DENSITY` | 6 | 设置页本来就密，本轮只补「一块面该有的算术」 |

功能契约：用户要能一眼看出「这个按钮现在按不动」，同时**读得清它写着什么**（这是改前
不成立的——墨只剩 43 级对比）。走的是「已有方向，只要打磨」那条路径（`craft-loop.md` +
`preflight.md`），**没有**出四方向预览页（方向已由用户给死：轻便液态玻璃）。

## 偷了什么（工艺密度菜单）

- 从 `craft-loop.md` **第 12 项「材质细节」**（内边框高光 / glass 折射边）：落成
  「**受光边是「可以被碰」的记号**，禁用面不戴它」——`Tokens.DimFace` 只画
  `InsetShadow(0, -1, 0, 0, LensEdge)`，不画 `(0, 1, 0, 0, LensRim)`。
- 从 **第 5 项「多层染色阴影」**：落成「禁用面保留**底对侧**这一层（浅 −7 / 深 −3 级）」，
  而不是把两层一起抹掉——面仍然遮挡它下面的东西，只是不再受光。
- 从 **第 6 项「编辑细节 / hairline」**：落成「**一道淡，不是两道**」这条算术——
  0.4 × 0.5 = 0.2 不是设计，是同一道淡被画了两次；落地值是框架的 0.5（实测墨对比 106 级）。
- 没有取第 1/7/8/9/10 项（氛围层 / 展示排版 / 流体字号 / 滚动条 / 彩蛋）：这是一枚 30u
  的胶囊，加任何一项都是噪点。

## 落地项

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `internal/theme/theme.go` | 无 `DimFace` | 新方法 `Tokens.DimFace`：只画底对侧 `LensEdge`，无受光边 |
| 2 | `internal/settingsui/rows.go:475` `extensionActionFace` | `disabled` 只影响 hover / press，材质恒 `LitFace` | 禁用臂材质走 `tokens.DimFace(button)`，面保持 `Control` |
| 3 | `rows.go` `busyActionButton` | `Disabled(true).Opacity(0.4)` | `Disabled(true)` |
| 4 | `rows.go` `extensionGlyphActionButton` | 无 `disabled` 参数（内部恒 `false`） | 加 `disabled bool` 透传 |
| 5 | `internal/settingsui/config.go` Restore defaults | `.Opacity(0.4)` | `busy` 进面，外面只留 `Disabled(true)` |
| 6 | `internal/settingsui/settingsui.go` Open configuration | `.Opacity(0.4)` | `!openable` 进面，同上 |

**为什么没有新增 token**：先做过 `ControlDim`（`Control` 的一半，浅 0.035 / 深 0.05），实测
被框架的 0.5 再乘一次，落到浅色 **243（−4）**——与改前逐像素相同。这一档框架已经给了；
产品再补一档只会把两个淡叠成三个。

## 视觉 pin 台账

**V 0 / B 0 / S 0**。三处禁用臂此前都没有像素 pin；新 pin 写在
`internal/settingsui/dimface_test.go`（3 条 pin + 1 条 opt-in dump）：

1. `TestTheDisabledActionPillKeepsAFaceAndLosesItsCrown`——面台阶 ≥6 级且 ≤15 级（静息是
   17 / 25）、顶内线不许亮（≤2 级）、底内线还在（浅 ≥5 / 深 ≥2 级）。
2. `TestTheBusyActionPillWearsTheDimFace`——在飞的胶囊（`busyActionButton`）同一套判据。
3. `TestTheDisabledActionPillInkIsOneFadeNotTwo`——墨对比 60~160 级（实测 106；双重淡 43；
   静息 202）。

判据单位：`litSum` 是三通道之和，所以「一级」= 3；pin 里写成 `6*level` 而不是 6（第一版
写成 6 时，双重淡的 −4 级 = −12 和恰好越过阈值，pin 假绿——这是本轮自己抓到的第一个坑）。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...    → 净
go test -count=1 ./...                                 → 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2973（基线 2970，+3）
                            ... | grep -c -- "--- FAIL" → 0
                            ... | grep -c -- "--- SKIP" → 76（基线 75，+1 = 本轮 dump）
测试函数数                                              → 2188（基线 2184，+4）
GOOS=linux/windows/darwin go build ./...               → 过
```

## 变异（4 条，全部实跑变红并逐字节还原）

| # | 注入 | 红在哪（原始串） |
|---|---|---|
| M1 | `extensionActionFace` 的禁用臂 `DimFace` → `LitFace` | `the disabled action pill (dark): the disabled pill kept a lit edge: top inner row {23 23 25} is 4 levels over the face {19 19 21}` |
| M2 | `extensionGlyphActionButton` 加回 `.Opacity(0.4)`（+ `busyActionButton`） | `the disabled action pill (light): the disabled face is only 4 levels under the ground ({243 243 244} vs {247 247 248})` / `the label is only 129 levels off its face (601 vs 730); the ink is faded twice` |
| M3 | `DimFace` 改成空实现 | `the disabled pill lost its counter-side: bottom inner row {238 238 239} is only 0 levels under the face {238 238 239}` |
| M4 | `DimFace` 画成冠线（`0,1,0,0,LensRim`） | `kept a lit edge: top inner row {36 36 38} is 10 levels over the face {26 26 28}` + 底对侧 0 级 |

还原核对：`git diff > /tmp/p13s-final.patch` 的 md5 在变异前后都是 `c79acf0df0da84d6a416d98156098d94`。

## 出图（自己复核过）

`/tmp/floter-P13-s-dimface-{light,dark}-{before,after}.png`（before 由注入态 M1+M2 实拍）。

- 差异 bbox：`(24,63)-(295,128)`，**只覆盖第二、三枚胶囊**；第一枚（静息）一个像素没动。
- 这张图上看到的一处变化：浅色档 before 里第二、三枚胶囊**只剩一行字浮在地面上**（面几乎
  与地面同色，只有字的一点墨）；after 里它们第一次有了一块完整的圆角面，**底边那条线比面
  暗 7 级**，而顶边什么也没有——面在，灯灭。

## preflight（`references/preflight.md`）

- **A 反 AI 套路**：通过。没有渐变白边、没有玻璃叠玻璃、没有 blur 冒充（§1 三条硬禁令
  全部满足：`DimFace` 是一条 1px 实色线；禁用面里没有第二个浮层；没有 gradient）。
- **B 设计读取一致性**：通过。改的是 `theme.Tokens` 与两个调用点，视图仍只读 token；
  深浅两套外观走同一个 `DimFace`（数值差异来自框架的 0.5，不是两套规则）。
- **C 动效抽查**：通过（本轮不动效）。禁用的过渡仍是原有的 120ms 颜色过渡，没有新增。
- **D2 中文页**：通过。本轮没有新增中文 UI 文案（只有代码注释与文档）。
- 未验证项：真机（非测试渲染器）上的观感差异未拍；Windows/darwin 只做了 vet + build，
  没有出图（像素 pin 走的是同一套渲染器，三平台同码）。
