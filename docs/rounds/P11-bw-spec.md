# P11-bw · 界面四个档位上，行盒、字盒与它报出的窗口是同一个比例

## 意图

启动器的行几何在**任何一个界面档位**上都等于「行自己的 40u」或「两行字的盒」里更高的那个，
它报给外壳的窗口高度在**任何一个档位**上都装得下它画的十行 —— 这两条至今**没有一行测试**看着：
`internal/launcher` 的每一条 pin 都画在框架的库存主题上（`Spacing = 4`、`FontSize = 12`），
而界面档位只改 `theme.ForPreferences` 产出的主题，测试从没把它装上过。

为什么现在做（证据）：P11-bv 报告 §6 ① 实测过一次「四个界面档位 × 两外观 × 两语言」的字压字审计，
把字号改成随档位**二次**缩放后仍然全绿 —— 因为那一组审计根本没把主题交给画布。
`grep -rn "UIScale\|SetTheme" internal/launcher/*_test.go`（P11-bv 后）只有
`framememo_test.go:26`（只查 memo 键）与 `plate_lens_test.go:25`（深色主题），
`height_test.go` 的 `Geometry` 夹具除 `launcher_test.go:157` 的收起态公式外全写死 `Spacing: 4`。
所以「档位改了、行没跟着走」这一类回归，本包一条 pin 都看不见。

## 实测（本轮跑出来的当前值，Linux，`InputWindowWidth = 720`，十个最近启动）

| 档位 | spacing | font | 行距（实测） | 40u | 字盒和 title+sub | 窗口高（报出） | 十行全在窗内 |
|------|---------|------|--------------|-----|------------------|----------------|--------------|
| tiny | 3.2 | 9.6 | 33 | 32 | 18+15 = 33 | 389 | 是 |
| small | 3.6 | 10.8 | 36 | 36 | 19+17 = 36 | 427 | 是 |
| default | 4.0 | 12.0 | 40 | 40 | 22+18 = 40 | 474 | 是 |
| large | 4.4 | 13.2 | 44 | 44 | 23+20 = 43 | 522 | 是 |

三件事读出来：① 行距 **= max(rowUnits × spacing, TitleLine + SubtitleLine)**，四档逐档成立
（tiny 是字盒更高那一路，large 是 40u 更高那一路，两个分支都真的被走到过）；
② 行距 / 字号在四档上是 3.4375 / 3.333 / 3.333 / 3.333（tiny 差 3% 是行盒按整像素栅格化，
18/9.6 ≠ 22/12，不是分歧）；
③ 报出的窗口高在四档上都装得下它画的十行。

**结论：产品代码没有缺陷，缺的是尺子。** 本轮只加 pin。

## 落地项

### 1. `internal/launcher/uiscale_test.go` · 行盒在四档上都等于自己的 40u 或两行字

- 位置：新文件 `internal/launcher/uiscale_test.go`（测量助手 `measureStep`）。
- 现状：没有这条测量；本包所有 pin 画在 `Spacing=4` 的库存主题上。
- 应改成：每档用**该档自己的主题**（`c.SetTheme(a.tokens(c).Theme)`，与设置页
  `extensionfieldmetrics_test.go:33` 同一手法）画一帧，量 `Recent 00`/`Recent 01` 的 y 差。
- 断言：
  1. `pitch == max(rowUnits*spacing, a.TitleLine+a.SubtitleLine)`，容差 1px；
  2. 四档的 `pitch/font` 极差 ≤ 5%（字与盒同比例）；
  3. 行距随档位严格递增（一条「不跟档位走」的回归在这里红）。
- pin：`TestTheRowBoxIsTheSameUnitsAtEveryInterfaceStep`。

### 2. `internal/launcher/uiscale_test.go` · 报出的窗口装得下它画的十行

- 位置：同上，第二个 pin `TestTheChargedWindowHoldsTheTenRowsAtEveryInterfaceStep`。
- 现状：`App.View` 把 `Geometry.Height()` 交给 `Actions.ResizeTo`
  （`launcher.go:1235-1244`），但没有任何 pin 把「报出的高度」与「画出来的行」放在同一帧里比。
- 应改成：第一帧量出报出的高度，**换一个新的 `App`**（同一个 App 交给第二个 `ui.Tester` 会在
  `FocusSearch` panic，见 P11-bu 仍待第 2 条）按那个高度再画一帧，十行必须全在窗内。
- pin 同上。

## 行为契约零改动红线

- 产品代码**零改动**：本轮只新增测试文件。
- 高度链（`rowUnits`/`fieldRowUnits`/`Geometry.Height`）、键盘模型、会话管理、设置写回、
  i18n 文案、命令接线一个字节不动。

## 门槛

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
```

PASS 基线（实测，开工前）：**2560**（P11-bv），测试函数 **2026**。

## mutation 计划

| # | 注入 | 预期变红的 pin |
|---|------|----------------|
| M1 | `launcher.go:2242` `floor := t.Space(rowUnits)` → `floor := t.Space(rowUnits) * tokens.Scale`（二次缩放） | `TestTheRowBoxIsTheSameUnitsAtEveryInterfaceStep`（large 档 44 → 48.4，比值极差 9%） |
| M2 | 同上 → `floor := float32(40)`（钉死不跟档位走） | 同上（tiny 33 → 40、large 44 → 40） |
| M3 | `height.go` `Height()` 的 `chrome` 里去掉 `breathUnits`（少算 4u） | **仍绿（实测）**：框架把被裁的 run 夹进裁剪框，十行都还在帧里，只是第十行文字落到窗口外 16px；这条 pin 只看「整行还在不在」。要守 4u 这一档得换一把能读裁剪前几何的尺子（见报告 §6.1） |

M3 的结论写进了报告与 `.pending-bugs.md`，不是「变异没跑」。
