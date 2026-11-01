# P13-cn 报告：设置页最后两个「两档的可点面」

方向一句话：**按住不放的那一帧不再和悬停帧逐像素相同——手指按下去，界面回话。**
合约新增 **§6.57**（`docs/liquid-glass.md`）。

## 1. 选题与证据

通用队列 → **状态反馈** 的另一半（可点面）。上一轮 P13-cm 修的是「同一个结局被说成两件事」
（墨），这一轮修「按住与悬停长得一样」（面）。

先量账：`internal/{shell,settingsui,launcher,terminalui}` 里 `Hovered()` 有、`Pressed()` 没有的
函数，逐个看——只剩四个，其中两个是既有**需拍板**（`launcher/filters.go` 的过滤 chip
§6.40-5、`launcher/power.go` 的确认带 chip §6.40-4），另外两个就是本轮这两行：

| 位置 | 改前 |
|---|---|
| `settingsui.sidebarRow`（`settingsui.go:1382`） | 静息透明 → 悬停 `RowHover`；**按下 = 悬停帧** |
| `settingsui.runOutputToggle`（`settingsui.go:7084`） | 静息透明 → 悬停 `ControlHover`；**按下 = 悬停帧** |

**改前实测**（把产品改动 stash 掉再跑新 pin，原始报错串）：

```
pressrung2_test.go:98: press is 9 levels off rest and hover is 9: a row's press rung is the heavier wash…
```

按下与悬停**同一个像素值**（浅 `(221,221,221)` / 深 `(48,48,49)`）——不是「差不多」，是逐字节相同。

## 2. 改了什么

### 2-1 侧栏的导航行（`sidebarRow`）

按下压过选中与悬停，取**行阶梯**的第三格 `tokens.RowPressed`（浅 `rgba(0,0,0,0.08)` /
深 `rgba(255,255,255,0.11)`，`theme.go:745/871`）。**不戴 `LitFace`**——启动器的结果行按下
那一档也不戴（`launcher.go` 的 `case rowPressed:` 只换面）：受光边是「这块面可以站起来」的
记号，按住是**沉下去**。墨的判据加 `|| row.Pressed()`，与悬停同档 `t.Text`。

### 2-2 集成页折起的「查看输出」行（`runOutputToggle`）

旧写法 `hovered := button.Hovered()` + 单档 helper `runOutputToggleFace(tokens, hovered)`
换成三档 `switch`（按下 `ControlPress` / 悬停 `ControlHover` / 静息透明）+ 一份面共用一对线
（`if plane.A > 0 { ink = t.Text; tokens.LitFace(button) }`）。helper 删除（`internal/ci` 的
死代码合约会数它）。**几何一个像素不动**：不加 `Scale`，参考的 `:active` 变换是给胶囊的
（`.extensions-action-button:active { scale(.97) }`），而这一行是整宽的文字控制
（`extensions.css:1243-1256`），整宽收缩会把 chevron 从行首挪进去。

### 2-3 顺带补的普查漏账

`internal/ci/litarm_contract_test.go` 的臂级普查只认 `case X.Hovered():` / `if X.Hovered() {`，
而 `runOutputToggle` 把状态读进变量 → `len(arms)==0` → **整个函数对普查隐形**，于是它悬停那块
`ControlHover` 面一直没有一对线（§2 判据 1 的 E1 缺件）也没人发现。改成 `switch` 之后它进入
视野，并当场要求那一对线（第一版注释里写了 `ControlPress` 六个字母，普查把**注释**也算进臂
文本，直接判红——记在这里，下一轮别再写进去）。

## 3. 实测数值（改后，像素直读）

| 面 | 外观 | 静息 | 悬停 | 按下 | 按下 vs 静息 | 按下 vs 悬停 |
|---|---|---|---|---|---|---|
| 侧栏行 | light | `(230,230,230)` | `(221,221,221)` | `(212,212,212)` | **18** | **9** |
| 侧栏行 | dark | `(39,39,40)` | `(48,48,49)` | `(63,63,64)` | **24** | **15** |
| 折叠行 | light | `(229,229,230)` | `(205,205,206)` | `(222,222,223)` | **7** | **17** |
| 折叠行 | dark | `(39,39,41)` | `(69,69,71)` | `(50,50,52)` | **11** | **19** |

**两条阶梯的按下方向是反的，这不是笔误**：行阶梯（`RowHover 0.045 → RowPressed 0.11`）
按下是**最重的一遍洗色**（离静息更远）；控制阶梯（`ControlHover 0.14 → ControlPress 0.05`）
按下是**把抬起来的那一档还回去**（落在静息与悬停之间）。pin 把这条写死成两条相反的不等式。

## 4. 门槛（实测）

```
gofmt -l cmd internal                 # 空
go vet ./...                          # 净
GOOS=windows go vet ./...             # 净
GOOS=darwin  go vet ./...             # 净
go test -count=1 ./...                # 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3267 → 3276
GOOS=linux|windows|darwin go build ./...            # 三个都过
```

测试函数数：**2421 → 2424**（+3：`TestTheSidebarNavRowTakesAPressRung`、
`TestThePressedSidebarRowLeavesItsNeighbourAlone`、`TestTheFoldedOutputToggleTakesAPressRung`；
PASS +9 来自 2 套外观的子测试）。基线 3267 是本轮开工前那棵树（HEAD `359ad2cd`，与上一轮
实测同一棵树，`git status` 干净）的实测值。

**V / B / S 台账：V 0 / B 0 / S 0**——没有一条既有 pin 需要改断言；新 pin 是新增文件。

## 5. 变异（三条实跑，全部真红后逐字节还原）

| # | 注入 | 报错串（原样） |
|---|---|---|
| M1 | 侧栏按下臂 `if false && row.Pressed()` | `press face {221 221 221} is 0 levels off the hover face {221 221 221}: holding the pointer down is pixel-identical to hovering (P13-cn)` + `press is 9 levels off rest and hover is 9` |
| M2 | 折叠行按下臂改成悬停面 | `press face {205 205 206} is 0 levels off the hover face {205 205 206}`（深 `{69 69 71}` 同值）+ 方向不等式红 |
| M3 | 侧栏按下臂借**控制**阶梯的按下档 | `press face {223 223 223} is 2 levels off the hover face {221 221 221}` + `press is 7 levels off rest and hover is 9` |

M3 是「两条阶梯不能互换」的证据：借错阶梯，按下只差 2 级，而且方向反了。
还原：`md5sum -c` 逐字节成功 → `touch` 后复跑三条 pin 全绿。

## 6. 出图

`/tmp/floter-P13-cn-{sidebar,outputtoggle}-{light,dark}-{rest,hover,press}.png`，
前/后对比：`…-press-before.png`（产品改动 stash 掉时拍的）对 `…-press-after.png`。

**这张图上看到的一处变化**：侧栏 `About` 那一行，`before` 帧与悬停帧一模一样（`(221,221,221)`
一块淡淡的洗色），`after` 帧明显更深（`(212,212,212)`）——按住的那一瞬，这一行比指针停在
上面时更「陷进去」，松开才跳页。

## 7. 偷了什么（qiaomu-design / craft-loop 工艺密度菜单）

- **第 12 项「材质细节：内边框高光、glass 折射边」** → 落在 `runOutputToggle` 的
  `if plane.A > 0 { tokens.LitFace(button) }`：受光边 `LensRim 0.09`（顶 1px）+ 对侧
  `LensEdge 0.20 深 / 0.06 浅`（底 1px），**只在有面的时候画**（静息透明时不画，否则就是
  §1 禁令 3 的「类玻璃边」）。这一项此前在这个函数里**完全缺失**，而且因为普查隐形没人发现。
- **第 4 项「签名交互：hover 揭示 / 一个足矣」** → 落在侧栏行的第三档 `RowPressed`
  （0.11 白 / 0.08 黑）：按住 → 洗色加深 9~15 级，松开才跳页。这一族的判据不是「加动效」，
  而是「按住必须和悬停分得开」。
- **没落地的 3 项，以及为什么**：这一轮是**两像素的状态档**，不是一个屏。菜单里 #1 氛围层、
  #5 多层染色阴影、#7 展示排版动作、#9 自定义滚动条在这个尺度上无处安放——尤其 #5 与
  §2 判据 2（E1 不画投影）直接冲突，硬凑就是「为了密度而密度」。**工艺密度 ≥5 的门槛
  对「一个表面的一档状态」不适用**，这一点如实记账，不假装凑满。
- **两轮制**：第一轮 = 三档 `switch`；四视角评审里「工程师」挑出的最弱一处 = `runOutputToggle`
  对普查隐形（E1 缺件），第二轮只修这一处（补共享一对线 + 删 helper）。**没有推倒重做。**

## 8. preflight 逐条

- **A 反套路（AI Tells）**：过。没有渐变、没有外发光、没有 `#000000`、没有斜体；
  新加的两条线都是 **1px 实色**（`LensRim` / `LensEdge`），不是渐变白边——正是 §1 禁令 3
  点名的那一类。本轮零新增 i18n 文案（`grep` 过：没有 `copy.` 改动）。
- **B 设计读取一致性**：过。设计读取是「轻便液态玻璃 + 明确 elevation 阶梯」，本轮只用了
  **既有 token**（`RowPressed` / `ControlPress` / `LitFace`），没有新增 token、没有写死色值；
  两条阶梯各自取自己那一族的 token，没有混用（M3 证明混用会红）。
- **C 动效抽查**：过，附一条说明——侧栏行自带 `Transition(Colors: true, Duration: ColorStep)`，
  按下会**渐变**到 `RowPressed`（120ms）；折叠行没有 transition，按下是**瞬变**。两者不一致
  是既有的：`savealert.go` 的 ✕、`parameditor.go` 的小格、`pluginsheet.go` 的胶囊都没有
  transition。本轮**不**给折叠行加 transition（那是另一个决定，且会让 press 帧读不到稳定值）。
- **D2 中文页**：过（本轮没有新增/修改任何中文文案）。**未过的既有项**：`tokens.Caption` 11px
  与 D2「正文 ≥ 14px」的门槛仍悬着（`.pending-bugs.md` 的第 17 份证据），本轮不动它。

## 9. 仍待

1. **需拍板（沿用）**：`launcher/filters.go` 过滤 chip、`launcher/power.go` 确认带 chip 的
   按下档——前者参考自己就没有面，后者暖面没有第三档 token。**这一族只剩这两条**。
2. **需拍板（沿用）**：`tokens.Caption` 11px 与 preflight D2。
3. **需拍板（沿用）**：会话页空态说错了对象（`SessionsEmpty` 说「持久化会话」，这一页列的是
   进程内会话）——i18n 红线，先量再进拍板。
4. **需拍板（沿用）**：终端页的会话抽屉 / 标签对失败只加字不换墨，与 §6.56 落地的会话列表
   不是同一个说法。
5. **下一格（不需要拍板）**：通用队列「间距节奏」。`docs/liquid-glass.md` §6 没有 `未开始` 行，
   间距刻度已在 §6.35/§6.36 量过（行纵向只有 9u 一档），下一格从**文案密度**或**空态满态**
   里挑，先量再改。
6. **口径教训（写进报告）**：`internal/ci/litarm_contract_test.go` 把**注释文本**也算进臂，
   所以在指针臂的注释里写别的阶梯的 token 名会直接判红。写注释时用中文说法，别写 token 全名。
