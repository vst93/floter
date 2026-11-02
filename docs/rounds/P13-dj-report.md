# P13-dj 报告：About 卡第四行的名字，是全卡最重的一行字

**方向一句话**：同一张卡、同一列、相邻四行，名字此前是两把尺子（11px/500/`TextSecondary`
三次 + 15px/580/`TextStrong` 一次），第四行还是这张卡**唯一可点**的一行 —— 一列回到一个字阶，
`Project` 归位成「第四条」，可点由链接自己的色说。

**这轮为什么让它更轻 / 更有空间感**：卡片的左列此前在第 4 行突然长大（字号、字重、墨深三项
一起跳），读起来像「这里开始新的一节」；一列一个字阶之后，这张卡的竖向节奏第一次是四条等重
的记录，而不是「三条事实 + 一个标题」。

选题：任务书「下一轮 = 细节打磨；下一格从状态反馈的另一半（禁用 / 只读面）或空态满态里挑，
先量再改」+ P13-di「仍待」第 1 条（About 页的只读面）。规格 `docs/rounds/P13-dj-spec.md`，
合约新增 **§6.79**。表面 = **设置页 → About 的身份卡（四行）**。

## 先量（`inkSpan` 三通道和，1 级 = 3；`aboutPage` 760×1400，两套外观）

改前（HEAD `ef1e4ecd`，新 pin 首跑，红）：

| 行 | 名字 | 名字墨 | 值 | 值墨 |
|---|---|---|---|---|
| 0 `Framework` | 11px/500/`TextSecondary` | **122 / 115** | `mygo 0.9` | 202 / 198 |
| 1 `Link scheme` | 同 | 122 / 115 | `floter` | 202 / 198 |
| 2 `Settings file` | 同 | 122 / 115 | `/home/tar/.config/floter/settings.json` | 202 / 198 |
| 3 `Project` | **15px/580/`TextStrong`** | **202 / 198** | `https://github.com/vst93/floter`（14px body，链接色） | **165 / 140** |

全卡最重的字属于**名字**，最轻的字属于**链接自己**。行盒：名字 18 vs 15、值 17 vs 18 ——
四行里唯一一行三项都不同。

## 落地项（`38-1…38-6` 实为 79-1…79-6，见合约 §6.79）

| # | 位置 | 改法 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go:4200-4214` | `a.row(...)` → `cardRow`，名字走新的 `a.factName(c, copy.AboutProject)` |
| 2 | `internal/settingsui/settingsui.go:4345` 新增 `factName` | 把 §6.11 / P13-bs 的 `Row.FillHeight().AlignItems(ui.Center)` 提成一个函数，`keyValue` 与链接行**共用**（此前那份居中技巧只属于 `keyValue`） |
| 3 | 链接的字阶 | `FontSize(t.FontSize)` → `tokens.Emphasis` / 580：**值回到这张卡的一条尺子**（三条事实的值也是 emphasis），色号仍是 `Theme.Accent` |
| 4 | `internal/settingsui/settingsui.go:4188` | `t := c.Theme()` 不再有人用 → 删掉（这一页的墨全部走 `a.tokens(c)`） |
| 5 | `internal/settingsui/aboutlinkink_test.go`（新） | 两条 pin + opt-in 出图 |

改后（同一夹具、同一像素读法）：

| 行 | 名字墨 | 名字盒 | 值盒 |
|---|---|---|---|
| 0–2 | 122 / 115 | 15 | 18 |
| 3 `Project` | **122 / 115** | **15** | **18** |

四行名字同在 y=145.8 / 190.8 / 235.8 / **280.8**（45px 一拍），同在 x=200.4。

## 台账：V 0 / B 0 / S 0

没有一条既有 pin 变红 —— 这一行此前**根本没有墨账**（`keyvalueink_test.go` 只量
`aboutFacts()` 那三行）。新增覆盖而不是替换覆盖：2472 测试函数（+3）、PASS 3314（+2）、
SKIP 150（+1，本轮 opt-in dump）。

## 门槛（实测）

```
gofmt -l cmd internal                                  → 空
go vet ./... / GOOS=windows / GOOS=darwin              → 净
go test -count=1 ./...                                 → 全绿
--- PASS 计数                                          → 3314（基线 3312）
--- SKIP 计数                                          → 150（基线 149）
测试函数                                               → 2472（基线 2469）
GOOS=linux/windows/darwin go build ./...               → BUILD-OK
```

## 变异（2 条实跑变红并还原，`md5sum` 逐字节核对 + `touch` 后复跑 `ok`）

- **M1**：`a.factName(c, copy.AboutProject)` → 旧写法 `ui.Text(...).FontSize(tokens.Emphasis)
  .FontWeight(580).TextColor(tokens.TextStrong)`。原始报错：
  `light: row 3's name "Project" resolves 202 levels — on this card the name is the caption over
  its value, not a setting's title (§6.11)` 与 `light: row 3's name "Project" is drawn in a
  18px-tall box, the card's first name in 15px`（深浅各两条，共 4 条红）。
- **M2**：链接字号 `tokens.Emphasis` → `tokens.Body`。**第一次跑是绿的** —— 值行盒的容差当时是
  ±1px，而 17 vs 18 恰好落在里面。这不是「测试太严」，是**断言太松**：行盒是同一套字体同一档
  字号算出来的，不该有 1px 的合法差。容差收到 ±0.5px 后 M2 如实变红：
  `light: row 3's value "https://github.com/vst93/floter" is drawn in a 17px-tall box, the card's
  first value in 18px`（深浅各一条）。改后（正确值）复跑仍绿。
- 还原：`cp` 备份文件回去，`md5sum` = `59621130e4403ce786718587de81ec55`（改前改后同一串），
  `touch` 后复跑 `ok floter/internal/settingsui 0.208s`。

## 出图（视觉轮必须）

`FLOTER_P13DJ_DUMP=1 FLOTER_P13DJ_TAG=after go test -run TestDumpP13DJAboutProject ./internal/settingsui/`

- `/tmp/floter-P13-dj-about-light-after.png`（57 773 B）
- `/tmp/floter-P13-dj-about-dark-after.png`（57 426 B）

**这张图上看到的一处变化**：第四行标签列的左上角，此前那枚「P」比上面三行的首字母高一档、
也重一档（行盒 18、墨 202）；现在它和 `Framework` / `Link scheme` / `Settings file` 的首字母
一样高（15）、一样淡（122），而右边那条 URL 从 14px 长到 15px —— 卡的第四行第一次读成
「第四条记录」，而不是「下一节标题」。改前数值来自本轮首跑的同一套读法（202 / 198、盒 18 / 17）。

## 用 skill（`qiaomu-design`，打磨车道：`craft-loop.md` + `preflight.md`）

开工读了 `SKILL.md` → `references/user-preferences.md`（逐条对照 P-1 无斜体 / P-5 可读性优先 /
P-38 禁左侧竖线：本轮三样都没碰）→ `craft-loop.md` / `preflight.md`。
**没走 Phase 2 四方向预览**：方向由用户定死（轻便液态玻璃），这个自动车道无人点选。

### 偷了什么（逐条，`craft-loop.md` 的工艺密度菜单）

1. **第 6 项「编辑细节 —— hairline / 大写字距标签 / 章法」** → 落成：四行 caption **同一行盒
   （15px）+ 同一列（x=200.4）+ 同一拍（45px）**。密度不是加装饰，是把「同一列」这件事做到像素上。
2. **第 12 项「材质细节 —— 内边框高光」** → 落成：这张卡的 E1 受光边（`tokens.LitFace`，P12-c 已落）
   继续是卡唯一的材质声明；本轮**没有**给它加第二层（§2 判据 2：E1 不投影、不叠 sheen）。
3. **第 5 项「多层染色阴影」** → **故意不用**：E1 面长在底上，压不出影子；投影是 E2 浮层的事
   （§2 表）。写在这里是因为「密度凑满 5 项」不能凌驾于层级判据。

### preflight 结论（A / B / C / D2）

- **A 反 AI 套路**：过 —— 无渐变字、无发光、无纯黑、无斜体、无三等分卡片、无假数据、无空词；
  本轮只动两处墨与一处对齐。
- **B 设计读取一致性**：过 —— 一页一个主题；记忆点一句话：「这张卡只有一种尺子」。
- **C 动效抽查**：过（不适用为主）—— 本轮新增动画 0；链接 hover 的 `Underline` 是框架
  `coreLink` 既有行为，未被本轮改动；`ReduceMotion` 夹具照旧。
- **D2 中文页**：**一项没过** —— `tokens.Caption` = 9.9px / 正文 10.8px，preflight D2 要求
  正文 ≥ 14px。这是**沿用第八轮**的同一条冲突（`.pending-bugs.md` 需拍板第 3 条，第 35 份证据）：
  这一支的密度尺子（`--u` 4.0 / 行 44u）与「正文 ≥14px」在 1x 屏上不可同时成立，改哪一边是产品
  决策，不由视觉轮擅动。

## 仍待

1. **下一格（细节打磨 · 只读面的另一半）**：本轮的判据「`cardRow` 之外画一行事实 = 待查」可以直接
   当 grep 用 —— 审批记录 `PermissionBoundary` / `tool.Path` / `updateRow` 的只读臂（§6.10 的 13-5
   记作「值已在标签里」）**还没人量过它们的行盒与列**；先量再改。
2. **P13-dg-t / -l / -s**：夹具取景收口三格仍在队里（§6.75 / §6.76 / §6.77 的顺序表）。
3. **需拍板（沿用 + 第 35 份证据）**：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
4. **需拍板（沿用）**：终端会话抽屉失败只加字不换墨 / 扩展页空态只有一行结论没有出路 /
   会话页空态说错对象 / `ui/tooltip.go:133` 禁用元素不弹提示 / `cardRow` 控制槽 `Shrink(0)` /
   多选箱宽度 / 路径截断保头弃尾 / 会话行 56 地板 / 终端弧 S1b / S1c / S2 尾。
5. **附带量到、未证实**：640 宽窗下 `Settings file` 的标签仍会被省略号吃掉（§6.11 14-8 的账），
   本轮只钉住 `Project` 34px 未被截 —— 宽窗那笔仍需框架层的 `Shrink(0)` 决策。
