# P11-az 报告：列表底部那条带（toast / 电源确认）占列表的位置，不浮在列表上

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z 收完）→ 合约未落地条目（`docs/raycast-design.md`
§6 全表 `已落地`，只剩 §4.6 的 `⌘K` 动作面＝拍板）→ 上一轮「仍待」第 1 条（五条拍板项，
本轮不动）→ **界面质感与交互逻辑**（用户 2026-10-28 定的优先档）。

这一档里 P11-aq 留了一条明确没做的侦察项：「全仓其余 `Absolute()` 位置逐个量『预留高度 vs
自然高度』」。本轮把那三处量完：`actionBarFooter` 在流里（页脚是滚动器的兄弟）、终端浮条
已 `PassThrough`（P11-af），**只剩启动器底部的状态带没量过**——量下来它不是「挡了指针」，
而是**窗口为它长了 30u、列表也跟着长了 30u，那 30u 正好是它盖住的那 30u**：列表的最后
一行藏在带子底下。

## 2. 根因（实测）

- 带子：`internal/launcher/launcher.go:2952`（`feedbackRow`）与
  `internal/launcher/power.go:145`（`confirmPowerRow`），两条都是
  `ui.Row(c).Absolute().Bottom(a.footerBand)…Height(t.Space(7.5))`——**浮层**。
- 窗口：`internal/launcher/launcher.go:1170` 的 `Geometry.Feedback: a.toast != "" ||
  a.pendingPower != ""` → `internal/launcher/height.go:350` 的 `feedbackBand = 7.5 * spacing`
  ——**窗口已经为这条带长了 30px**。
- 列表：`internal/launcher/launcher.go:1299` 的 `ui.List(...).Grow(1)`——面板剩下的高度全归它，
  于是它**也**长了 30px。两者相加：多出来的 30px 就是带子盖住的那 30px。
- 实测（`720×488`，12 个工具 + 查询 `a`，未滚到底）：列表最后一条画出来的行 `india`
  `{X:54 Y:428 W:40 H:15}`（已被列表底边裁到 15u），带子的句子
  `{X:40 Y:417 W:488 H:22}`、按钮 `{X:536 Y:416}` / `{X:638 Y:416}`——**带子占
  `y≈410..440`，正是列表的最后 30u**。滚到底时那一行就是「看不见、点不到」的那一行。
- 附带一条（同一片）：`a.toast` 与 `a.pendingPower` 同时在场时，两条带落在**同一个矩形**
  （实测 `{X:40 Y:462 W:488 H:22}` 与 `{X:40 Y:462 W:664 H:22}`，同一原点、同一高度），
  而 `Geometry.Feedback` 是 `bool`——窗口只付一条带的高度，两条同时画本来就没有位置。

## 3. 落地

1. **列表的底部内边距含那条带**（`launcher.go:1295`）：
   `Padding(listEdge, Space(2), Space(0.5)+a.statusBand(c), Space(2))`。
   新方法 `statusBand`（`launcher.go:3006`）在 `a.toast != "" || a.pendingPower != ""` 时返回
   `t.Space(launcherStatusUnits)`（7.5u），否则 0——**判据与 `Geometry.Feedback` 逐字相同**，
   不新增第二个真值来源。效果：窗口长 30px、列表的内容也少 30px，列表的最后一行停在带子上沿
   （与参考把 `.launcher-feedback` 放在流里、`main:src/styles/launcher.css:693` 的
   `min-height: calc(var(--u) * 30)` 同一个答案）。
2. **一条带一个句子，问句赢**（`launcher.go:2942`）：`feedbackRow` 在 `a.pendingPower != ""`
   时先不画，时钟也不启动（`toast` 留着）；问句答完/过期后它照常出现
   （`TestTheToastComesBackAfterTheQuestion` 钉住「让位不是吞掉」）。
   为什么不是叠两条：`Geometry.Feedback` 是 `bool`，改成计数＝改高度链，红线不碰。

**没动的**：带子的几何（`Bottom(footerBand)`、`Left/Right Space(1)`、30u、`RadiusSM`、
`IconSurface`/`RaisedWarm`/`ElevContact`）、高度链（`Geometry.Feedback` 仍是 `bool`、
仍是 7.5u）、键盘模型、会话管理、设置写回、i18n 文案、命令接线。

## 4. 台账（V / B / S）

- **V = 0**：一个墨、一个面、一个几何值都没改（带子自己的盒子在 720×488 上逐像素不变）。
- **B = 0**：既有 pin 零改动全绿（`actionbarfooter`、`feedback`、`power`、`wheel`、
  `textframe`、`emptystate`、`filterband` 全过）。`Geometry.Feedback` 的语义与数字未变。
- **S = 0**：没有换结构（带子仍是浮层，只是列表的内容给它让了位）。
- 函数数 **1962 → 1966**（+4：3 条 pin + 1 条 opt-in 截图 dump）。

## 5. 门槛（本轮实测）

- `gofmt -l cmd internal`：空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**全绿**，FAIL **0**
- PASS 计数：**2415**（基线 2412，+3 = 三条新 pin）
- SKIP：**37**（基线 35；+1 是本轮 opt-in 截图 dump，其余是既有的 real-tool/registry 探针，
  与 HEAD 同一批）
- 三平台 `go build ./...`：过

## 6. mutation（两条实跑变红并还原）

- **M1** `statusBand` 恒返回 0（＝修前的底部内边距）→
  `TestTheStatusBandTakesItsSpaceFromTheList` 红：
  ```
  the list's lowest row "app-20" ends at y=449.0, inside the band whose sentence starts at y=447.0
  — the band covers the list instead of taking its space
  ```
  还原后同一条 pin：`lowest row "app-20" bottom=419.0, band sentence top=447.0`（28u 净空）。
- **M2** 删掉 `feedbackRow` 的 `pendingPower` 让位分支 →
  `TestOnlyOneSentenceWinsTheBand` 红：
  ```
  the toast shares the band with the question: [... "Could not open the system terminal"
  "Shut down this computer now? Unsaved work may be lost." ...]
  ```
  `TestTheToastComesBackAfterTheQuestion` 同步红（`the toast drew while the question was up`）。
- 两条都 `md5sum -c`（`internal/launcher/launcher.go` = `dcbbc885a9541ad40d50001c2e454f8c`）
  逐字节核对还原，`touch` 后复跑一次确认绿。

## 7. 视觉证据（`FLOTER_PNG=1`）

- `/tmp/floter-P11-az-band-light.png`、`/tmp/floter-P11-az-band-dark.png`
  （dump 里窗口按外壳的做法长了 30px，所以图上是用户真会看到的窗口）。
- **我实测看到了什么**：浅色图上 `y=443` 以上是面板底 `#F7F7F8`、`y=443..473` 是那条
  琥珀色带子（探针取色 `{217,119,6}` 是警示墨、`{248,235,227}` 是带子自己的暖面），
  带子里是「Shut down this computer now? Unsaved work may be lost.」加两个按钮；
  列表最后一行（`app-20` 与它下面的 `/usr/bin/app-20`）整条停在带子上沿之上
  （行文字底 `y=419`，带子上沿 `y=443`）——**修前那行是画在带子底下的**。
  深色图同一几何，带子 `{245,158,11}`。

## 8. 仍待

1. 五条拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1），外加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
2. **侦察结论（本轮实测，别当 TODO 重领）**：`actionBarFooter` 在流里、终端浮条已
   `PassThrough`；`Absolute()` 清单里剩下的 `filtersRow`（已 `PassThrough`，P11-ap）、
   `fieldRow`（56u 不透明底）、`settingsui` 的确认遮罩（`Modal()`）、`shell` 的圆角遮罩
   （`PassThrough`）都不是同一族缺陷。
3. **「行从带子底下滚过去」不是缺陷**：带子不透明，行在它底下与「被裁在带子上沿」在像素上
   同义（参考里带子在流里时也是裁），所以整帧的「零压字」审计会把这一条报成压字——
   那是审计只看盒子、不看谁盖谁，别拿它当回归。
