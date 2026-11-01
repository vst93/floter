# P13-bn 报告 · 权限审计行的尾部：一枚警示环陪着一条「在/不在」

**方向一句话**：权限审计的每一行在尾槽里画**两件东西**——一枚 `alert` 警示环 + 一枚 `●`/`○`，
两者染同一个墨、说同一个状态。那枚环没有语义，而它读起来像「这一行坏了」。

- 规格 `docs/rounds/P13-bn-spec.md`；合约新增 `docs/liquid-glass.md` **§6.32**（4 行：3 `已落地` / 1 `需拍板`）。

## 为什么这轮让它更轻、更有空间感

z 阶梯的价值是**一个物体只说一句话**。尾槽里两枚记号相隔 **7 级像素**，读成一枚徽标的
两个部件——于是「这条权限属于哪一层」这个唯一的意思被画了两遍，而且其中一遍（`alert`）
在别处（`internal/shell/notices.go:294-299` 的 `toastMark`）是 `error` 的记号。删掉它之后
尾槽只有一个物体，左边界从 **493 → 514**、墨宽从 **29 → 8** 列：那一格终于只装一件东西。

## 起点与侦察

`docs/liquid-glass.md` §6 已无 `未开始` 行 → 通用队列「状态反馈」。上一轮（P13-bm）的「仍待 1」
是路径截断（**需拍板**，不擅自动），所以本轮取不需要拍板的一格：审计行的状态标记。

**先量再改**（`ui.Tester`，520u 卡，`slotInk` 以行面为基准扫尾槽）：

| 外观 | 状态 | 行尾墨段（x 坐标） |
|---|---|---|
| 浅 | enforced | `493..506`(14) + `514..521`(8) |
| 浅 | declared | `493..506`(14) + `514..520`(7) |
| 深 | enforced | `493..506`(14) + `514..521`(8) |
| 深 | declared | `493..506`(14) + `515..516`(2) + `520..520`(1) |

`493..506` = 16u 的 `alert` 环，`514..52x` = 11px 的盘。真页复核（720×1200 集成卡展开审计）
也确认 `tt.Texts()` 里 `●`/`○` 各一次，但**同一张卡上两个层级标题出现两次**（审计块
`TextStrong`/650、审批记录块 `TextSecondary`/620，相隔 146u）——**这一条只记账、不改**，
抬成 §6.32 的 `32-4` 需拍板。

## 落地

| # | 位置 | 旧 | 新 |
|---|---|---|---|
| 1 | `settingsui.go` `permissionTier` 尾槽（`:1292` 那一行） | `ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(color)` + `ui.Text(mark)` | **删掉 `ui.Icon` 整行**，只留 `●`/`○` |
| 2 | 同一格的注释 | 「an enforced permission's row is a plane with the accent edge」——**这行没有 plane、也没有 edge** | 改成实况：一枚圆盘，实心 = 主机自己决定，空心 = 仅声明；并把「两枚 → 一枚」的实测写进注释 |
| 3 | 同一格的字阶 | `FontSize(t.FontSize-3)` = **11.00px** | **不动**（见下） |
| 4 | 同一格的墨 | `●` = `t.Accent` / `○` = `t.TextMuted` | **不动**（§5 注脚第 1 条） |

**为什么字阶不动（先量再说）**：`FontSize` 在 shipped 步是 **14.00**，所以 `FontSize-3` 是
**11.00px**；`tokens.Caption` 是 **9.90px**。把这一枚改成 `Caption` 是**降一档**
（盒 11×17 → 10×15、墨 11×10 → 10×9），而它是这一行尾**唯一**的状态记号——降它等于让
「已强制 / 仅声明」更难读。规格里我原本写的「`FontSize-3` 是 6.6px」是**错的**（那时没量），
量完之后改成了「不动」，并把这个更正留在规格 §三 第 2 条里。

## 门槛（全部实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` → 净；`GOOS=windows go vet ./...` → 净；`GOOS=darwin go vet ./...` → 净
- `go test -count=1 ./...` → **全绿**；PASS **3192 → 3198**（+6）| FAIL 0 | SKIP **112 → 113**
  （+1，opt-in 出图）
- 测试函数：**2346 → 2349**（+3：两条 pin + 一条 opt-in dump）
- `GOOS=linux/windows/darwin go build ./...` → 三个都过
- **V 0 / B 0 / S 0**（没有 pin 钉旧数值；`raycast_test.go:1517` 的 §5-注脚-1 那条两侧判据
  在删掉环之后**原样通过**——它读的是 `●`/`○` 本身）

## mutation（3 条实跑，全部变红并逐字节还原）

1. **M1**：把 `ui.Icon(c, mustGlyph("alert")).Size(t.Space(4), t.Space(4)).TextColor(color)` 加回去：
   ```
   permimark_test.go:150: [light] the enforced row's trailing slot starts its ink at x=493 but the
       mark's box begins at x=514 (runs [493..506(14) 514..521(8)]): something is drawn in front of
       the state mark
   permimark_test.go:157: [light] the enforced row's trailing slot paints 29 columns of ink
       (runs [493..506(14) 514..521(8)]), more than one 11px mark: the row's state is one mark
   ```
   （深色同形；`declared` 两档也红。同一次注入也用来出 `before` 图。）`md5sum -c` 逐字节还原。
2. **M2**：`enforced` 的 `color = t.Accent` → `tokens.TextStrong`：
   ```
   permimark_test.go:180: [light] the enforced mark {27 27 29} is only 2 levels warm; the solid
       disc is the accent (§5 注脚第 1 条)
   permimark_test.go:180: [dark] the enforced mark {237 237 239} is only 2 levels warm; ...
   ```
   `md5sum -c` 逐字节还原。
3. **M3**：`color := t.TextMuted` → `color := t.Accent`（两条 tier 都拿 accent）：
   ```
   permimark_test.go:184: [light] the declared mark {137 162 196} carries a hue (R−B 59): the
       hollow ring is the neutral half of the ON/OFF pair
   permimark_test.go:184: [dark] the declared mark {91 106 133} carries a hue (R−B 42): ...
   ```
   `md5sum -c` 逐字节还原。

**没做成的一条（如实记录）**：原计划的 M3 是「把 `FontSize-3` 改回 `tokens.Caption`」，
实跑**不红**——`Caption` 下墨仍是 8~10 列，落在我给的两条括号（左边界 ±2 列、总宽 ≤14）之内。
按纪律**不造脆弱断言**（加一条「字阶必须等于 11.00」的 pin 只是把常量判常量），改为上面
那条同族注入。这条也解释了为什么第 3 项落地是「不动」：那一格的字阶本来就对。

## 出图（`FLOTER_P13BN_DUMP=1`）

- `/tmp/floter-P13-bn-permimark-light-{before,after}.png`
- `/tmp/floter-P13-bn-permimark-dark-{before,after}.png`

**眼睛复核到的一处**（`before` 由 M1 注入取得；`after` 是 HEAD）：深色 `before` 那张在
`x=486..532, y=98..110` 的字符图里能同时看到**一个空心圆环**（左，`x≈494..506`）和**一个
实心盘**（右，`x≈514..521`），两者之间只有 7 列空白；`after` 那张**只剩右边那个盘**，
左边那一片是纯色。浅色同形（`before` 的环是 `.` 级的灰、盘是 `-` 级；`after` 只有盘）。

## 偷了什么（qiaomu-design · craft-loop.md 工艺密度菜单）

- **第 7 项「编辑细节」**：取「同一语义只能有一种画法」。落成：尾槽从**两枚**记号收到**一枚**，
  并用一条**括号夹**的 pin（左边界必须落在记号盒子自己的左边缘 ±2 列、总宽 ≤14 列）把
  「第二个物体」这件事钉死——不是钉一个数字，是钉「这里只能装一件东西」。
- **第 12 项「材质细节」**：取「减法工艺」。这一轮没有新增任何墨，只**删**了一枚图标；
  实测证据是行尾墨的左边界 493 → 514、宽度 29 → 8 列。
- 未取项说明：第 5 项（多层染色阴影）不适用——这一格是**行内**元素，E1 不画影（§2 判据 2）；
  第 2/3/4/9/10 项是页面级签名动作，套在一个 16u 的行尾槽上就是「密度 ≠ 吵闹」的反例。

## preflight 逐条（打磨模式，A / B / C / D2）

- **A 反 AI 套路**：无渐变描边冒充高光（本轮**删**一枚图标）；无纯黑/外发光/渐变文字；
  无三等分卡片；无「革命性/无缝/赋能」；感叹号 0。
- **B 设计读取一致性**：读取 =「桌面启动器的设置页，面向高级用户，用轻便液态玻璃的克制语言，
  倾向 E1/E2 两档材质」。本轮产出与读取一致——**只减不加**，没有引入任何新的面档或墨档。
  记忆点一句话：一行只说一句话，行尾只装一件东西。
- **C 动效抽查**：本轮不动效。这一行本来就没有过渡以外的动效；`a.row` 的 `Transition` 未动。
- **D2 中文页**：本轮**没有**新增/改动任何用户可见文案（`copy.*` 一个字段没动）；删掉的是
  一枚无文字图标，中文页的唯一变化是尾槽少一个物体。**未过项（沿用，需拍板）**：
  `tokens.Caption` 9.90px / `tokens.Body` 10.80px 低于 D2 的「正文 ≥ 14px」；这条是全仓字阶的账，
  `.pending-bugs.md` 里已挂多轮，本轮**特意没有**把这一格往 `Caption` 降（见上文），不擅自改。
- **D 工程抽查**：删掉的是 `ui.Icon`，它没有 label、不是可点元素；`a.row` 的命中与键盘模型
  未动（`●`/`○` 是 `ui.Text`，本来就不吃指针）。

## 仍待（P13-bn）

1. **需拍板（本轮新记，§6.32 的 32-4）**：同一张卡上**两个层级标题**（审计块
   `TextStrong`/650/`FontSize-1` vs 审批记录块 `TextSecondary`/620/`Caption`，实测相隔 146u）。
   两处都是「这一层的名字」，统一要先定「谁是权威写法」——审计块没有参考 CSS，审批记录块
   来自 `extensions.css`。**不是墨的问题**，是语义决定。
2. **需拍板（沿用）**：`tokens.Caption` / `tokens.Body` 与 preflight D2；`ui/tooltip.go:133`
   的「禁用元素不弹提示」；`cardRow` 控制槽 `Shrink(0)`；多选箱宽度；路径截断保头弃尾
   （P13-bm 仍待 1）；终端弧 S1b / S1c / S2 尾。
3. **下一格候选**：① 集成卡里「同一件事说两遍」的其余位置——本轮只修了尾槽，`tt.Texts()` 里
   还能看到 `Permissions` 这个行名与它下面的层级标题重复；② 空态/满态的**文案密度**；
   ③ `internal/shell` / `internal/launcher` 的「后置禁用」普查（P13-ba 只扫了 `settingsui`）。
4. **抢救 stash 复核（本轮做了）**：`stash@{0}`=`afc38b247c` 的 `metaSeparator` 变体已被
   P13-af 的 `metaFact` 取代（HEAD 里 `metaSeparator` 出现 0 次）；`stash@{1}`=`114b7d1585`
   只改 `.pending-bugs.md`，文字已在文件里。两条都**无需再试合**，也**没有**静默 drop。
