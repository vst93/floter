# P13-gd · 规格：受光边只有一个来源（并把「下一个表面」这条幻影选题销掉）

## 一、意图与「为什么现在」（证据）

**意图**：给 `docs/liquid-glass.md` §3 的那句话（「受光边是 E1 的结构线，不是装饰」）
补一条**可执行的**守卫——**一条受光边/对侧线的颜色只能来自 token，不许在视图里手写**；
同时把上一轮报告「仍待」第 2 条点名的那个表面**证伪**（它在生产代码里已经不存在）。

**为什么现在**：选题纪律顺序是 0（§6 差距表 `未开始`）→ 1（用户实测）→ 2（合约未落地）
→ 3（上轮「仍待」）→ 4（通用队列）→ 5（结构性小活）。

- 0 空：`grep -c 未开始 docs/liquid-glass.md` 的 16 处命中全在**散文**里（§6 表的说明与
  历史小节），表里**没有** `未开始` 行（P13-br 起就空着）。
- 1 空：`.pending-bugs.md` 顶部「⚠️ 用户实测反馈」没有新条目。
- 2 空：§5/§6 的注脚里写着「下轮」的项，全部已落到 P13-f* / P13-g* 各轮。
- 3 **只剩拍板项**：P13-gc 报告的「仍待」第 1 条是 `140-5`（`需拍板`）；
  第 2 条点名 `RaisedQuiet` 的面差 —— 本轮先量它，结论是**它已经没有生产调用点**（见二）。
- 4 通用队列的每一格都已有专轮（间距节奏 / 状态反馈 / 空态满态 / 文案密度 / 边界情况 /
  像素细节 / 三平台一致）。剩下的候选全部落在 `.pending-bugs.md` 的「需用户拍板」表里。
- ⇒ 落到 5：结构性小活，但只做**能证明「这一族到此为止」的那一条**。

## 二、先量：`RaisedQuiet` 是死 token（P13-gc「仍待」第 2 条的证伪）

```
$ grep -rn "RaisedQuiet" --include=*.go internal/ cmd/ | grep -v _test.go
internal/settingsui/custom.go:75:       // P8-b 重定基线：`RaisedQuiet` 不透面 → `Control`（…）
internal/settingsui/healthfaces.go:73:  // P8-b 重定基线（…）：`healthy` 曾是 `RaisedQuiet` + `AccentEdge`
internal/settingsui/healthfaces.go:221: // …`RaisedQuiet` + `AccentEdge` inset…
internal/settingsui/settingsui.go:2677:// P8-c 重定基线（…）：命名行的 `RaisedQuiet` + `AccentEdge` + …
internal/settingsui/settingsui.go:3983:  // P8-b 重定基线：`RaisedQuiet` 不透面 → `Control`（…）
internal/theme/theme.go:369            // 字段文档
internal/theme/theme.go:372            // 字段声明
internal/theme/theme.go:595            // `ForPlatform` 里的赋值
```

**五处非 theme 命中全是注释**，一条 `SelectorExpr` 都没有：P8-b / P8-c 把这四处面
换成了 `Control` / `RowSelected` 之后，`Tokens.RaisedQuiet` 已经**零生产调用点**
（只剩 `internal/theme/theme_test.go` 的值 pin 与 5 个 `_test.go` 里把它当**变异靶子**）。

于是 P13-gc 提议的下一格（「`RaisedQuiet` 在 `Sunken` 变厚之后的面差」）**没有可量的对象**：
它不是「一个还没量的面」，而是「一个已经不画的面」。按纪律不能靠猜，先记这一条事实。

## 三、先量：受光边的颜色来源（本轮要守卫的那一层）

```
$ grep -rn "InsetShadow(" --include=*.go internal/ cmd/ | grep -v _test.go | wc -l
43
$ … | grep "ui\.RGBA\|ui\.Hex"
internal/theme/theme.go:1357  e.InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.85))   # LensInset
internal/theme/theme.go:1358  e.InsetShadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))          # LensInset
```

43 条内线里，**41 条在视图层**（`internal/{shell,settingsui,launcher,terminalui,extensions}`），
**全部**用 token 取色；唯一的两个字面值在 `internal/theme` 的 `LensInset`——那是
**镜片栈本身**（凹槽的一对线），也就是 Token 层的**定义处**。

顺手量了视图层的 `ui.RGBA(` 总数：**14 处**（`terminalui` 的 8 个终端主题的
selection 色、`theme/elevation.go` 的 5 条投影、卸载对话框的 1 层遮罩），没有一处是
材质线——墨与线的账目目前是**干净**的。

⇒ 这条不变量今天成立，但**没有东西守着它**。下一轮谁在视图里手写一条
`ui.RGBA(255, 255, 255, 0.09)`（P13-ah 之前的浅色 `LensRim` 正是这个值），
就会在浅色档画出一条「看不见的受光边」，而且**跑测试全绿**——P12-a / P13-n / P13-ag /
P13-ah 四次重定基线，每一次都在治同一种病。

## 四、落地项（位置 → 现状 → 改成）

| # | 位置 | 现状 | 改成 |
|---|---|---|---|
| 1 | `internal/ci/litedgecolour_contract_test.go`（新） | 无 | AST 扫 `internal/`（跳过 `internal/theme/` 这一层定义）与 `cmd/` 的**生产**文件：每个 `.InsetShadow(...)` 的**颜色实参**不许是 `ui.RGBA(...)` / `ui.Hex(...)` / `ui.Color{...}` 字面量。反面：字面量 ⇒ 红，报 `file:line` 与实参源码。前置：扫到的站点数 ≥ 40（测不动时**大声红**，不是静默绿）。逃生口 `litedgeLiteralAllow`（空表；登记了却已不违规 ⇒ 报「过期」并红） |
| 2 | `internal/theme/theme.go:369-372` | 字段文档写着「RaisedQuiet is the sidebar's chosen page」，而四处调用点在 P8-b/P8-c 已改 `Control`/`RowSelected` | 加 `Deprecated:` 段：零生产调用点的实测证据、被谁取代、以及**为什么字段保留不删**（同 `AccentTintQuiet` / `AccentGlow` 的先例：视觉收敛轮不顺手删字段） |
| 3 | `docs/liquid-glass.md` | §3 只有散文判据 | 新增 **§6.149**（含 149-1…-4 台账：死 token、守卫、允许表、空变异说明） |
| 4 | `.pending-bugs.md` 顶部 | — | 本轮处理条目 + 把 P13-gc「仍待」第 2 条标成**已证伪** |

**产品像素零改动**：`git diff --stat` 里只有 `internal/theme/theme.go` 的**注释**行、
新测试文件与三份文档。

## 五、行为契约零改动红线

不动高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线；不动任何 token 数值
（本轮只标 `Deprecated` 与加守卫）。视觉 pin 一条都不该变红。

## 六、门槛命令与 PASS 基线（开工实测）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                     # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3462（P13-gc 实测）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线：**PASS 3462 / 0 FAIL / 函数 2629**（HEAD `5f5c2124`，本轮开工前实测）。

## 七、mutation 计划（≥2 条，实跑）

| # | 注入 | 预期变红 |
|---|---|---|
| M1 | 把 `internal/settingsui/rows.go` 里一条 `tokens.EdgeHighlight` 换成 `ui.RGBA(255, 255, 255, 0.24)`（值相同，只是离开 token） | 守卫 1 处红，报 `rows.go:<行>`；**像素本来不变**，所以只有新守卫能抓到 |
| M2 | 在 `internal/shell/detach.go` 里加一条裸 `InsetShadow(0, 1, 0, 0, ui.RGBA(0, 0, 0, 0.06))` | 守卫 1 处红（同一函数两条 ⇒ 站点计数不变，违规数 +1） |

两条都要 `md5sum -c` 逐字节还原，再 `touch` 目标文件复跑一次（避免测试缓存）。
