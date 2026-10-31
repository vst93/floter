# P13-af 报告：分隔符属于它引入的那条事实

规格 `docs/rounds/P13-af-spec.md`。`docs/liquid-glass.md` §6 无 `未开始` 行，按纪律走通用
队列的「文案密度 / 边界情况」，接的是 P13-ae「仍待」第 2 条（集成行 meta 行）+ 抢救 stash
`afc38b247c`（2026-10-31 10:06 的 P13-af 未完成工作）。

**这轮为什么让它更轻 / 更有空间感**：轻不是「东西更少」，是**每一处标记都紧贴着它说明的
东西**。一行结尾挂一个 `·`，下一行以裸事实开头，读起来就是「有个东西丢了」——这是四种
不均匀里最难忍的一种，因为它是**句子被切错了地方**。dot 跟着事实走之后，折行只发生在
逗号语义的位置上，四行读起来仍是「四条事实」，不是「三条半」。

## 落地

| 改动 | 位置 |
|---|---|
| dot 进事实自己的节点：`a.metaFact(c, tokens, fact, lead)` = `RichText(· span, fact span)`，dot 是 `TextTertiary.Alpha(0.7)`（参考的 `.7`），事实 `Weight: 500` | `internal/settingsui/settingsui.go` `integrationMeta` / `metaFact` |
| 状态字也带 dot（透明臂）；**盒装胶囊不带**（参考 `:not(--recommended):not(--broken)`） | `stateMark(c, integration, copy, lead bool)` |
| 状态字在 8u 行 gap 之后再加 2u（参考 `.extension-status{margin-left:2px}`） | 两条透明臂 + 胶囊臂的 `Margin(0,0,0,t.Space(0.5))` |
| 三条 pin + 一份 opt-in 出图 | `internal/settingsui/metaline_test.go`、`metalinepng_test.go` |

**实测（tester，460u / 860u）**

- 860u 一行装得下四条：`· Managed` 结束于 x=527.8，`· Running` 起于 **x=537.8** → gap
  **10.0u** = 8u 行 gap + 2u 参考 margin（没这 2u 就是 8.0u，M4 实测）。
- 460u 四条事实各占一行（y=355.7 / 374.7 / 374.7 / 393.7），**每行都以自己的 `·` 开头**：
  labels 从 `"·" "Managed"` 两个节点变成一个 `"· Managed"`。dot 与事实之间是 8u（dot 盒 6u +
  字体自身空格 2u），参考是 6px margin——字体度量不同，**ink 上差 2u，1x 屏不足 1px**，
  如实记在这里而不是假装相等。

## 偷了什么（qiaomu-design / craft-loop.md）

读了 `SKILL.md` → `references/user-preferences.md`（逐条对照，P-1 无斜体 / P-38 无左侧竖线 /
P-26 奥卡姆均不涉及本轮新增物）→ `craft-loop.md` + `preflight.md`。走的是「已有方向，只要
打磨」车道（没有 Phase 2 四方向预览，方向是用户给的）。

- **工艺密度菜单第 12 项「材质细节」**：dot 的墨取 `.7` 而不是跟事实同墨——落在
  `ui.Span{Text:"· ", Color: tokens.TextTertiary.Alpha(0.7)}`，即 `rgba(…,0.7)` 的合成，
  和旧代码 `.Opacity(0.7)` 是同一个复合。
- **第 6 项「编辑细节」**：状态字前的半个单位（2u）——落在 `Margin(0,0,0,t.Space(0.5))`，
  实测 gap 从 8.0u 变 10.0u。
- **第 5 项「多层染色阴影」不适用**（meta 行是文字，不出面），**第 2 项 ::selection、
  第 4 项签名交互**也不适用（自绘 Go UI 的选中/交互另有既有实现）——为凑数去加就是 A 节
  的「无语义装饰」，按 skill 的删减通道**主动不做**。
- **DNA 注入**：本轮供体是参考实现自己的 CSS（`extensions.css:960-995`），不是 58 站库——
  这一支的单一来源是 `docs/liquid-glass.md`，换供体会破契约。偷的三条值：`content:"·"` 跟随
  事实、`margin-right: 6px`、`margin-left: 2px`。

## preflight 逐条

- **A 反 AI 套路**：本轮零新增装饰层、零渐变、零 glow、零图标；`·` 是标点不是图形。无斜体
  （P-1）、无左侧竖线（P-38）、无多余说明文案（P-26/P-27）。**过**。
- **B 设计读取一致性**：读作「设置页 · 集成列表 · 工具型功能页」，DENSITY 6 / VARIANCE 4 /
  MOTION 3；本轮不动布局与配色，只动标点归属，与声明一致。**过**。
- **C 动效抽查**：本轮无动效改动（meta 行无过渡）；既有 120ms `theme.ColorStep` 未触碰。
  不适用项如实标注：popover origin / reduced-motion 与本轮无关。**过（不适用部分已注明）**。
- **D2 中文页**：`· 运行中` 在 zh 下同样成立（`TestTheMetaLineDotsTheStateWordUnlessItIsABoxedChip`
  的 zh 段），dot 是本仓写的标点，不随语言移动；全角标点不受影响（`·` 是中西通用的间隔号，
  参考原文即用它）。**过**。
- **E 三秒测试**：眯眼看这一行，读到的是四条事实而不是「三个逗号 + 三句话」。**过**。

## V / B / S 台账

- **V 0**：没有改任何材质数值或颜色 token。
- **B 0**：行为/度量/接线测试零改动全绿。
- **S 2**（结构真变了，改断言并注明旧结构 → 新结构）：
  - `extensionsection_test.go` `TestIntegrationMetaLineCombinesTheFacts`：后三条事实的节点文本
    从 `"Extension package"` 变 `"· Extension package"`。
  - `raycast_test.go` `TestTheRunningStateMarkIsTheNeutralStrongRung`：状态字节点从 `"Running"`
    变 `"· Running"`（取样仍是离面最远的那颗像素 = 字本身，dot 是 tertiary@.7，比两档中性墨都淡）。

## 门槛（实测）

| 项 | 值 |
|---|---|
| PASS | 3026 → **3032**（+3 测试 + 3 子测试） |
| FAIL | 0 |
| SKIP | 87 → **88**（+1 opt-in 出图） |
| 测试函数 | 2234 → **2238** |
| gofmt | 空 |
| go vet（linux / windows / darwin） | 净 |
| go build（三平台） | 过 |

## mutation（4 条实跑，全红 → 还原）

| # | 注入 | 结果 | 原始报错 |
|---|---|---|---|
| M1 | dot 退回独立 `ui.Text` flex 子节点 | **RED** | `metaline_test.go:54: 860u: "· Extension package" is not one node` / `:59: a bare · is drawn as a node of its own` |
| M2 | 状态字丢掉前导 dot | **RED** | `metaline_test.go:98: the state word "Running" is not introduced by a ·`（+ `off` 子测试） |
| M3 | 破损胶囊也补一个独立 dot | **RED** | `metaline_test.go:95: 1 bare · nodes are drawn; the separator has to ride inside the node it introduces` |
| M4 | 状态字去掉 2u margin | **RED** | `metaline_test.go:147: the state word starts 8.0u after the last fact, want 10.0u (8.0u row gap + the reference's 2px margin-left)` |

每条还原后 `md5sum` 与原件**逐字节相同**（`1f516a7be64621165b5798ef4f7b9f9f`），并 `touch`
后复跑一次确认不是缓存。

## 出图

- `/tmp/floter-P13-af-meta-dark.png` / `-light.png`（460u，四条事实各占一行）
- `/tmp/floter-P13-af-meta-dark-before.png` / `-light-before.png`（同一表面，`git show HEAD:` 的
  旧 `settingsui.go` 渲染）

逐像素复核：before/after 的差异 bbox = **(237,362)-(405,408)**，正好是 meta 那三行的文字区，
页面其余部分**一字节没动**（标题、版本、Approval record、卡面全在 bbox 外）。旧图里每行末尾
那颗 `·` 在新图里出现在下一行行首——这就是「dot 跟着它引入的事实走」。

## 仍在

1. **需拍板**：`identity()` 把 publisher / version / id 用 ` · ` 拼成**一个字符串**
   （`settingsui.go` `Integration.identity`），所以那条事实内部的分隔符仍可能被折断。参考里
   identity 是一个 span（同样会折断），所以本轮**不动**——要不要把它拆成三条事实属信息结构
   变更，写进 `.pending-bugs.md` 待拍板。
2. meta 行在 460u 下仍是四行（参考也折）；如果想把 meta 收成两行需要裁事实，属行为变更。
3. P13-ae 第 1 条（两个前缀相同的会话截断后只能靠悬停区分）原样有效，仍待拍板。
