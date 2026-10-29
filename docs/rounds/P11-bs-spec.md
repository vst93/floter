# P11-bs 规格：一帧只解析一份调色板（`theme.Memo`）

## 意图

**一句话**：设置页在一帧里把整套主题/调色板解析 **97 次**（每个 `a.tokens(c)` 调用点一次），
本机实测占 Integrations 帧 CPU 的 ~29%；本轮让它一帧只解析一次，并与启动器已有的那份
帧内 memo 合并成**一个答案**。

为什么现在做：P11-br §4「下一个可做的非拍板候选：`Element.Cache` 与 `#753` 脏矩形——
先量『设置页滚动时的重绘面积』再决定」。量下来最大的一块不是框架的绘制，而是**本仓自己的
重复计算**，先把这块拿掉，框架级缓存才有意义。

### 证据（本机实测，2026-10-29，i5-1135G7）

```
BenchmarkGeneralFrame-8         346206 ns/op   45897 B/op    399 allocs/op
BenchmarkIntegrationsFrame-8   1238687 ns/op  233449 B/op   1457 allocs/op
BenchmarkLauncherFrame-8        306061 ns/op  190000 B/op    125 allocs/op
```

`go test -bench IntegrationsFrame -cpuprofile` 的 top（560ms 采样，400 帧）：

```
110ms 19.64%  floter/internal/theme.ForPlatform   (cum)
 50ms  8.93%  floter/internal/theme.paletteFor    (cum)
 10ms  1.79%  floter/internal/settings.(*Store).Snapshot
```

`internal/settingsui/rows.go:30-32` 是唯一的入口：

```go
func (a *App) tokens(c *ui.Context) theme.Tokens {
	return theme.ForPreferences(a.Store.Snapshot(), c.Theme().Dark, c.Preferences())
}
```

`grep -c "a\.tokens(c)" internal/settingsui/*.go` 合计 **97** 处调用点，全在绘制期
（`rows.go` 24、`settingsui.go` 38、`config.go` 6、`parameditor.go` 6、`pluginsheet.go` 6 …），
所以一帧 = 97 次 `Store.Snapshot()`（整份设置 + 未认领键的 map 的深拷贝）+ 97 次
`theme.ForPreferences`（`paletteFor` 整套调色板，含 `contrast()` 的算术）。

启动器**早就有正确答案**：`internal/launcher/launcher.go:1114-1157` 的 `tokensKey` /
`tokensCached` / `tokensValid` 用 `Store.Version()` 做键（`internal/settings/store.go:52-57`
的注释就写着「same version, same value」），`internal/launcher/framememo_test.go` 也钉了它。
两套写法并存 = 同一个问题两个答案，而设置页这一套是慢的那个。

## 逐条落地项

### 1. `internal/theme/memo.go`（新文件）

- 现状：没有这个类型；launcher 的私有三字段 + settingsui 的每调用点重算。
- 应改成：`theme.Memo`，键 = `(settings 版本, dark, ui.Preferences)`，**惰性**取设置值
  （命中的那次不许 `Snapshot`），并带一个 `Builds()` 计数器（测试读它证明「一帧一份」）。

```go
type Memo struct { key memoKey; tok Tokens; valid bool; built uint64 }
type memoKey struct { version uint64; dark bool; prefs ui.Preferences }

func (m *Memo) Tokens(version uint64, dark bool, prefs ui.Preferences, load func() settings.Settings) Tokens
func (m *Memo) Builds() uint64
func (m *Memo) Reset()
```

`load` 只在未命中时调用 —— 这是「97 次 Snapshot → 1 次」的那一半。

### 2. `internal/settingsui/rows.go:30-32` + `internal/settingsui/settingsui.go` 的 `App`

- 现状：见上，每次调用重算。
- 应改成：`App` 加 `tokensStore *settings.Store` 与 `tokensMemo theme.Memo`，`a.tokens(c)`
  走 memo；`Store` 指针变了就 `Reset()`（测试里会换 store，`belowcolumn_test.go:179` 等，
  两个不同 store 的版本号可能都从 0 起，只按版本比会串味）。

### 3. `internal/launcher/launcher.go:1114-1157`

- 现状：私有 `tokensKey`/`tokensCached`/`tokensValid`（+ App 字段 `209-211`）。
- 应改成：换成同一个 `theme.Memo`，私有键类型删除。行为逐字节不变（同一个键、同一份值）。

### 4. pins

- `internal/theme/memo_test.go`：
  - 同键两次 → `Builds()==1`，第二次 `testing.AllocsPerRun` == 0；
  - `version` 变 → 重建，且新值可见（`UIScale` 改 `large` 后 `Body` 变大）；
  - `dark` 翻面 / `prefs.HighContrast` 翻面 → 重建（两个外观下 `Tokens` 不同）。
- `internal/settingsui/tokensmemo_test.go`：
  - 画一帧 General 页后 `a.tokensMemo.Builds()==1`（不是 97）；
  - 换 store 后重建（指针变了不许吃旧值）；
  - `Store.Update` 改 `UIScale` 后画一帧，卡片墨/字号跟着变（不许吃到 stale）。

## 行为契约零改动红线

- **墨零改动**：本轮的判据是 PNG 逐字节相同（`/tmp/floter-P11-bs-{general,integrations}-{light,dark}-{before,after}.png`）。
- 键盘模型、会话、设置写回、i18n 文案、命令接线、高度链：一行不碰。
- `theme.ForPreferences`/`ForPlatform`/`paletteFor` 的**输出**：一行不碰（只加一个键控的缓存）。

## 门槛命令 + PASS 基线

```
gofmt -l cmd internal      # 必须空
go vet ./...               # 净
GOOS=windows go vet ./...  # 净
GOOS=darwin  go vet ./...  # 净
go test -count=1 ./...     # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux/windows/darwin go build ./...
```

基线（P11-br 实测，本仓库 HEAD `c73db54`）：**PASS 2462 / 0 FAIL / 54 SKIP / 2016 测试函数**。
本轮预期 **PASS +5（2467）**、函数 2019…2020。PASS 只许涨。

## mutation 计划

| # | 注入 | 应红的 pin |
|---|------|-----------|
| M1 | `Memo.Tokens` 里删掉 `m.valid && m.key == key` 的比较（永远重建，仍正确/或永远命中） | `internal/theme` 的 memo 单测（命中的分配数 / 换版本可见新值） |
| M2 | `settingsui` 的 `a.tokens` 绕过 memo（直接 `theme.ForPreferences`） | `TestOneSettingsFrameResolvesOneCardLanguage`（`Builds()==1` 会读成 0） |
| M3 | `memoKey` 去掉 `version`（只按 dark/prefs） | 换设置后墨不变的 pin（stale） |
