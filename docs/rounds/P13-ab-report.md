# P13-ab 报告 · 指针状态的材质账（臂级）+ kill-confirm 药丸补齐对侧

规格 `docs/rounds/P13-ab-spec.md`。`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律走通用
队列的「状态反馈」；对象是上一轮报告「仍待」第 1 条（并且它自己写着「下一轮先量再改」）。

**这轮为什么让它更轻 / 更有空间感**：这套语言里「这块面能被指针碰」只有一句话——顶亮一分、
底暗一分。会话页的「结束会话」药丸此前**只说前半句**（只有顶受光边），于是它读起来像
「贴在卡片上的一层暖色纸」；补上对侧之后它与电源条的 execute 键、每一个 E1 面说同一句话。
同时这一轮给这条语言加了一把**尺子**：以后指针状态里出现的面，臂里没有一对线就直接红。

## 落地

### A. 新账：`internal/ci/litarm_contract_test.go`

`TestEveryPointerStateFaceNamesItsPair`，扫 `../control ../launcher ../settingsui ../shell
../terminalui` 的非测试源码，`go/ast` 取指针臂（`case X.Hovered():` / `case X.Pressed():` /
`if X.Hovered() {`），臂体里出现面标记即要求一对线。三条判据写在文件头：

1. **一对线是严格的**：`LitFace(` / `.Lens(`。裸的 `InsetShadow(0, 1, 0, 0, …)` **不算**——
   那正是 P13-aa 的 execute 键（只有顶边），而 P13-p 的 `facecensus` 把它算成 pair，所以
   「半对」从来没进过视野。
2. **逃生口只有一条**：同一个函数在**臂之外**画了一次那一对线（`power.go` 的
   `if plane.A > 0 { LitFace }`、`cardRow` 的 `if face.Lit { LitFace }`、`shortcutRecorder`
   的序言）。`case` 子句一律**不算**共享——一个 switch 的不同分支是互斥状态：
   `SegmentedIndex` 里选中格的 `Lens` 从来没盖到 `segment.Hovered()` 那一格上，这正是这条账
   要抓的洞。
3. 都不满足 ⇒ 允许表写理由；条目**已经满足**了就报「过期」并变红。

允许表 3 条（本轮量完只剩这些）：

| 条目 | 理由 |
|---|---|
| `control.SegmentedIndex/Hovered` | 凹槽里未被选中的那一格：`ControlHover` 是参考自己的 `.settings-option:hover` 平涂；一对线会读成「抬起来的一格」，那是选中格（`Thumb` + `Lens` + accent 环，P13-r）独占的一句话（§6.2b 9-11）。 |
| `launcher.rowControl/Pressed` | **需拍板**（P13-aa 仍待第 2 条）：按下档的 `ControlPress` + `Scale(0.92)` 要不要保留一对线。设置页同族的 P13-q 保留了，启动器这两枚没有——两条路都有先例，本轮只记账。 |
| `launcher.fieldSettingsButton/Pressed` | 同上，同一个 28u 小格。 |

**先量再改的证据**：函数级是空账（25 个带指针臂的函数，25 个都有一对线）；落到臂级才剩真缺口
（下面 B）。另加回测（见「变异」M3）。

### B. 产品：`internal/settingsui/rows.go:893-916` `sessionKillConfirm`

- 改前：`pill.Shadow(0, 0.25u, 0.5u, 0, ShadowContact)` + `pill.InsetShadow(0, 1, 0, 0, LensRim)`
  ——**半对**。
- 改后：`tokens.LitFace(pill)`（顶 `LensRim` + 底 `LensEdge`），`ShadowContact` 保留
  （`--glass-raised-shadow` 是这块暖面的身份；§2 判据 2 管的是中性 E1），警告色
  `Border(1, t.Warning)` 不动（「不可逆」的身份线）。
- 悬停臂只换面（`IconSurfaceHover`），一对线四个状态共用——正好是新账要求的那种形状。
- 高度链、文案、`disabled` 的 0.4、点击接线**零改动**。

## 实测（像素，同一帧同一列；探针地面 128 灰）

| 状态 | 外观 | 面 | 冠（Y+1..Y+2） | 对侧（H-3..H-2） |
|---|---|---|---|---|
| 静息 | 浅 | 139,126,117 | 144,132,123（+5） | **135,122,113（−4）**（改前 = 面） |
| 悬停 | 浅 | 146,126,110 | 151,132,116（+5） | **141,122,107（−5）**（改前 = 面） |
| 静息 | 深 | 142,133,128 | 147,138,133（+5） | **128,119,115（−14）**（改前 = 面） |
| 悬停 | 深 | 153,137,128 | 157,142,133（+4） | **137,123,115（−16）**（改前 = 面） |

「改前 = 面」是逐像素相等：改前 `[light/rest]` 的 face 与 shade 都是 `{139 126 117 255}`、
`[light/hover]` 都是 `{146 126 110 255}`。

出图：`/tmp/floter-P13-ab-killconfirm-{light,dark}-{rest,hover}-{before,after}.png`。
像素 diff 的 bbox 四张图完全一致：`(21,26)-(118,44)`——落在药丸自己的盒子
`(20,20)-(118,45)` **之内**，没有一根像素漏到盒子外（这一块**有**影，但它改前改后都在，
diff 不是影）。

**图上看到的一处变化**：`-dark-rest-after.png` 里药丸下沿从「与面同色」（142,133,128）
变成一条暗线（128,119,115）——暖色药丸从「贴在卡片上的色块」变成「立在卡片上的一块面」，
而同一帧里顶沿那条受光边一像素没动（改前就有）。

## 门槛（全部实测）

- `gofmt -l cmd internal` 空 ｜ `go vet ./...` 净 ｜ `GOOS=windows/darwin go vet ./...` 净 ｜
  三平台 `go build ./...` 过
- `go test -count=1 ./...` 全绿；PASS **3010 → 3019（+9）**、FAIL **0**、
  SKIP **84 → 85（+1 opt-in dump）**、测试函数 **2221 → 2225（+4）**
- **V 0 / B 0 / S 0**：没有一条既有 pin 需要改断言（这条语言此前没有指针臂级的账）

## 变异（3 条，全部实跑变红，`md5sum -c` 逐字节还原，`touch` 后复跑绿）

- **M1**（产品）：`rows.go` 把 `tokens.LitFace(pill)` 退回 `pill.InsetShadow(0, 1, 0, 0, tokens.LensRim)`
  → 新 pin 红：
  `[light/rest] the rows inside its bottom frame are {139 126 117 255}, not darker than its face {139 126 117 255} — E1 is a pair (顶亮一分、底暗一分); this pill used to carry only the lit half`
  （`light/hover` 同一条，深色两条同形）；**并且**新账也红（`never a pair` 命中 1 处）——这一条
  同时证明「半对」不会被 census 漏掉。
- **M2**（允许表）：给 `control.go` 的 `case segment.Hovered():` 补上 `tokens.LitFace(segment)`
  → 新账报
  `../control/control.go:131:4: the allowlist entry "control.SegmentedIndex/Hovered" is stale — this arm now names its pair (§6.2b 第 9-11 行…)`，
  允许表不是摆设。
- **M3**（回测 P13-aa）：把 `output.go:517` 的
  `button = tokens.LitFace(button.Background(tokens.ControlHover))` 退回
  `button.Background(tokens.ControlHover)` → 新账报
  `… and never a pair — not in the arm, not once for every state elsewhere in the function`。
  也就是说 P13-aa 那四处**在修之前**全部会被这条判据抓到（`power.go` 的 execute 键因为只有
  顶边，也在其中——`facecensus` 当时把它算成了 pair）。

## 偷了什么（qiaomu-design · `craft-loop.md` 工艺密度菜单）

- **第 12 项「材质细节 · 内边框高光 / glass 折射边」** → 落成**对侧**那半句：
  `LitFace` 的 `InsetShadow(0, -1, 0, 0, LensEdge)` 是 1px **实色**线，实测深色档药丸下沿
  142 → **128（−14）**。不是渐变白边（§1 禁令 3）。
- **第 6 项「编辑细节 · hairline」** → 反过来用：药丸外圈那条 `Border(1, t.Warning)` **保留不动**，
  它是「不可逆」的身份线；E1 的一对线画在它**里面**一行，两条线各说一件事（身份 / 可碰）。
- **第 5 项「多层染色阴影」** → `ShadowContact` 那一层保留、**不新增第二层**：§2 判据 2 说面板内
  E1 不画外投影，这块暖面是例外（影是它身份的来源），多一层就变浮层。实测 diff 的 bbox 没有
  一像素落在盒子外——影没长大。
- **第 4 项「签名交互」**：本轮**不适用**（零动效改动，如实记录；悬停仍是 120ms 颜色过渡）。

## preflight.md 逐条

- **A 反 AI 套路**：过。受光边/对侧都是 1px 实色线；无 glow、无渐变白边、无紫蓝渐变；
  无斜体（用户偏好 P-1，本轮零文案改动）；没有新增装饰性卡片。
- **B 设计读取一致性**：过。产品只读 `theme.Tokens`，一个字面色值都没写；新账的判据读的是
  **符号名**（`LitFace(` / `.Lens(`），不是数值，所以 token 再调值不会让账失真。
- **C 动效抽查**：过。pin 在 `Advance(300ms)` 之后读数（settled），量到的是材质不是过渡中间帧。
- **D2 中文页**：过。文案零改动（`SessionsKillArm` 一个字没碰），中文排版零变化。

## 仍待（P13-ab，追加在 P13-aa 之后）

1. **按下档要不要保留一对线——需拍板**（P13-aa 第 2 条，本轮把它写进了允许表而不是偷偷决定）。
   现状：设置页 P13-q 保留了（按下仍戴一对线），启动器 `rowControl` / `fieldSettingsButton`
   只换面 + `Scale(0.92)`。拍板后删允许表那两行即可。
2. **新账的窄口**：它只认 token 名（`ControlHover` / `ControlPress` / `IconSurfaceHover` /
   `RaisedWarm` / `inks.Wash`）。行悬停（`RowHover` / `SurfaceHover`）**故意不进账**——行不是控件，
   参考自己的 `:hover` 就是一层平涂（P13-aa 判过）。如果以后有人把一行做成「面」，这条账看不见它。
3. `control.SegmentedIndex` 的悬停格在深色档与选中格**同面**（`Thumb` dark = `ControlHover` dark
   = 0.14 白），只有 accent 环分得开——要不要给悬停格第三档**需拍板**（P13-r 第 1 条的邻居）。
4. P13-aa 第 3 条、P13-z 第 1/2 条、P13-y 第 3/4 条、P13-x 第 1-3 条、P13-w 第 2/4 条、
   P13-u 第 1/2/3 条、P13-t 第 3 条、P13-s 第 1/2 条、P13-r 第 1 条、P13-q 第 1 条、
   P13-p 第 1/2 条、P13-k 第 1/2 条、P13-c 四条、P13-h 第 9 条原样有效（均需拍板）。
5. **抢救 stash**：`stash@{0}` = `df5647edab`、`stash@{1}` = `567cf0a916`，两条 `tree`
   **逐字节相同**（`9e302887…`），`git diff stash@{0} 4783c62d` 只剩 `docs/rounds/P13-x-spec.md`
   与 `internal/launcher/headingfold_test.go`（P13-x 提交里的测试与文档），生产改动已被 P13-x
   完整覆盖、**无可接续**。本轮复核 `git stash list` 索引未变。请在任务书里把这两行标
   **「已回收 —— P13-ab」**。
