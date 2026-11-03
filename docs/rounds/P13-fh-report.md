# P13-fh 报告：集成页结果格最长的那句话，量在它真正会被画的那个宽度上

**方向一句话**：这一轮的产出是「**不加**」——量清楚后，那块两格 metadata 保持两列等高、单行、
零额外高度，而不是为一件在固定 720u 面板上永不发生的事长出一层换行。

- 选题：P13-fg「下一格（首选）」/ `docs/rounds/P13-fg-report.md`「仍待」第 1 条。
- 表面：集成页抽屉 `freshnessSection` 的 `Result` 格（`internal/settingsui/settingsui.go:6076-6140`）。
- 合约：新增 `docs/liquid-glass.md` **§6.129**。规格 `docs/rounds/P13-fh-spec.md`。

## 1. 先量（探针跑完即删，数字抄在这里）

`TestProbeP13FHResultCell`，720×900 生产取景 + 1600u 取自然宽，`Succeeded with warnings`：

| 面板宽 | 结果格盒宽 | 墨列 | 句子 |
|---|---|---|---|
| 1600u | 683.5u | 129 | 整句 |
| 860u | 313.5u | 129 | 整句 |
| **720u** | **243.5u** | **129** | **整句，余 114.5u** |
| 560u | 163.5u | 129 | 被切（尾端是省略号） |
| 460u | 113.5u | 98 | 被切 |

`TestProbeP13FHBlockGeometry`（同一帧的纵向节奏，确认块本身没有别的病）：

```
y=357.70 h=15 x=195.80 w= 52.00  "Freshness"
y=372.70 h=15 x=195.80 w=243.50  "Last probe"      y=372.70 x=451.30 "Result"
y=387.70 h=17 x=195.80 w=243.50  "3 yr. ago"       y=387.70 x=451.30 "Succeeded with warnings"
y=404.70 h=17 x=256.80 w=  6.00  "6"
y=405.70 h=15 x=195.80 w= 53.00  "Commands"  ·  x=270.80  "+2 since the previous probe" x=284.80 w=125
```

两格等宽 243.5u、gap 12u（`Space(3)`）、标签与值同列、值不压标签 —— 几何没有问题。

## 2. 为什么 560/460 不可达（两条独立事实，任一条够）

1. 设置面板是**固定面板**：`internal/shell/shell.go:4743-4745` 的 `targetSize` 对
   `SurfaceSettings` 一律返回 `launcher.InputWindowWidth`（`internal/launcher/launcher.go:30` = 720），
   只有 `SurfaceTerminal` 读 `s.TerminalWidth`；`shell.go:1316-1320` 写明「launcher 与 settings 是
   固定面板，只有终端可 resize」。`s.UIScale` 只改**高度**（`SettingsHeight`）。
2. 界面语言只有两种：`internal/i18n/i18n.go:1303-1310` 的 switch 只有 `ZH` 与默认 en，
   zh 的同一句是 `成功但有警告`（更短）。

## 3. 落地

| # | 位置 | 内容 | 台账 |
|---|---|---|---|
| 129-1 | `internal/settingsui/freshnessanswer_test.go`（新，142 行） | `TestTheFreshnessAnswerFitsTheCellItIsDrawnIn`：en/zh × 深浅 × 五词，在 `launcher.InputWindowWidth` 那一帧上，墨跨必须 == 1600u 下的自然宽，格盒不窄于自然宽 | 新 pin |
| 129-2 | 同上 | `TestTheFreshnessGridGivesItsAnswerRoom`：最长答案余量 ≥ 自身宽 1/4（实测 114.5/129 = 0.89），标签与值同列、值不压标签 | 新 pin |
| 129-3 | `freshnessSection` / `metadataCell` | 零像素改动 | 不变 |
| 129-4 | 窄窗换行 / 两格堆叠 / 标签让位 | **不做**（不可达；多一行 = 多一层高度，正是「空间感」的反面） | 故意不做 |
| 129-5 | 面板可缩放 / 允许更长的结果词 | 窗口模型与 i18n 文案是行为契约 | 需拍板 |

pin 里的宽度取 `launcher.InputWindowWidth` 而不是字面 720，尺子跟着常量走：谁改窄面板、谁把句子
写长、谁把字阶提一档，这里先红。

## 4. 台账与门槛（实测）

- **V 0 / B 0 / S 0**：本轮不碰任何既有断言，只新增测试文件。
- `gofmt -l cmd internal` → 空；`go vet ./...` / `GOOS=windows` / `GOOS=darwin` → 净；
  三平台 `go build ./...` → 过。
- `go test -count=1 ./...` → 全绿；**3406 PASS / 0 FAIL / 190 SKIP**，测试函数 2572 → **2574**
  （基线 P13-fg：3404 / 0 / 190 / 2572）。

### mutation（实跑，原始报错抄在下面）

**M1**：`internal/i18n/copy.go:704` 的 en `FreshnessResultDegraded` → `"Succeeded with warnings from the last probe run"`：

```
--- FAIL: TestTheFreshnessAnswerFitsTheCellItIsDrawnIn
    freshnessanswer_test.go:105: [en dark=false] the degraded cell is 243.5u wide but its answer
    "Succeeded with warnings from the last probe run" needs 247.0u: the settings panel is a fixed
    720u and this is the width it is drawn at
--- FAIL: TestTheFreshnessGridGivesItsAnswerRoom
    freshnessanswer_test.go:144: [dark=false] "Succeeded with warnings from the last probe run"
    leaves only -3.5u of its 243.5u cell free (needs a quarter of the word's 247u)
```

（两套外观各一条，共 4 处红。）

**M2**：`metadataCell` 的值字号 `t.FontSize` → `t.FontSize*2`（自然宽 252u）：

```
--- FAIL: TestTheFreshnessAnswerFitsTheCellItIsDrawnIn
    freshnessanswer_test.go:105: [en dark=false] the degraded cell is 243.5u wide but its answer
    "Succeeded with warnings" needs 252.0u
--- FAIL: TestTheFreshnessGridGivesItsAnswerRoom
    freshnessanswer_test.go:144: [dark=false] "Succeeded with warnings" leaves only -8.5u of its
    243.5u cell free (needs a quarter of the word's 252u)
```

还原：`md5sum -c /tmp/p13fh.md5`（`internal/i18n/copy.go` = `026742f7…`、
`internal/settingsui/settingsui.go` = `6577d45f…`）两文件「成功」，随后 `touch` 两文件复跑
`-run TestTheFreshness` → 全绿（避免测试缓存复用）。

## 5. 出图

`/tmp/p13fh/after/floter-P13-fg-freshness-{light,dark}.png`（复用 P13-fg 的 opt-in dump，
`FLOTER_P13FG_DUMP=1`）。本轮**零像素改动**，所以没有 before/after 差分可看（diff 必为 0）——
图在这里是**证据**：深浅两套上，`Succeeded with warnings` 整句都在，尾巴是真正的字形而不是
省略号；右侧余量一眼能看出比句子本身宽。

## 6. 「偷了什么」（qiaomu-design / craft-loop.md 工艺密度菜单）

- **第 8 项「流体字号 clamp(min, vw, max)」——反用，落成三个具体数值**：菜单假设容器宽度会变，
  所以字阶要 clamp；本仓设置面板宽度只有一个值（720u，`shell.go:4743`），`UIScale` 只改高度。
  于是 clamp 在本仓是死代码，本项的真实边界值是 **格 243.5u / 最长答案 129u / 余量 114.5u（89%）**，
  已写进 §6.129 的表。这条边界值就是本轮从菜单里取到的东西：它把「要不要上 clamp / 换行」
  从审美问题变成了算术问题。
- 第 1 项（氛围层）、第 5 项（多层染色阴影）、第 12 项（材质细节）：本轮不适用——**零像素改动**，
  没有面可上材质。故意不硬凑，理由写在这里而不是静默跳过。
- `craft-loop.md` §三「两轮制」照跑：第一轮量 → 四视角评审（工程师视角的发现就是这条切点）→
  第二轮只做「钉住它」这一件事，没有推倒重做。

## 7. preflight 逐条

- **A 反套路**：本轮不新增任何视觉元素，A 组 12 条无新增违规。既有的 `#000000` 纯黑 / 字体 /
  斜体三项由既有 pin（`noitalic_test.go` 等）继续守着，本轮全绿。
- **B 设计读取一致性**：读取不变——「设置页 · 集成页抽屉 · 轻便液态玻璃 · 功能性 UI」；
  本轮不改主题、不改明暗、不改字阶，一致性成立。记忆点仍是「一块格子同时说时间与结果，
  但两种值各有各的档」（§6.128）。
- **C 动效抽查**：本轮无动效改动；该块没有任何 transition。C 组不适用（不是跳过：没有新增动效可查）。
- **D2 中文页追加项**：**不过**，原因沿用：正文/值那一档是 `t.FontSize` = 10.8px、标签 `Caption`
  = 9.9px，低于 D2 的「正文 ≥ 14px」。这是**全局字阶**问题（与 preflight 的 14px 门槛冲突），
  已在 `.pending-bugs.md`「需拍板」里挂着（第 75 份证据：9.8px 的字描边够不到自己的 token 墨）。
  本轮不擅自改字阶——改了整仓 pin 全红，且属需拍板项。zh 侧其余三项（系统字体栈、行高、
  字重 400/500/600）成立；中西文间距本轮不涉及新文案。

## 8. 仍待

1. **禁用 / 只读面**（「状态反馈」的另一半，15 个 `Disabled(true)` 调用点）：目前禁用只靠
   `Cursor(ui.CursorDefault)` + 各自的墨，没有统一的「禁用面」档（`theme.Tokens` 里没有
   `Disabled`，`ui` 也没有 `Disabled()`）。**先量**：挑 `rows.go:568-588` 那枚「建时就禁用」的
   胶囊，量它现在与可用态差几级，再决定要不要立一个 `DisabledInk` 档。
2. 空态满态：只剩**空态**的密度。
3. `docs/liquid-glass.md` §6 差距表仍无 `未开始` 行。
