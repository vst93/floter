# P13-p 报告：材质 census，不是函数名 census——设置页最后三块「有面无线」的面

规格：`docs/rounds/P13-p-spec.md`。合约：`docs/liquid-glass.md` §6.2b-4（新增）。

## 一句话

`docs/liquid-glass.md` §6 在 P13-o 之后没有 `未开始` 行，按纪律转通用队列的「层级一致性」格。
P13-i / P13-l / P13-m 三轮都是「把某几处控件面退成 E1 的一对面」，**但三轮的排查都是 grep**：
先是数 `tokens.Lens(`，后来是数 `Border(1,` 与 `tokens.Control)` 在不在**同一行**。
数「某个函数的调用」只能找到「已经这么写的」，找不到「本该这么写却没写的」——于是这一族
还剩三块面没戴那一对线，而它们对两种 grep 都不可见：一块从来没有 `Lens` 可数，
两块把面写进变量、把两条线分两行写。

**这一轮为什么让它更轻 / 更有空间感**：三块面此前都是「一块平涂 + 一圈画框」——顶内行、
面、底内行是**同一个像素值**（浅色 221/221/221、230/230/230；深色 48/48/48、39/39/40），
读起来是画在纸上的框，不是立在纸上的玻璃。补上一对线之后同一个盒子里第一次有算术：
浅色顶 +3、底 −13；深色顶 +19、底 −10。块的下边界第一次存在（P13-n 的结论：深色档读得出
的是**对侧**，浅色档读得出的是**顶线**）。

## 落地

1. **`internal/settingsui/rows.go` `sessionIconButton` 的 `Hovered()` 臂**（Sessions 页 28u
   方格）：追加 `tokens.LitFace(button)`。它的三个同胞——30u 的 `extensionIconButton`
   （P13-i 9-2）、28u 的 `paramIconButton`（9-3）、页头 28u 的 `headerButton`（9-4）——都戴了，
   只有它没戴。静息臂不动（透明面 + `Border(1, ui.Transparent)`）：受光边只在指针把面点亮
   时出现，与三个同胞完全同一写法。
2. **`internal/settingsui/savealert.go` `shortcutRejectedAlert`**（Shortcuts 页的拒绝条）：
   `tokens.LitFace(ui.Box(…))`。它与 `noticeBand` 的非危险臂（P13-l 9-14）是同一个形状
   ——一条 `Control` 平涂 + 一圈 `ControlEdge` 画框坐在页面地面上——那条已经戴了，这条没有。
   **不是 E2**：不浮、无影，所以不碰 `EdgeHighlight`。
3. **`internal/settingsui/pluginsheet.go` `sheetActionButton` 的中性臂**：
   `if plane.A > 0 { tokens.LitFace(button) }`。弹层里的一切是 E1（§2 判据 1），同族胶囊
   （9-1 / 9-5 / 9-6）都戴了。**`danger` 臂故意不戴**：它的面是 `ui.Transparent`，在一块
   透明的面上画 0.09 的白线就是 §1 禁令 3 的「类玻璃边」；指针把面点亮（`ControlHover`）
   之后它才戴——那时它真的是一块玻璃。反面 pin 守着这件事。
4. **把 census 本身换掉**：新增 `internal/settingsui/facecensus_test.go`——`go/ast` 取每个
   `FuncDecl` 的**函数体源码文本**，只要同时出现「`Control`/`ControlHover`/`ControlPress`/
   `Field`/`RaisedWarm`/`plane` 的面」与「`Border(1` 或 `InsetShadow(` 的边」，就必须出现
   `LitFace` / `.Lens(` / `LensInset(` / 直接的 `InsetShadow(0,1,0,0,…)` 之一，否则必须在
   `faceCensusAllow` 里写明理由。允许表 11 条，每条一句话写清是 §2 的哪条理由
   （4px 药丸 / 26~30u 方格 / 14u 勾选框是噪点；警告内环与 accent 环占住那一行）。
   **反向也钉着**：允许表里的函数开始戴一对线而表没删，同样红——表不许变成历史垃圾场。
   多行写法、变量面、跨语句的材质从此都躲不过去。
5. 合约：`docs/liquid-glass.md` 新增 §6.2b-4（9-18 / 9-19 / 9-20 / 9-20b）与 §6 尾部的
   轮次叙述。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...  /  GOOS=darwin go vet ./...  → 净
go test -count=1 ./...                                 → 全绿（FAIL 0）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2953
GOOS=linux/windows/darwin go build ./...               → 过
```

- PASS **2948 → 2953**（+5）| FAIL **0** | SKIP **73 → 74**（+1 opt-in dump）|
  测试函数 **2170 → 2176**（+6）| 基线为 P13-o 实测值。
- **V 0 / B 0 / S 0**：三处此前都没有 pin，所以没有既有断言需要改；既有 2948 条一条没动。

## 变异（6 条实跑，逐条抄原始报错串）

| # | 注入的缺陷 | 结果 |
|---|---|---|
| M1 | `rows.go:804` 删掉 `tokens.LitFace(button)` | 红：`the session icon button (light): the top inner row is not lighter than the face: {221 221 222} vs face {221 221 222}` / `(dark): the bottom inner row is only 0 levels under the face ({48 48 50} vs {48 48 50})` |
| M2 | `savealert.go:179` 把 `tokens.LitFace(ui.Box(c).` 换成 `(ui.Box(c).`（退回裸平涂） | 红：`the shortcut refusal band (light): the top inner row is not lighter than the face: {230 230 230} vs face {230 230 230}` + 三条同族 |
| M2b | 同处 `LitFace` → `Lens`（四层环回来） | 红：`(light): the top inner row is not lighter…` / `(dark): the bottom inner row is only 6 levels under the face ({39 39 40} vs {41 41 42})` ——环把 0.09 的受光边压掉了，与 §6.2b 9-1 的根因同一个 |
| M3 | `pluginsheet.go:391` `if plane.A > 0` → `if false &&` | 红：`the plugin sheet's action pill (light): … {230 230 230} vs face {230 230 230}` / `(dark): … only 0 levels under the face ({39 39 40} vs {39 39 40})` |
| M4 | 同处改成 `if true`（danger 臂也戴） | 红：`the danger pill (light) grew a counter-side shade: bottom row {232 232 233} over ground {247 247 248} (15 levels)` / `(dark) grew a lit edge: top row {36 36 38} over ground {14 14 16} (22 levels)` |
| M5 | `facecensus_test.go` 删掉 `packageChip` 的允许表条目 | 红：`rows.go.packageChip draws a control face and a ControlEdge border but no 1px inner line: paint tokens.LitFace (E1) or add a reason…` |

每条注入后 `cp` 还原并用 `md5sum -c` **逐字节核对通过**；改测试文件的那条（M5）同样核过。

## 像素（前 / 后，两套外观）

量法：`realPageGround` 取真页面的地面当底，画**真的控件**（不是照着画的替身），
在控件中间列读顶内行（`y+1`，取两行最亮）/ 面（`y+4`，避开字的行）/ 底内行（`y+h−2`，
取两行最暗）。

| 表面 | 浅色 before（顶/面/底） | 浅色 after | 深色 before | 深色 after |
|---|---|---|---|---|
| Sessions 28u 方格（悬停） | 221 / 221 / 221 | **224 / 221 / 208** | 48 / 48 / 48 | **67 / 48 / 38** |
| 快捷键页拒绝条 | 230 / 230 / 230 | **232 / 230 / 216** | 39 / 39 / 40 | **58 / 39 / 31** |
| 插件弹层 28u 胶囊 | 230 / 230 / 230 | **232 / 230 / 216** | 39 / 39 / 40 | **58 / 39 / 31** |

**图上看到的一处变化**（自己用眼睛复核的方式：逐像素 diff 的 bbox）：三张图的差异区域
**恰好是控件自己的盒子**——按钮 `(25,25)-(48,48)`（24×24 变化 74/78 px），两条带子
`(25,25)-(295,58)`（整宽，574/584 px）。控件之外**一个像素没动**：不是布局变化，是墨变化。

出图：
`/tmp/floter-P13-p-sessionbutton-{light,dark}-{before,after}.png`、
`/tmp/floter-P13-p-refusalband-{light,dark}-{before,after}.png`、
`/tmp/floter-P13-p-sheetpill-{light,dark}-{before,after}.png`（`before` 由 M1/M2/M3 的注入态
实拍，不是重画）。

## 用了 `qiaomu-design`（`craft-loop.md` + `preflight.md` 的「已有方向，只要打磨」车道）

**读取为**：桌面应用内嵌的设置面板，面向每天用快捷键开关它的普通用户，用「轻便液态玻璃」
的语言（半透明面 + 一层受光边 + 柔和外投影），倾向 **Apple 的材质分层**这一族。
拨盘（本车道不改方向，只记录现状）：`VARIANCE 3`（功能页，用户已把方向给死）、
`MOTION 4`（只有 90~120ms 的颜色过渡）、`DENSITY 6`（设置页是密集表单）。
功能契约：用户来这一页是为了**改一项设置**——3 秒内要看出「哪块能改、哪块是开的」，
所以层级的职责是回答「这块是面还是底」，不是好看。

### 偷了什么（逐条，取自 `craft-loop.md` 的工艺密度菜单）

| 菜单 # | 偷了什么 | 落成哪个具体数值 |
|---|---|---|
| **12 材质细节** | 「内边框高光」——受光边是**一条 1px 的实色线**，不是一片渐变 | `LensRim` = 白 0.09（顶内行），对侧 `LensEdge` = 黑 0.20（深）/ 0.06（浅）；实测浅色 232 / 深色 58 的顶内行 |
| **5 多层染色阴影** | 只偷「拒绝单层」这条**判据**，不偷它的实现：E1 面**不许有外投影**（§2 判据 2） | 三处都不加 `Shadow(`；`internal/ci/elevation_contract_test.go` 守着 |
| **2 定制 focus 环 / 3 品牌 focus 环** | 不偷：设置页的 `FocusRing` 已经存在且不属于本轮 | 三处的 `FocusRing`/`Border` 一个字节不动（`shortcutRecorder` 的 `FocusRing(true)` 保持） |
| **9 自定义滚动条 / 7 展示排版动作 / 8 流体字号** | 不适用：这是桌面应用的自绘面板，没有滚动条皮肤，字号链在 `docs/raycast-design.md` §2 钉着 | 本轮零字号、零布局改动 |
| `craft-loop.md` §三「两档看图」 | 1× 看整体 + **4× 局部**找「没落在任何东西上的线」 | 本仓没有浏览器，等价物是**逐像素行**读：顶内行 / 面 / 底内行三个值（表见上）。正是这一步抓到「顶 = 面 = 底」的三块平涂 |

`user-preferences.md` 的 P-17（自发现的设计问题必须自动沉淀）直接落成了
`facecensus_test.go`：这轮的根因是**排查方式**而不是三处代码，所以沉淀的是一条守卫，
不是三条一次性修补。

### preflight 逐条

- **A 反套路（AI Tells）**：通过。这一轮**只加 1px 实色线**，没有渐变、没有外发光、
  没有 `#000000` 纯黑面；反而把 §1 禁令 3 的「渐变白边模拟高光」再钉了一次
  （9-20b 的反面 pin：透明面上不许有受光边）。A 的字体/三等分卡片/Hero 等条目属于
  web 页面，本仓是 Go 自绘的桌面面板，不适用（如实记）。
- **B 设计读取一致性**：通过。方向由用户给定（轻便液态玻璃），本轮不改方向；两套外观
  （深/浅）各量一遍，不混主题；**记忆点**：这个应用里每一个「面」的顶边都有一条受光边、
  底边都有一道暗线——同一句话在三处新面、以及此前 12 处旧面上成立。
- **C 动效抽查**：通过（本轮零动效改动）。三处控件的既有 `Transition(Colors, 120ms)` 保留；
  受光边是静态墨，不参与过渡，所以指针进出不会出现「线在动」的假动效。
- **D2 中文页追加项**：通过（本轮零文案改动）。拒绝条用的仍是既有 i18n 字符串
  （`copy.ShortcutRejected`），中文行高/字重/中西文空格都在 `internal/i18n` 与字阶里，
  本轮一个字节没碰。

## 没做 / 留给下一轮

见 `.pending-bugs.md` 的 P13-p「仍待」。
