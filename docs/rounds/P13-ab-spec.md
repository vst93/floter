# P13-ab 规格 · 指针状态的材质账（arm 级）+ kill-confirm 药丸补齐对侧

## 意图一句话

`docs/liquid-glass.md` §2 只有一句话说「这块面能被指针碰」：**顶亮一分、底暗一分**。
P13-p 的材质 census 按「面 **+ 边**」找面，P13-aa 又发现「**没有边**的面」根本不在视野里；
这一轮把账补到**指针状态**这一层：**`Hovered()` / `Pressed()` 臂里出现的面，也必须有一对线**。
量完顺手修掉量出来的那一处真缺口（会话页的 kill-confirm 药丸只有顶边、没有对侧）。

## 为什么现在做

上一轮报告「仍待」第 1 条就是这条账，并且写明「下一轮先量再改」。先量了：

- 全仓 `Hovered()` / `Pressed()` 命中 **51 处**（`internal/` 非测试代码）。
- 按**函数体**扫（面标记 × 一对线标记）得到 **25 个函数**，**25 个都有**一对线——所以
  「函数级」的反向判据是空账，**必须落到臂级**。
- 按**臂级**扫（`case *.Hovered():` / `if *.Hovered() {` 的块体，加上「同一函数在臂之外的
  别处画了这一对」这条逃生口）只剩两处真缺口：
  1. `internal/control/control.go:131` `SegmentedIndex` 的 `case segment.Hovered():` ——
     凹槽里未被选中的那一格只有一层 `ControlHover` 洗色；
  2. `internal/settingsui/rows.go:906` `sessionKillConfirm` 的 `else if pill.Hovered():` ——
     药丸**只有顶受光边**（`rows.go:905` 的 `InsetShadow(0, 1, 0, 0, tokens.LensRim)`），
     没有对侧；这与 P13-aa 修掉的 execute 键是**同一种半对**。
- 顺带证明这条账不是空转：P13-aa 四处（`output.go:517` / `power.go:191` / `power.go:200` /
  静息透明面）在**修之前**全部会被这条新判据抓到（臂里 `ControlHover`/`IconSurfaceHover`
  有面、函数体内既无 `LitFace` 也无别的一对线）。报告里有实跑回测。

## 逐条落地项

### A. 新账：`internal/ci/litarm_contract_test.go` → `TestEveryPointerStateFaceNamesItsPair`

- 扫描 `../control` `../launcher` `../settingsui` `../shell` `../terminalui` 的非测试 `.go`。
- `go/ast` 取每个 `FuncDecl`；在函数体里找**指针臂**：`case X.Hovered():` / `case X.Pressed():`
  的 `CaseClause` 与以 `X.Hovered()`/`X.Pressed()` 为条件的 `IfStmt`。
- 臂体里出现**面标记**（`ControlHover` / `ControlPress` / `IconSurfaceHover` / `RaisedWarm` /
  `SurfaceHover` / `SurfacePressed` / `RowHover` / `RowSelected` / `Background(`）⇒ 这个臂说了
  「我能被碰」。
- **一对线**的判据是**严格**的：`LitFace(` / `.Lens(` / `LensInset(` / `InsetShadow(0, 1, 0, 0`。
  裸的 `InsetShadow(0, 1, 0, 0, …)` **不算**（那就是 P13-aa 的 execute 键：只有顶边；
  P13-p 的 `facecensus` 把它算成 pair，正是第二个盲区）。
- 满足条件：线在**臂内**，或在**同一函数里臂之外的别处**画了一次（`power.go` 的
  `if plane.A > 0 { LitFace }`、`cardRow` 的 `if face.Lit { LitFace }`、`shortcutRecorder`
  的序言 `LitFace` 都是这个形状：一份面，所有状态共用一对线）。
- 都不满足 ⇒ 必须在 `pointerFaceAllow` 里有理由；允许表里的条目**如果已经满足了**就报
  「条目过期」并变红（与 `facecensus` 同一条纪律）。
- 扫描到的臂少于 20 条 ⇒ 直接 `Fatal`（扫描自己坏了，不是包变干净了）。

允许表（本轮量完只剩一条）：

| 条目 | 理由 |
|---|---|
| `control.SegmentedIndex` | 凹槽里未被选中的那一格：`ControlHover` 是参考自己的 `.settings-option:hover` 平涂；给它一对线会读成「抬起来的一格」，而那正是**选中格**（`Thumb` + `Lens` + accent 环，P13-r）独占的一句话。一个凹槽里只有一格能站起来（§6.2b 9-11）。 |

### B. 产品：`sessionKillConfirm` 补齐对侧

- 位置：`internal/settingsui/rows.go:893-910`。
- 现状：`pill.Shadow(0, 0.25u, 0.5u, 0, ShadowContact)` + `pill.InsetShadow(0, 1, 0, 0, LensRim)`
  ——**半对**（只有顶边）。悬停臂 `pill.Background(tokens.IconSurfaceHover)` 只换面。
- 应改成：`tokens.LitFace(pill)`（顶 `LensRim` + 底 `LensEdge`），`Shadow` **保留**——
  这块暖色抬升面的影是它身份的来源（`--glass-raised-shadow`），与 P13-aa 的 execute 键
  同一条判据。警告色 `Border(1, t.Warning)` 不动（它是「不可逆」的身份线，不是 E1 的边）。
- 行为契约零改动：不改高度链（`MinHeight(7u)`）、不改文案、不改 `disabled` 的 0.4 深度、
  不改点击接线。

### C. 像素 pin：`internal/settingsui/killconfirm_test.go`

`TestTheKillConfirmPillSitsOnAnE1Pair`（深浅两套外观各跑一遍，静息 + 悬停两个状态）：

- 借用现成的 `lensProbe` + `pressProbeWith` 的指针位置（`(35,35)` = 药丸中心）。
- 读数：面 = 药丸中线行；冠 = `box.Y+1..Y+2`（在 1px 警告边之内，`LitFace` 的 `y=1`）；
  对侧 = `box.Y+box.H-3..H-2`。
- 断言：冠 **亮于**面、对侧 **暗于**面（`liteBrighter` / `liteDarker`），且**四个状态**都成立
  （静息 / 悬停 × 浅 / 深）；悬停档还要求面本身比静息**更亮**（`IconSurfaceHover` 确实抬了面）
  或者更深，方向不钉死，只钉「换了一档」（`channelDelta ≥ 2`）。
- 不是 `lensAssertE1`：那块药丸**有** `ShadowContact`（§2 判据 2 对 E1 禁影，但暖色抬升面
  的影是身份的一部分，P13-aa 已判过），所以「外面不许有影」那条不适用，本轮不假装适用。
- 出图：`FLOTER_PNG=1` 时写 `/tmp/floter-P13-ab-killconfirm-<appearance>-<rest|hover>.png`。

## 门槛命令 + PASS 基线（本轮实测起点）

```
gofmt -l cmd internal                                  # 期望：空
go vet ./...                                           # 期望：净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 期望：净
go test -count=1 ./...                                 # 期望：全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 起点 3010
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

起点（P13-aa 收尾，实测）：PASS **3010** / FAIL **0** / SKIP **84** / 测试函数 **2221**。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. **M1**：`rows.go` 把 `tokens.LitFace(pill)` 改回 `pill.InsetShadow(0, 1, 0, 0, tokens.LensRim)`
   → 新 pin 的「对侧暗于面」失败（四个子测试全红），且新 census **不会**红（臂外的半对不算一对线，
   但 census 判的是「有没有一对线」——这条要实测确认，若 census 也红则一并报告）。
2. **M2**：`control.go:132` 给 `case segment.Hovered():` 补上 `tokens.LitFace(segment)`
   → 新 census 报「`control.SegmentedIndex` 的允许表条目过期」并变红（证明允许表不是摆设）。
3. **M3**（回测，证明这条账能抓到 P13-aa 那四处）：把 `output.go:517` 的
   `button = tokens.LitFace(button.Background(tokens.ControlHover))` 还原成
   `button.Background(tokens.ControlHover)` → 新 census 报该臂没有一对线。

V/B/S 台账：本轮**不改任何既有 pin**；新 pin 全在本轮新增文件里。
