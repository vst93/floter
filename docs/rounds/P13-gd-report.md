# P13-gd · 报告：受光边只有一个来源（并把一条幻影选题销掉）

- 规格：`docs/rounds/P13-gd-spec.md`
- 合约：`docs/liquid-glass.md` **§6.149**（新，含 149-1…149-5 台账）
- 选题：纪律第 **5** 档（结构性小活）。0-4 全空的证据写在 spec §一；上轮「仍待」第 1 条
  `140-5` 仍是 `需拍板`，第 2 条（`RaisedQuiet` 的面差）本轮**先量后证伪**。

## 这轮为什么让它更轻 / 更有空间感

空间感在这套自绘材质里靠三样东西表达：**面差、阴影、一条受光边**。前两样在 P13-gb / P13-gc
被换算到「框的可见 alpha」上，三平台已经是同一个数；第三样（受光边）却只靠**散文**守着——
`LensRim` 被重定基线四次（`0.04 → 0.09 → 薄档不缩放 →` 浅色 `0.24`），每一次的病根都是
某个视图**手写**了那条线的颜色：token 改了、测试全绿、屏幕上还是旧值（浅色档曾经就是
「一个看不见的像素」）。这一轮把「这条结构线只能来自一个数」写成断言——**受光边从此
不会在某个表面悄悄退回旧值**，这正是「靠受光边表达层级」能成立的前提。

## 一、先量：`RaisedQuiet` 是零生产调用点的 token（幻影选题证伪）

```
$ grep -rn "RaisedQuiet" --include=*.go internal/ cmd/ | grep -v _test.go
internal/settingsui/custom.go:75        // 注释
internal/settingsui/healthfaces.go:73   // 注释
internal/settingsui/healthfaces.go:221  // 注释
internal/settingsui/settingsui.go:2677  // 注释
internal/settingsui/settingsui.go:3983  // 注释
internal/theme/theme.go:369/372/595     // 字段文档 / 声明 / 赋值
```

五处非 theme 命中**全是注释**，AST 层面 **0 个 `SelectorExpr`**：P8-b / P8-c 已经把它
四处面换成 `Control` / `RowSelected`。P13-gc 报告提议的下一格（「`RaisedQuiet` 在 `Sunken`
变厚之后的面差」）**没有可量的对象**——那个面已经不画了。

处置：字段文档加 `Deprecated:`（证据 + 被谁取代 + **保留不删**的理由，同 `AccentTintQuiet`
/ `AccentGlow` 先例）。**产品代码零像素改动。**

## 二、先量：视图层的受光边，取色来源

| 层 | `InsetShadow(` grep 行数 | AST 实见 | 字面量取色 |
|---|---|---|---|
| `internal/theme`（定义处，跳过） | 17 | —（不在视野内） | `LensInset` 的 `0.85` / `0.06`（镜片原始数值） |
| 视图树（`shell` / `settingsui` / `launcher` / `terminalui` / `extensions` / `control`） | 26 | **21** | **0** |

26 行 grep 里有 5 行只存在于散文注释（`// …InsetShadow(0, 1, 0, 0…`），所以 AST 的账是 21。
**21 条内线全部用 token 取色**——今天这条不变量成立，但**没有东西守着它**。

顺手量了视图树的字面色：`ui.RGBA(` 共 **14 处**，无一处是材质线——
`terminalui.go` 8 处（8 个终端主题的 selection）、`theme/elevation.go` 5 处
（投影 `0.30*scale` / `0.20*scale`，本来就该随 `main_opacity` 缩放）、
`uninstall.go` 1 处（模态遮罩）。

## 三、落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/ci/litedgecolour_contract_test.go`（新） | AST 扫 `internal/`（跳过 `../theme`）与 `../../cmd` 的生产文件：每个 `.InsetShadow(...)` 的**颜色实参**不许是 `ui.RGBA` / `ui.RGB` / `ui.Hex` / `ui.Color{}`；报 `file:line` + 实参源码。前置：站点数 < **18** 就 `Fatal`（实测 21，「扫描器变瞎」必须大声红）。逃生口 `litedgeLiteralAllow`（空表；登记了却已不违规 ⇒ 报过期并红） |
| 2 | `internal/theme/theme.go`（`Tokens.RaisedQuiet` 文档） | 加 `Deprecated:`：零生产调用点的实测证据、被 `Control`/`RowSelected` 取代、字段保留不删 |
| 3 | `docs/liquid-glass.md` | 新增 §6.149（149-1…149-5） |
| 4 | `.pending-bugs.md` | 本轮处理置顶；P13-gc「仍待」第 2 条标成**已证伪** |

## 四、偷了什么（qiaomu-design · craft-loop 工艺密度菜单）

逐条落成具体数值/位置，不写空话：

1. **菜单第 6 项「编辑细节：hairline」** → 这条线的**唯一来源**：视图 21 条内线全走 token
   （`LensRim` 浅 0.24 / 深 0.09、`LensEdge` 浅 0.06 / 深 0.20、`EdgeHighlight` 浅 0.92 /
   深 0.20、`TrackRim` 浅 0.70 / 深 0.09）；定义处只有 `LensInset` 的 `0.85` / `0.06` 两笔。
2. **菜单第 12 项「材质细节：内边框高光」** → 把「内边框高光必须来自 token」变成可执行断言
   （`litedgecolour_contract_test.go`），并 pin 住 21 条站点的存在（< 18 即红）。
3. **菜单第 5 项「多层染色阴影」** → 复核视图层的 5 条投影：`0.30*scale`（环境）+ `0.20*scale`
   （接触），`scale = ElevScale(main_opacity)`；结论是**它们就该是字面值 + 缩放**，登记为
   `故意不做`（149-4），不是遗漏。
4. **A2 两轮制** → 本轮是零像素轮：第一轮「量」（上面两张表）与第二轮「立守卫」在同一个
   工作单元里完成；四视角评审见 §六。

## 五、preflight 逐条结论

- **A 反 AI 套路**：通过（本轮不加任何视觉元素；无渐变/发光/斜体/三等分卡）。**A2 DNA 注入：
  部分不适用的项写明——本仓是 Go 自绘 canvas，没有 CSS，58 站库给的是 web 值（阴影栈、
  `clamp()`、`::selection`），能迁移的只有「数值可溯源」这条机制，本轮落成 `Deprecated:`
  里的旧值→新值台账与 §6.149 的实测表。**
- **B 设计读取一致性**：通过。一句话记忆点＝「所有受光边读同一个数」。本轮产出与该语气一致
  （只加约束，不换皮）。
- **C 动效抽查**：不适用——本轮零动效改动；`theme.ColorStep` 120ms 那笔账在 P13-ad 已收口。
  `prefers-reduced-motion` 在 mygo 自绘里没有对应查询，属**已知缺口**，不在本轮范围
  （写在这里而不是跳过）。
- **D2 中文页**：通过（本轮新增文案只有报告与合约；中文行高、全角标点、中西文空格照旧）。
  沿用需拍板项：`tokens.Caption` 9.9px / 正文 10.8px 与 D2「正文 ≥ 14px」的冲突仍是
  **需拍板**，本轮不动。

## 六、四视角（craft-loop 第二轮）

- **艺术总监**：这一轮不出新画面，最平庸的是「重复的 pin 报告」——记忆点只有一条
  「受光边读一个数」，成立但薄。
- **工程师**：`InsetShadow` 的**其他**内线形态（`.Lens(` / `LitFace(` 的 1px 顶边）没有单独
  守卫，只有 `litarm_contract` / `flatface_contract` 管「有没有」，不管「颜色从哪来」。
  本轮只堵了 `.InsetShadow(` 这一半，剩余一半记在「仍待」。
- **甲方**：三秒看完——「它保证那条线不会悄悄退回旧值」。可读。
- **读者**：§6.149 的表比正文长，但每行可核。

## 七、门槛（实测）与变异

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 -v ./...                 # 退出 0；--- PASS 3463（基线 3462）；--- FAIL 0；SKIP 207
测试函数                                   # 2630（基线 2629）
GOOS=linux/windows/darwin go build ./...  # 三个都过
```

变异 2 条**实跑**（原始报错串照抄）：

```
M1  internal/settingsui/savealert.go:114  tokens.EdgeHighlight → ui.RGBA(255, 255, 255, 0.92)
    --- FAIL: TestEveryLitEdgeTakesItsColourFromAToken
        a lit edge takes its colour from a token, not a literal:
        settingsui/savealert.go:114  <- ui.RGBA(255, 255, 255, 0.92) — §3 的受光边只有一个来源

M2  internal/shell/detach.go:340 前插 InsetShadow(0, 1, 0, 0, ui.RGBA(0, 0, 0, 0.06))
    --- FAIL: TestEveryLitEdgeTakesItsColourFromAToken
        a lit edge takes its colour from a token, not a literal:
        shell/detach.go:340  <- ui.RGBA(0, 0, 0, 0.06) — §3 的受光边只有一个来源
```

两条都用 `git checkout` 还原，`md5sum -c` 逐字节核对（`76a2eeca353969e9051255e080f9f032`
`savealert.go`、`7ad4fed5827e4e2dcbed6a93883beb82` `detach.go`，后者与 P13-fz 台账同值），
再 `touch` 两个文件复跑一次（避免测试缓存）→ `ok floter/internal/ci`。

**V / B / S 台账**：**V 0 / B 0 / S 0**——没有一条既有 pin 变红，没有一条断言被改，
产品像素零改动（`git diff` 里 `theme.go` 只有注释）。

**出图**：本轮零像素改动，不给 before/after 假图；复核路径是 §二 的两张表 + 新 pin。

## 八、仍待

1. **`140-5` 仍需拍板**（同一条顶边在深浅下读成相反的东西）。
2. **`RaisedQuiet` 这条幻影选题已销**——下一轮不要再量它。
3. 同族的另一半：`.Lens(` / `LitFace(` 的**颜色来源**没有守卫（只有「有没有」的守卫）。若要走，
   得先把 `LitFace` 的入参形式统一（它现在只吃 token，所以今天也是干净的）。
4. 拍板项沿用：138-3 / 138-4 / 137-4 / 129-5 / 111 / 字号账（D2 的 9.9px / 10.8px）。
5. 终端弧 S1b / S1c / S2 尾仍在等拍板。
