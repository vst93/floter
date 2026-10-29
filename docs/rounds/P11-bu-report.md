# P11-bu 报告 · 两张「实测干净」的表面变成常驻 pin

规格 `docs/rounds/P11-bu-spec.md`。选题：用户实测反馈 F1–F4 已收完（`P11-bj` / `P11-bk` /
`P11-bo` 已推送）、合约 §6 只剩拍板项、P11-bt 仍待第 1 条是拍板项、第 2/4 条判据未定、
第 5 条是上游缺口 —— 第 3 条明写「要变成常驻 pin 就起正经文件名」，本轮就是它。
**产品代码零改动**，改的是「这条不变量还被谁守着」。

## 1. 改了什么

### `internal/launcher/filesframe_test.go`（新，2 条）

- `TestTheFilesPagePaintsNoTextOverText`：`lang∈{en,zh}` × `dark∈{false,true}` × 两个窗口尺寸
  （`720×frameHeight(maxResultSlots)`、`720×360` 最短窗）× 六个状态（未拖入 / 7 个文件收起 /
  收起并滚到末尾 / 展开 / 展开并滚到末尾 / 输入不匹配的 needle），每个状态调一次
  `auditLauncherFrame`（`internal/launcher/textframe_test.go:47`，尺子一条没改）。
  夹具里有一个长到会被省略号截断的文件名和一条长目录（`Detail`）。
- `TestTheFilesPageAuditDrawsRealRows`：覆盖自证 —— 收起时空页画的是
  「Dropped files / Open report.pdf / …and 2 more files + 提示行」，第七个文件**不在**页上；
  滚到末尾后点展开器，`budget.xlsx` 出现且展开器消失。这条同时钉住
  `MaxVisibleDroppedFiles` 的语义（cap 50 那条变异就是死在它手上，见 §4 M3）。

文件页为什么值得单独立 pin：它的行是**每个文件三行**（`files.go:124 fileRows`）、标题是
`动作 + 文件名`、`Detail` 是目录路径、末尾还有一条 `…and N more files` 折叠行
（`files.go:112`）——全仓没有任何一张列表是这个形状，P11-aj 的夹具只 type 普通查询。

### `internal/settingsui/pluginsheetframe_test.go`（新，2 条）

- `TestThePluginSheetPaintsNoTextOverText`：`lang∈{en,zh}` × `dark∈{false,true}` × 两个尺寸
  （`720×900`、`720×560`，宽度是**启动器**的窗口宽度，因为表单就画在那条带里，
  `internal/shell/pluginconfig.go:180`）× 五个状态（三种已发布 schema：clipboard /
  calculator / browser，外加 clipboard 的「已武装」（按钮换问句 + 退路）与
  「两条失败行」），每个状态调 `auditSettingsFrame`（`internal/settingsui/textframe_test.go:30`）。
- `TestThePluginSheetAuditDrawsEveryField`：覆盖自证 —— 三种 schema 的标题、每个分组标题、
  每个字段的标签都必须在页上（遍历 `plugincfg.Sections`，不是手抄字符串）。

## 2. 实测：两张表都干净

两张表第一次跑就绿（不是靠放宽规则）：

```
go test -count=1 -run 'TestTheFilesPage(PaintsNoTextOverText|AuditDrawsRealRows)' ./internal/launcher/     ok
go test -count=1 -run 'TestThePluginSheet(PaintsNoTextOverText|AuditDrawsEveryField)' ./internal/settingsui/ ok
```

没有一处 overlap、没有一段字出窗。也就是说 P11-bt 报告 §6.3 说的「探针跑完即删、实测干净」，
本轮把这份「干净」变成了会一直响的断言。

调试中留下的两条实测事实（写下来，省下一轮）：

- **启动器的窗口永远只画十行**（`height.go:82 rowBudget = 10`，`RowCount` 封顶）。所以
  「7 个文件 = 17 行」这种页，末尾的折叠行**只能滚到**才看得见 —— 高度调再大也没用
  （第一版试图用 `frameHeight(2*maxResultSlots)` 的窗口把它撑出来，实测仍是十行）。
  滚轮要先 `tt.Move` 把指针放到列表上、再喂**正数** notch（负数是往回滚）、
  中间 `tt.Advance(200ms)` 等滚动动画落定（抄 `wheelscroll_test.go:57` 的写法）。
- **同一个 `App` 不能交给第二个 `ui.Tester`**：`a.View` 里的 `FocusSearch()` 会
  `a.Search.Focus()`，句柄被两个窗口绑过就 panic（`ui: Handle.Focus needs a Context when
  bound in several windows`）。展开态因此要么开新 App、要么在**同一个** tester 上点完再
  `Frame()` —— 本轮的覆盖自证用的是后者（点击展开器，比直接改私有字段更接近用户）。

## 3. 台账

- **V = 0**：没有视觉数值需要重定基线（产品墨与面一个字节没动）。
- **B = 0**：既有测试零改动全绿；产品对外行为零改动（只新增测试文件）。
- **S = 0**：没有测试因结构变化改断言。

## 4. mutation（4 条实跑变红 → `md5sum -c` 逐字节还原）

`md5sum internal/launcher/launcher.go internal/settingsui/pluginsheet.go internal/launcher/files.go
> /tmp/p11bu.md5`，每次注入前先备份、还原后核对（三份全部「成功」）。

**M1（审计对这两张脸的敏感度）** —— `launcher.go:1352` 的占位条
`ui.Box(c).Shrink(0).FillWidth().Height(listEdge)` → `Height(0)`（列表不再为字段带让位，
就是 P11-bj 修掉的那个 bug）：

```
--- FAIL: TestTheFilesPagePaintsNoTextOverText/en_light_720x488_collapsed
    filesframe_test.go:97: en light 720x488 collapsed: "Dropped files" {X:54 Y:6 W:91 H:22}
        is painted over "Search" {X:54 Y:15.5 W:512 H:25} (overlap 91.0x12.5)
    filesframe_test.go:97: en light 720x488 collapsed: "Open  quarterly-report-2026-…-final.pdf" {X:54 Y:32 W:591 H:24}
        is painted over "Search" {X:54 Y:15.5 W:512 H:25} (overlap 512.0x8.5)
```

**M2（设置尺子对表单的敏感度）** —— `pluginsheet.go` 的分组标题加
`.Margin(-t.Space(6), 0, 0, 0)`（标题向上压进上一组的卡）：

```
--- FAIL: TestThePluginSheetPaintsNoTextOverText/en_light_browser_720x900
    pluginsheetframe_test.go:86: … "Browser history" {X:14.4 Y:7.7 W:85 H:17}
        is painted over "General" {X:14.4 Y:18 W:691.2 H:19} (overlap 85.0x6.7)
    pluginsheetframe_test.go:86: … "Searches the installed browsers' history, …" {X:28 Y:72.3 W:621.8 H:17}
        is painted over "Data source" {X:14.4 Y:87.6 W:691.2 H:19} (overlap 621.8x1.7)
```

（顺带实测：`Gap(-t.Space(2))` 这条注入**不会**变红 —— 框架把负 gap 夹到 0 了。
`Margin(-t.Space(2))` 也不会（标题自己 14u 的 padding 把 -8 吃掉了），要 -24 才压穿。）

**M3（覆盖自证不是空跑 · 文件页）** —— `files.go:29 MaxVisibleDroppedFiles = 5` → `50`
（折叠行再也不出现）：

```
--- FAIL: TestTheFilesPageAuditDrawsRealRows
    filesframe_test.go:159: the collapsed files page does not draw "…and 2 more files";
        it drew ["Launcher results" "Open  screenshot.png" … "Open  budget.xlsx" "Ctrl" "7" …]
```

**M4（覆盖自证 · 表单）** —— `sheetField` 删掉 `case plugincfg.Slider:` 那一支
（滑块静默不画）：

```
--- FAIL: TestThePluginSheetAuditDrawsEveryField
    pluginsheetframe_test.go:124: clipboard: the sheet does not draw the 4 field "History size";
        it drew ["Clipboard history" "Keep a clipboard history" … "Clear history"]
    pluginsheetframe_test.go:124: calculator: the sheet does not draw the 4 field "History size"; …
```

四条都还原、`md5sum -c` 三份「成功」，`touch` 两个新测试文件后复跑绿。

## 5. 门槛（实测）

```
gofmt -l cmd internal                    空
go vet ./...                             净
GOOS=windows go vet ./...                净
GOOS=darwin  go vet ./...                净
go test -count=1 ./...                   0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2559（基线 2467，+92 个子测试）
... | grep -c -- "--- SKIP"                        → 54（不变）
grep -rhn '^func Test' … | wc -l                   → 2025（基线 2021，+4）
GOOS=linux / windows / darwin go build ./...      三份过
```

PASS 涨 92 是表驱动的子测试（文件页 2×2×2×6 里的量、表单 2×2×2×5 + 3 个 schema 的标签臂），
不是新行为。

## 6. 不出图

非视觉轮：产品墨与面**零改动**（只加测试），没有「修前 / 修后」可对比的像素。
上一轮 P11-bt 已经出过终端页的深/浅两张并逐字节比对过；本轮若出图，出来的就是同一批像素。

## 7. 仍待（P11-bu）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行）。
2. **审计的两处已知盲点**（不是缺陷，是尺子的边界，下一轮要扩就从这里挑）：
   - `tt.Find` 按**字符串**去重，所以文件页十五行共享的 `Detail: "/tmp"` 只量一次；
     要更严就得让夹具的每个目录都不同（改夹具不改产品）。
   - 滚动容器里被裁掉的行不进 `Texts()`，所以「滚到末尾」是唯一能看见末行的办法：
     本轮 `collapsed-scrolled` / `expanded-scrolled` 就是为此加的；**别用加高窗口代替滚动**
     （启动器封顶十行，见 §2）。
3. `Element.Cache` / `#753` 脏矩形仍停在「`ui.Tester` 看不见 renderer 侧」；设置页
   `actionSpinner` 仍是上游构建期缺口（P11-bt 仍待第 4、5 条，原样有效）。
