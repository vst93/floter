# P11-ay 报告：抽屉「滚到末」的 pin 钉到真正被裁的那一行

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目
（`docs/raycast-design.md` §6 全表 `已落地`，只剩 §4.6 的 `⌘K` 动作面＝**拍板**）→
**上一轮「仍待」第 1 条**（P11-ax 报告 §6 的 M2 实测发现：
`TestTheDrawerScrollsItsLastRowsIntoView` 对「滚动是否工作」**不敏感**）。

这不是像素细节，是**交互逻辑的正确性**：那条 pin 声称钉「抽屉放不下时最后几行能被滚到」，
但它断言的 `Terminal palette` 行在 `640×360` 的 `Y=0`（未滚）时就已经整行在视口里，
于是即使把滚动彻底关掉也照样绿——一条钉不住任何东西的 pin。方向属用户 2026-10-28 定的
优先档（界面质感与交互逻辑 + 交互正确性）。

## 2. 根因（实测）

`640×360`（`en`，`Y=0` 与滚到末两种状态）实测抽屉网格各行几何：

| 行 | Y（Y=0） | H（Y=0） | Y（滚到末） | H（滚到末） |
|---|---|---|---|---|
| `Terminal appearance`（条带） | 28 | 332 | 28 | 332 |
| `Font size` | 46 | 22 | — | — |
| `Terminal palette` | 224 | 22 | 191 | 22 |
| `Palette preview` | 259 | 88 | — | — |
| `Copy on select`（左列末行） | **357** | **3** | 324 | 22 |
| `Safe paste`（右列末行） | **357** | **3** | 324 | 22 |

- **真正被裁的是网格最后一行**（`Copy on select` / `Safe paste`）：`Y=0` 时它们的盒子
  只有 3u 高（被条带底边 360 裁掉 19u）。
- **旧的 pin 断言的是 `Terminal palette`**（网格**最宽**的那一行，排在中间）：它在
  `Y=0` 时 `Y=224 H=22`，整行落在视口里，所以「滚到末」那一步对滚动不敏感。

## 3. 落地

`internal/terminalui/drawerscroll_test.go` 的 `TestTheDrawerScrollsItsLastRowsIntoView`：

1. **前置断言**（新）：`Y=0` 时 `tt.Find(scopy.PasteSafe)` 的 `H < wholeRow`（20u）——
   夹具必须真的让最后一行被裁，否则测试空转（`t.Fatalf` 报「fixture does not exercise
   scrolling」）。
2. **滚到末**：`a.DrawerScroll.Y = math.MaxFloat32` → `tt.Frame()` → 断言
   `tt.Find(scopy.PasteSafe)` 的 `H >= wholeRow`（回到整行高）。
3. 保留原有的「落在条带视口内」矩形断言。

`wholeRow = 20.0`：网格最后一行是 `drawerRow` 的 `MinHeight(t.Space(7.5))`（默认档 22u）；
被裁时实测 `H=3`，整行时 `H=22`。20 是「整行」的保守下界（默认档与各档都成立）。

**为什么钉 `PasteSafe` 而不是 `Palette`**：`PasteSafe` 是网格**最后一行**，`Y=0` 时被裁得
最狠（3u）；`Palette` 是网格**最宽**的那一行，排在中间，永远在视口内——这正是旧 pin 失效
的原因。

## 4. 台账（V / B / S）

- **V = 0**：产品墨与面零改动。
- **B = 0**：行为契约零改动（高度链、键盘模型、会话管理、设置写回、i18n、命令接线全不碰）。
- **S = 0**：结构没变，只改了**一条既有 pin 的断言**（钉哪一行）+ 新增一条 opt-in 截图 dump。
- **产品代码零改动**：`internal/terminalui/drawer.go` 一个字节没动
  （`md5sum` = `957c841cddbc5c7b851406528a78efec`，与 P11-ax 报告一致）。
- 既有 pin：`internal/terminalui` 全套零改动全绿（含 `TestTheDrawerDoesNotScrollWhenItFits`、
  `TestReopeningTheDrawerStartsAtItsTop`、`TestTheDrawerSeamIsOnItsTopEdge` 等）。

## 5. 门槛（本轮实测）

- `gofmt -l cmd internal`：空
- `go vet ./...`：净
- `GOOS=windows go vet ./...`：净
- `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**全绿**，FAIL **0**
- PASS 计数：**2412**（基线 2412；本轮是「收紧一条既有 pin」而不是「加新用例」，
  所以 PASS 计数持平——测试函数数 1961 不变，没有新增 `t.Skip`）
- SKIP：35（与 HEAD 逐条一致；`git stash` 掉本轮的改动后实测也是 35，是环境相关的
  real-tool/registry 探针，**本轮改动新增 0 条 skip**）
- 三平台 `go build ./...`：见下

> 关于「PASS 只许涨」：本轮交付的是「让一条既有 pin 对滚动敏感」，属修正而非新增覆盖。
> 函数数 1961 前后不变（本轮没有加/删测试函数）。收紧后的 pin 见 §6 的变异证明。

## 6. mutation（两条实跑变红并还原）

`md5sum` 逐字节核对还原（`drawer.go` = `957c841cddbc5c7b851406528a78efec`），`touch` 后复跑确认绿。

**M1** —— 在 `drawer()` 的 `strip.TrackScroll(&a.DrawerScroll)` 之前加一行
`a.DrawerScroll = ui.ScrollState{}`（每帧清零，即 P11-ax 报告 §6 的 M2 缺陷）：
```
--- FAIL: TestTheDrawerScrollsItsLastRowsIntoView
    drawerscroll_test.go:93: en 640x360: the last row ("Safe paste") is still clipped
    after scrolling to the end (H=3.0 < 20.0) — the scroll did not bring it into view
    （同样 720x360 / zh 两格）
```
**这条正是旧 pin 会漏掉的**：旧 pin 断言 `Palette`，每帧清零后它仍在视口里，旧断言照样绿。

**M2** —— 删掉 `.TrackScroll(&a.DrawerScroll)`（条带不再报范围）：
```
--- FAIL: TestTheDrawerScrollsItsLastRowsIntoView
    drawerscroll_test.go:61: en 640x360: the drawer reports no scroll range (MaxY=0.0) —
    the strip is 332.0u tall and its content does not fit, so the last rows are clipped
    with no way to reach them
```

两条都还原后 `md5sum -c` 一致、`git diff` 只剩 `drawerscroll_test.go` 一处。

## 7. 视觉证据

`/tmp/floter-P11-ay-drawer-lastrow-{light,dark}-{clipped,whole}.png`（`FLOTER_PNG=1`
的 opt-in dump，`TestDumpP11AYDrawerLastRowPNGs`）。

**我实测看到了什么**：`640×360` 浅色。`clipped` 图里右下角的 `Safe paste` 标签**没有可读
墨迹**（右列 x=329..430 只有它上一行 y=300..312 的墨；`Safe paste` 的盒子在 y=357 只剩 3u，
字被裁掉）；滚到末的 `whole` 图里 `Safe paste` 是**一整行字**（y=329..344，16 行墨）。
左列的 `Copy on select` 同理：`clipped` 只剩字头（y=319..330，每行墨 1..10），`whole`
是整行（y=329..344，每行墨 35..50）。这正是收紧后的 pin 量的那处变化。
