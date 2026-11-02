# P13-ee 规格：显式开窗那一族（`renderAt`）换到出货主题

选题：`P13-ed`「仍待」第 1 条 = `liquid-glass.md` §6.86 的 **73-5** 尾巴。
表面 = **测试夹具**（不是界面）。

## 意图

`renderAt`（`internal/launcher/modetextframe_test.go:135`）是这一包最后一个**不装主题**的
取景：它把帧的尺寸交给调用点（这一族把窗口本身当被测对象），但**帧里的墨**跟着框架的
库存主题走（`Spacing 4.0` / `FontSize 14`）。于是 13 个调用点量的是
「488px 高的卡 + 库存墨」，而 app 出货的是「439px + 3.6 / 10.8」。

为什么现在做：这是 73-5 在 `internal/launcher` 的最后一格；做完之后这一包所有像素 pin
都读同一个寄存器，后面「细节打磨」的读数不再有第二把尺子可以赖。

## 落地项（动手前逐条核过行号）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| ee-1 | `modetextframe_test.go` `renderAt` | 回调只有 `a.View(c)`；帧高由调用点给 | 回调先 `c.SetTheme(a.tokens(c).Theme)`，与 `renderShipped` / `renderIn` 同一个寄存器 |
| ee-2 | 13 个 `renderAt(... frameHeight(maxResultSlots) ...)` 调用点（`filesempty` ×3、`filesframe` ×1、`filesmarks` ×2、`modetextframe` ×5、`quietclip` ×1，其中 `quietclip` 的宽度是 `width` 不是 `InputWindowWidth`） | 帧高按库存尺记 **488** | `themedFrameHeight(maxResultSlots)` = **439** |
| ee-3 | `filesframe_test.go` / `modetextframe_test.go` 的 `sizes` 表 | 两张表各自写一遍 `{InputWindowWidth, frameHeight(maxResultSlots)}` | 收成**一个**包级寄存器 `fixtureWindowSizes`（放在 `fixtureruler_test.go` 的 pin 旁边），两处 `sizes := fixtureWindowSizes` |
| ee-4 | `quietclip_test.go:111` 的墨带 | 帧画在 439，带子按 `frameHeight(...)` = 488 算 | `themedFrameHeight(...)` |
| ee-5 | 新 pin `TestTheExplicitWindowFixtureDrawsTheShippedInks` | 这一族没有任何 pin 读「帧里的墨是不是出货的」 | 两档窗口（439 / 360）各钉：帧高 = 调用点给的、发丝 **49**（库存 55）、选中行块左沟 **7**（库存 8）；并钉 `fixtureWindowSizes[0][1]` = 出货帧高 |
| ee-6 | opt-in 出图 `TestDumpP13EEWindowFixturePNGs` | — | `/tmp/floter-P13-ee-{stock,shipped}-{light,dark}.png`（剪贴板页，`renderAt` 用得最多的表面） |

## 行为契约零改动红线

只动 `internal/launcher` 的 `_test.go`。**产品代码 0 行**：高度链、键盘模型、会话管理、
设置写回、i18n 文案、命令接线一条不碰。

## 门槛与 PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净
GOOS=windows go vet ./...      # 净
GOOS=darwin  go vet ./...      # 净
go test -count=1 ./...         # 全绿
--- PASS 计数                  # 基线 3331，只许涨
go build ×3                    # 过
```

基线：**3331 PASS / 0 FAIL / 163 SKIP / 2500 测试函数**（P13-ed 收尾实测）。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. **M1** `renderAt` 退回不装主题 → `TestTheExplicitWindowFixtureDrawsTheShippedInks`
   的发丝臂（49 → 55）与沟臂（7 → 8）同时红。
2. **M2** `fixtureWindowSizes[0][1]` 退回 `frameHeight` → 同一条 pin 的寄存器臂红
   （`the explicit-window audits draw at 488px, want the shipped frame 439px`）。
3. **M3** 把发丝的 `want` 反向写成 55 → 四条子断言全红，证明这条 pin 分得清两把尺子
   （不是「两边都是 49」里空过）。
