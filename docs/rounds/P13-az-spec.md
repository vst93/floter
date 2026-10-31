# P13-az 规格：设置页 10 处提示戴上浮面（§6.20 的 20-3）

## 意图（一句话）

把 `internal/settingsui` 里最后 10 处**框架平涂提示**（`ui/tooltip.go:96-117` 的
`Element.Tooltip`）换成 `theme.Tokens.Tip`，让设置页的提示与抽屉/对话框/便条/终端页/
启动器说同一套 z 语言。

**为什么现在做**：`docs/liquid-glass.md` §6.20 是本支最后一行 `未开始`。
20-1（终端页）已 P13-ax 落地、20-2（启动器）已 P13-ay 落地；20-3 是同一族里
**最大的剩块**，也是唯一还留着「深色档近白片子」的表面。P13-ay 的「仍待 1」逐字点名了它。

## 现状（动手前实测的真实行号）

框架那枚提示（`ui/tooltip.go:96-117`）：

```
tip.MaxWidth(t.Space(80)).Padding(t.Space(1.25), t.Space(2)).Radius(t.Space(1.25)).
    Background(fill).TextColor(text).FontSize(t.FontSize - 1)
tip.Shadow(0, 2, 8, 0, RGBA(0, 0, 0, 0.2))
tip.AttachTo(e, AnchorTop, AnchorBottom).Margin(0, 0, t.Space(1.5), 0)
```

`fill, text := t.inverse()`（`ui/theme.go:71-80`）；本仓**没有**设 `ui.Theme.Inverse`，
所以 `fill` 退回 `t.Text` —— 深色档是一枚近白片子，浅色档是一枚近黑片子。圆角
`Space(1.25)` = 5px（键帽那一档，不是浮面 8），一层 `0 2px 8px 0.2`（既不是环境影
也不是接触影），**没有边**。

10 处调用点（`grep -n "\.Tooltip(" internal/settingsui/*.go`，共 10 条）：

| # | 位置 | 锚点是什么 | 备注 |
|---|---|---|---|
| 1 | `rows.go:889` `sessionIconButton` | 28u 方形图标键（Play / Trash） | 禁用臂走 `lockedControl`，见下方「框架事实」 |
| 2 | `rows.go:957` `sessionKillConfirm` | 28u 危险药丸 | 同上 |
| 3 | `rows.go:1045` `rowTitle` | **非交互**的截断名（`SingleLine().Ellipsis`） | 读全名的唯一出口之一 |
| 4 | `settingsui.go:2569` `integrationRow` | **非交互**的包名文本 | 同上 |
| 5 | `settingsui.go:2773` | 扩展行 Run 键的**禁用原因** | 禁用态，见「框架事实」 |
| 6 | `settingsui.go:2813` | Homepage `rowAction` | 行忙时禁用 |
| 7 | `settingsui.go:3558` `repairAction` | 重连的**进行中**面（`busyActionButton`） | 该面本身 `Disabled(true)` |
| 8 | `settingsui.go:3561` `repairAction` | 重连 / 重查键 | 行忙时禁用 |
| 9 | `settingsui.go:5604` `freshnessDot` | 8×8 的**非交互**状态点 | 无障碍名与提示同一句 |
| 10 | `config.go:476` 配置字段 | 必填标记 `*`（**非交互**文本） | 参考里的 `aria-label` |

### 一条必须记下的框架事实（不改，只记录）

`ui/tooltip.go:133`：

```
return s != nil && s.tip && s.flags&flagDisabled == 0
```

**禁用元素不弹提示。** 所以上表第 5、7 两处（以及第 1、2、6、8 处在禁用/忙碌态）
的提示在指针下**根本不会出现**——迁到 `tokens.Tip` 只是让它在可交互的那一档变对。
这条是框架行为，改它属于「必须改交互才能做好」，按纪律写进 `.pending-bugs.md` 的
「需用户拍板」，本轮不动。

## 逐条落地项

**A. 10 处迁移**：`X.Tooltip(s)` → `tokens.Tip(c, X, s)`（`theme.Tokens.Tip`，
`internal/theme/theme.go:1281`）。机械迁移，锚点、文案、命中框、`Disabled` 顺序
**零改动**；只有链式写法要拆成「先建、再 Tip」（`Tip` 返回锚点，接在建好的元素上）。

注意两点：
1. 第 3 处（`rows.go:1045`）与第 4 处（`settingsui.go:2569`）是**链式表达式**，需要
   把文本先收进局部变量再 `Tip`，不能内联。
2. `tokens` 变量：`rows.go` 三处、`settingsui.go:2569/2773/2813/5604`、`config.go:476`
   都在作用域内；`repairAction`（3558/3561）**没有**，要现取 `a.tokens(c)`。

**B. 像素 pin**（新文件 `internal/settingsui/tipglass_test.go`）：

- `TestTheSettingsTipsAreFloatingPanes`：真帧（sessions 页，460u）上，长名被截断后
  把指针放上去，找到第二枚同名文本（即提示），在该矩形内取均值：
  ① 面 == `theme.FaceOver(悬停前同一像素, tokens.Float)`（**逐通道**，不是「大致变了」）；
  ② 深色档提示顶行**亮于**面（`EdgeHighlight` 冠线），浅色档不假装有；
  ③ 间隙 == `Space(1.5)`（框架那行 margin 本来就有，本 pin 锁住它不回归）。
- `TestTheSettingsKillPillTipIsAPane`：同页的 `sessionKillConfirm`？——不，改成
  `TestTheSettingsIconButtonTipIsAPane`：Play 键的提示（第 1 处），证明**按钮锚点**
  与文本锚点同一条链。
- `TestTheSettingsPageHasNoFrameworkTips`：源码普查。`.Tooltip(` 在
  `internal/settingsui` 必须 **0**；`tokens.Tip(c, ` 必须 **10**，且逐条点名
  10 个 `file:line`（位置表过期即红）。
- 一条 opt-in 出图：`/tmp/floter-P13-az-settings-tip-<dark|light>.png`。

**C. 文档**：`docs/liquid-glass.md` §6.20 的 20-3 转 `已落地`（写实测读数）；
`raycast-design.md` 如有提示条目则加一行指向。

## 行为契约零改动红线

只改墨与面。以下**一个字节都不动**：0.6s 停留 / 0.4s 热接力 / Esc 关掉 / 锚点方向 /
80u 上限 / `Description` 的无障碍句子 / 每一处的 `Label`、`Clicked`、`Disabled`、
`Cursor`。既有 B 类 pin（`TestATruncatedSessionNameStillReadsInFull` 等）必须零改动全绿。

## 门槛与 PASS 基线（实测，改前）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
GOOS=windows go vet ./... / GOOS=darwin go vet ./...   → 净
go test -count=1 ./...                                 → PASS 3112 / FAIL 0 / SKIP 102
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 3112
测试函数数（grep -c "^func Test"）                      → 2294
三平台 go build ./...                                  → 过
```

本轮新增 3 条 pin + 1 条 env 开关出图 → 实得 PASS **3117**（3112 + 5：那 3 条里有一条带
两个子测试），测试函数 **2297**，SKIP **102**（不变，出图走 `FLOTER_P13AZ_DUMP` 环境变量，
不是 `t.Skip`）。

## mutation 计划（每条必须实跑变红）

- **M1**：把 `settingsui.go:3561` 退回 `.Tooltip(tooltip)` → 普查 pin 点名
  `settingsui.go:3561`，`tokens.Tip` 计数 9 ≠ 10。
- **M2**：`Tokens.Tip` 的面退回 `t.Theme.Text`（即框架那枚反色片子）→
  `0 处像素等于 FaceOver(悬停前, Float)`。
- **M3**：删掉 `Tokens.Tip` 的 `InsetShadow(... EdgeHighlight)` 冠线 → 深色档
  顶行不再亮于面。
- **M4**（若预算允许）：删掉 `Tokens.Tip` 的 `.RadiusSM`（退回框架 5px）→ 角上
  多出 3px 的方角像素（用四角读数的差断言）。

## 判类预判

**V 0 / B 0 / S 0**。本轮不动任何数值基线、不动任何测试断言，只把 10 处调用点的
材质换成已经有 pin 的那一枚；既有 pin 全部保持零改动。
