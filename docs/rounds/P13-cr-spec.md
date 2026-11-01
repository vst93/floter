# P13-cr 规格 · 命令板上唯一靠乘法站队的墨，在两套外观上落在两个不同的梯级

## 0. 意图（一句话）

扩展页命令板上的「Summon with: …」这一行，墨是 `TextMuted × 0.85` 算出来的，不是阶梯给的；
这个乘法在**浅色**上恰好撞到阶梯的下一档（141 对 142，1 级），在**深色**上却落在两档当中
（122，介于 `TextMuted` 133 与 `TextTertiary` 110）。同一行字在两套外观上站在不同的梯级上。
改成 `tokens.TextTertiary` 之后，两套外观都落在阶梯自己的那一档：浅 **141 → 142**（1 级，肉眼不可见），
深 **122 → 110**（12 级）。

## 1. 为什么现在做（证据，全部本轮实测）

- 位置：`internal/settingsui/settingsui.go:3315`（`commandIdentity` 的第三行字）：

  ```go
  ui.Text(c, copy.IntegrationsCommandsTriggers(strings.Join(words, ", "))).
      FontSize(tokens.Caption).TextColor(t.TextMuted).Opacity(0.85).
  ```

  同一个函数里，第二行（description，`:3313`）是 `t.TextMuted` 一档到底；同一块板下面的
  别名编辑器（`:3348` `commandAliasEditor` 的 label）与「taken」注（`:3362`）也都是
  `t.TextMuted`。整块板上**只有这一行**多乘了一把 0.85。
- 来源：`commandlist_test.go:19` 抄的是参考 CSS `.extension-command-list__triggers { opacity: 0.85 }`，
  叠在 `.extension-command-list span { color: var(--text-muted) }` 之上——两个旋钮一道算。
  参考是网页，`opacity` 直接和它自己的底色复合；本仓的墨是**面板上的洗色**，同一个乘法落到
  两套外观上就是两个位置（见 §1 表）。
- 实测（`renderIn` + `actionRungProbe`，扩展页命令列表，`720×900`，`commandListIntegration`）：

  | 外观 | 板 | description | Summon 行（改前） | `TextMuted` over 板 | `TextMuted×0.85` over 板 | `TextTertiary` over 板 |
  |---|---|---|---|---|---|---|
  | 浅 | `{213,213,214}` | `{128,128,135}` | `{141,141,147}` | `{128,128,135}` | `{141,141,147}` | `{142,142,148}` |
  | 深 | `{61,61,63}` | `{133,133,141}` | `{122,122,129}` | `{133,133,141}` | `{122,122,129}` | `{110,110,118}` |

  读出来的三件事：① 浅色的 `×0.85` 与 `TextTertiary` 只差 **1 级**——那一档在这个外观上
  本来就是阶梯的第三档，乘法只是绕了一圈；② 深色的同一个乘法落在 **122**，比 `TextMuted`
  低 11、比 `TextTertiary` 高 12——**两档之间的无人区**；③ 墨阶梯是 token 表（`theme.Tokens`
  `TextStrong > TextSecondary > TextMuted > TextTertiary`），乘法不在表上，于是同一句话
  在两套外观上是两个位置——正是 §1「同一色相、token 唯一来源」要消掉的那类偏差。

## 2. 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `settingsui.go:3315` | `TextColor(t.TextMuted).Opacity(0.85)` | `TextColor(tokens.TextTertiary)`（删掉 `.Opacity(0.85)`） |
| 2 | `commandlist_test.go:19` 的 CSS 台账注释 | 只写 `__triggers at opacity 0.85` | 补一句「P13-cr 重定基线：0.85 的乘法 → 阶梯的 `TextTertiary`，理由见 `docs/liquid-glass.md` §6.61」 |
| 3 | 新 pin `triggerrung_test.go` | — | 两套外观各一条：Summon 行的墨 == `panelOver(TextTertiary, 板)`（±2）；且它离 `description` 的墨 ≥ 8 级（仍是**一档**，不是同一档）；且三行墨单调（name > desc > triggers 的离板距离） |
| 4 | 新 dump `p13cr_dump_test.go` | — | opt-in：出 `/tmp/floter-P13-cr-commands-<light\|dark>-<before\|after>.png`，并在 `after` 存在 `before` 时打印差分 px 数与 bbox |

**不动的**：命令板的 `Opacity(0.62)`（开关关掉时的整板淡，参考 `.extension-command-list__row--off`，
不是墨）；名字的 `TextSecondary`；description / 别名 label / taken 注的 `TextMuted`；字号
（全行 `tokens.Caption`，与 `Caption 11px` 那条**需拍板**项无关，本轮一个字号不改）；间距；盒子度量。

## 3. 行为契约零改动（红线）

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一行不碰**。本轮只改**墨**
（一个颜色 token），外加一条测试与一条 dump。`commandIdentity` 输出的文字、可点区域、
`switchControl` 的 `Label()`、`SetCommandEnabled` 的接线全部原样。

## 4. 门槛命令与 PASS 基线（本轮开工实测，不用旧报告数字）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./...                              → 净
GOOS=darwin  go vet ./...                              → 净
go test -count=1 ./...                                 → 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"       → 基线 3283
测试函数数                                              → 基线 2430
GOOS=linux/windows/darwin go build ./...                → 全过
```

## 5. mutation 计划（≥2 条，逐条说明哪条 pin 会红）

| # | 注入 | 预期红的 |
|---|---|---|
| M1 | 把 `.Opacity(0.85)` 放回（`TextMuted` 再乘 0.85） | 新 pin ①：**深色** `{122,122,129}` 对 `{110,110,118}`（12 级 > 2）；浅色仍绿——这正是本轮量到的「乘法只在深色上错位」 |
| M2 | 改成 `TextMuted`（整个乘法删掉，不换档） | 新 pin ①：浅 `{128,128,135}` 对 `{142,142,148}`、深 `{133,133,141}` 对 `{110,110,118}`；新 pin ②（仍是两档：差 ≥ 8）也会红——这行会与 description 逐像素相同 |
| M3 | 改成 `TextSecondary`（跳两档） | 新 pin ②：它比 description 还重（`{107,107,112}` < 128），阶梯倒挂 |

## 6. 台账预判

**V 0 / B 0 / S 0**（新 pin 是新增，不改任何既有断言；既有 `commandlist_test.go` 量的是
板面像素与开关，不量这一行的墨）。
