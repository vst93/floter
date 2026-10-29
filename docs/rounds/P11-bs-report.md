# P11-bs 报告：一帧只解析一份调色板（`theme.Memo`）

## 1. 选题与证据

选题顺序：用户实测反馈（F1–F4 全部收完，见 `.pending-bugs.md` 顶部标题行）→ 合约
`docs/raycast-design.md` §6 全 `已落地`（只剩拍板项）→ 上一轮「仍待」第 1 条是拍板项
→ **P11-br §4 明写的下一个非拍板候选：`Element.Cache` / `#753` 脏矩形——「先量『设置页
滚动时的重绘面积』再决定」**。

量下来第一块不是框架的绘制，而是本仓自己的重复计算：

```
BEFORE（HEAD c73db54，本机 i5-1135G7）
BenchmarkGeneralFrame-8         346206 ns/op   45897 B/op    399 allocs/op
BenchmarkIntegrationsFrame-8   1238687 ns/op  233449 B/op   1457 allocs/op
```

`-cpuprofile`（Integrations，560ms 采样）里 `theme.ForPlatform` cum **19.6%**、
`theme.paletteFor` cum **8.9%**、`settings.(*Store).Snapshot` 1.8%。根因一处：

```
internal/settingsui/rows.go:30-32
func (a *App) tokens(c *ui.Context) theme.Tokens {
	return theme.ForPreferences(a.Store.Snapshot(), c.Theme().Dark, c.Preferences())
}
```

`grep -c "a\.tokens(c)" internal/settingsui/*.go` = **97**，全在绘制期。于是一帧 = 97 次
`Store.Snapshot()`（整份设置 + 未认领键 map 的深拷贝）+ 97 次 `ForPreferences`（整套
`ui.Theme` + `paletteFor`）。而**启动器早就有正确答案**（`internal/launcher/launcher.go`
的 `tokensKey`/`tokensCached`/`tokensValid` + `internal/launcher/framememo_test.go` 的
pin）——同一个问题两个答案，设置页那个是慢的。

## 2. 改了什麼

| 文件 | 改动 |
|------|------|
| `internal/theme/memo.go`（新，71 行） | `theme.Memo`：键 = `(settings 版本, dark, ui.Preferences)`；`Tokens(version, dark, prefs, load)` **只在未命中时**调 `load` 取设置值；`Builds()` 计数给测试看；`Reset()` |
| `internal/theme/memo_test.go`（新） | `TestTheMemoBuildsOnceAndNeverServesAStaleLook` |
| `internal/settingsui/rows.go:30-50` | `a.tokens` 走 memo；`Store` 指针变了先 `Reset()`（两个 store 版本号都从 0 起，靠身份不靠版本） |
| `internal/settingsui/settingsui.go:646-656` | `App` 加 `tokensStore *settings.Store` / `tokensMemo theme.Memo` |
| `internal/settingsui/tokensmemo_test.go`（新） | 一帧一份 + 设置/store 变了不许 stale |
| `internal/launcher/launcher.go:205-212, 1113-1132` | 删掉私有 `tokensKey`/`tokensCached`/`tokensValid`，换成同一个 `theme.Memo`（行为逐字节不变） |

`internal/theme/theme.go` **一行没改**：`ForPreferences`/`ForPlatform`/`paletteFor` 的输出
不变，只是多了一个键控缓存。

## 3. 实测数字

### 3.1 确定性指标（不看机器忙闲）

```
GeneralFrame      allocs 399 → 225 (−44%)    B/op 45,897 → 17,090 (−63%)
IntegrationsFrame allocs 1457 → 329 (−77%)   B/op 233,449 → 46,952 (−80%)
```

### 3.2 墙钟：交错测量 + 一个没被本轮碰过的对照

机器当时在被别的进程占（load 1.0–1.8），单独跑前后没有可比性（同一份代码两次能差 1.7×）。
所以把**本轮没碰的 `BenchmarkMatch`（排序，纯 CPU）当机器速度对照**，前后各跑 5 次取最小值：

| 基准 | 前（best/5） | 后（best/5） | 对照 `Match` | 归一化后 |
|------|-------------|-------------|--------------|----------|
| `Match`（对照） | 118,090 ns | 94,485 ns | — | 机器当时快 0.80× |
| `GeneralFrame` | 544,144 ns | 436,168 ns | 118,090 → 94,485 | 544,144×0.80=435,3xx ≈ 436,168 → **持平** |
| `IntegrationsFrame` | 2,009,711 ns | 1,218,628 ns | 同上 | 2,009,711×0.80=1,607,769 → **−24%**（对照比：17.02 → 12.90） |

**诚实的读法**：Integrations 页（10 张卡 × 5 条命令行，≈97 个 tokens 调用点）真快了约
1/4，和 profile 里「theme 占 29%」对得上；General 页的调用点少（行少），去掉的这点重复
计算被布局/绘制的固定成本盖住，测不出差别。所以本轮的价值是**分配量 −44%/−77%** 与
「一帧一份」这条契约，不是「每页都快 30%」。

### 3.3 墨零改动（逐字节）

```
FLOTER_PNG=1 go test ./internal/settingsui/ -run TestDumpSettingsPagePNGs
cmp /tmp/floter-P11-bs-integrations-{light,dark}-{before,after}.png   → SAME / SAME
```

前后各出一份 integrations 页（浅/深）的 PNG，`cmp` 逐字节相同。这就是「本轮不碰墨」的证据；
**本轮不是视觉轮，不出新图**（P11-br 同规矩）。

## 4. 台账

**V = 0 / B = 0 / S = 0**。没有产品行为改动：memo 的键就是 `ForPreferences` 的全部入参
（设置值、外观、桌面偏好），命中的值与重建的值是同一个；`a.tokens` 的调用点一处没动。

## 5. 门槛（实测）

```
gofmt -l cmd internal                    空
go vet ./...                             净
GOOS=windows go vet ./...                净
GOOS=darwin  go vet ./...                净
GOOS=linux/windows/darwin go build ./... 三平台过
go test -count=1 ./...                   0 FAIL
```

| 指标 | 基线（P11-br / `c73db54`） | 本轮 |
|------|---------------------------|------|
| PASS | 2462 | **2465**（+3，本轮 3 条新 pin） |
| FAIL | 0 | 0 |
| SKIP | 54 | 54（未新增 `t.Skip`） |
| 测试函数 | 2016 | **2019** |

## 6. mutation（3 条实跑变红 → `md5sum -c` 还原 → `touch` 复跑绿）

| # | 注入 | 实测报错 |
|---|------|---------|
| M1 | `Memo.Tokens` 删掉键比较（`if m.valid` 永远命中） | `memo_test.go:47: a settings change left the memo at 1 builds, want 2 (a stale key)` |
| M2 | `settingsui` 的 `a.tokens` 绕过 memo，直接 `theme.ForPreferences` | `tokensmemo_test.go:22: one General frame built 0 palettes, want 1` + `:46 with the integrations page drawn the memo built 0 palettes over two frames, want 1` |
| M3 | `memoKey` 去掉 `version`（只按 dark/prefs） | `tokensmemo_test.go:73: the update left the memo at 1 builds, want 2` + `memo_test.go:47: ... (a stale key)` |

每条还原后 `md5sum -c` 逐字节核对通过，`touch` 后再跑一次绿。

## 7. 仍待

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行）。
2. **`Element.Cache` 还没领**：本轮量出来的是「本仓自己的重复计算」，框架级缓存留在下一轮，
   但**判据要先定**——`Cache` 省的是 renderer 侧的层绘制，`Tester` 看不到（它不给 scene），
   所以要么先找到可测的代理指标（例如设置页滚动时的帧时间分布），要么改成
   「`Element.Cache` 只加在真正贵的静态子树（卡片底/阴影/玻璃）+ 墨逐字节不变」的验收方式。
   **别只凭「框架有这功能」就加**。
3. `#753` 脏矩形是框架内部的（`ui/runtime.go` 的 `dirty`），本仓没有可读的 damage 统计；
   想量「重绘面积」得先给 mygo 提一个计数器或自己在测试里 diff 两张 PNG 的差异矩形——
   下一轮若还做性能，从这里入手。
4. `internal/settingsui` 还有 18 处 `Store.Snapshot()`（非 tokens 路径，如 `a.language()`）；
   一帧里跑几次、值不值得一并 memo，**先量再说**（本轮不猜）。
5. 设置页 `actionSpinner` 仍是上游缺口（P11-br §2），`Element.Cache` 也不解决它（构建期旋转）。
