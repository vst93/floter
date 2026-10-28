# P11-v 规格：一门语言的三个名字只有一个答案

## 意图

「floter 支持哪些语言、存下来的值落到哪一门、设置页的选项表长什么样」这三个问题，
今天有三份答案；本轮把它们收成 `internal/i18n` 里的一张表。

为什么现在做：上一轮（P11-u）报告「仍待」第 4 条点名了这处；它也是「基础功能」——
用户改了 `settings.json` 里的 `language`、或者旧版本写进一个今天不认的值，落点由
`NormalizeLanguage` 决定，而界面文案由 `i18n.Normalize` 决定，两份分开写就可能
「设置页显示中文、界面是英文」。今天三份恰好同答案（所以本轮不是修 bug，是收口），
但#692「日语 locale」一旦拍板，加一门语言要改三处、漏一处就分叉。

## 现状（本轮实测行号，动手前确认过）

| 位置 | 内容 |
| --- | --- |
| `internal/settings/settings.go:41` | `DefaultLanguage = "en"` |
| `internal/settings/settings.go:114` | `languages = []string{"en", "zh"}`（第 1 份白名单） |
| `internal/settings/settings.go:551-559` | `NormalizeLanguage` 遍历上面那个 slice |
| `internal/i18n/i18n.go:11-27` | `Language` 类型、`EN`/`ZH`、`Normalize` 里硬写 `if Language(language) == ZH`（第 2 份白名单） |
| `internal/i18n/copy.go:344-347` | 英文副本的 `Languages: []Option{{"en","English"},{"zh","中文"}}`（第 3 份） |
| `internal/i18n/copy.go:1239-1242` | 中文副本的同名表（第 4 份，与第 3 份必须逐字相同） |

三份今天的答案一致：`en`/`zh`，默认 `en`。分歧只会在「加一门语言」时出现。

## 逐条落地项

1. **新增 `internal/i18n/language.go`**：一张表
   `languageTable = []struct{ id Language; endonym string }{{EN,"English"},{ZH,"中文"}}`，
   外带 `Default = EN`、`Languages() []Language`、`LanguageOptions() []Option`、
   `Normalize(string) Language`（从 `i18n.go` 搬来，改成遍历那张表）。
   顺序即设置页选项顺序；endonym 是「这门语言用自己的写法称呼自己」，两门语言下同字。
2. **`internal/i18n/i18n.go`**：删掉旧 `Normalize` 与它上面的注释（第 2 份白名单），
   `Language`/`EN`/`ZH` 留在原处（`Language` 是公开类型，`copy.go`/`shell` 都在用）。
3. **`internal/i18n/copy.go`**：两处 `Languages: []Option{…}` → `Languages: LanguageOptions()`。
   字段本身不动（`settingsui:3989` 的 `a.pick` 与 `view_test.go:456` 都读它），
   只是数据不再有第二份。
4. **`internal/settings/settings.go`**：
   - `DefaultLanguage = string(i18n.Default)`（常量转换，仍是 `const`）；
   - 删 `languages` slice；
   - `NormalizeLanguage` → `return string(i18n.Normalize(language))`，注释改成
     「白名单在 internal/i18n」。
   依赖方向：`settings → i18n`。`i18n` 不 import 任何 internal 包，无环（`grep -rn
   "floter/internal" internal/i18n/*.go` 空）。
5. **新增 pin（`internal/i18n/language_test.go`）**：
   - `TestLanguageOptionsComeFromTheWhitelist`：`LanguageOptions()` 的 ID 序列 ≡
     `Languages()`，标签非空且互不相同。
   - `TestBothCopiesShowTheSameLanguagePicker`：`For("en").Settings.Languages` 与
     `For("zh").Settings.Languages` 逐项相等（endonym 不翻译）。
6. **新增契约扫描（`internal/ci/language_contract_test.go`）**：
   - `TestLanguageWhitelistHasOneAnswer`：扫 `internal/`+`cmd/` 的 `.go`（跳过
     `internal/i18n`），命中 `[]string{"en", "zh"}` 或 `{"zh", "` 形状即红。
     `internal/desktopentry` 的 `locales`（`zh_CN`/`zh_TW`…）是 `.desktop` 的
     locale 匹配，不是白名单，不在扫描形状里。
   - `TestTheTwoLanguagePackagesAgree`：探针 `{"", "en", "zh", "zh-CN", "fr", "EN",
     " zh"}`，断言 `settings.NormalizeLanguage(v) == string(i18n.Normalize(v))`
     且结果 ∈ `i18n.Languages()`。这条把「设置页/界面各判一次」的分叉钉死。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**零改动**：
- `settings.json` 里 `language` 的合法值与落点不变（`en`/`zh`，其它 → `en`）；
- 设置页选项的 id 与显示字不变（`en`/English、`zh`/中文），选项顺序不变；
- 文案一个字不改。

本轮是纯重构：**V=0 / B=0 预期**（既有断言零改动必须全绿），S 记「4 份表 → 1 份」。

## 门槛与基线（本轮实测，非旧报告）

- `gofmt -l cmd internal` 空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` 净
- `go test -count=1 ./...` 全绿；PASS **2320**（本轮起点实测）；SKIP 15；FAIL 0；
  测试函数 **1879**（`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l`）
- 三平台 build 过

## mutation 计划

1. `settings.NormalizeLanguage` 改成 `return language`（去掉白名单）→
   `internal/ci` 的 `TestTheTwoLanguagePackagesAgree` 对 `"fr"` 变红（settings 放行、
   i18n 落 `en`）。还原后 `md5sum -c`。
2. `copy.go` 中文副本里 `{"zh","中文"}` 改成 `{"zh","Chinese"}` →
   `TestBothCopiesShowTheSameLanguagePicker` 变红（两门语言的选项表不再逐项相等）。
3. （附加）把 `var languages = []string{"en","zh"}` 塞回 `settings.go` 并让
   `NormalizeLanguage` 用它 → `TestLanguageWhitelistHasOneAnswer` 变红。

每条实跑、抄原始报错、`md5sum -c` 逐字节还原、`touch` 后复跑。
