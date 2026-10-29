# P11-bt 报告 · 终端页一帧只解析一份调色板

规格 `docs/rounds/P11-bt-spec.md`。选题理由：F1–F4 收完、合约 §6 只剩拍板项、P11-bs
仍待第 1 条是拍板项、第 2/3 条（`Element.Cache` / `#753`）都停在「`ui.Tester` 看不见
renderer 侧」的判据上；于是把 P11-bs 的收口推到**同一族里最热的表面**——终端页。

## 1. 改了什么

`internal/terminalui/terminalui.go`

- `App` 加 `tokenMemo theme.Memo` 与 `tokenStore *settings.Store`（后者只在换 store 时
  用来看出「两个 store 都从 version 0 开始」这件事）。
- `tokensOf`（原 `terminalui.go:991`）从「每次 `Store.Snapshot()` + `theme.ForPreferences`」
  改成「先按 store 指针判换，再走 `theme.Memo`」。**13 个调用点一个都没动**。
- `residentNote`（原 `:1228`）与 `feedbackNote`（原 `:1293`）里两处**直接**调
  `theme.ForPreferences` 的地方改走 `tokensOf(a, c)`：同一份输入（外观 + `ui.Preferences`），
  同一个答案。

`internal/terminalui/tokensmemo_test.go`（新）

- `TestOnePalettePerTerminalFrame`：一帧建 1 份；再来三帧仍是 1 份；**命中值 == 冷读者
  建的值**（逐字段比，`Tokens.Theme` 是指针不能整份 `%v`）；改设置 → 2 份；换外观 → 3 份。
- `TestASwappedStoreIsANewPalette`：换一个同样从 version 0 起、但外观不同的 store →
  memo 忘掉旧的（`Reset`），下一帧建 1 份，且拿到的是新 store 的深色调色板。
- `BenchmarkTerminalFrame`：终端页的一帧（面板 + 画布 + 条，握着一个会话）。

## 2. 实测（同一台机器，前后各 3 次，`-benchtime 2000x`）

```
AFTER   BenchmarkTerminalFrame-8   10421 / 10363 / 10909 ns/op   1432 B/op   39 allocs/op
BEFORE  BenchmarkTerminalFrame-8   18427 / 17500 / 19024 ns/op   4904 B/op   60 allocs/op
```

中位数 **18.43µs → 10.42µs（−43%）**，分配字节 **4904 → 1432 B（−71%）**，
分配次数 **60 → 39**。墙钟在不同负载下会抖（首轮空跑里 after 也出现过 21µs/op），
所以报告以**分配字节与次数**为准，时间只作量级参考。

**一个字节的墨都没动**：

```
FLOTER_PNG=1 go test -run TestDumpP11ZTerminalPagePNGs ./internal/terminalui/
cmp /tmp/p11bt-{after,before}-floter-P11-z-terminal-{dark,light}.png
  → SAME dark / SAME light
```

前后两份 PNG（深/浅各一张）`cmp` 逐字节相同。这张图上看得到的一处：
`floter-P11-z-terminal-dark.png` 里终端画布的近黑底、面板那唯一一圈描边、条上的
状态点与键帽，与修前逐像素一致——**调色板缓存不该、也没有移动任何一个像素**。

## 3. 台账

- **V = 0**：没有一个视觉数值需要重定基线（墨逐字节相同，见 §2）。
- **B = 0**：既有测试零改动全绿；产品对外行为零改动（同一份 settings、同一个外观、
  同一份 `ui.Preferences` → 同一份调色板）。
- **S = 0**：没有测试因结构变化改断言。

## 4. 门槛（实测）

```
gofmt -l cmd internal                    空
go vet ./...                             净
GOOS=windows go vet ./...                净
GOOS=darwin  go vet ./...                净
go test -count=1 ./...                   0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"  → 2467（基线 2465，+2）
... | grep -c -- "--- SKIP"                        → 54（不变）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 2021（基线 2019，+2）
GOOS=linux / windows / darwin go build ./...      三份过
```

## 5. mutation（3 条实跑变红 → `md5sum -c` 还原 → `touch` 复跑绿）

| # | 注入 | 实测报错（照抄） |
|---|------|-----------------|
| M1 | `tokensOf` 直接 `theme.ForPreferences`（绕过 memo） | `tokensmemo_test.go:37: the first frame built 0 palettes, want 1` + `tokensmemo_test.go:125: the first frame built 0 palettes, want 1` |
| M2 | 键里恒传 `0` 而不是 `Store.Version()` | `tokensmemo_test.go:67: a settings change left the memo at 1 builds, want 2` |
| M3 | 换 store 时不 `Reset()` | `tokensmemo_test.go:140: the swapped store served the old store's palette: light\|{247 247 248 255 [0 0 0]}\|{0 0 0 13 [0 0 0]}, want dark\|{14 14 16 255 [0 0 0]}\|{255 255 255 19 [0 0 0]}` |

三次都 `md5sum -c /tmp/p11bt.md5` 逐字节还原（`internal/terminalui/terminalui.go: 成功`），
`touch internal/terminalui/tokensmemo_test.go` 后复跑绿（`ok 0.063s`）。
**记一笔**：M1 之后我把「还原」做成了「用 python 反向打补丁」，两次打在已改过的文件上，
md5 校验当场报 `失败`，于是从 `/tmp/p11bt-orig.go` 整份拷回、`md5sum -c` 通过后再做下一条。
以后注入-还原只走「备份整份文件 + 拷回」。

## 6. 本轮顺带实测封死的候选（不是 TODO）

1. **`Store.Snapshot()` 的 18 个残余调用点不值得再收**（P11-bs 仍待第 4 条）。实测：设置页
   一帧 **General 15 次 / Integrations 53 次**，而 `Snapshot()` 本身
   **123.7 ns/op、48 B、1 alloc**（`clone` 只拷 `extra` 那张小 map）——53 次 ≈ **6.6µs**，
   对照 `BenchmarkIntegrationsFrame` 的 1.38ms 是 **0.5%**。**没有做**，也不该当 TODO 重领。
   与 P11-bs 的差别在于：那次省的是 `ForPreferences`（4µs 级 × 97），这次只是 `Snapshot`
   （0.12µs 级）。
2. **「谁在一直要帧」逐条核对完了**（P11-bp 仍待第 3 条）。产品里剩余三处
   `Context.AnimationFrame()`（rebuild）全部**有会结束的门**：
   `settingsui/savealert.go:119`（`!a.saveAlertShown`，且之后的秒数已由 P11-bq 的 deadline 接走）、
   `shell/notices.go:396`（`progress < 1`，`toastEntrances` 夹到 1）、
   `shell/shell.go:4305`（`entrance < 1`，`shellEnterProgress` 到 `shellEnter` 就把
   `enterLive` 关掉，`shellenter_test.go` 已钉）。**没有一条是无界的**，不需要改。
3. **两张没进过审计的表面实测是干净的**（各写了一次临时审计、跑完即删，不进仓库）：
   文件页（`EnterFiles` + 7 个 drop，两外观 × 默认/360 高度 × 空/命中/零匹配）与
   插件配置表单（`PluginSheet`，en/zh × 720×320 / 720×240 / 640×300）。
   要不要把它们变成常驻 pin 是下一轮的决定（会 +2 个测试函数），本轮不顺手带进来。
4. `glassmap.SpecForPreferencesOS`（`terminalui.go:840`）与 `shell/notices.go:331` 仍各有一处
   未 memo 的解析，但都**一帧一次**（不是每行一次），量级不值得动。

## 7. 仍待

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行）。
2. §6.1 与 §6.2 两条候选**已封死**，别再当 TODO 重领：`Snapshot` 残余调用点（0.5%）、
   剩余 `Context.AnimationFrame`（三条全有界）。
3. §6.3 的两个审计（文件页 / 插件表单）**没有进仓库**：下一轮若要做，直接给它们起正经文件名
   （`filesframe_test.go` / `pluginsheetframe_test.go`），别再用 `zz_temp*` 探针。
4. `Element.Cache` 仍未领：`ui.Tester` 不给 scene，测试看不到它的层绘制；而它的文档自己写着
   「文字在没有不透明底的子树里走灰度平滑」，所以**它会动墨**，不能拿「逐字节相同」当验收。
   要动它得先有一份能看见 renderer 侧的度量（或先给 mygo 提一个计数器）。
5. 设置页 `actionSpinner` 仍是上游缺口（构建期旋转），`Element.Cache` 也不解决它。
