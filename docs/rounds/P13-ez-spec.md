# P13-ez 规格：透明度组的那句「答案」比它回答的数字还小一档

## 意图（一句话）

设置页 · 常规页 · **透明度组**里唯一一行**读数**（`EffectiveOpacity` + `47%`）此前穿 caption 档
（盒高 15），而它回答的那个数字——滑块自己的实时读数——穿正文档（盒高 17）：**同一张卡里
同一角色（「这一组说的是什么数」）两个字号档，且答案比问题小一档**。本轮把这行抬回正文档。

为什么现在做：P13-ey（§6.120）为「说明句」一族立了规矩——「一页之内同一角色的东西只有一个
字号档，层级由**墨**表达，不由字号表达」。它当时没量到这行，把它留在「仍待」第 1 条
（`docs/rounds/P13-ey-report.md`）。§6.118（P13-ew）刚刚把它的**缝**补齐、并把它定义为
「这一组的答案」——一个答案不该是这一组里最小的字。

## 证据（实测，`go test -run TestProbeP13EZReadoutStep`，860×2400 真页面，深浅两套）

| 行 | 位置（真页面 Y） | 字号档 | 盒高 | 墨 |
|---|---|---|---|---|
| `Main opacity`（行名） | 1442.4 | `tokens.Emphasis` | 18.0 | `TextStrong` 580 |
| `47%`（滑块的实时读数） | 1442.9 | `t.FontSize` = 正文档 | **17.0** | `TextStrong`（默认重） |
| `Transparent` / `Solid`（轴词） | 1486.6 | `tokens.Caption` | 15.0 | `t.TextMuted` 500 |
| `Effective opacity`（答案的名字） | 1589.7 | `tokens.Caption` | **15.0** | `TextTertiary` 500 |
| `47%`（答案本身） | 1589.7 | `tokens.Caption` | **15.0** | `TextSecondary` 560 |

深浅两套的盒高逐值相同（字号档与主题无关），所以这不是外观问题。

两处病，都是**结构**：

1. **同一角色两个档**：滑块读数是「值」，走正文档（17）；这一组的总读数是「值」，走 caption（15）。
   同一张卡里两个同类数字，眼睛要在第二处重新校准一次字号。
2. **答案比问题小**：读数是这一组的结论（§6.118 原话「这一组的答案，不是又一行」），
   却是全卡唯一一个**比它上面那条轴词还同级、比它回答的数字低一档**的东西。墨已经把层级说清
   （答案 `TextSecondary` 560 比行名 `TextStrong` 安静），字号不该再说一遍，尤其不该说反。

## 逐条落地项

| # | 位置 | 现状 → 落成 |
|---|---|---|
| 1 | `internal/settingsui/settingsui.go:4880`（`a.effectiveOpacity` 名字那一行） | `tokens.Caption` → `tokens.Body`；`FontWeight(500)` / `TextTertiary` **不动** |
| 2 | `internal/settingsui/settingsui.go:4881`（同一行的数值） | `tokens.Caption` → `tokens.Body`；`FontWeight(560)` / `TextSecondary` / `ContentTransition{Numeric}` **不动** |
| 3 | 行盒 | 15 → 17（**唯一**被改动的几何）。这一行的内边距（`Space(1)` / `Space(2.25)`，§6.118）不动，所以卡只长 2px，卡内其它行的位置一个像素不动 |
| 4 | 读数的墨 | **不动**。规则沿用 §6.120 的 120-3：答案的名字 = 行名墨低两档（行名 `TextStrong` → `TextTertiary`），答案本身 `TextSecondary` 560（它是数，比名字实）。统一成同一个色号会让答案与行名同墨，层级塌掉 |

## 行为契约零改动红线（本轮逐条自查）

- 高度链：**设置页是自由滚动页**，卡高 +2px 落在滚动内容里，不参与窗口/终端的高度链。
  `a.effectiveOpacity` 的 `Padding` 不动。
- 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**一字节不动**（本轮不新增文案、
  不改 `EffectiveOpacity` / `EffectiveOpacityValue` 两条 i18n 串）。
- `glassmap.Spec.Composite` 的算法、`ContentTransition{Numeric: true}` 的滚动数字：不动。

## 门槛命令 + PASS 基线（本轮实测的当前值，不抄旧报告）

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux/windows/darwin go build ./...
```

基线（HEAD `f19f79b2`，本轮开工实测）：**3377 PASS / 0 FAIL / 182 SKIP / 2549 测试函数**。

## 新 pin 计划

1. `TestTheReadoutLineWearsTheSameStepAsTheValueItAnswers`：真页面两臂，读数的**名字**与**数值**
   两个盒高必须相等，且等于同卡滑块实时读数的盒高（±0.5px）；并把旧的 15 钉成下限
   （≥16.5），防回退。
2. `TestTheReadoutLineKeepsItsOwnInk`：盒高变了，墨**没变**——名字仍比数值安静（明暗两套都量
   展开度），且两者都比行名安静。这条防「顺手把墨也统一了」。
3. opt-in 出图 `TestDumpP13EZReadoutPNGs`（`FLOTER_P13EZ_DUMP` / `FLOTER_P13EZ_DIR` / `FLOTER_P13EZ_TAG`）。

## mutation 计划（≥2 条，写明每条会让哪个 pin 变红）

- **M1**：把数值那一行改回 `tokens.Caption` → pin 1 红（17 vs 15）。
- **M2**：把名字那一行改回 `tokens.Caption` → pin 1 红（名字 15、数值 17 不相等）。
- **M3**（等价写法，预期全绿）：`tokens.Body` 换成 `t.FontSize`（两者在出货尺上是同一个数
  10.8，但 `tokens.Body` 才是这页的语义档）——记成已知覆盖边界。
- **M4**：把数值的墨改成 `tokens.TextStrong`（与行名同墨）→ pin 2 红。

## V / B / S 预判

- **V 0**：`transparencyreadout_test.go` 只钉文本内容，不钉字号。
- **B 0**：无行为断言依赖这行的盒高。
- **S 0**：结构未变（仍是一行两段）。
