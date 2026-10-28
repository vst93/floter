# P11-aa 报告：高对比度要抬「字」，不只抬「线」（用户实测 F2 的可读性半边）

> 用户实测反馈 F2（最高优先级）的验收标准原文：「保持浅色终端背景下的**可读性与
> 对比度达标**，不要为了降噪把错误提示也弄看不见」。P11-z 只量了描边像素（暖像素
> 2237 → 25），**一个字的对比度都没量过**。本轮补上这半边。
> 规格 `docs/rounds/P11-aa-spec.md`。

## §1 意图与证据

一句话：用户一旦要求更多对比度（系统 `prefers-contrast: more` →
`ui.Preferences.HighContrast`），界面上的**文字**也必须真的变够读——而它以前只抬
**线**。

动手前的证据（HEAD `8804e62`，逐条读过代码确认行号）：

| 位置 | 现状 |
|---|---|
| `internal/theme/theme.go:427-429` | `if prefs.HighContrast { p = p.contrast(appearance) }` |
| `internal/theme/theme.go:832-843` | `func (p palette) contrast`：只写 `p.stroke`、`p.controlEdge`、`p.textMuted`；`p.textSecondary` / `p.textTertiary` **一个字都没动** |
| `internal/theme/theme.go:606-607` | `TextSecondary`/`TextTertiary` 直接取 `p.textSecondary`/`p.textTertiary` → 两档安静墨在高对比度下与普通外观**逐字节相同** |
| `internal/theme/theme_test.go:660` | `TestHighContrastSwapsTheStrokeAndPlaceholderTokens` 的「Nothing else moved」把「只动三条」写成了断言——它正是这条缺口的守卫 |

WCAG 2.1 相对亮度实测（脚本算的，背景 = 面板底 `#F7F7F8` / `#0E0E10`）：

| 档 | 浅色 | 深色 |
|---|---|---|
| `Text` | 16.06:1 | 16.49:1 |
| `TextSecondary` | 4.95:1 | 6.89:1 |
| `TextMuted`（高对比度下） | 5.68:1 | 6.27:1 |
| `TextTertiary` | **3.04:1** ✗ | **3.82:1** ✗ |

也就是说：开了高对比度的用户拿到的是「线更清楚、字还是一样的淡」。而安静两档正是
**说明句、键帽标号、元数据**（`internal/settingsui/rows.go:230`、
`internal/launcher/keycap.go:90`、`internal/launcher/launcher.go:2278`）用的墨——
要读的东西恰恰没变。

## §2 改了什么

1. `internal/theme/theme.go` · `palette.contrast()`：同一分支里再设两档安静墨。

   | 外观 | 档 | 新值 | 面板底 | 选中行填充上 |
   |---|---|---|---|---|
   | 浅 | `textSecondary` | `#4E4E54` | 7.72:1 | 6.75:1 |
   | 浅 | `textTertiary` | `#66666C` | 5.33:1 | 4.66:1 |
   | 深 | `textSecondary` | `#B0B0B8` | 8.95:1 | 7.37:1 |
   | 深 | `textTertiary` | `#8A8A92` | 5.63:1 | 4.64:1 |

   `textMuted` 两支本来就已经够读（5.68 / 6.27），**不动**。**两个表面都量**是因为
   键帽标号与元数据正落在选中行（`RowSelected`）的填充里：浅 `(232,232,233)`、深
   `(34,34,36)`，那里的对比度比面板底低一档。
2. **默认外观一个字节都不改**：`paletteFor` 未动 → 普通用户的界面像素级不变。
   为什么不跟着改默认值：浅色 `#F7F7F8`（L=0.925）上「三档安静墨全部 ≥ 4.5:1」只
   允许它们落在 L ∈ [0.147, 0.167] 这条 **0.02 宽**的窗口里（约 2–3 个灰阶），三档
   会互相看不见；要让三档都过线且可辨，`TextSecondary` 必须压到 ~6.5:1，整族安静
   墨明显变重，与用户要的「轻」相反。**这是设计取舍，不是 bug → 进需拍板表。**
3. `internal/theme/theme_test.go`：新增 `TestEveryInkStepClearsTheReadableContrast`
   （WCAG 数学：四档墨 × 两个表面 × 两种外观 × 两种偏好；正面 4.5:1 + 阶梯次序，
   反面「高对比度不许让任何一档变淡」），并把 `TestHighContrastSwapsTheStrokeAndPlaceholderTokens`
   按 V 类重定基线（旧「只动三条」→ 新「只动五条」）。
4. `internal/settingsui/png_test.go`：新增 opt-in dump `TestDumpP11AAHighContrastPNGs`
   （General 页 × 深/浅 × 开/关 四帧）。
5. `docs/raycast-design.md`：§1 补「可读性不变量」，新增 §6.7。

## §3 V / B / S 台账

- **V = 1**：`TestHighContrastSwapsTheStrokeAndPlaceholderTokens` —— 加了两条期望
  （`secondary`/`tertiary`），`blank()` 里把这两个 token 一起清空（它们现在是**被允许
  移动**的），注释写 `P11-aa 重定基线：旧值 → 新值`。断言本身仍是「只动这五条」。
- **B = 0**：其余测试零改动全绿（含 `TestStrokes`、`TestTheFocusedCardEdgeIsNeutral`、
  `TestPaletteComesFromBaseCSS`、全部视觉 pin —— 因为默认外观没动）。
- **S = 0**：没有新结构，只多了两个字段赋值。

新增 pin（正面 + 反面成对）：

| pin | 文件 | 正面 | 反面 |
|---|---|---|---|
| `TestEveryInkStepClearsTheReadableContrast` | `internal/theme/theme_test.go` | 高对比度下四档墨 × 两个表面全部 ≥ 4.5:1 | (a) 高对比度不许把 `TextSecondary`/`TextTertiary` 留在原值；(b) 阶梯次序在两支里都严格递减；(c) 高对比度不许让任何一档**变淡** |

测试函数：1907 → **1909**（+2：一条 pin + 一条 opt-in dump）。无新增 `t.Skip` 以外的删减。

## §4 门槛（实测，2026-10-28）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./...                 # 净
GOOS=darwin  go vet ./...                 # 净
go test -count=1 ./...                    # 全绿，0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2349
--- SKIP                                  # 18（17 + 本轮新 dump）
测试函数数                                 # 1909
GOOS=linux/windows/darwin go build ./...  # 三平台过
```

**基线是我自己在 HEAD `8804e62` 上重跑的**：PASS 2346 / SKIP 17 / 函数 1907（与
P11-z 报告一致）。本轮 +3 条 PASS（新 pin 的两条子测试 + 父测试各算一行）、
+1 SKIP（新 dump）、+2 函数。

## §5 mutation（实跑，全部变红并按字节还原）

备份 `cp internal/theme/theme.go /tmp/p11aa-theme.go.bak` + `md5sum`；每次还原后
`md5sum -c` 都是「成功」，最后 `touch` 复跑全绿（避免缓存）。

**M1** 删掉 `contrast()` 里新加的四行（回到只动三条）：

```
--- FAIL: TestHighContrastSwapsTheStrokeAndPlaceholderTokens/light
    theme_test.go:708: TextSecondary = {107 107 112 255}, want the contrast ink {78 78 84 255}
    theme_test.go:711: TextTertiary = {142 142 148 255}, want the contrast ink {102 102 108 255}
--- FAIL: TestHighContrastSwapsTheStrokeAndPlaceholderTokens/dark
    theme_test.go:708: TextSecondary = {154 154 160 255}, want the contrast ink {176 176 184 255}
    theme_test.go:711: TextTertiary = {110 110 118 255}, want the contrast ink {138 138 146 255}
--- FAIL: TestEveryInkStepClearsTheReadableContrast/light
    theme_test.go:1071: more contrast left TextSecondary at {107 107 112 255}: the words stay as faint as the lines get sharp
    theme_test.go:1074: more contrast left TextTertiary at {142 142 148 255}: the words stay as faint as the lines get sharp
    theme_test.go:1083: high contrast: TextTertiary on the panel = 3.04:1, want at least 4.5:1
    theme_test.go:1083: high contrast: TextSecondary on a chosen row = 4.33:1, want at least 4.5:1
    theme_test.go:1086: high contrast: TextMuted on the panel = 5.68:1 does not sit below the rung above it (4.95:1)
```

（最后一行顺带暴露了**默认**外观的同一件事：`TextSecondary` 在选中行上 4.33:1 ——
这正是进拍板表的那个缺口。）

**M2** 浅色 `textTertiary` 改成 `#6A6A70`（面板底 5.02:1 过、选中行 **4.39:1** 不过）：

```
--- FAIL: TestHighContrastSwapsTheStrokeAndPlaceholderTokens/light
    theme_test.go:711: TextTertiary = {106 106 112 255}, want the contrast ink {102 102 108 255}
--- FAIL: TestEveryInkStepClearsTheReadableContrast/light
    theme_test.go:1083: high contrast: TextTertiary on a chosen row = 4.39:1, want at least 4.5:1
```

只有「选中行」那半边红 → 证明这条 pin 真的量了两个表面，不是只量背景。

**M3** 深色 `textSecondary` 改成 `#8A8A92`（比 `textTertiary` 还暗，阶梯倒挂）：

```
--- FAIL: TestEveryInkStepClearsTheReadableContrast/dark
    theme_test.go:1086: high contrast: TextMuted on the panel = 6.27:1 does not sit below the rung above it (5.63:1)
    theme_test.go:1086: high contrast: TextMuted on a chosen row = 5.71:1 does not sit below the rung above it (4.58:1)
    theme_test.go:1122: more contrast made TextSecondary quieter: 5.63:1 vs the default's 6.89:1
```

三条都实跑变红，还原后 `md5sum -c` 逐字节核对通过。

## §6 出图与「我实测看到了什么」

`FLOTER_PNG=1 go test -run TestDumpP11AAHighContrastPNGs ./internal/settingsui/`
（General 页，860×1100，四帧）：

| 文件 | 表面 |
|---|---|
| `/tmp/floter-P11-aa-contrast-light-off.png` / `-light-on.png` | 浅色，高对比度关 / 开 |
| `/tmp/floter-P11-aa-contrast-dark-off.png` / `-dark-on.png` | 深色，高对比度关 / 开 |

dump 自己把两档墨打了日志（实测值，不是抄的）：

```
light-off: TextSecondary={107 107 112 255} TextTertiary={142 142 148 255}
light-on : TextSecondary={ 78  78  84 255} TextTertiary={102 102 108 255}
dark-off : TextSecondary={154 154 160 255} TextTertiary={110 110 118 255}
dark-on  : TextSecondary={176 176 184 255} TextTertiary={138 138 146 255}
```

**我实测看到的（像素计数，不是形容词）**：按墨色在帧里数像素（PIL，容差 ±3）：

| 帧 | `TextTertiary` 旧墨 | `TextTertiary` 新墨 |
|---|---|---|
| 浅色 关 → 开 | `(142,142,148)` **1616 → 427** px | `(102,102,108)` **269 → 914** px |
| 深色 关 → 开 | `(110,110,118)` **659 → 210** px | `(138,138,146)` **31 → 453** px |

也就是：打开高对比度后，页面上那片「说明句/元数据」的墨**整族换成了深一档的那个
颜色**——浅色下从 `#8E8E94`（3.04:1）换到 `#66666C`（5.33:1），深色下从 `#6E6E76`
（3.82:1）换到 `#8A8A92`（5.63:1）。**关掉时那四张图的普通外观与上一轮逐像素相同**
（`paletteFor` 没动），所以没有「顺手把默认界面也改了」的副作用。

## §7 仍待 / 需拍板

1. **拍板项（本轮新增，证据在规格 §附注）**：**默认**外观的浅色安静档要不要也推到
   4.5:1？现在 `TextTertiary` 3.04:1、`TextMuted` 3.66:1、`TextSecondary` 4.95:1
   （选中行上 4.33:1）。三档同时过线要在 `#F7F7F8` 上把整族压重（`TextSecondary`
   → ~6.5:1），与「轻」冲突。可选方案：(a) 不动，靠高对比度开关兜底（本轮现状）；
   (b) 三档压重；(c) 砍成两档安静墨（结构改动，`paletteFor` 的注释明写不许合并）。
2. P11-z 的「仍待」第 1 条（搜索框 caret 要不要改中性）原样有效。
3. P11-y 的「仍待」第 2 条起（需拍板清单）原样有效。
