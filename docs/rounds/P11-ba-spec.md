# P11-ba 规格：中文界面 + 终端最小宽度，三面「字压字 / 出窗」审计

## 意图

用户 2026-10-28 定的优先级是「界面质感与交互逻辑优先」。F1/F2/F3 已由 P11-y/z/aa 收完，
合约 §6 全表 `已落地`，上一轮「仍待」第 1 条是五条拍板项。这一档里**唯一还能自己推进、
不需要拍板**的一件事，是把已有的「一条面上没有两段字压在同一块像素上」审计补到**用户
真正在看的那套界面**上。

## 为什么现在做（证据）

用户用中文写反馈，说明他实际读的是 `zh` 界面；而三张审计表一直只量英文：

- `internal/launcher/textframe_test.go` 的 `TestTheLauncherPaintsNoTextOverText`：
  只循环 `dark`，语言取 `settings.Default()`（= `i18n.Default` = `en`）。
- `internal/settingsui/textframe_test.go` 的 `TestTheSettingsPagesPaintNoTextOverText`：
  同上，只量 `en`。
- `internal/terminalui/draweroverlap_test.go` 的 `TestTheDrawerRowsKeepTheirWordsInsideTheirCells`：
  宽度只量 `720 / 900 / 1100`，**漏了终端允许的最小宽度 640**
  （`settings.MinTerminalWidth` = 640）。

两个语言的词长不同（中文分组头是四个宽字形，英文是两个词），同一批盒子里的字宽不同，
所以「英文不压字」不蕴含「中文不压字」；最窄的 640 才是格子最容易挤爆的那一档。

## 落地项（位置 → 现状 → 应改成）

1. `internal/launcher/textframe_test.go:107` 的 `TestTheLauncherPaintsNoTextOverText`
   → 只量 `en` → 外层加 `lang ∈ {en, zh}`，每个 `(lang, appearance)` 一个 `t.Run` 子测试。
2. `internal/settingsui/textframe_test.go:100` 的 `TestTheSettingsPagesPaintNoTextOverText`
   → 只量 `en` → 加 `lang ∈ {en, zh}`，`(lang, appearance, size)` 一个子测试，五页在里面循环。
3. `internal/terminalui/draweroverlap_test.go:36` 的
   `TestTheDrawerRowsKeepTheirWordsInsideTheirCells`
   → 宽度 `{720, 900, 1100}` → `{settings.MinTerminalWidth, 720, 900, 1100}`。

## 行为契约零改动红线

- 产品代码**零改动**（只改 `_test.go`）。高度链、键盘模型、会话管理、设置写回、i18n 文案、
  命令接线一律不碰。
- 不改任何断言的判据、不加容差、不 `t.Skip`、不删断言。

## 门槛命令 + PASS 基线（本轮实测当前值）

```
gofmt -l cmd internal                         # 必须空
go vet ./...                                  # 必须净
GOOS=windows go vet ./...                     # 必须净
GOOS=darwin  go vet ./...                     # 必须净
go test -count=1 ./...                        # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（P11-az 实测）：PASS **2415** / FAIL **0** / SKIP **37** / 测试函数 **1966**。

## mutation 计划（≥2 条，说明每条会让哪个 pin 变红）

- **M1**（launcher 臂，含 zh）：把 `internal/launcher/launcher.go:1295` 列表的
  `Padding(listEdge, …)` 顶内边距改成 `Padding(0, …)`，让第一行顶到字段行底下。
  → `TestTheLauncherPaintsNoTextOverText` 的 `en_light` / `en_dark` / `zh_*` 子测试全红
  （`"Restart the computer"` 压在 `"Search"` 上）。
- **M2**（terminalui 臂，含新的 640）：把 `internal/terminalui/drawer.go:365` 的调色板
  `ui.Select(...)` 改成 `Absolute().Left(0).Top(0)`，让它压在自己的标签上。
  → `TestTheDrawerRowsKeepTheirWordsInsideTheirCells` 的 `640 / 720 / 900 / 1100` 全红
  （`"Terminal palette"` 压 `"Follow app"`，重叠 72×15u）。

每条注入后看到红、还原、`md5sum -c` 逐字节核对、`touch` 再复跑一次绿。
