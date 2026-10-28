# P11-aa 规格：高对比度开关要真的抬「字」，不只是抬「线」

## 意图（一句话）

用户一旦要求更多对比度（系统 `prefers-contrast: more` → `ui.Preferences.HighContrast`），
界面上的**文字**也必须真的变得够读：`palette.contrast()` 现在只抬了三条线
（`stroke`/`controlEdge`）和一个 placeholder（`textMuted`），把真正承载内容的二级/三级墨
（说明句、键帽标号、元数据）留在原处——浅色 **3.04:1**、深色 **3.82:1**，低于 AA 的 4.5:1。
开了高对比度的用户拿到的是一张「线更清楚、字还是一样的淡」的界面。

## 为什么现在做

1. 用户实测反馈（`.pending-bugs.md` 顶部 F2）的验收标准里写着「保持浅色终端背景下的
   **可读性与对比度达标**」；P11-z 只量了描边像素（暖像素 2237 → 25），**没有量任何文字**。
   这一条是 F2 遗留的、可量化的半边。
2. 默认外观的安静档文字确实也不达标，但把三档安静墨同时推到 4.5:1 在 `#F7F7F8` 上
   **数学上不可能保持三档可辨**（见 §附注），所以改默认值属**拍板项**（已记入
   `.pending-bugs.md`「需用户拍板」）。高对比度是**用户自己按下的开关**，它存在的理由就是
   「我不要好看，我要读得清」——这一支不需要拍板。
3. 现状证据（本轮实测，`internal/theme/theme.go`）：

| 位置 | 现状 |
|---|---|
| `internal/theme/theme.go:427-429` | `if prefs.HighContrast { p = p.contrast(appearance) }` |
| `internal/theme/theme.go` `func (p palette) contrast` | 只写 `p.stroke`、`p.controlEdge`、`p.textMuted`；`p.textSecondary`/`p.textTertiary` **一个字都没动** |
| `internal/theme/theme.go:606-607` | `TextSecondary`/`TextTertiary` 直接取 `p.textSecondary`/`p.textTertiary`，所以两档安静墨在高对比度下与普通外观**逐字节相同** |
| `internal/theme/theme_test.go:660` | `TestHighContrastSwapsTheStrokeAndPlaceholderTokens` 的「Nothing else moved」把「只动三条」写成了断言——它正是这条缺口的守卫，本轮要按 V 类重定基线 |

WCAG 相对亮度实测（本轮用脚本算的，背景 = 面板底）：

| 档 | 浅色 | 深色 |
|---|---|---|
| `Text` | `#1B1B1D` 16.06:1 | `#EDEDEF` 16.49:1 |
| `TextSecondary` | `#6B6B70` **4.95:1** | `#9A9AA0` **6.89:1** |
| `TextMuted`（高对比度下） | `rgba(28,28,32,.68)` → **5.68:1** | `rgba(242,243,245,.58)` → **6.27:1** |
| `TextTertiary` | `#8E8E94` **3.04:1** ✗ | `#6E6E76` **3.82:1** ✗ |

## 逐条落地项

### 1. `internal/theme/theme.go` · `func (p palette) contrast`

现状：只设 `stroke`、`controlEdge`、`textMuted`。
应改成：同一分支里再设两档安静墨，使**四档墨**在两种外观下都 ≥ 4.5:1，且阶梯次序
（`Text` > `TextSecondary` > `TextMuted` > `TextTertiary`）在两个表面上都保持：

| 外观 | 档 | 新值 | 面板底 | 选中行填充上 |
|---|---|---|---|---|
| 浅 | `textSecondary` | `ui.Hex("#4E4E54")` | 7.72:1 | 6.75:1 |
| 浅 | `textMuted` | 不动（`rgba(28,28,32,.68)`） | 5.68:1 | 5.36:1 |
| 浅 | `textTertiary` | `ui.Hex("#66666C")` | 5.33:1 | 4.66:1 |
| 深 | `textSecondary` | `ui.Hex("#B0B0B8")` | 8.95:1 | 7.37:1 |
| 深 | `textMuted` | 不动（`rgba(242,243,245,.58)`） | 6.27:1 | 5.72:1 |
| 深 | `textTertiary` | `ui.Hex("#8A8A92")` | 5.63:1 | 4.64:1 |

「选中行填充上」= 浅 `RowSelected` 0.06 黑合成后的 `(232,232,233)`、深 0.085 白合成后的
`(34,34,36)`。**两档安静墨在填充上也要过线**，因为键帽标号与元数据正落在选中行里。

**默认外观（非高对比度）一个字节都不改**：`paletteFor` 不动。

### 2. `internal/theme/theme_test.go` · 新增 `TestEveryInkStepClearsTheReadableContrast`

- 正面：两种外观 × 两档偏好（普通 / 高对比度）里，`Text`/`TextSecondary`/`TextMuted`/
  `TextTertiary` 对**面板底**与**选中行填充**的 WCAG 对比度都 ≥ 4.5。
  （普通外观下安静两档不达标 → 所以这条只对高对比度那一支断言 4.5；普通外观那一支断言
  的是**阶梯次序**：四档严格递减，任何外观都不许两档相等。）
- 反面：高对比度下 `TextTertiary` 与 `TextSecondary` 必须**不等于**普通外观的同名 token
  （旧缺口：两者逐字节相同 → 红）。

### 3. `internal/theme/theme_test.go` · `TestHighContrastSwapsTheStrokeAndPlaceholderTokens`（V 类）

- 现状：`placeholder` 一条 + `blank()` 里把「其它都不许动」写成断言。
- 应改成：加两条期望（`secondary`/`tertiary`），`blank()` 里把这两个 token 一起清空
  （它们现在是**被允许移动**的），注释写 `P11-aa 重定基线：旧值 → 新值`。
  断言本身仍是「只动这五条」。

### 4. `docs/raycast-design.md`

- §1 表下补一条不变量：**「高对比度下四档墨全部 ≥ 4.5:1（面板底与选中行填充两个表面）」**。
- §6.7 记本轮：改了什么、为什么默认外观不动、剩下的拍板项（默认浅色安静档 3.04:1）。

### 5. 视觉证据

`internal/settingsui/png_test.go` 新增 opt-in dump（`FLOTER_PNG=1`）：
`TestDumpP11AAHighContrastPNGs` —— 设置页 General 页在 `HighContrast` 开/关 × 深/浅四帧，
写 `/tmp/floter-P11-aa-contrast-<appearance>-<on|off>.png`。报告里写实测像素（说明句的墨色）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一个字都不动**。本轮只改
`palette.contrast()` 的两个字段与两条测试，另加一条 dump。默认外观（`paletteFor`）不动，
所以普通用户的界面**像素级不变**。

## 门槛命令与 PASS 基线（本轮实测，HEAD `8804e62`）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2346（实测基线）
--- SKIP                                  # 17（实测）
测试函数数                                 # 1907（实测）
GOOS=linux/windows/darwin go build ./...  # 三平台过
```

本轮预期：PASS 2346 → **2348**（+1 新 pin +1 新 dump 的 SKIP 不计），SKIP 17 → **18**，
函数 1907 → **1909**。

## mutation 计划（≥2 条，每条说明让哪个 pin 变红）

1. **M1**：把 `contrast()` 里新加的两行删掉（回到只动三条）→
   `TestEveryInkStepClearsTheReadableContrast` 红（浅 `#8E8E94` 3.04:1）+
   `TestHighContrastSwapsTheStrokeAndPlaceholderTokens` 红（`secondary`/`tertiary` 期望不符）。
2. **M2**：把浅色 `textTertiary` 的新值改成 `#6A6A70`（5.02:1 面板底、**4.39:1** 选中行）
   → 只有「选中行填充」那半边红：证明这条 pin 真的量了两个表面，不是只量背景。
3. **M3**：把深色 `textSecondary` 的新值改成 `#8A8A92`（比 `textTertiary` 还暗）→
   阶梯次序断言红：证明次序不是摆设。

还原一律 `cp` 备份回写 + `md5sum -c` 逐字节核对（P11-z 的教训：**不许用 `git checkout`**）。

## 附注：为什么默认外观不跟着改（拍板项的证据）

浅色面板底 `#F7F7F8` 的相对亮度 L = 0.925。4.5:1 要求文字 L ≤
`(0.925+0.05)/4.5 − 0.05` = **0.1667**；而当前 `TextSecondary`（4.95:1）的 L = 0.147。
也就是说「三档安静墨全部 ≥ 4.5:1」只允许它们落在 L ∈ [0.147, 0.167] 这条 **0.02 宽的
窗口**里——换算成 sRGB 大约 2–3 个灰阶，三档会互相看不见。要让三档都过线且可辨，
`TextSecondary` 必须压到 ~6.5:1（L ≈ 0.10），整族安静墨明显变重，与用户要的「轻」
相反。这是设计取舍，不是 bug，所以进「需用户拍板」。
