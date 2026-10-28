# P11-w 规格：一个值域的 id 清单只有一个答案

## 意图（一句话）

「这个设置能存哪些 id」这个问题，今天在 `internal/i18n`（14 张 `[]Option` 字面量）、
`internal/settings`（4 个 `[]string` + 1 个 `map[string]float64`）、`internal/terminalui`
（`palettes` 的 map 键）、`internal/glassmap`（`stops` 的 map 键）、`internal/shell` 与
`internal/settingsui`（两处 `== "light"`）里各有答案；本轮把它收成**一张 id 表**
（新 `internal/valuedomain`），其余各面全部转发，并给三张「渲染参数表」加键集相等 pin。

## 为什么现在做（证据）

上一轮（P11-v）把语言那一族收成了 `internal/i18n/language.go` 一张表，收尾「仍待」第 3 条
指的就是同族的其余七张表。它们的现状与风险：

- **加一个 id 要改 2–4 处，漏一处不报错**。例如 `internal/terminalui/terminalui.go:1404`
  `themeFor` 对 `palettes[name]` 查不到时静默退回 `inherit`（`internal/terminalui/terminalui.go:1404-1412`），
  `internal/terminalui/terminalui.go:1374` `panelFill` 同理：在 `copy.Palettes` 与
  `settings.terminalPalettes` 里加一个 `sand`、忘了给颜色 → 拾取器里有这一行、存得下、
  渲染出来是「跟随应用」，**全仓没有任何测试会红**。
- **同一个值域两份写法，顺序还不一样**：`internal/settings/settings.go:114`
  `themes = []string{"dark", "light", "auto"}`，而 `internal/i18n/copy.go:339-343`
  （英文）与 `:1231-1235`（中文）是 `auto, light, dark`。界面序只由后者决定
  （`internal/settingsui/view_test.go:455` 钉住首尾标签），前者只做成员检查——两份答案是
  同一个集合的两个序，今天无害，但「哪个序算数」没有写下来的地方。
- **应用图标的归一化写了两遍**：`internal/shell/system.go:51-57` `storedAppIcon` 与
  `internal/settingsui/settingsui.go:4442-4450` `appIcon` 是同一段 `if value == "light"`；
  值域本身（`dark`/`light`）第三遍写在 `internal/i18n/copy.go:335-338` / `:1227-1230`。
  `app_icon` 是 `Extra` 键（`internal/settings/settings.go:169` 之外，`SetExtra` 原样存），
  所以它没有 `settings` 侧归一化，两处私有拷贝就是唯一的答案。

## 落地项（位置 → 现状 → 应改成）

1. **新增 `internal/valuedomain/valuedomain.go`**（叶子包，不 import 任何 internal 包）：
   - `AppIcons` `Themes` `GlassSteps` `Scales` `CursorShapes` `Paddings` `Palettes`
     `BoldModes` 八张 `[]string`，**序取界面序**（唯一可观测的序；成员检查不看序）；
   - `ScaleFactors map[string]float64`（`tiny 0.8 / small 0.9 / default 1.0 / large 1.1`，
     从 `internal/settings/settings.go:138-143` 搬来，因为「一个 step 多大」是这一格的
     定义，不是 settings 的私事）；
   - `Has(ids []string, id string) bool`；
   - `NormalizeAppIcon(name string) string`（`light` → `light`，其余一律 `dark`，即
     `internal/shell/system.go:53-57` 的那条规则）。
2. **`internal/settings/settings.go`**：删 `themes`/`glassSteps`/`cursorShapes`/
   `terminalPaddings`/`terminalPalettes`/`boldModes`/`uiScaleFactors` 七个局部值域
   （`:114-124`、`:138-143`），`NormalizeTheme`（`:495-504`）、`NormalizeGlassStep`
   （`:542-556`）、`normalized()` 的三处 `Pick`（`:406`、`:407`、`:408`、`:410`）、
   `NormalizeUIScale`/`UIScaleFactor`（`:563-578`）改读 `valuedomain`。
   `legacyGlassSteps`/`legacyUIScales` **不动**（那是迁移表，不是值域）。
3. **`internal/i18n/copy.go`**：14 张 `[]Option` 字面量（英文 `:335-378`、中文 `:1227-1270`）
   改成 `optionsFor(valuedomain.X, map[string]string{id: 文案…})`。新增
   `internal/i18n/options.go` 放 `optionsFor`（按 id 序产出 `[]Option`，标签缺失留空，
   不 panic、不回落成 id）。
4. **`internal/shell/system.go`**：`storedAppIcon` 转发 `valuedomain.NormalizeAppIcon`；
   `appIconBytes`（`:40-47`）也走同一条归一化再选字节。
5. **`internal/settingsui/settingsui.go`**：`appIcon`（`:4442-4450`）转发
   `valuedomain.NormalizeAppIcon`。

## 新增 pin（都是 B 类，产品回归才红）

- `internal/i18n` `TestPickerOptionsMatchTheStoredDomains`：两份 `Copy` × 七张表，
  `len(options) == len(domain)`、`options[i].ID == domain[i]`、`Label` 非空且 `!= ID`。
- `internal/valuedomain` `TestEveryScaleStepHasAFactor`：`Scales` 的每个 id 都在
  `ScaleFactors` 里且为正；`ScaleFactors` 不多余任何键。
- `internal/glassmap` `TestGlassStopsCoverTheStoredDomain`：`keys(stops) ∪ {"off"} ==
  valuedomain.GlassSteps`（今天 3 个 stop + `off`，靠人记）。
- `internal/terminalui` `TestPalettesCoverTheStoredDomain`：`keys(palettes) ∪ {"inherit"}
  == valuedomain.Palettes`。
- `internal/ci` `TestValueDomainsHaveOneAnswer`：源码扫描，`internal/valuedomain` 与
  `internal/i18n` 之外不得出现第二份 id 清单（`[]string{"off", "frosted"`、
  `[]string{"auto", "light", "dark"}`、`[]string{"dark", "light", "auto"}`、
  `[]string{"beam"`、`[]string{"compact"`、`[]string{"inherit", "contrast"`、
  `[]string{"font", "bright"`）与第二份应用图标判据（`== "light"`）。

## 行为契约零改动红线

- **界面序不变**：`Themes` 仍是 `auto, light, dark`（`internal/settingsui/view_test.go:455`
  零改动必须绿）；`GlassSteps`/`Scales`/`CursorShapes`/`Paddings`/`Palettes`/`AppIcons`
  的界面序与今天逐字一致。
- **存下来的值不变**：`Pick`/`NormalizeGlassStep`/`NormalizeUIScale`/`UIScaleFactor` 的
  结果对任何输入都与今天相同（含 `legacyGlassSteps`、`legacyUIScales` 的迁移）。
- 不碰高度链、键盘模型、会话管理、设置写回路径、i18n 文案、命令接线。
- 非视觉轮：**不出图**，不改任何像素值。

## 门槛命令与 PASS 基线（本轮实测起点）

```
gofmt -l cmd internal                                   # 空
go vet ./...                                            # 净
GOOS=windows go vet ./...                               # 净
GOOS=darwin  go vet ./...                               # 净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 2324（本轮实测起点）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

起点实测：PASS **2324** · SKIP **15** · FAIL **0** · 测试函数 **1883** · 42 行包
（40 ok + 2 no test files）。

## mutation 计划（≥2 条，每条说明谁变红）

1. 在 `valuedomain.Palettes` 里插一个 `sand`（i18n 与 terminalui 都不加）→
   `TestPalettesCoverTheStoredDomain` 红（渲染参数表缺一格）。
2. 把 `i18n` 的 `GlassSteps` 标签表里 `liquid` 那条删掉 → `TestPickerOptionsMatchTheStoredDomains`
   红（`Label` 为空）。
3. 把 `Themes` 的序改回 `dark, light, auto` → `internal/settingsui/view_test.go:455` 红
   （界面序是行为），同时 `TestPickerOptionsMatchTheStoredDomains` 仍绿——证明两条 pin 分工不同。
4. 把 `NormalizeAppIcon` 改成 `strings.ToLower(name) == "light"` 之外的第二套判据
   （如 `name == "Light"`）→ 新增的 `internal/shell` 应用图标测试红。
