# P13-ed 规格：外观取景（`renderIn`）换到出货尺

**意图一句话**：这一包最后一族「量的不是这个设计」的取景 —— `renderIn` 是 17 个文件、
40 个调用点用的「两套外观各画一帧」取景，它仍装着框架的**库存主题**（`Spacing 4.0` /
`FontSize 14`）与库存帧高（488），于是那一族的像素断言量的是用户看不到的帧。

**为什么现在做**：`P13-ec` 把 `render()` 换到出货尺后，「仍待」第 1 条就是这一族
（`internal/launcher/appearance_test.go:19`）。它也是 `liquid-glass.md` §6.86 的 73-5
尾巴：`dg-l` 的取景债只剩它。

## 落地项（行号为动手前核对过的当前值）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `appearance_test.go:19` `renderIn` | `ui.NewTester(…, InputWindowWidth, frameHeight(maxResultSlots))`，回调里不装主题 | 装 `a.tokens(c).Theme` + `themedFrameHeight(maxResultSlots)` |
| 2 | `actionbarfooter_test.go:147/456/561/846`、`raycast_test.go:319` | `gutter := int(ui.LightTheme().Space(2))` = 8（库存尺） | `int(math.Round(8 * designUnit()))` = 7（8u × 0.9 = 7.20，painter 从 7 画） |
| 3 | `plateface_test.go:246` 悬停小格两侧列 | `img.RGBAAt(int(clear.X), cy)`（截断） | 按 painter 的像素取整 `[round(x), round(x+w))` 读，左右各一 |
| 4 | `selface_test.go:161` 电源行图标板取样 | `img.RGBAAt(int(title.X)-38, …)` = 10.6 → 板外一格（面板） | 从格子内部取 `int(cell.X)+3` |
| 5 | `selface_test.go:172` 图标墨 | 要求读数**精确等于** token 色 | 出货尺 10.8px 的描边没有全覆像素（实测覆盖率 0.90）→ 用 `coverageOver` 反解覆盖率，要求三通道同意且 ≥ 0.8；`Warning` 仍必须反解失败 |
| 6 | `fieldtail_test.go:202` 尾部 vs chips 旁注 | 阈值 `+50`（库存尺读数 84） | 出货尺实测 41 / 69 → `+30` |
| 7 | `fixtureruler_test.go` 新增 | 无 | `TestTheAppearanceFixtureDrawsTheShippedFrame`（帧高 439 / 发丝 49 / 行块左沟 7）+ opt-in 出图 `TestDumpP13EDAppearancePNGs` |

## 行为契约零改动红线

- 产品代码 **0 行**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线全部不动。
- 只改测试的**取景与读数**；没有一条断言被放宽成「非空」/加容差带/`t.Skip`。

## 门槛与 PASS 基线（实测，动手前）

- `gofmt -l cmd internal` 空；`go vet ./...` 三平台净。
- `go test -count=1 ./...` 全绿；PASS **3328** / SKIP **162** / 测试函数 **2498**。

## mutation 计划（每条说明会让哪个 pin 变红）

1. `renderIn` 退回库存尺（去 `SetTheme`、`frameHeight`）→ 新 pin 报 488 / 发丝 55 / 沟 8。
2. 尾部墨 `t.TextMuted` → `tk.TextTertiary` → `TestTheFieldTailIsStrongerThanTheChipsAside` 两支逐级相等。
3. 电源图标墨 `tokens.SystemIconText` → `tokens.Theme.Warning` → `coverageOver` 反解失败（覆盖率 0.00）。
4. 悬停小格读回 `int(clear.X)` 截断 → `TestTheHoveredFieldControlIsAFaceNotARing` 读到地面（假的「有环」）。
