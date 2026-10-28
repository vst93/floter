# P11-aw 报告：插件模式页也进「一条面上没有两段字压在同一块像素上」

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目
（`docs/raycast-design.md` §6 全表 `已落地`，只剩 §4.6 的 `⌘K` 动作面＝**拍板**）→
上一轮「仍待」第 1 条（抽屉重开的滚动偏移＝**交互决定，需拍板**）→ **界面质感与交互逻辑**
（用户 2026-10-28 定的优先档）。

P11-aj 立了这条不变量，但它的夹具（`internal/launcher/textframe_test.go`）只 type 普通查询，
**从没走过插件模式页**。而三张模式页与普通页的布局不共用：

- 各自的筛选 chips 带（`filters.go` 的 `filtersRow`）浮在字段与列表之间；
- 字段行左侧放 **scope mark**（模式的图标 + 词），不是放大镜；
- 行型不同：剪贴板缩略图 + 时间戳、计算器「表达式 + 右侧答案」、浏览器 tab/书签分组；
- 没有页脚。

这四处都是「字压字」的常见现场。本轮把同一把尺子量到这三页，并在两种窗口尺寸
（默认卡 720×488、窗口最小高 720×360）× 两外观 × 每页空态下都量。

## 2. 结论：三张模式页是干净的（本轮没找到要修的产品缺陷）

实测 20 个子测试全绿，**零压字、零出框**。这不是「审计空转」——第 5 节两条 mutation 实跑
证明这把尺子会红。所以本轮的产出是**一条新的不变量覆盖 + 它自己的非空转证明**，产品代码
一个字节没改（与 P11-aj 同形态：P11-aj 的报告也写着「三表面零压字、零出框，产品墨与面零改动」）。

## 3. 落地

新增 `internal/launcher/modetextframe_test.go`：

| 测试 | 作用 |
|---|---|
| `TestThePluginModesPaintNoTextOverText` | 剪贴板 / 计算器 / 浏览器 × 两外观 × 两尺寸（720×488、720×360）× 每页空态 = 20 个子测试，逐个跑 `auditLauncherFrame` |
| `TestThePluginModesAuditDrawsRealRows` | 断言每个模式确实画出了行（防「空列表让审计白过」） |
| `TestDumpPluginModePNGs` | opt-in（`FLOTER_PNG=1`）导出六张 `/tmp/floter-P11-aw-<mode>-<light\|dark>.png` |

夹具里每个模式放一条「长到被省略号截断」的标题：

- 剪贴板：`clip 05 — a line long enough that the row's own ellipsis has to cut it before the timestamp`；
- 计算器：`(0 + 1) * (2 - 3) / 4` 这类长表达式 + `0.0000` 宽答案；
- 浏览器：`Effective Go — a documentation page whose title is long enough to be ellipsized in its row`。

复用 `textframe_test.go` 的 `auditLauncherFrame`（P11-aj 的尺子：两盒两轴都重叠 > 1 DIP 算压字，
除非一盒包含另一盒；盒越出窗口算从没被看见；整窗宽的盒按构造跳过）。

## 4. 台账（V / B / S）

- **V = 0**：产品代码零改动（没有一条墨/面/几何变了）。
- **B = 0**：行为契约零改动（高度链、键盘、会话、设置写回、i18n、命令接线）。
- **S = 0**：结构没变，只加测试文件。
- 既有 pin：`internal/launcher` 全套零改动全绿（含 `TestTheLauncherPaintsNoTextOverText`、
  `TestTheDrawerRowsKeepTheirWordsInsideTheirCells` 等）。

## 5. mutation（两条实跑变红并还原）

`md5sum` 逐字节核对还原（`filters.go` = `05d874a8df5fc71d04928d0923b2fe23`，
`git diff --stat` 对 `filters.go`/`launcher.go` 均为空），`touch` 后复跑确认绿。

**M1** —— `internal/launcher/filters.go:236`，筛选提示从 `.Grow(1).TextAlign(ui.End)` 改成
`.Absolute().Top(0).Left(0)`（提示字落到 chips 行左上角，压在第一枚 chip 上）：

```
light 720x488 clipboard: "All" {X:16 Y:59 W:20 H:22} is painted over
  "Tab to switch · Ctrl+D to favorite" {X:0 Y:60 W:199 H:20} (overlap 20.0x20.0)
light 720x488 clipboard: "Favorites" {X:50 Y:59 W:62 H:22} is painted over
  "Tab to switch · Ctrl+D to favorite" {X:0 Y:60 W:199 H:20} (overlap 62.0x20.0)
```

**M2** —— `internal/launcher/filters.go:210`，chip 从 `.Focusable().Shrink(0)` 改成
`.Focusable().Absolute().Left(0)`（所有 chip 叠在行首）：

```
light 720x488 clipboard: "Tab to switch · Ctrl+D to favorite" {X:16 Y:60 W:688 H:20} is painted over
  "All" {X:0 Y:60 W:20 H:22} (overlap 4.0x20.0)
light 720x488 clipboard: "Tab to switch · Ctrl+D to favorite" {X:16 Y:60 W:688 H:20} is painted over
  "Favorites" {X:0 Y:60 W:62 H:22} (overlap 46.0x20.0)
```

两条都只让 `TestThePluginModesPaintNoTextOverText` 红，产品其它 pin 不动（改动在插件模式页
专属的 `filtersRow` 上，普通页不画 chips 带）。

## 6. 图（`FLOTER_PNG=1`）

导出六张（`/tmp/floter-P11-aw-{clipboard,calculator,browser}-{light,dark}.png`，720×488）。
用眼睛与像素复核看到的变化/事实：

- **三张模式页的第一行都在 chips 带之下**：默认卡上字段带 `y=0..56`、chips 带 `y≈56..80`、
  第一行块从 `y≈80` 起（`row1` 采样 `y=110` 是行块自身的填充）。
- **scope mark 是中性墨，不是 accent**：浅色字段行左槽最深的墨 `(113,113,122)`（`TextMuted`
  一族），三张模式页一致——模式名与放大镜同一个静息档，不花 accent。
- **选中 chip 走主文字档**：浅色第一枚 chip（`All`，选中）最深的墨 `(24,24,27)` ≈ `Theme.Text`
  （`#1B1B1D`）；未选中的 chip 在它右侧更浅。
- 深色下字段填充 `(28,28,31)`、chips 带 `(23,23,25)`（面板底）、行块 `(22,22,24)`——
  三层明度分得开，字读得清。

## 7. 门槛（本轮实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 42 包全 ok，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2411（基线 2389）
grep -c -- "--- SKIP"                    → 34（基线 33，+1 opt-in dump）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 1959（基线 1956，+3）
GOOS=linux/windows/darwin go build ./... → 三平台过
```

## 8. 仍待

1. **抽屉关闭再打开时的滚动偏移**（P11-au/av 仍待 1）原样有效：保留偏移；要「一打开就在
   顶部」需拍板。
2. 老拍板项原样有效：外观抽屉 < 900 折单列 / caret / 默认浅色 3.04:1 / 终端滚动缓冲 /
   `⌘K` 动作面 / `AccentEdge` 3:1 / P11-an 的终端便条投影。
3. **侦察结论（本轮实测，别当 TODO 重领）**：三张插件模式页在 720×488 与 720×360 × 两外观 ×
   空态下 **0 压字、0 出框**；`auditLauncherFrame` 的「出窗」臂对**位置**改动不敏感（框架把
   `Absolute().Left(900)` / `.Left(-200)` 夹回父盒内），只对**重叠**敏感——两条 mutation 都
   走重叠臂。若要专门钉「出窗」，得另造一个「内容比父盒宽」的夹具（本轮未做，记在这里）。
