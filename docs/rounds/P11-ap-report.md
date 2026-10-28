# P11-ap 报告 · 一条没有底的浮带不该吃掉它下面那行的指针

## 1 意图

启动器 chips 行（`全部 / 收藏 / 图片 / 链接 / 文件`）浮在结果列表之上、本身什么也不画，
于是底下的行在它那段 20u 带宽里照常可见。可见的行必须能被指针碰到：本轮把这条浮带对指针
透明，只留 chips 与提示字作命中目标。

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 全表已落地，
只剩 `⌘K`＝拍板）→ 上一轮「仍待」第 1 条（五条拍板项）→ **界面质感与交互逻辑**。

## 2 证据（修前实测）

夹具：剪贴板模式、40 条文本、720×488、滚轮下滚两格（`Visible()` 0 → 3）。几何：
字段行 56、chips 带子 60..80（`launcher.go:1289` + `filters.go:170`）、列表顶内边距 80
随内容滚走、行盒子 54..94（实测标题 y=70）。

- 逐点扫指针（`hoveredRow` 先归 -1）：x=200/340/700 三处在 y=60..78 全部**没有行**被
  悬停；y=56/58 与 y=82 以上正常。`40..54` 的同一段是字段块（不透明、行被盖住），吃
  指针合法。
- 点击：`ClickAt(340, 70)`（带子里）`Selected` 仍为 0；`ClickAt(340, 120)`（带子下）
  选到第 1 行 → **行看得见、点不动**。
- 参考侧不存在这个形状：`.launcher-filter` 与滚动器 `.launcher-results` 是正常流里的
  兄弟（`main:src/App.tsx:3407`、`main:src/styles/launcher.css:450`），行不从它底下过。
  本仓「行从字段底下滚过去」是既有设计，因此浮带必须自己声明透明——P11-af 的终端标题栏
  （`terminalui.go:1310`）已经因同一个根因改过一次。

## 3 落地

`internal/launcher/filters.go`：

- 绝对定位的 chips 行加 `.PassThrough()`（`ui/runtime.go:1210` 只跳过它自己的框，
  子节点照旧命中，所以 chips 仍可点）；
- 提示字 `ui.Text(...).Grow(1).TextAlign(End)` 加 `.PassThrough()`——它的盒子是整个
  `Grow(1)` 尾巴（实测连 x=340 都在里面），提示不是控件，没有可点的东西。

几何、墨、字阶、窗口高度链（`filtersHeight`）、Tab 换 chip 的逻辑一个字节没改。

## 4 台账

- **B 类**（行为/接线）：产品改动就是行为本身；既有 pin 一条没改，全绿。
- V=0：没有视觉数值被重定基线（这条改动**不画任何像素**，见 §7 的 0 像素差）。
- S=1 类：一条浮带从「命中目标」变成「透明」——新 pin 的注释写明旧结构 → 新结构。

## 5 门槛（实测）

```
gofmt -l cmd internal        → 空
go vet ./...                 → 净
GOOS=windows go vet ./...    → 净
GOOS=darwin  go vet ./...    → 净
go test -count=1 ./...       → 全绿（0 FAIL）
--- PASS                     → 2377（基线 2376）
--- SKIP                     → 28（基线 27，+1 是 opt-in 截图 dump）
测试函数                     → 1943（基线 1941）
三平台 go build ./...        → 过
```

## 6 变异（实跑，原始报错串）

1. 摘掉带子的 `.PassThrough()`：

```
--- FAIL: TestTheChipsBandLeavesTheRowsBeneathItToThePointer (0.22s)
    filterband_test.go:84: the pointer over the chips band at (340, 61) finds no row: the band is taking the pointer from the rows it covers
    filterband_test.go:84: the pointer over the chips band at (340, 64) finds no row: ...
```
   `md5sum -c` 还原成功，`touch` 后复跑绿。

2. 只摘掉提示字的 `.PassThrough()`（带子留着）：同一条 pin 同样红，x=340 与 x=660
   一起报——提示字的 `Grow(1)` 盒子确实横跨带子中段。还原、`touch`、复跑绿。

## 7 图与「我实测看到了什么」

- `/tmp/floter-P11-ap-chipband-light.png`、`/tmp/floter-P11-ap-chipband-dark.png`
  （`FLOTER_PNG=1 go test -run TestChipBandHoverDump ./internal/launcher/`）。
  这个 dump 先把指针停在带子**下面**的第 7 行（指针进入即选中），再**只移动指针**进带子：
  `row below the band 7, row in the band 4` —— 指针进带子后落到第 4 行。
- 浅色：x=340 竖切 `y=56..69` 是选中行填充 `(239,239,240)`，带子（60..80）里因此有一块
  选中底；带子下方 y>70 回到未选中的行填充 `(253,253,254)`。
- 深色：带子里 y=64..66 = `(41,41,43)`（选中行填充），带子下的面板底 y=86 = `(23,23,25)`。
- 我实测看到的：同一帧在**带子里**有一块选中底（chips 那行文字下面、紧贴第一行可见行），
  而不是像修前那样带子里空无一物。修前的 dump **拍不出这张图**——它的卫语句直接触发
  `no row under the band to photograph`：指针在带子里找不到任何行。
- 这条改动本身**不画像素**：修前/修后同一帧的逐像素比对是 **0 差**（除了悬停带来的选中底
  位移）。所以本轮没有「配色/边线」类的前后图，只有上面这张「带子里的选中底」。

## 8 顺手否掉的两条（别当 TODO 重领）

1. **列表在结果变少时会不会留着越界的滚动偏移？——不会。** 31 条结果滚到 18，
   把查询改成只剩 2 条后 `Visible()` 回到 **0..0**（只留 3 条时 0..1）：框架在内容变短时
   自己夹住偏移。
2. **零结果时 `a.List.Visible()` 仍报上一帧的 18..29**：那时列表根本没建（画的是空态那句
   话），没有行被画出来，`Visible()` 的返回值是上一帧的残留。量它没有意义，不是 bug。

## 9 仍待（P11-ap）

1. **同族的下一条**：`internal/launcher/output.go:467` 的输出面板标题带也是「绝对定位 +
   无填充 + 跨满宽」浮在滚动内容上（`ui.Column(c).Absolute().Top(edge).Left(Space(3))
   .Right(Space(3))`，标题与右侧三个 `28u` 按钮），而输出正文的内边距是 `edge+header`、
   滚动时从它底下过。按 P11-af/P11-ap 的判据它也得 `PassThrough`（按钮保留）。本轮没做，
   因为要在输出面板上量「底下那行文字能不能 drag-select」得先有新的夹具。
2. 五条拍板项原样有效（caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1），外加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
3. 侦察结论：设置页关闭再打开是否回顶仍无证据（P11-ao 已记）。
