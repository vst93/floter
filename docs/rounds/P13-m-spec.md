# P13-m 规格：设置页三块「`Control` 平涂 + 一圈画框」的面补上 E1 的一对面

- 分支 `mygo-rewrite`，起点 `0b3b7d67`（P13-l）。
- 总目标：细节打磨 + 轻便液态玻璃 + 空间感（2026-10-30 用户定）。
- 合约行：`docs/liquid-glass.md` §6.2b-2 新增 9-15 / 9-16 / 9-17。

## 意图（一句话）

设置页上还留着三块「`Control` 平涂 + 一圈画框、没有内线」的面——它们与 P12-c 第 2-4 行
点名的 `fieldFace` 是同一族（同一个应用里字段/块只有一种材质），却停在升代之前的样子，
读起来是「卡上贴的纸片」而不是「站起来的块」；这一轮把 E1 的那一对线补上。

## 为什么现在做（证据）

P13-i（十处 `Lens`）与 P13-l（两处漏网）用的都是**单行** grep：

```
grep -n "tokens.Control)" internal/settingsui/*.go | grep "Border(1"
```

这三处的 `Border(...)` 与 `Background(tokens.Control)` **不在同一行**（一个在前一行、
一个在后一行），所以两轮都照不到它们——不是「判过之后决定不做」，是**从来没进过视野**。
这就是「细节打磨」队列里「层级一致性」那一格的残余。

| 位置 | 现状 | 与合约的差距 |
|---|---|---|
| `internal/settingsui/config.go:834-840` `configMultiselectBox` | `Border(1, ControlEdge)` + `Background(Control)` + `Clip()`，**没有内线** | 它自己的注释（`config.go:815-817`）写着「与 `fieldFace`/`sheetFieldSlot` 同面……同一个应用里字段只有一种材质」，而 `fieldFace`（`textinput.go:81`）自 P12-c 起就是 `tokens.LitFace(...)`——**注释在说一件代码没做的事** |
| `internal/settingsui/settingsui.go:3070-3076` `commandPlate` | `Background(Control)` + `Border(1, Hairline)`，**没有内线** | 它是扩展页命令列表里**每一条命令**的块（`.extension-command-list__row`），与 §6.2 第 2-3 行的分组卡同档（E1 面）；分组卡有那一对线，它没有 |
| `internal/settingsui/textinput.go:132-146` `aliasField` | `Background(Control)` + `Border(1, Hairline)`，**没有内线** | 它是面板字段族里**唯一**没有 E1 一对线的字段。它的注释说「它是 inline command-chip，不是 settings slot」——那是**边框**的选择（`Hairline` 而不是 `ControlEdge`），不是「不要受光边」的理由：受光边是**内线**，与画框是两件事 |

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| L1 | `config.go:834-840`（`configMultiselectBox` 的 `box`） | `ui.Column(...).Padding(...).Border(1, ControlEdge).Radius(RadiusSM).Background(Control).Clip()` | 建好之后 `box = tokens.LitFace(box)`。**`Clip()` 必须留在 `LitFace` 之前**：`InsetShadow` 画在 padding box 内侧，`Clip` 只是把滚动内容裁到圆角里；顺序不影响内线，但保持「先定形状、后上墨」的读法 |
| L2 | `settingsui.go:3070-3076`（`commandPlate` 的 `plate`） | `... Radius(RadiusSM).Background(Control)`，随后 `plate.Border(1, Hairline)` | `plate = tokens.LitFace(plate)`（放在 `Border` 之后、`Opacity` 之前）。`Opacity(0.62)` 的停用臂**不动**：透明度是整块的状态，不参与材质 |
| L3 | `textinput.go:146`（`aliasField` 的返回值） | `return e.Background(tokens.Control).Border(1, tokens.Hairline)` | `return tokens.LitFace(e.Background(tokens.Control).Border(1, tokens.Hairline))`。外圈 `Hairline` 一个字节不动 |
| L4 | `docs/liquid-glass.md` §6.2b-2 | 9-13 / 9-14 / 9-14b | 新增 9-15 / 9-16 / 9-17，并在节首写清「这一族是**多行写法**漏在单行 grep 框外」 |
| L5 | `internal/settingsui/multilineface_test.go`（新） | — | 三条像素 pin（每条深/浅两套外观，走 `lensAssertE1` 的四条判据）+ 一条 opt-in dump |

## 行为契约零改动红线

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**。
- 几何零改动：`MinHeight(66u)`、`RadiusSM`、`Border(1, …)`、`Clip()`、`Opacity(0.62)`、
  `Padding` 对全部保留。`LitFace` 只加两条 1px **内线**，不占布局。
- 三处的外圈线（`ControlEdge` / `Hairline` / `Hairline`）**不改色**：受光边是内线，画框是外线，
  §1 的「受光边不是画框」这条不变量本轮正好被这三块（两种画框）同时验证。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（P13-l 收尾，本轮开工时实测）：**PASS 2932 / FAIL 0 / SKIP 70 / 测试函数 2161**。

## mutation 计划（每条说明会让哪个 pin 变红）

| # | 注入 | 期望 |
|---|---|---|
| M1 | 删掉 `configMultiselectBox` 的 `LitFace`（回成 P13-l 之前的写法） | `TestTheMultiselectBoxIsAFaceNotARing` 红：顶内行 == 面 |
| M2 | 删掉 `commandPlate` 的 `LitFace` | `TestTheCommandPlateIsAFaceNotARing` 红：同上 |
| M3 | 删掉 `aliasField` 的 `LitFace` | `TestTheAliasFieldIsAFaceNotARing` 红：同上 |
| M4 | 给 `configMultiselectBox` 补一笔 `Shadow(0, 4, 8, 0, ShadowContact)`（E1 不许投影） | 该 pin 的「块外仍是地」判据红 |
| M5 | 把 `commandPlate` 的 `LitFace` 换成 `tokens.Lens`（四层环回来） | 该 pin 的「两侧列 == 面」判据红 |

每条：注入 → 看它真的红 → `git checkout -- <file>` 还原 → `git status --porcelain` 为空 →
`touch` 后再跑一次（避免缓存复用）。原始报错串抄进报告。

## 出图

`/tmp/floter-P13-m-faces-{light,dark}.png`：每张六行，同一个面上并排「画框之前」与
「E1 之后」的三块（多选箱 / 命令块 / 别名框），同一块地面、同一个尺寸，只有框内那一对线不同。
