# P13-t 报告 · 锁定的表单：一道淡，不是三道

方向一句话：**同一张只读表单上，「不可用」只有一个深度。**

## 这轮为什么让它更轻 / 更有空间感

锁定（`locked`）的扩展配置表单此前在三处画了三个深度：

| 控件 | 有效淡度 | 来源 |
|---|---|---|
| 字段 / 开关 / 下拉 | **0.3** | 框架给 `Disabled` 子树的 0.5 × 产品自己的 0.6 |
| 列表框（multiselect） | **0.6** | 只有产品那一档，`Disabled` 忘了挂 |
| 框里的勾选框 | **0.25** | 0.6 × 自己的 `Disabled` 0.5 |

实测（浅色地面 247 / 深色 14）：

| 读数 | 静息 | 改前 | 改后 |
|---|---|---|---|
| 字段面 | 230（−17）/ 39（+24） | **242（−5）/ 22（+7）** | **238（−9）/ 27（+12）** |
| 字段墨对比 | 203 | **65** | **101** |
| 开关轨道 | 79 / 187 | **195 / 67** | **163 / 101** |
| 列表框面 | 230（−17）/ 39 | **236（−11）/ 29（+15）** | **238（−9）/ 27** |
| 列表框选项文字 | 108（−121） | **160（−76）** | **177（−61）** |

两个读法：①0.3 把字段的 E1 一对面**双双抹平**（顶内线 = 面 = 242、底内线 = 面），
「不能碰的槽」变成一块几乎不存在的槽——与 P13-s 量到胶囊只剩 4 级台阶是同一个算术；
②同一个表单里 0.3 / 0.6 / 0.25 三种深度**同时存在**，用户读到的是「有的禁用有的没禁用」，
这正是「平、糊、旧」在状态反馈这一侧的来源。改后三块落在同一级：**面 −9、灯熄了、
墨只淡一道**。

## 设计读取（qiaomu-design，打磨模式）

> **读取为：桌面应用里一张「正在运行中所以只读」的扩展配置表单，面向会改扩展配置的
> 用户，用「轻便液态玻璃」语言，倾向自绘材质 + 明确 elevation 阶梯（E0/E1/E2）这一族。**

| 拨盘 | 值 | 人话 |
|---|---|---|
| `VARIANCE` | 3 | 不加新形状、不新增 token，只改状态反馈的墨 |
| `MOTION` | 0 | 不动效（锁定的 120ms 颜色过渡原样） |
| `DENSITY` | 6 | 表单本来就密，只补「一块槽该有的算术」 |

功能契约：用户要能一眼看出「这张表单现在不能改」，同时**读得清它写着什么**（改前字段墨
只剩 65 级对比）。走「已有方向，只要打磨」那条路径（`craft-loop.md` + `preflight.md`），
**没有**出四方向预览页。

## 偷了什么（工艺密度菜单）

- 从 `craft-loop.md` **第 12 项「材质细节」**（内边框高光 / glass 折射边）：落成
  「**一道淡是控件根的属性，不是每一块面自己的事**」——新增 `lockedControl`，一个
  `Disabled(true)` 挂在控件根上，整棵子树（箱面、选项文字、勾选框）同乘 0.5；数值上
  字段顶内线从 242 回到 **239**、底对侧从 242 回到 **231**。
- 从 **第 5 项「多层染色阴影」**：落成「锁定不是把两层一起抹掉」——`LitFace` 的
  `LensRim` / `LensEdge` 那一对在 0.5 下**仍然各留一半**（+1 / −7 级），而 0.3 下
  两者都归零。面的台阶与对侧阴影是「它还占着 z 位置」的证据，不能一起熄。
- 从 **第 6 项「编辑细节 / hairline」**：落成「**一道淡，不是两道**」这条算术——
  0.5 × 0.6 = 0.3、0.6 × 0.5 = 0.25 都不是设计，是同一道淡被画了两次/三次；落地值
  是框架的 0.5（字段墨 65 → 101 级）。
- 没有取第 1/7/8/9/10 项（氛围层 / 展示排版 / 流体字号 / 滚动条 / 彩蛋）：这是一张
  表单里的状态反馈，加任何一项都是噪点。

## 落地项

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `internal/settingsui/config.go:163` | 无 `lockedControl` | 新 helper：`Disabled(true)` + 默认光标，**不带 `Opacity`**；doc 写明为什么 |
| 2 | `config.go:532` boolean 开关臂 | `.Disabled(true).Opacity(0.6)` | `lockedControl(e)` |
| 3 | `config.go:559` select 臂 | 同上 | 同上 |
| 4 | `config.go:661` number 字段臂 | 同上 | 同上 |
| 5 | `config.go:715` text/password/path 字段臂 | 同上 | 同上 |
| 6 | `config.go:855` `configMultiselectBox` | 锁定档在**调用方**，且只有 `.Opacity(0.6)`、没有 `Disabled` | 签名加 `locked bool`，档位落在控件根上（`lockedControl`）；调用方那一臂删掉 |
| 7 | `config.go:900` `configOptionRow` 勾选框 | `locked` 时再挂一次 `Disabled(true)`（0.5 × 0.5 = 0.25） | 删掉——祖先已经 disabled，`IsDisabled` 走祖先链；`locked` 只留两条点击守卫 |

**为什么没有新增 token**：P13-s 已经定过「这一档框架已经给了」。产品再补一档只会把两个
淡叠成三个——本轮正是那三个的收口。

**为什么 `configMultiselectBox` 要收 `locked`**：档位如果留在调用方，pin 就只能钉到
「调用方传什么」，钉不到「这个控件怎么淡」——第一次做变异时 M2（把根上的 flag 换回
`.Opacity(0.6)`）**没变红**，就是因为测试自己调了 `lockedControl` 而产品那一行没被走到。
把决定挪进控件内部之后，同一条变异立刻变红。

## 行为契约零改动红线

- `configLocked` 的判定、`e.Changed() && !locked` 的守卫、`word.Clicked() && !locked`
  一个字节没动；`locked` 只是从「每一块面各淡各的」变成「根上一个 flag」。
- **滚动**：`Disabled` 只挡点击 / 焦点 / hover，不挡滚轮（`ui/input.go:515-536` 的
  `scrollBy` 不查 `flagDisabled`），锁定的长选项列表仍然读得到。
- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：零改动。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...    → 净
go test -count=1 ./...                                 → 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2977（基线 2973，+4）
                            ... | grep -c -- "--- FAIL" → 0
                            ... | grep -c -- "--- SKIP" → 77（基线 76，+1 = 本轮 dump）
测试函数数（`git grep -h "^func Test" HEAD -- '*_test.go' | wc -l`）
                                                       → 2190 → 2195（+5 = 4 pin + 1 dump）
GOOS=linux/windows/darwin go build ./...               → 过
```

V 0 / B 0 / S 0：没有一条旧 pin 变红（`configform_test.go` 与 `multilineface_test.go`
各改两行是**编译期**签名跟随，断言一个字节没动）。

## 变异（3 条，全部实跑变红并逐字节还原）

| # | 注入 | 红在哪（原始串） |
|---|---|---|
| M1 | `lockedControl` 加回 `.Opacity(0.6)`（一道变两道） | `the config field (light): the locked face is {242 242 243} but one fade of {230 230 230} over the ground {247 247 248} is {239 239 239} (off by 3 levels)` |
| M2 | `configMultiselectBox` 的根 flag 退回 `.Opacity(0.6)` | `the multiselect list box (light): the locked face is {236 236 237} … is {239 239 239} (off by 2 levels)` |
| M3 | `configOptionRow` 给整行再挂一次 `Disabled(true)` | `the multiselect list box (light): the locked ink is {205 205 207} but one fade of {107 107 112} over the ground {247 247 248} is {177 177 180} (off by 27 levels)` |

还原核对：`md5sum -c` 三次都对（`a74219c8…` → 改后 `md5` 与本轮终值相同），每次
`cp` 回备份后再跑一次绿。

**没能变红的一条（如实记）**：`lockedControl` 改成 `Opacity(0.5)`（不挂 `Disabled`）时
像素**完全相同**——像素 pin 钉不到「用哪条机制淡」。这一条只有语义差别（不挂 flag 时
hover / press / focus 仍然亮），靠代码注释与 §6.2b-6 的规矩守，不靠 pin。

## 出图（自己复核过）

`/tmp/floter-P13-t-lockedform-{light,dark}-{before,after}.png`（before 由
`FLOTER_PNG_TAG=before` 把旧的第二道淡注回去实拍）。版面是「静息 / 锁定」成对的
字段、开关、列表框。

- 差异 bbox：浅色 `(20,62)-(300,294)`、深色同——两套外观里**每一对控件都动了**，
  静息那一枚也在框内（因为锁定枚的像素变了会带动整列）。
- 这张图上看到的一处变化：浅色档第二块（锁定的那个字段）`(160,66)` 的读数
  **before `(242,242,243)` → after `(238,238,239)`**，而地面 `(247,247,248)` 不动
  ——锁定字段第一次**离地面 9 级**，before 只有 5 级，整块面几乎和卡片同色。

## preflight（`references/preflight.md`）

- **A 反 AI 套路**：通过。没有渐变、没有 `#000000`、没有外发光、没有 italic、没有
  emoji 图标；本轮不新增文案，所以没有「革命性/无缝」这类词可查。§1 三条硬禁令
  全部满足：没有 blur 冒充玻璃、没有 glass 叠 glass、没有渐变白边（改的是一个
  `Disabled` flag 的去留）。
- **B 设计读取一致性**：通过。改的是 `lockedControl` 与它的四个调用点，视图仍只读
  token；深浅两套外观走同一个 helper（数值差异来自框架的 0.5 与地面，不是两套规则）。
  一句话记忆点：**锁定 = 面退一半、灯熄、字只淡一道**。
- **C 动效抽查**：通过（本轮不动效）。锁定没有入场动画，颜色过渡仍是原有的 120ms。
- **D2 中文页**：通过。本轮没有新增中文 UI 文案（只有代码注释与文档）。
- 未验证项：真机（非测试渲染器）上的观感未拍；Windows / darwin 只做了 vet + build，
  没有出图（像素 pin 走同一套渲染器，三平台同码）。

## 仍待（P13-t，追加在 P13-s 之后）

1. **accent 主按钮的禁用臂仍是 `.Opacity(0.4)`**（`extensionPrimaryButton` 的 Save、
   `updateButton`）：accent 面要淡的是**色相**（fill 本身），不是「面还在」的问题。
   实测改前 Save 的禁用面是 214（地面 247，−33），墨 198——比中性胶囊好，但
   `DimFace` 在这里没有意义（它没有受光边可熄，面也不是中性灰）。要做先定
   「禁用 accent = accent 与卡面按什么比例混」，**另起一轮**。
2. `uninstall.go:361`（卸载确认按钮）、`settingsui.go:5871`（`updateButton`）、
   `rows.go:829`（`sessionIconButton` 的禁用臂，透明面）、`rows.go:895`
   （`sessionKillConfirm`，警告色胶囊）四处 `.Opacity(0.4)` 与 `rows.go:668`
   （`disabledSwitch`，设置页的开关，不是配置表单）仍是两道淡。**这一族在设置页
   还有 5 处**，与 `lockedControl` 同形，可以一轮收掉——但 `sessionKillConfirm`
   戴 `RaisedWarm` + `ShadowContact` + `LensRim`，`DimFace` 会把警告环洗淡
   （§6.2b-4 的 9-14b 同一个理由），要先拍板。
3. **锁定列表框的选项文字落在可读性下限**：改后 61 级对比（`TextSecondary` 本身
   就比 `TextPrimary` 静，再乘 0.5）。P-5 的下限是 60 级，**刚好过**。要不要给锁定
   的选项行换一档更深的墨，需拍板。
4. P13-s 第 1/2 条、P13-r 第 1 条（`Thumb` 在浮层上的取值）、P13-q 第 1 条、
   P13-p 第 1/2 条、P13-k 第 1/2 条、P13-c 四条、P13-h 第 9 条原样有效（均需拍板）。
