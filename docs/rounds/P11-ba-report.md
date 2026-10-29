# P11-ba 报告：中文界面 + 终端最小宽度，三面「字压字 / 出窗」审计

## 1. 选它做什么

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（`docs/raycast-design.md`
§6 全表 `已落地`，只剩 §4.6 的 `⌘K` 动作面＝拍板）→ 上一轮「仍待」第 1 条（五条拍板项，
本轮不动）→ **界面质感与交互逻辑**（用户 2026-10-28 定的优先档）。

这一档里，可见缺陷一条都没量到：`zh` 的启动器（空页 / 命中 / 零结果，720×488、640×360，
两外观）、`zh` 的设置五页（860×900、720×560，两外观）、`zh` 的终端抽屉（640/720×360、
720×600）**全部零压字、零出窗**（本轮开工时用临时探针实测，见 §4）。所以本轮不去制造一个
「产品改动」，而是把已有的三张审计表补到**用户真正在看的界面**上——`zh` 语言，和终端允许的
**最小宽度 640**。这是「界面质感」这一档里唯一不需要拍板、又能真实防回归的一步。

## 2. 根因（为什么原来量不到）

三张审计表都只量了英文和常规宽度：

- `internal/launcher/textframe_test.go` 的 `TestTheLauncherPaintsNoTextOverText` 只循环
  `dark`，语言取 `settings.Default()`（`en`）。
- `internal/settingsui/textframe_test.go` 的 `TestTheSettingsPagesPaintNoTextOverText` 同上。
- `internal/terminalui/draweroverlap_test.go` 的
  `TestTheDrawerRowsKeepTheirWordsInsideTheirCells` 宽度是 `{720, 900, 1100}`，
  漏了 `settings.MinTerminalWidth` = **640**。

用户用中文写反馈，读的是 `zh`；两个语言的词长不同（中文分组头是四个宽字形，英文是两个词），
同一批盒子里字宽不同，「英文不压字」不蕴含「中文不压字」。最窄的 640 才是抽屉格子最容易
挤爆的那一档。

## 3. 落地

1. **launcher**：`TestTheLauncherPaintsNoTextOverText` 外层加 `lang ∈ {en, zh}`，
   `(lang, appearance)` 一个 `t.Run` 子测试；语言经
   `a.Store.Update(func(s *settings.Settings){ s.Language = lang })` 设进夹具。
2. **settingsui**：`TestTheSettingsPagesPaintNoTextOverText` 加 `lang ∈ {en, zh}`，
   `(lang, appearance, size)` 一个子测试，五页在子测试里循环；语言同上写进 store。
3. **terminalui**：`TestTheDrawerRowsKeepTheirWordsInsideTheirCells` 的宽度表
   从 `{720, 900, 1100}` 改成 `{settings.MinTerminalWidth, 720, 900, 1100}`。

**产品代码零改动**（只动三个 `_test.go`）。没有改判据、没有加容差、没有 `t.Skip`、没有删断言。

## 4. 台账（V / B / S）

- **V = 0**：没有一个墨、面、几何值改动。
- **B = 0**：既有 pin 零改动全绿（三张审计表原来的断言一条没动，只加循环）。
- **S = 0**：没有换结构；`t.Run` 只是把同一批断言拆成命名的子测试。
- 测试函数数 **1966**（不变，没加/删测试函数）。
- PASS **2415 → 2427**（+12：launcher 由 1 个 PASS 变成 4 个子测试 +3；settings 由 1 个
  变成 8 个子测试 +7；两个父测试的 PASS 行仍在。其余 +2 来自子测试树的父/子行计数）。
  FAIL **0**。SKIP **37**（不变）。

## 5. 门槛（本轮实测）

- `gofmt -l cmd internal`：空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**全绿**，FAIL **0**
- PASS 计数：**2427**（基线 2415）
- SKIP：**37**（与 HEAD 同一批）
- 三平台 `go build ./...`：过

## 6. mutation（两条实跑变红并还原）

- **M1**（launcher 臂，含 zh）：`internal/launcher/launcher.go:1295` 的
  `Padding(listEdge, t.Space(2), …)` → `Padding(0, t.Space(2), …)`。红：
  ```
  --- FAIL: TestTheLauncherPaintsNoTextOverText/en_light
      light en empty: "Restart the computer" {X:54 Y:0 W:166 H:24} is painted over
      "Search" {X:54 Y:15.5 W:512 H:25} (overlap 166.0x8.5)
      light en match: "git" {X:54 Y:0 W:20 H:24} is painted over
      "Search" {X:54 Y:15.5 W:578 H:25} (overlap 20.0x8.5)
  --- FAIL: TestTheLauncherPaintsNoTextOverText/en_dark
      (同形)
  ```
  `en` 与 `zh` 两套子测试都红。还原后 `md5sum` = `dcbbc885a9541ad40d50001c2e454f8c`，
  `touch` 复跑绿。
- **M2**（terminalui 臂，含新的 640）：`internal/terminalui/drawer.go:365` 的
  `ui.Select(...).Grow(1)` → `ui.Select(...).Absolute().Left(0).Top(0).Grow(1)`。红：
  ```
  --- FAIL: TestTheDrawerRowsKeepTheirWordsInsideTheirCells
      light 640: "Terminal palette" {X:12 Y:438 W:116 H:22} paints over
      "Follow app" {X:27 Y:445 W:72 H:22} (overlap 72.0x15.0)
      light 720: … light 900: … light 1100: …
      dark 640: … dark 720: … dark 900: … dark 1100: …
  ```
  **新的 640 臂与旧的 720/900/1100 一起红**——证明 640 那一格真的在跑。
  还原后 `md5sum` = `957c841cddbc5c7b851406528a78efec`，`touch` 复跑绿。

## 7. 视觉证据

本轮产品代码零改动，界面像素与 HEAD **逐字节相同**（没有出图：没有任何可见变化）。
`zh` 三面在开工时用临时探针实测零压字、零出窗，探针随后删除，结论由 §3 的永久审计臂守住。

## 8. 仍待

1. 五条拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1），外加 P11-an 的「终端便条投影跟不跟 `terminal_opacity`」。
2. **新拍板候选（本轮侦察，未实现）**：终端的「进程已退出」便条（`residentNote`）与失败提示
   （`feedbackNote`）是**浮动卡**（`Absolute` + `tokens.Float` + 投影）。用户 F3 的原话是
   「不要 floating/overlay 观感，元素要 inline、要贴进布局」。把便条改成 inline（占据自己
   一条带、压短画布）会改变会话可见行数与高度节奏，属**交互/布局决定**，不擅自实现。
3. **侦察结论（本轮实测，别当 TODO 重领）**：`zh` 的启动器 / 设置五页 / 终端抽屉在
   720×488、640×360、860×900、720×560、640×360、720×600 全部零压字、零出窗；
   `internal/inputtext` 与参考 `inputNormalize.ts` 逐条同答案（不是缺陷）。
