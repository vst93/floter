# P13-dr 规格：字段行一族的取景换成出货尺（`54u → 48.6`、`56u-1 → 49`）

## 意图

`docs/rounds/P13-dq-report.md` 把「10 条单位算术 V」排成下一格（`P13-dg-l-1`）。本轮做其中
**属于同一个表面**的那一族：**启动器的字段行（field row）与字段块的下沿（field block floor）**。
不改产品代码一个字，改的是夹具用**哪把尺子**去量它们。

## 为什么现在做（证据）

`frameHeight`（库存尺：`ui.LightTheme().Spacing = 4.0`）一直是这些 pin 的帧。app 画的是
`Spacing 3.6`（`docs/liquid-glass.md` §6.86 实测）。于是同一句话在两个坐标系里：
参考把字段行写成 `56u`、文字列写成 `16u + 28u + 10u = 54u`，而 `--u: 4px` 只在库存尺上成立。

| 位置 | 现状（实测，库存尺） | 出货尺 |
|---|---|---|
| `actionbarfooter_test.go:TestTheFieldAndTheRowsShareTheReferenceTextColumn` | caret / 首行标题 `x=54` | **48.6**（`54u × 0.9`） |
| `fieldsurface_test.go:TestTheFieldBandIsThePanelsOwnSurface` | 块底 `55`（`int(14×4.0)-1`） | **49**（`int(14×3.6)-1`） |
| `raycast_test.go:TestTheFieldBlockClosesOnOneWeakHairline` | 块底 `55` / `fieldRow 56` | **49 / 50** |
| `fieldsurface_test.go:TestNoRowScrollsUnderTheFieldBand` | 列表沿 `60` | **54**（`int(15×3.6)`） |

## 落地项

1. `internal/launcher/view_test.go` 新增两个 helper：
   - `designUnit()` = `Spacing/4`：参考的 `u` 在出货步子上是 **0.9px**（§6.86 第 1 行的唯一写法定点）。
   - `renderShipped(t, a, dark)`：装 `a.tokens(c).Theme`（`shell.View` 的装法）+ `themedFrameHeight`。
   - `tenths(v)`：把画出来的位置读成「十分之一像素」的整数，让 float32 的落点与 float64 的
     乘积比**同一个数**（不加容差带）。
2. 四个 pin 从 `render`/`renderIn` 换到 `renderShipped`，期望值改成 `54 × designUnit()` 这类乘积
   （不写死 48.6）。
3. 新增锚点 `frameruler_test.go:TestTheReferenceUnitIsNineTenths`：钉 `0.9 / 48.6 / 50.4 / 49`
   四个数，谁把 0.9 写回 1.0 或把 48.6 写回 54，这里先红。
4. 新增 opt-in dump `TestDumpP13DRFieldRowPNGs` → `/tmp/floter-P13-dr-field-<light|dark>.png`。

## 零改动红线

产品代码 0 改动（`git diff --stat` 只有 `_test.go`）：高度链、键盘模型、会话管理、设置写回、
i18n 文案、命令接线全部未触。

## 门槛

`gofmt -l cmd internal` 空 / 三平台 `go vet` 净 / 三平台 `build` 过 / `go test ./...` 全绿 /
PASS 计数不许降（基线 3324）/ 测试函数不许减（基线 2489）。

## mutation 计划

- **M1** `designUnit()` 返回 `spacing`（把「1u = 一个 Space 单位」这个经典单位错写进去）
  → 预期 `TestTheReferenceUnitIsNineTenths` + caret 两条红。
- **M2** `renderShipped` 去掉 `c.SetTheme(a.tokens(c).Theme)`（夹具回到库存尺）
  → 预期三个字段 pin 以**旧值 54 / 55** 报红，证明转换量的是出货尺。

## 判不准时的纪律

V 类：断言钉新值并注释 `P13-dr 重定基线：旧值 → 新值`。不加容差、不 `t.Skip`、不弱化。
