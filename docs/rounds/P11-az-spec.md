# P11-az 规格：列表底部那条带（toast / 电源确认）要占列表的位置，不要浮在列表上

## 0. 意图

启动器底部的「状态带」（`feedbackRow` 的 toast、`confirmPowerRow` 的关机/重启确认）是
`Absolute().Bottom(a.footerBand)` 浮层；而窗口**已经**为它长了 30u（`Geometry.Feedback`）。
两者相加的后果：列表的视口跟着长了 30u，那条 30u 又整块盖在列表上——**列表的最后一行
藏在带子底下**，滚到底也看不见、点不到。本轮让带子真正占掉那 30u（列表的底部内边距），
并让「两条带同时在场」只有一个说法（问句优先），符合本仓「一条带一句话」的高度契约。

为什么现在做（用户 2026-10-28 定的优先档：界面质感与交互逻辑）：这是 P11-aq 记下的
「全仓其余 `Absolute()` 位置逐个量『预留高度 vs 自然高度』」那一族里，**最后一处没量过的**
（`actionBarFooter` 已在流里、终端浮条已 `PassThrough`）。量下来它不是「挡了指针」，是
**带子占了窗口的高度、却没占列表的高度**。

## 1. 实测（本轮，`720×488`，12 个工具 + 查询 `a`）

| 元素 | 修前盒子 |
|---|---|
| 列表最后一条**画出来的**行 `india` | `{X:54 Y:428 W:40 H:15}`（被列表底边裁到 15u） |
| 状态带的句子（`Shut down this computer now? …`） | `{X:40 Y:417 W:488 H:22}` |
| 状态带的两个按钮 `Shut Down` / `Cancel` | `{X:536 Y:416 W:94 H:24}` / `{X:638 Y:416 W:66 H:24}` |
| 固定页脚（动作条）第一行 `Run in shell` | `{Y:470 H:18}` |

- 带子占 `y≈410..440`，列表的视口到 `y≈440`（页脚 `footerBand`=45u 之上）：**列表的
  最后 30u 就是带子**。`india` 的 `y=428..443` 整个落在带子里。
- 窗口为带子长了 `7.5u × spacing = 30px`（`internal/launcher/height.go:350`），列表
  `Grow(1)` 跟着长 30px，于是「多出来的 30px」正好是带子盖住的那 30px——净效果是
  **滚到底时最后一行永远在带子底下**（带子在场时：toast 4.5s、电源确认 3s）。
- 两条带同时在场（`a.toast` 有字 + `a.pendingPower` 有值）时，两个 `Absolute()` 落在
  **同一个矩形**：`{X:40 Y:462 W:488 H:22}`（确认句）与 `{X:40 Y:462 W:664 H:22}`
  （toast 句）**逐像素重合**，而 `Geometry.Feedback` 是 `bool`——只给了一条带的高度。

## 2. 落地项

### 2.1 列表的底部内边距要含那条带

- 位置：`internal/launcher/launcher.go:1299-1302`（`ui.List(c.Key("launcher.results") …)`
  的 `Padding(listEdge, t.Space(2), t.Space(0.5), t.Space(2))`）。
- 现状：底部 0.5u（2px）——带子在场时那 30px 是列表的一部分。
- 应改成：底部 `t.Space(0.5) + a.statusBand(c)`，其中 `statusBand` 在
  `a.toast != "" || a.pendingPower != ""` 时返回 `t.Space(launcherStatusUnits)`（7.5u=30px），
  否则 0。`statusBand` 与 `Geometry.Feedback` 读**同一个判据**（`a.toast != "" || a.pendingPower != ""`），
  不新增第二个真值来源。

### 2.2 两条带同时在场：问句优先

- 位置：`internal/launcher/launcher.go:2940`（`feedbackRow` 开头）。
- 现状：toast 与确认句画在同一个矩形里；`Geometry.Feedback` 只报一条带的高度，
  所以「两条都画」在高度链上本来就没有位置。
- 应改成：`feedbackRow` 在 `a.pendingPower != ""` 时**先不画**（时钟也不启动，`toast` 留着），
  问句答完/过期后它再出现。一句话：一条带只有一个句子，问句赢。
- 为什么不是「叠两条」：`Geometry.Feedback` 是 `bool`，改成计数＝改高度链（红线）。

### 2.3 不动的部分（行为契约红线）

高度链（`Geometry.Feedback` 仍是 `bool`、仍是 7.5u）、键盘模型、会话管理、设置写回、
i18n 文案、命令接线、带子的几何（`Bottom(footerBand)`、`Left/Right Space(1)`、30u、
`RadiusSM`、`IconSurface` / `RaisedWarm`）**一个字节不改**。视觉轮只改「占位」。

## 3. 门槛

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # PASS 计数只许涨
三平台 go build ./...
```

PASS 基线（本轮实测 HEAD `e9868b1`）：**2412**，FAIL 0，SKIP 35，测试函数 **1961**。

## 4. 新 pin（两条，都要「修前红 / 修后绿」）

1. `TestTheStatusBandTakesItsSpaceFromTheList`（`internal/launcher/statusband_test.go`）：
   长列表滚到底 + 电源确认在场 → 列表**最后一条画出来的行**的盒子底边 `<=` 带子顶边
   （修前：`india` 的 `Y+H=443` 落进带子 `410..440`）。
2. `TestOnlyOneSentenceWinsTheBand`：`toast` 与 `pendingPower` 同时在 → 确认句在场、
   toast 的句子不在场（`HasText`），并且 `auditLauncherFrame` 在整帧上**零压字**
   （修前：两条句子逐像素重合 → 压字 488×22）。

## 5. mutation 计划（≥2，实跑）

- **M1**：把 `statusBand` 恒返回 0（＝恢复修前的底部内边距）→ pin 1 红。
- **M2**：删掉 `feedbackRow` 的 `pendingPower` 让位分支 → pin 2 红（压字）。
- 每条都 `md5sum -c` 逐字节核对还原，`touch` 后复跑一次确认绿。
