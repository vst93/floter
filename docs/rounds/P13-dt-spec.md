# P13-dt 规格 · 启动器夹具剩下的三条单位算术（pin `20u`、空页带顶 `60`、过滤条带 `60..80`）

## 意图（一句话）

`internal/launcher` 的夹具里**还剩三处**把参考的 `u` 当成 `4.0` 像素来算的 pin ——
它们量的是框架库存主题（`Spacing 4.0`）的帧，不是 app 出货的帧（`Spacing 3.6`，`u = 0.9`）；
把这三处换到出货尺，夹具就再也说不出「app 画了 20 宽」这种它从没画过的数。

为什么现在做：`§6.87` 交割了字段行一族（`54u → 48.6`、`56u − 1 → 49`），`§6.88` 交割了
状态带与编号族视口沿（`30 → 27`、`60 → 54`），`§6.87` 末尾列的「剩下的 6 条单位算术」里
现在只剩这三处（pin 控件、空页带顶、过滤条带）。它们是同一族：pin 读库存 `Spacing`。

## 先量（本轮实测，`renderShipped` 帧，`designUnit = 0.9000`）

| 量 | 库存尺（pin 此前钉的） | 出货尺（实测画出来的） | 证据 |
|---|---|---|---|
| 行内 pin 控件静息盒 | `20`（`19.5 < W < 20.5` 容差带） | **`18.00`**（20u × 0.9），x=632.60，右沿 650.60 | `TestP13DTProbe` |
| pin 按下盒 | 只断言「比静息小」 | **`16.56`** = 18 × 0.92（`transform: scale(0.92)`） | `TestP13DTProbeHeld` |
| 空页句子中线 | `60` 带顶 + `488` 帧 | 句 y=**232.10**、h=18、中线 **241.10**；带 `54 .. 428.2` 的中线 **241.1** | `TestP13DTProbe` |
| 过滤条带上沿 | `60`（15 spaces × 4.0） | **`54`**（15 × 3.6）；chip 盒 y=54.50 | `TestP13DTProbe` |
| 过滤条带下沿 | `80`（`+ 5 × 4.0`） | **`72`**（`+ filtersHeight = Space(5) = 18`） | 同上 |
| 十行帧高 | `488` | **`439`** | `themedFrameHeight` |

**病**：`60..80` 这条带子比真带子（`54..72`）往下多出 8px，于是「带子里没有行」这条断言
有一段**永远量不到的空档**（72..80 里真有行也不会红）——它不是假红，是**假绿**。

## 落地项（位置 → 现状 → 应改成）

1. `internal/launcher/powerfaces_test.go:216` `TestTheRowPinSinksLikeTheSharedFieldControl`
   - 现状：`render(t, a)`（库存尺，帧 488）；`pin.W < 19.5 || pin.W > 20.5`（容差带）；
     按下只比大小。
   - 应改成：`renderShipped(t, a, false)`；`tenths(pin.W) == tenths(5*4*designUnit())`（**无容差**）；
     按下断言 `tenths(held.W) == tenths(rest*0.92)`（参考 `main:src/styles/launcher.css:407-411`
     的 `scale(0.92)`，实测 16.56）。
2. `internal/launcher/emptystate_test.go:51` `TestTheEmptyPageSaysSoUnderTheField`
   - 现状：`render`；`bandTop = (fieldRowUnits+breathUnits) * 4`（=60）；`height = frameHeight`
     （=488）；中线比较带 `±6` 容差。
   - 应改成：`renderShipped`；`bandTop = (fieldRowUnits+breathUnits) * 4 * designUnit()`（=54）；
     `height = themedFrameHeight(maxResultSlots)`（=439）；中线与带顶都走 `tenths()` 整数比较，
     删掉 `±6`。
3. `internal/launcher/filterband_test.go:45` `chipBandScene`
   - 现状：`renderIn`；`spacing := float32(ui.LightTheme().Spacing)`（4.0），`top = 15*4`、
     `bottom = top + 5*4`。
   - 应改成：`renderShipped(t, a, dark)`；`top = (fieldRowUnits+breathUnits) * 4 * designUnit()`、
     `bottom = top + 5*designUnit()*4`（`filtersHeight` = `Space(5)`），返回 `float64` 由调用方
     走 `tenths()`；行位置比较也换 `tenths()`。
4. 出图：pin 行（悬停态）与空页，深/浅各一张，`/tmp/floter-P13-dt-{pin,empty}-{light,dark}.png`。

## 行为契约零改动红线

只改 `internal/launcher` 的 `_test.go`。**产品代码零改动**：高度链、键盘模型、会话管理、
设置写回、i18n 文案、命令接线一字不动。断言强度**只升不降**（容差带被换成十分之一像素的
整数相等，`±6` 被删掉，按下从「更小」变成「正好 0.92」）。

## 门槛命令 + PASS 基线

```
- `gofmt -l cmd internal`（必须空）
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...`（必须净）
- `go test -count=1 ./...`（全绿）
- `go test -count=1 -v ./... | grep -c -- "--- PASS"`（计数只许涨）
- 三平台 `go build ./...`

基线（P13-ds 实测）：PASS **3325** / FAIL 0 / SKIP 153 / 测试函数 2489。本轮新增两条
opt-in dump 测试函数（pin 行、空页各一条，深/浅在一条函数里循环），基线预期
PASS **3325**（不变）/ SKIP **155** / 测试函数 **2491**。

## mutation 计划（≥2 条，实跑）

- **M1** `designUnit()` 返回 `spacing`（0.9 → 3.6）：pin 报红
  `the pin's resting box is 18.00 wide (18.0), want the reference's 20u = 72.00`（数量级错），
  空页与过滤条带同时报红 —— 证明三处都真的读这个单位。
- **M2** 三处夹具退回库存尺的 `spacing`（只把乘数从 `designUnit()` 换成 `1`）：pin 期望变 20、
  空页带顶变 15×4=60、条带 60..80 —— 报红串要出现**旧值 20 / 60 / 80**，这就是
  「转换量的是出货尺」的直接证据。
- **M3**（若 M1/M2 有一条静默）把 `chipBandScene` 的 `bottom` 改回 `top + 5*4`（库存 `Space(5)`）：
  过滤条带那条应报「band top/bottom」不符。
