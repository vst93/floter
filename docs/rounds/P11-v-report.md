# P11-v 报告：一门语言的三个名字只有一个答案

## 1. 做了什么

方向按用户定的「三平台一致性 → 基础功能 → 像素细节」，落在**基础功能**：
「floter 支持哪些语言 / 存下来的值落到哪一门 / 设置页的选项表长什么样」这三问，
本轮之前有四份答案（`settings.languages`、`i18n.Normalize` 的硬比较、`copy.go` 里
en 与 zh 各一份 `Languages: []Option{…}`）。它们今天恰好同答案，所以本轮不是修 bug，
是把「加一门语言要改四处」收成「改一张表」——#692 日语一旦拍板就是那个时刻。
规格 `docs/rounds/P11-v-spec.md`。

- 新增 `internal/i18n/language.go`（56 行）：`languageTable`（id + endonym 一张表）、
  `Default = EN`、`Languages() []Language`、`LanguageOptions() []Option`、
  `Normalize`（从 `i18n.go` 搬来，改成遍历那张表而不是 `== ZH`）。
- `internal/i18n/i18n.go`：旧 `Normalize` 删除（-10 行）。
- `internal/i18n/copy.go`：两处 `Languages: []Option{{"en","English"},{"zh","中文"}}`
  → `Languages: LanguageOptions()`（-8 行）。
- `internal/settings/settings.go`：`DefaultLanguage = string(i18n.Default)`（仍是 `const`）、
  删 `languages` slice、`NormalizeLanguage` 转发 `i18n.Normalize`。
  依赖方向 `settings → i18n`；`i18n` 不 import 任何 internal 包，无环
  （`grep -rn "floter/internal" internal/i18n/*.go` 空，本轮复核过）。

净：产品代码 3 文件 **+14 / −25**；新增 237 行测试与契约（含规格）。

## 2. V/B/S 台账

- **V=0**：非视觉轮，无像素 pin。
- **B=0**：`internal/settings`、`internal/settingsui`、`internal/i18n` 的既有断言
  **零改动**全绿（含 `settings_test.go:79` 的 `Language != DefaultLanguage`、
  `view_test.go:456` 读 `copy.Languages[0].Label`/`[1].Label`、
  `i18n_test.go:81` 的 `TestForNormalizesTheLanguage`）。
- **S=2**：①4 份语言表 → 1 张表；②`settings.NormalizeLanguage` 由自持白名单 →
  转发 `i18n`。两条都是结构变化，断言面没有需要重定基线的数值。
- 新增测试函数 **4 条**：`TestLanguageOptionsComeFromTheWhitelist`、
  `TestBothCopiesShowTheSameLanguagePicker`（`internal/i18n/language_test.go`）、
  `TestLanguageWhitelistHasOneAnswer`、`TestTheTwoLanguagePackagesAgree`
  （`internal/ci/language_contract_test.go`）。

## 3. 门槛实测

| 门槛 | 结果 |
| --- | --- |
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` | 净 |
| `GOOS=windows go vet ./...` | 净 |
| `GOOS=darwin go vet ./...` | 净 |
| `go test -count=1 ./...` | 全绿，FAIL **0** |
| PASS 计数 | **2324**（本轮起点实测 2320） |
| SKIP | **15**（不变） |
| 测试函数 | **1883**（起点 1879） |
| 三平台 build | `GOOS=linux/windows/darwin go build ./...` 全过 |

非视觉轮，不出图。

## 4. mutation 实跑（3 条，全部先红后逐字节还原）

基线 `md5sum`：`settings.go bdf6cb7c…`、`copy.go 094899a5…`、`language.go 4f7f9998…`。

1. `settings.NormalizeLanguage` 改成 `return language`（去掉白名单）→
   `TestTheTwoLanguagePackagesAgree` 变红，原始报错：
   ```
   --- FAIL: TestTheTwoLanguagePackagesAgree (0.00s)
       language_contract_test.go:93: stored "": settings lands on "" but the copy lands on "en"
       language_contract_test.go:96: stored "": settings lands on "", which is not a shipped language ([en zh])
       language_contract_test.go:93: stored "zh-CN": settings lands on "zh-CN" but the copy lands on "en"
       language_contract_test.go:93: stored "fr": settings lands on "fr" but the copy lands on "en"
   ```
2. `copy.go` 中文副本的选项表改成 `[]Option{{"en","English"},{"zh","Chinese"}}` →
   ```
   --- FAIL: TestBothCopiesShowTheSameLanguagePicker (0.00s)
       language_test.go:61: language option 1 differs between copies: en {zh 中文}, zh {zh Chinese}
   ```
3. 把 `languages = []string{"en", "zh"}` 塞回 `settings.go:114` 并让
   `NormalizeLanguage` 用它 → 源码扫描 pin 变红：
   ```
   --- FAIL: TestLanguageWhitelistHasOneAnswer (0.01s)
       language_contract_test.go:63: internal/settings/settings.go:114 spells the language whitelist again ([]string{"en", "zh"}): read it from i18n.Languages/LanguageOptions/Normalize (see internal/i18n/language.go)
   ```

每条还原后 `md5sum -c /tmp/p11v.md5` 三行全「成功」，`touch` 相关包后复跑
（避免测试缓存）均 `ok`。

## 5. 为什么没顺手做别的

- **没有加日语**（#692）：加语言是拍板项，本轮只把「加一门语言要改几处」变成一处。
- **没有碰 `internal/desktopentry` 的 `locales`**（`zh_CN`/`zh_TW`…）：那是 `.desktop`
  文件的 locale 匹配顺序，不是语言白名单，契约扫描的 needle 形状也刻意避开它。
- **没有碰 `scriptlang.Languages`**（8 种脚本语言）：那是集成脚本的运行时语言，
  与界面语言是两个问题。

## 6. 仍待（P11-v）

1. 用户实机复测四条，连续第 77 轮（背景透明 / 外圈底色+三大金刚 / 卡死 / 打包实跑）。
2. 需拍板清单不变：P9-g 表（#692 日语 locale、#642 `MYGO_LDFLAGS`、
   `ShortcutTargetName` 无扩展名）+ register 只留 `cmd` + `open` 是否拒一切 query +
   未知宿主是否静默忽略；剪贴板轮询 600 vs 900、复制回来是否置顶；
   计算器每次写是否从磁盘重读；`usage.Record` 写失败是否回滚内存；
   「搜索页答案行」Enter 之后算什么；非 macOS 快捷键词序；`Esc`/`Escape`；
   SVG/XPM 图标；icns 落盘缓存；发布 `dist/SHA256SUMS`；`Name` 缺失回落文件名主干；
   `Hidden`/`NoDisplay` 大小写。
3. 下一处候选（自己验过再动手）：`internal/i18n/copy.go` 的 `Themes`/`GlassSteps`/
   `Scales`/`CursorShapes`/`Paddings`/`Palettes`/`AppIcons` 七张选项表与
   `internal/settings` 的同名值域 slice（`themes`/`glassSteps`/`uiScaleFactors`…）
   是**同一个值域的两种写法**——与语言表同族，但它们的 label 是**要翻译的文案**
   （不像 endonym），收口方式不同：id 序只有一份、label 仍按语言各自写。
   动它要先确认 `settingsui` 的写回路径会不会存下 id 之外的字符串。
