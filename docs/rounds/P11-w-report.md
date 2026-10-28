# P11-w 报告：一个值域的 id 清单只有一个答案

规格 `docs/rounds/P11-w-spec.md`。方向按用户定的「三平台一致性 → 基础功能 → 像素细节」，
落在**基础功能**：一个设置能存哪些 id、拾取器能显示哪些行、渲染层能画哪些 id，本轮之前是
五个包各自回答。

## 1. 改动

### 1.1 新增 `internal/valuedomain`（叶子包）

八张 id 表（`AppIcons` `Themes` `GlassSteps` `Scales` `CursorShapes` `Paddings`
`Palettes` `BoldModes`）+ `ScaleFactors`（界面尺寸 id → 倍率，从
`internal/settings/settings.go` 搬来）+ `Has` + `NormalizeAppIcon` + `DefaultAppIcon`。
序取**界面序**（唯一可观测的序；成员检查不看序）。不 import 任何 internal 包，无环。

### 1.2 各面转发

| 面 | 之前 | 现在 |
| --- | --- | --- |
| `internal/i18n/copy.go` | 14 张 `[]Option` 字面量（en/zh 各七张） | `optionsFor(valuedomain.X, <lang>XLabels)` 一行一张 |
| `internal/i18n/options.go` | — | 新增：`optionsFor` + 14 张**标签** map（唯一按语言写的部分） |
| `internal/settings/settings.go` | 4 个 `[]string` + `boldModes` + `uiScaleFactors` | 删掉，改读 `valuedomain`（`NormalizeTheme` 变一行 `Pick`） |
| `internal/shell/system.go` | `appIconBytes`/`storedAppIcon` 各一份 `== "light"` | `trayIcons` 按 id 存 PNG + `valuedomain.NormalizeAppIcon` |
| `internal/settingsui/settingsui.go` | `appIcon()` 第三份 `== "light"` | 转发 `valuedomain.NormalizeAppIcon` |

`legacyGlassSteps`（旧五档 → 四档）与 `legacyUIScales`（`larger` → `large`）留在
`internal/settings`：那是**迁移表**，不是值域，本轮不动。

一处**顺带定序**：`settings.themes` 原本是 `dark, light, auto`，而界面表是
`auto, light, dark`。值域是集合，序只对拾取器可见，所以单一表取界面序；
`NormalizeTheme` 由手写循环改成一个 `Pick`，成员判定不变。

侦察时的一个反例（写进 pin 了）：**拾取器的行序此前没有任何 pin**。
`internal/settingsui/view_test.go:455` 看起来像在钉首尾标签，实际是
`copy.Themes[0].Label` / `copy.Themes[len-1].Label` —— 从被检的表自己读期望值，
重排顺序它照样绿（实测：把 `Themes` 改成 `dark, light, auto`，整个 `settingsui`
包 0 条红）。所以本轮补了 `TestPickerOrderIsTheShippedOrder`，把七张表的
英文行序逐字钉住（这也是 M3 变异能红的前提）。

### 1.3 i18n 文案零改动

七张表的 14 份标签逐字搬进 `options.go`，含两处容易被「顺手改掉」的：中文副本里应用图标
的两行**故意是英文** `Dark`/`Light`（它命名的是图标美术，不是界面外观），本轮保持原样。

## 2. 新增 pin

| 位置 | 名字 | 守什么 |
| --- | --- | --- |
| `internal/i18n/options_test.go` | `TestPickerOptionsMatchTheStoredDomains` | 两份 `Copy` × 七张表：行数 = id 数、第 i 行 id = 第 i 个 id、标签非空且不等于 id |
| `internal/i18n/options_test.go` | `TestPickerOrderIsTheShippedOrder` | 七张表的行序逐字钉住（此前无人守，见 §1.2） |
| `internal/i18n/options_test.go` | `TestLanguageLabelsAreEndonyms` | 语言表不进值域（endonym 两份副本必须相同，P11-v 的形状） |
| `internal/valuedomain/valuedomain_test.go` | `TestEveryScaleStepHasAFactor` | `Scales` ↔ `ScaleFactors` 双向相等，倍率 > 0 |
| `internal/valuedomain/valuedomain_test.go` | `TestHasIsAnExactMatch` / `TestNormalizeAppIcon` / `TestDomainsHaveNoDuplicateIDs` | 精确匹配、图标归一化、无重复 id |
| `internal/glassmap/domain_test.go` | `TestGlassStopsCoverTheStoredDomain` / `TestOffIsNotAnEffectStop` | `keys(stops) ∪ {off} == GlassSteps` |
| `internal/terminalui/palette_domain_test.go` | `TestPalettesCoverTheStoredDomain` / `TestInheritIsNotAColourSpec` | `keys(palettes) ∪ {inherit} == Palettes` |
| `internal/shell/appicon_test.go` | `TestEveryAppIconHasArtwork` / `TestTheTwoAppIconFacesUseOneRule` | 每个图标 id 有 PNG；托盘与设置页对同一个存值给同一个答案 |
| `internal/ci/valuedomain_contract_test.go` | `TestValueDomainsHaveOneAnswer` | 源码扫描：`internal/valuedomain` 之外不得有第二份 id 清单，也不得再有 `== "light"` |
| `internal/ci/valuedomain_contract_test.go` | `TestTheLoaderAndThePickableDomainsAgree` | 行为：拾取器给的 id 过加载器后原样返回，不存在的 id 会被拒 |

## 3. V / B / S 台账

- **V = 0**：非视觉轮，没有像素/颜色断言被重定基线。
- **B = 0**：既有断言零改动全绿，包括 `internal/shell/shell_test.go:870-887`
  （`appIconBytes` 的默认回落）与 `internal/settings` 的迁移/归一化断言。
  `internal/settingsui/view_test.go:455` 也零改动全绿——但它并不钉行序（见 §1.2），
  所以行序由本轮新增的 `TestPickerOrderIsTheShippedOrder` 守。
- **S = 3**（结构真变了）：①五处 id 清单 → 一张表 + 各面转发；②应用图标归一化两处 → 一处；
  ③`uiScaleFactors` 从 settings 私表 → `valuedomain.ScaleFactors`（定义跟着 id 走）。

## 4. 门槛（全部实测）

```
gofmt -l cmd internal                        空
go vet ./...                                 净
GOOS=windows go vet ./...                    净
GOOS=darwin  go vet ./...                    净
go test -count=1 ./...                       全绿（42 ok + 2 no test files = 44 行）
--- PASS                                     2339   （起点实测 2324，+15）
--- SKIP                                     15
--- FAIL                                     0
测试函数                                      1898   （起点实测 1883，+15）
三平台 build                                 过
```

净行数：产品代码 `-142 / +60`（4 个改动文件）+ 新增 201 行（`valuedomain` 92、
`i18n/options.go` 109）；测试新增 508 行。产品代码净 **+119** —— 涨在标签 map 上：
七张表的每个 id 现在各占一行，换来的是「id 只有一份」。

## 5. 变异（4 条，全部实跑变红并逐字节还原）

| # | 注入 | 变红 |
| --- | --- | --- |
| M1 | `valuedomain.Palettes` 末尾加 `"sand"`（渲染表与标签都不加） | `terminalui` `TestPalettesCoverTheStoredDomain`：`palette "sand" is a stored value with no colour spec`；`i18n` `TestPickerOptionsMatchTheStoredDomains`：`zh/palettes: id "sand" has no label`（+ 既有 `TestEveryLanguageIsComplete`） |
| M2 | 删掉 `enGlassStepLabels` 的 `liquid` 行 | `i18n` `TestPickerOptionsMatchTheStoredDomains`：`en/glass steps: id "liquid" has no label`（+ 既有 `TestEveryLanguageIsComplete`） |
| M3 | `Themes` 序改回 `dark, light, auto` | `i18n` `TestPickerOrderIsTheShippedOrder`：`themes: row 0 is "Dark", want "Auto"` / `row 2 is "Auto", want "Dark"`。**注**：`settingsui` 整个包 0 条红——这正是 §1.2 说的「行序此前无人守」 |
| M4 | `appIconBytes` 跳过归一化（`trayIcons[name]`） | `shell` `TestTheTwoAppIconFacesUseOneRule`：`the tray paints a different icon than the settings page names for ""` / `"Light"` / `"bogus"`；既有 `TestAppIconFollowsTheSetting`：`an unknown icon is not the dark one` |

还原方式：改前 `md5sum` 存三份被改文件（`valuedomain.go` / `i18n/options.go` /
`shell/system.go`），还原后 `md5sum -c` 三个「成功」，再 `touch` 复跑对应包（避免测试缓存
复用）全绿。

## 6. 不出图

非视觉轮：本轮没有改任何像素值、颜色、间距或字号，`Themes`/`GlassSteps`/`Scales` 的
行序与文案逐字保持。所以不出 PNG。

## 7. 仍待

1. 用户实机复测四条，连续第 78 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器 `loadLocked`
   + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序 + `Esc`/`Escape`
   + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS` + `Name` 缺失回落文件名主干
   + `Hidden`/`NoDisplay` 大小写）。
3. `internal/theme.Resolve`（`internal/theme/theme.go:38-50`）仍是对 `auto`/`light`/`dark` 的
   `switch`：它是消费者不是清单，本轮未动；若以后把 `Themes` 加一档（比如「跟随终端」），
   这里必须跟着加一条分支，`TestTheLoaderAndThePickableDomainsAgree` 不会发现（它只验
   `NormalizeTheme` 原样返回）。
4. 下一处候选：`internal/i18n` 里其余几张**非设置页**的 `[]Option` 表
   （`ShortcutsActions` / `BrowserSortOrders` / `BrowserSearchFields` / `CalculatorCopyModes`，
   英文 `copy.go:839/874/880/908` 一带、中文同族）——它们的 id 也有第二份消费者
   （`internal/browser`、`internal/shortcuts`、`internal/calculator`），与本轮同族；
   动手前先确认那些 id 是否真的落盘（落盘的才需要值域）。
