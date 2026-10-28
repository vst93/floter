# P11-at 报告：抽屉不让出浮条那条带

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 只剩 `⌘K`
动作面＝拍板）→ 上一轮「仍待」第 1 条（P11-as 的 zh 本地化量测，本轮量过，见 §7）→
**界面质感与交互逻辑**。这一档自己找，找到的是**终端窗口缩到最小高度再打开外观抽屉时，
抽屉画到浮着的标题条底下**。

先按「不许猜」把三条早已存在的文字审计扩到它们没覆盖的尺寸与语言（launcher 720 定宽 × zh、
设置页五页 × 720…1200 × zh、抽屉 520…1400 × zh）——**一处重叠都没有**，说明那三层已经被
P11-aj/as 钉住了。真正出问题的尺寸是**比它们都小的那一档**：终端自己的最小窗口 640×360 /
720×360。

## 2. 根因

面板是一列：画布 `Grow(1)`、抽屉（`Shrink(0)`，网格自然高 365@720 宽）、标题条
（`Absolute().Top(0).Height(Space(7))`=28u，浮在最上层）。**画布没有地板高度**，抽屉按自然
高度要位置；面板最小高度 360 < 365，于是画布被压到 0、抽屉从 y=0 起画，与浮条重合。

修前实测（`ui.Tester` 的绘制盒，原文）：

```
en 720x360: "Font family" {X:12 Y:17 W:81 H:22} paints over "Terminal" {X:12 Y:3 W:628 H:22} (overlap 81.0x8.0)
en 720x360: "Close terminal settings" {X:680 Y:10 W:28 H:28} paints over "Close terminal" {X:680 Y:0 W:28 H:28} (overlap 28.0x18.0)
en 720x360: "14 px" {X:642 Y:19.5 W:28 H:17} paints over "New command" {X:648 Y:0 W:28 H:28} (overlap 22.0x8.5)
zh 640x360: "字体" {X:12 Y:17 W:28 H:22} paints over "终端" {X:12 Y:3 W:548 H:22} (overlap 28.0x8.0)
zh 640x360: "关闭终端设置" {X:600 Y:10 W:28 H:28} paints over "关闭终端" {X:600 Y:0 W:28 H:28} (overlap 28.0x18.0)
en 720x380: "Close terminal settings" {X:680 Y:25 W:28 H:28} paints over "Close terminal" {X:680 Y:0 W:28 H:28} (overlap 28.0x3.0)
```

最刺眼的一条是**两个 X 叠成一个**：抽屉自己的关闭钮（`Close terminal settings`）与浮条的
关闭钮（`Close terminal`）落在同一块 28×28 上，指针分不出点的是哪一个。

## 3. 落地

`internal/terminalui/terminalui.go`：画布 `body` 加 `MinHeight(t.Space(7))` —— 浮条的带宽
就是画布的地板（`Space(7)` 正是 `titleBar` 自己的高度，不是新猜的常量）。抽屉因此永远排在
浮条之下；面板放不下时它的最后几行被**面板底边**裁掉，而不是爬上去压标题。

`internal/terminalui/drawer.go`：只加注释（这条地板的由来与后果 + 为什么没做成滚动），
几何/墨/字阶/padding/gap/关闭钮**零改动**。

改后实测（同一个夹具）：

| 窗口 | 抽屉条带（`Find(GroupTerminal)`） | 抽屉关闭钮 | 浮条关闭钮 | 重叠 |
| --- | --- | --- | --- | --- |
| 640×360 | `{X:0 Y:28 W:640 H:332}` | `{X:600 Y:38 W:28 H:28}` | `{X:600 Y:0 W:28 H:28}` | 无 |
| 720×360 | `{X:0 Y:28 W:720 H:332}` | `{X:680 Y:38 W:28 H:28}` | `{X:680 Y:0 W:28 H:28}` | 无 |
| 720×400 | `{X:0 Y:35 W:720 H:365}` | 同修前 | 同修前 | 无（**未变**） |
| 720×420 | `{X:0 Y:55 W:720 H:365}` | 同修前 | 同修前 | 无（**未变**） |
| 720×600 | `{X:0 Y:235 W:720 H:365}` | 同修前 | 同修前 | 无（**未变**） |
| 900×360 | `{X:0 Y:63 W:900 H:297}` | 同修前 | 同修前 | 无（**未变**） |

即：**高度够的窗口一像素不变**，只有放不下的那一档换了答案。

## 4. 台账（V / B / S）

- **V = 0**：没有一个墨、半径、字号、padding、gap 被改。
- **B = 0**：既有 pin 零改动全绿（`TestTheTerminalDrawerShowsTheAppearanceRows`、
  `TestTheDrawerGridCellsKeepTheReferenceFloor`、`TestTheDrawerRowsKeepTheirWordsInsideTheirCells`、
  `TestTheTerminalViewsPaintFromTokensNotLiterals` 等）。
- **S = 0**：没有结构变更（`strip` 仍是 `Shrink(0)` + `AlignItems(Start)`，抽屉仍是画布之后的兄弟）。
- 新增：`internal/terminalui/drawerband_test.go` 的 `TestTheDrawerKeepsTheBarsBand`（2 语言 × 3 尺寸
  = 6 个子断言组，三条判据：抽屉顶不高于浮条带底 / 两个关闭钮不重叠 / 抽屉带内无文字重叠）
  + opt-in 截图 dump `TestDumpP11ATDrawerBandPNGs`；`auditDrawerBand` 拆出
  `auditDrawerBandFrom(label)`，让非英文（`zh` 的「终端外观」）用同一条规则审计，原有调用点零改动。

## 5. 「为什么没顺手做成滚动」（实测否掉）

正确形态本该是「抽屉放不下就在自己那条带里滚动」。本轮真的试过：条带 `Shrink(1)` +
网格包进 `ui.Scroll(...).TrackScroll(&a.DrawerScroll).Grow(1)`、画布 `MinHeight(Space(7))`。
结果**滚动容器不报范围**：640×360 下 `DrawerScroll.MaxY` 恒为 **0**（滚轮打两帧、两个方向都试过），
同时内容仍被视口裁掉——网格还会被压扁，`Safe paste` / `Copy on select` 的绘制盒回成 **H:4**
（22u 的行被压到 4u）。把网格从 `Grow(1)` 换成 `FillWidth()` 只解决压扁，`MaxY` 仍是 0。
于是**不硬凑**：本轮只落「不让出浮条带」这条，滚动留成下一条（见 §9 第 1 项），
并把「`MaxY` 恒为 0」这条实测留在报告里，免得下一轮从头再撞一次。

## 6. 图（`FLOTER_PNG=1`）

`/tmp/floter-P11-at-drawer-min-light.png`、`/tmp/floter-P11-at-drawer-min-dark.png`
（640×360 = 最小宽 × 最小高，终端页 + 一个会话 + 抽屉打开）。

我实测在这两张图上看到的：**y=0…27 是一整条画布色**（浅 `(255,255,255)` / 深 `(24,24,27)`，
整行 640 个像素一个颜色都没变——浮条在指针不在它上面时不画字、不画面），**y=28 起整行变成
抽屉的 `Control` 面**（浅 `(239,239,240)` / 深 `(38,38,40)`，同样 640 个像素一色），
再往下（y=40）才是行与行的字。也就是说：抽屉的上沿正好压着浮条那 28u 的下沿，中间没有一条
缝、也没有一块被两段字共用的像素；而修前这条分界是**不存在**的——抽屉从 y=0 起就是
`Control` 面，浮条的标题与 X 直接画在那块面上。

## 7. 上一轮「仍待」第 1 条：量过了，没有重叠（别当 TODO 重领）

P11-as 留下的疑虑是「本地化：`zh` 的标签更长」。本轮把三条审计扩到它们没覆盖的面上量了一遍，
**一处重叠都没有**：

- 抽屉：`en`/`zh` × 520/640/720/900/1100/1400 × 深/浅 —— 0 条重叠、0 条出窗。
- 启动器（720 是它的定宽）：`zh` × 空态/命中/无结果 × 深/浅 —— 0。
- 设置页五页 × 520…1200 × `en`/`zh` × 深/浅 —— 0。

结论：**文字重叠这一族在「够大的窗口」里已经收口**，剩下的唯一缺口是**最小尺寸那一档**，
也就是本轮修的这条。

## 8. 门槛（全部实测）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → FAIL 0
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2386（基线 2385）
grep -c -- "--- SKIP"                                  → 31（基线 30，+1 opt-in dump）
func Test 计数                                          → 1951（基线 1949）
三平台 build                                            → 过
```

变异 2 条**实跑变红并还原**（`md5sum` 逐字节核对 = `599c3a6523ce6823e9364f704e51675e`）：

1. 删掉 `MinHeight(t.Space(7))` →
   `drawerband_test.go:53: en 640x360: the drawer starts at y=0.0, the bar's band ends at y=28.0`，
   并连带报出 §2 那三条修前原文（`81.0x8.0` / `28.0x18.0` / `22.0x8.5`）。
2. `Space(7)` → `Space(3)` →
   `en 640x360 / en 720x360 / zh 640x360: the drawer starts at y=12.0, the bar's band ends at y=28.0`
   —— pin 量的是「与浮条的关系」，不是某个写死的数。

## 9. 仍待（P11-at）

1. **抽屉在最小高度下最后几行被面板底边裁掉**（本轮已知后果）：`640×360` 时 `Safe paste` /
   `Copy on select` 只剩 4u 可见。正解是让那条带滚动，但这需要一个**会报范围的滚动容器**
   ——实测本布局里 `ui.Scroll` 的 `MaxY` 恒为 0（§5），要先弄清楚框架的测量条件。
   **别再用「把网格包进 Scroll」重领一次**，那条路本轮已经走到头。
2. 上一轮那条拍板项原样有效：外观抽屉要不要在窄窗口（< 900）折成**单列**。
3. 五条老拍板项原样有效（caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 / `AccentEdge` 3:1），
   外加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
4. 侦察结论（本轮实测否掉，别当 TODO 重领）：文字重叠审计在「够大的窗口」里已经三个面全清
   （§7 的六组量测）。
