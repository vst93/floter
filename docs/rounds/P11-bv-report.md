# P11-bv 报告 · 中文界面里不会再冒出一句英文（同字集合只许缩小）

规格 `docs/rounds/P11-bv-spec.md`。选题：用户实测反馈 F1–F4 全部收完 → 合约 §6 全 `已落地`
（只剩拍板项）→ P11-bu 仍待第 1 条整张是老拍板项、第 2 条是尺子的边界、第 3 条卡在
`Element.Cache` / 上游 `actionSpinner` → 按用户定的下一档自己找，落在「用户读的是 zh」
（P11-ba）这一条上：**zh 里留一句英文，是界面质感里最刺眼的一种，而没有一条测试管它。**

## 1. 改了什么

`internal/i18n/sharedwords_test.go`（新，1 条）：
`TestOnlyIdentifiersAndNamesAreSharedByBothLanguages`。反射遍历 `en` / `zh` 两份 `Copy`，
收集**逐字节相同**的字符串叶子，然后按**路径形状**（不是字符串内容）归类：

| 类 | 判据 | 实测条数 |
| --- | --- | --- |
| 存进设置文件的 ID | 叶子名 `ID` | 47 |
| 语言自己的名字 | `Copy.Settings.Languages[N].Label` | 2（`English` / `中文`）|
| 品牌 | `Copy.Notifications.Title` | 1（`floter`）|
| 已记账的用词债务 | `Copy.Settings.AppIcons[N].Label` | 2（`Dark` / `Light`）|

归不进这四类的同字叶子 → 报错「这是一段显示文案，没有翻译」。另有遍历自证：
同字叶子少于 47 条即判遍历漏了分支（ID 一族实测正好 47）。四条 `AppIcons`/自名/品牌
只 `t.Logf`，是**故意**同字，不是漏译。

**产品代码零改动**：`internal/i18n/copy.go` 一个字节没动（M1 变异后 `md5sum -c` 核对）。
`Dark` / `Light` 在中文界面下要不要写成「深色 / 浅色」是**用词拍板项**
（`.pending-bugs.md` 的 P9-g 表那一路），本轮只把它钉成"已知债务"。

## 2. 实测：52 条同字叶子的完整清单

探针（跑完即删）逐叶打印，`en == zh` 的恰好 52 条，全部落在上表四类：

```
Copy.Settings.Themes[0..2].ID              auto / light / dark
Copy.Settings.Languages[0..1].ID           en / zh
Copy.Settings.Languages[0..1].Label        English / 中文
Copy.Settings.GlassSteps[0..3].ID          off / frosted / regular / liquid
Copy.Settings.Scales[0..3].ID              tiny / small / default / large
Copy.Settings.CursorShapes[0..2].ID        beam / block / underline
Copy.Settings.Paddings[0..2].ID            compact / regular / relaxed
Copy.Settings.Palettes[0..8].ID            inherit / contrast / paper / ink / fog / forest / dusk / mist / amber
Copy.Settings.AppIcons[0..1].ID            dark / light
Copy.Settings.AppIcons[0..1].Label         Dark / Light        ← 唯一的真债务
Copy.Settings.ShortcutsActions[0..7].ID    plugin:* / action:* / command
Copy.Settings.BrowserSortOrders[0..3].ID   relevance / recent / alphabetical / visits
Copy.Settings.BrowserSearchFields[0..2].ID all / title / url
Copy.Settings.CalculatorCopyModes[0..1].ID full / result
Copy.Notifications.Title                   floter
```

47 + 2 + 1 + 2 = 52。除了 `AppIcons` 那两条，**没有第三处**英文显示文案留在 zh 里。

## 3. 台账

- **V = 0**：没有视觉数值（墨与面一个字节没动）。
- **B = 0**：既有测试零改动全绿；产品行为零改动（只新增测试文件）。
- **S = 0**：没有因结构变化改断言。

## 4. mutation（2 条实跑变红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 复跑绿）

**M1 · 尺子对「漏译」的敏感度** —— `internal/i18n/copy.go:1000` 的
`Clear: "清除"` → `Clear: "Clear"`（中文界面里冒出一句英文）：

```
--- FAIL: TestOnlyIdentifiersAndNamesAreSharedByBothLanguages (0.00s)
    sharedwords_test.go:42: Copy.Launcher.Clear 在 zh 里与 en 同字：这是一段显示文案，没有翻译
```

同一注入下 `TestEveryLanguageIsComplete` **仍然绿** —— 这正是本轮的增量，旧的
「不空」断言查不到这一类。还原后 `md5sum -c` 逐字节一致，`touch` 复跑绿。

**M2 · 遍历是不是真的在走** —— 删掉 `sharedLeaves` 的 `reflect.Slice` 分支（只走标量，
切片的 50 条全丢）：

```
--- FAIL: TestOnlyIdentifiersAndNamesAreSharedByBothLanguages (0.00s)
    sharedwords_test.go:35: 同字叶子只有 1 条，遍历漏了分支：[Copy.Notifications.Title]
```

还原后 `md5sum -c` 逐字节一致。

## 5. 门槛（本轮实测）

```
gofmt -l cmd internal                    空
go vet ./...                             净
GOOS=windows go vet ./...                净
GOOS=darwin  go vet ./...                净
go test -count=1 ./...                   全绿（无 FAIL）
--- PASS 计数                            2559 → 2560
--- SKIP                                 54（不变）
测试函数                                 2025 → 2026
GOOS=linux/windows/darwin go build ./... 三平台过
```

## 6. 本轮**试了但没出货**的三件事（写成事实，省下一轮）

### 6.1 四档界面大小的字压字审计：跑绿，但**变异证明它不敏感** → 没进仓库

界面大小（`valuedomain.ScaleFactors` 的 0.8/0.9/1.0/1.1）从来没进过 P11-aj 起的审计。
我按 `auditLauncherFrame` / `auditSettingsFrame` 的形状各写了一版（4 档 × en/zh × 深/浅 ×
两尺寸，launcher 3 个查询状态、settings 全 5 页），**第一次跑就全绿**：

```
go test -count=1 -run TestEveryInterfaceSizePaintsNoTextOverText ./internal/launcher/ ./internal/settingsui/
ok  	floter/internal/launcher	1.172s
ok  	floter/internal/settingsui	1.257s
```

但变异把它证伪了：`internal/theme/theme.go:418` 的
`t.FontSize = round1(BodyUnits * f)` → `round1(BodyUnits * f * f)`（字号随界面大小**二次**缩放，
default 不变、large 1.21 倍），两条测试**仍然绿**——因为行的行高也是从同一份字号推出来的，
字与行**同比**放大，框永远不会相交。**这条尺子看不见"比例被改"，只看"字互压"**，
所以它是没有敏感度的一条 pin，没进仓库（两个文件已删，`git status` 干净）。
→ 记进 `.pending-bugs.md` 仍待，作为尺子的第三处盲点。

### 6.2 shell（主窗口）第一次有了帧基准：数字与瓶颈

`internal/shell` 是四个表面里唯一没有帧基准的（launcher / settingsui / terminalui 都有）。
探针实测（720×420 / 720×580，`ui.Tester` 一帧）：

| 帧 | 时间 | 分配 |
| --- | --- | --- |
| 启动器（shell 组合后） | 33.7 µs | 1.5 KB / 29 allocs |
| 设置页（Integrations 起手页的组合） | 271.5 µs | 20 KB / 317 allocs |

CPU profile 里 **70–80% 是框架的栅格化**（`raster.blend` / `raster.fill` / `to8` / cgo），
本仓自己的绘制只占个位数百分比；最大的单点是
`theme.ForPlatform` ← `launcher.Resolve`（`internal/shell/detach.go:199`、`pinned.go:71`
每帧各一次，4.5%），而它已经在 `theme.Memo` 的射程内。**没有值得动的**——这条线到此为止，
别再用「加一个 memo」的姿势来领 shell 的帧。

### 6.3 搜索字段的长查询溢出：探针干净

160 个 `a`、120 个汉字（`搜索`×60）、一条 88 字符的绝对路径，在 720 与 640 两种宽度下
typed（`tt.Type`，不是直接写 `a.Query`）后跑审计，全部干净——字段自己会滚动/裁剪，
不和尾随键帽相交。

## 7. 仍待

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行）。
2. **新增拍板项**：`Copy.Settings.AppIcons[].Label` 在 zh 下是 `Dark` / `Light`，
   要不要改「深色 / 浅色」（本轮的新 pin 已把它钉成唯一一处已知债务，改完钉会自动松）。
3. **尺子的第三处盲点**（本轮的实测产物）：`auditLauncherFrame` / `auditSettingsFrame`
   对「字号与行高同比被改」完全不敏感（§6.1 的变异仍绿）。要守界面大小这一档，得换一把
   能把**比例**读出来的尺子（例如读 `Tokens.FontSize` 与行高的比），而不是继续加字压字臂。
4. `Element.Cache` / `#753` 脏矩形仍停在「`ui.Tester` 看不见 renderer 侧」；
   设置页 `actionSpinner` 仍是上游构建期缺口。
5. 审计的两处老盲点（`Tester.Find` 按字符串去重；滚动容器里被裁掉的行不进 `Texts()`）不变。
