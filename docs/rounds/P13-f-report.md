# P13-f 报告：文件页表面（`docs/liquid-glass.md` §6.5）

**一句话**：文件页的 E1 从「继承到了」变成「量到了」，而这一页每行第一眼看的
那块 28u 图标板终于不再对十五行说同一句「最近用过」。

- 规格 `docs/rounds/P13-f-spec.md`，合约 `docs/liquid-glass.md` §6.5（本轮从 `未开始` 收口）。
- 轮号 P13-f（视觉队列第 5 片），基线 HEAD `1c7e9953`（P13-e）。

---

## 1. 选题与意图

`docs/liquid-glass.md` §6 是唯一权威选题队列。§6.1（启动器）= P12-b、§6.2（设置页）
= P12-c、§6.3（终端页）= P13-d、§6.4（扩展页）= P13-e 已落地，**§6.5 文件页是表里
最后一个 `未开始` 的表面**。终端弧剩下的 S1b（`CloseAll` 改 detach）**改会话管理
语义 = 需用户拍板**，所以按纪律回到视觉队列。

**这轮为什么让它更轻 / 更有空间感**：这一页的 E1 在 P12-b 之后已经继承到了
（文件页的行就是 `rowBody` 的普通结果行），但这一页**从来没有一条像素证据**；
更重的是，它的图标列把「打开 / 进终端 / 拷路径」三件不同的事全画成**历史时钟**
——一屏五个文件就是十五个时钟。一个说错话的图标列会把这一页唯一的层级信号
（面 + 受光边）淹掉：读起来是「一堆一样的东西」。记号各归各位之后，面与线的
三级（底 / 面 / 选中的面）才读得出来。

## 2. 落地清单（实测）

| 行 | 位置 | 落成 | 实测 |
|---|---|---|---|
| 5-1 | `launcher.go:2370` `rowBody` 的选中臂 | **复核 + 像素 pin，产品代码零改动**（文件页的行走同一条路：`files.go:144` 的 id 是 `drop:<action>:<path>`，不带 `bar:` 前缀） | 选中块在 `x=600` 一列：浅色 face `(238,238,239)` / 顶边 `(240,240,240)`（+2）/ 底边 `(224,224,225)`（−14）；深色 face `(43,43,45)` / 顶边 `(62,62,64)`（+19）/ 底边 `(40,40,42)`（−3） |
| 5-2 | `glyphs.go` `rowGlyph` | 新增三条前缀分支：`drop:open:` → `glyphFolder`（与 `bar:path-open` 同一条）、`drop:cd:` → `glyphTerminal`（`bar:path-cd`）、`drop:copy:` → `glyphFile`（`bar:path-copy`） | 字形框 16×16 内 152/256 px 的墨变了，几何零改动 |
| 5-3 | 同处 `drop:more` | `glyphChevronRight`（它做的是**展开**） | 同上 |
| 5-4 | `glyphs.go` `iconPlate` | **故意不做**：这块板启动器所有页共用，改它等于跨表面 | — |

**「同一语汇」不是比喻**：文件页的三行就是动作条上同一个路径的同一组动作
（`glyphs.go:201-204` 早就钉死了 `bar:path-open/cd/copy` 的三个记号），所以它们必须
戴同一组。这正是 §6.5 原行「与 1-2 同一条规则」的意思，只是它说的是面，而这一页
真正缺的是**记号**。

## 3. 顺手修掉的一条 B 类回归（门槛抓到的）

整仓 `go test ./...` 第一次跑红在 `internal/termsession`：

```
--- FAIL: TestTheKillEndsTheProgram (0.00s)
    host_test.go:174: the record says killed but process 2811846 is alive
```

**测试零改动**（B 类），红的是产品：`host.go` 的 `tagKill` 先把
`session.info.State = StateKilled` 写进记录，**再**去 `pty.Kill()`——记录比事实
早了几十微秒。单包跑绿、整仓并发跑红（两次复现，都是全量 `go test ./...`）。

改成「意图先行、状态后写」：`Session` 新增 `killRequested`（只在 `session.mu`
下读写，不进 `Info`），`tagKill` 只写意图再动刀，`watch()` 在
`<-pty.Exited()` 之后按意图决定 `killed` / `exited`。契约面（`Info`、帧、记录
文件格式）一个字没变。

## 4. 偷了什么（`qiaomu-design` 的工艺密度菜单）

本轮读的是 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` →
`preflight.md`（按任务书只读这四份，不全库通读）。取了三项，逐项对到数值上：

1. **菜单第 11 项「图形资产：拒绝通用图标堆」** → `rowGlyph` 的 `drop:*` 三个动作
   各给一个专属记号（folder / terminal / file），折叠行给 disclosure 的
   `glyphChevronRight`。具体数值：每个字形框仍是 16×16（`t.Space(4)`），
   位置不动，只有 152/256 px 的墨换了形状——不是加装饰，是**删掉一句假话**。
2. **菜单第 12 项「材质细节：内边框高光」** → 选中块的一对线就是这一项的 Go 版：
   顶边 `LensRim` 0.09（浅 238→240 / 深 43→62）、底对侧 `LensEdge` 0.06
   （浅 238→224 / 深 43→40）。顶亮一分、底暗一分，块才站起来（§3）。
3. **菜单第 6 项「编辑细节：hairline」** → 反向使用：这一页**不加**任何新的
   hairline。分组头、行边框、图标板外环一个都没动——本支的不变量是「一个面
   只说一句话」，加线就是加噪点。这一条是「故意不做」的工艺依据。

**DNA 注入（§一）本轮 N/A**：`craft-loop.md` 要求从 58 站库取 1–2 个供体，
本仓的视觉单一来源是 `internal/theme/theme.go` + `docs/liquid-glass.md`，
再叠一层外部 DNA 会与「单一来源」合约冲突；本轮的「供体」是合约自己的 §1/§3。
**两轮制（§三）成立**：第一轮出 before 图 → 四视角里「读者/艺术总监」指出的
最弱处就是那十五个时钟（眯眼看整页，第一眼全落在板上）→ 第二轮只改这一处 →
重截图 + 像素 diff 复核（变化全部落在 9 个 16×16 字形框内，行盒与文字列零变化）。

## 5. preflight 逐条

**A · 反 AI 套路**：✅ 不自绘渐变/发光/纯黑（配色来自 token 层，本客户端是全原生
自绘，无 CSS）；✅ 无斜体（P-1）；✅ 无三等分卡片；✅ 无假数据/烂梗名；✅ 文案
零改动（i18n 一个字没碰）；✅ 无左侧竖线装饰。**不过的项**：`A` 的「字体不是系统
默认」在本仓**不适用**——字体是设置项里用户自己选的 `FontFamily`，本轮不动字体。

**B · 设计读取一致性**：✅ 一页一个明暗主题（深/浅两套各自出图）；✅ 记忆点一句话
——「每一行第一眼的那个方块，说的是这一行要做的事」；✅ 动效气质不变（未新增任何
transition，行入场仍是 120ms 的既有 `rowTransition`）。

**C · 动效抽查**：✅ 本轮零动效改动；✅ 无 `transition: all`（本仓用
`ui.ElementTransition` 显式字段）；✅ 按压反馈沿用既有 `row.Scale(0.986)`；
✅ 高频/键盘触发路径无动画（拾取/方向键不带动画）。**不过的项**：无——
「仅动 transform/opacity」这条本仓由 `ui` 框架保证，本轮未碰。

**D2 · 中文页**：✅ 中文用系统字体栈（`settings.FontFamily` 缺省即系统栈）；
✅ 中文行高与字号由既有字体刻度决定，未改；✅ 中文文案零改动；✅ 中西文空格规则
沿用既有 i18n 文案（`FilesSection` / `FilesMore`）。**不过的项**：无。

**E · 三秒测试**：✅ 一屏只有一处争夺注意力（选中块）；✅ 图标列从「十五个一样」
变成「三个一组」，眯眼能看出分组。

## 6. V / B / S 台账

- **V 0**：没有一条视觉数值被重定基线（5-1 的数值早由 P12-b 定死，本轮只是量它）。
- **B 1**：`internal/termsession/host_test.go` 的
  `TestTheKillEndsTheProgram` **测试零改动**，红是产品（见 §3），改产品。
- **S 0**：结构未变（id、行数、顺序、`Run`、高度链、键盘模型全部未动）。

## 7. 门槛（实测）

```
gofmt -l cmd internal                       → 空
go vet ./...                                → 净
GOOS=windows go vet ./...                   → 净
GOOS=darwin  go vet ./...                   → 净
go test -count=1 ./...                      → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS" → 2891   （基线 2887，+4）
--- SKIP 计数                                → 64       （基线 63，+1 = 本轮 opt-in dump）
--- FAIL 计数                                → 0
测试函数数（grep -h "^func Test"）            → 2140     （基线 2137，+3）
GOOS=linux/windows/darwin go build ./...     → 三平台过
```

**观察（如实记录）**：整仓并发跑的第二、第三次里，`internal/shell` 的
`TestARerunMarksTheWindowRunningUntilTheAnswerLands` 红过一次（0.00s）。它是纯逻辑
（`rerunDetached` 无条件置 `running = true`），本轮改动不可达该路径；`-run` 单跑
与随后两次整仓跑都绿。记为**观察**，不当作已修。

## 8. 变异（实跑，逐条还原 + `md5sum -c` 逐字节核对 + `touch` 复跑绿）

| # | 注入 | 变红的 pin 与原始报错串 |
|---|---|---|
| M1 | `glyphs.go` 三条 `drop:` 前缀改成 `drop:nope:` | `--- FAIL: TestTheFilesPageLeadsWithItsOwnActionMarks`；`the "drop:open:/tmp/report.pdf" row leads with clock, want folder (the files page's three actions are the action bar's three actions, so they wear its three marks)`（三条动作 × 三个文件共 9 行） |
| M2 | `drop:more` 改回 `glyphClock` | 同上文件；`the expander row leads with clock (ok=true), want chevron: it opens the list, it does not list a launch` |
| M3 | `launcher.go:2370` 的 `tokens.LitFace(row)` 抹成 `_ = tokens` | `--- FAIL: TestTheFilesPageChosenRowStandsOnOneLitEdge/light`；`the chosen file row's top edge {238 238 239 255} is not brighter than its own face {238 238 239 255}` |
| M4 | 选中块补一条 `row.Shadow(0, 3, 10, 0, tokens.ShadowAmbient)`（E1 冒充 E2） | 两个外观都红：`the chosen file row is a flat block ({52 83 1 63}, face {238 238 239 255}): none of its top four rows is LensRim … over that face` / `{52 86 1 60}, face {43 43 45}` |
| M5 | `watch()` 的 `if h.session.killRequested` 改成 `if false && …` | `--- FAIL: TestTheKillEndsTheProgram (5.02s)`；`host_test.go:178: the record = {ID:sh-2827335 Name:sh State:exited ExitCode:137 Seq:2 Cols:80 Rows:24 Pid:2827335}, want killed` |
| M6 | 把旧顺序加回来（`tagKill` 在动刀**前**写 `info.State = StateKilled`） | **没能稳定变红**：单包 `-count=40` + 6 个 CPU 打满的 `yes` 都绿。这条窗口只有几十微秒，只有整仓并发（几千个测试 + 大量进程创建）才抓得到——改动前**两次**整仓跑红的原始串就是它的证据（见 §3）。如实记录，不算作「变异已实跑变红」 |

五条实跑变红的（M1–M5）逐条 `md5sum -c` 还原成功后才 `touch` 复跑，复跑全绿。

## 9. 出图

`/tmp/floter-P13-f-files-{dark,light}-{before,after}.png`（before = 生产改动前的
HEAD 代码 + 本轮新增的 dump，after = 改动后；同一 fixture、同一尺寸 720×488）。

**这张图上看到的一处变化**：整页只有 **9 块 16×16 的字形框**变了（浅色 1397 px /
深色 1353 px，y 起点 107 / 153 / 199 / … 步长 46，全部落在 x=22..37 的字形框里）；
浅色图第一行走廊的板里，原本是一个**圆环加两针**的时钟（像素 diff 掩膜能直接看出
圆形轮廓与指针），改完之后是折角向下的「文件夹」轮廓——行盒、标题列、字号、
选中块的上下两条线**一个像素没动**（diff 里 y 范围之外没有任何变化）。

## 10. 仍待

1. **S1b（`CloseAll` 改 detach，改会话管理语义）仍等拍板**、**S1c（Windows ConPTY）**、
   **客户端不告而别的检测**、**capture 通道迁移** —— P13-c 的四条原样有效。
2. **视觉队列下一站 = §6.6 弹层**（1 行 `未开始`：toast 补 `ElevThrow` 两层影；
   另一行已判 `故意不做`）。§6.6 收口之后 §6 表里就没有 `未开始` 的表面了。
3. **本轮观察（新）**：`iconPlate` 的四层 `Lens`（第四层是四边 `ControlEdge` 的环）
   是启动器所有页共用的板，收成 `LitFace` 是跨表面的活，留给启动器表面那一轮。
4. **观察**：`Serve()` 的 idle 分支（无人 attach 且 `Linger >= 0`）会 `pty.Kill()`
   但**不写意图**，于是记录说 `exited`——一个被宿主回收的程序与用户主动结束的
   程序在记录里是同一句话。改它要动「结束原因」的语义，写进需拍板。
