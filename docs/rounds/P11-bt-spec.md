# P11-bt · 终端页一帧只解析一份调色板

## 0. 意图与选题

**意图**：终端页的一帧只建一次 `ui.Theme` + 调色板，不再让每个读者各自 `Store.Snapshot()`
+ `theme.ForPreferences`。

选题顺序：用户实测反馈 F1–F4 全部收完 → 合约 §6 全 `已落地`（只剩拍板项）→
P11-bs 仍待第 1 条是拍板项 → 第 2/3 条（`Element.Cache` / `#753` 脏矩形）都卡在「`ui.Tester`
看不见 renderer 侧」的判据上 → 于是按 P11-bs 自己的做法往**同一族的下一张表面**走：
P11-bs 只把设置页的 `a.tokens` 收进 `theme.Memo`，终端页从没做过，而它是这个应用里
**最热的帧**（程序每写一个字节就重画一次）。证据：

- `internal/terminalui/terminalui.go:991` 的 `tokensOf` 每次调用都
  `theme.ForPreferences(a.Store.Snapshot(), …)`；它在本包有 **13** 个调用点
  （`drawer.go:65,128,314,339`、`terminalui.go:814,909,937,1060,1344,1365,1414,1427`）。
- 另外两处**直接**调 `ForPreferences`：`terminalui.go:1228`（`residentNote`）、
  `terminalui.go:1293`（`feedbackNote`）——都逐帧跑。
- `theme.ForPreferences` 的代价在 P11-bs §1 量过：整份 `ui.Theme` + `paletteFor`
  （高对比度还要算 `contrast`），设置页上它是 Integrations 帧 CPU 的 29%。

## 1. 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/terminalui/terminalui.go:101`（`App`） | 没有帧内 memo | 新增 `tokenMemo theme.Memo` + `tokenStore *settings.Store` |
| `internal/terminalui/terminalui.go:991`（`tokensOf`） | 每次 `Store.Snapshot()` + `ForPreferences` | 换 store 指针先 `Reset()`，然后 `tokenMemo.Tokens(Store.Version(), c.Theme().Dark, c.Preferences(), Store.Snapshot)` |
| `terminalui.go:1228` / `:1293` | 各自 `theme.ForPreferences(a.Store.Snapshot(), t.Dark, c.Preferences())` | 改走 `tokensOf(a, c)`（同一份输入，同一个答案） |

**为什么不新写一个 memo**：`theme.Memo` 是 P11-bs 建的公共件，键 = store 版本 + 外观 +
`ui.Preferences` —— 正是 `ForPreferences` 的全部入参。启动器与设置页已经在用它，
终端页接上去就不会出现第二套键。

## 2. 行为契约零改动红线

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**一行没动**。
- 墨：`ForPreferences` 的返回值逐字节相同（命中值 == 未命中值，见 §3 的 `cmp` 证据）。
- 预览滑块（抽屉里的外观/透明度）仍然即时生效：改设置 → `Store` 的 version 前进 →
  下一帧重建。

## 3. 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                    → 空
go vet ./...                             → 净
GOOS=windows go vet ./...                → 净
GOOS=darwin  go vet ./...                → 净
go test -count=1 ./...                   → 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"          → 2467（基线 2465）
go test -count=1 -v ./... | grep -c -- "--- SKIP"          → 54（不变）
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l → 2021（基线 2019）
GOOS=linux/windows/darwin go build ./... → 三份过
```

## 4. mutation 计划（每条会让哪个 pin 变红）

- **M1**：`tokensOf` 绕过 memo（回到 `ForPreferences`）→
  `TestOnePalettePerTerminalFrame` 的「第一帧建 1 份」与
  `TestASwappedStoreIsANewPalette` 的同名断言红。
- **M2**：键里去掉 store 的 version（恒传 0）→ `TestOnePalettePerTerminalFrame`
  的「改了设置下一帧重建」红。
- **M3**：换 store 时不再 `Reset()` → `TestASwappedStoreIsANewPalette` 的
  「换了的 store 拿到旧 store 的调色板」红。
