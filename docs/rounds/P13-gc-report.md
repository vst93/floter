# P13-gc · 报告：内容页那一档的 0.09，也是「框的可见 alpha」上的一步

- 规格：`docs/rounds/P13-gc-spec.md`
- 合约：`docs/liquid-glass.md` **§2.3**（新）+ **§6.148**（新）
- 选题：通用队列「**三平台一致性**」那一格的第二笔（P13-gb 报告的「仍待」第 2 条：
  `MaterialVeil` 只补偿了 `FloatAlpha` / `ContentAlpha` 里的浮层那一半，`Sunken` 那一档
  从没人在 veil 之下量过 1x 可辨性；`140-5` 仍 `需拍板` → 走替代路径）

## 这轮为什么让它更轻 / 更有空间感

空间感 = 面与面之间那个**差值**看得见。内容页（设置体 / 结果列表那张面）承诺比它压着的框
深一步（0.09），但这一步从前算在**应用自己画的填充**上，而 macOS 的窗口材质已经把填充补偿
薄了——于是同一步在 1x 屏上只剩 6~7 级，**低于 8 级的可辨门槛**：最漂亮的玻璃档里，
内容页读成一个平面。修完之后三平台、四档位、所有滑块上都是同一步、同一个级数，
darwin 与 linux 在同一个滑块档上**逐字节相同**。

## 量（实测，先量后改）

可见 recess = `Composite(面) − Composite(框)` = `0.09 · (1 − veil)`：

| 平台 / 档（47%） | veil | `TintAlpha` | 旧填充 | 旧可见 recess | 新填充 | 新可见 recess |
|---|---|---|---|---|---|---|
| linux / windows | 0 | 0.470 | 43 | 0.090 | 43 | 0.090 |
| darwin `frosted` | 0.12 | 0.398 | 38 | 0.079（−12%） | 43 | 0.090 |
| darwin `regular` | 0.22 | 0.321 | 38 | 0.070（−22%） | 43 | 0.090 |
| darwin `liquid` | 0.34 | 0.197 | 38 | **0.059（−34%）** | 43 | 0.090 |

**渲染像素**（`ui.NewTester` 三层真 token：框 / 面 / 控制，中灰 `RGB(128,128,128)` 地面，
级数 = 面与框动得最多的那个通道）：

| 格 | 浅 旧 → 新 | 深 旧 → 新 |
|---|---|---|
| linux `liquid` 47% | 11 → 11 | 10 → 10 |
| darwin `frosted` 47% | 9 → 11 | **8 → 10** |
| darwin `regular` 47% | 8 → 11 | 8 → 10 |
| darwin `liquid` 47% | **7 → 11** | **7 → 10** |
| darwin `liquid` 90% | **6 → 9** | **5 → 8** |
| darwin `liquid` 10% | **7 → 11** | **6 → 10** |

两条量出来的事实：

1. **`liquid` 那一档在 macOS 上一直读不出来**（6~7 级 / 门槛 8），而 `regular` 是 8 级——
   正好骑在门槛上，所以「面站起来了」这件事在三档玻璃里只有最薄的一档成立。
2. **级数与滑块无关，与 `(1−veil)` 有关**：旧算法下 47% 和 10% 是同一个 7 级（`liquid`），
   因为掉的是材质那一份，不是滑块那一份。这正是「量在填充空间」的症状。

## 落地

1. `internal/glassmap/glassmap.go` · `Spec.ContentAlpha`：`overFrame(TintAlpha, TintAlpha+0.09)`
   → **`overFrame(Composite(), Composite()+0.09)`**（可见 alpha 上的一步）。注释写清旧值 → 新值、
   6.8 / 6.6 级那两格，以及**为什么 `TerminalFrameAlpha` / `CanvasAlpha` 不动**
   （它们的目标是那个**绝对** alpha `TintAlpha`，绝对 alpha 在填充空间里本来就对）。
   `Material == 0`（linux / windows / `off`）逐位不变。
2. `internal/glassmap/contentplatform_test.go`（新，2 函数）：三平台 × 四档 × 六滑块档的
   **字面量** `ContentAlpha` 表（含 0.10/0.12/0.22/0.34/0.47/0.90/0.98 的可见 alpha 格）
   + 「可见 recess 恒为 0.09」+ 同 `C` 则同填充 + 面不高于浮 + 反面臂（面 ≠ 框的填充、
   `off` 两平台同值且 `Material == 0`）。
3. `internal/theme/sunkenveil_test.go`（新，2 pin + 1 opt-in dump）：14 格 × 深浅两套，
   渲染真 token 三层并读像素——面 vs 框 ≥ 8 级、控制 vs 面 ≥ 8 级（实测 13~24 级，
   `Control` 这一档**量过、不改**：它是画在面之上的一笔，veil 只挪它脚下的地面）。
4. `internal/theme/theme_test.go`：`darwin.Sunken.A >= linux.Sunken.A` 这条**旧基线的 V**
   改成相等（`P13-gc 重定基线：旧值 darwin 38 / linux 43 → 新值两者相等`），并把 P13-gb
   那句「`Sunken` 不动」改成指向本轮。
5. 合约：§2.3（新）、§6.148（新，含四行落地表与出图证据）。

**行为契约零改动**：高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n / 命令接线一字未动。
Linux 与 Windows 的渲染逐字节不变（本轮改的量只长在 `Material > 0` 的分支上）。

## 图（自己复核过）

`FLOTER_P13GC_DUMP=1 go test -run TestDumpP13gcVeilPane ./internal/theme/` →
`/tmp/floter-P13-gc-veilpane-dark.png` / `-light.png`。一排六格：左三格出厂算式
（linux 47% ｜ darwin `liquid` 47% ｜ darwin `liquid` 10%），右三格同一张框、同一块地面上的
**旧填充空间**那一笔。读面那一块（框读的是它下面那条边）：

| 格 | 浅 面 / 框 | 深 面 / 框 |
|---|---|---|
| linux `liquid` 47% | `(194,194,196)` / `(184,184,185)` = 11 | `(65,65,66)` / `(75,75,76)` = 10 |
| darwin `liquid` 47% 新 | `(194,194,196)` / `(184,184,185)` = 11 | `(65,65,66)` / `(75,75,76)` = 10 |
| darwin `liquid` 10% 新 | `(178,178,180)` / `(168,168,169)` = 11 | `(79,79,80)` / `(89,89,89)` = 10 |
| darwin `liquid` 47% 旧 | `(191,191,192)` / `(184,184,185)` = **7** | `(68,68,69)` / `(75,75,76)` = **7** |
| darwin `liquid` 10% 旧 | `(175,175,176)` / `(168,168,169)` = **7** | `(83,83,83)` / `(89,89,89)` = **6** |

**这张图上看到的一处变化**：左半边（新）里 linux 与 darwin 两格**逐字节相同**——同一张框
就配同一笔面；右半边（旧）两格的面前一档都往框的方向缩了 3~4 级，浅色那张图上尤其明显
（191 与 184 之间只剩 7 级，眯眼看就是一张平面）。

**一处只有 1 级的连带**：设置体把面合成成不透明再画（`settingsui.go:1303`，
`FaceOver(地, Sunken)`），所以它跟的是面**自有**的 alpha。实测 darwin `liquid` 47%：
浅 **247 → 246**、深 **14 → 15**，正好落在 linux 的同名字节上——三平台在**不透明合成**
这一层也终于是同一个数（pin：`TestTheOpaqueBodyLandsOnTheMaterialFreeByte`）。

## 「偷了什么」（qiaomu-design）

读：`SKILL.md`（Router：已有方向 → **打磨模式**，不走 Phase 2 的四方向预览）→
`references/user-preferences.md`（P-1 无斜体 / P-5 可读性优先 / P-38 无左侧色条）→
`references/craft-loop.md`（工艺密度菜单）→ `references/preflight.md`（A / B / C / D2）。
`craft-loop.md` 的「两轮制」按本仓形态落成：**先量（探针）→ 读数 → 只修量出来的那一条**。

| 取自 `craft-loop.md` 工艺密度菜单 | 落成的具体数值 |
|---|---|
| **#12 材质细节**（内边框高光 / glass 折射边） | 内容页的 recess = **框的可见 alpha + 0.09**（填充 43，浅色地面 11 级 / 深色 10 级）；材质越厚不再吃掉这一步 |
| **#5 多层染色阴影**（拒绝单层灰影） | 不新增层：浮层的 `ElevThrow` 两层（`0 10px 26px` 0.30 + `0 1px 2px` 0.20）保持不变，本轮只保证面档站在它下面 ≥ 8 级 |
| **#3 品牌 focus 环** | 已有：环在盒外 3px、`RGBA(6,74,166,0.82)` / `RGBA(122,167,255,0.78)`（P13-ga §6.146），本轮不动 |
| **#6 编辑细节**（hairline / 大写字距标签） | 已有：1px 发丝线按整格判定（P13-fu §6.140）；本轮的合约表也按格写、分隔行不带单元格 |
| **#9 自定义滚动条** | 已有（`Scrollbar` 0.24 + `ScrollbarWidth` 6×f）；本轮不动 |
| **#10 收尾彩蛋** | 不适用：材质常数的轮次，没有交互揭示面 |

## preflight 逐条

- **A 反套路**：无渐变白边（受光边是 1px 实色线）、无纯黑（深色面 `17,18,20`、地面 `247/14`）、
  无外发光、无斜体（P-1）、无三等分卡横排、无假数据、无左竖色条（P-38）。**通过**。
- **B 设计读取一致性**：记忆点一句话 = **「同一个滑块档，三个平台画的是同一笔面」**；
  深浅两套逐值同构（本轮所有 pin 都两套跑）；动效零改动，气质不变。**通过**。
- **C 动效抽查**：本轮零动效改动（材质常数 + 一个 `overFrame` 的输入）。
  动效条目 N/A，理由写在此。**通过（N/A）**。
- **D2 中文页**：中文行高 / 字号两条**不能过**——正文 10.8px、`Caption` 9.9px 低于「正文 ≥ 14px」，
  这是挂了 76 份证据的 `需拍板` 项（不许擅自改）；中西文空格、全角标点由 i18n 文案与既有 pin 守着。
  **部分不通过，原因见上**。

## 台账 V / B / S

- **V 1**：`internal/theme/theme_test.go` 的 `darwin.Sunken.A >= linux.Sunken.A` → `!=` 相等
  （注释写旧值 38/43 → 新值 43/43）。
- **B 0**：其余测试零改动，全绿（`internal/ci` 的合约表检测也绿）。
- **S 0**：无结构变化。

## 门槛（实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → **净**（三条全跑）
- `go test -count=1 ./...` → **全绿**
- PASS 计数 **3462**（基线 3456 = P13-gb 收尾；+6 = glassmap 2 个 pin + theme 的
  `TestTheContentPaneStillReadsAsAFaceUnderTheMaterial` 1 + 2 子测试 + `TestTheOpaqueBodyLandsOnTheMaterialFreeByte` 1；
  dump 是 opt-in，跳过不计）
- 测试函数 **2629**（基线 2624，`grep -rh "^func Test" internal cmd`；+5：glassmap 2 +
  theme 2 pin + 1 dump，未减）
- `GOOS=linux/windows/darwin go build ./...` → **三条都过**

## 变异（实跑，3 条）

- **M1** `ContentAlpha` 退回 `overFrame(s.TintAlpha, …)`：
  `internal/glassmap` **18** 行红，例如
  `darwin/frosted/47%: the content pane paints 0.14943396, want 0.16981132 — 薄档的默认滑块：旧填充 0.149（可见 recess 0.079）`
  与 `darwin/frosted/47%: the pane composites to 0.54919994, want the frame + step 0.55999994 (recess 0.07919997, want 0.089999974)`；
  `internal/theme` 3 处红：`[light] darwin liquid 47%: the content pane {191 191 192 255 [0 0 0]} sits 7 levels above the frame … under the face floor 8`
  / `[light] darwin liquid 90%: … sits 6 levels above the frame …`（顺带量出 `regular` 47% 是 8 级、
  `frosted` 是 9 级——正好解释了为什么旧值在 `regular` 上「看起来没问题」）。
- **M2** `contentStep` 0.09 → 0.06：`internal/glassmap` **17** 行红，含
  `linux/off/10%: the content pane paints 0.0923077, want 0.13846155`（证明这套 pin 不是
  「只有 macOS 会红」）；`internal/theme` 红在 `linux liquid 47%: … sits 7 levels …` 与
  `TestTheOpaqueBodyLandsOnTheMaterialFreeByte`。
- **M3**（像素层）`sunken := spec.ContentAlpha()` → `spec.TintAlpha`（让面等于框的填充）：
  `[light] darwin liquid 10%: the content pane {168 168 169 255 [0 0 0]} sits 0 levels above the frame`
  / 深色同句 + `TestTheOpaqueBodyLandsOnTheMaterialFreeByte` 红 2 处。
  （第一次尝试写成 `baseFill.Alpha(spec.TintAlpha)` 直接编译不过：`declared and not used: sunken`——
  如实记在这里，那不是一条有效的变异。）
- 还原：`md5sum -c /tmp/p13gc.md5` 两条全 `成功`
  （`c53f36a54ee8012ccbb014bc382fc355` / `4789e24299169cf5c4560a8d6076a747`）
  + `touch internal/glassmap/*.go internal/theme/*.go` 再复跑
  `ok floter/internal/glassmap 0.005s` / `ok floter/internal/theme 0.025s`。

## 仍待

1. **`140-5` 仍需拍板**（同一条顶边在深浅下读成相反的东西）：本轮没有动它。
2. **这一族还剩一格没量**：`RaisedQuiet`（侧栏选中页）是 `blendOpaque(baseFill, controlHover)`
   的**不透明**合成，所以在 veil 之下不缩——但它也因此**不吃**滑块，`Sunken` 变厚之后
   「选中行 vs 侧栏面」的差会跟着动。下一轮可量：§6.2 侧栏那 8 级面差在 `liquid` + 10% 上
   是否仍成立（先量后改，可能只是一条 pin）。
3. 沿用：`137-4`（About 页 92 字时标签盒 0×0）、`129-5`、`111`（浮层失败句宽度上限）、
   `138-3` / `138-4`、以及 `tokens.Caption` 9.9px / 正文 10.8px 的字号账（preflight D2）。
